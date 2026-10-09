#!/usr/bin/env bash
# ==============================================================================
# HyperDNS — 1-Line Quick Installer for Linux (Ubuntu, Debian, CentOS, AlmaLinux, Rocky)
# Next-Gen Standalone SmartDNS & Anti-Sanction Gaming Gateway
# GitHub: https://github.com/jozmoz/HyperDNS
#
# Quick 1-Line Installation Command:
#   bash <(curl -Ls https://raw.githubusercontent.com/jozmoz/HyperDNS/main/install.sh)
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
REPO_RAW="https://raw.githubusercontent.com/jozmoz/HyperDNS/main"
SERVICE_NAME="hyperdns"

# 1. Root verification
if [ "$EUID" -ne 0 ]; then
    echo -e "${RED}[ERROR] Please run this installer as root (e.g. sudo bash ...)${NC}"
    exit 1
fi

# 2. Architecture Check
ARCH=$(uname -m)
if [ "$ARCH" != "x86_64" ] && [ "$ARCH" != "amd64" ]; then
    echo -e "${RED}[ERROR] Currently pre-built HyperDNS supports x86_64 (amd64) architecture.${NC}"
    echo -e "Detected architecture: $ARCH"
    exit 1
fi

# Banner
clear 2>/dev/null || true
echo -e "${CYAN}${BOLD}"
echo "  ██╗  ██╗██╗   ██╗██████╗ ███████╗██████╗ ██████╗ ███╗   ██╗███████╗"
echo "  ██║  ██║╚██╗ ██╔╝██╔══██╗██╔════╝██╔══██╗██╔══██╗████╗  ██║██╔════╝"
echo "  ███████║ ╚████╔╝ ██████╔╝█████╗  ██████╔╝██║  ██║██╔██╗ ██║███████╗"
echo "  ██╔══██║  ╚██╔╝  ██╔═══╝ ██╔══╝  ██╔══██╗██║  ██║██║╚██╗██║╚════██║"
echo "  ██║  ██║   ██║   ██║     ███████╗██║  ██║██████╔╝██║ ╚████║███████║"
echo "  ╚═╝  ╚═╝   ╚═╝   ╚═╝     ╚══════╝╚═╝  ╚═╝╚═════╝ ╚═╝  ╚═══╝╚══════╝"
echo -e "       ${PURPLE}⚡ Standalone SmartDNS & Anti-Sanction Gaming Gateway ⚡${NC}"
echo -e "       ${YELLOW}Automated 1-Line Installer · GitHub: https://github.com/jozmoz/HyperDNS${NC}"
echo -e "${CYAN}────────────────────────────────────────────────────────────────────────${NC}\n"

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

# 4. System dependencies
echo -e "\n${CYAN}[2/5] Installing dependencies and freeing Port 53...${NC}"
if command -v apt-get >/dev/null 2>&1; then
    apt-get update -y -q
    apt-get install -y -q curl openssl lsof systemd iptables net-tools
elif command -v dnf >/dev/null 2>&1; then
    dnf install -y -q curl openssl lsof systemd iptables net-tools
elif command -v yum >/dev/null 2>&1; then
    yum install -y -q curl openssl lsof systemd iptables net-tools
fi

# Disable systemd-resolved DNSStubListener if it's hogging port 53
if systemctl is-active --quiet systemd-resolved 2>/dev/null; then
    mkdir -p /etc/systemd/resolved.conf.d/
    cat <<EOF > /etc/systemd/resolved.conf.d/hyperdns.conf
[Resolve]
DNS=1.1.1.1 8.8.8.8
DNSStubListener=no
EOF
    systemctl restart systemd-resolved 2>/dev/null || true
fi

# Ensure reliable public upstream DNS resolution in /etc/resolv.conf
if ! grep -q "nameserver 1.1.1.1" /etc/resolv.conf 2>/dev/null; then
    echo "nameserver 1.1.1.1" >> /etc/resolv.conf 2>/dev/null || true
fi

# 5. Setup Installation Directories
mkdir -p "$INSTALL_DIR"
mkdir -p "$INSTALL_DIR/certs"
mkdir -p "$INSTALL_DIR/rules"
mkdir -p "$INSTALL_DIR/backups"

IS_UPDATE=false
if [ -f "$INSTALL_DIR/hyperdns" ] || [ -f "$INSTALL_DIR/data.db" ]; then
    IS_UPDATE=true
fi

