#!/usr/bin/env bash
# ==============================================================================
# JozmozDNS — Interactive Terminal Management Console
# Usage: jdns or jozmozdns (alias: jmdns, JozmozDNS)
# GitHub: https://github.com/jozmoz/JozmozDNS
# ==============================================================================

set +e

RED='\033[0;31m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
PURPLE='\033[0;35m'
BOLD='\033[1m'
NC='\033[0m'

INSTALL_DIR="/opt/JozmozDNS"
SERVICE_NAME="JozmozDNS"
REPO_RAW_URL="https://raw.githubusercontent.com/jozmoz/HyperDNS/main"

# Detect binary location
if [ -f "$INSTALL_DIR/jozmozdns" ]; then
    BIN_PATH="$INSTALL_DIR/jozmozdns"
elif [ -f "$INSTALL_DIR/JozmozDNS" ]; then
    BIN_PATH="$INSTALL_DIR/JozmozDNS"
elif [ -f "/opt/jozmozdns/jozmozdns" ]; then
    BIN_PATH="/opt/jozmozdns/jozmozdns"
elif [ -f "/root/JozmozDNS-main/JozmozDNS" ]; then
    BIN_PATH="/root/JozmozDNS-main/JozmozDNS"
elif [ -f "/root/JozmozDNS" ]; then
    BIN_PATH="/root/JozmozDNS"
else
    BIN_PATH="/opt/JozmozDNS/JozmozDNS"
fi

