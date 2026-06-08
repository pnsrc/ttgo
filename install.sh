#!/usr/bin/env bash
#
# TrustTunnel endpoint installer.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/pnsrc/ttgo/main/install.sh | bash
# or with options:
#   bash install.sh [--prefix /opt/trusttunnel_endpoint] [--no-systemd] [--no-setup]

set -euo pipefail

REPO_URL="https://github.com/pnsrc/ttgo.git"
PREFIX="/opt/trusttunnel_endpoint"
INSTALL_SYSTEMD=1
RUN_SETUP=1
BUILD_DIR=""

# ── arg parsing ───────────────────────────────────────────────────────────────

while [[ $# -gt 0 ]]; do
    case "$1" in
        --prefix)       PREFIX="$2"; shift 2 ;;
        --no-systemd)   INSTALL_SYSTEMD=0; shift ;;
        --no-setup)     RUN_SETUP=0; shift ;;
        --build-dir)    BUILD_DIR="$2"; shift 2 ;;
        -h|--help)
            cat <<EOF
TrustTunnel endpoint installer.

Options:
  --prefix DIR      Install location (default: /opt/trusttunnel_endpoint)
  --no-systemd      Skip systemd service installation
  --no-setup        Skip ttadmin setup wizard at the end
  --build-dir DIR   Use existing source dir instead of cloning
  -h, --help        Show this help

Examples:
  curl -fsSL https://raw.githubusercontent.com/pnsrc/ttgo/main/install.sh | bash
  bash install.sh --prefix /usr/local/trusttunnel --no-systemd
  bash install.sh --build-dir .       # build from local clone

EOF
            exit 0
            ;;
        *) echo "unknown option: $1" >&2; exit 1 ;;
    esac
done

# ── helpers ───────────────────────────────────────────────────────────────────

log() { printf "\033[1;32m==>\033[0m %s\n" "$*"; }
warn() { printf "\033[1;33m==>\033[0m %s\n" "$*" >&2; }
fail() { printf "\033[1;31m==>\033[0m %s\n" "$*" >&2; exit 1; }

require_cmd() {
    command -v "$1" >/dev/null 2>&1 || fail "$1 is required but not installed"
}

# ── prerequisites ─────────────────────────────────────────────────────────────

log "Checking prerequisites..."
require_cmd git
require_cmd go

GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
log "Found Go $GO_VERSION"

if [[ "$EUID" -ne 0 ]]; then
    warn "Not running as root. Installation to $PREFIX may fail."
    warn "Consider re-running with sudo."
fi

# ── fetch sources ─────────────────────────────────────────────────────────────

if [[ -z "$BUILD_DIR" ]]; then
    BUILD_DIR="$(mktemp -d -t ttgo-build.XXXXXX)"
    trap 'rm -rf "$BUILD_DIR"' EXIT
    log "Cloning $REPO_URL into $BUILD_DIR"
    git clone --depth 1 "$REPO_URL" "$BUILD_DIR"
fi

cd "$BUILD_DIR"

# ── build ─────────────────────────────────────────────────────────────────────

log "Building binaries..."
go build -ldflags="-s -w" -o trusttunnel_endpoint ./cmd/endpoint
go build -ldflags="-s -w" -o ttadmin ./cmd/ttadmin

# ── install ───────────────────────────────────────────────────────────────────

log "Installing to $PREFIX"
mkdir -p "$PREFIX"
install -m 755 trusttunnel_endpoint "$PREFIX/trusttunnel_endpoint"
install -m 755 ttadmin "$PREFIX/ttadmin"

# Symlink ttadmin into PATH for convenience
if [[ -d /usr/local/bin ]]; then
    ln -sf "$PREFIX/ttadmin" /usr/local/bin/ttadmin
    log "Symlinked ttadmin → /usr/local/bin/ttadmin"
fi

# ── systemd ───────────────────────────────────────────────────────────────────

if [[ "$INSTALL_SYSTEMD" -eq 1 ]] && command -v systemctl >/dev/null 2>&1; then
    log "Installing systemd service..."
    cat > /etc/systemd/system/trusttunnel-endpoint.service <<EOF
[Unit]
Description=TrustTunnel Endpoint
After=network.target

[Service]
ExecStart=$PREFIX/trusttunnel_endpoint $PREFIX/vpn.toml $PREFIX/hosts.toml
WorkingDirectory=$PREFIX
Restart=on-failure
RestartSec=5
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF
    systemctl daemon-reload
    log "Systemd service installed (not started yet — run setup first)"
fi

# ── setup wizard ──────────────────────────────────────────────────────────────

if [[ "$RUN_SETUP" -eq 1 ]]; then
    if [[ -f "$PREFIX/vpn.toml" ]]; then
        log "Config already exists at $PREFIX/vpn.toml — skipping wizard"
        log "Run setup manually:  ttadmin setup --vpn $PREFIX/vpn.toml --hosts $PREFIX/hosts.toml --creds $PREFIX/credentials.toml"
    else
        log "Launching setup wizard..."
        "$PREFIX/ttadmin" setup \
            --vpn   "$PREFIX/vpn.toml" \
            --hosts "$PREFIX/hosts.toml" \
            --creds "$PREFIX/credentials.toml"
    fi
fi

# ── done ──────────────────────────────────────────────────────────────────────

log "Installation complete."
echo
echo "Binaries:"
echo "  $PREFIX/trusttunnel_endpoint"
echo "  $PREFIX/ttadmin"
echo
if [[ "$INSTALL_SYSTEMD" -eq 1 ]] && command -v systemctl >/dev/null 2>&1; then
    echo "Start the server:"
    echo "  systemctl enable --now trusttunnel-endpoint"
    echo "  systemctl status trusttunnel-endpoint"
    echo
fi
echo "Manage the server:"
echo "  ttadmin --vpn $PREFIX/vpn.toml --hosts $PREFIX/hosts.toml"
