import { useEffect, useState } from "react";
import { api, isAuthed, clearAuth } from "./api";
import { Login } from "./pages/Login";
import { Sessions } from "./pages/Sessions";
import { Users } from "./pages/Users";

type Tab = "sessions" | "users";

export default function App() {
  const [authed, setAuthed] = useState(isAuthed());
  const [tab, setTab] = useState<Tab>("sessions");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!authed) return;
    api.whoami().catch((e) => setError(e.message));
  }, [authed]);

  if (!authed) {
    return <Login onLogin={() => setAuthed(true)} />;
  }

  return (
    <div className="min-h-screen flex flex-col">
      <header className="border-b border-zinc-800 bg-zinc-900">
        <div className="max-w-6xl mx-auto px-4 py-3 flex items-center justify-between">
          <div className="flex items-center gap-6">
            <h1 className="text-cyan-400 font-bold">TrustTunnel Admin</h1>
            <nav className="flex gap-1 text-sm">
              <TabBtn active={tab === "sessions"} onClick={() => setTab("sessions")}>
                Sessions
              </TabBtn>
              <TabBtn active={tab === "users"} onClick={() => setTab("users")}>
                Users
              </TabBtn>
            </nav>
          </div>
          <button
            onClick={() => {
              clearAuth();
              setAuthed(false);
            }}
            className="text-xs text-zinc-400 hover:text-red-400"
          >
            log out
          </button>
        </div>
      </header>

      {error && (
        <div className="bg-red-950 border-b border-red-800 px-4 py-2 text-sm text-red-300">
          {error}
        </div>
      )}

      <main className="flex-1 max-w-6xl w-full mx-auto px-4 py-6">
        {tab === "sessions" && <Sessions onError={setError} />}
        {tab === "users" && <Users onError={setError} />}
      </main>
    </div>
  );
}

function TabBtn(props: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      onClick={props.onClick}
      className={`px-3 py-1.5 rounded ${
        props.active
          ? "bg-cyan-500/10 text-cyan-300 border border-cyan-500/30"
          : "text-zinc-400 hover:text-zinc-100"
      }`}
    >
      {props.children}
    </button>
  );
}
