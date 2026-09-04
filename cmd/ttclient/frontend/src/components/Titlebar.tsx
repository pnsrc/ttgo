import { WindowMinimise, WindowToggleMaximise, Quit } from "../../wailsjs/runtime/runtime";
import { useTranslation } from "../i18n";
import { ArrowLeft, SettingsIcon, Activity, LogsIcon, GlobeIcon } from "../icons";

export function Titlebar(props: {
  showBack: boolean;
  onBack: () => void;
  title: string;
  onSettings: () => void;
  onAnalytics: () => void;
  onLogs: () => void;
  onConns: () => void;
  hideSettings?: boolean;
}) {
  const isWindows = /win/i.test(navigator.userAgent) && !/darwin/i.test(navigator.userAgent);

  return (
    <div className="titlebar flex-shrink-0 h-12 flex items-center justify-between px-3 relative z-10 w-full" style={{ WebkitAppRegion: "drag" } as any}>
      <div className="flex items-center gap-2" style={{ WebkitAppRegion: "no-drag" } as any}>
        {props.showBack && (
          <button onClick={props.onBack} className="p-1.5 rounded-full hover:bg-bg-elevated transition-colors">
            <ArrowLeft className="w-4 h-4" />
          </button>
        )}
      </div>
      <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
        <span className="text-sm font-semibold tracking-wide truncate max-w-[200px]">
          {props.title}
        </span>
      </div>
      <div className="flex items-center gap-1" style={{ WebkitAppRegion: "no-drag" } as any}>
        {!props.hideSettings && (
          <>
            <button onClick={props.onConns} className="p-1.5 rounded-full hover:bg-bg-elevated transition-colors text-fg-muted hover:text-fg-primary" title="Connections">
              <GlobeIcon className="w-4 h-4" />
            </button>
            <button onClick={props.onLogs} className="p-1.5 rounded-full hover:bg-bg-elevated transition-colors text-fg-muted hover:text-fg-primary" title="Logs">
              <LogsIcon className="w-4 h-4" />
            </button>
            <button onClick={props.onAnalytics} className="p-1.5 rounded-full hover:bg-bg-elevated transition-colors text-fg-muted hover:text-fg-primary">
              <Activity className="w-4 h-4" />
            </button>
            <button onClick={props.onSettings} className="p-1.5 rounded-full hover:bg-bg-elevated transition-colors text-fg-muted hover:text-fg-primary">
              <SettingsIcon className="w-4 h-4" />
            </button>
          </>
        )}
        {isWindows && (
          <div className="flex items-center ml-2 -mr-1">
            <button onClick={() => WindowMinimise()} className="w-8 h-8 flex items-center justify-center hover:bg-white/10 rounded transition-colors text-fg-faint hover:text-fg-primary">
              <svg width="10" height="1" viewBox="0 0 10 1"><rect width="10" height="1" fill="currentColor"/></svg>
            </button>
            <button onClick={() => WindowToggleMaximise()} className="w-8 h-8 flex items-center justify-center hover:bg-white/10 rounded transition-colors text-fg-faint hover:text-fg-primary">
              <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" strokeWidth="1"><rect x="0.5" y="0.5" width="9" height="9" rx="1.5"/></svg>
            </button>
            <button onClick={() => Quit()} className="w-8 h-8 flex items-center justify-center hover:bg-red-500/80 rounded transition-colors text-fg-faint hover:text-white">
              <svg width="10" height="10" viewBox="0 0 10 10" stroke="currentColor" strokeWidth="1.2" strokeLinecap="round"><line x1="1" y1="1" x2="9" y2="9"/><line x1="9" y1="1" x2="1" y2="9"/></svg>
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