# 6. Install HyperDNS Binary and Terminal Console
echo -e "\n${CYAN}[3/5] Installing HyperDNS binary and Terminal Console...${NC}"
systemctl stop "$SERVICE_NAME" 2>/dev/null || true
pkill -9 -f "${INSTALL_DIR}/hyperdns" 2>/dev/null || true

CACHE_BUSTER=$(date +%s)
echo -e "  Downloading latest HyperDNS binary from GitHub..."
curl -fsSL --progress-bar -o "$INSTALL_DIR/hyperdns" "${REPO_RAW}/hyperdns-linux?t=${CACHE_BUSTER}"
chmod +x "$INSTALL_DIR/hyperdns"

# Install management CLI menu
curl -fsSL -o "/usr/local/bin/hyperdns" "${REPO_RAW}/scripts/hyperdns-menu.sh?t=${CACHE_BUSTER}"
chmod +x "/usr/local/bin/hyperdns"
ln -sf "/usr/local/bin/hyperdns" "/usr/local/bin/hdns"

# Ensure systemd service configuration
echo -e "\n${CYAN}[4/5] Configuring systemd background service...${NC}"
cat <<EOF > /etc/systemd/system/hyperdns.service
[Unit]
Description=HyperDNS Standalone SmartDNS & Gaming Gateway
After=network.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/hyperdns -server -public-ip ${PUBLIC_IP}
Restart=always
RestartSec=3
LimitNOFILE=65535
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload

if [ "$IS_UPDATE" = true ]; then
    echo -e "\n${CYAN}[5/5] Updating HyperDNS service with new Game Intelligence Engine (HGI)...${NC}"
    systemctl enable "$SERVICE_NAME" >/dev/null 2>&1
    systemctl restart "$SERVICE_NAME"
    sleep 2

    # Read Credentials and Dashboard Link from systemd journal
    JOURNAL_LOG=$(journalctl -u "$SERVICE_NAME" -n 80 --no-pager 2>/dev/null || true)
    DASH_URL=$(echo "$JOURNAL_LOG" | grep -i "HyperDNS Dashboard" | tail -1 | sed -e 's/.*HyperDNS Dashboard : //I' | tr -d '[:space:]' || true)
    if [ -z "$DASH_URL" ]; then
        DASH_URL=$(echo "$JOURNAL_LOG" | grep -oE "https?://[^ ]+/[a-f0-9]{16}/dash/login" | tail -1 || true)
    fi

    echo ""
    echo -e "${GREEN}${BOLD}========================================================================${NC}"
    echo -e "${GREEN}${BOLD}       🎉 HyperDNS Successfully Updated to v2.3.0 (HGI)!                ${NC}"
    echo -e "${GREEN}${BOLD}========================================================================${NC}"
    echo ""
    echo -e "  ${BOLD}Status:${NC}        ${GREEN}● ONLINE (Active)${NC}"
    if [ -n "$DASH_URL" ]; then
        echo -e "  ${BOLD}Web Dashboard:${NC}  ${CYAN}${DASH_URL}${NC}"
    fi
    echo -e "  ${BOLD}DNS Server IP:${NC}  ${GREEN}${PUBLIC_IP}${NC}"
    echo -e "  ${BOLD}Terminal Menu:${NC}  Type ${PURPLE}hyperdns${NC} or ${PURPLE}hdns${NC} anywhere in your terminal"
    echo -e "  ${BOLD}Data Integrity:${NC} ${GREEN}All existing clients, policies, domains & certs preserved.${NC}"
    echo -e "${GREEN}========================================================================${NC}"
    echo ""
    echo -e "${CYAN}📌 برای مدیریت سرور یا مشاهده اطلاعات داشبورد دستور زیر را وارد کنید:${NC}"
    echo -e "   ${PURPLE}${BOLD}hyperdns${NC}"
    echo ""
    exit 0
fi

# 7. Interactive Domain & SSL Configuration (Fresh Install Only)
echo ""
echo -e "${YELLOW}──────────────────────────────────────────────────────────${NC}"
echo -e "${YELLOW}${BOLD}  ⚡ HyperDNS Domain & SSL Setup${NC}"
echo -e "  Server Public IP: ${GREEN}${PUBLIC_IP}${NC}"
echo -e "  (Point an A record from your domain to this IP before proceeding)"
echo -e "${YELLOW}──────────────────────────────────────────────────────────${NC}"
read -rp "Enter Panel Domain (e.g. dns.example.com) [Press Enter to skip & use direct IP]: " USER_DOMAIN
USER_DOMAIN=$(echo "$USER_DOMAIN" | tr -d '[:space:]')

