#!/usr/bin/env bash
#
# TrustTunnel endpoint uninstaller.
#
# Usage:
#   sudo bash uninstall.sh [--prefix /opt/trusttunnel_endpoint] [--purge] [--yes]

set -euo pipefail

PREFIX="/opt/trusttunnel_endpoint"
PURGE=0
ASSUME_YES=0
SERVICE_NAME="trusttunnel-endpoint"

# ── arg parsing ───────────────────────────────────────────────────────────────

while [[ $# -gt 0 ]]; do
    case "$1" in
        --prefix)  PREFIX="$2"; shift 2 ;;
        --purge)   PURGE=1; shift ;;
        --yes|-y)  ASSUME_YES=1; shift ;;
        -h|--help)
            cat <<EOF
TrustTunnel endpoint uninstaller.

Options:
  --prefix DIR    Install location (default: /opt/trusttunnel_endpoint)
  --purge         Also remove config files, SQLite DB, certs (all data)
  --yes, -y       Don't prompt for confirmation
  -h, --help      Show this help

Default behavior keeps configs (vpn.toml, hosts.toml, users.db, certs)
in case you reinstall. Use --purge to wipe everything.

EOF
            exit 0
            ;;
        *) echo "unknown option: $1" >&2; exit 1 ;;
    esac
done

# ── helpers ───────────────────────────────────────────────────────────────────

log()  { printf "\033[1;32m==>\033[0m %s\n" "$*"; }
warn() { printf "\033[1;33m==>\033[0m %s\n" "$*" >&2; }
fail() { printf "\033[1;31m==>\033[0m %s\n" "$*" >&2; exit 1; }

if [[ "$EUID" -ne 0 ]]; then
    fail "Must run as root (use sudo)."
fi

# ── confirmation ──────────────────────────────────────────────────────────────

cat <<EOF

About to uninstall TrustTunnel endpoint:

  Prefix:        $PREFIX
  Systemd unit:  /etc/systemd/system/${SERVICE_NAME}.service
  Symlink:       /usr/local/bin/ttadmin

EOF
if [[ "$PURGE" -eq 1 ]]; then
    cat <<EOF
  PURGE MODE: configs, users database, and certs in $PREFIX will be DELETED.

EOF
else
    cat <<EOF
  Configs (vpn.toml, hosts.toml, users.db, certs) will be KEPT.
  Use --purge to remove them too.

EOF
fi

if [[ "$ASSUME_YES" -ne 1 ]]; then
    read -rp "Continue? [y/N] " ans
    case "$ans" in
        y|Y|yes|YES) ;;
        *) log "Aborted."; exit 0 ;;
    esac
fi

# ── stop and disable service ──────────────────────────────────────────────────

if command -v systemctl >/dev/null 2>&1; then
    if systemctl list-unit-files | grep -q "^${SERVICE_NAME}.service"; then
        log "Stopping and disabling ${SERVICE_NAME}..."
        systemctl stop "${SERVICE_NAME}.service" 2>/dev/null || true
        systemctl disable "${SERVICE_NAME}.service" 2>/dev/null || true
        rm -f "/etc/systemd/system/${SERVICE_NAME}.service"
        systemctl daemon-reload
    fi
fi

# Fallback: убиваем любые оставшиеся процессы
if pgrep -x trusttunnel_endpoint >/dev/null 2>&1; then
    log "Killing remaining trusttunnel_endpoint processes..."
    pkill -x trusttunnel_endpoint || true
    sleep 1
fi

# ── remove binaries and symlinks ──────────────────────────────────────────────

log "Removing binaries..."
rm -f "$PREFIX/trusttunnel_endpoint" "$PREFIX/ttadmin"

if [[ -L /usr/local/bin/ttadmin ]]; then
    log "Removing symlink /usr/local/bin/ttadmin"
    rm -f /usr/local/bin/ttadmin
fi

# ── purge or keep data ────────────────────────────────────────────────────────

if [[ "$PURGE" -eq 1 ]]; then
    log "Purging configs and data from $PREFIX..."
    rm -rf "$PREFIX"
else
    # Удаляем только пустую директорию если бинарники были единственным содержимым
    if [[ -d "$PREFIX" ]] && [[ -z "$(ls -A "$PREFIX")" ]]; then
        rmdir "$PREFIX"
    elif [[ -d "$PREFIX" ]]; then
        log "Kept data files in $PREFIX:"
        ls -la "$PREFIX"
    fi
fi

# ── iptables hint ─────────────────────────────────────────────────────────────

if command -v iptables >/dev/null 2>&1; then
    if iptables -t nat -L POSTROUTING -n 2>/dev/null | grep -q MASQUERADE; then
        warn "iptables NAT rules (MASQUERADE) are still active. Review with:"
        warn "  iptables -t nat -L POSTROUTING"
    fi
fi

log "Uninstall complete."
