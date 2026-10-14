package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/xjasonlyu/tun2socks/v2/engine"
	_ "github.com/xjasonlyu/tun2socks/v2/proxy/direct"
)

func runCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func findPhysicalInterface() string {
	psCmd := `(Get-NetRoute -DestinationPrefix '0.0.0.0/0' | Where-Object { $_.InterfaceAlias -notlike '*Jozmoz*' -and $_.InterfaceAlias -notlike '*Wintun*' -and $_.InterfaceAlias -notlike '*tun*' } | Sort-Object RouteMetric | Select-Object -First 1).InterfaceAlias`
	out, err := runCommand("powershell", "-NoProfile", "-Command", psCmd)
	if err == nil && len(strings.TrimSpace(out)) > 0 {
		name := strings.TrimSpace(out)
		if _, err := net.InterfaceByName(name); err == nil {
			return name
		}
	}
	return ""
}

func configureAdapter(name, ip, dnsServer string) error {
	log.Printf("[JozmozTun] Configuring adapter %s (IP: %s, DNS: %s)...", name, ip, dnsServer)

	// Wait up to 5 seconds for the Wintun adapter to appear in Windows
	var found bool
	for i := 0; i < 25; i++ {
		out, err := runCommand("powershell", "-NoProfile", "-Command", fmt.Sprintf("Get-NetAdapter -Name '%s' -ErrorAction SilentlyContinue | Select-Object -ExpandProperty ifIndex", name))
		if err == nil && len(strings.TrimSpace(out)) > 0 {
			found = true
			log.Printf("[JozmozTun] Adapter %s detected with ifIndex %s", name, out)
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	if !found {
		return fmt.Errorf("timeout waiting for adapter %s to appear", name)
	}

	// Configure Static IP, on-link route 0.0.0.0/0, Metric, and Profile exactly like xray_tun
	psConfig := fmt.Sprintf(`
		$ErrorActionPreference = 'SilentlyContinue'
		$ad = Get-NetAdapter -Name '%s'
		if ($ad) {
			$cur = (Get-NetIPAddress -InterfaceIndex $ad.ifIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue).IPAddress
			if ($cur -ne '%s') {
				Get-NetIPAddress -InterfaceIndex $ad.ifIndex -AddressFamily IPv4 -ErrorAction SilentlyContinue | Remove-NetIPAddress -Confirm:$false -ErrorAction SilentlyContinue
				New-NetIPAddress -InterfaceIndex $ad.ifIndex -IPAddress '%s' -PrefixLength 30 -ErrorAction SilentlyContinue | Out-Null
			}
			
			# Add on-link default route 0.0.0.0/0 -> 0.0.0.0 with RouteMetric 0 (enables "Internet access" in Windows flyout)
			$r = Get-NetRoute -InterfaceIndex $ad.ifIndex -DestinationPrefix '0.0.0.0/0' -ErrorAction SilentlyContinue
			if (-not $r) {
				New-NetRoute -InterfaceIndex $ad.ifIndex -DestinationPrefix '0.0.0.0/0' -NextHop '0.0.0.0' -RouteMetric 0 -ErrorAction SilentlyContinue | Out-Null
			}
			
			# Set DNS on this adapter
			Set-DnsClientServerAddress -InterfaceIndex $ad.ifIndex -ServerAddresses @('%s', '1.1.1.1') -ErrorAction SilentlyContinue
			
			# Set highest priority metric (1) like xray_tun
			Set-NetIPInterface -InterfaceIndex $ad.ifIndex -InterfaceMetric 1 -AutomaticMetric Disabled -ErrorAction SilentlyContinue
			
			# Name resolution and network profile
			Start-Sleep -Milliseconds 400
			$prof = Get-NetConnectionProfile -InterfaceAlias '%s' -ErrorAction SilentlyContinue
			if ($prof) {
				Set-NetConnectionProfile -InterfaceAlias '%s' -NetworkCategory Public -ErrorAction SilentlyContinue
			}
		}
	`, name, ip, ip, dnsServer, name, name)

	_, err := runCommand("powershell", "-NoProfile", "-Command", psConfig)
	return err
}

func main() {
	adapterName := flag.String("name", "JozmozDNS", "Wintun adapter cosmetic name")
	dnsIP := flag.String("dns", "167.104.219.20", "DNS server IP address")
	tunIP := flag.String("ip", "172.19.0.1", "TUN IPv4 address")
	flag.String("gw", "0.0.0.0", "TUN IPv4 default gateway")
	flag.Parse()

	log.Printf("═════════════════════════════════════════════════════════")
	log.Printf(" 🛡️ JozmozDNS Virtual System Tunnel Engine")
	log.Printf(" Adapter: %s | DNS: %s | IP: %s", *adapterName, *dnsIP, *tunIP)
	log.Printf("═════════════════════════════════════════════════════════")

	// Ensure wintun.dll is available in execution directory or System32
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	wintunPath := filepath.Join(exeDir, "wintun.dll")
	if _, err := os.Stat(wintunPath); os.IsNotExist(err) {
		// Try current directory
		if _, err := os.Stat("wintun.dll"); err == nil {
			_ = copyFile("wintun.dll", wintunPath)
		}
	}

	physIface := findPhysicalInterface()
	if physIface != "" {
		log.Printf("[JozmozTun] Bound to physical outbound interface: %s", physIface)
	}

	key := &engine.Key{
		Device:    fmt.Sprintf("tun://%s", *adapterName),
		Proxy:     "direct://",
		Interface: physIface,
		MTU:       1500,
		LogLevel:  "info",
	}

	engine.Insert(key)
	go func() {
		engine.Start()
	}()

	// Configure network adapter properties in Windows
	go func() {
		time.Sleep(1 * time.Second)
		if err := configureAdapter(*adapterName, *tunIP, *dnsIP); err != nil {
			log.Printf("[JozmozTun] Configure warning: %v", err)
		} else {
			log.Printf("[JozmozTun] ✓ Adapter %s successfully configured & activated in Windows!", *adapterName)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("[JozmozTun] Tunnel engine running. Press Ctrl+C to terminate.")
	<-sig

	log.Printf("[JozmozTun] Shutting down tunnel engine...")
	engine.Stop()
	log.Printf("[JozmozTun] ✓ Tunnel closed and adapter removed cleanly.")
}

func copyFile(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, in, 0o644)
}
