#!/usr/bin/env bash
# ==============================================================================
# HyperDNS — Interactive Terminal Management Console
# Usage: hyperdns or hdns
# GitHub: https://github.com/jozmoz/HyperDNS
# ==============================================================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
PURPLE='\033[0;35m'
BOLD='\033[1m'
NC='\033[0m'

INSTALL_DIR="/opt/hyperdns"
SERVICE_NAME="hyperdns"
REPO_RAW_URL="https://raw.githubusercontent.com/jozmoz/HyperDNS/main"

# Detect binary location
if [ -f "$INSTALL_DIR/hyperdns" ]; then
    BIN_PATH="$INSTALL_DIR/hyperdns"
elif [ -f "/root/HyperDNS-main/hyperdns" ]; then
    BIN_PATH="/root/HyperDNS-main/hyperdns"
elif [ -f "/root/hyperdns" ]; then
    BIN_PATH="/root/hyperdns"
else
    BIN_PATH="/opt/hyperdns/hyperdns"
fi

get_public_ip() {
    curl -4 -s --connect-timeout 3 https://api.ipify.org 2>/dev/null || \
    curl -4 -s --connect-timeout 3 https://ifconfig.me 2>/dev/null || \
    echo "127.0.0.1"
}

get_service_status() {
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        echo -e "${GREEN}● ONLINE (Active)${NC}"
    else
        echo -e "${RED}● OFFLINE (Stopped)${NC}"
    fi
}

show_banner() {
    clear 2>/dev/null || true
    echo -e "${CYAN}${BOLD}"
    echo "  ██╗  ██╗██╗   ██╗██████╗ ███████╗██████╗ ██████╗ ███╗   ██╗███████╗"
    echo "  ██║  ██║╚██╗ ██╔╝██╔══██╗██╔════╝██╔══██╗██╔══██╗████╗  ██║██╔════╝"
    echo "  ███████║ ╚████╔╝ ██████╔╝█████╗  ██████╔╝██║  ██║██╔██╗ ██║███████╗"
    echo "  ██╔══██║  ╚██╔╝  ██╔═══╝ ██╔══╝  ██╔══██╗██║  ██║██║╚██╗██║╚════██║"
    echo "  ██║  ██║   ██║   ██║     ███████╗██║  ██║██████╔╝██║ ╚████║███████║"
    echo "  ╚═╝  ╚═╝   ╚═╝   ╚═╝     ╚══════╝╚═╝  ╚═╝╚═════╝ ╚═╝  ╚═══╝╚══════╝"
    echo -e "       ${PURPLE}⚡ Standalone SmartDNS & Anti-Sanction Gaming Gateway ⚡${NC}"
    echo -e "       ${YELLOW}Management Console · GitHub: github.com/jozmoz/HyperDNS${NC}"
    echo ""
    echo -e "  ${BOLD}Status:${NC} $(get_service_status) | ${BOLD}Server IP:${NC} ${CYAN}$(get_public_ip)${NC}"
    echo -e "${CYAN}────────────────────────────────────────────────────────────────────────${NC}"
}

show_credentials() {
    echo -e "\n${BOLD}${CYAN}=== HyperDNS Dashboard Information ===${NC}"
    local journal_log
    journal_log=$(journalctl -u "$SERVICE_NAME" -n 250 --no-pager 2>/dev/null || true)
    
    if [ -f "$INSTALL_DIR/install.log" ]; then
        journal_log="$journal_log"$'\n'"$(cat "$INSTALL_DIR/install.log" 2>/dev/null)"
    fi

    local dash_line
    dash_line=$(echo "$journal_log" | grep -i "HyperDNS Dashboard" | tail -1 || true)
    if [ -n "$dash_line" ]; then
        echo -e "  ${GREEN}$dash_line${NC}"
    else
        local admin_path
        admin_path=$(echo "$journal_log" | grep -oE "https://[^ ]+/[a-f0-9]{16}/dash/login" | tail -1 || true)
        if [ -n "$admin_path" ]; then
            echo -e "  ${GREEN}Dashboard URL: $admin_path${NC}"
        else
            echo -e "  ${YELLOW}Dashboard URL: Check journal logs below:${NC}"
            echo "$journal_log" | grep -iE "Dashboard|admin panel path" | tail -3 || true
        fi
    fi

    local user_line
    user_line=$(echo "$journal_log" | grep -A 2 -i "first-run credentials" | tail -2 || true)
    if [ -n "$user_line" ]; then
        echo -e "\n${BOLD}Initial Credentials:${NC}"
        echo "$user_line"
    else
        local u_line p_line
        u_line=$(echo "$journal_log" | grep -E "username\s*:" | tail -1 || true)
        p_line=$(echo "$journal_log" | grep -E "password\s*:" | tail -1 || true)
        if [ -n "$u_line" ] && [ -n "$p_line" ]; then
            echo -e "\n${BOLD}Initial Credentials:${NC}"
            echo "    $u_line"
            echo "    $p_line"
        fi
    fi
    echo ""
    read -rp "Press Enter to continue..."
}

