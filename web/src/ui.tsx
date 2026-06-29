import { ReactNode, createContext, useCallback, useContext, useEffect, useState } from "react";

// ── Icons ─────────────────────────────────────────────────────────────────────

type IconProps = { className?: string };

export const Icon = {
  Logo: (p: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" className={p.className}>
      <path
        d="M12 2L3 7v6c0 5 3.8 9.4 9 11 5.2-1.6 9-6 9-11V7l-9-5z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
      <path
        d="M9 12l2 2 4-4"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  ),
  Activity: (p: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" className={p.className}>
      <path d="M22 12h-4l-3 9L9 3l-3 9H2" />
    </svg>
  ),
  Users: (p: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" className={p.className}>
      <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2" />
      <circle cx="9" cy="7" r="4" />
      <path d="M23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75" />
    </svg>
  ),
  LogOut: (p: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" className={p.className}>
      <path d="M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4" />
      <polyline points="16 17 21 12 16 7" />
      <line x1="21" y1="12" x2="9" y2="12" />
    </svg>
  ),
  Plus: (p: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className={p.className}>
      <line x1="12" y1="5" x2="12" y2="19" />
      <line x1="5" y1="12" x2="19" y2="12" />
    </svg>
  ),
  Trash: (p: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" className={p.className}>
      <polyline points="3 6 5 6 21 6" />
      <path d="M19 6l-2 14a2 2 0 0 1-2 2H9a2 2 0 0 1-2-2L5 6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
    </svg>
  ),
  Edit: (p: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" className={p.className}>
      <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
      <path d="M18.5 2.5a2.12 2.12 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
    </svg>
  ),
  Refresh: (p: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" className={p.className}>
      <polyline points="23 4 23 10 17 10" />
      <polyline points="1 20 1 14 7 14" />
      <path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15" />
    </svg>
  ),
  X: (p: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className={p.className}>
      <line x1="18" y1="6" x2="6" y2="18" />
      <line x1="6" y1="6" x2="18" y2="18" />
    </svg>
  ),
  Check: (p: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className={p.className}>
      <polyline points="20 6 9 17 4 12" />
    </svg>
  ),
  ArrowUp: (p: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" className={p.className}>
      <line x1="12" y1="19" x2="12" y2="5" />
      <polyline points="5 12 12 5 19 12" />
    </svg>
  ),
  ArrowDown: (p: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" className={p.className}>
      <line x1="12" y1="5" x2="12" y2="19" />
      <polyline points="19 12 12 19 5 12" />
    </svg>
  ),
  Alert: (p: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" className={p.className}>
      <circle cx="12" cy="12" r="10" />
      <line x1="12" y1="8" x2="12" y2="12" />
      <line x1="12" y1="16" x2="12.01" y2="16" />
    </svg>
  ),
  Menu: (p: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" className={p.className}>
      <line x1="3" y1="6" x2="21" y2="6" />
      <line x1="3" y1="12" x2="21" y2="12" />
      <line x1="3" y1="18" x2="21" y2="18" />
    </svg>
  ),
  Settings: (p: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" className={p.className}>
      <circle cx="12" cy="12" r="3" />
      <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06a1.65 1.65 0 0 0 .33-1.82 1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06a1.65 1.65 0 0 0 1.82.33H9a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z" />
    </svg>
  ),
  Server: (p: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" className={p.className}>
      <rect x="2" y="2" width="20" height="8" rx="2" ry="2" />
      <rect x="2" y="14" width="20" height="8" rx="2" ry="2" />
      <line x1="6" y1="6" x2="6.01" y2="6" />
      <line x1="6" y1="18" x2="6.01" y2="18" />
    </svg>
  ),
  Shield: (p: IconProps) => (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" className={p.className}>
      <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
    </svg>
  ),
};

// ── Toast ─────────────────────────────────────────────────────────────────────

type Toast = { id: number; type: "ok" | "err"; text: string };
type ToastCtx = (t: { type: "ok" | "err"; text: string }) => void;

const ToastContext = createContext<ToastCtx>(() => {});

export function useToast() {
  return useContext(ToastContext);
}

