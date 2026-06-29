import { useEffect, useState, useCallback } from "react";
import {
  ConnectProfile,
  Disconnect,
  Status,
  Profiles,
  ImportProfile,
  DeleteProfile,
  OpenProfilesDir,
} from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime/runtime";
import { StatusPayload, Snapshot, Profile } from "./types";
import {
  Logo,
  ArrowLeft,
  Power,
  Alert,
  Plus,
  Trash,
  Folder,
  Refresh,
  ChevronRight,
  Empty,
} from "./icons";

const DEFAULT_SNAP: Snapshot = {
  state: "disconnected",
  bytes_in: 0,
  bytes_out: 0,
  tunnels: 0,
  active_conn: 0,
  uptime_sec: 0,
};

export default function App() {
  const [snap, setSnap] = useState<Snapshot>(DEFAULT_SNAP);
  const [activeID, setActiveID] = useState("");
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [errToast, setErrToast] = useState<string | null>(null);

  const refreshProfiles = useCallback(async () => {
    try {
      const list = (await Profiles()) as unknown as Profile[];
      setProfiles(list ?? []);
    } catch (e: any) {
      setErrToast(String(e?.message ?? e));
    }
  }, []);

  useEffect(() => {
    Status().then((s: any) => {
      setSnap(s.snapshot);
      setActiveID(s.active_profile_id);
    }).catch(() => {});
    const unsub = EventsOn("status", (s: StatusPayload) => {
      setSnap(s.snapshot);
      setActiveID(s.active_profile_id);
    });
    refreshProfiles();
    return () => unsub();
  }, [refreshProfiles]);

  // Когда подключение активно, переключаем UI на detail-экран активного профиля
  useEffect(() => {
    if (activeID) setSelectedID(activeID);
  }, [activeID]);

  // Ошибка из snapshot → toast
  useEffect(() => {
    if (snap.state === "error" && snap.error) {
      setErrToast(snap.error);
      const t = setTimeout(() => setErrToast(null), 6000);
      return () => clearTimeout(t);
    }
  }, [snap.state, snap.error]);

  async function importProfile() {
    try {
      const p = (await ImportProfile()) as unknown as Profile | null;
      if (p) {
        await refreshProfiles();
        setSelectedID(p.id);
      }
    } catch (e: any) {
      setErrToast(String(e?.message ?? e));
    }
  }

  async function removeProfile(id: string) {
    if (!confirm("Delete this profile?")) return;
    try {
      await DeleteProfile(id);
      if (selectedID === id) setSelectedID(null);
      await refreshProfiles();
    } catch (e: any) {
      setErrToast(String(e?.message ?? e));
    }
  }

  async function connect(id: string) {
    try {
      await ConnectProfile(id);
    } catch (e: any) {
      setErrToast(String(e?.message ?? e));
    }
  }

  async function disconnect() {
    try {
      await Disconnect();
    } catch (e: any) {
      setErrToast(String(e?.message ?? e));
    }
  }

  const selected = profiles.find((p) => p.id === selectedID) ?? null;

  return (
    <div className="h-screen flex flex-col bg-bg text-zinc-100 relative">
      <Titlebar
        showBack={!!selected && !activeID}
        title={selected?.name ?? "TrustTunnel"}
        onBack={() => setSelectedID(null)}
      />

      <div className="flex-1 overflow-y-auto">
        {selected ? (
          <DetailScreen
            profile={selected}
            snap={snap}
            isActive={selected.id === activeID}
            onConnect={() => connect(selected.id)}
            onDisconnect={disconnect}
            onDelete={() => removeProfile(selected.id)}
          />
        ) : (
          <ProfilesScreen
            profiles={profiles}
            activeID={activeID}
            onSelect={(id) => setSelectedID(id)}
            onImport={importProfile}
            onRefresh={refreshProfiles}
            onOpenDir={() => OpenProfilesDir()}
          />
        )}
      </div>

      {errToast && (
        <div className="absolute bottom-4 left-4 right-4 animate-slide-up bg-red-500/10 border border-red-500/30 text-red-200 px-3.5 py-2.5 rounded-lg flex items-start gap-2.5 text-sm shadow-lg backdrop-blur">
          <Alert className="w-4 h-4 mt-0.5 shrink-0" />
          <div className="flex-1 leading-tight break-words">{errToast}</div>
          <button onClick={() => setErrToast(null)} className="text-red-400 hover:text-red-200 text-xs px-1">
            ✕
          </button>
        </div>
      )}
    </div>
  );
}

