import { useEffect, useState } from "react";
import { api, fmtBytes, fmtDuration, ServerInfo, SystemInfo } from "../api";
import { Icon, useConfirm, useToast } from "../ui";

// Поля доступные для редактирования (mapping в EditableConfig на сервере).
type EditableField =
  | "listen_address"
  | "allow_private_net"
  | "ipv6_available"
  | "cache_ttl_secs"
  | "tls_handshake_timeout_secs"
  | "connect_timeout_secs"
  | "tcp_idle_timeout_secs"
  | "udp_idle_timeout_secs";

export function Settings() {
  const [info, setInfo] = useState<ServerInfo | null>(null);
  const [system, setSystem] = useState<SystemInfo | null>(null);
  const [, tick] = useState(0);
  const [reloading, setReloading] = useState(false);
  const [restarting, setRestarting] = useState(false);
  // Локальные изменения по сравнению с server config
  const [edits, setEdits] = useState<Partial<Record<EditableField, string | number | boolean>>>({});
  const toast = useToast();
  const { confirm, dialog } = useConfirm();

  async function load(silent = false) {
    try {
      const [i, s] = await Promise.all([api.info(), api.system().catch(() => null)]);
      setInfo(i);
      if (s) setSystem(s);
    } catch (e) {
      if (!silent) toast({ type: "err", text: e instanceof Error ? e.message : String(e) });
    }
  }

  useEffect(() => {
    load();
    const id = setInterval(() => load(true), 10000);
    const t = setInterval(() => tick((x) => x + 1), 1000);
    return () => {
      clearInterval(id);
      clearInterval(t);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  function setField(key: EditableField, value: string | number | boolean) {
    if (!info?.config) return;
    const current = info.config[key as keyof typeof info.config];
    if (value === current) {
      // вернулось к исходному — убираем из diff
      const next = { ...edits };
      delete next[key];
      setEdits(next);
    } else {
      setEdits({ ...edits, [key]: value });
    }
  }

  function value<T>(key: EditableField): T {
    if (key in edits) return edits[key] as T;
    return (info?.config?.[key as keyof typeof info.config] as T);
  }

  const dirty = Object.keys(edits).length > 0;
  const dirtyNeedsRestart = dirty; // все наши editable требуют рестарт

  async function saveAndRestart() {
    const ok = await confirm(
      "Save and restart",
      "The endpoint will save changes to vpn.toml and restart. Active connections will drop and reconnect after ~2 seconds.",
      false
    );
    if (!ok) return;
    setRestarting(true);
    try {
      await api.patchConfig(edits);
      await api.restart();
      toast({ type: "ok", text: "Saved, restarting…" });
      setEdits({});

      // Poll /api/whoami пока сервер не поднимется
      const startedAt = Date.now();
      while (Date.now() - startedAt < 15000) {
        await new Promise((r) => setTimeout(r, 800));
        try {
          await api.whoami();
          toast({ type: "ok", text: "Server back online" });
          await load();
          break;
        } catch {
          // ещё не поднялся
        }
      }
    } catch (e) {
      toast({ type: "err", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setRestarting(false);
    }
  }

  async function reloadTLS() {
    const ok = await confirm(
      "Reload TLS certificates",
      "The endpoint will reread hosts.toml and reload all certificates. Existing connections are not affected.",
      false
    );
    if (!ok) return;
    setReloading(true);
    try {
      await api.reloadTLS();
      toast({ type: "ok", text: "TLS certificates reloaded" });
      load();
    } catch (e) {
      toast({ type: "err", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setReloading(false);
    }
  }

  if (!info) return <div className="text-sm text-zinc-500">Loading…</div>;

  const cfg = info.config;
  const liveUptime = info.uptime_seconds + Math.floor((Date.now() - new Date(info.started_at).getTime()) / 1000 - info.uptime_seconds);

  return (
    <div className="space-y-6 pb-24">
      <div>
        <h1 className="text-xl font-semibold text-zinc-100">Settings</h1>
        <p className="mt-1 text-sm text-zinc-500">Server information, configuration and runtime actions.</p>
      </div>

      {/* Server info */}
      <Section title="Server" icon={<Icon.Server className="w-4 h-4" />}>
        <Row label="Status" value={<StatusPill ok={true} text="Running" />} />
        <Row label="Uptime" value={<span className="tabular-nums">{fmtDuration(liveUptime)}</span>} />
        <Row label="Started at" value={<span className="text-xs">{new Date(info.started_at).toLocaleString()}</span>} />
        <Row label="Version" value={<code className="text-xs">{info.version}</code>} />
        {cfg && (
          <Row
            label="Listen address"
            hint="Address:port the endpoint binds to (changes require restart)."
            value={
              <EditableText
                value={value<string>("listen_address")}
                onChange={(v) => setField("listen_address", v)}
                edited={"listen_address" in edits}
                placeholder="0.0.0.0:443"
              />
            }
          />
        )}
      </Section>

      {/* User store */}
      {cfg && (
        <Section title="User store" icon={<Icon.Users className="w-4 h-4" />}>
          <Row label="Type" value={<code className="text-xs">{cfg.store_type}</code>} />
          {cfg.store_dsn && <Row label="DSN / path" value={<code className="text-xs break-all">{cfg.store_dsn}</code>} />}
          <Row
            label="Auth cache TTL"
            hint="How long credentials stay cached in memory before re-checking the store."
            value={
              <EditableNumber
                value={value<number>("cache_ttl_secs")}
                onChange={(v) => setField("cache_ttl_secs", v)}
                edited={"cache_ttl_secs" in edits}
                suffix="s"
                min={1}
                max={3600}
              />
            }
          />
        </Section>
      )}

      {/* TLS hosts */}
      {cfg && cfg.hostnames && cfg.hostnames.length > 0 && (
        <Section
          title="TLS hosts"
          icon={<Icon.Shield className="w-4 h-4" />}
          action={
            info.has_tls_reload && (
              <button onClick={reloadTLS} disabled={reloading} className="btn-secondary">
                <Icon.Refresh className={`w-3.5 h-3.5 ${reloading ? "animate-spin" : ""}`} />
                {reloading ? "Reloading…" : "Reload TLS"}
              </button>
            )
          }
        >
          <div className="divide-y divide-border-subtle">
            {cfg.hostnames.map((h) => {
              const exp = h.not_after ? new Date(h.not_after) : null;
              const days = exp ? Math.round((exp.getTime() - Date.now()) / 86400000) : null;
              const color =
                days === null ? "text-zinc-500" : days < 30 ? "text-red-400" : days < 60 ? "text-amber-400" : "text-emerald-400";
              return (
                <div key={h.hostname} className="flex items-center justify-between py-3 first:pt-0 last:pb-0">
                  <div className="min-w-0">
                    <div className="text-sm font-medium text-zinc-100 truncate">{h.hostname}</div>
                    {h.issuer && <div className="text-[11px] text-zinc-500 mt-0.5 truncate">issued by {h.issuer}</div>}
                  </div>
                  {exp && (
                    <div className={`text-xs ${color} tabular-nums shrink-0 ml-3 text-right`}>
                      <div>{exp.toLocaleDateString()}</div>
                      <div className="text-[10px] opacity-70">{days! > 0 ? `${days}d left` : "EXPIRED"}</div>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </Section>
      )}

      {/* Runtime */}
      {system && (
        <Section title="Runtime" icon={<Icon.Activity className="w-4 h-4" />}>
          <Row label="Go runtime" value={<code className="text-xs">{system.go_version}</code>} />
          <Row label="Platform" value={<code className="text-xs">{system.os}/{system.arch}</code>} />
          <Row label="CPU cores" value={<span className="tabular-nums">{system.num_cpu}</span>} />
          <Row label="Goroutines" value={<span className="tabular-nums">{system.goroutines}</span>} />
          <Row label="Heap allocated" value={<span className="tabular-nums">{fmtBytes(system.heap_alloc_bytes)}</span>} />
          <Row label="Heap reserved" value={<span className="tabular-nums">{fmtBytes(system.heap_sys_bytes)}</span>} hint="System memory held by the heap." />
          <Row label="Lifetime allocations" value={<span className="tabular-nums">{fmtBytes(system.total_alloc_bytes)}</span>} />
          <Row label="GC cycles" value={<span className="tabular-nums">{system.gc_runs}</span>} />
          <Row label="Active connections" value={<span className="tabular-nums">{system.active_sessions} from {system.active_users} {system.active_users === 1 ? "user" : "users"}</span>} />
        </Section>
      )}

      {/* Protocols & limits */}
      {cfg && (
        <Section title="Protocols & limits" icon={<Icon.Activity className="w-4 h-4" />}>
          <Row label="HTTP/3 (QUIC)" value={<Pill on={cfg.http3_enabled} />} hint="Enabled when [listen_protocols.quic] is configured." />
          <Row label="ICMP proxy" value={<Pill on={cfg.icmp_enabled} />} hint="Allows clients to send ping traffic through the endpoint via raw socket." />
          <Row
            label="IPv6 routing"
            value={
              <EditableToggle
                value={value<boolean>("ipv6_available")}
                onChange={(v) => setField("ipv6_available", v)}
                edited={"ipv6_available" in edits}
              />
            }
            hint="Affects how the client handles AAAA records."
          />
          <Row
            label="Allow private networks"
            value={
              <EditableToggle
                value={value<boolean>("allow_private_net")}
                onChange={(v) => setField("allow_private_net", v)}
                edited={"allow_private_net" in edits}
              />
            }
            hint="If on, clients can CONNECT to RFC1918 addresses behind the endpoint."
          />
          <Row
            label="TCP idle timeout"
            value={
              <EditableNumber
                value={value<number>("tcp_idle_timeout_secs")}
                onChange={(v) => setField("tcp_idle_timeout_secs", v)}
                edited={"tcp_idle_timeout_secs" in edits}
                suffix="s"
                min={60}
              />
            }
          />
          <Row
            label="UDP idle timeout"
            value={
              <EditableNumber
                value={value<number>("udp_idle_timeout_secs")}
                onChange={(v) => setField("udp_idle_timeout_secs", v)}
                edited={"udp_idle_timeout_secs" in edits}
                suffix="s"
                min={10}
              />
            }
          />
          <Row
            label="Connect timeout"
            value={
              <EditableNumber
                value={value<number>("connect_timeout_secs")}
                onChange={(v) => setField("connect_timeout_secs", v)}
                edited={"connect_timeout_secs" in edits}
                suffix="s"
                min={1}
                max={300}
              />
            }
          />
          <Row
            label="TLS handshake timeout"
            value={
              <EditableNumber
                value={value<number>("tls_handshake_timeout_secs")}
                onChange={(v) => setField("tls_handshake_timeout_secs", v)}
                edited={"tls_handshake_timeout_secs" in edits}
                suffix="s"
                min={1}
                max={120}
              />
            }
          />
        </Section>
      )}

      <div className="text-xs text-zinc-700 pt-2">
        Editable values are saved to <code className="bg-bg-elevated px-1 py-0.5 rounded">vpn.toml</code> on the server. Changes require a restart, which is performed automatically by systemd.
      </div>

      {/* Sticky save bar */}
      {dirty && (
        <div className="fixed bottom-0 left-0 right-0 md:left-56 z-30 border-t border-border bg-bg-subtle/95 backdrop-blur px-4 py-3 md:px-8 animate-slide-up">
          <div className="max-w-6xl mx-auto flex items-center justify-between gap-3">
            <div className="text-sm">
              <span className="text-amber-400 font-medium">{Object.keys(edits).length}</span>
              <span className="text-zinc-400"> unsaved {Object.keys(edits).length === 1 ? "change" : "changes"}</span>
              {dirtyNeedsRestart && (
                <span className="hidden sm:inline text-zinc-600"> · server will restart</span>
              )}
            </div>
            <div className="flex gap-2">
              <button onClick={() => setEdits({})} disabled={restarting} className="btn-ghost">
                Discard
              </button>
              <button onClick={saveAndRestart} disabled={restarting} className="btn-primary">
                {restarting ? (
                  <>
                    <Icon.Refresh className="w-3.5 h-3.5 animate-spin" />
                    Restarting…
                  </>
                ) : (
                  <>
                    <Icon.Check className="w-3.5 h-3.5" />
                    Save & restart
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}

      {dialog}
    </div>
  );
}

// ── Sections / Rows ──────────────────────────────────────────────────────────

function Section(props: { title: string; icon: React.ReactNode; action?: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="card">
      <div className="flex items-center justify-between px-5 py-3 border-b border-border">
        <div className="flex items-center gap-2 text-zinc-300">
          <span className="text-zinc-500">{props.icon}</span>
          <span className="text-sm font-medium">{props.title}</span>
        </div>
        {props.action}
      </div>
      <div className="px-5 py-4 space-y-3">{props.children}</div>
    </div>
  );
}

function Row(props: { label: string; value: React.ReactNode; hint?: string }) {
  return (
    <div className="flex items-start justify-between gap-4">
      <div className="min-w-0 flex-1">
        <div className="text-sm text-zinc-300">{props.label}</div>
        {props.hint && <div className="text-[11px] text-zinc-600 mt-0.5">{props.hint}</div>}
      </div>
      <div className="text-sm text-zinc-100 shrink-0 max-w-[60%] text-right break-words">{props.value}</div>
    </div>
  );
}

function StatusPill(props: { ok: boolean; text: string }) {
  return (
    <span
      className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-xs font-medium ${
        props.ok ? "bg-emerald-500/10 text-emerald-400 border border-emerald-500/30" : "bg-red-500/10 text-red-400 border border-red-500/30"
      }`}
    >
      <span className={`w-1.5 h-1.5 rounded-full ${props.ok ? "bg-emerald-400" : "bg-red-400"} animate-pulse`} />
      {props.text}
    </span>
  );
}

function Pill(props: { on: boolean }) {
  return props.on ? (
    <span className="text-xs px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/30">on</span>
  ) : (
    <span className="text-xs px-2 py-0.5 rounded-full bg-zinc-800 text-zinc-500 border border-border">off</span>
  );
}

// ── Editable controls ────────────────────────────────────────────────────────

function EditableText(props: { value: string; onChange: (v: string) => void; edited: boolean; placeholder?: string }) {
  return (
    <input
      type="text"
      value={props.value ?? ""}
      placeholder={props.placeholder}
      onChange={(e) => props.onChange(e.target.value)}
      className={`bg-bg-elevated border rounded px-2 py-1 text-xs font-mono w-40 sm:w-56 text-right focus:outline-none transition-colors ${
        props.edited ? "border-amber-500/50 text-amber-200" : "border-border text-zinc-100 hover:border-zinc-600 focus:border-zinc-500"
      }`}
    />
  );
}

function EditableNumber(props: {
  value: number;
  onChange: (v: number) => void;
  edited: boolean;
  suffix?: string;
  min?: number;
  max?: number;
}) {
  return (
    <div className={`inline-flex items-center gap-1 ${props.edited ? "text-amber-200" : "text-zinc-100"}`}>
      <input
        type="number"
        value={props.value ?? 0}
        min={props.min}
        max={props.max}
        onChange={(e) => props.onChange(parseInt(e.target.value) || 0)}
        className={`bg-bg-elevated border rounded px-2 py-1 text-xs tabular-nums w-24 text-right focus:outline-none transition-colors ${
          props.edited ? "border-amber-500/50" : "border-border hover:border-zinc-600 focus:border-zinc-500"
        }`}
      />
      {props.suffix && <span className="text-zinc-500 text-xs">{props.suffix}</span>}
    </div>
  );
}

function EditableToggle(props: { value: boolean; onChange: (v: boolean) => void; edited: boolean }) {
  return (
    <button
      type="button"
      onClick={() => props.onChange(!props.value)}
      className={`relative w-10 h-5 rounded-full transition-colors ${
        props.value ? "bg-emerald-500/60" : "bg-zinc-700"
      } ${props.edited ? "ring-2 ring-amber-500/50 ring-offset-2 ring-offset-bg-subtle" : ""}`}
    >
      <span
        className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform ${
          props.value ? "translate-x-5" : ""
        }`}
      />
    </button>
  );
}
