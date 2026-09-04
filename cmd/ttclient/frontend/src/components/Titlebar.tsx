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
          <div className="flex items-center gap-1.5 ml-2 -mr-1">
            <button onClick={() => Quit()} className="w-3 h-3 rounded-full bg-[#ff5f57] hover:brightness-90 transition-all group flex items-center justify-center" title="Close">
              <svg className="w-1.5 h-1.5 opacity-0 group-hover:opacity-100 transition-opacity" viewBox="0 0 6 6" stroke="#4a0002" strokeWidth="1.2" strokeLinecap="round"><line x1="0.5" y1="0.5" x2="5.5" y2="5.5"/><line x1="5.5" y1="0.5" x2="0.5" y2="5.5"/></svg>
            </button>
            <button onClick={() => WindowMinimise()} className="w-3 h-3 rounded-full bg-[#febc2e] hover:brightness-90 transition-all group flex items-center justify-center" title="Minimize">
              <svg className="w-1.5 h-1.5 opacity-0 group-hover:opacity-100 transition-opacity" viewBox="0 0 6 6"><line x1="0.5" y1="3" x2="5.5" y2="3" stroke="#995700" strokeWidth="1.2" strokeLinecap="round"/></svg>
            </button>
            <button onClick={() => WindowToggleMaximise()} className="w-3 h-3 rounded-full bg-[#28c840] hover:brightness-90 transition-all group flex items-center justify-center" title="Maximize">
              <svg className="w-1.5 h-1.5 opacity-0 group-hover:opacity-100 transition-opacity" viewBox="0 0 6 6"><path d="M1 4.5L3 1.5L5 4.5" stroke="#006500" strokeWidth="1" strokeLinecap="round" strokeLinejoin="round" fill="none"/></svg>
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
