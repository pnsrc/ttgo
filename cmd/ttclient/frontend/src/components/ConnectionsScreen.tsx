import { useState, useEffect } from "react";
import { useTranslation } from "../i18n";
import { GetConnections, AddExclusion } from "../api";
import { Plus } from "../icons";
import { fmtBytes, fmtDuration } from "../utils";

export function ConnectionsScreen({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  const [conns, setConns] = useState<any[]>([]);
  const [excluded, setExcluded] = useState<Set<string>>(new Set());
  const [filter, setFilter] = useState("");
  const [ctxMenu, setCtxMenu] = useState<{ x: number; y: number; domain: string; ip: string } | null>(null);

  useEffect(() => {
    const poll = () => GetConnections().then((c) => setConns(c || [])).catch(() => {});
    poll();
    const interval = setInterval(poll, 1500);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    const close = () => setCtxMenu(null);
    window.addEventListener("click", close);
    window.addEventListener("ctxmenu:close", close);
    return () => { window.removeEventListener("click", close); window.removeEventListener("ctxmenu:close", close); };
  }, []);

  async function handleExclude(domain: string) {
    if (!domain) return;
    try {
      await AddExclusion(domain);
      setExcluded((prev) => new Set(prev).add(domain));
      setCtxMenu(null);
    } catch {}
  }

  async function handleExcludeWildcard(domain: string) {
    if (!domain) return;
    const parts = domain.split(".");
    const wildcard = parts.length > 2 ? "*." + parts.slice(-2).join(".") : "*." + domain;
    try {
      await AddExclusion(wildcard);
      setExcluded((prev) => new Set(prev).add(domain));
      setCtxMenu(null);
    } catch {}
  }

  const sorted = [...conns].reverse();
  const filtered = filter
    ? sorted.filter((c) => (c.domain || c.target).toLowerCase().includes(filter.toLowerCase()))
    : sorted;
  const activeCount = conns.filter((c) => c.active).length;

  return (
    <div className="flex flex-col h-full bg-transparent text-fg-primary p-4 animate-fade-in">
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-xs uppercase tracking-wide text-fg-faint font-medium">
          {t.active_conns}
          <span className="ml-2 text-emerald-400">{activeCount}</span>
          <span className="text-fg-faint"> / {conns.length}</span>
        </h2>
      </div>

      <input
        type="text"
        value={filter}
        onChange={(e) => setFilter(e.target.value)}
        placeholder={t.conn_filter}
        className="w-full bg-bg-elevated border border-border rounded px-3 py-1.5 text-xs font-mono text-fg-secondary focus:outline-none focus:border-emerald-500 mb-3"
      />

      <div className="flex-1 overflow-y-auto bg-bg-elevated border border-border rounded-lg relative">
        {filtered.length === 0 ? (
          <div className="flex items-center justify-center h-full text-fg-faint text-sm">{t.conn_empty}</div>
        ) : (
          filtered.map((c: any, i: number) => {
            const label = c.domain || c.target.split(":")[0];
            const isExcluded = excluded.has(c.domain || label);
            return (
              <div
                key={`${c.id}-${i}`}
                className={`group flex items-center gap-2 px-3 py-1.5 border-b border-border last:border-0 hover:bg-white/5 transition-colors ${!c.active ? "opacity-50" : ""}`}
                onContextMenu={(e) => {
                  e.preventDefault();
                  window.dispatchEvent(new Event("ctxmenu:close"));
                  const domain = c.domain || "";
                  const ip = c.target.split(":")[0];
                  if (domain || ip) setTimeout(() => setCtxMenu({ x: e.clientX, y: e.clientY, domain, ip }), 0);
                }}
              >
                <div className={`w-1.5 h-1.5 rounded-full shrink-0 ${c.active ? "bg-emerald-400" : "bg-zinc-600"}`} />
                <div className="flex-1 min-w-0">
                  <div className="text-[11px] font-mono text-fg-secondary truncate">{label}</div>
                  {c.domain && <div className="text-[9px] text-fg-faint font-mono truncate">{c.target}</div>}
                </div>
                <span className="text-[10px] text-fg-faint font-mono whitespace-nowrap">
                  {c.active ? fmtDuration(Math.floor(Date.now() / 1000 - c.started_at)) : fmtDuration(c.ended_at - c.started_at)}
                </span>
                {(c.bytes_up > 0 || c.bytes_down > 0) && (
                  <span className="text-[9px] text-fg-faint font-mono whitespace-nowrap">
                    ↑{fmtBytes(c.bytes_up)} ↓{fmtBytes(c.bytes_down)}
                  </span>
                )}
                {c.domain && !isExcluded ? (
                  <button
                    onClick={() => handleExclude(c.domain)}
                    className="opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-amber-500/20 text-fg-faint hover:text-amber-400 transition-all shrink-0"
                    title={t.add_exclusion}
                  >
                    <Plus className="w-3 h-3" />
                  </button>
                ) : isExcluded ? (
                  <span className="text-[9px] text-amber-400/70 shrink-0">✓</span>
                ) : null}
              </div>
            );
          })
        )}
      </div>

      {ctxMenu && (
        <div
          className="fixed z-[100] bg-bg-elevated border border-border rounded-lg shadow-2xl py-1 min-w-[180px] text-xs"
          style={{ left: ctxMenu.x, top: ctxMenu.y }}
          onClick={(e) => e.stopPropagation()}
        >
          {ctxMenu.domain && (
            <>
              <button onClick={() => handleExclude(ctxMenu.domain)} className="w-full text-left px-3 py-1.5 hover:bg-white/10 text-fg-secondary hover:text-fg-primary transition-colors">
                {t.exclude_domain}: <span className="font-mono text-amber-400">{ctxMenu.domain}</span>
              </button>
              <button onClick={() => handleExcludeWildcard(ctxMenu.domain)} className="w-full text-left px-3 py-1.5 hover:bg-white/10 text-fg-secondary hover:text-fg-primary transition-colors">
                {t.exclude_wildcard}: <span className="font-mono text-amber-400">*.{ctxMenu.domain.split(".").slice(-2).join(".")}</span>
              </button>
            </>
          )}
          {ctxMenu.ip && (
            <button onClick={() => handleExclude(ctxMenu.ip)} className="w-full text-left px-3 py-1.5 hover:bg-white/10 text-fg-secondary hover:text-fg-primary transition-colors">
              {t.exclude_ip}: <span className="font-mono text-amber-400">{ctxMenu.ip}</span>
            </button>
          )}
          <hr className="border-border my-1" />
          <button
            onClick={() => { navigator.clipboard.writeText(ctxMenu.domain || ctxMenu.ip); setCtxMenu(null); }}
            className="w-full text-left px-3 py-1.5 hover:bg-white/10 text-fg-secondary hover:text-fg-primary transition-colors"
          >
            {t.copy}
          </button>
        </div>
      )}

      <div className="flex justify-end mt-3">
        <button onClick={onClose} className="btn-ghost px-4 py-2 rounded-md text-xs font-medium">{t.cancel}</button>
      </div>
    </div>
  );
}
