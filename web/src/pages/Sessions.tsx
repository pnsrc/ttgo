import { useEffect, useMemo, useState } from "react";
import { api, fmtBytes, fmtUptime, Session } from "../api";
import { Empty, Icon, Stat, useConfirm, useToast } from "../ui";

export function Sessions() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [loading, setLoading] = useState(true);
  const [, tick] = useState(0);
  const toast = useToast();
  const { confirm, dialog } = useConfirm();

  async function load(silent = false) {
    try {
      const s = await api.sessions();
      setSessions(s ?? []);
    } catch (e) {
      if (!silent) toast({ type: "err", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
    const id = setInterval(() => load(true), 3000);
    const t = setInterval(() => tick((x) => x + 1), 1000);
    return () => {
      clearInterval(id);
      clearInterval(t);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const stats = useMemo(() => {
    const users = new Set(sessions.map((s) => s.username));
    let totalIn = 0,
      totalOut = 0,
      totalTunnels = 0;
    for (const s of sessions) {
      totalIn += s.bytes_in;
      totalOut += s.bytes_out;
      totalTunnels += s.open_tunnels;
    }
    return { users: users.size, totalIn, totalOut, totalTunnels };
  }, [sessions]);

  async function kickSession(s: Session) {
    const ok = await confirm(
      "Disconnect session",
      `${s.username} from ${s.remote_addr} will receive a 407 and be disconnected.`,
      true
    );
    if (!ok) return;
    try {
      await api.kickSession(s.username, s.remote_addr);
      toast({ type: "ok", text: `Session ${s.remote_addr} kicked` });
      load();
    } catch (e) {
      toast({ type: "err", text: e instanceof Error ? e.message : String(e) });
    }
  }

  async function kickAll(username: string) {
    const ok = await confirm(
      `Disconnect all sessions of ${username}`,
      `All connections will be terminated with a 407 error.`,
      true
    );
    if (!ok) return;
    try {
      await api.kickUser(username);
      toast({ type: "ok", text: `Kicked all sessions of ${username}` });
      load();
    } catch (e) {
      toast({ type: "err", text: e instanceof Error ? e.message : String(e) });
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h1 className="text-xl font-semibold text-zinc-100">Sessions</h1>
          <p className="mt-1 text-sm text-zinc-500">
            <span className="hidden sm:inline">Live TLS connections to the endpoint, refreshing every 3 seconds.</span>
            <span className="sm:hidden">Live TLS connections</span>
          </p>
        </div>
        <button onClick={() => load()} className="btn-secondary shrink-0">
          <Icon.Refresh className="w-3.5 h-3.5" />
          <span className="hidden sm:inline">Refresh</span>
        </button>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
        <Stat label="Connections" value={sessions.length} accent="indigo" />
        <Stat label="Users online" value={stats.users} />
        <Stat label="Tunnels open" value={stats.totalTunnels} />
        <Stat
          label="Throughput"
          value={fmtBytes(stats.totalIn + stats.totalOut)}
          sub={
            <span>
              <span className="text-emerald-500">↓ {fmtBytes(stats.totalIn)}</span>
              {" · "}
              <span className="text-amber-500">↑ {fmtBytes(stats.totalOut)}</span>
            </span>
          }
        />
      </div>

      {sessions.length === 0 && !loading && (
        <Empty
          title="No active sessions"
          hint="When clients connect, they will appear here in real time."
          icon={<Icon.Activity className="w-8 h-8" />}
        />
      )}

      {sessions.length > 0 && (
        <div className="card overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-[11px] uppercase tracking-wide text-zinc-500">
                  <th className="text-left font-medium px-3 sm:px-4 py-2.5">User</th>
                  <th className="text-left font-medium px-3 sm:px-4 py-2.5 hidden md:table-cell">Remote address</th>
                  <th className="text-left font-medium px-3 sm:px-4 py-2.5 hidden sm:table-cell">Uptime</th>
                  <th className="text-right font-medium px-3 sm:px-4 py-2.5 hidden lg:table-cell">Tunnels</th>
                  <th className="text-right font-medium px-3 sm:px-4 py-2.5">↓ In</th>
                  <th className="text-right font-medium px-3 sm:px-4 py-2.5">↑ Out</th>
                  <th className="text-right font-medium px-3 sm:px-4 py-2.5"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border-subtle">
                {sessions.map((s) => (
                  <tr key={s.remote_addr + s.connected_at} className="hover:bg-bg-elevated/40 transition-colors">
                    <td className="px-3 sm:px-4 py-3 font-medium text-zinc-100">
                      <div>{s.username}</div>
                      <div className="md:hidden text-[11px] text-zinc-500 font-mono mt-0.5 truncate max-w-[160px]">{s.remote_addr}</div>
                      <div className="sm:hidden text-[11px] text-zinc-600 tabular-nums mt-0.5">{fmtUptime(s.connected_at)}</div>
                    </td>
                    <td className="px-3 sm:px-4 py-3 text-zinc-400 font-mono text-xs hidden md:table-cell">{s.remote_addr}</td>
                    <td className="px-3 sm:px-4 py-3 text-zinc-300 tabular-nums hidden sm:table-cell">{fmtUptime(s.connected_at)}</td>
                    <td className="px-3 sm:px-4 py-3 text-right text-zinc-300 tabular-nums hidden lg:table-cell">
                      <span className="text-zinc-100">{s.open_tunnels}</span>
                      <span className="text-zinc-600"> / {s.total_tunnels}</span>
                    </td>
                    <td className="px-3 sm:px-4 py-3 text-right text-emerald-400 tabular-nums whitespace-nowrap">{fmtBytes(s.bytes_in)}</td>
                    <td className="px-3 sm:px-4 py-3 text-right text-amber-400 tabular-nums whitespace-nowrap">{fmtBytes(s.bytes_out)}</td>
                    <td className="px-3 sm:px-4 py-3 text-right">
                      <div className="flex justify-end gap-1">
                        <button
                          onClick={() => kickSession(s)}
                          className="text-xs text-zinc-500 hover:text-zinc-200 px-2 py-1.5 rounded hover:bg-bg-elevated transition-colors"
                          title="Kick this session"
                        >
                          Kick
                        </button>
                        <button
                          onClick={() => kickAll(s.username)}
                          className="text-xs text-red-400/70 hover:text-red-300 px-2 py-1.5 rounded hover:bg-red-500/10 transition-colors hidden sm:inline"
                          title="Kick all sessions of user"
                        >
                          Kick all
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
      {dialog}
    </div>
  );
}
