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

# 3. Detect Public IP
echo -e "${CYAN}[1/5] Detecting server environment and public IP...${NC}"
PUBLIC_IP=$(curl -4 -s --connect-timeout 4 https://api.ipify.org 2>/dev/null || \
            curl -4 -s --connect-timeout 4 https://ifconfig.me 2>/dev/null || \
            echo "127.0.0.1")
echo -e "  Server Public IP: ${GREEN}${PUBLIC_IP}${NC}"

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

# 6. Install HyperDNS Binary and Terminal Console
echo -e "\n${CYAN}[3/5] Installing HyperDNS binary and Terminal Console...${NC}"
if [ -f "./hyperdns-linux" ]; then
    cp -f "./hyperdns-linux" "$INSTALL_DIR/hyperdns"
else
    echo -e "  Downloading HyperDNS binary from GitHub..."
    curl -fsSL --progress-bar -o "$INSTALL_DIR/hyperdns" "${REPO_RAW}/hyperdns-linux"
fi
chmod +x "$INSTALL_DIR/hyperdns"

# Install management CLI menu
if [ -f "./scripts/hyperdns-menu.sh" ]; then
    cp -f "./scripts/hyperdns-menu.sh" "/usr/local/bin/hyperdns"
else
    curl -fsSL -o "/usr/local/bin/hyperdns" "${REPO_RAW}/scripts/hyperdns-menu.sh"
fi
chmod +x "/usr/local/bin/hyperdns"
ln -sf "/usr/local/bin/hyperdns" "/usr/local/bin/hdns"

# 7. Interactive Domain & SSL Configuration
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

# 8. Setup Systemd Service
echo -e "\n${CYAN}[4/5] Configuring systemd background service...${NC}"
cat <<EOF > /etc/systemd/system/hyperdns.service
[Unit]
Description=HyperDNS Standalone SmartDNS & Gaming Gateway
After=network.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/hyperdns -server
Restart=always
RestartSec=3
LimitNOFILE=65535
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload

# 9. Initial Bootstrap & Credentials Generation
echo -e "\n${CYAN}[5/5] Initializing HyperDNS and generating security credentials...${NC}"
systemctl stop "$SERVICE_NAME" 2>/dev/null || true

BOOTSTRAP_LOG="$INSTALL_DIR/install.log"
rm -f "$BOOTSTRAP_LOG"

if [ -n "$USER_DOMAIN" ]; then
    echo -e "  Configuring domain ${GREEN}${USER_DOMAIN}${NC} and generating SSL certificate..."
    timeout 15 "$INSTALL_DIR/hyperdns" -server -domain "$USER_DOMAIN" -email "$USER_EMAIL" > "$BOOTSTRAP_LOG" 2>&1 || true
else
    echo -e "  Configuring direct IP mode..."
    timeout 10 "$INSTALL_DIR/hyperdns" -server > "$BOOTSTRAP_LOG" 2>&1 || true
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
