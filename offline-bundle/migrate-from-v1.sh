#!/usr/bin/env bash
# ==============================================================================
# JozmozDNS v1 → v2.2.0 MIGRATION SCRIPT
#
# Run this on a server that still carries an old JozmozDNS 1.x install.
# It removes EVERY trace of the old install, then hands over to the v2.2.0
# offline installer sitting in the same directory as this script.
#
#   bash migrate-from-v1.sh [-y] [--no-backup]
#
#   -y            skip the typed confirmation (still safe: v1 data is
#                 archived to /root before anything is deleted, and an
#                 archive failure aborts the run)
#   --no-backup   do not archive the old install to /root (faster, and
#                 irreversible — the old database is NOT v2-compatible)
#
# What v1.x touched (swept here, learned from the v1.2 installers):
#   - systemd unit: /etc/systemd/system/JozmozDNS.service (+ any copy in
#     /usr/lib/systemd/system)
#   - systemd-resolved drop-in: /etc/systemd/resolved.conf.d/JozmozDNS.conf
#   - CLI: /usr/local/bin/jmdns (+ /usr/bin/jmdns, /usr/local/bin/JozmozDNS,
#     /usr/bin/JozmozDNS)
#   - install tree: /opt/JozmozDNS (binary, config.json, certs, backups, logs)
#   - extras a hand-run v1 may have left: /etc/JozmozDNS, /var/log/JozmozDNS,
#     cron lines mentioning JozmozDNS, still-running processes
#
# What it does NOT do:
#   - it does not migrate v1 data. v1's database format is not v2-compatible
#     (v2 is BoltDB with AES-256-GCM field sealing); subscribers must be
#     re-provisioned. The archive in /root is the only copy of the old data.
#   - it does not delete firewall rules (it prints any it finds, you decide).
# ==============================================================================

set -u

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'
say()  { printf "%b\n" "$1"; }
ok()   { printf "  ${GREEN}✓${NC} %b\n" "$1"; }
warn() { printf "  ${YELLOW}!${NC} %b\n" "$1"; }
step() { printf "\n${CYAN}${BOLD}[%s] %s${NC}\n" "$1" "$2"; }

ASSUME_YES=0
NO_BACKUP=0
for arg in "$@"; do
    case "$arg" in
        -y|--yes)      ASSUME_YES=1 ;;
        --no-backup)   NO_BACKUP=1 ;;
        *) say "${RED}unknown argument: $arg${NC}"; exit 1 ;;
    esac
done

if [ "$(id -u)" -ne 0 ]; then
    say "${RED}Error: run as root (or with sudo).${NC}"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ ! -f "${SCRIPT_DIR}/install.sh" ] || [ ! -f "${SCRIPT_DIR}/JozmozDNS" ]; then
    say "${RED}Error: install.sh and the JozmozDNS binary must sit next to this script.${NC}"
    say "Put this file inside the v2.2.0 offline bundle directory and run it from there."
    exit 1
fi

# ── What did this server actually carry? ─────────────────────────────────────
step "1/8" "Detecting the old install"

FOOTPRINT=()
systemctl list-unit-files 2>/dev/null | grep -qi '^JozmozDNS' \
    && FOOTPRINT+=("systemd unit: $(systemctl list-unit-files | grep -i '^JozmozDNS' | awk '{print $1}' | tr '\n' ' ')")
[ -f /etc/systemd/system/JozmozDNS.service ] && FOOTPRINT+=("/etc/systemd/system/JozmozDNS.service")
[ -f /usr/lib/systemd/system/JozmozDNS.service ] && FOOTPRINT+=("/usr/lib/systemd/system/JozmozDNS.service")
[ -f /etc/systemd/resolved.conf.d/JozmozDNS.conf ] && FOOTPRINT+=("systemd-resolved drop-in")
[ -d /opt/JozmozDNS ] && FOOTPRINT+=("/opt/JozmozDNS")
for cli in /usr/local/bin/jmdns /usr/bin/jmdns /usr/local/bin/JozmozDNS /usr/bin/JozmozDNS; do
    [ -e "$cli" ] && FOOTPRINT+=("$cli")