export function ToastHost(props: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);

  const push = useCallback<ToastCtx>((t) => {
    const id = Date.now() + Math.random();
    setToasts((curr) => [...curr, { id, ...t }]);
    setTimeout(() => setToasts((curr) => curr.filter((x) => x.id !== id)), 4000);
  }, []);

  return (
    <ToastContext.Provider value={push}>
      {props.children}
      <div className="fixed bottom-4 right-4 z-50 flex flex-col gap-2 pointer-events-none">
        {toasts.map((t) => (
          <div
            key={t.id}
            className={`pointer-events-auto animate-slide-up flex items-start gap-2.5 max-w-sm rounded-lg border px-3.5 py-2.5 shadow-lg backdrop-blur ${
              t.type === "ok"
                ? "bg-emerald-500/10 border-emerald-500/30 text-emerald-200"
                : "bg-red-500/10 border-red-500/30 text-red-200"
            }`}
          >
            <div className="mt-0.5">
              {t.type === "ok" ? <Icon.Check className="w-4 h-4" /> : <Icon.Alert className="w-4 h-4" />}
            </div>
            <div className="text-sm leading-tight">{t.text}</div>
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

// ── Modal ─────────────────────────────────────────────────────────────────────

export function Modal(props: { title: string; onClose: () => void; children: ReactNode }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") props.onClose();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [props]);

  return (
    <div
      className="fixed inset-0 z-40 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 animate-fade-in"
      onClick={props.onClose}
    >
      <div
        className="card w-full max-w-md shadow-2xl animate-slide-up"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-5 py-3.5 border-b border-border">
          <h3 className="text-sm font-semibold text-zinc-100">{props.title}</h3>
          <button
            onClick={props.onClose}
            className="text-zinc-500 hover:text-zinc-200 transition-colors"
          >
            <Icon.X className="w-4 h-4" />
          </button>
        </div>
        <div className="p-5">{props.children}</div>
      </div>
    </div>
  );
}

// ── Confirm dialog hook ───────────────────────────────────────────────────────

export function useConfirm() {
  const [state, setState] = useState<{
    title: string;
    body: string;
    danger?: boolean;
    resolve: (ok: boolean) => void;
  } | null>(null);

  const confirm = useCallback(
    (title: string, body: string, danger = false) =>
      new Promise<boolean>((resolve) => {
        setState({ title, body, danger, resolve });
      }),
    []
  );

  const dialog = state && (
    <Modal title={state.title} onClose={() => { state.resolve(false); setState(null); }}>
      <p className="text-sm text-zinc-400 mb-5">{state.body}</p>
      <div className="flex justify-end gap-2">
        <button
          className="btn-ghost"
          onClick={() => { state.resolve(false); setState(null); }}
        >
          Cancel
        </button>
        <button
          className={state.danger ? "btn-danger" : "btn-primary"}
          onClick={() => { state.resolve(true); setState(null); }}
        >
          {state.danger ? "Delete" : "Confirm"}
        </button>
      </div>
    </Modal>
  );

  return { confirm, dialog };
}

// ── Field ─────────────────────────────────────────────────────────────────────

export function Field(props: { label: string; children: ReactNode; hint?: string }) {
  return (
    <div>
      <label className="label">{props.label}</label>
      {props.children}
      {props.hint && <p className="mt-1 text-xs text-zinc-600">{props.hint}</p>}
    </div>
  );
}

// ── Empty state ───────────────────────────────────────────────────────────────

export function Empty(props: { title: string; hint?: string; icon?: ReactNode }) {
  return (
    <div className="card flex flex-col items-center justify-center py-16 px-4 text-center">
      {props.icon && <div className="mb-3 text-zinc-700">{props.icon}</div>}
      <p className="text-sm text-zinc-300 font-medium">{props.title}</p>
      {props.hint && <p className="mt-1 text-xs text-zinc-600">{props.hint}</p>}
    </div>
  );
}

// ── Stat card ─────────────────────────────────────────────────────────────────

export function Stat(props: { label: string; value: ReactNode; sub?: ReactNode; accent?: "emerald" | "amber" | "indigo" | "zinc" }) {
  const accentClass = {
    emerald: "text-emerald-400",
    amber: "text-amber-400",
    indigo: "text-indigo-400",
    zinc: "text-zinc-100",
  }[props.accent ?? "zinc"];

  return (
    <div className="card p-4">
      <div className="text-xs text-zinc-500 font-medium uppercase tracking-wide">{props.label}</div>
      <div className={`mt-2 text-2xl font-semibold tabular-nums ${accentClass}`}>{props.value}</div>
      {props.sub && <div className="mt-1 text-xs text-zinc-500">{props.sub}</div>}
    </div>
  );
}
