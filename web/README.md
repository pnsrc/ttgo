# ttgo-webui

React + Vite + Tailwind UI for the TrustTunnel admin API.

## Dev

```bash
cd web
npm install
npm run dev
```

The dev server runs on `http://localhost:5173` and proxies `/api/*` to `http://127.0.0.1:9090`. Make sure the endpoint is running locally with the admin API enabled, or change the proxy target in `vite.config.ts`.

To sign in, paste the token from `[admin].token` in `vpn.toml`. Leave the endpoint URL empty when running on the same origin as the API; in dev mode the empty value works thanks to the vite proxy.

## Build

```bash
npm run build
```

Outputs static files to `web/dist`. Point the endpoint at this directory by setting `[admin].web_root` in `vpn.toml`:

```toml
[admin]
address  = "0.0.0.0:9090"
token    = "..."
web_root = "/opt/trusttunnel_endpoint/web/dist"
```

Then visit `http://your-server:9090/`.

## Features

- Sessions tab — live table of TLS connections (auto-refresh 3s), kick one or all sessions of a user
- Users tab — CRUD, set device limits, view per-user lifetime traffic, password change
