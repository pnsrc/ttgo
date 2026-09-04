import { useState, useEffect } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { useTranslation } from "../i18n";
import { ReadClipboard } from "../api";
import { Profile } from "../types";
import {
  Activity, Refresh, Folder, Plus, Empty, ChevronRight,
  ClipboardIcon, FileTextIcon,
} from "../icons";

export function ProfilesScreen(props: {
  profiles: Profile[];
  activeID: string;
  onSelect: (id: string) => void;
  onImport: () => void;
  onRefresh: () => void;
  onOpenDir: () => void;
  onPingAll: () => void;
  pingData: Record<string, number>;
  showEnroll: boolean;
  enrollURL: string;
  enrolling: boolean;
  onToggleEnroll: () => void;
  onEnrollURLChange: (v: string) => void;
  onEnroll: () => void;
  onPasteImport: () => void;
  onDeleteProfile: (id: string) => void;
  connecting: boolean;
}) {
  const { t } = useTranslation();
  return (
    <div className="px-4 py-4 space-y-3 animate-fade-in">
      <div className="flex items-center justify-between px-1">
        <h2 className="text-xs uppercase tracking-wide text-fg-faint font-medium">
          {t.servers}
        </h2>
        <div className="flex items-center gap-1">
          <button onClick={props.onPingAll} title="Ping all servers" className="p-1.5 rounded text-fg-faint hover:text-fg-primary hover:bg-bg-elevated">
            <Activity className="w-3.5 h-3.5" />
          </button>
          <button onClick={props.onRefresh} title="Refresh" className="p-1.5 rounded text-fg-faint hover:text-fg-primary hover:bg-bg-elevated">
            <Refresh className="w-3.5 h-3.5" />
          </button>
          <button onClick={props.onOpenDir} title="Open profiles folder" className="p-1.5 rounded text-fg-faint hover:text-fg-primary hover:bg-bg-elevated">
            <Folder className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      <AnimatePresence>
        {props.showEnroll && (
          <motion.div initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: "auto" }} exit={{ opacity: 0, height: 0 }} className="overflow-hidden">
            <div className="bg-bg-elevated border border-border rounded-lg p-3 space-y-2">
              <p className="text-xs text-fg-faint">{t.enroll_hint}</p>
              <div className="flex gap-1">
                <input
                  type="text"
                  value={props.enrollURL}
                  onChange={(e) => props.onEnrollURLChange(e.target.value)}
                  placeholder="https://..."
                  className="flex-1 bg-bg border border-border rounded px-3 py-2 text-xs font-mono text-fg-secondary focus:outline-none focus:border-emerald-500"
                  onKeyDown={(e) => { if (e.key === "Enter") props.onEnroll(); }}
                  autoFocus
                />
                <button
                  onClick={() => ReadClipboard().then((text) => { if (text) props.onEnrollURLChange(text); })}
                  className="px-2 rounded border border-border bg-bg hover:bg-bg-elevated text-fg-faint hover:text-fg-primary transition-colors"
                  title={t.paste_clipboard}
                >
                  <ClipboardIcon className="w-3.5 h-3.5" />
                </button>
              </div>
              <button
                onClick={props.onEnroll}
                disabled={props.enrolling || !props.enrollURL.trim()}
                className="btn-primary w-full py-2 rounded text-xs font-medium bg-emerald-500 text-zinc-900 hover:bg-emerald-400 disabled:opacity-50"
              >
                {props.enrolling ? t.enrolling : t.enroll_button}
              </button>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      {props.profiles.length === 0 ? (
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="flex flex-col items-center text-center py-12 border border-dashed border-border rounded-lg">
          <Empty className="w-8 h-8 text-zinc-700 mb-3" />
          <p className="text-sm text-fg-secondary font-medium">{t.no_profiles}</p>
          <p className="text-xs text-fg-faint mt-1 max-w-[260px]">{t.import_hint}</p>
          <div className="flex flex-col gap-2 mt-4 items-center w-full max-w-[280px]">
            <button onClick={props.onImport} className="btn-primary w-full justify-center py-2.5">
              <Folder className="w-4 h-4" /> {t.import_profile}
            </button>
            <div className="flex gap-2 w-full">
              <button onClick={props.onPasteImport} className="btn-ghost flex-1 justify-center py-2">
                <FileTextIcon className="w-3.5 h-3.5" /> {t.paste_config}
              </button>
              <button onClick={props.onToggleEnroll} className={`btn-ghost flex-1 justify-center py-2 ${props.showEnroll ? "text-emerald-400" : ""}`}>
                <Plus className="w-3.5 h-3.5" /> {t.enroll_link}
              </button>
            </div>
          </div>
        </motion.div>
      ) : (
        <>
          <div className="space-y-2">
            <AnimatePresence>
              {props.profiles.map((p) => (
                <motion.div key={p.id} initial={{ opacity: 0, x: -10 }} animate={{ opacity: 1, x: 0 }} exit={{ opacity: 0, scale: 0.9 }}>
                  <ProfileRow
                    profile={p}
                    active={p.id === props.activeID}
                    connecting={props.connecting}
                    onClick={() => props.onSelect(p.id)}
                    ping={props.pingData[p.id]}
                    onDelete={() => props.onDeleteProfile(p.id)}
                  />
                </motion.div>
              ))}
            </AnimatePresence>
          </div>
          <div className="flex gap-1.5 mt-2">
            <button onClick={props.onImport} className="btn-ghost flex-1 justify-center py-2" title={t.import_another}>
              <Folder className="w-3.5 h-3.5" />
            </button>
            <button onClick={props.onPasteImport} className="btn-ghost flex-1 justify-center py-2" title={t.paste_config}>
              <FileTextIcon className="w-3.5 h-3.5" />
            </button>
            <button onClick={props.onToggleEnroll} className={`btn-ghost flex-1 justify-center py-2 ${props.showEnroll ? "text-emerald-400" : ""}`} title={t.enroll_link}>
              <Plus className="w-3.5 h-3.5" />
            </button>
          </div>
        </>
      )}
    </div>
  );
}