// ── Titlebar ─────────────────────────────────────────────────────────────────

function Titlebar(props: { showBack: boolean; title: string; onBack: () => void }) {
  return (
    <div className="titlebar flex items-center gap-2 px-4 py-3 border-b border-border bg-bg-subtle">
      {props.showBack ? (
        <button
          onClick={props.onBack}
          className="p-1 -ml-1 rounded text-zinc-400 hover:text-zinc-100 hover:bg-bg-elevated"
        >
          <ArrowLeft className="w-4 h-4" />
        </button>
      ) : (
        <Logo className="w-4 h-4 text-zinc-100 ml-2" />
      )}
      <span className="text-sm font-semibold tracking-wide flex-1 truncate">{props.title}</span>
    </div>
  );
}

// ── Profiles list ────────────────────────────────────────────────────────────

function ProfilesScreen(props: {
  profiles: Profile[];
  activeID: string;
  onSelect: (id: string) => void;
  onImport: () => void;
  onRefresh: () => void;
  onOpenDir: () => void;
}) {
  return (
    <div className="px-4 py-4 space-y-3 animate-fade-in">
      <div className="flex items-center justify-between px-1">
        <h2 className="text-xs uppercase tracking-wide text-zinc-500 font-medium">
          Servers
        </h2>
        <div className="flex items-center gap-1">
          <button
            onClick={props.onRefresh}
            title="Refresh"
            className="p-1.5 rounded text-zinc-500 hover:text-zinc-100 hover:bg-bg-elevated"
          >
            <Refresh className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={props.onOpenDir}
            title="Open profiles folder"
            className="p-1.5 rounded text-zinc-500 hover:text-zinc-100 hover:bg-bg-elevated"
          >
            <Folder className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {props.profiles.length === 0 ? (
        <div className="flex flex-col items-center text-center py-12 border border-dashed border-border rounded-lg">
          <Empty className="w-8 h-8 text-zinc-700 mb-3" />
          <p className="text-sm text-zinc-300 font-medium">No profiles yet</p>
          <p className="text-xs text-zinc-600 mt-1 max-w-[260px]">
            Import a TrustTunnel .toml config to get started
          </p>
          <button onClick={props.onImport} className="btn-primary mt-4">
            <Plus className="w-3.5 h-3.5" /> Import profile
          </button>
        </div>
      ) : (
        <>
          <div className="space-y-2">
            {props.profiles.map((p) => (
              <ProfileRow
                key={p.id}
                profile={p}
                active={p.id === props.activeID}
                onClick={() => props.onSelect(p.id)}
              />
            ))}
          </div>
          <button onClick={props.onImport} className="btn-ghost w-full justify-center mt-2">
            <Plus className="w-3.5 h-3.5" /> Import another profile
          </button>
        </>
      )}
    </div>
  );
}

