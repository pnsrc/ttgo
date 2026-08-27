export type State = "disconnected" | "connecting" | "connected" | "error";

export type Snapshot = {
  state: State;
  error?: string;
  tun_name?: string;
  endpoint?: string;
  bytes_in: number;
  bytes_out: number;
  tunnels: number;
  active_conn: number;
  uptime_sec: number;
};

export type StatusPayload = {
  snapshot: Snapshot;
  active_profile_id: string;
};

export type Profile = {
  id: string;
  path: string;
  name: string;
  endpoint: string;
  username: string;
  toml: any; // ProfileTOML — frontend используется только для preview
};

export type Config = {
  endpoint: string;
  hostname?: string;
  username: string;
  password: string;
  insecure?: boolean;
};

export type GlobalSettings = {
  last_profile_id: string;
  bypass_domains: boolean;
  global_exclusions: string[] | null;
  language: string;
  auto_connect: boolean;
  enable_adblock: boolean;
  theme: string;
  upstream_dns: string;
  routing_mode: string;
};
