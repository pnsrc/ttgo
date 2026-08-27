# ttgo

Go implementation of the [TrustTunnel](https://github.com/TrustTunnel/TrustTunnel) proxy protocol endpoint, with several extensions beyond the original specification.

## What it is

TrustTunnel is a VPN protocol that tunnels traffic as HTTP CONNECT requests over TLS (HTTP/2 or HTTP/3). The client authenticates with Basic Auth per connection, and all TCP, UDP, and ICMP traffic is forwarded by the endpoint.

This repository contains the server-side endpoint implementation in Go.

## Differences from the original TrustTunnel endpoint

### User authentication

The original endpoint reads credentials from a single file and requires a restart to apply changes. This implementation supports three backends:

- **file** - credentials.toml, auto-reloaded every 10 seconds without restart
- **sqlite** - pure Go SQLite, no CGO dependency, queried on demand
- **postgres** - standard database/sql with connection pooling

All backends share a configurable in-memory cache with TTL and negative caching to avoid hitting the store on every request. The cache supports per-user invalidation.

### Hot user management without disconnects

When a user is deleted, the server sends an HTTP/2 GOAWAY frame (error code 0x1F) to all active connections belonging to that user, then closes them. This terminates the session immediately without restarting the server or affecting other users.

Adding a new user takes effect within the cache TTL (default 30 seconds) with no impact on existing sessions.

### TLS hot reload

Sending SIGHUP to the endpoint process reloads TLS certificates and the hosts configuration without dropping any existing connections. The TLS manager swaps the certificate map under a read-write lock.

### ICMP proxy

The endpoint implements a custom extension for ICMP echo forwarding using a single raw socket shared across all sessions. Clients can route ping traffic through the endpoint. This is not part of the base TrustTunnel protocol.

### Traffic rules

An optional rules engine filters connections by client IP (CIDR) and TLS ClientHello random prefix with bitwise mask. Rules are evaluated before proxying. This allows distinguishing clients at the TLS layer before authentication, which is useful when the ClientHello random is pre-configured on the client side.

### UDP session isolation

UDP sessions are tracked per TLS connection rather than globally. When a connection closes, all associated UDP sockets are released immediately. Each client gets an isolated UDP namespace with no cross-session leakage.

### HTTP/3 support

The endpoint can serve HTTP/2 and HTTP/3 (QUIC) simultaneously on the same port. Alt-Svc headers are added automatically when QUIC is enabled.

### Admin tool

`ttadmin` is a terminal UI for managing the running endpoint:

- **setup** - interactive wizard to generate vpn.toml, hosts.toml, credentials
- **manage** - user CRUD, certificate management with Let's Encrypt or self-signed, server status
- **migrate** - migrate users from credentials.toml to SQLite, patches vpn.toml automatically

The admin tool detects the configured store type from vpn.toml and operates against the correct backend.

## Binaries

```
trusttunnel_endpoint  - the proxy server
ttadmin               - admin TUI
ttclient              - desktop VPN client (macOS, Windows)
```

## Quick install

One-line install on a fresh Linux server (requires Go and git):

```bash
curl -fsSL https://raw.githubusercontent.com/pnsrc/ttgo/main/install.sh | sudo bash
```

The script clones the repo into a temp dir, builds both binaries, installs them to `/opt/trusttunnel_endpoint`, sets up a systemd service, symlinks `ttadmin` into `/usr/local/bin`, and launches the setup wizard.

Options:

```bash
# Custom install path
curl -fsSL https://raw.githubusercontent.com/pnsrc/ttgo/main/install.sh | sudo bash -s -- --prefix /usr/local/trusttunnel

# Skip systemd / wizard (useful for containers)
curl -fsSL https://raw.githubusercontent.com/pnsrc/ttgo/main/install.sh | sudo bash -s -- --no-systemd --no-setup

# Build from a local clone instead of fetching
sudo bash install.sh --build-dir .
```

After install:

```bash
systemctl enable --now trusttunnel-endpoint
ttadmin --vpn /opt/trusttunnel_endpoint/vpn.toml --hosts /opt/trusttunnel_endpoint/hosts.toml
```

## Uninstall

```bash
curl -fsSL https://raw.githubusercontent.com/pnsrc/ttgo/main/uninstall.sh | sudo bash
```

By default this stops the systemd service, removes binaries and the symlink, and keeps configs/users.db/certs in case you reinstall. To wipe everything:

```bash
curl -fsSL https://raw.githubusercontent.com/pnsrc/ttgo/main/uninstall.sh | sudo bash -s -- --purge --yes
```

## Configuration

The server takes two config files as positional arguments:

```
trusttunnel_endpoint [flags] <vpn.toml> <hosts.toml>
```

### vpn.toml

```toml
listen_address = "0.0.0.0:443"
ipv6_available = false
allow_private_network_connections = false

tls_handshake_timeout_secs = 10
client_listener_timeout_secs = 600
connection_establishment_timeout_secs = 30
tcp_connections_timeout_secs = 604800
udp_connections_timeout_secs = 300

# User store: "file", "sqlite", or "postgres"
store_type = "sqlite"
store_dsn  = "users.db"

# For file store only
credentials_file = "credentials.toml"

rules_file = ""
auth_failure_status_code = 407
cache_ttl_secs = 30

[listen_protocols.http2]
max_concurrent_streams = 1000
initial_stream_window_size = 131072
max_frame_size = 16384

# Optional QUIC/HTTP3
# [listen_protocols.quic]

# Optional admin API (localhost only)
[admin]
address = "127.0.0.1:9090"
token   = "secret"
```

### hosts.toml

```toml
[[main_hosts]]
hostname         = "example.com"
cert_chain_path  = "/etc/certs/example.com.crt"
private_key_path = "/etc/certs/example.com.key"

[[ping_hosts]]
hostname         = "ping.example.com"
cert_chain_path  = "/etc/certs/example.com.crt"
private_key_path = "/etc/certs/example.com.key"
```

### credentials.toml (file store)

```toml
[[client]]
username = "alice"
password = "hunter2"
```

## Building

Requires Go 1.22+. No CGO required for server binaries.

```
make build         # endpoint binary
make build-admin   # ttadmin binary
make build-linux   # both, cross-compiled for Linux amd64
```

For a full install including systemd and the setup wizard, use `install.sh`:

```bash
sudo bash install.sh --build-dir .
```

### Building ttclient (desktop VPN client)

ttclient is a native desktop app built with [Wails v2](https://wails.io) (Go + WebView). It supports macOS and Windows.

**Prerequisites (all platforms):**

- Go 1.22+ — https://go.dev/dl/
- Node.js 18+ — https://nodejs.org
- Wails CLI:
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```

**macOS:**

```bash
cd cmd/ttclient
wails build
```

The `.app` bundle will be in `build/bin/`. Requires root for TUN — the app auto-elevates via osascript on launch.

**Windows:**

1. Install prerequisites above
2. Verify Wails dependencies:
   ```bash
   wails doctor
   ```
   WebView2 Runtime is required (pre-installed on Windows 10 21H2+ and Windows 11; if missing, download from https://developer.microsoft.com/en-us/microsoft-edge/webview2/)
3. Download **wintun.dll** (amd64) from https://www.wintun.net and place it in `cmd/ttclient/`
4. Build:
   ```bash
   cd cmd/ttclient
   wails build
   ```
5. Copy `wintun.dll` next to `build/bin/ttclient.exe`

The app auto-elevates via UAC on launch (Administrator required for TUN adapter).

**Cross-compile Windows from macOS** (requires `brew install mingw-w64`):

```bash
cd cmd/ttclient
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
  go build -tags desktop -o ttclient.exe -ldflags="-s -w -H windowsgui" .
```

**Logs:** `%LOCALAPPDATA%\ttclient\ttclient.log` (Windows) or `~/.config/ttclient/ttclient.log` (macOS/Linux).

**Hotkeys:** Cmd+Shift+V (macOS) / Ctrl+Shift+V (Windows) — toggle connect/disconnect.

**Profiles:** standard TrustTunnel `.toml` format. Import via UI or drop files into `~/.config/ttclient/profiles/` (macOS) / `%APPDATA%\ttclient\profiles\` (Windows).

## Admin tool

```
# First-time setup
ttadmin setup --vpn vpn.toml --hosts hosts.toml --creds credentials.toml

# Migrate users from file to SQLite
ttadmin migrate --vpn vpn.toml --hosts hosts.toml --creds credentials.toml

# Management TUI
ttadmin --vpn vpn.toml --hosts hosts.toml --creds credentials.toml
```

Keys in the management TUI:

```
1        Users tab
2        Certificates tab
3        Status tab
1        Users tab (CRUD + device limits + traffic)
2        Sessions tab (live connections, kick)
3        Certificates tab
4        Status tab
A        Add user / add certificate
D        Delete user
P        Change password
L        Set device limit (on Users tab)
k        Kick selected session (on Sessions tab)
K        Kick all sessions of selected user
R        Reload TLS (SIGHUP) / refresh
Ctrl+C   Quit
```

## Per-user features

### Device limits

Each user has an optional `max_devices` cap. When a user already has N active TLS connections and tries to open another, the new request gets a `407 Proxy Authentication Required` with `X-Revoke-Reason: Device limit reached` and a human-readable message. Set the limit through the TUI (`L` key on Users tab) or directly in the store. `0` means unlimited (default).

### P2P relay

The endpoint exposes a special pseudo-host `_p2p` that pairs two authenticated TCP streams of the same user in a named room. This is used by `ttadmin tunnel` to forward TCP services between devices through the server, without any open ports or NAT traversal on either side.

## Tunneling between devices

`ttadmin tunnel` builds an end-to-end TCP forwarder over the server. Two devices logged in as the same user join a shared room name — the server connects their streams. Useful for reaching a home machine from anywhere, exposing a router admin page, RDP, SSH, etc.

```
ttadmin tunnel expose --server HOST:PORT --user X --pass Y --room NAME --target HOST:PORT
ttadmin tunnel reach  --server HOST:PORT --user X --pass Y --room NAME --local  HOST:PORT
```

Common flags:

```
--server HOST:PORT     TrustTunnel endpoint address
--user NAME            TrustTunnel username
--pass PASSWORD        TrustTunnel password
--room NAME            shared room (must match on both sides)
--insecure             skip TLS verification (self-signed cert)
--hostname NAME        override SNI / Host header
```

`expose` only:

```
--target HOST:PORT     local service to make reachable
--pool N               number of pending listen sessions (default 4)
```

`reach` only:

```
--local HOST:PORT      local listen address
```

Example — SSH to a home PC from a laptop:

```bash
# On home PC
ttadmin tunnel expose --server my.endpoint:443 --user alice --pass secret \
                      --room ssh --target 127.0.0.1:22

# On laptop
ttadmin tunnel reach  --server my.endpoint:443 --user alice --pass secret \
                      --room ssh --local 127.0.0.1:2222

ssh -p 2222 localhost   # reaches the home PC through the server
```

Both sides must authenticate as the same user. The server will not bridge different users sharing the same room name. Each P2P stream counts as a connection against the user's `max_devices` limit.

For other platforms:

```
make build-mac        # darwin/arm64 + darwin/amd64
make build-android    # android/arm64 (e.g. Termux)
```

## Admin API

When configured, the endpoint exposes a local HTTP API:

```
POST /users/kick?username=X&reason=Z         invalidate cache, mark sessions revoked, GOAWAY in 2s
POST /sessions/kick?username=X&remote_addr=Y kick a single TLS connection
GET  /users/active                           map of username to active connection count
GET  /sessions                               list of all active TLS sessions with traffic counters
GET  /stats                                  per-user lifetime traffic and connection stats
```

All requests require `Authorization: Bearer <token>`.

## systemd

```ini
[Unit]
Description=TrustTunnel Endpoint
After=network.target

[Service]
ExecStart=/opt/trusttunnel_endpoint/trusttunnel_endpoint /opt/trusttunnel_endpoint/vpn.toml /opt/trusttunnel_endpoint/hosts.toml
WorkingDirectory=/opt/trusttunnel_endpoint
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

TLS certificates and hosts can be reloaded at runtime:

```
systemctl kill -s HUP trusttunnel-endpoint
```