USER_EMAIL=""
if [ -n "$USER_DOMAIN" ]; then
    read -rp "Enter Admin Email for Let's Encrypt (e.g. admin@$USER_DOMAIN): " USER_EMAIL
    USER_EMAIL=$(echo "$USER_EMAIL" | tr -d '[:space:]')
fi

# 8. Initial Bootstrap & Credentials Generation (Fresh Install Only)
echo -e "\n${CYAN}[5/5] Initializing HyperDNS and generating security credentials...${NC}"
systemctl stop "$SERVICE_NAME" 2>/dev/null || true

BOOTSTRAP_LOG="$INSTALL_DIR/install.log"
rm -f "$BOOTSTRAP_LOG"

if [ -n "$USER_DOMAIN" ]; then
    echo -e "  Configuring domain ${GREEN}${USER_DOMAIN}${NC} and generating SSL certificate..."
    timeout 15 "$INSTALL_DIR/hyperdns" -server -domain "$USER_DOMAIN" -email "$USER_EMAIL" -public-ip "$PUBLIC_IP" > "$BOOTSTRAP_LOG" 2>&1 || true
else
    echo -e "  Configuring direct IP mode..."
    timeout 10 "$INSTALL_DIR/hyperdns" -server -public-ip "$PUBLIC_IP" > "$BOOTSTRAP_LOG" 2>&1 || true
fi

# Enable and start the permanent background service
systemctl enable "$SERVICE_NAME" >/dev/null 2>&1
systemctl restart "$SERVICE_NAME"
sleep 2

# Read Credentials and Dashboard Link
LOG_CONTENT=$(cat "$BOOTSTRAP_LOG" 2>/dev/null || true)
SYSTEMD_LOG=$(journalctl -u "$SERVICE_NAME" -n 60 --no-pager 2>/dev/null || true)
FULL_LOG="${LOG_CONTENT}
${SYSTEMD_LOG}"

DASH_URL=$(echo "$FULL_LOG" | grep -i "HyperDNS Dashboard" | tail -1 | sed -e 's/.*HyperDNS Dashboard : //I' | tr -d '[:space:]' || true)
if [ -z "$DASH_URL" ]; then
    ADMIN_PATH=$(echo "$FULL_LOG" | grep -oE "https?://[^ ]+/[a-f0-9]{16}/dash/login" | tail -1 || true)
    if [ -n "$ADMIN_PATH" ]; then
        DASH_URL="$ADMIN_PATH"
    fi
fi

USERNAME=$(echo "$FULL_LOG" | grep -E "username\s*:" | tail -1 | awk -F':' '{print $2}' | tr -d '[:space:]' || true)
if [ -z "$USERNAME" ]; then
    USERNAME="admin"
fi

PASSWORD=$(echo "$FULL_LOG" | grep -E "password\s*:" | tail -1 | awk -F':' '{print $2}' | tr -d '[:space:]' || true)

# 10. Installation Success Banner
echo ""
echo -e "${GREEN}${BOLD}========================================================================${NC}"
echo -e "${GREEN}${BOLD}          🎉 HyperDNS Successfully Installed and Running!              ${NC}"
echo -e "${GREEN}${BOLD}========================================================================${NC}"
echo ""
if [ -n "$DASH_URL" ]; then
    echo -e "  ${BOLD}Web Dashboard:${NC}  ${CYAN}${DASH_URL}${NC}"
else
    echo -e "  ${BOLD}Web Dashboard:${NC}  ${CYAN}https://${USER_DOMAIN:-$PUBLIC_IP}:8080/<admin-path>/dash/login${NC}"
fi

echo -e "  ${BOLD}Admin Username:${NC} ${YELLOW}${USERNAME}${NC}"
if [ -n "$PASSWORD" ]; then
    echo -e "  ${BOLD}Admin Password:${NC} ${YELLOW}${PASSWORD}${NC}"
else
    echo -e "  ${BOLD}Admin Password:${NC} ${YELLOW}(Already configured in previous installation)${NC}"
fi

echo ""
echo -e "  ${BOLD}DNS Server IP:${NC}  ${GREEN}${PUBLIC_IP}${NC}"
echo -e "  ${BOLD}Terminal Menu:${NC}  Type ${PURPLE}hyperdns${NC} or ${PURPLE}hdns${NC} anywhere in your terminal"
echo -e "${GREEN}========================================================================${NC}"
echo ""
echo -e "${CYAN}📌 برای مدیریت سرور، تغییر دامنه، مشاهده مشخصات یا آپدیت دستور زیر را وارد کنید:${NC}"
echo -e "   ${PURPLE}${BOLD}hyperdns${NC}"
echo ""