get_public_ip() {
    local svc_ip
    if [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
        svc_ip=$(grep -oE -- '-public-ip\s+[^ ]+' "/etc/systemd/system/${SERVICE_NAME}.service" 2>/dev/null | awk '{print $2}' | tr -d '[:space:]' || true)
    fi
    if [ -n "$svc_ip" ]; then
        echo "$svc_ip"
        return
    fi
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
    echo "  ██╗ ██████╗ ███████╗███╗   ███╗ ██████╗ ███████╗██████╗ ███╗   ██╗███████╗"
    echo "  ██║██╔═══██╗╚══███╔╝████╗ ████║██╔═══██╗╚══███╔╝██╔══██╗████╗  ██║██╔════╝"
    echo "  ██║██║   ██║  ███╔╝ ██╔████╔██║██║   ██║  ███╔╝ ██║  ██║██╔██╗ ██║███████╗"
    echo "  ██║██║   ██║ ███╔╝  ██║╚██╔╝██║██║   ██║ ███╔╝  ██║  ██║██║╚██╗██║╚════██║"
    echo "  ██║╚██████╔╝███████╗██║ ╚═╝ ██║╚██████╔╝███████╗██████╔╝██║ ╚████║███████║"
    echo "  ╚═╝ ╚═════╝ ╚══════╝╚═╝     ╚═╝ ╚═════╝ ╚══════╝╚═════╝ ╚═╝  ╚═══╝╚══════╝"
    echo -e "       ${PURPLE}⚡ Standalone SmartDNS & Anti-Sanction Gaming Gateway ⚡${NC}"
    echo -e "       ${YELLOW}Management Console · GitHub: github.com/jozmoz/JozmozDNS${NC}"
    echo ""
    echo -e "  ${BOLD}Status:${NC} $(get_service_status) | ${BOLD}Server IP:${NC} ${CYAN}$(get_public_ip)${NC}"
    echo -e "${CYAN}────────────────────────────────────────────────────────────────────────${NC}"
}

show_credentials() {
    echo -e "\n${BOLD}${CYAN}=== JozmozDNS Dashboard Information ===${NC}"

    local saved_user="" saved_pass="" saved_url=""
    if [ -f "$INSTALL_DIR/credentials.txt" ]; then
        saved_user=$(grep -E "^username\s*:" "$INSTALL_DIR/credentials.txt" 2>/dev/null | awk -F':' '{print $2}' | tr -d '[:space:]' || true)
        saved_pass=$(grep -E "^password\s*:" "$INSTALL_DIR/credentials.txt" 2>/dev/null | awk -F':' '{print $2}' | tr -d '[:space:]' || true)
        saved_url=$(grep -E "^admin_url\s*:" "$INSTALL_DIR/credentials.txt" 2>/dev/null | sed -e 's/^admin_url\s*:\s*//' | tr -d '[:space:]' || true)
    fi

    local journal_log
    journal_log=$(journalctl -u "$SERVICE_NAME" -n 250 --no-pager 2>/dev/null || true)
    
    if [ -f "$INSTALL_DIR/install.log" ]; then
        journal_log="$journal_log"$'\n'"$(cat "$INSTALL_DIR/install.log" 2>/dev/null)"
    fi

    local dash_line
    dash_line=$(echo "$journal_log" | grep -i "JozmozDNS Dashboard" | tail -1 || true)
    if [ -n "$dash_line" ]; then
        echo -e "  ${GREEN}$dash_line${NC}"
    elif [ -n "$saved_url" ]; then
        echo -e "  ${GREEN}Dashboard URL: $saved_url${NC}"
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

    if [ -n "$saved_user" ] && [ -n "$saved_pass" ]; then
        echo -e "\n${BOLD}Admin Credentials:${NC}"
        echo -e "    ${BOLD}username :${NC} ${YELLOW}$saved_user${NC}"
        echo -e "    ${BOLD}password :${NC} ${YELLOW}$saved_pass${NC}"
    else
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
    fi
    echo ""
    read -rp "Press Enter to continue..."
}

# Helper: Detect if Port is occupied on a specific IP or wildcard
# Usage: is_port_busy <port> [ip]
is_port_busy() {
    local port="$1"
    local ip="${2:-}"

    if [ -n "$ip" ] && [ "$ip" != "0.0.0.0" ] && [ "$ip" != "::" ]; then
        local esc_ip
        esc_ip=$(echo "$ip" | sed 's/\./\\./g')
        local pattern="(0\.0\.0\.0|\[::\]|:::?|\*|${esc_ip}):${port}([[:space:]]|$)"
        if command -v ss >/dev/null 2>&1; then
            if ss -tuln 2>/dev/null | grep -E "$pattern" >/dev/null 2>&1; then
                return 0
            fi
            return 1
        fi
        if command -v netstat >/dev/null 2>&1; then
            if netstat -tuln 2>/dev/null | grep -E "$pattern" >/dev/null 2>&1; then
                return 0
            fi
            return 1
        fi
        if command -v lsof >/dev/null 2>&1; then
            if lsof -i "TCP@${ip}:${port}" >/dev/null 2>&1 || lsof -i "UDP@${ip}:${port}" >/dev/null 2>&1 || \
               lsof -i "TCP@0.0.0.0:${port}" >/dev/null 2>&1 || lsof -i "UDP@0.0.0.0:${port}" >/dev/null 2>&1; then
                return 0
            fi
            return 1
        fi
    else
        if command -v ss >/dev/null 2>&1; then
            if ss -tuln 2>/dev/null | grep -qE ":(${port})[[:space:]]"; then
                return 0
            fi
        fi
        if command -v lsof >/dev/null 2>&1; then
            if lsof -i ":${port}" >/dev/null 2>&1; then
                return 0
            fi
        fi
        if command -v netstat >/dev/null 2>&1; then
            if netstat -tuln 2>/dev/null | grep -qE ":(${port})[[:space:]]"; then
                return 0
            fi
        fi
        if command -v fuser >/dev/null 2>&1; then
            if fuser "${port}/tcp" >/dev/null 2>&1 || fuser "${port}/udp" >/dev/null 2>&1; then
                return 0
            fi
        fi
    fi
    return 1
}

# Helper: Identify process occupying a port on a specific IP or wildcard
get_port_process() {
    local port="$1"
    local ip="${2:-}"
    local proc=""

    if [ -n "$ip" ] && [ "$ip" != "0.0.0.0" ] && [ "$ip" != "::" ]; then
        local esc_ip
        esc_ip=$(echo "$ip" | sed 's/\./\\./g')
        local pattern="(0\.0\.0\.0|\[::\]|:::?|\*|${esc_ip}):${port}([[:space:]]|$)"
        if command -v ss >/dev/null 2>&1; then
            proc=$(ss -tulpn 2>/dev/null | grep -E "$pattern" | grep -oP 'users:\(\("\K[^"]+' | head -n1)
        fi
        if [ -z "$proc" ] && command -v lsof >/dev/null 2>&1; then
            proc=$(lsof -i "TCP@${ip}:${port}" -sTCP:LISTEN -t 2>/dev/null | head -n1 | xargs -r ps -o comm= 2>/dev/null)
            [ -z "$proc" ] && proc=$(lsof -i "TCP@0.0.0.0:${port}" -sTCP:LISTEN -t 2>/dev/null | head -n1 | xargs -r ps -o comm= 2>/dev/null)
        fi
    else
        if command -v ss >/dev/null 2>&1; then
            proc=$(ss -tulpn 2>/dev/null | grep -E ":(${port})[[:space:]]" | grep -oP 'users:\(\("\K[^"]+' | head -n1)
        fi
        if [ -z "$proc" ] && command -v lsof >/dev/null 2>&1; then
            proc=$(lsof -i ":${port}" -sTCP:LISTEN -t 2>/dev/null | head -n1 | xargs -r ps -o comm= 2>/dev/null)
        fi
        if [ -z "$proc" ] && command -v fuser >/dev/null 2>&1; then
            local pid
            pid=$(fuser "${port}/tcp" 2>/dev/null | tr -d ' ' | head -n1)
            [ -z "$pid" ] && pid=$(fuser "${port}/udp" 2>/dev/null | tr -d ' ' | head -n1)
            [ -n "$pid" ] && proc=$(ps -o comm= -p "$pid" 2>/dev/null)
        fi
    fi
    if [ -z "$proc" ]; then
        if [ "$port" -eq 80 ]; then
            if systemctl is-active --quiet nginx 2>/dev/null; then proc="nginx"; fi
            if systemctl is-active --quiet apache2 2>/dev/null; then proc="apache2"; fi
            if systemctl is-active --quiet httpd 2>/dev/null; then proc="httpd"; fi
            if systemctl is-active --quiet caddy 2>/dev/null; then proc="caddy"; fi
        fi
    fi
    echo "${proc:-unknown}"
}

# Helper: Detect if Port 80 is occupied (optionally on target IP)
is_port_80_busy() {
    is_port_busy 80 "${1:-$(get_public_ip)}"
}

# Helper: Find the process occupying Port 80 (optionally on target IP)
get_port_80_process() {
    get_port_process 80 "${1:-$(get_public_ip)}"
}

# Helper: Check if Port 53 is occupied (optionally on target IP)
is_port_53_busy() {
    is_port_busy 53 "${1:-$(get_public_ip)}"
}

# Helper: Identify process listening on Port 53 (optionally on target IP)
get_port_53_process() {
    get_port_process 53 "${1:-$(get_public_ip)}"
}

# Helper: Safely release Port 53 and preserve upstream DNS resolution
free_port_53() {
    local target_ip="${1:-$(get_public_ip)}"
    if [ -n "$target_ip" ] && [ "$target_ip" != "0.0.0.0" ]; then
        echo -e "  ${CYAN}Verifying and freeing DNS Port 53 for IP ${target_ip}...${NC}"
    else
        echo -e "  ${CYAN}Freeing DNS Port 53 (disabling conflicting stub listeners & services)...${NC}"
    fi

    # 1. Back up and replace /etc/resolv.conf so host resolution stays working
    if [ ! -f "${INSTALL_DIR}/.resolv-backup" ] && [ -d "${INSTALL_DIR}" ]; then
        if [ -L /etc/resolv.conf ]; then
            printf 'symlink\n%s\n' "$(readlink /etc/resolv.conf)" > "${INSTALL_DIR}/.resolv-backup" 2>/dev/null || true
        elif [ -f /etc/resolv.conf ]; then
            { printf 'file\n'; cat /etc/resolv.conf; } > "${INSTALL_DIR}/.resolv-backup" 2>/dev/null || true
        else
            printf 'absent\n' > "${INSTALL_DIR}/.resolv-backup" 2>/dev/null || true
        fi
        chmod 600 "${INSTALL_DIR}/.resolv-backup" 2>/dev/null || true
    fi

    rm -f /etc/resolv.conf 2>/dev/null || true
    cat <<EOF > /etc/resolv.conf
nameserver 1.1.1.1
nameserver 8.8.8.8
nameserver 9.9.9.9
options timeout:2 attempts:2
EOF
    chmod 644 /etc/resolv.conf 2>/dev/null || true

    # 2. Disable systemd-resolved stub listener if conflict exists on target_ip
    if is_port_53_busy "$target_ip"; then
        if [ -f /etc/systemd/resolved.conf ]; then
            sed -i -E 's/^#?DNSStubListener=.*/DNSStubListener=no/' /etc/systemd/resolved.conf 2>/dev/null || true
            sed -i -E 's/^#?DNS=.*/DNS=1.1.1.1 8.8.8.8/' /etc/systemd/resolved.conf 2>/dev/null || true
        fi
        mkdir -p /etc/systemd/resolved.conf.d/
        cat <<EOF > /etc/systemd/resolved.conf.d/JozmozDNS.conf
[Resolve]
DNS=1.1.1.1 8.8.8.8
DNSStubListener=no
EOF
        chmod 755 /etc/systemd/resolved.conf.d 2>/dev/null || true
        chmod 644 /etc/systemd/resolved.conf.d/JozmozDNS.conf 2>/dev/null || true

        if systemctl is-active --quiet systemd-resolved 2>/dev/null; then
            systemctl restart systemd-resolved 2>/dev/null || true
            sleep 1
        fi
    fi

    # 3. Stop traditional competing DNS services ONLY if Port 53 on target_ip is occupied
    if is_port_53_busy "$target_ip"; then
        for d_svc in bind9 named dnsmasq unbound pdns pdns-recursor knot-resolver; do
            if systemctl is-active --quiet "$d_svc" 2>/dev/null; then
                echo -e "    ${YELLOW}Stopping competing DNS service: ${d_svc}...${NC}"
                systemctl stop "$d_svc" 2>/dev/null || true
                systemctl disable "$d_svc" 2>/dev/null || true
            fi
        done
    fi

    # 4. If Port 53 is still held on target_ip, terminate the specific occupying process
    if is_port_53_busy "$target_ip"; then
        local p53_owner
        p53_owner=$(get_port_53_process "$target_ip")
        echo -e "    ${YELLOW}Port 53 on ${target_ip:-system} is still held by '${p53_owner}'. Terminating listener...${NC}"

        if [ "$p53_owner" = "systemd-resolve" ] || [ "$p53_owner" = "systemd-resolved" ]; then
            systemctl stop systemd-resolved 2>/dev/null || true
            systemctl disable systemd-resolved 2>/dev/null || true
        fi

        if command -v fuser >/dev/null 2>&1; then
            fuser -k 53/udp >/dev/null 2>&1 || true
            fuser -k 53/tcp >/dev/null 2>&1 || true
        fi
        if command -v lsof >/dev/null 2>&1; then
            lsof -i :53 -t 2>/dev/null | xargs -r kill -9 >/dev/null 2>&1 || true
        fi
        sleep 1
    fi

    # 5. Guarantee /etc/resolv.conf is still valid
    if ! grep -q "nameserver" /etc/resolv.conf 2>/dev/null; then
        cat <<EOF > /etc/resolv.conf
nameserver 1.1.1.1
nameserver 8.8.8.8
options timeout:2 attempts:2
EOF
    fi

    if is_port_53_busy "$target_ip"; then
        echo -e "    ${YELLOW}⚠️  Warning: Port 53 is still occupied by $(get_port_53_process "$target_ip").${NC}"
    else
        echo -e "    ${GREEN}✓ Port 53 successfully freed for ${target_ip:-all}!${NC}"
    fi
}

# Helper: Install verified certificate files into JozmozDNS directories
install_cert_files() {
    local cert_src="$1"
    local key_src="$2"
    local domain="$3"

    mkdir -p "${INSTALL_DIR}/certs/acme"
    cp -L "$cert_src" "${INSTALL_DIR}/certs/cert.pem" 2>/dev/null || true
    cp -L "$key_src" "${INSTALL_DIR}/certs/key.pem" 2>/dev/null || true
    if [ -n "$domain" ]; then
        cp -L "$cert_src" "${INSTALL_DIR}/certs/acme/${domain}.crt" 2>/dev/null || true
        cp -L "$key_src" "${INSTALL_DIR}/certs/acme/${domain}.key" 2>/dev/null || true
    fi
    chmod 600 "${INSTALL_DIR}/certs/"*.pem "${INSTALL_DIR}/certs/acme/"* 2>/dev/null || true
}

# Helper: Verify if a domain has a valid, CA-signed certificate (not self-signed)
is_valid_domain_cert() {
    local domain="$1"
    [ -z "$domain" ] && return 1

    local c_path="${INSTALL_DIR}/certs/acme/${domain}.crt"
    local k_path="${INSTALL_DIR}/certs/acme/${domain}.key"

    if [ ! -s "$c_path" ] || [ ! -s "$k_path" ]; then
        if [ -s "/etc/letsencrypt/live/${domain}/fullchain.pem" ] && [ -s "/etc/letsencrypt/live/${domain}/privkey.pem" ]; then
            install_cert_files "/etc/letsencrypt/live/${domain}/fullchain.pem" "/etc/letsencrypt/live/${domain}/privkey.pem" "$domain"
            c_path="${INSTALL_DIR}/certs/acme/${domain}.crt"
            k_path="${INSTALL_DIR}/certs/acme/${domain}.key"
        else
            c_path="${INSTALL_DIR}/certs/cert.pem"
            k_path="${INSTALL_DIR}/certs/key.pem"
        fi
    fi

    [ ! -s "$c_path" ] && return 1
    [ ! -s "$k_path" ] && return 1

    # Check validity dates (at least 1 day left)
    if ! openssl x509 -in "$c_path" -noout -checkend 86400 >/dev/null 2>&1; then
        return 1
    fi

    # Check hostname covers domain in SAN or CN
    if ! openssl x509 -in "$c_path" -noout -text 2>/dev/null | grep -E "DNS:${domain}([ ,]|$)|CN\s*=\s*${domain}([ ,]|$)" >/dev/null 2>&1; then
        return 1
    fi

    # Check NOT self-signed (issuer != subject)
    local subj iss
    subj=$(openssl x509 -in "$c_path" -noout -subject 2>/dev/null | sed -e 's/^[[:space:]]*//' -e 's/subject= *//')
    iss=$(openssl x509 -in "$c_path" -noout -issuer 2>/dev/null | sed -e 's/^[[:space:]]*//' -e 's/issuer= *//')
    if [ -n "$subj" ] && [ "$subj" = "$iss" ]; then
        return 1
    fi

    return 0
}

# Helper: Interactive Guided Wizard for manual SSL when Port 80 is occupied
manual_ssl_wizard() {
    local domain="$1"
    local email="$2"
    local public_ip="$3"
    local reason="${4:-Port 80 is occupied}"

    while true; do
        local occ_proc
        occ_proc=$(get_port_80_process)
        [ -z "$occ_proc" ] && occ_proc="unknown"

        echo ""
        echo -e "${RED}${BOLD}────────────────────────────────────────────────────────────────────────${NC}"
        echo -e "${RED}${BOLD}  ⚠️  اخطار: صدور خودکار SSL روی پورت ۸۰ امکان‌پذیر نیست!${NC}"
        echo -e "${YELLOW}      دلیل / Reason: ${reason}${NC}"
        if [ "$occ_proc" != "unknown" ]; then
            echo -e "${YELLOW}      پروسه اشغال‌کننده پورت ۸۰: ${CYAN}${BOLD}${occ_proc}${NC}"
        fi
        echo -e "${RED}${BOLD}────────────────────────────────────────────────────────────────────────${NC}"
        echo -e "  دامنه: ${CYAN}${domain}${NC}"
        echo -e "  پروتکل Let's Encrypt برای صدور خودکار (HTTP-01) به پورت 80 آزاد نیاز دارد."
        echo -e "  لطفاً یکی از روش‌های زیر را برای دریافت یا تنظیم دستی سرتیفیکیت انتخاب کنید:\n"
        echo -e "  ${CYAN}[1]${NC} ${GREEN}${BOLD}دریافت دستی با چالش DNS در Certbot (بدون نیاز به پورت ۸۰)${NC} ${YELLOW}★ توصیه می‌شود${NC}"
        echo -e "      Certbot DNS-01 Challenge (با افزودن یک رکورد TXT در پنل کلودفلر/DNS دامنه)"
        echo -e "  ${CYAN}[2]${NC} ${BOLD}توقف موقت پروسه اشغال‌کننده (${occ_proc}) و صدور خودکار SSL${NC}"
        echo -e "      Temporarily stop '${occ_proc}', issue certificate, and restart it"
        echo -e "  ${CYAN}[3]${NC} ${BOLD}معرفی مسیر فایل‌های سرتیفیکیت موجود روی سرور (.crt/.pem و .key)${NC}"
        echo -e "      Specify paths to existing certificate and private key files on disk"
        echo -e "  ${CYAN}[4]${NC} ${BOLD}پیست کردن مستقیم محتوای سرتیفیکیت و کلید در ترمینال${NC}"
        echo -e "      Paste Certificate & Private Key PEM text directly into terminal"
        echo -e "  ${CYAN}[5]${NC} ${YELLOW}${BOLD}انصراف از دامنه و استفاده از حالت IP مستقیم (Direct IP Mode)${NC}"
        echo -e "      Switch to Direct IP mode with dedicated SSL certificate"
        echo -e "  ${CYAN}[6]${NC} ${BOLD}بررسی مجدد وضعیت پورت ۸۰ (Retry)${NC}\n"

        local wiz_choice=""
        read -rp "  شماره گزینه مورد نظر را انتخاب کنید [1-6] (پیش‌فرض: 1): " wiz_choice
        wiz_choice=$(echo "$wiz_choice" | tr -d '[:space:]')
        [ -z "$wiz_choice" ] && wiz_choice="1"

        case "$wiz_choice" in
            1)
                echo -e "\n${CYAN}════════════════════════════════════════════════════════════════════════${NC}"
                echo -e "${CYAN}${BOLD}  دریافت سرتیفیکیت از طریق چالش DNS (Certbot DNS-01)${NC}"
                echo -e "${CYAN}════════════════════════════════════════════════════════════════════════${NC}"
                echo -e "  راهنمای گام به گام:"
                echo -e "  ۱. ابزار Certbot اجرا شده و یک نام و مقدار رکورد ${YELLOW}TXT${NC} تولید می‌کند."
                echo -e "  ۲. وارد پنل مدیریت دامنه خود (مثلاً Cloudflare یا Arvan) شوید."
                echo -e "  ۳. یک رکورد از نوع ${GREEN}TXT${NC} بسازید:"
                echo -e "     - Name / Host:  ${GREEN}_acme-challenge.${domain}${NC} (یا فقط _acme-challenge)"
                echo -e "     - Value / Text: (مقداری که Certbot در ادامه روی صفحه می‌نویسد)"
                echo -e "     - TTL:          Auto یا 1 minute"
                echo -e "  ۴. بعد از ذخیره رکورد در پنل، در همین ترمینال دکمه ${YELLOW}Enter${NC} را بزنید.\n"

                if ! command -v certbot >/dev/null 2>&1; then
                    echo -e "  ${YELLOW}در حال نصب ابزار Certbot...${NC}"
                    if command -v apt-get >/dev/null 2>&1; then
                        apt-get update -y -q || true
                        apt-get install -y -q certbot || true
                    elif command -v dnf >/dev/null 2>&1; then
                        dnf install -y -q epel-release 2>/dev/null || true
                        dnf install -y -q certbot || true
                    elif command -v yum >/dev/null 2>&1; then
                        yum install -y -q epel-release 2>/dev/null || true
                        yum install -y -q certbot || true
                    fi
                fi

                if ! command -v certbot >/dev/null 2>&1; then
                    echo -e "  ${RED}خطا: امکان نصب خودکار Certbot فراهم نشد. لطفاً گزینه دیگری را انتخاب کنید.${NC}"
                    continue
                fi

                local cb_mail="--register-unsafely-without-email"
                if [ -n "$email" ]; then
                    cb_mail="--email ${email} --no-eff-email"
                fi

                echo -e "  ${GREEN}در حال اجرای Certbot... لطفاً مراحل روی صفحه را دنبال کنید:${NC}\n"
                certbot certonly --manual --preferred-challenges dns -d "${domain}" ${cb_mail} --agree-tos || true

                local cb_live_cert="/etc/letsencrypt/live/${domain}/fullchain.pem"
                local cb_live_key="/etc/letsencrypt/live/${domain}/privkey.pem"
                if [ -f "$cb_live_cert" ] && [ -f "$cb_live_key" ]; then
                    install_cert_files "$cb_live_cert" "$cb_live_key" "$domain"
                    echo -e "\n  ${GREEN}✓ سرتیفیکیت Let's Encrypt با موفقیت صادر و نصب گردید!${NC}"
                    return 0
                else
                    echo -e "\n  ${RED}سرتیفیکیت صادر نشد یا فرآیند توسط کاربر لغو گردید.${NC}"
                fi
                ;;
            2)
                echo -e "\n${CYAN}════════════════════════════════════════════════════════════════════════${NC}"
                echo -e "${CYAN}${BOLD}  توقف موقت پروسه اشغال‌کننده (${occ_proc})${NC}"
                echo -e "${CYAN}════════════════════════════════════════════════════════════════════════${NC}"
                read -rp "  آیا اجازه می‌دهید پروسه '${occ_proc}' موقتاً متوقف شود؟ [y/N]: " stop_ok
                if [[ "$stop_ok" =~ ^[yY] ]]; then
                    systemctl stop "$occ_proc" 2>/dev/null || pkill -15 "$occ_proc" 2>/dev/null || true
                    sleep 1
                    if is_port_80_busy "$public_ip"; then
                        echo -e "  ${RED}پورت ۸۰ روی ${public_ip} هنوز اشغال است! توقف سرویس انجام نشد.${NC}"
                    else
                        echo -e "  ${GREEN}✓ پورت ۸۰ روی ${public_ip} آزاد شد.${NC}"
                        echo -e "  در حال صدور سرتیفیکیت Standalone..."

                        if command -v certbot >/dev/null 2>&1; then
                            local cb_mail="--register-unsafely-without-email"
                            [ -n "$email" ] && cb_mail="--email ${email} --no-eff-email"
                            certbot certonly --standalone --http-01-address "${public_ip}" -d "${domain}" ${cb_mail} --agree-tos --non-interactive 2>/dev/null || true
                        fi

                        local cb_live_cert="/etc/letsencrypt/live/${domain}/fullchain.pem"
                        local cb_live_key="/etc/letsencrypt/live/${domain}/privkey.pem"
                        if [ -f "$cb_live_cert" ] && [ -f "$cb_live_key" ]; then
                            install_cert_files "$cb_live_cert" "$cb_live_key" "$domain"
                            echo -e "  ${GREEN}✓ سرتیفیکیت با موفقیت دریافت و نصب شد!${NC}"
                        elif [ -f "${INSTALL_DIR}/JozmozDNS" ]; then
                            echo -e "  تلاش صدور از طریق موتور داخلی JozmozDNS..."
                            timeout 20 "${INSTALL_DIR}/JozmozDNS" -server -domain "${domain}" -email "${email}" -public-ip "${public_ip}" -bind "${public_ip}" >/dev/null 2>&1 || true
                            if [ -f "${INSTALL_DIR}/certs/acme/${domain}.crt" ]; then
                                install_cert_files "${INSTALL_DIR}/certs/acme/${domain}.crt" "${INSTALL_DIR}/certs/acme/${domain}.key" "$domain"
                                echo -e "  ${GREEN}✓ سرتیفیکیت با موفقیت صادر و ذخیره شد!${NC}"
                            fi
                        fi

                        echo -e "  در حال راه‌اندازی مجدد پروسه ${occ_proc}..."
                        systemctl start "$occ_proc" 2>/dev/null || true

                        if [ -f "${INSTALL_DIR}/certs/acme/${domain}.crt" ] || [ -f "${INSTALL_DIR}/certs/cert.pem" ]; then
                            return 0
                        else
                            echo -e "  ${RED}صدور خودکار ناموفق بود.${NC}"
                        fi
                    fi
                fi
                ;;
            3)
                echo -e "\n${CYAN}════════════════════════════════════════════════════════════════════════${NC}"
                echo -e "${CYAN}${BOLD}  معرفی مسیر فایل‌های سرتیفیکیت موجود روی سرور${NC}"
                echo -e "${CYAN}════════════════════════════════════════════════════════════════════════${NC}"
                read -rp "  مسیر فایل Certificate (.crt یا .pem یا fullchain.pem): " in_cert
                read -rp "  مسیر فایل Private Key (.key یا privkey.pem): " in_key
                in_cert=$(echo "$in_cert" | tr -d '[:space:]"'\''')
                in_key=$(echo "$in_key" | tr -d '[:space:]"'\''')

                if [ ! -f "$in_cert" ] || [ ! -f "$in_key" ]; then
                    echo -e "  ${RED}خطا: فایل‌های مشخص‌شده یافت نشدند!${NC}"
                else
                    install_cert_files "$in_cert" "$in_key" "$domain"
                    echo -e "  ${GREEN}✓ سرتیفیکیت و کلید اختصاصی با موفقیت اعمال شدند!${NC}"
                    return 0
                fi
                ;;
            4)
                echo -e "\n${CYAN}════════════════════════════════════════════════════════════════════════${NC}"
                echo -e "${CYAN}${BOLD}  پیست کردن مستقیم سرتیفیکیت و کلید در ترمینال${NC}"
                echo -e "${CYAN}════════════════════════════════════════════════════════════════════════${NC}"
                echo -e "  متن کامل Certificate خود را پیست کنید (از -----BEGIN CERTIFICATE----- تا -----END CERTIFICATE-----)."
                echo -e "  پس از پیست کردن، در خط جدید کلمه ${YELLOW}EOF${NC} را تایپ کرده و Enter بزنید:\n"
                local cert_content=""
                while IFS= read -r line; do
                    [ "$line" = "EOF" ] && break
                    cert_content="${cert_content}${line}"$'\n'
                done

                echo -e "\n  متن کامل Private Key خود را پیست کنید (از -----BEGIN PRIVATE KEY----- تا -----END PRIVATE KEY-----)."
                echo -e "  پس از پیست کردن، در خط جدید کلمه ${YELLOW}EOF${NC} را تایپ کرده و Enter بزنید:\n"
                local key_content=""
                while IFS= read -r line; do
                    [ "$line" = "EOF" ] && break
                    key_content="${key_content}${line}"$'\n'
                done

                if echo "$cert_content" | grep -q "BEGIN CERTIFICATE" && echo "$key_content" | grep -q "PRIVATE KEY"; then
                    local tmp_c="/tmp/pasted_cert_$$.crt"
                    local tmp_k="/tmp/pasted_key_$$.key"
                    echo "$cert_content" > "$tmp_c"
                    echo "$key_content" > "$tmp_k"
                    install_cert_files "$tmp_c" "$tmp_k" "$domain"
                    rm -f "$tmp_c" "$tmp_k"
                    echo -e "  ${GREEN}✓ سرتیفیکیت و کلید با موفقیت در JozmozDNS ذخیره شدند!${NC}"
                    return 0
                else
                    echo -e "  ${RED}خطا: محتوای پیست‌شده ساختار استاندارد Certificate یا Private Key را ندارد.${NC}"
                fi
                ;;
            5)
                echo -e "\n  ${YELLOW}حالت دامنه لغو شد و سرور به حالت IP مستقیم (${public_ip}) تغییر یافت.${NC}"
                if [ -f "${BIN_PATH}" ]; then
                    "${BIN_PATH}" -domain "none" -public-ip "${public_ip}" -bind "${public_ip}" >/dev/null 2>&1 || true
                fi
                return 0
                ;;
            6)
                echo -e "\n  در حال بررسی مجدد پورت ۸۰ روی ${public_ip}..."
                if ! is_port_80_busy "$public_ip"; then
                    echo -e "  ${GREEN}✓ پورت ۸۰ روی ${public_ip} آزاد است!${NC}"
                    return 0
                else
                    echo -e "  ${RED}پورت ۸۰ روی ${public_ip} هنوز اشغال است.${NC}"
                fi
                ;;
            *)
                echo -e "  ${RED}گزینه نامعتبر است.${NC}"
                ;;
        esac
    done
}

change_domain_ssl() {
    echo -e "\n${BOLD}${CYAN}=== Domain & SSL Certificate Management ===${NC}"
    echo -e "  ${CYAN}[1]${NC} Set / Change Domain with Let's Encrypt Auto SSL"
    echo -e "  ${CYAN}[2]${NC} Issue Manual SSL via Certbot DNS-01 Challenge (No Port 80 needed)"
    echo -e "  ${CYAN}[3]${NC} Install Custom SSL Certificate (.crt & .key files)"
    echo -e "  ${CYAN}[4]${NC} Paste Certificate & Key directly into Terminal"
    echo -e "  ${CYAN}[5]${NC} Switch / Revert to Direct IP Mode (Dedicated IP SSL)"
    echo -e "  ${CYAN}[6]${NC} Force Renew / Re-request Let's Encrypt Certificate"
    echo -e "  ${YELLOW}[0]${NC} Return to Main Menu"
    read -rp " Choose option [0-6]: " ssl_choice

    case "$ssl_choice" in
        1)
            local s_ip
            s_ip=$(get_public_ip)
            echo -e "\n${YELLOW}Ensure an 'A' record points from your domain to ${s_ip} before proceeding.${NC}\n"
            read -rp "Enter new domain name (e.g. dns.example.com): " new_domain
            new_domain=$(echo "$new_domain" | tr -d '[:space:]')
            if [ -z "$new_domain" ]; then
                echo -e "${RED}Domain cannot be empty.${NC}"
                sleep 2
                return
            fi
            read -rp "Enter Admin Email for Let's Encrypt [admin@${new_domain}]: " new_email
            new_email=$(echo "$new_email" | tr -d '[:space:]')
            [ -z "$new_email" ] && new_email="admin@${new_domain}"

            echo -e "\n${CYAN}Stopping service temporarily...${NC}"
            systemctl stop "$SERVICE_NAME" 2>/dev/null || true
            pkill -9 -f "$BIN_PATH" 2>/dev/null || true

            if ! is_valid_domain_cert "$new_domain" && is_port_80_busy "$s_ip"; then
                local occ_proc
                occ_proc=$(get_port_80_process "$s_ip")
                echo ""
                echo -e "${RED}${BOLD}────────────────────────────────────────────────────────────────────────${NC}"
                echo -e "${RED}${BOLD}  ⚠️  اخطار: پورت ۸۰ روی ${s_ip} اشغال است! / Warning: Port 80 on ${s_ip} is occupied!${NC}"
                echo -e "${YELLOW}      پروسه در حال استفاده / Occupying Process: ${CYAN}${BOLD}${occ_proc}${NC}"
                echo -e "${RED}${BOLD}────────────────────────────────────────────────────────────────────────${NC}"
                echo -e "  پروتکل Let's Encrypt برای صدور خودکار SSL به پورت ۸۰ آزاد روی ${s_ip} نیاز دارد."
                echo -e "  چون این پورت اشغال است، لطفاً روش دریافت یا تنظیم دستی سرتیفیکیت را انتخاب کنید.\n"

                manual_ssl_wizard "$new_domain" "$new_email" "$s_ip" "Port 80 on ${s_ip} is occupied by ${occ_proc}"
            else
                echo -e "${CYAN}Requesting SSL & updating domain to '${new_domain}'...${NC}"
                if [ -f "$BIN_PATH" ]; then
                    timeout 15 "$BIN_PATH" -server -domain "$new_domain" -email "$new_email" -public-ip "$s_ip" -bind "$s_ip" >/dev/null 2>&1 || true
                fi

                pkill -9 -f "$BIN_PATH" 2>/dev/null || true
                if ! is_valid_domain_cert "$new_domain"; then
                    echo -e "\n${YELLOW}⚠️  Automated issuance failed on port 80. Launching manual wizard...${NC}"
                    manual_ssl_wizard "$new_domain" "$new_email" "$s_ip" "Automated ACME verification failed on Port 80"
                fi
            fi

            pkill -9 -f "$BIN_PATH" 2>/dev/null || true
            free_port_53 "$s_ip"
            systemctl daemon-reload || true
            echo -e "${GREEN}Restarting JozmozDNS service...${NC}"
            systemctl restart "$SERVICE_NAME" 2>/dev/null || systemctl start "$SERVICE_NAME" 2>/dev/null || true
            sleep 2
            if ! systemctl is-active --quiet "$SERVICE_NAME"; then
                echo -e "\n${RED}⚠️  JozmozDNS service failed to start! Checking logs...${NC}"
                journalctl -u "$SERVICE_NAME" -n 25 --no-pager 2>/dev/null || true
                if is_port_53_busy "$s_ip"; then
                    echo -e "\n${YELLOW}Port 53 was occupied. Freeing and retrying...${NC}"
                    free_port_53 "$s_ip"
                    systemctl restart "$SERVICE_NAME" 2>/dev/null || true
                    sleep 2
                fi
            fi
            if systemctl is-active --quiet "$SERVICE_NAME"; then
                echo -e "${GREEN}✓ Domain and SSL configured!${NC}"
            fi
            sleep 2
            ;;
        2)
            local s_ip
            s_ip=$(get_public_ip)
            read -rp "Enter domain name for DNS-01 challenge (e.g. dns.example.com): " new_domain
            new_domain=$(echo "$new_domain" | tr -d '[:space:]')
            if [ -z "$new_domain" ]; then
                echo -e "${RED}Domain cannot be empty.${NC}"
                sleep 2
                return
            fi
            read -rp "Enter Admin Email [admin@${new_domain}]: " new_email
            new_email=$(echo "$new_email" | tr -d '[:space:]')
            [ -z "$new_email" ] && new_email="admin@${new_domain}"

            manual_ssl_wizard "$new_domain" "$new_email" "$s_ip" "Manual DNS-01 requested by user"

            if [ -f "$BIN_PATH" ]; then
                timeout 10 "$BIN_PATH" -server -domain "$new_domain" -email "$new_email" >/dev/null 2>&1 || true
            fi
            systemctl restart "$SERVICE_NAME" || systemctl start "$SERVICE_NAME"
            sleep 2
            ;;
        3)
            echo -e "\n${BOLD}Custom SSL Certificate Installation${NC}"
            read -rp "Enter full path to certificate file (.crt or .pem or fullchain.pem): " cert_path
            read -rp "Enter full path to private key file (.key or privkey.pem): " key_path
            cert_path=$(echo "$cert_path" | tr -d '[:space:]"'\''')
            key_path=$(echo "$key_path" | tr -d '[:space:]"'\''')
            if [ ! -f "$cert_path" ] || [ ! -f "$key_path" ]; then
                echo -e "${RED}Certificate or Key file not found!${NC}"
                sleep 2
                return
            fi
            read -rp "Enter domain name this certificate belongs to: " dom_name
            dom_name=$(echo "$dom_name" | tr -d '[:space:]')
            install_cert_files "$cert_path" "$key_path" "$dom_name"
            if [ -n "$dom_name" ] && [ -f "$BIN_PATH" ]; then
                timeout 10 "$BIN_PATH" -server -domain "$dom_name" >/dev/null 2>&1 || true
            fi
            echo -e "${GREEN}✓ Certificates saved and applied!${NC}"
            echo -e "${CYAN}Restarting JozmozDNS...${NC}"
            systemctl restart "$SERVICE_NAME"
            sleep 2
            ;;
        4)
            echo -e "\n${BOLD}Paste Certificate & Key directly into Terminal${NC}"
            read -rp "Enter domain name this certificate belongs to: " dom_name
            dom_name=$(echo "$dom_name" | tr -d '[:space:]')
            echo -e "Paste Certificate PEM text, then type EOF and press Enter:"
            local cert_buf=""
            while IFS= read -r line; do
                [ "$line" = "EOF" ] && break
                cert_buf="${cert_buf}${line}"$'\n'
            done
            echo -e "Paste Private Key PEM text, then type EOF and press Enter:"
            local key_buf=""
            while IFS= read -r line; do
                [ "$line" = "EOF" ] && break
                key_buf="${key_buf}${line}"$'\n'
            done
            if echo "$cert_buf" | grep -q "BEGIN CERTIFICATE" && echo "$key_buf" | grep -q "PRIVATE KEY"; then
                local tmp_c="/tmp/menu_pasted_$$.crt"
                local tmp_k="/tmp/menu_pasted_$$.key"
                echo "$cert_buf" > "$tmp_c"
                echo "$key_buf" > "$tmp_k"
                install_cert_files "$tmp_c" "$tmp_k" "$dom_name"
                rm -f "$tmp_c" "$tmp_k"
                if [ -n "$dom_name" ] && [ -f "$BIN_PATH" ]; then
                    timeout 10 "$BIN_PATH" -server -domain "$dom_name" >/dev/null 2>&1 || true
                fi
                echo -e "${GREEN}✓ Certificate and Key installed successfully!${NC}"
                systemctl restart "$SERVICE_NAME"
            else
                echo -e "${RED}Invalid certificate or key content.${NC}"
            fi
            sleep 2
            ;;
        5)
            echo -e "\n${YELLOW}Reverting to Direct IP Mode (Dedicated IP SSL)...${NC}"
            local s_ip
            s_ip=$(get_public_ip)
            if [ -f "$BIN_PATH" ]; then
                "$BIN_PATH" -domain "none" -public-ip "$s_ip" >/dev/null 2>&1 || true
            fi
            local OPENSSL_CNF="/tmp/openssl_JozmozDNS_$$.cnf"
            cat <<EOF > "$OPENSSL_CNF"
[req]
default_bits = 2048
prompt = no
default_md = sha256
distinguished_name = dn
x509_extensions = v3_req

[dn]
C = US
O = JozmozDNS
CN = ${s_ip}

[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names

[alt_names]
IP.1 = ${s_ip}
IP.2 = 127.0.0.1
DNS.1 = localhost
EOF
            openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
                -keyout "$INSTALL_DIR/certs/key.pem" \
                -out "$INSTALL_DIR/certs/cert.pem" \
                -config "$OPENSSL_CNF" 2>/dev/null || true
            rm -f "$OPENSSL_CNF"
            chmod 600 "$INSTALL_DIR/certs/key.pem" "$INSTALL_DIR/certs/cert.pem" 2>/dev/null || true
            systemctl restart "$SERVICE_NAME"
            echo -e "${GREEN}✓ Switched to Direct IP Mode (${s_ip}) with dedicated IP SSL!${NC}"
            sleep 2
            ;;
        6)
            echo -e "\n${CYAN}Clearing cached certificates in $INSTALL_DIR/certs/acme/...${NC}"
            systemctl stop "$SERVICE_NAME" 2>/dev/null || true
            rm -rf "$INSTALL_DIR/certs/acme"/* 2>/dev/null || true
            echo -e "${GREEN}Restarting JozmozDNS to re-request certificate...${NC}"
            systemctl restart "$SERVICE_NAME" || systemctl start "$SERVICE_NAME"
            echo -e "${GREEN}✓ Certificate cache flushed and renewal triggered!${NC}"
            sleep 2
            ;;
        *)
            return
            ;;
    esac
}

update_JozmozDNS() {
    echo -e "\n${BOLD}${CYAN}=== Update JozmozDNS from GitHub ===${NC}"
    echo -e "Checking for latest release or building from repository..."
    
    local cache_buster
    cache_buster=$(date +%s)
    local tmp_file="/tmp/JozmozDNS_update_$$"
    local updated=false

    if curl -fsSL -o "$tmp_file" "https://github.com/jozmoz/HyperDNS/releases/latest/download/jozmozdns-linux" 2>/dev/null || \
       curl -fsSL -o "$tmp_file" "${REPO_RAW_URL}/jozmozdns-linux?t=${cache_buster}" 2>/dev/null || \
       curl -fsSL -o "$tmp_file" "${REPO_RAW_URL}/JozmozDNS-linux?t=${cache_buster}" 2>/dev/null; then
        chmod +x "$tmp_file"
        updated=true
    else
        echo -e "${YELLOW}Pre-built binary not found. Compiling latest version from GitHub source...${NC}"
        if ! command -v go >/dev/null 2>&1; then
            echo -e "${CYAN}Installing Go toolchain...${NC}"
            if command -v apt-get >/dev/null 2>&1; then
                apt-get update -y -q && apt-get install -y -q golang git
            elif command -v dnf >/dev/null 2>&1; then
                dnf install -y -q golang git
            elif command -v yum >/dev/null 2>&1; then
                yum install -y -q golang git
            fi
        fi
        local tmp_build="/tmp/jozmoz_build_$$"
        rm -rf "$tmp_build"
        if git clone --depth 1 https://github.com/jozmoz/HyperDNS.git "$tmp_build" 2>/dev/null; then
            echo -e "${CYAN}Compiling binary with Go...${NC}"
            if (cd "$tmp_build" && go build -trimpath -ldflags="-s -w" -o "$tmp_file" ./cmd/jozmozdns); then
                chmod +x "$tmp_file"
                updated=true
            fi
            rm -rf "$tmp_build"
        fi
    fi

    if [ "$updated" = true ] && [ -s "$tmp_file" ]; then
        echo -e "${CYAN}Stopping JozmozDNS service...${NC}"
        systemctl stop "$SERVICE_NAME" 2>/dev/null || true
        pkill -9 -f "${INSTALL_DIR}/jozmozdns" 2>/dev/null || true
        pkill -9 -f "${INSTALL_DIR}/JozmozDNS" 2>/dev/null || true

        echo -e "${CYAN}Replacing binary at ${BIN_PATH}...${NC}"
        cp "$tmp_file" "$BIN_PATH"
        chmod +x "$BIN_PATH"
        rm -f "$tmp_file"

        if [ "$BIN_PATH" != "/opt/JozmozDNS/jozmozdns" ] && [ -d "/opt/JozmozDNS" ]; then
            cp "$BIN_PATH" "/opt/JozmozDNS/jozmozdns" 2>/dev/null || true
            chmod +x "/opt/JozmozDNS/jozmozdns" 2>/dev/null || true
        fi
        ln -sf "$BIN_PATH" "/opt/JozmozDNS/JozmozDNS" 2>/dev/null || true

        # Also update the menu script itself
        echo -e "${CYAN}Updating management menu script...${NC}"
        curl -fsSL -o /usr/local/bin/jozmozdns "${REPO_RAW_URL}/scripts/jozmozdns-menu.sh?t=${cache_buster}" 2>/dev/null || \
        curl -fsSL -o /usr/local/bin/jozmozdns "${REPO_RAW_URL}/scripts/JozmozDNS-menu.sh?t=${cache_buster}" 2>/dev/null || true
        chmod +x /usr/local/bin/jozmozdns 2>/dev/null || true
        ln -sf /usr/local/bin/jozmozdns /usr/local/bin/jdns 2>/dev/null || true
        ln -sf /usr/local/bin/jozmozdns /usr/local/bin/jmdns 2>/dev/null || true
        ln -sf /usr/local/bin/jozmozdns /usr/local/bin/JozmozDNS 2>/dev/null || true

        echo -e "${CYAN}Restarting JozmozDNS service...${NC}"
        free_port_53
        systemctl restart "$SERVICE_NAME" 2>/dev/null || systemctl start "$SERVICE_NAME" 2>/dev/null || true
        echo -e "\n${GREEN}${BOLD}✓ JozmozDNS and Management Menu updated to the latest version successfully!${NC}"
    else
        echo -e "\n${RED}Failed to build or update binary from GitHub.${NC}"
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
    echo -e "\n${BOLD}${CYAN}=== JozmozDNS Diagnostic Port Check ===${NC}"
    local cur_ip
    cur_ip=$(get_public_ip)
    echo "Testing DNS, DoT, DoH and SNI Proxy ports on ${cur_ip}..."
    for port in 53 853 8443 80 443; do
        if is_port_busy "$port" "$cur_ip"; then
            echo -e "  Port ${port}: ${GREEN}OPEN & LISTENING${NC}"
        else
            echo -e "  Port ${port}: ${RED}CLOSED / NOT LISTENING${NC}"
        fi
    done

    if is_port_53_busy "$cur_ip"; then
        local p53_p
        p53_p=$(get_port_53_process "$cur_ip")
        if [ "$p53_p" != "JozmozDNS" ] && [ "$p53_p" != "unknown" ]; then
            echo -e "\n  ${YELLOW}⚠️  Port 53 on ${cur_ip} is occupied by external process: ${BOLD}${p53_p}${NC}"
            read -rp "  Do you want to free Port 53 now? [y/N]: " fix_p53
            if [[ "$fix_p53" =~ ^[yY] ]]; then
                free_port_53 "$cur_ip"
                systemctl restart "$SERVICE_NAME" 2>/dev/null || true
            fi
        fi
    fi
    echo ""
    read -rp "Press Enter to continue..."
}

get_db_path() {
    if [ -f "$INSTALL_DIR/data.db" ]; then
        echo "$INSTALL_DIR/data.db"
    elif [ -f "/root/JozmozDNS-main/data.db" ]; then
        echo "/root/JozmozDNS-main/data.db"
    elif [ -f "./data.db" ]; then
        echo "./data.db"
    else
        echo "$INSTALL_DIR/data.db"
    fi
}

backup_database() {
    echo -e "\n${BOLD}${CYAN}=== JozmozDNS Database Backup ===${NC}"
    local db_path
    db_path=$(get_db_path)
    if [ ! -f "$db_path" ]; then
        echo -e "${RED}Error: Database file not found at $db_path${NC}"
        read -rp "Press Enter to continue..."
        return
    fi

    local backup_dir="$INSTALL_DIR/backups"
    mkdir -p "$backup_dir"
    local timestamp
    timestamp=$(date +"%Y%m%d_%H%M%S")
    local backup_file="$backup_dir/JozmozDNS_backup_${timestamp}.db"

    echo -e "${CYAN}Creating database snapshot from ${db_path}...${NC}"
    cp "$db_path" "$backup_file"
    chmod 600 "$backup_file"

    echo -e "${GREEN}✓ Database backup created successfully:${NC}"
    echo -e "  File: ${YELLOW}$backup_file${NC}"
    echo -e "  Size: $(du -h "$backup_file" | cut -f1)"
    echo ""
    read -rp "Press Enter to continue..."
}

restore_database() {
    echo -e "\n${BOLD}${CYAN}=== JozmozDNS Database Restore ===${NC}"
    local backup_dir="$INSTALL_DIR/backups"
    local db_path
    db_path=$(get_db_path)

    echo -e "Available local backups in ${YELLOW}$backup_dir${NC}:"
    local backups=()
    if [ -d "$backup_dir" ]; then
        while IFS= read -r f; do
            [ -n "$f" ] && backups+=("$f")
        done < <(ls -1t "$backup_dir"/*.db 2>/dev/null || true)
    fi

    local selected_backup=""
    if [ ${#backups[@]} -eq 0 ]; then
        echo -e "${YELLOW}No automated backups found in $backup_dir.${NC}"
        read -rp "Enter full path to backup .db file (or press Enter to cancel): " custom_path
        if [ -z "$custom_path" ] || [ ! -f "$custom_path" ]; then
            echo -e "${RED}File not found or cancelled.${NC}"
            sleep 1.5
            return
        fi
        selected_backup="$custom_path"
    else
        local i=1
        for b in "${backups[@]}"; do
            local bsize
            bsize=$(du -h "$b" | cut -f1)
            local bdate
            bdate=$(date -r "$b" "+%Y-%m-%d %H:%M:%S" 2>/dev/null || stat -c "%y" "$b" 2>/dev/null || echo "")
            echo -e "  ${CYAN}[$i]${NC} $(basename "$b") ($bsize) - $bdate"
            ((i++))
        done
        echo -e "  ${CYAN}[c]${NC} Custom file path"
        echo -e "  ${YELLOW}[0]${NC} Cancel"
        read -rp "Select backup to restore [1-${#backups[@]}]: " bchoice
        if [ "$bchoice" = "0" ] || [ -z "$bchoice" ]; then
            return
        elif [ "$bchoice" = "c" ] || [ "$bchoice" = "C" ]; then
            read -rp "Enter full path to backup .db file: " custom_path
            if [ ! -f "$custom_path" ]; then
                echo -e "${RED}File not found!${NC}"
                sleep 1.5
                return
            fi
            selected_backup="$custom_path"
        elif [[ "$bchoice" =~ ^[0-9]+$ ]] && [ "$bchoice" -ge 1 ] && [ "$bchoice" -le "${#backups[@]}" ]; then
            selected_backup="${backups[$((bchoice-1))]}"
        else
            echo -e "${RED}Invalid selection.${NC}"
            sleep 1.5
            return
        fi
    fi

    echo -e "\n${RED}${BOLD}⚠️  WARNING: Restoring will overwrite the active database!${NC}"
    echo -e "Target Database: ${CYAN}$db_path${NC}"
    echo -e "Backup Source:   ${YELLOW}$selected_backup${NC}"
    read -rp "Are you sure you want to proceed? (y/N): " confirm_restore
    if [[ ! "$confirm_restore" =~ ^[Yy]$ ]]; then
        echo -e "${YELLOW}Restore cancelled.${NC}"
        sleep 1.5
        return
    fi

    echo -e "${CYAN}Stopping JozmozDNS service...${NC}"
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true

    if [ -f "$db_path" ]; then
        cp "$db_path" "${db_path}.before_restore"
    fi

    echo -e "${CYAN}Restoring database file...${NC}"
    cp "$selected_backup" "$db_path"
    chmod 600 "$db_path"

    echo -e "${CYAN}Starting JozmozDNS service...${NC}"
    systemctl restart "$SERVICE_NAME" || systemctl start "$SERVICE_NAME"

    echo -e "\n${GREEN}${BOLD}✓ Database successfully restored from $(basename "$selected_backup")!${NC}"
    read -rp "Press Enter to continue..."
}

uninstall_JozmozDNS() {
    echo -e "\n${RED}${BOLD}======================================================${NC}"
    echo -e "${RED}${BOLD}           ⚠️  UNINSTALL JozmozDNS  ⚠️                ${NC}"
    echo -e "${RED}${BOLD}======================================================${NC}"
    echo -e "This will stop the service, remove binaries and restore system settings."
    read -rp "Are you SURE you want to completely uninstall? (y/N): " confirm
    if [[ "$confirm" =~ ^[Yy]$ ]]; then
        systemctl stop "$SERVICE_NAME" 2>/dev/null || true
        systemctl disable "$SERVICE_NAME" 2>/dev/null || true
        rm -f /etc/systemd/system/JozmozDNS.service
        systemctl daemon-reload

        rm -f /usr/local/bin/JozmozDNS /usr/local/bin/jmdns

        read -rp "Do you also want to delete all database and settings in /opt/JozmozDNS? (y/N): " del_data
        if [[ "$del_data" =~ ^[Yy]$ ]]; then
            rm -rf /opt/JozmozDNS
            echo -e "${RED}✓ Data directory /opt/JozmozDNS removed.${NC}"
        else
            echo -e "${YELLOW}Data preserved in /opt/JozmozDNS for backup.${NC}"
        fi

        # Restore systemd-resolved if it was modified
        if [ -f /etc/systemd/resolved.conf.d/JozmozDNS.conf ]; then
            rm -f /etc/systemd/resolved.conf.d/JozmozDNS.conf
            systemctl restart systemd-resolved 2>/dev/null || true
        fi
        systemctl enable systemd-resolved 2>/dev/null || true
        systemctl start systemd-resolved 2>/dev/null || true

        echo -e "\n${GREEN}JozmozDNS has been uninstalled successfully.${NC}"
        exit 0
    else
        echo -e "${YELLOW}Uninstall cancelled.${NC}"
        sleep 1.5
    fi
}

manage_telegram_bot() {
    echo -e "\n${BOLD}${CYAN}=== JozmozDNS Telegram Bot Management ===${NC}"
    local bot_service="JozmozDNS-bot"
    local bot_script="$INSTALL_DIR/integrations/telegram_bot.py"
    local bot_env="$INSTALL_DIR/telegram.env"
    local bot_unit="/etc/systemd/system/${bot_service}.service"

    if systemctl is-active --quiet "$bot_service" 2>/dev/null; then
        echo -e "  Bot Status: ${GREEN}● RUNNING${NC}"
    else
        echo -e "  Bot Status: ${RED}● STOPPED${NC}"
    fi

    echo ""
    echo -e "  ${CYAN}[1]${NC} 🚀 نصب و راه‌اندازی ربات (Install & Start Bot)"
    echo -e "  ${CYAN}[2]${NC} ▶️  روشن کردن ربات (Start Bot)"
    echo -e "  ${CYAN}[3]${NC} ⏹️  توقف ربات (Stop Bot)"
    echo -e "  ${CYAN}[4]${NC} 🔄 ری‌استارت ربات (Restart Bot)"
    echo -e "  ${CYAN}[5]${NC} 📜 مشاهده لاگ ربات (View Bot Logs)"
    echo -e "  ${CYAN}[6]${NC} 🔑 ویرایش API Key ربات (Edit Bot API Key)"
    echo -e "  ${YELLOW}[0]${NC} بازگشت (Back)"
    read -rp " Choose option [0-6]: " bot_choice

    case "$bot_choice" in
        1)
            echo -e "\n${CYAN}Installing Telegram Bot dependencies...${NC}"
            # Install Python3 and requests if not present
            if ! command -v python3 &>/dev/null; then
                apt-get update -qq && apt-get install -y -qq python3 python3-pip 2>/dev/null || {
                    yum install -y python3 python3-pip 2>/dev/null || true
                }
            fi
            python3 -c "import requests" 2>/dev/null || pip3 install requests 2>/dev/null || python3 -m pip install requests 2>/dev/null

            # Copy bot files
            mkdir -p "$INSTALL_DIR/integrations"
            if [ -f "$(dirname "$BIN_PATH")/integrations/telegram_bot.py" ]; then
                cp "$(dirname "$BIN_PATH")/integrations/telegram_bot.py" "$INSTALL_DIR/integrations/telegram_bot.py"
            elif [ -f "/root/JozmozDNS-main/integrations/telegram_bot.py" ]; then
                cp /root/JozmozDNS-main/integrations/telegram_bot.py "$INSTALL_DIR/integrations/telegram_bot.py"
            else
                echo -e "${CYAN}Downloading bot script from GitHub...${NC}"
                curl -fsSL -o "$INSTALL_DIR/integrations/telegram_bot.py" "${REPO_RAW_URL}/integrations/telegram_bot.py"
            fi

            # Create env file if not exists
            if [ ! -f "$bot_env" ]; then
                echo -e "\n${YELLOW}Please enter your JozmozDNS REST API Key:${NC}"
                echo -e "(You can find it in the Dashboard → Settings → API Key)"
                read -rp "API Key: " input_api_key
                cat > "$bot_env" <<ENVEOF
# JozmozDNS Telegram Bot Environment
# The bot reads all other settings (Token, Admin IDs, etc.) from the panel database.
JozmozDNS_API_BASE=http://127.0.0.1:8080/api/v2
JozmozDNS_API_KEY=${input_api_key}
ENVEOF
                chmod 600 "$bot_env"
                echo -e "${GREEN}✓ Environment file created at $bot_env${NC}"
            fi

            # Install systemd service
            cat > "$bot_unit" <<'SVCEOF'
[Unit]
Description=JozmozDNS Telegram Sales & Subscription Bot
Documentation=https://github.com/jozmoz/JozmozDNS
After=JozmozDNS.service
Wants=JozmozDNS.service

[Service]
Type=simple
User=root
WorkingDirectory=/opt/JozmozDNS
EnvironmentFile=-/opt/JozmozDNS/telegram.env
ExecStart=/usr/bin/python3 /opt/JozmozDNS/integrations/telegram_bot.py
Restart=always
RestartSec=10s
TimeoutStopSec=10s
SyslogIdentifier=JozmozDNS-bot

[Install]
WantedBy=multi-user.target
SVCEOF

            systemctl daemon-reload
            systemctl enable "$bot_service"
            systemctl restart "$bot_service"

            echo -e "\n${GREEN}${BOLD}✓ Telegram Bot installed and started successfully!${NC}"
            echo -e "${YELLOW}Note: Make sure you have configured the Telegram bot settings in the JozmozDNS Dashboard → Settings → Telegram Bot.${NC}"
            ;;
        2)
            systemctl start "$bot_service" 2>/dev/null && echo -e "${GREEN}✓ Bot started!${NC}" || echo -e "${RED}Failed to start bot. Run option [1] first to install.${NC}"
            ;;
        3)
            systemctl stop "$bot_service" 2>/dev/null && echo -e "${YELLOW}✓ Bot stopped.${NC}" || echo -e "${RED}Bot service not found.${NC}"
            ;;
        4)
            systemctl restart "$bot_service" 2>/dev/null && echo -e "${GREEN}✓ Bot restarted!${NC}" || echo -e "${RED}Failed. Run option [1] first.${NC}"
            ;;
        5)
            echo -e "\n${CYAN}Bot logs (Press Ctrl+C to return)...${NC}"
            sleep 1
            journalctl -u "$bot_service" -f -n 50 || true
            ;;
        6)
            echo -e "\n${YELLOW}Enter the new JozmozDNS API Key:${NC}"
            read -rp "API Key: " new_key
            if [ -n "$new_key" ]; then
                cat > "$bot_env" <<ENVEOF
JozmozDNS_API_BASE=http://127.0.0.1:8080/api/v2
JozmozDNS_API_KEY=${new_key}
ENVEOF
                chmod 600 "$bot_env"
                systemctl restart "$bot_service" 2>/dev/null || true
                echo -e "${GREEN}✓ API Key updated and bot restarted!${NC}"
            fi
            ;;
        *)
            return
            ;;
    esac
    sleep 2
}

change_dns_server_ip() {
    echo -e "\n${BOLD}${CYAN}=== JozmozDNS Server IP Management ===${NC}"
    local current_ip
    current_ip=$(get_public_ip)
    echo -e "  Current Configured IP: ${YELLOW}${current_ip}${NC}\n"

    # Collect all available server IPs
    local detected_ips=()
    local ip_labels=()

    # 1. External Public WAN IPv4
    local wan_ip
    wan_ip=$(curl -4 -s --connect-timeout 3 https://api.ipify.org 2>/dev/null || \
             curl -4 -s --connect-timeout 3 https://ifconfig.me 2>/dev/null || true)
    wan_ip=$(echo "$wan_ip" | tr -d '[:space:]')
    if [[ "$wan_ip" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]] && [ "$wan_ip" != "127.0.0.1" ]; then
        detected_ips+=("$wan_ip")
        ip_labels+=("Public WAN")
    fi

    # 2. Local interface IPs via 'ip -4 -o addr show'
    if command -v ip >/dev/null 2>&1; then
        while IFS= read -r line; do
            local ip_cand if_cand
            ip_cand=$(echo "$line" | awk '{print $4}' | cut -d/ -f1)
            if_cand=$(echo "$line" | awk '{print $2}')
            if [ -z "$ip_cand" ] || [[ "$ip_cand" =~ ^127\. ]]; then
                continue
            fi
            local is_dup=false
            for cur in "${detected_ips[@]}"; do
                if [ "$cur" = "$ip_cand" ]; then
                    is_dup=true
                    break
                fi
            done
            if [ "$is_dup" = false ] && [[ "$ip_cand" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
                detected_ips+=("$ip_cand")
                ip_labels+=("Interface: $if_cand")
            fi
        done < <(ip -4 -o addr show scope global 2>/dev/null || true)
    fi

    # 3. Fallback hostname -I
    if [ ${#detected_ips[@]} -eq 0 ] && command -v hostname >/dev/null 2>&1; then
        for hip in $(hostname -I 2>/dev/null || true); do
            if [[ "$hip" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]] && [ "$hip" != "127.0.0.1" ]; then
                detected_ips+=("$hip")
                ip_labels+=("Local IP")
            fi
        done
    fi

    echo -e "  ${BOLD}Detected Server IP Addresses:${NC}"
    for idx in "${!detected_ips[@]}"; do
        local num=$((idx + 1))
        local ip_val="${detected_ips[$idx]}"
        local ip_lbl="${ip_labels[$idx]}"
        if [ "$ip_val" = "$current_ip" ]; then
            echo -e "    ${CYAN}[$num]${NC} ${GREEN}${BOLD}${ip_val}${NC} (${ip_lbl} - ${YELLOW}Current${NC})"
        else
            echo -e "    ${CYAN}[$num]${NC} ${GREEN}${BOLD}${ip_val}${NC} (${ip_lbl})"
        fi
    done
    echo -e "    ${CYAN}[c]${NC} ${YELLOW}Enter custom IP manually (ورود دستی)${NC}"
    echo -e "    ${YELLOW}[0]${NC} Cancel (انصراف)\n"

    read -rp "  Select option or enter IP [Current: ${current_ip}]: " user_sel
    user_sel=$(echo "$user_sel" | tr -d '[:space:]"'\''')

    local new_ip=""
    if [ -z "$user_sel" ]; then
        echo -e "${YELLOW}No changes made.${NC}"
        sleep 1.5
        return
    elif [ "$user_sel" = "0" ]; then
        return
    elif [[ "$user_sel" =~ ^[0-9]+$ ]] && [ "$user_sel" -ge 1 ] && [ "$user_sel" -le "${#detected_ips[@]}" ]; then
        new_ip="${detected_ips[$((user_sel - 1))]}"
    elif [ "$user_sel" = "c" ] || [ "$user_sel" = "C" ]; then
        read -rp "  Enter custom DNS Server IP: " manual_ip
        manual_ip=$(echo "$manual_ip" | tr -d '[:space:]"'\''')
        new_ip="$manual_ip"
    elif [[ "$user_sel" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
        new_ip="$user_sel"
    else
        echo -e "${RED}Invalid input.${NC}"
        sleep 1.5
        return
    fi

    if [ -z "$new_ip" ] || [ "$new_ip" = "$current_ip" ]; then
        echo -e "${YELLOW}IP unchanged.${NC}"
        sleep 1.5
        return
    fi

    echo -e "\n${CYAN}Updating systemd service configuration to ${new_ip}...${NC}"
    local svc_file="/etc/systemd/system/${SERVICE_NAME}.service"
    if [ -f "$svc_file" ]; then
        if grep -q -- "-public-ip" "$svc_file"; then
            sed -i -E "s/-public-ip\s+[^ ]+(\s+-bind\s+[^ ]+)?/-public-ip ${new_ip} -bind ${new_ip}/" "$svc_file"
        else
            sed -i -E "s/(JozmozDNS\s+-server)/\1 -public-ip ${new_ip} -bind ${new_ip}/" "$svc_file"
        fi
        systemctl daemon-reload
    fi

    # Ensure kernel nonlocal bind is enabled
    sysctl -w net.ipv4.ip_nonlocal_bind=1 >/dev/null 2>&1 || true

    echo -e "${CYAN}Applying new IP to database...${NC}"
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    if [ -f "$BIN_PATH" ]; then
        timeout 5 "$BIN_PATH" -server -public-ip "$new_ip" -bind "$new_ip" >/dev/null 2>&1 || true
    fi

    echo -e "${GREEN}Restarting JozmozDNS service...${NC}"
    free_port_53 "$new_ip"
    systemctl restart "$SERVICE_NAME" || systemctl start "$SERVICE_NAME"
    echo -e "\n${GREEN}${BOLD}✓ DNS Server IP updated to ${new_ip}!${NC}"
    sleep 2
}

change_admin_password() {
    echo -e "\n${BOLD}${CYAN}=== JozmozDNS Admin Password Management ===${NC}"
    echo -e "You can set a new custom password for the Web Dashboard.\n"
    read -rp "Enter new Admin Password (or press Enter to generate random): " new_pass
    new_pass=$(echo "$new_pass" | tr -d '[:space:]')
    if [ -z "$new_pass" ]; then
        new_pass=$(openssl rand -hex 12 2>/dev/null || date +%s%N | sha256sum | head -c 24)
        echo -e "Generated random password: ${YELLOW}${new_pass}${NC}"
    fi

    echo -e "\n${CYAN}Updating admin password in database...${NC}"
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    if [ -f "$BIN_PATH" ]; then
        timeout 5 "$BIN_PATH" -password "$new_pass" < /dev/null >/dev/null 2>&1 || true
    fi
    systemctl restart "$SERVICE_NAME" || systemctl start "$SERVICE_NAME"

    # Update credentials.txt
    if [ -f "$INSTALL_DIR/credentials.txt" ]; then
        if grep -q "^password\s*:" "$INSTALL_DIR/credentials.txt"; then
            sed -i -E "s/^password\s*:.*/password: ${new_pass}/" "$INSTALL_DIR/credentials.txt"
        else
            echo "password: ${new_pass}" >> "$INSTALL_DIR/credentials.txt"
        fi
    else
        cat <<CREDEOF > "$INSTALL_DIR/credentials.txt"
username: admin
password: ${new_pass}
CREDEOF
        chmod 600 "$INSTALL_DIR/credentials.txt"
    fi

    echo -e "\n${GREEN}${BOLD}✓ Admin password successfully updated to: ${YELLOW}${new_pass}${NC}\n"
    read -rp "Press Enter to continue..."
}

main_menu() {
    while true; do
        show_banner
        echo -e "  ${CYAN}[1]${NC}  📊 وضعیت سرویس (Service Status)"
        echo -e "  ${CYAN}[2]${NC}  🔄 راه‌اندازی مجدد سرویس (Restart JozmozDNS)"
        echo -e "  ${CYAN}[3]${NC}  ⏹️  توقف موقت سرویس (Stop Service)"
        echo -e "  ${CYAN}[4]${NC}  ▶️  روشن کردن سرویس (Start Service)"
        echo -e "  ${CYAN}[5]${NC}  📜 مشاهده لاگ‌های زنده سرور (View Live Logs)"
        echo -e "  ${CYAN}[6]${NC}  🔑 لینک داشبورد و مشخصات ورود (Dashboard URL & Credentials)"
        echo -e "  ${CYAN}[7]${NC}  🌐 تغییر دامنه و صدور گواهینامه SSL / Let's Encrypt"
        echo -e "  ${CYAN}[8]${NC}  🧹 پاک‌سازی کش DNS (Flush DNS Cache)"
        echo -e "  ${CYAN}[9]${NC}  🩺 بررسی پورت‌ها و عیب‌یابی (Diagnostics & Port Check)"
        echo -e "  ${CYAN}[10]${NC} 🚀 آپدیت JozmozDNS به آخرین نسخه از گیت‌هاب (Update)"
        echo -e "  ${CYAN}[11]${NC} 💾 ایجاد فایل پشتیبان دیتابیس (Backup Database)"
        echo -e "  ${CYAN}[12]${NC} ♻️  بازیابی نسخه پشتیبان (Restore Database)"
        echo -e "  ${CYAN}[13]${NC} 🤖 مدیریت ربات تلگرام (Telegram Bot)"
        echo -e "  ${CYAN}[14]${NC} 🌐 تغییر IP عمومی سرور DNS (Change DNS Server IP)"
        echo -e "  ${CYAN}[15]${NC} 🔑 تغییر / بازنشانی رمز عبور ادمین (Change Admin Password)"
        echo -e "  ${RED}[16]${NC} 🗑️  حذف کامل JozmozDNS (Uninstall)"
        echo -e "  ${YELLOW}[0]${NC}  🚪 خروج (Exit)"
        echo -e "${CYAN}────────────────────────────────────────────────────────────────────────${NC}"
        read -rp " عدد مورد نظر را وارد کنید [0-16]: " choice

        case "$choice" in
            1)
                echo -e "\n${BOLD}--- Systemd Service Status ---${NC}"
                systemctl status "$SERVICE_NAME" --no-pager || true
                read -rp "Press Enter to continue..."
                ;;
            2)
                echo -e "\n${CYAN}Restarting JozmozDNS...${NC}"
                free_port_53
                systemctl restart "$SERVICE_NAME"
                sleep 2
                if ! systemctl is-active --quiet "$SERVICE_NAME"; then
                    echo -e "\n${RED}⚠️  Service failed to start! Recent logs:${NC}"
                    journalctl -u "$SERVICE_NAME" -n 20 --no-pager || true
                else
                    echo -e "${GREEN}✓ Service restarted!${NC}"
                fi
                sleep 1.5
                ;;
            3)
                echo -e "\n${YELLOW}Stopping JozmozDNS...${NC}"
                systemctl stop "$SERVICE_NAME"
                echo -e "${YELLOW}✓ Service stopped!${NC}"
                sleep 1.5
                ;;
            4)
                echo -e "\n${CYAN}Starting JozmozDNS...${NC}"
                free_port_53
                systemctl start "$SERVICE_NAME"
                sleep 2
                if ! systemctl is-active --quiet "$SERVICE_NAME"; then
                    echo -e "\n${RED}⚠️  Service failed to start! Recent logs:${NC}"
                    journalctl -u "$SERVICE_NAME" -n 20 --no-pager || true
                else
                    echo -e "${GREEN}✓ Service started!${NC}"
                fi
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
                update_JozmozDNS
                ;;
            11)
                backup_database
                ;;
            12)
                restore_database
                ;;
            13)
                manage_telegram_bot
                ;;
            14)
                change_dns_server_ip
                ;;
            15)
                change_admin_password
                ;;
            16)
                uninstall_JozmozDNS
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
