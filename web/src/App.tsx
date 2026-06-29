import { useEffect, useState } from "react";
import { api, isAuthed, clearAuth } from "./api";
import { Login } from "./pages/Login";
import { Sessions } from "./pages/Sessions";
import { Users } from "./pages/Users";
import { Rules } from "./pages/Rules";
import { Settings } from "./pages/Settings";
import { Icon, ToastHost } from "./ui";

type Tab = "sessions" | "users" | "rules" | "settings";

export default function App() {
  return (
    <ToastHost>
      <AppShell />
    </ToastHost>
  );
}

function AppShell() {
  const [authed, setAuthed] = useState(isAuthed());
  const [tab, setTab] = useState<Tab>("sessions");
  const [version, setVersion] = useState<string>("");
  const [sidebarOpen, setSidebarOpen] = useState(false);

  useEffect(() => {
    if (!authed) return;
    api.whoami().then((w) => setVersion(w.version)).catch(() => {});
  }, [authed]);

  if (!authed) return <Login onLogin={() => setAuthed(true)} />;

  function pick(t: Tab) {
    setTab(t);
    setSidebarOpen(false);
  }

  const sidebar = (
    <>
      <div className="px-5 py-5 flex items-center gap-2.5">
        <Icon.Logo className="w-5 h-5 text-zinc-100" />
        <div>
          <div className="text-sm font-semibold text-zinc-100">TrustTunnel</div>
          <div className="text-[10px] text-zinc-600 -mt-0.5 uppercase tracking-wider">Admin</div>
        </div>
      </div>

      <nav className="px-3 flex-1 flex flex-col gap-0.5">
        <NavBtn active={tab === "sessions"} onClick={() => pick("sessions")} icon={<Icon.Activity className="w-4 h-4" />}>
          Sessions
        </NavBtn>
        <NavBtn active={tab === "users"} onClick={() => pick("users")} icon={<Icon.Users className="w-4 h-4" />}>
          Users
        </NavBtn>
        <NavBtn active={tab === "rules"} onClick={() => pick("rules")} icon={<Icon.Shield className="w-4 h-4" />}>
          Routing rules
        </NavBtn>
        <NavBtn active={tab === "settings"} onClick={() => pick("settings")} icon={<Icon.Settings className="w-4 h-4" />}>
          Settings
        </NavBtn>
      </nav>

      <div className="px-3 py-3 border-t border-border">
        <button
          onClick={() => {
            clearAuth();
            setAuthed(false);
          }}
          className="w-full flex items-center gap-2.5 px-3 py-2 rounded-md text-sm text-zinc-400 hover:text-zinc-100 hover:bg-bg-elevated transition-colors"
        >
          <Icon.LogOut className="w-4 h-4" />
          Sign out
        </button>
        {version && <div className="mt-2 px-3 text-[10px] text-zinc-700 font-mono">{version}</div>}
      </div>
    </>
  );

  return (
    <div className="h-screen flex bg-bg">
      {/* Desktop sidebar */}
      <aside className="hidden md:flex w-56 shrink-0 bg-bg-subtle border-r border-border flex-col">
        {sidebar}
      </aside>

      {/* Mobile drawer */}
      {sidebarOpen && (
        <div
          className="md:hidden fixed inset-0 z-30 bg-black/60 backdrop-blur-sm animate-fade-in"
          onClick={() => setSidebarOpen(false)}
        />
      )}
      <aside
        className={`md:hidden fixed top-0 left-0 bottom-0 z-40 w-64 bg-bg-subtle border-r border-border flex flex-col transition-transform duration-200 ${
          sidebarOpen ? "translate-x-0" : "-translate-x-full"
        }`}
      >
        {sidebar}
      </aside>

      <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
        {/* Mobile top bar */}
        <header className="md:hidden flex items-center gap-3 px-4 py-3 border-b border-border bg-bg-subtle">
          <button
            onClick={() => setSidebarOpen(true)}
            className="p-1.5 -ml-1.5 rounded-md text-zinc-400 hover:text-zinc-100 hover:bg-bg-elevated"
          >
            <Icon.Menu className="w-5 h-5" />
          </button>
          <div className="flex items-center gap-2">
            <Icon.Logo className="w-4 h-4 text-zinc-100" />
            <span className="text-sm font-semibold text-zinc-100">TrustTunnel</span>
          </div>
        </header>

        <main className="flex-1 overflow-auto">
          <div className="max-w-6xl mx-auto px-4 py-5 md:px-8 md:py-8">
            {tab === "sessions" && <Sessions />}
            {tab === "users" && <Users />}
            {tab === "rules" && <Rules />}
            {tab === "settings" && <Settings />}
          </div>
        </main>
      </div>
    </div>
  );
}

function NavBtn(props: { active: boolean; onClick: () => void; icon: React.ReactNode; children: React.ReactNode }) {
  return (
    <button
      onClick={props.onClick}
      className={`w-full flex items-center gap-2.5 px-3 py-2 rounded-md text-sm transition-colors ${
        props.active ? "bg-bg-elevated text-zinc-100" : "text-zinc-500 hover:text-zinc-200 hover:bg-bg-elevated/50"
      }`}
    >
      {props.icon}
      {props.children}
    </button>
  );
}
