package main

import (
	"flag"
	"fmt"
	"log"
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

func configureAdapter(name, ip, gateway, dnsServer string) error {
	log.Printf("[JozmozTun] Configuring adapter %s (IP: %s, GW: %s, DNS: %s)...", name, ip, gateway, dnsServer)

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

	// 1. Configure Static IP & Gateway on the Wintun adapter
	psConfig := fmt.Sprintf(`
		$ErrorActionPreference = 'SilentlyContinue'
		$ad = Get-NetAdapter -Name '%s'
		if ($ad) {
			# Set Static IP
			New-NetIPAddress -InterfaceIndex $ad.ifIndex -IPAddress '%s' -PrefixLength 24 -DefaultGateway '%s' -ErrorAction SilentlyContinue | Out-Null
			
			# Set DNS on this adapter
			Set-DnsClientServerAddress -InterfaceIndex $ad.ifIndex -ServerAddresses @('%s', '1.1.1.1') -ErrorAction SilentlyContinue
			
			# Set Interface Metric with high priority (5) so Windows selects it as primary Ethernet connection
			Set-NetIPInterface -InterfaceIndex $ad.ifIndex -InterfaceMetric 5 -ErrorAction SilentlyContinue
			
			# Name resolution and network profile
			Start-Sleep -Milliseconds 500
			$prof = Get-NetConnectionProfile -InterfaceAlias '%s' -ErrorAction SilentlyContinue
			if ($prof) {
				Set-NetConnectionProfile -InterfaceAlias '%s' -NetworkCategory Private -ErrorAction SilentlyContinue
			}
		}
	`, name, ip, gateway, dnsServer, name, name)

	_, err := runCommand("powershell", "-NoProfile", "-Command", psConfig)
	return err
}

func main() {
	adapterName := flag.String("name", "JozmozDNS", "Wintun adapter cosmetic name")
	dnsIP := flag.String("dns", "167.104.219.20", "DNS server IP address")
	tunIP := flag.String("ip", "10.254.1.2", "TUN IPv4 address")
	tunGW := flag.String("gw", "10.254.1.1", "TUN IPv4 default gateway")
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

	key := &engine.Key{
		Device:   fmt.Sprintf("wintun://%s", *adapterName),
		Proxy:    "direct://",
		MTU:      1500,
		LogLevel: "silent",
	}

	engine.Insert(key)
	go func() {
		engine.Start()
	}()

	// Configure network adapter properties in Windows
	go func() {
		time.Sleep(1 * time.Second)
		if err := configureAdapter(*adapterName, *tunIP, *tunGW, *dnsIP); err != nil {
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