function ProfileRow(props: { profile: Profile; active: boolean; onClick: () => void }) {
  const { profile, active } = props;
  return (
    <button
      onClick={props.onClick}
      className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg border transition-colors text-left
        ${
          active
            ? "bg-emerald-500/10 border-emerald-500/30"
            : "bg-bg-subtle border-border hover:bg-bg-elevated hover:border-zinc-700"
        }`}
    >
      <div
        className={`w-2 h-2 rounded-full shrink-0 ${
          active ? "bg-emerald-400 animate-pulse" : "bg-zinc-700"
        }`}
      />
      <div className="min-w-0 flex-1">
        <div className="text-sm font-medium text-zinc-100 truncate">{profile.name}</div>
        <div className="text-[11px] text-zinc-500 truncate font-mono">
          {profile.username}@{profile.endpoint}
        </div>
      </div>
      <ChevronRight className="w-4 h-4 text-zinc-600 shrink-0" />
    </button>
  );
}

// ── Detail / connection screen ───────────────────────────────────────────────

function DetailScreen(props: {
  profile: Profile;
  snap: Snapshot;
  isActive: boolean;
  onConnect: () => void;
  onDisconnect: () => void;
  onDelete: () => void;
}) {
  const { profile, snap, isActive } = props;
  const isOn = isActive && snap.state === "connected";
  const isPending = isActive && snap.state === "connecting";

  return (
    <div className="flex flex-col items-center px-6 py-8">
      <button
        onClick={isOn || isPending ? props.onDisconnect : props.onConnect}
        disabled={isPending}
        className={`relative group flex items-center justify-center w-40 h-40 rounded-full transition-all duration-300
          ${
            isOn
              ? "bg-emerald-500/15 border-2 border-emerald-400 shadow-[0_0_60px_-10px_rgba(52,211,153,0.5)]"
              : isPending
              ? "bg-amber-500/15 border-2 border-amber-400/60"
              : "bg-bg-elevated border-2 border-zinc-700 hover:border-zinc-500"
          }`}
      >
        {isOn && (
          <span className="absolute inset-0 rounded-full border-2 border-emerald-400/40 animate-ping" />
        )}
        <Power
          className={`w-14 h-14 transition-colors ${
            isOn
              ? "text-emerald-400"
              : isPending
              ? "text-amber-400 animate-pulse"
              : "text-zinc-500 group-hover:text-zinc-300"
          }`}
        />
      </button>

      <div className="mt-6 text-center">
        <div
          className={`text-lg font-medium ${
            isOn
              ? "text-emerald-400"
              : isPending
              ? "text-amber-400"
              : snap.state === "error" && isActive
              ? "text-red-400"
              : "text-zinc-300"
          }`}
        >
          {isOn
            ? "Protected"
            : isPending
            ? "Connecting…"
            : snap.state === "error" && isActive
            ? "Error"
            : "Tap to connect"}
        </div>
        <div className="mt-1 text-xs text-zinc-500">{profile.username}@{profile.endpoint}</div>
      </div>

      {isOn && (
        <div className="mt-7 w-full max-w-xs space-y-2 animate-fade-in">
          <StatRow label="Uptime" value={fmtDuration(snap.uptime_sec)} />
          <StatRow label="Tunnels" value={`${snap.active_conn} / ${snap.tunnels}`} />
          <StatRow label="↓ Download" value={fmtBytes(snap.bytes_in)} color="text-emerald-400" />
          <StatRow label="↑ Upload" value={fmtBytes(snap.bytes_out)} color="text-amber-400" />
          {snap.tun_name && <StatRow label="Interface" value={snap.tun_name} mono />}
        </div>
      )}

      {/* Profile details */}
      <div className="mt-7 w-full max-w-xs">
        <div className="text-[11px] uppercase tracking-wide text-zinc-500 font-medium px-1 mb-2">
          Profile
        </div>
        <div className="space-y-2">
          <DetailRow label="Address" value={profile.endpoint} mono />
          {profile.toml?.endpoint?.hostname && (
            <DetailRow label="Hostname" value={profile.toml.endpoint.hostname} mono />
          )}
          {profile.toml?.endpoint?.upstream_protocol && (
            <DetailRow label="Protocol" value={profile.toml.endpoint.upstream_protocol} />
          )}
          {profile.toml?.endpoint?.skip_verification && (
            <DetailRow label="TLS verify" value="skipped" warn />
          )}
        </div>

        {!isActive && (
          <button
            onClick={props.onDelete}
            className="mt-5 w-full inline-flex items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-xs text-red-400 hover:text-red-300 hover:bg-red-500/10 transition-colors"
          >
            <Trash className="w-3.5 h-3.5" /> Delete profile
          </button>
        )}
      </div>
    </div>
  );
}

function StatRow(props: { label: string; value: string; color?: string; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between px-3 py-2 rounded-md bg-bg-subtle border border-border-subtle">
      <span className="text-xs text-zinc-500">{props.label}</span>
      <span
        className={`text-sm tabular-nums ${props.color ?? "text-zinc-100"} ${
          props.mono ? "font-mono text-xs" : ""
        }`}
      >
        {props.value}
      </span>
    </div>
  );
}

function DetailRow(props: { label: string; value: string; mono?: boolean; warn?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-3 px-3 py-1.5">
      <span className="text-[11px] text-zinc-500">{props.label}</span>
      <span
        className={`text-xs truncate ${props.warn ? "text-amber-400" : "text-zinc-300"} ${
          props.mono ? "font-mono" : ""
        }`}
      >
        {props.value}
      </span>
    </div>
  );
}

// ── helpers ──────────────────────────────────────────────────────────────────

function fmtBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  const u = ["KB", "MB", "GB", "TB"];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < u.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(v < 10 ? 2 : 1)} ${u[i]}`;
}

function fmtDuration(s: number): string {
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${s % 60}s`;
  const h = Math.floor(m / 60);
  return `${h}h ${m % 60}m`;
}
