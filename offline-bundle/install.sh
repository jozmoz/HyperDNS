#!/usr/bin/env bash
# ==============================================================================
# JozmozDNS — 1-Line Quick Installer for Linux (Ubuntu, Debian, CentOS, AlmaLinux, Rocky)
# Next-Gen Standalone SmartDNS & Anti-Sanction Gaming Gateway
# GitHub: https://github.com/jozmoz/HyperDNS
#
# Quick 1-Line Installation Command:
#   bash <(curl -Ls https://raw.githubusercontent.com/jozmoz/HyperDNS/main/install.sh)
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
REPO_RAW="https://raw.githubusercontent.com/jozmoz/HyperDNS/main"
SERVICE_NAME="JozmozDNS"

# 1. Root verification
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}[ERROR] Please run this installer as root (e.g. sudo bash ...)${NC}"
    exit 1
fi

# 2. Architecture Check
ARCH=$(uname -m)
if [ "$ARCH" != "x86_64" ] && [ "$ARCH" != "amd64" ]; then
    echo -e "${RED}[ERROR] Currently pre-built JozmozDNS supports x86_64 (amd64) architecture.${NC}"
    echo -e "Detected architecture: $ARCH"
    exit 1
fi

# Banner
clear 2>/dev/null || true
echo -e "${CYAN}${BOLD}"
echo "  ██╗ ██████╗ ███████╗███╗   ███╗ ██████╗ ███████╗██████╗ ███╗   ██╗███████╗"
echo "  ██║██╔═══██╗╚══███╔╝████╗ ████║██╔═══██╗╚══███╔╝██╔══██╗████╗  ██║██╔════╝"
echo "  ██║██║   ██║  ███╔╝ ██╔████╔██║██║   ██║  ███╔╝ ██║  ██║██╔██╗ ██║███████╗"
echo "  ██║██║   ██║ ███╔╝  ██║╚██╔╝██║██║   ██║ ███╔╝  ██║  ██║██║╚██╗██║╚════██║"
echo "  ██║╚██████╔╝███████╗██║ ╚═╝ ██║╚██████╔╝███████╗██████╔╝██║ ╚████║███████║"
echo "  ╚═╝ ╚═════╝ ╚══════╝╚═╝     ╚═╝ ╚═════╝ ╚══════╝╚═════╝ ╚═╝  ╚═══╝╚══════╝"
echo -e "       ${PURPLE}⚡ Standalone SmartDNS & Anti-Sanction Gaming Gateway ⚡${NC}"
echo -e "       ${YELLOW}Automated 1-Line Installer · GitHub: https://github.com/jozmoz/HyperDNS${NC}"
echo -e "${CYAN}────────────────────────────────────────────────────────────────────────${NC}\n"

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
    is_port_busy 80 "${1:-$PUBLIC_IP}"
}

# Helper: Find the process occupying Port 80 (optionally on target IP)
get_port_80_process() {
    get_port_process 80 "${1:-$PUBLIC_IP}"
}

# Helper: Check if Port 53 is occupied (optionally on target IP)
is_port_53_busy() {
    is_port_busy 53 "${1:-$PUBLIC_IP}"
}

# Helper: Identify process listening on Port 53 (optionally on target IP)
get_port_53_process() {
    get_port_process 53 "${1:-$PUBLIC_IP}"
}

