import { useEffect, useState } from "react";
import { api, fmtBytes, User } from "../api";
import { Empty, Field, Icon, Modal, useConfirm, useToast } from "../ui";

export function Users() {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState(false);
  const [editing, setEditing] = useState<User | null>(null);
  const toast = useToast();
  const { confirm, dialog } = useConfirm();

  async function load(silent = false) {
    try {
      const u = await api.users();
      setUsers(u ?? []);
    } catch (e) {
      if (!silent) toast({ type: "err", text: e instanceof Error ? e.message : String(e) });
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
    const id = setInterval(() => load(true), 5000);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function del(u: User) {
    const ok = await confirm(
      `Delete ${u.username}`,
      "Active connections will be terminated and the user will no longer be able to authenticate.",
      true
    );
    if (!ok) return;
    try {
      await api.deleteUser(u.username);
      toast({ type: "ok", text: `User ${u.username} deleted` });
      load();
    } catch (e) {
      toast({ type: "err", text: e instanceof Error ? e.message : String(e) });
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h1 className="text-xl font-semibold text-zinc-100">Users</h1>
          <p className="mt-1 text-sm text-zinc-500">
            <span className="hidden sm:inline">Manage accounts, device limits and view per-user traffic.</span>
            <span className="sm:hidden">Accounts and limits</span>
          </p>
        </div>
        <button onClick={() => setAdding(true)} className="btn-primary shrink-0">
          <Icon.Plus className="w-3.5 h-3.5" />
          <span className="hidden sm:inline">Add user</span>
          <span className="sm:hidden">Add</span>
        </button>
      </div>

      {users.length === 0 && !loading && (
        <Empty
          title="No users yet"
          hint="Create your first user to start authenticating clients."
          icon={<Icon.Users className="w-8 h-8" />}
        />
      )}

      {users.length > 0 && (
        <div className="card overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-border text-[11px] uppercase tracking-wide text-zinc-500">
                  <th className="text-left font-medium px-3 sm:px-4 py-2.5">Username</th>
                  <th className="text-right font-medium px-3 sm:px-4 py-2.5 hidden sm:table-cell">Devices</th>
                  <th className="text-right font-medium px-3 sm:px-4 py-2.5">Active</th>
                  <th className="text-right font-medium px-3 sm:px-4 py-2.5 hidden lg:table-cell">Total tunnels</th>
                  <th className="text-right font-medium px-3 sm:px-4 py-2.5 hidden md:table-cell">↓ In</th>
                  <th className="text-right font-medium px-3 sm:px-4 py-2.5 hidden md:table-cell">↑ Out</th>
                  <th className="text-right font-medium px-3 sm:px-4 py-2.5">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border-subtle">
                {users.map((u) => {
                  const atLimit = u.max_devices > 0 && u.active_conns >= u.max_devices;
                  return (
                    <tr key={u.username} className="hover:bg-bg-elevated/40 transition-colors">
                      <td className="px-3 sm:px-4 py-3 font-medium">
                        <div className="flex items-center gap-2">
                          <span className={u.enabled ? "text-zinc-100" : "text-zinc-500 line-through"}>{u.username}</span>
                          {!u.enabled && (
                            <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-zinc-800 text-zinc-500 border border-border">disabled</span>
                          )}
                          <ExpiryBadge expiresAt={u.expires_at} />
                          <TrafficBadge used={u.traffic_used} limit={u.traffic_limit} />
                        </div>
                        <div className="sm:hidden text-[11px] text-zinc-500 tabular-nums mt-0.5">
                          {u.max_devices === 0 ? "unlimited" : `limit ${u.max_devices}`}
                          {" · "}
                          <span className="text-emerald-500">↓ {fmtBytes(u.bytes_in)}</span>
                          {" "}
                          <span className="text-amber-500">↑ {fmtBytes(u.bytes_out)}</span>
                        </div>
                      </td>
                      <td className="px-3 sm:px-4 py-3 text-right text-zinc-300 tabular-nums hidden sm:table-cell">
                        {u.max_devices === 0 ? (
                          <span className="text-zinc-600">unlimited</span>
                        ) : (
                          u.max_devices
                        )}
                      </td>
                      <td className="px-3 sm:px-4 py-3 text-right tabular-nums">
                        {u.active_conns === 0 ? (
                          <span className="text-zinc-600">—</span>
                        ) : (
                          <span
                            className={`inline-flex items-center gap-1.5 ${
                              atLimit ? "text-red-400" : "text-emerald-400"
                            }`}
                          >
                            <span className={`w-1.5 h-1.5 rounded-full ${atLimit ? "bg-red-400" : "bg-emerald-400"} animate-pulse`} />
                            {u.active_conns}
                          </span>
                        )}
                      </td>
                      <td className="px-3 sm:px-4 py-3 text-right text-zinc-400 tabular-nums hidden lg:table-cell">{u.total_tunnels}</td>
                      <td className="px-3 sm:px-4 py-3 text-right text-emerald-400 tabular-nums hidden md:table-cell whitespace-nowrap">{fmtBytes(u.bytes_in)}</td>
                      <td className="px-3 sm:px-4 py-3 text-right text-amber-400 tabular-nums hidden md:table-cell whitespace-nowrap">{fmtBytes(u.bytes_out)}</td>
                      <td className="px-3 sm:px-4 py-3 text-right">
                        <div className="flex justify-end gap-1">
                          <button
                            onClick={() => setEditing(u)}
                            className="text-zinc-400 hover:text-zinc-100 p-2 sm:p-1.5 rounded hover:bg-bg-elevated transition-colors"
                            title="Edit"
                          >
                            <Icon.Edit className="w-4 h-4 sm:w-3.5 sm:h-3.5" />
                          </button>
                          <button
                            onClick={() => del(u)}
                            className="text-zinc-400 hover:text-red-400 p-2 sm:p-1.5 rounded hover:bg-red-500/10 transition-colors"
                            title="Delete"
                          >
                            <Icon.Trash className="w-4 h-4 sm:w-3.5 sm:h-3.5" />
                          </button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {adding && <AddUserModal onClose={() => setAdding(false)} onDone={load} />}
      {editing && <EditUserModal user={editing} onClose={() => setEditing(null)} onDone={load} />}
      {dialog}
    </div>
  );
}

function AddUserModal(props: { onClose: () => void; onDone: () => void }) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [maxDevices, setMaxDevices] = useState(0);
  const [expiresDays, setExpiresDays] = useState(0);
  const [trafficLimitGB, setTrafficLimitGB] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const toast = useToast();

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const expiresAt = expiresDays > 0 ? Math.floor(Date.now() / 1000) + expiresDays * 86400 : 0;
      const trafficLimit = trafficLimitGB > 0 ? Math.floor(trafficLimitGB * 1024 * 1024 * 1024) : 0;
      await api.addUser(username, password, maxDevices, expiresAt, trafficLimit);
      toast({ type: "ok", text: `User ${username} created` });
      props.onDone();
      props.onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setBusy(false);
    }
  }

  return (
    <Modal title="Add user" onClose={props.onClose}>
      <form onSubmit={submit} className="space-y-4">
        <Field label="Username">
          <input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            className="input"
            autoFocus
            placeholder="alice"
          />
        </Field>
        <Field label="Password">
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="input"
          />
        </Field>
        <div className="grid grid-cols-3 gap-3">
          <Field label="Devices" hint="0 = ∞">
            <input
              type="number"
              min={0}
              value={maxDevices}
              onChange={(e) => setMaxDevices(parseInt(e.target.value) || 0)}
              className="input tabular-nums"
            />
          </Field>
          <Field label="Expires in (days)" hint="0 = never">
            <input
              type="number"
              min={0}
              value={expiresDays}
              onChange={(e) => setExpiresDays(parseInt(e.target.value) || 0)}
              className="input tabular-nums"
            />
          </Field>
          <Field label="Traffic limit (GB)" hint="0 = ∞">
            <input
              type="number"
              min={0}
              step={0.1}
              value={trafficLimitGB}
              onChange={(e) => setTrafficLimitGB(parseFloat(e.target.value) || 0)}
              className="input tabular-nums"
            />
          </Field>
        </div>
        {error && (
          <div className="flex items-start gap-2 bg-red-500/10 border border-red-500/30 text-red-300 px-3 py-2 rounded-md text-xs">
            <Icon.Alert className="w-4 h-4 mt-0.5 shrink-0" />
            <span>{error}</span>
          </div>
        )}
        <div className="flex justify-end gap-2 pt-2">
          <button type="button" onClick={props.onClose} className="btn-ghost">Cancel</button>
          <button type="submit" disabled={busy || !username || !password} className="btn-primary">
            {busy ? "Creating…" : "Create user"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

function EditUserModal(props: { user: User; onClose: () => void; onDone: () => void }) {
  const [password, setPassword] = useState("");
  const [maxDevices, setMaxDevices] = useState(props.user.max_devices);
  const [enabled, setEnabled] = useState(props.user.enabled);
  // Дата истечения как YYYY-MM-DD
  const initialExpiry = props.user.expires_at
    ? new Date(props.user.expires_at * 1000).toISOString().slice(0, 10)
    : "";
  const [expiresStr, setExpiresStr] = useState(initialExpiry);
  // Лимит трафика в GB
  const initialLimitGB = props.user.traffic_limit
    ? props.user.traffic_limit / (1024 * 1024 * 1024)
    : 0;
  const [trafficLimitGB, setTrafficLimitGB] = useState(initialLimitGB);
  const [resetTraffic, setResetTraffic] = useState(false);

  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const toast = useToast();

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const body: Parameters<typeof api.patchUser>[1] = {};
      if (password) body.password = password;
      if (maxDevices !== props.user.max_devices) body.max_devices = maxDevices;
      if (enabled !== props.user.enabled) body.enabled = enabled;

      const newExp = expiresStr ? Math.floor(new Date(expiresStr).getTime() / 1000) : 0;
      if (newExp !== props.user.expires_at) body.expires_at = newExp;

      const newLimit = Math.floor(trafficLimitGB * 1024 * 1024 * 1024);
      if (newLimit !== props.user.traffic_limit) body.traffic_limit = newLimit;

      if (resetTraffic) body.reset_traffic = true;

      if (Object.keys(body).length === 0) {
        props.onClose();
        return;
      }
      await api.patchUser(props.user.username, body);
      toast({ type: "ok", text: `User ${props.user.username} updated` });
      props.onDone();
      props.onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setBusy(false);
    }
  }

  return (
    <Modal title={`Edit ${props.user.username}`} onClose={props.onClose}>
      <form onSubmit={submit} className="space-y-4">
        {/* Enabled toggle */}
        <div className="flex items-center justify-between p-3 rounded-md border border-border">
          <div>
            <div className="text-sm text-zinc-200">Account enabled</div>
            <div className="text-[11px] text-zinc-600 mt-0.5">Disabled accounts cannot authenticate.</div>
          </div>
          <Toggle on={enabled} onChange={setEnabled} />
        </div>

        <Field label="New password" hint="Leave empty to keep current password">
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="input"
            placeholder="••••••••"
          />
        </Field>

        <div className="grid grid-cols-2 gap-3">
          <Field label="Device limit" hint="0 = unlimited">
            <input
              type="number"
              min={0}
              value={maxDevices}
              onChange={(e) => setMaxDevices(parseInt(e.target.value) || 0)}
              className="input tabular-nums"
            />
          </Field>
          <Field label="Expires on" hint="Leave empty for never">
            <input
              type="date"
              value={expiresStr}
              onChange={(e) => setExpiresStr(e.target.value)}
              className="input tabular-nums"
            />
          </Field>
        </div>

        <Field label="Traffic limit (GB)" hint="0 = unlimited. Counts toward in + out combined.">
          <input
            type="number"
            min={0}
            step={0.1}
            value={trafficLimitGB}
            onChange={(e) => setTrafficLimitGB(parseFloat(e.target.value) || 0)}
            className="input tabular-nums"
          />
        </Field>

        <label className="flex items-center gap-2 text-sm text-zinc-300 cursor-pointer">
          <input
            type="checkbox"
            checked={resetTraffic}
            onChange={(e) => setResetTraffic(e.target.checked)}
            className="rounded border-border bg-bg-elevated"
          />
          Reset traffic counter to zero
        </label>

        {error && (
          <div className="flex items-start gap-2 bg-red-500/10 border border-red-500/30 text-red-300 px-3 py-2 rounded-md text-xs">
            <Icon.Alert className="w-4 h-4 mt-0.5 shrink-0" />
            <span>{error}</span>
          </div>
        )}
        <div className="flex justify-end gap-2 pt-2">
          <button type="button" onClick={props.onClose} className="btn-ghost">Cancel</button>
          <button type="submit" disabled={busy} className="btn-primary">
            {busy ? "Saving…" : "Save changes"}
          </button>
        </div>
      </form>
    </Modal>
  );
}

// ── Mini-components ───────────────────────────────────────────────────────────

function Toggle(props: { on: boolean; onChange: (on: boolean) => void }) {
  return (
    <button
      type="button"
      onClick={() => props.onChange(!props.on)}
      className={`relative w-10 h-5 rounded-full transition-colors ${
        props.on ? "bg-emerald-500/60" : "bg-zinc-700"
      }`}
    >
      <span
        className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform ${
          props.on ? "translate-x-5" : ""
        }`}
      />
    </button>
  );
}

function ExpiryBadge({ expiresAt }: { expiresAt: number }) {
  if (!expiresAt) return null;
  const days = Math.floor((expiresAt * 1000 - Date.now()) / 86400000);
  if (days < 0) {
    return (
      <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-red-500/10 text-red-400 border border-red-500/30">
        expired
      </span>
    );
  }
  const color = days < 7 ? "bg-amber-500/10 text-amber-400 border-amber-500/30" : "bg-bg-elevated text-zinc-500 border-border";
  return (
    <span className={`text-[10px] px-1.5 py-0.5 rounded-full border ${color}`} title={new Date(expiresAt * 1000).toLocaleDateString()}>
      {days}d left
    </span>
  );
}

function TrafficBadge({ used, limit }: { used: number; limit: number }) {
  if (!limit) return null;
  const pct = Math.min(100, Math.floor((used / limit) * 100));
  const overlimit = used >= limit;
  const color = overlimit
    ? "bg-red-500/10 text-red-400 border-red-500/30"
    : pct > 80
    ? "bg-amber-500/10 text-amber-400 border-amber-500/30"
    : "bg-bg-elevated text-zinc-500 border-border";
  return (
    <span className={`text-[10px] px-1.5 py-0.5 rounded-full border ${color}`} title={`${fmtBytes(used)} / ${fmtBytes(limit)}`}>
      {pct}%
    </span>
  );
}