function ProfileRow(props: { profile: Profile; active: boolean; connecting?: boolean; onClick: () => void; ping?: number; onDelete?: () => void }) {
  const { profile, active, ping } = props;
  const connecting = props.connecting && active;
  const { t } = useTranslation();
  const [ctxMenu, setCtxMenu] = useState<{ x: number; y: number } | null>(null);

  useEffect(() => {
    if (!ctxMenu) return;
    const close = () => setCtxMenu(null);
    window.addEventListener("click", close);
    window.addEventListener("contextmenu", close);
    window.addEventListener("ctxmenu:close", close);
    return () => {
      window.removeEventListener("click", close);
      window.removeEventListener("contextmenu", close);
      window.removeEventListener("ctxmenu:close", close);
    };
  }, [ctxMenu]);

  return (
    <>
      <button
        onClick={props.onClick}
        onContextMenu={(e) => { e.preventDefault(); window.dispatchEvent(new Event("ctxmenu:close")); setTimeout(() => setCtxMenu({ x: e.clientX, y: e.clientY }), 0); }}
        className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg border transition-all duration-200 text-left
          ${connecting
            ? "bg-amber-500/15 border-amber-500/30 shadow-[0_0_15px_rgba(245,158,11,0.05)]"
            : active
            ? "bg-emerald-500/15 border-emerald-500/30 shadow-[0_0_15px_rgba(16,185,129,0.05)]"
            : "bg-transparent border-transparent hover:bg-zinc-500/10 hover:border-zinc-500/20"
          }`}
      >
        <div className={`w-2 h-2 rounded-full shrink-0 ${connecting ? "bg-amber-400 animate-pulse" : active ? "bg-emerald-400 animate-pulse" : "bg-zinc-700"}`} />
        <div className="min-w-0 flex-1">
          <div className="text-sm font-medium text-fg-primary truncate">{profile.name}</div>
          <div className="text-[11px] text-fg-faint truncate font-mono">{profile.username}@{profile.endpoint}</div>
        </div>
        {ping !== undefined && (
          <div className={`text-xs mr-2 font-mono ${ping > 150 ? "text-red-400" : ping > 80 ? "text-amber-400" : "text-emerald-400"}`}>
            {ping >= 0 ? `${ping}ms` : "err"}
          </div>
        )}
        <ChevronRight className="w-4 h-4 text-fg-faint shrink-0" />
      </button>
      {ctxMenu && (
        <div
          className="fixed z-[100] bg-bg-elevated border border-border rounded-lg shadow-2xl py-1 min-w-[160px] text-xs"
          style={{ left: ctxMenu.x, top: ctxMenu.y }}
          onClick={(e) => e.stopPropagation()}
        >
          <button
            onClick={() => { navigator.clipboard.writeText(profile.endpoint); setCtxMenu(null); }}
            className="w-full text-left px-3 py-1.5 hover:bg-white/10 text-fg-secondary hover:text-fg-primary transition-colors"
          >
            {t.copy}: <span className="font-mono text-fg-faint">{profile.endpoint}</span>
          </button>
          <button
            onClick={() => { navigator.clipboard.writeText(`${profile.username}@${profile.endpoint}`); setCtxMenu(null); }}
            className="w-full text-left px-3 py-1.5 hover:bg-white/10 text-fg-secondary hover:text-fg-primary transition-colors"
          >
            {t.copy}: <span className="font-mono text-fg-faint">{profile.username}@{profile.endpoint}</span>
          </button>
          {props.onDelete && (
            <>
              <hr className="border-border my-1" />
              <button
                onClick={() => { setCtxMenu(null); props.onDelete!(); }}
                className="w-full text-left px-3 py-1.5 hover:bg-red-500/10 text-red-400 hover:text-red-300 transition-colors"
              >
                {t.delete}
              </button>
            </>
          )}
        </div>
      )}
    </>
  );
}