change_domain_ssl() {
    echo -e "\n${BOLD}${CYAN}=== Domain & SSL Certificate Management ===${NC}"
    echo -e "  ${CYAN}[1]${NC} Set / Change Domain with Let's Encrypt Auto SSL"
    echo -e "  ${CYAN}[2]${NC} Install Custom SSL Certificate (.crt & .key)"
    echo -e "  ${CYAN}[3]${NC} Force Renew / Re-request Let's Encrypt Certificate"
    echo -e "  ${YELLOW}[0]${NC} Return to Main Menu"
    read -rp " Choose option [0-3]: " ssl_choice

    case "$ssl_choice" in
        1)
            echo -e "\n${YELLOW}Ensure an 'A' record points from your domain to $(get_public_ip) before proceeding.${NC}\n"
            read -rp "Enter new domain name (e.g. dns.example.com): " new_domain
            new_domain=$(echo "$new_domain" | tr -d '[:space:]')
            if [ -z "$new_domain" ]; then
                echo -e "${RED}Domain cannot be empty.${NC}"
                sleep 2
                return
            fi
            read -rp "Enter Admin Email for Let's Encrypt: " new_email
            new_email=$(echo "$new_email" | tr -d '[:space:]')

            echo -e "\n${CYAN}Stopping service temporarily...${NC}"
            systemctl stop "$SERVICE_NAME" 2>/dev/null || true

            echo -e "${CYAN}Requesting SSL & updating domain to '${new_domain}'...${NC}"
            if [ -f "$BIN_PATH" ]; then
                timeout 15 "$BIN_PATH" -server -domain "$new_domain" -email "$new_email" >/dev/null 2>&1 || true
            fi

            echo -e "${GREEN}Restarting HyperDNS service...${NC}"
            systemctl restart "$SERVICE_NAME" || systemctl start "$SERVICE_NAME"
            echo -e "${GREEN}✓ Domain and SSL updated to ${new_domain}!${NC}"
            sleep 2
            ;;
        2)
            echo -e "\n${BOLD}Custom SSL Certificate Installation${NC}"
            read -rp "Enter full path to certificate file (.crt or .pem): " cert_path
            read -rp "Enter full path to private key file (.key): " key_path
            if [ ! -f "$cert_path" ] || [ ! -f "$key_path" ]; then
                echo -e "${RED}Certificate or Key file not found!${NC}"
                sleep 2
                return
            fi
            mkdir -p "$INSTALL_DIR/certs"
            cp "$cert_path" "$INSTALL_DIR/certs/custom.crt"
            cp "$key_path" "$INSTALL_DIR/certs/custom.key"
            echo -e "${GREEN}✓ Certificates saved to $INSTALL_DIR/certs/custom.{crt,key}${NC}"
            echo -e "${CYAN}Restarting HyperDNS...${NC}"
            systemctl restart "$SERVICE_NAME"
            sleep 2
            ;;
        3)
            echo -e "\n${CYAN}Clearing cached certificates in $INSTALL_DIR/certs/acme/...${NC}"
            systemctl stop "$SERVICE_NAME" 2>/dev/null || true
            rm -rf "$INSTALL_DIR/certs/acme"/* 2>/dev/null || true
            echo -e "${GREEN}Restarting HyperDNS to re-request certificate...${NC}"
            systemctl restart "$SERVICE_NAME" || systemctl start "$SERVICE_NAME"
            echo -e "${GREEN}✓ Certificate cache flushed and renewal triggered!${NC}"
            sleep 2
            ;;
        *)
            return
            ;;
    esac
}

update_hyperdns() {
    echo -e "\n${BOLD}${CYAN}=== Update HyperDNS from GitHub ===${NC}"
    echo -e "Downloading latest release from: ${YELLOW}${REPO_RAW_URL}/hyperdns-linux${NC}..."
    
    local tmp_file="/tmp/hyperdns_update_$$"
    if curl -fsSL -o "$tmp_file" "${REPO_RAW_URL}/hyperdns-linux"; then
        chmod +x "$tmp_file"
        echo -e "${CYAN}Stopping HyperDNS service...${NC}"
        systemctl stop "$SERVICE_NAME" 2>/dev/null || true

        echo -e "${CYAN}Replacing binary at ${BIN_PATH}...${NC}"
        cp "$tmp_file" "$BIN_PATH"
        chmod +x "$BIN_PATH"
        rm -f "$tmp_file"

        if [ "$BIN_PATH" != "/opt/hyperdns/hyperdns" ] && [ -d "/opt/hyperdns" ]; then
            cp "$BIN_PATH" "/opt/hyperdns/hyperdns" 2>/dev/null || true
        fi

        # Also update the menu script itself
        echo -e "${CYAN}Updating management menu script...${NC}"
        curl -fsSL -o /usr/local/bin/hyperdns "${REPO_RAW_URL}/scripts/hyperdns-menu.sh" 2>/dev/null || true
        chmod +x /usr/local/bin/hyperdns 2>/dev/null || true
        ln -sf /usr/local/bin/hyperdns /usr/local/bin/hdns 2>/dev/null || true

        echo -e "${CYAN}Restarting HyperDNS service...${NC}"
        systemctl restart "$SERVICE_NAME" || systemctl start "$SERVICE_NAME"
        echo -e "\n${GREEN}${BOLD}✓ HyperDNS and Management Menu updated to the latest version successfully!${NC}"
    else
        echo -e "\n${RED}Failed to download updated binary from GitHub.${NC}"
        rm -f "$tmp_file"
    fi
    read -rp "Press Enter to continue..."
}

flush_cache() {
    echo -e "\n${CYAN}Flushing DNS cache...${NC}"
    if [ -f "$BIN_PATH" ]; then
        "$BIN_PATH" flush 2>/dev/null || systemctl restart "$SERVICE_NAME"
        echo -e "${GREEN}✓ DNS Cache flushed successfully!${NC}"
    else
        systemctl restart "$SERVICE_NAME"
        echo -e "${GREEN}✓ Service restarted (cache cleared)!${NC}"
    fi
    sleep 1.5
}

run_diagnostics() {
    echo -e "\n${BOLD}${CYAN}=== HyperDNS Diagnostic Port Check ===${NC}"
    echo "Testing local DNS, DoT, DoH and SNI Proxy ports..."
    for port in 53 853 8443 80 443; do
        if ss -tuln 2>/dev/null | grep -q ":${port} "; then
            echo -e "  Port ${port}: ${GREEN}OPEN & LISTENING${NC}"
        elif netstat -tuln 2>/dev/null | grep -q ":${port} "; then
            echo -e "  Port ${port}: ${GREEN}OPEN & LISTENING${NC}"
        else
            echo -e "  Port ${port}: ${RED}CLOSED / NOT LISTENING${NC}"
        fi
    done
    echo ""
    read -rp "Press Enter to continue..."
}

uninstall_hyperdns() {
    echo -e "\n${RED}${BOLD}======================================================${NC}"
    echo -e "${RED}${BOLD}           ⚠️  UNINSTALL HYPERDNS  ⚠️                ${NC}"
    echo -e "${RED}${BOLD}======================================================${NC}"
    echo -e "This will stop the service, remove binaries and restore system settings."
    read -rp "Are you SURE you want to completely uninstall? (y/N): " confirm
    if [[ "$confirm" =~ ^[Yy]$ ]]; then
        systemctl stop "$SERVICE_NAME" 2>/dev/null || true
        systemctl disable "$SERVICE_NAME" 2>/dev/null || true
        rm -f /etc/systemd/system/hyperdns.service
        systemctl daemon-reload

        rm -f /usr/local/bin/hyperdns /usr/local/bin/hdns

        read -rp "Do you also want to delete all database and settings in /opt/hyperdns? (y/N): " del_data
        if [[ "$del_data" =~ ^[Yy]$ ]]; then
            rm -rf /opt/hyperdns
            echo -e "${RED}✓ Data directory /opt/hyperdns removed.${NC}"
        else
            echo -e "${YELLOW}Data preserved in /opt/hyperdns for backup.${NC}"
        fi

        # Restore systemd-resolved if it was modified
        if [ -f /etc/systemd/resolved.conf.d/hyperdns.conf ]; then
            rm -f /etc/systemd/resolved.conf.d/hyperdns.conf
            systemctl restart systemd-resolved 2>/dev/null || true
        fi
        systemctl enable systemd-resolved 2>/dev/null || true
        systemctl start systemd-resolved 2>/dev/null || true

        echo -e "\n${GREEN}HyperDNS has been uninstalled successfully.${NC}"
        exit 0
    else
        echo -e "${YELLOW}Uninstall cancelled.${NC}"
        sleep 1.5
    fi
}

main_menu() {
    while true; do
        show_banner
        echo -e "  ${CYAN}[1]${NC}  📊 وضعیت سرویس (Service Status)"
        echo -e "  ${CYAN}[2]${NC}  🔄 راه‌اندازی مجدد سرویس (Restart HyperDNS)"
        echo -e "  ${CYAN}[3]${NC}  ⏹️  توقف موقت سرویس (Stop Service)"
        echo -e "  ${CYAN}[4]${NC}  ▶️  روشن کردن سرویس (Start Service)"
        echo -e "  ${CYAN}[5]${NC}  📜 مشاهده لاگ‌های زنده سرور (View Live Logs)"
        echo -e "  ${CYAN}[6]${NC}  🔑 لینک داشبورد و مشخصات ورود (Dashboard URL & Credentials)"
        echo -e "  ${CYAN}[7]${NC}  🌐 تغییر دامنه و صدور گواهینامه SSL / Let's Encrypt"
        echo -e "  ${CYAN}[8]${NC}  🧹 پاک‌سازی کش DNS (Flush DNS Cache)"
        echo -e "  ${CYAN}[9]${NC}  🩺 بررسی پورت‌ها و عیب‌یابی (Diagnostics & Port Check)"
        echo -e "  ${CYAN}[10]${NC} 🚀 آپدیت HyperDNS به آخرین نسخه از گیت‌هاب (Update)"
        echo -e "  ${RED}[11]${NC} 🗑️  حذف کامل HyperDNS (Uninstall)"
        echo -e "  ${YELLOW}[0]${NC}  🚪 خروج (Exit)"
        echo -e "${CYAN}────────────────────────────────────────────────────────────────────────${NC}"
        read -rp " عدد مورد نظر را وارد کنید [0-11]: " choice

        case "$choice" in
            1)
                echo -e "\n${BOLD}--- Systemd Service Status ---${NC}"
                systemctl status "$SERVICE_NAME" --no-pager || true
                read -rp "Press Enter to continue..."
                ;;
            2)
                echo -e "\n${CYAN}Restarting HyperDNS...${NC}"
                systemctl restart "$SERVICE_NAME"
                echo -e "${GREEN}✓ Service restarted!${NC}"
                sleep 1.5
                ;;
            3)
                echo -e "\n${YELLOW}Stopping HyperDNS...${NC}"
                systemctl stop "$SERVICE_NAME"
                echo -e "${YELLOW}✓ Service stopped!${NC}"
                sleep 1.5
                ;;
            4)
                echo -e "\n${CYAN}Starting HyperDNS...${NC}"
                systemctl start "$SERVICE_NAME"
                echo -e "${GREEN}✓ Service started!${NC}"
                sleep 1.5
                ;;
            5)
                echo -e "\n${CYAN}Viewing live logs (Press Ctrl+C to return to menu)...${NC}"
                sleep 1
                journalctl -u "$SERVICE_NAME" -f -n 50 || true
                ;;
            6)
                show_credentials
                ;;
            7)
                change_domain_ssl
                ;;
            8)
                flush_cache
                ;;
            9)
                run_diagnostics
                ;;
            10)
                update_hyperdns
                ;;
            11)
                uninstall_hyperdns
                ;;
            0|q|exit)
                echo -e "\n${GREEN}خداحافظ!${NC}\n"
                exit 0
                ;;
            *)
                echo -e "\n${RED}گزینه نامعتبر است!${NC}"
                sleep 1
                ;;
        esac
    done
}

# Run menu
main_menu
