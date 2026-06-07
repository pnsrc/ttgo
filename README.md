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

Requires Go 1.22+. No CGO required.

```
make build         # endpoint binary
make build-admin   # ttadmin binary
make build-linux   # both, cross-compiled for Linux amd64
```

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
A        Add user / add certificate
D        Delete user
P        Change password
R        Reload TLS (SIGHUP) / refresh
Ctrl+C   Quit
```

## Admin API

When configured, the endpoint exposes a local HTTP API:

```
POST /users/kick?username=X    invalidate cache and send GOAWAY to active connections
GET  /users/active             map of username to active connection count
```

Requests require `Authorization: Bearer <token>`.

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
