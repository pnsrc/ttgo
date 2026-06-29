import { useState } from "react";
import { api, setAuth } from "../api";
import { Field, Icon } from "../ui";

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
      if (!w.ok) throw new Error("Server rejected token");
      props.onLogin();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
      setBusy(false);
    }
  }

  return (
    <div className="min-h-screen flex items-center justify-center p-6 bg-bg">
      <div className="w-full max-w-sm animate-slide-up">
        <div className="flex items-center justify-center mb-8">
          <div className="flex items-center gap-2.5">
            <Icon.Logo className="w-7 h-7 text-zinc-100" />
            <div>
              <div className="text-lg font-semibold text-zinc-100">TrustTunnel</div>
              <div className="text-[10px] text-zinc-600 -mt-0.5 uppercase tracking-widest">
                Admin Console
              </div>
            </div>
          </div>
        </div>

        <form onSubmit={submit} className="card p-6 space-y-5">
          <div>
            <h2 className="text-base font-semibold text-zinc-100">Sign in</h2>
            <p className="mt-1 text-xs text-zinc-500">
              Connect to your TrustTunnel admin API
            </p>
          </div>

          <Field label="Endpoint URL" hint="Leave empty if hosted from the endpoint itself">
            <input
              value={endpoint}
              onChange={(e) => setEndpoint(e.target.value)}
              placeholder="https://example.com:9090"
              className="input font-mono text-xs"
              autoFocus
            />
          </Field>

          <Field label="Bearer token">
            <input
              type="password"
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder="from [admin].token in vpn.toml"
              className="input font-mono text-xs"
            />
          </Field>

          {error && (
            <div className="flex items-start gap-2 bg-red-500/10 border border-red-500/30 text-red-300 px-3 py-2.5 rounded-md text-xs">
              <Icon.Alert className="w-4 h-4 mt-0.5 shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <button
            type="submit"
            disabled={busy || !token}
            className="btn-primary w-full py-2.5"
          >
            {busy ? "Connecting…" : "Sign in"}
          </button>
        </form>

        <p className="mt-6 text-center text-xs text-zinc-700">
          Self-hosted · No telemetry
        </p>
      </div>
    </div>
  );
}
