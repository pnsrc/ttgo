import { useState, useEffect, useCallback, useRef } from "react";
import { useTranslation } from "../i18n";
import { ReadLogs } from "../api";
import { Refresh } from "../icons";

export function LogsScreen({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  const [logs, setLogs] = useState("");
  const [loading, setLoading] = useState(true);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const logsEndRef = useRef<HTMLDivElement>(null);
  const [ctxMenu, setCtxMenu] = useState<{ x: number; y: number; text: string } | null>(null);

  const fetchLogs = useCallback(async () => {
    try {
      const text = await ReadLogs(500);
      setLogs(text);
    } catch {
      setLogs("Failed to read logs");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { fetchLogs(); }, [fetchLogs]);

  useEffect(() => {
    if (!autoRefresh) return;
    const interval = setInterval(fetchLogs, 2000);
    return () => clearInterval(interval);
  }, [autoRefresh, fetchLogs]);

  useEffect(() => {
    if (logsEndRef.current) logsEndRef.current.scrollIntoView({ behavior: "smooth" });
  }, [logs]);

  useEffect(() => {
    if (!ctxMenu) return;
    const close = () => setCtxMenu(null);
    window.addEventListener("click", close);
    window.addEventListener("ctxmenu:close", close);
    return () => { window.removeEventListener("click", close); window.removeEventListener("ctxmenu:close", close); };
  }, [ctxMenu]);

  return (
    <div className="flex flex-col h-full bg-transparent text-fg-primary p-4 animate-fade-in">
      <div className="flex items-center justify-between mb-3">
        <h2 className="text-xs uppercase tracking-wide text-fg-faint font-medium">{t.logs}</h2>
        <div className="flex items-center gap-2">
          <label className="flex items-center gap-1.5 cursor-pointer">
            <input type="checkbox" checked={autoRefresh} onChange={(e) => setAutoRefresh(e.target.checked)} className="w-3 h-3 rounded border-border bg-bg-elevated text-emerald-500" />
            <span className="text-[11px] text-fg-faint">{t.logs_auto}</span>
          </label>
          <button onClick={fetchLogs} className="p-1.5 rounded text-fg-faint hover:text-fg-primary hover:bg-bg-elevated">
            <Refresh className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>
      <div
        className="flex-1 overflow-y-auto bg-bg-elevated border border-border rounded-lg p-3 font-mono text-[11px] text-fg-secondary leading-relaxed whitespace-pre-wrap break-all"
        onContextMenu={(e) => {
          e.preventDefault();
          const sel = window.getSelection()?.toString();
          const text = sel || logs;
          setCtxMenu({ x: e.clientX, y: e.clientY, text });
        }}
      >
        {loading ? t.loading : (logs || t.logs_empty)}
        <div ref={logsEndRef} />
      </div>

      {ctxMenu && (
        <div className="fixed z-[100] bg-bg-elevated border border-border rounded-lg shadow-2xl py-1 min-w-[140px] text-xs" style={{ left: ctxMenu.x, top: ctxMenu.y }} onClick={(e) => e.stopPropagation()}>
          <button onClick={() => { navigator.clipboard.writeText(ctxMenu.text); setCtxMenu(null); }} className="w-full text-left px-3 py-1.5 hover:bg-white/10 text-fg-secondary hover:text-fg-primary transition-colors">
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