done
[ -d /etc/JozmozDNS ] && FOOTPRINT+=("/etc/JozmozDNS")
[ -d /var/log/JozmozDNS ] && FOOTPRINT+=("/var/log/JozmozDNS")
for f in $(grep -rls JozmozDNS /etc/cron.d /var/spool/cron/crontabs /var/spool/cron/root 2>/dev/null); do
    FOOTPRINT+=("cron: $f")
done
OLD_VERSION="unknown"
[ -x /opt/JozmozDNS/JozmozDNS ] && OLD_VERSION="$(/opt/JozmozDNS/JozmozDNS -version 2>/dev/null || echo 'unknown')"

if [ "${#FOOTPRINT[@]}" -eq 0 ]; then
    say "${YELLOW}No JozmozDNS v1 footprint found on this server. Nothing to remove.${NC}"
    say "Jumping straight to the v2.2.0 installer."
else
    say "Found ${BOLD}${#FOOTPRINT[@]}${NC} traces of an old install (version: ${OLD_VERSION}):"
    for item in "${FOOTPRINT[@]}"; do say "  - $item"; done
fi

if pgrep -x JozmozDNS >/dev/null 2>&1; then
    warn "a JozmozDNS process is RUNNING right now:"
    pgrep -ax JozmozDNS | sed 's/^/    /'
fi

# ── Confirmation + archive ───────────────────────────────────────────────────
if [ "$ASSUME_YES" -ne 1 ]; then
    say ""
    say "${RED}${BOLD}This will DELETE every trace listed above and install JozmozDNS v2.2.0.${NC}"
    say "The old database is NOT compatible with v2 — subscribers start from scratch."
    printf "%b" "${YELLOW}Type MIGRATE to continue: ${NC}"
    read -r CONFIRM
    [ "$CONFIRM" = "MIGRATE" ] || { say "${YELLOW}Cancelled. Nothing was modified.${NC}"; exit 0; }
fi

if [ "$NO_BACKUP" -ne 1 ]; then
    step "2/8" "Archiving the old install to /root"
    # Stop the daemon first: data.db is a BoltDB file the running service
    # writes continuously, and a tar taken mid-commit is not a consistent copy.
    systemctl stop JozmozDNS 2>/dev/null && ok "JozmozDNS service stopped for a consistent archive"
    pkill -x JozmozDNS 2>/dev/null && warn "leftover JozmozDNS processes killed" || true
    sleep 1
    # Archive EVERYTHING this script is about to delete — not just /opt: a
    # hand-maintained /etc/JozmozDNS or stray logs are irreversible losses too,
    # and the archive-then-delete contract has to cover the full deletion set.
    ARCHIVE="/root/JozmozDNS-v1-remnant-$(date +%Y%m%d_%H%M%S).tar.gz"
    tar czf "$ARCHIVE" -C / \
        $([ -d /opt/JozmozDNS ] && echo opt/JozmozDNS) \
        $([ -d /etc/JozmozDNS ] && echo etc/JozmozDNS) \
        $([ -d /var/log/JozmozDNS ] && echo var/log/JozmozDNS) \
        2>/dev/null || true
    if [ -s "$ARCHIVE" ]; then
        ok "archived to ${ARCHIVE} (keep it until the new install is verified)"
    else
        # The wipe below is irreversible; with no archive it must not run.
        say "${RED}ARCHIVE FAILED — ${ARCHIVE} is empty or missing (is /root full?).${NC}"
        say "${RED}The old install was NOT touched and will NOT be deleted. Re-run after freeing space.${NC}"
        exit 1
    fi
else
    systemctl stop JozmozDNS 2>/dev/null || true
    pkill -x JozmozDNS 2>/dev/null || true
fi

