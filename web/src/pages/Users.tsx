import { useEffect, useState } from "react";
import { api, fmtBytes, User } from "../api";

export function Users(props: { onError: (msg: string | null) => void }) {
  const [users, setUsers] = useState<User[]>([]);
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState(false);
  const [editing, setEditing] = useState<User | null>(null);

  async function load() {
    try {
      const u = await api.users();
      setUsers(u ?? []);
      props.onError(null);
    } catch (e) {
      props.onError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    load();
    const id = setInterval(load, 5000);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  async function del(u: User) {
    if (!confirm(`Delete user "${u.username}"?\nActive connections will be terminated.`)) return;
    try {
      await api.deleteUser(u.username);
      await load();
    } catch (e) {
      props.onError(e instanceof Error ? e.message : String(e));
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-zinc-300 text-sm">
          Users
          {!loading && <span className="text-zinc-500 ml-2">({users.length})</span>}
        </h2>
        <button
          onClick={() => setAdding(true)}
          className="text-xs bg-cyan-500/20 border border-cyan-500/40 text-cyan-300 hover:bg-cyan-500/30 rounded px-3 py-1.5"
        >
          + add user
        </button>
      </div>

      {users.length === 0 && !loading && (
        <div className="text-zinc-500 text-sm text-center py-12 border border-dashed border-zinc-800 rounded">
          no users
        </div>
      )}

      {users.length > 0 && (
        <div className="overflow-x-auto border border-zinc-800 rounded">
          <table className="w-full text-xs">
            <thead className="bg-zinc-900 text-zinc-400 uppercase text-[10px]">
              <tr>
                <th className="px-3 py-2 text-left">Username</th>
                <th className="px-3 py-2 text-right">Devices</th>
                <th className="px-3 py-2 text-right">Active</th>
                <th className="px-3 py-2 text-right">Tunnels</th>
                <th className="px-3 py-2 text-right">In</th>
                <th className="px-3 py-2 text-right">Out</th>
                <th className="px-3 py-2 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-800">
              {users.map((u) => {
                const atLimit = u.max_devices > 0 && u.active_conns >= u.max_devices;
                return (
                  <tr key={u.username} className="hover:bg-zinc-900/60">
                    <td className="px-3 py-2 text-cyan-300">{u.username}</td>
                    <td className="px-3 py-2 text-right text-zinc-300">
                      {u.max_devices === 0 ? "∞" : u.max_devices}
                    </td>
                    <td
                      className={`px-3 py-2 text-right ${
                        atLimit ? "text-red-300" : u.active_conns > 0 ? "text-emerald-300" : "text-zinc-500"
                      }`}
                    >
                      {u.active_conns}
                    </td>
                    <td className="px-3 py-2 text-right text-zinc-400">{u.total_tunnels}</td>
                    <td className="px-3 py-2 text-right text-emerald-300">{fmtBytes(u.bytes_in)}</td>
                    <td className="px-3 py-2 text-right text-amber-300">{fmtBytes(u.bytes_out)}</td>
                    <td className="px-3 py-2 text-right space-x-2">
                      <button
                        onClick={() => setEditing(u)}
                        className="text-[10px] uppercase text-cyan-300 hover:text-cyan-200"
                      >
                        edit
                      </button>
                      <button
                        onClick={() => del(u)}
                        className="text-[10px] uppercase text-red-300 hover:text-red-200"
                      >
                        delete
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {adding && <AddUserModal onClose={() => setAdding(false)} onDone={load} />}
      {editing && <EditUserModal user={editing} onClose={() => setEditing(null)} onDone={load} />}
    </div>
  );
}

function Modal(props: { title: string; onClose: () => void; children: React.ReactNode }) {
  return (
    <div
      className="fixed inset-0 bg-black/60 flex items-center justify-center p-4"
      onClick={props.onClose}
    >
      <div
        className="bg-zinc-900 border border-zinc-800 rounded-lg p-5 w-full max-w-md"
        onClick={(e) => e.stopPropagation()}
      >
        <h3 className="text-cyan-400 text-sm font-bold mb-4">{props.title}</h3>
        {props.children}
      </div>
    </div>
  );
}

function AddUserModal(props: { onClose: () => void; onDone: () => void }) {
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [maxDevices, setMaxDevices] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      await api.addUser(username, password, maxDevices);
      props.onDone();
      props.onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setBusy(false);
    }
  }

  return (
    <Modal title="Add user" onClose={props.onClose}>
      <form onSubmit={submit} className="space-y-3 text-xs">
        <Field label="Username">
          <input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            className="input"
            autoFocus
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
        <Field label="Max devices (0 = unlimited)">
          <input
            type="number"
            min={0}
            value={maxDevices}
            onChange={(e) => setMaxDevices(parseInt(e.target.value) || 0)}
            className="input"
          />
        </Field>
        {error && (
          <div className="bg-red-950 border border-red-900 text-red-300 px-3 py-2 rounded">
            {error}
          </div>
        )}
        <div className="flex justify-end gap-2 pt-2">
          <button type="button" onClick={props.onClose} className="btn-ghost">
            Cancel
          </button>
          <button
            type="submit"
            disabled={busy || !username || !password}
            className="btn-primary"
          >
            {busy ? "..." : "Add"}
          </button>
        </div>
      </form>
      <style>{`
        .input { width:100%; background:#09090b; border:1px solid #3f3f46; border-radius:4px; padding:6px 10px; color:#fafafa; }
        .input:focus { outline:none; border-color:#06b6d4; }
        .btn-ghost { padding:6px 12px; color:#a1a1aa; }
        .btn-ghost:hover { color:#fafafa; }
        .btn-primary { padding:6px 12px; background:rgba(6,182,212,0.2); border:1px solid rgba(6,182,212,0.4); color:#67e8f9; border-radius:4px; }
        .btn-primary:hover { background:rgba(6,182,212,0.3); }
        .btn-primary:disabled { opacity:0.4; cursor:not-allowed; }
      `}</style>
    </Modal>
  );
}

function EditUserModal(props: { user: User; onClose: () => void; onDone: () => void }) {
  const [password, setPassword] = useState("");
  const [maxDevices, setMaxDevices] = useState(props.user.max_devices);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      const body: { password?: string; max_devices?: number } = {};
      if (password) body.password = password;
      if (maxDevices !== props.user.max_devices) body.max_devices = maxDevices;
      if (Object.keys(body).length === 0) {
        props.onClose();
        return;
      }
      await api.patchUser(props.user.username, body);
      props.onDone();
      props.onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setBusy(false);
    }
  }

  return (
    <Modal title={`Edit "${props.user.username}"`} onClose={props.onClose}>
      <form onSubmit={submit} className="space-y-3 text-xs">
        <Field label="New password (leave empty to keep current)">
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="input"
          />
        </Field>
        <Field label="Max devices (0 = unlimited)">
          <input
            type="number"
            min={0}
            value={maxDevices}
            onChange={(e) => setMaxDevices(parseInt(e.target.value) || 0)}
            className="input"
          />
        </Field>
        {error && (
          <div className="bg-red-950 border border-red-900 text-red-300 px-3 py-2 rounded">
            {error}
          </div>
        )}
        <div className="flex justify-end gap-2 pt-2">
          <button type="button" onClick={props.onClose} className="btn-ghost">
            Cancel
          </button>
          <button type="submit" disabled={busy} className="btn-primary">
            {busy ? "..." : "Save"}
          </button>
        </div>
      </form>
      <style>{`
        .input { width:100%; background:#09090b; border:1px solid #3f3f46; border-radius:4px; padding:6px 10px; color:#fafafa; }
        .input:focus { outline:none; border-color:#06b6d4; }
        .btn-ghost { padding:6px 12px; color:#a1a1aa; }
        .btn-ghost:hover { color:#fafafa; }
        .btn-primary { padding:6px 12px; background:rgba(6,182,212,0.2); border:1px solid rgba(6,182,212,0.4); color:#67e8f9; border-radius:4px; }
        .btn-primary:hover { background:rgba(6,182,212,0.3); }
        .btn-primary:disabled { opacity:0.4; cursor:not-allowed; }
      `}</style>
    </Modal>
  );
}

function Field(props: { label: string; children: React.ReactNode }) {
  return (
    <label className="block">
      <span className="text-zinc-400">{props.label}</span>
      <div className="mt-1">{props.children}</div>
    </label>
  );
}
