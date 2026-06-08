import { useEffect, useState } from "react";
import { api, fmtBytes, fmtUptime, Session } from "../api";

export function Sessions(props: { onError: (msg: string | null) => void }) {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [loading, setLoading] = useState(true);
  const [, tick] = useState(0);

  async function load() {
    try {
      const s = await api.sessions();
      setSessions(s ?? []);
      props.onError(null);
    } catch (e) {
      props.onError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
    const id = setInterval(load, 3000);
    const t = setInterval(() => tick((x) => x + 1), 1000); // тикаем uptime
    return () => {
      clearInterval(id);
      clearInterval(t);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function kickSession(s: Session) {
    if (!confirm(`Kick session ${s.username} @ ${s.remote_addr}?`)) return;
    try {
      await api.kickSession(s.username, s.remote_addr);
      await load();
    } catch (e) {
      props.onError(e instanceof Error ? e.message : String(e));
    }
  }

  async function kickAll(username: string) {
    if (!confirm(`Kick ALL sessions of "${username}"?`)) return;
    try {
      await api.kickUser(username);
      await load();
    } catch (e) {
      props.onError(e instanceof Error ? e.message : String(e));
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-zinc-300 text-sm">
          Active sessions
          {!loading && <span className="text-zinc-500 ml-2">({sessions.length})</span>}
        </h2>
        <button
          onClick={load}
          className="text-xs text-zinc-400 hover:text-zinc-100 border border-zinc-700 rounded px-2 py-1"
        >
          refresh
        </button>
      </div>

      {sessions.length === 0 && !loading && (
        <div className="text-zinc-500 text-sm text-center py-12 border border-dashed border-zinc-800 rounded">
          no active sessions
        </div>
      )}

      {sessions.length > 0 && (
        <div className="overflow-x-auto border border-zinc-800 rounded">
          <table className="w-full text-xs">
            <thead className="bg-zinc-900 text-zinc-400 uppercase text-[10px]">
              <tr>
                <th className="px-3 py-2 text-left">User</th>
                <th className="px-3 py-2 text-left">Remote</th>
                <th className="px-3 py-2 text-left">Uptime</th>
                <th className="px-3 py-2 text-right">Tunnels</th>
                <th className="px-3 py-2 text-right">In</th>
                <th className="px-3 py-2 text-right">Out</th>
                <th className="px-3 py-2 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-800">
              {sessions.map((s) => (
                <tr key={s.remote_addr + s.connected_at} className="hover:bg-zinc-900/60">
                  <td className="px-3 py-2 text-cyan-300">{s.username}</td>
                  <td className="px-3 py-2 text-zinc-400">{s.remote_addr}</td>
                  <td className="px-3 py-2 text-zinc-300">{fmtUptime(s.connected_at)}</td>
                  <td className="px-3 py-2 text-right text-zinc-300">
                    {s.open_tunnels} / {s.total_tunnels}
                  </td>
                  <td className="px-3 py-2 text-right text-emerald-300">{fmtBytes(s.bytes_in)}</td>
                  <td className="px-3 py-2 text-right text-amber-300">{fmtBytes(s.bytes_out)}</td>
                  <td className="px-3 py-2 text-right space-x-2">
                    <button
                      onClick={() => kickSession(s)}
                      className="text-[10px] uppercase text-red-300 hover:text-red-200"
                      title="kick this session"
                    >
                      kick
                    </button>
                    <button
                      onClick={() => kickAll(s.username)}
                      className="text-[10px] uppercase text-red-500 hover:text-red-400"
                      title="kick all sessions of user"
                    >
                      kick all
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
