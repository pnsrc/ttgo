// Thin wrapper over fetch с авторизацией через bearer token.

const TOKEN_KEY = "ttgo_admin_token";
const ENDPOINT_KEY = "ttgo_admin_endpoint";

export type Session = {
  username: string;
  remote_addr: string;
  connected_at: string;
  bytes_in: number;
  bytes_out: number;
  open_tunnels: number;
  total_tunnels: number;
};

export type User = {
  username: string;
  max_devices: number;
  enabled: boolean;
  expires_at: number; // unix seconds, 0 = never
  traffic_limit: number; // bytes, 0 = unlimited
  traffic_used: number; // bytes
  active_conns: number;
  bytes_in: number;
  bytes_out: number;
  total_tunnels: number;
};

export type Rule = {
  cidr: string;
  client_random_prefix: string;
  action: "allow" | "deny";
};

export type RulesResponse = {
  path: string;
  rules: Rule[];
};

export type SystemInfo = {
  go_version: string;
  os: string;
  arch: string;
  goroutines: number;
  num_cpu: number;
  heap_alloc_bytes: number;
  heap_sys_bytes: number;
  total_alloc_bytes: number;
  gc_runs: number;
  active_sessions: number;
  active_users: number;
};

export type WhoAmI = {
  ok: boolean;
  version: string;
  has_userstore: boolean;
};

export type HostInfo = {
  hostname: string;
  not_after: string;
  issuer: string;
};

export type FullConfig = {
  listen_address: string;
  store_type: string;
  store_dsn: string;
  cache_ttl_secs: number;
  allow_private_net: boolean;
  ipv6_available: boolean;
  tcp_idle_timeout_secs: number;
  udp_idle_timeout_secs: number;
  connect_timeout_secs: number;
  tls_handshake_timeout_secs: number;
  http3_enabled: boolean;
  icmp_enabled: boolean;
  hostnames: HostInfo[];
};

export type ServerInfo = {
  version: string;
  uptime_seconds: number;
  started_at: string;
  has_userstore: boolean;
  has_tls_reload: boolean;
  config: FullConfig | null;
};

function token(): string {
  return localStorage.getItem(TOKEN_KEY) ?? "";
}

function endpoint(): string {
  return localStorage.getItem(ENDPOINT_KEY) ?? "";
}

export function setAuth(ep: string, tok: string) {
  // empty endpoint → use same-origin (server раздаёт фронт)
  localStorage.setItem(ENDPOINT_KEY, ep);
  localStorage.setItem(TOKEN_KEY, tok);
}

export function clearAuth() {
  localStorage.removeItem(TOKEN_KEY);
  localStorage.removeItem(ENDPOINT_KEY);
}

export function isAuthed(): boolean {
  return !!token();
}

async function req<T>(method: string, path: string, body?: unknown): Promise<T> {
  const url = endpoint() ? endpoint().replace(/\/$/, "") + path : path;
  const res = await fetch(url, {
    method,
    headers: {
      "Content-Type": "application/json",
      Authorization: "Bearer " + token(),
    },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    throw new Error(`${res.status} ${res.statusText}: ${(await res.text()).slice(0, 200)}`);
  }
  const txt = await res.text();
  return txt ? JSON.parse(txt) : (undefined as T);
}

export const api = {
  whoami: () => req<WhoAmI>("GET", "/api/whoami"),
  info: () => req<ServerInfo>("GET", "/api/info"),
  system: () => req<SystemInfo>("GET", "/api/system"),
  reloadTLS: () => req<void>("POST", "/api/tls/reload"),

  rules: () => req<RulesResponse>("GET", "/api/rules"),
  saveRules: (rules: Rule[]) => req<void>("PUT", "/api/rules", { rules }),

  patchConfig: (patch: Record<string, unknown>) =>
    req<{ path: string; status: string }>("PATCH", "/api/config", patch),
  restart: () => req<{ status: string }>("POST", "/api/restart"),

  sessions: () => req<Session[]>("GET", "/api/sessions"),
  kickSession: (username: string, remote: string) =>
    req<void>("POST", `/api/sessions/kick?username=${encodeURIComponent(username)}&remote_addr=${encodeURIComponent(remote)}`),

  users: () => req<User[]>("GET", "/api/users"),
  addUser: (
    username: string,
    password: string,
    max_devices = 0,
    expires_at = 0,
    traffic_limit = 0
  ) => req<void>("POST", "/api/users", { username, password, max_devices, expires_at, traffic_limit }),
  deleteUser: (username: string) =>
    req<void>("DELETE", `/api/users/${encodeURIComponent(username)}`),
  patchUser: (
    username: string,
    body: {
      password?: string;
      max_devices?: number;
      enabled?: boolean;
      expires_at?: number;
      traffic_limit?: number;
      reset_traffic?: boolean;
    }
  ) => req<void>("PATCH", `/api/users/${encodeURIComponent(username)}`, body),
  kickUser: (username: string) =>
    req<void>("POST", `/api/users/${encodeURIComponent(username)}/kick`),
};

// Human-readable byte formatter
export function fmtBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  const units = ["KB", "MB", "GB", "TB"];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(v < 10 ? 2 : 1)} ${units[i]}`;
}

// Duration in seconds → human-readable
export function fmtDuration(secs: number): string {
  if (secs < 60) return `${secs}s`;
  if (secs < 3600) return `${Math.floor(secs / 60)}m ${secs % 60}s`;
  if (secs < 86400) return `${Math.floor(secs / 3600)}h ${Math.floor((secs % 3600) / 60)}m`;
  return `${Math.floor(secs / 86400)}d ${Math.floor((secs % 86400) / 3600)}h`;
}

// Uptime from ISO timestamp
export function fmtUptime(iso: string): string {
  const ms = Date.now() - new Date(iso).getTime();
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${s % 60}s`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ${m % 60}m`;
  const d = Math.floor(h / 24);
  return `${d}d ${h % 24}h`;
}