# Helper: Safely release Port 53 and preserve upstream DNS resolution
free_port_53() {
    local target_ip="${1:-$PUBLIC_IP}"
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

    local TTY_DEV=""
    if [ -t 0 ]; then
        TTY_DEV=""
    elif [ -e /dev/tty ]; then
        TTY_DEV="/dev/tty"
    else
        echo -e "${YELLOW}  [!] Non-interactive session: Switching to Direct IP mode...${NC}"
        USER_DOMAIN=""
        USER_EMAIL=""
        return 0
    fi

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
        if [ -n "$TTY_DEV" ]; then
            read -rp "  شماره گزینه مورد نظر را انتخاب کنید [1-6] (پیش‌فرض: 1): " wiz_choice < "$TTY_DEV" || true
        else
            read -rp "  شماره گزینه مورد نظر را انتخاب کنید [1-6] (پیش‌فرض: 1): " wiz_choice
        fi
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

                # Ensure certbot is installed
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
                if [ -n "$TTY_DEV" ]; then
                    certbot certonly --manual --preferred-challenges dns -d "${domain}" ${cb_mail} --agree-tos < "$TTY_DEV" || true
                else
                    certbot certonly --manual --preferred-challenges dns -d "${domain}" ${cb_mail} --agree-tos || true
                fi

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
                local stop_ok=""
                if [ -n "$TTY_DEV" ]; then
                    read -rp "  آیا اجازه می‌دهید پروسه '${occ_proc}' موقتاً متوقف شود؟ [y/N]: " stop_ok < "$TTY_DEV" || true
                else
                    read -rp "  آیا اجازه می‌دهید پروسه '${occ_proc}' موقتاً متوقف شود؟ [y/N]: " stop_ok
                fi
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
                local in_cert="" in_key=""
                if [ -n "$TTY_DEV" ]; then
                    read -rp "  مسیر فایل Certificate (.crt یا .pem یا fullchain.pem): " in_cert < "$TTY_DEV" || true
                    read -rp "  مسیر فایل Private Key (.key یا privkey.pem): " in_key < "$TTY_DEV" || true
                else
                    read -rp "  مسیر فایل Certificate (.crt یا .pem یا fullchain.pem): " in_cert
                    read -rp "  مسیر فایل Private Key (.key یا privkey.pem): " in_key
                fi
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
                if [ -n "$TTY_DEV" ]; then
                    while IFS= read -r line < "$TTY_DEV"; do
                        [ "$line" = "EOF" ] && break
                        cert_content="${cert_content}${line}"$'\n'
                    done
                else
                    while IFS= read -r line; do
                        [ "$line" = "EOF" ] && break
                        cert_content="${cert_content}${line}"$'\n'
                    done
                fi

                echo -e "\n  متن کامل Private Key خود را پیست کنید (از -----BEGIN PRIVATE KEY----- تا -----END PRIVATE KEY-----)."
                echo -e "  پس از پیست کردن، در خط جدید کلمه ${YELLOW}EOF${NC} را تایپ کرده و Enter بزنید:\n"
                local key_content=""
                if [ -n "$TTY_DEV" ]; then
                    while IFS= read -r line < "$TTY_DEV"; do
                        [ "$line" = "EOF" ] && break
                        key_content="${key_content}${line}"$'\n'
                    done
                else
                    while IFS= read -r line; do
                        [ "$line" = "EOF" ] && break
                        key_content="${key_content}${line}"$'\n'
                    done
                fi

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
                USER_DOMAIN=""
                USER_EMAIL=""
                if [ -f "${INSTALL_DIR}/JozmozDNS" ]; then
                    "${INSTALL_DIR}/JozmozDNS" -domain "none" -public-ip "${public_ip}" -bind "${public_ip}" >/dev/null 2>&1 || true
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

# 3. Detect & Configure DNS Server Public IP
echo -e "${CYAN}[1/5] Detecting server environment and all IP addresses...${NC}"

# Detect existing IP in systemd service if available
EXISTING_IP=""
if [ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]; then
    EXISTING_IP=$(grep -oE -- '-public-ip\s+[^ ]+' "/etc/systemd/system/${SERVICE_NAME}.service" 2>/dev/null | awk '{print $2}' | tr -d '[:space:]' || true)
fi

# Collect all available server IPs
declare -a DETECTED_IPS=()
declare -a IP_LABELS=()

# 1. External Public WAN IPv4
WAN_IP=$(curl -4 -s --connect-timeout 4 https://api.ipify.org 2>/dev/null || \
         curl -4 -s --connect-timeout 4 https://ifconfig.me 2>/dev/null || true)
WAN_IP=$(echo "$WAN_IP" | tr -d '[:space:]')
if [[ "$WAN_IP" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]] && [ "$WAN_IP" != "127.0.0.1" ]; then
    DETECTED_IPS+=("$WAN_IP")
    IP_LABELS+=("Public WAN")
fi

# 2. Local interface IPs via 'ip -4 -o addr show'
if command -v ip >/dev/null 2>&1; then
    while IFS= read -r line; do
        ip_cand=$(echo "$line" | awk '{print $4}' | cut -d/ -f1)
        if_cand=$(echo "$line" | awk '{print $2}')
        if [ -z "$ip_cand" ] || [[ "$ip_cand" =~ ^127\. ]]; then
            continue
        fi
        is_dup=false
        for cur in "${DETECTED_IPS[@]}"; do
            if [ "$cur" = "$ip_cand" ]; then
                is_dup=true
                break
            fi
        done
        if [ "$is_dup" = false ] && [[ "$ip_cand" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
            DETECTED_IPS+=("$ip_cand")
            IP_LABELS+=("Interface: $if_cand")
        fi
    done < <(ip -4 -o addr show scope global 2>/dev/null || true)
fi

# 3. Fallback via hostname -I if no IPs found yet
if [ ${#DETECTED_IPS[@]} -eq 0 ] && command -v hostname >/dev/null 2>&1; then
    for hip in $(hostname -I 2>/dev/null || true); do
        if [[ "$hip" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]] && [ "$hip" != "127.0.0.1" ]; then
            DETECTED_IPS+=("$hip")
            IP_LABELS+=("Local IP")
        fi
    done
fi

# 4. Fallback if still empty
if [ ${#DETECTED_IPS[@]} -eq 0 ]; then
    DETECTED_IPS+=("127.0.0.1")
    IP_LABELS+=("Loopback")
fi

# Determine default choice
DEFAULT_IDX=1
if [ -n "$EXISTING_IP" ]; then
    for idx in "${!DETECTED_IPS[@]}"; do
        if [ "${DETECTED_IPS[$idx]}" = "$EXISTING_IP" ]; then
            DEFAULT_IDX=$((idx + 1))
            break
        fi
    done
fi
DEFAULT_IP="${DETECTED_IPS[$((DEFAULT_IDX - 1))]}"

if [ -n "$DNS_SERVER_IP" ]; then
    PUBLIC_IP="$DNS_SERVER_IP"
    echo -e "  Using DNS Server IP from environment: ${GREEN}${PUBLIC_IP}${NC}"
elif [ -n "$CUSTOM_IP" ]; then
    PUBLIC_IP="$CUSTOM_IP"
    echo -e "  Using DNS Server IP from environment: ${GREEN}${PUBLIC_IP}${NC}"
else
    echo -e "\n  ${BOLD}Detected Server IP Addresses:${NC}"
    for idx in "${!DETECTED_IPS[@]}"; do
        num=$((idx + 1))
        ip_val="${DETECTED_IPS[$idx]}"
        ip_lbl="${IP_LABELS[$idx]}"
        if [ "$num" -eq "$DEFAULT_IDX" ]; then
            echo -e "    ${CYAN}[$num]${NC} ${GREEN}${BOLD}${ip_val}${NC} (${ip_lbl} - ${YELLOW}Default${NC})"
        else
            echo -e "    ${CYAN}[$num]${NC} ${GREEN}${BOLD}${ip_val}${NC} (${ip_lbl})"
        fi
    done
    echo -e "    ${CYAN}[c]${NC} ${YELLOW}Enter custom IP manually (ورود دستی)${NC}\n"

    USER_CHOICE=""
    if [ -t 0 ]; then
        read -rp "  Select an IP [1-${#DETECTED_IPS[@]}, or 'c' for manual] (Default: [${DEFAULT_IDX}] ${DEFAULT_IP}): " USER_CHOICE
    elif [ -e /dev/tty ]; then
        read -rp "  Select an IP [1-${#DETECTED_IPS[@]}, or 'c' for manual] (Default: [${DEFAULT_IDX}] ${DEFAULT_IP}): " USER_CHOICE < /dev/tty || true
    fi

    USER_CHOICE=$(echo "$USER_CHOICE" | tr -d '[:space:]"'\''')

    if [ -z "$USER_CHOICE" ]; then
        PUBLIC_IP="$DEFAULT_IP"
    elif [[ "$USER_CHOICE" =~ ^[0-9]+$ ]] && [ "$USER_CHOICE" -ge 1 ] && [ "$USER_CHOICE" -le "${#DETECTED_IPS[@]}" ]; then
        PUBLIC_IP="${DETECTED_IPS[$((USER_CHOICE - 1))]}"
    elif [ "$USER_CHOICE" = "c" ] || [ "$USER_CHOICE" = "C" ] || [ "$USER_CHOICE" = "manual" ]; then
        MANUAL_IP=""
        if [ -t 0 ]; then
            read -rp "  Enter custom DNS Server IP: " MANUAL_IP
        elif [ -e /dev/tty ]; then
            read -rp "  Enter custom DNS Server IP: " MANUAL_IP < /dev/tty || true
        fi
        MANUAL_IP=$(echo "$MANUAL_IP" | tr -d '[:space:]"'\''')
        PUBLIC_IP="${MANUAL_IP:-$DEFAULT_IP}"
    elif [[ "$USER_CHOICE" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
        PUBLIC_IP="$USER_CHOICE"
    else
        echo -e "  ${YELLOW}Invalid selection, using default:${NC} ${DEFAULT_IP}"
        PUBLIC_IP="$DEFAULT_IP"
    fi
fi

echo -e "\n  ${GREEN}✓ Confirmed DNS Server IP:${NC} ${CYAN}${BOLD}${PUBLIC_IP}${NC}"

# Interactive Domain & SSL Configuration
echo ""
echo -e "${YELLOW}──────────────────────────────────────────────────────────${NC}"
echo -e "${YELLOW}${BOLD}  ⚡ JozmozDNS Domain & SSL Configuration${NC}"
echo -e "  Configured Server IP: ${GREEN}${PUBLIC_IP}${NC}"
echo -e "${YELLOW}──────────────────────────────────────────────────────────${NC}"
echo -e "  💡 You can enter a domain (with free Let's Encrypt SSL),"
echo -e "     or press Enter to use the IP directly (Direct HTTPS on IP).\n"

USER_DOMAIN=""
if [ -t 0 ]; then
    read -rp "  Enter Panel Domain (e.g. dns.example.com) [Press Enter for IP: ${PUBLIC_IP}]: " USER_DOMAIN
elif [ -e /dev/tty ]; then
    read -rp "  Enter Panel Domain (e.g. dns.example.com) [Press Enter for IP: ${PUBLIC_IP}]: " USER_DOMAIN < /dev/tty || true
fi
USER_DOMAIN=$(echo "$USER_DOMAIN" | tr -d '[:space:]')

USER_EMAIL=""
if [ -n "$USER_DOMAIN" ]; then
    if [ -t 0 ]; then
        read -rp "  Enter Admin Email for Let's Encrypt [admin@${USER_DOMAIN}]: " USER_EMAIL
    elif [ -e /dev/tty ]; then
        read -rp "  Enter Admin Email for Let's Encrypt [admin@${USER_DOMAIN}]: " USER_EMAIL < /dev/tty || true
    fi
    USER_EMAIL=$(echo "$USER_EMAIL" | tr -d '[:space:]')
    if [ -z "$USER_EMAIL" ]; then
        USER_EMAIL="admin@${USER_DOMAIN}"
    fi
    echo -e "  ${GREEN}✓ Domain configured:${NC} ${CYAN}${USER_DOMAIN}${NC} (${USER_EMAIL})"
else
    echo -e "  ${GREEN}✓ Direct IP Mode selected:${NC} ${CYAN}${PUBLIC_IP}${NC}"
fi

# 4. System dependencies
echo -e "\n${CYAN}[2/5] Installing dependencies and freeing Port 53...${NC}"
if command -v apt-get >/dev/null 2>&1; then
    apt-get update -y -q || true
    apt-get install -y -q curl openssl lsof systemd iptables net-tools certbot psmisc 2>/dev/null || \
    apt-get install -y -q curl openssl lsof systemd iptables net-tools psmisc || true
elif command -v dnf >/dev/null 2>&1; then
    dnf install -y -q epel-release 2>/dev/null || true
    dnf install -y -q curl openssl lsof systemd iptables net-tools certbot psmisc 2>/dev/null || \
    dnf install -y -q curl openssl lsof systemd iptables net-tools psmisc || true
elif command -v yum >/dev/null 2>&1; then
    yum install -y -q epel-release 2>/dev/null || true
    yum install -y -q curl openssl lsof systemd iptables net-tools certbot psmisc 2>/dev/null || \
    yum install -y -q curl openssl lsof systemd iptables net-tools psmisc || true
fi

# Stop existing JozmozDNS instance so it does not falsely occupy ports or trigger conflicts
systemctl stop "$SERVICE_NAME" 2>/dev/null || true
pkill -9 -f "${INSTALL_DIR}/JozmozDNS" 2>/dev/null || true

# Check if Port 80 is occupied only when a domain was selected AND we don't already have a valid certificate
if [ -n "$USER_DOMAIN" ] && ! is_valid_domain_cert "$USER_DOMAIN"; then
    if is_port_80_busy "$PUBLIC_IP"; then
        OCC_PROC=$(get_port_80_process "$PUBLIC_IP")
        echo ""
        echo -e "${RED}${BOLD}────────────────────────────────────────────────────────────────────────${NC}"
        echo -e "${RED}${BOLD}  ⚠️  اخطار: پورت ۸۰ روی ${PUBLIC_IP} اشغال است! / Warning: Port 80 on ${PUBLIC_IP} is occupied!${NC}"
        echo -e "${YELLOW}      پروسه در حال استفاده / Occupying Process: ${CYAN}${BOLD}${OCC_PROC}${NC}"
        echo -e "${RED}${BOLD}────────────────────────────────────────────────────────────────────────${NC}"
        echo -e "  پروتکل Let's Encrypt برای صدور خودکار SSL به پورت ۸۰ آزاد روی ${PUBLIC_IP} نیاز دارد."
        echo -e "  چون این پورت اشغال است، لطفاً روش دریافت یا تنظیم دستی سرتیفیکیت را مشخص کنید.\n"

        manual_ssl_wizard "$USER_DOMAIN" "$USER_EMAIL" "$PUBLIC_IP" "Port 80 on ${PUBLIC_IP} is occupied by ${OCC_PROC}"
    fi
fi

# Free Port 53 for chosen IP
free_port_53 "$PUBLIC_IP"

# 5. Setup Installation Directories
mkdir -p "$INSTALL_DIR"
mkdir -p "$INSTALL_DIR/certs"
mkdir -p "$INSTALL_DIR/rules"
mkdir -p "$INSTALL_DIR/backups"

IS_UPDATE=false
if [ -f "$INSTALL_DIR/JozmozDNS" ] || [ -f "$INSTALL_DIR/data.db" ]; then
    IS_UPDATE=true
fi

# 6. Install JozmozDNS Binary and Terminal Console
echo -e "\n${CYAN}[3/5] Installing JozmozDNS binary and Terminal Console...${NC}"
systemctl stop "$SERVICE_NAME" 2>/dev/null || true
pkill -9 -f "${INSTALL_DIR}/jozmozdns" 2>/dev/null || true
pkill -9 -f "${INSTALL_DIR}/JozmozDNS" 2>/dev/null || true

CACHE_BUSTER=$(date +%s)
echo -e "  Downloading latest JozmozDNS binary from GitHub..."
curl -fsSL --progress-bar -o "$INSTALL_DIR/jozmozdns" "${REPO_RAW}/jozmozdns-linux?t=${CACHE_BUSTER}" || \
curl -fsSL --progress-bar -o "$INSTALL_DIR/jozmozdns" "${REPO_RAW}/JozmozDNS-linux?t=${CACHE_BUSTER}"
chmod +x "$INSTALL_DIR/jozmozdns"
ln -sf "$INSTALL_DIR/jozmozdns" "$INSTALL_DIR/JozmozDNS" 2>/dev/null || true
ln -sf "$INSTALL_DIR" "/opt/jozmozdns" 2>/dev/null || true

# Install management CLI menu
curl -fsSL -o "/usr/local/bin/jozmozdns" "${REPO_RAW}/scripts/jozmozdns-menu.sh?t=${CACHE_BUSTER}" || \
curl -fsSL -o "/usr/local/bin/jozmozdns" "${REPO_RAW}/scripts/JozmozDNS-menu.sh?t=${CACHE_BUSTER}"
chmod +x "/usr/local/bin/jozmozdns"
ln -sf "/usr/local/bin/jozmozdns" "/usr/local/bin/jdns" 2>/dev/null || true
ln -sf "/usr/local/bin/jozmozdns" "/usr/local/bin/JozmozDNS" 2>/dev/null || true
ln -sf "/usr/local/bin/jozmozdns" "/usr/local/bin/jmdns" 2>/dev/null || true

# 7. Generate dedicated SSL Certificate (covering IP and localhost)
echo -e "\n${CYAN}[4/5] Configuring SSL Certificate and systemd service...${NC}"
# Clean up any stale self-signed or invalid certs for domain so they don't block fresh ACME
if [ -n "$USER_DOMAIN" ] && ! is_valid_domain_cert "$USER_DOMAIN"; then
    rm -f "$INSTALL_DIR/certs/acme/${USER_DOMAIN}.crt" "$INSTALL_DIR/certs/acme/${USER_DOMAIN}.key" 2>/dev/null || true
fi

if [ -n "$USER_DOMAIN" ] && is_valid_domain_cert "$USER_DOMAIN"; then
    echo -e "  ${GREEN}✓ Valid CA-signed certificate found for ${USER_DOMAIN}!${NC}"
else
    OPENSSL_CNF="/tmp/openssl_JozmozDNS_$$.cnf"
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
CN = ${USER_DOMAIN:-$PUBLIC_IP}

[v3_req]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names

[alt_names]
IP.1 = ${PUBLIC_IP}
IP.2 = 127.0.0.1
DNS.1 = localhost
EOF

    if [ -n "$USER_DOMAIN" ]; then
        echo "DNS.2 = ${USER_DOMAIN}" >> "$OPENSSL_CNF"
    fi

    openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
        -keyout "$INSTALL_DIR/certs/key.pem" \
        -out "$INSTALL_DIR/certs/cert.pem" \
        -config "$OPENSSL_CNF" 2>/dev/null || \
    openssl req -x509 -nodes -days 3650 -newkey rsa:2048 \
        -keyout "$INSTALL_DIR/certs/key.pem" \
        -out "$INSTALL_DIR/certs/cert.pem" \
        -subj "/CN=${USER_DOMAIN:-$PUBLIC_IP}/O=JozmozDNS" 2>/dev/null || true

    rm -f "$OPENSSL_CNF"
    chmod 600 "$INSTALL_DIR/certs/key.pem" "$INSTALL_DIR/certs/cert.pem" 2>/dev/null || true
    echo -e "  ${GREEN}✓ Dedicated SSL certificate generated for IP ${PUBLIC_IP}!${NC}"
fi

# Enable nonlocal bind in kernel so JozmozDNS can bind to any designated IP address cleanly
sysctl -w net.ipv4.ip_nonlocal_bind=1 >/dev/null 2>&1 || true
mkdir -p /etc/sysctl.d 2>/dev/null || true
echo "net.ipv4.ip_nonlocal_bind=1" > /etc/sysctl.d/99-JozmozDNS.conf 2>/dev/null || true

# Ensure systemd service configuration (strictly bound to selected PUBLIC_IP)
cat <<EOF > /etc/systemd/system/JozmozDNS.service
[Unit]
Description=JozmozDNS Standalone SmartDNS & Gaming Gateway
After=network.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/JozmozDNS -server -public-ip ${PUBLIC_IP} -bind ${PUBLIC_IP}
Restart=always
RestartSec=3
LimitNOFILE=65535
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload

# 8. Initial Bootstrap & Service Startup
echo -e "\n${CYAN}[5/5] Initializing JozmozDNS and starting background engine...${NC}"
systemctl stop "$SERVICE_NAME" 2>/dev/null || true
pkill -9 -f "${INSTALL_DIR}/JozmozDNS" 2>/dev/null || true
free_port_53 "$PUBLIC_IP"

BOOTSTRAP_LOG="$INSTALL_DIR/install.log"
rm -f "$BOOTSTRAP_LOG"

if [ -n "$USER_DOMAIN" ]; then
    if is_valid_domain_cert "$USER_DOMAIN"; then
        echo -e "  Starting JozmozDNS engine with verified certificate for ${GREEN}${USER_DOMAIN}${NC}..."
        timeout 10 "$INSTALL_DIR/JozmozDNS" -server -domain "$USER_DOMAIN" -email "$USER_EMAIL" -public-ip "$PUBLIC_IP" -bind "$PUBLIC_IP" < /dev/null > "$BOOTSTRAP_LOG" 2>&1 || true
        pkill -9 -f "${INSTALL_DIR}/JozmozDNS" 2>/dev/null || true
    else
        echo -e "  Configuring domain ${GREEN}${USER_DOMAIN}${NC} and setting up Let's Encrypt SSL..."
        timeout 20 "$INSTALL_DIR/JozmozDNS" -server -domain "$USER_DOMAIN" -email "$USER_EMAIL" -public-ip "$PUBLIC_IP" -bind "$PUBLIC_IP" < /dev/null > "$BOOTSTRAP_LOG" 2>&1 || true
        pkill -9 -f "${INSTALL_DIR}/JozmozDNS" 2>/dev/null || true

        # If automated issuance failed or timed out:
        if ! is_valid_domain_cert "$USER_DOMAIN"; then
            echo ""
            echo -e "${YELLOW}────────────────────────────────────────────────────────────────────────${NC}"
            echo -e "${YELLOW}⚠️  صدور خودکار سرتیفیکیت روی پورت ۸۰ کامل نشد!${NC}"
            echo -e "    (احتمالاً پورت ۸۰ توسط فایروال مسدود است یا رکورد دامنه هنوز اعمال نشده است)"
            echo -e "${YELLOW}────────────────────────────────────────────────────────────────────────${NC}"
            manual_ssl_wizard "$USER_DOMAIN" "$USER_EMAIL" "$PUBLIC_IP" "Automated ACME verification failed on Port 80"
        fi
    fi
else
    echo -e "  Configuring direct IP mode with dedicated SSL..."
    timeout 10 "$INSTALL_DIR/JozmozDNS" -server -public-ip "$PUBLIC_IP" -bind "$PUBLIC_IP" < /dev/null > "$BOOTSTRAP_LOG" 2>&1 || true
    pkill -9 -f "${INSTALL_DIR}/JozmozDNS" 2>/dev/null || true
fi

# Enable and start the permanent background service safely
pkill -9 -f "${INSTALL_DIR}/JozmozDNS" 2>/dev/null || true
free_port_53 "$PUBLIC_IP"
systemctl daemon-reload || true
systemctl enable "$SERVICE_NAME" >/dev/null 2>&1 || true
echo -e "  Starting JozmozDNS background service via systemd..."
systemctl restart "$SERVICE_NAME" 2>/dev/null || systemctl start "$SERVICE_NAME" 2>/dev/null || true

# Wait for service to become active
SVC_ACTIVE=false
for i in 1 2 3 4 5 6 7; do
    if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
        SVC_ACTIVE=true
        break
    fi
    sleep 1
done

if [ "$SVC_ACTIVE" = true ]; then
    echo -e "  ${GREEN}✓ JozmozDNS service is active and running!${NC}"
fi

# Verify service is running, handle potential startup failure
if [ "$SVC_ACTIVE" = false ]; then
    SVC_ERR_LOG=$(journalctl -u "$SERVICE_NAME" -n 35 --no-pager 2>/dev/null || true)

    # Check if the failure was caused by Port 53 collision
    if echo "$SVC_ERR_LOG" | grep -iqE "bind: address already in use|bind DNS UDP.*:53|bind DNS TCP.*:53"; then
        echo -e "\n${YELLOW}⚠️  تشخیص: پورت ۵۳ هنوز آزاد نشده بود. در حال آزادسازی اجباری و راه‌اندازی مجدد...${NC}"
        free_port_53 "$PUBLIC_IP"
        systemctl restart "$SERVICE_NAME" 2>/dev/null || true
        for i in 1 2 3 4 5; do
            if systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null; then
                SVC_ACTIVE=true
                echo -e "  ${GREEN}✓ سرویس JozmozDNS با موفقیت شروع به کار کرد!${NC}"
                break
            fi
            sleep 1
        done
    fi

    # If still not active after port 53 retry
    if [ "$SVC_ACTIVE" = false ]; then
        echo -e "\n${RED}⚠️  سرویس JozmozDNS فعال نشد! در حال بررسی لاگ‌های سیستم...${NC}"
        echo "$SVC_ERR_LOG" | tail -15
        echo ""
        if echo "$SVC_ERR_LOG" | grep -iqE "tls:|certificate|x509|acme" && [ -n "$USER_DOMAIN" ] && [ -t 0 -o -e /dev/tty ]; then
            echo -e "${YELLOW}به دلیل خطای تایید سرتیفیکیت دامنه، آیا مایلید موقتاً با IP مستقیم (${PUBLIC_IP}) و SSL اختصاصی IP راه‌اندازی شود؟ [y/N]: ${NC}"
            fb_mode="N"
            if [ -t 0 ]; then
                read -rp "  انتخاب [y/N]: " fb_mode
            elif [ -e /dev/tty ]; then
                read -rp "  انتخاب [y/N]: " fb_mode < /dev/tty || true
            fi
            if [[ "$fb_mode" =~ ^[yY] ]]; then
                USER_DOMAIN=""
                "$INSTALL_DIR/JozmozDNS" -domain "none" -public-ip "$PUBLIC_IP" -bind "$PUBLIC_IP" < /dev/null >/dev/null 2>&1 || true
                pkill -9 -f "${INSTALL_DIR}/JozmozDNS" 2>/dev/null || true
                free_port_53 "$PUBLIC_IP"
                systemctl restart "$SERVICE_NAME" 2>/dev/null || true
                sleep 2
            fi
        fi
    fi
fi
sleep 1

# Read Credentials and Dashboard Link
echo -e "  Reading credentials and dashboard access information..."
LOG_CONTENT=$(cat "$BOOTSTRAP_LOG" 2>/dev/null || true)
SYSTEMD_LOG=$(journalctl -u "$SERVICE_NAME" -n 80 --no-pager 2>/dev/null || true)
FULL_LOG="${LOG_CONTENT}
${SYSTEMD_LOG}"

# Check previously saved credentials if any
SAVED_USER=""
SAVED_PASS=""
SAVED_URL=""
if [ -f "$INSTALL_DIR/credentials.txt" ]; then
    SAVED_USER=$(grep -E "^username\s*:" "$INSTALL_DIR/credentials.txt" 2>/dev/null | awk -F':' '{print $2}' | tr -d '[:space:]' || true)
    SAVED_PASS=$(grep -E "^password\s*:" "$INSTALL_DIR/credentials.txt" 2>/dev/null | awk -F':' '{print $2}' | tr -d '[:space:]' || true)
    SAVED_URL=$(grep -E "^admin_url\s*:" "$INSTALL_DIR/credentials.txt" 2>/dev/null | sed -e 's/^admin_url\s*:\s*//' | tr -d '[:space:]' || true)
fi

DASH_URL=$(echo "$FULL_LOG" | grep -iE "(JozmozDNS|JozmozDNS) Dashboard" | tail -1 | sed -e 's/.*Dashboard : //I' | tr -d '[:space:]' || true)
if [ -z "$DASH_URL" ]; then
    ADMIN_PATH=$(echo "$FULL_LOG" | grep -oE "https?://[^ ]+/[a-f0-9]{16}/dash/login" | tail -1 || true)
    if [ -n "$ADMIN_PATH" ]; then
        DASH_URL="$ADMIN_PATH"
    elif [ -n "$SAVED_URL" ]; then
        DASH_URL="$SAVED_URL"
    fi
fi

USERNAME=$(echo "$FULL_LOG" | grep -E "username\s*:" | tail -1 | awk -F':' '{print $2}' | tr -d '[:space:]' || true)
if [ -z "$USERNAME" ]; then
    USERNAME="${SAVED_USER:-admin}"
fi

PASSWORD=$(echo "$FULL_LOG" | grep -E "password\s*:" | tail -1 | awk -F':' '{print $2}' | tr -d '[:space:]' || true)
if [ -z "$PASSWORD" ]; then
    PASSWORD="$SAVED_PASS"
fi

# If password is still unknown (e.g. legacy upgrade where logs rotated), generate and apply a secure password
if [ -z "$PASSWORD" ]; then
    PASSWORD=$(openssl rand -hex 12 2>/dev/null || date +%s%N | sha256sum | head -c 24)
    systemctl stop "$SERVICE_NAME" 2>/dev/null || true
    "$INSTALL_DIR/JozmozDNS" -password "$PASSWORD" < /dev/null >/dev/null 2>&1 || true
    systemctl start "$SERVICE_NAME" 2>/dev/null || true
fi

# Persist credentials securely
cat <<CREDEOF > "$INSTALL_DIR/credentials.txt"
username: ${USERNAME}
password: ${PASSWORD}
admin_url: ${DASH_URL}
CREDEOF
chmod 600 "$INSTALL_DIR/credentials.txt"

# 10. Installation Success Banner
echo ""
echo -e "${GREEN}${BOLD}========================================================================${NC}"
if [ "$IS_UPDATE" = true ]; then
    echo -e "${GREEN}${BOLD}       🎉 JozmozDNS Successfully Updated & Configured!                  ${NC}"
else
    echo -e "${GREEN}${BOLD}          🎉 JozmozDNS Successfully Installed and Running!             ${NC}"
fi
echo -e "${GREEN}${BOLD}========================================================================${NC}"
echo ""
echo -e "  ${BOLD}Status:${NC}        ${GREEN}● ONLINE (Active)${NC}"
if [ -n "$DASH_URL" ]; then
    echo -e "  ${BOLD}Web Dashboard:${NC}  ${CYAN}${DASH_URL}${NC}"
else
    echo -e "  ${BOLD}Web Dashboard:${NC}  ${CYAN}https://${USER_DOMAIN:-$PUBLIC_IP}:8080/<admin-path>/dash/login${NC}"
fi

echo -e "  ${BOLD}Admin Username:${NC} ${YELLOW}${USERNAME}${NC}"
echo -e "  ${BOLD}Admin Password:${NC} ${YELLOW}${BOLD}${PASSWORD}${NC}"
echo ""
echo -e "  ${BOLD}DNS Server IP:${NC}  ${GREEN}${PUBLIC_IP}${NC}"
if [ -n "$USER_DOMAIN" ]; then
    echo -e "  ${BOLD}Domain & SSL:${NC}   ${CYAN}${USER_DOMAIN}${NC} (Let's Encrypt)"
else
    echo -e "  ${BOLD}Domain & SSL:${NC}   ${CYAN}${PUBLIC_IP}${NC} (Dedicated IP SSL Certificate)"
fi
echo -e "  ${BOLD}Terminal Menu:${NC}  Type ${PURPLE}jdns${NC} or ${PURPLE}jozmozdns${NC} (or ${PURPLE}jmdns${NC}) anywhere in your terminal"
echo -e "${GREEN}========================================================================${NC}"
echo ""
echo -e "${CYAN}📌 برای مدیریت سرور، تغییر دامنه، مشاهده مشخصات یا آپدیت دستور زیر را وارد کنید:${NC}"
echo -e "   ${PURPLE}${BOLD}jdns${NC}   ${NC}(یا ${PURPLE}jozmozdns${NC} / ${PURPLE}jmdns${NC})"
echo ""