# ── Stop everything ──────────────────────────────────────────────────────────
step "3/8" "Stopping services and processes"
systemctl stop JozmozDNS 2>/dev/null    && ok "JozmozDNS service stopped"
systemctl disable JozmozDNS 2>/dev/null && ok "JozmozDNS service disabled"
pkill -x JozmozDNS 2>/dev/null && ok "leftover JozmozDNS processes killed" || true
sleep 1
pgrep -x JozmozDNS >/dev/null 2>&1 && { pkill -9 -x JozmozDNS 2>/dev/null || true; warn "had to force-kill"; } || ok "no JozmozDNS process left"

# ── Units ────────────────────────────────────────────────────────────────────
step "4/8" "Removing systemd units"
rm -f /etc/systemd/system/JozmozDNS.service
rm -f /usr/lib/systemd/system/JozmozDNS.service
systemctl daemon-reload 2>/dev/null || true
systemctl reset-failed JozmozDNS 2>/dev/null || true
ok "units removed, daemon reloaded"

# ── Resolver ─────────────────────────────────────────────────────────────────
step "5/8" "Restoring the system DNS resolver"
rm -f /etc/systemd/resolved.conf.d/JozmozDNS.conf
if systemctl is-active --quiet systemd-resolved 2>/dev/null; then
    systemctl restart systemd-resolved 2>/dev/null || true
    ok "systemd-resolved restarted on port 53"
else
    warn "systemd-resolved not active — if port 53 is still held, that is another resolver, check: ss -ulnp | grep :53"
fi

# ── Files ────────────────────────────────────────────────────────────────────
step "6/8" "Removing binaries, CLI links, configs and logs"
rm -rf /opt/JozmozDNS                 && ok "/opt/JozmozDNS removed"
rm -rf /etc/JozmozDNS 2>/dev/null     && ok "/etc/JozmozDNS removed"
rm -rf /var/log/JozmozDNS 2>/dev/null && ok "/var/log/JozmozDNS removed"
for cli in /usr/local/bin/jmdns /usr/bin/jmdns /usr/local/bin/JozmozDNS /usr/bin/JozmozDNS; do
    rm -f "$cli"
done
ok "jmdns / JozmozDNS CLI links removed"

# cron lines that reference JozmozDNS (v1 never installed any, but a hand-run may have)
for f in /etc/cron.d/JozmozDNS /var/spool/cron/root /var/spool/cron/crontabs/root; do
    if [ -f "$f" ] && grep -q JozmozDNS "$f" 2>/dev/null; then
        sed -i '/JozmozDNS/d' "$f" && ok "JozmozDNS cron lines removed from $f"
    fi
done

# ── Verification ─────────────────────────────────────────────────────────────
step "7/8" "Verifying the sweep"
LEFT=0
systemctl is-active --quiet JozmozDNS 2>/dev/null && { warn "JozmozDNS service STILL ACTIVE"; LEFT=1; }
[ -d /opt/JozmozDNS ] && { warn "/opt/JozmozDNS still present"; LEFT=1; }
for cli in /usr/local/bin/jmdns /usr/bin/jmdns /usr/local/bin/JozmozDNS /usr/bin/JozmozDNS; do
    [ -e "$cli" ] && { warn "$cli still present"; LEFT=1; }
done
pgrep -x JozmozDNS >/dev/null 2>&1 && { warn "a JozmozDNS process is still alive"; LEFT=1; }
if [ "$LEFT" -eq 0 ]; then
    ok "no JozmozDNS trace left on the system"
fi

# Firewall rules are printed, not deleted — the operator decides.
if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -qiE 'active'; then
    RULES="$(ufw status 2>/dev/null | grep -E '^(53|80|443|853|8443)(/tcp)? ' || true)"
    [ -n "$RULES" ] && { say ""; warn "ufw rules for the JozmozDNS ports (left in place, remove by hand if unwanted):"; echo "$RULES" | sed 's/^/    /'; }
fi

# ── Install v2.2.0 ───────────────────────────────────────────────────────────
step "8/8" "Installing JozmozDNS v2.2.0"
cd "$SCRIPT_DIR" || exit 1
chmod +x install.sh JozmozDNS 2>/dev/null || true
say "Handing over to the v2.2.0 installer (it asks for the panel domain —"
say "HTTPS is mandatory and the daemon issues its own certificate)."
say ""
exec bash ./install.sh
