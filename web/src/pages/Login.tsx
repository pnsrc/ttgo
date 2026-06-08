import { useState } from "react";
import { api, setAuth } from "../api";

export function Login(props: { onLogin: () => void }) {
  const [endpoint, setEndpoint] = useState(
    window.location.origin === "http://localhost:5173" ? "" : window.location.origin
  );
  const [token, setToken] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setBusy(true);
    setAuth(endpoint, token);
    try {
      const w = await api.whoami();
      if (!w.ok) throw new Error("server rejected token");
      props.onLogin();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setBusy(false);
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center">
      <form
        onSubmit={submit}
        className="bg-zinc-900 border border-zinc-800 rounded-lg p-6 w-full max-w-md space-y-4"
      >
        <div>
          <h1 className="text-cyan-400 font-bold text-lg">TrustTunnel Admin</h1>
          <p className="text-zinc-500 text-xs mt-1">Enter admin API endpoint and token</p>
        </div>

        <label className="block">
          <span className="text-xs text-zinc-400">Endpoint URL</span>
          <input
            value={endpoint}
            onChange={(e) => setEndpoint(e.target.value)}
            placeholder="http://127.0.0.1:9090 (leave empty for same origin)"
            className="mt-1 w-full bg-zinc-950 border border-zinc-700 rounded px-3 py-2 text-sm focus:outline-none focus:border-cyan-500"
            autoFocus
          />
        </label>

        <label className="block">
          <span className="text-xs text-zinc-400">Bearer token</span>
          <input
            type="password"
            value={token}
            onChange={(e) => setToken(e.target.value)}
            placeholder="from [admin].token in vpn.toml"
            className="mt-1 w-full bg-zinc-950 border border-zinc-700 rounded px-3 py-2 text-sm focus:outline-none focus:border-cyan-500"
          />
        </label>

        {error && (
          <div className="bg-red-950 border border-red-900 text-red-300 px-3 py-2 rounded text-xs">
            {error}
          </div>
        )}

        <button
          type="submit"
          disabled={busy || !token}
          className="w-full bg-cyan-500/20 border border-cyan-500/40 text-cyan-300 hover:bg-cyan-500/30 disabled:opacity-40 disabled:cursor-not-allowed rounded px-3 py-2 text-sm"
        >
          {busy ? "Connecting..." : "Sign in"}
        </button>
      </form>
    </div>
  );
}
