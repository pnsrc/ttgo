import { useEffect, useState, useCallback, useRef } from "react";
import { useTranslation } from "./i18n";
import { motion, AnimatePresence } from "framer-motion";
import {
  ConnectProfile,
  Disconnect,
  Status,
  Profiles,
  ImportProfile,
  DeleteProfile,
  OpenProfilesDir,
  ReadProfileContent,
  SaveProfileContent,
  GetGlobalSettings,
  SaveGlobalSettings,
  PingAll,
} from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime/runtime";
import { StatusPayload, Snapshot, Profile, GlobalSettings } from "./types";
import {
  Logo,
  ArrowLeft,
  Power,
  Alert,
  Plus,
  Trash,
  Folder,
  Refresh,
  ChevronRight,
  Empty,
  SettingsIcon,
  Activity,
  ShareIcon,
} from "./icons";
import { QRCodeSVG } from "qrcode.react";
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer } from "recharts";

const DEFAULT_SNAP: Snapshot = {
  state: "disconnected",
  bytes_in: 0,
  bytes_out: 0,
  tunnels: 0,
  active_conn: 0,
  uptime_sec: 0,
};

export default function App() {
  const [snap, setSnap] = useState<Snapshot>(DEFAULT_SNAP);
  const [activeID, setActiveID] = useState("");
  const [profiles, setProfiles] = useState<Profile[]>([]);
  const [selectedID, setSelectedID] = useState<string | null>(null);
  const [toast, setToast] = useState<{msg: string, type: "error" | "info"} | null>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [showSettings, setShowSettings] = useState(false);
  const [showAnalytics, setShowAnalytics] = useState(false);
  const [pingData, setPingData] = useState<Record<string, number>>({});
  const [isWide, setIsWide] = useState(window.innerWidth >= 768);
  const { t } = useTranslation();
  const [theme, setTheme] = useState("glass");

  const [speedHistory, setSpeedHistory] = useState<{ rx: number; tx: number }[]>([]);
  const lastBytes = useRef({ rx: 0, tx: 0, time: 0 });

  useEffect(() => {
    GetGlobalSettings().then((s: any) => {
      if (s && s.theme) setTheme(s.theme);
    }).catch(console.error);
  }, []);

  useEffect(() => {
    const now = Date.now();
    const dt = (now - lastBytes.current.time) / 1000;
    if (dt > 0 && lastBytes.current.time > 0 && snap.state === "connected") {
      const rxSpeed = Math.max(0, snap.bytes_in - lastBytes.current.rx) / dt;
      const txSpeed = Math.max(0, snap.bytes_out - lastBytes.current.tx) / dt;
      setSpeedHistory((prev) => {
        const next = [...prev, { rx: rxSpeed, tx: txSpeed }];
        return next.slice(-60); // 60 points max
      });
    } else if (snap.state === "disconnected" || snap.state === "error") {
      setSpeedHistory([]);
    }
    lastBytes.current = { rx: snap.bytes_in, tx: snap.bytes_out, time: now };
  }, [snap.bytes_in, snap.bytes_out, snap.state]);

  useEffect(() => {
    const handler = () => setIsWide(window.innerWidth >= 768);
    window.addEventListener("resize", handler);
    return () => window.removeEventListener("resize", handler);
  }, []);

  function showToast(msg: string, type: "error" | "info" = "info") {
    setToast({ msg, type });
    setTimeout(() => setToast(null), 4000);
  }

  useEffect(() => {
    const isMac = /mac/i.test(navigator.platform);
    if (isMac) {
      document.body.classList.add("platform-macos");
    }
    return () => {
      document.body.classList.remove("platform-macos");
    };
  }, []);

  const refreshProfiles = useCallback(async () => {
    try {
      const list = (await Profiles()) as unknown as Profile[];
      setProfiles(list ?? []);
    } catch (e: any) {
      showToast(String(e?.message ?? e), "error");
    }
  }, []);

  useEffect(() => {
    Status().then((s: any) => {
      setSnap(s.snapshot);
      setActiveID(s.active_profile_id);
    }).catch(() => {});
    const unsub = EventsOn("status", (s: StatusPayload) => {
      setSnap(s.snapshot);
      setActiveID(s.active_profile_id);
    });
    refreshProfiles();
    return () => unsub();
  }, [refreshProfiles]);

  // Когда подключение активно, переключаем UI на detail-экран активного профиля
  useEffect(() => {
    if (activeID) setSelectedID(activeID);
  }, [activeID]);

  // Ошибка из snapshot → toast
  useEffect(() => {
    if (snap.state === "error" && snap.error) {
      showToast(snap.error, "error");
    }
  }, [snap.state, snap.error]);

  async function handlePingAll() {
    showToast(t.pinging, "info");
    try {
      const res = await PingAll();
      setPingData(res || {});
      showToast(t.ping_finished, "info");
    } catch (e: any) {
      showToast(String(e?.message ?? e), "error");
    }
  }

  async function importProfile() {
    try {
      const p = (await ImportProfile()) as unknown as Profile | null;
      if (p) {
        await refreshProfiles();
        setSelectedID(p.id);
      }
    } catch (e: any) {
      showToast(String(e?.message ?? e), "error");
    }
  }

  async function removeProfile(id: string) {
    if (!confirm("Delete this profile?")) return;
    try {
      await DeleteProfile(id);
      if (selectedID === id) setSelectedID(null);
      await refreshProfiles();
    } catch (e: any) {
      showToast(String(e?.message ?? e), "error");
    }
  }

  async function connect(id: string) {
    try {
      await ConnectProfile(id);
    } catch (e: any) {
      showToast(String(e?.message ?? e), "error");
    }
  }

  async function disconnect() {
    try {
      await Disconnect();
    } catch (e: any) {
      showToast(String(e?.message ?? e), "error");
    }
  }

  async function handleSaveContent(content: string) {
    if (!selectedID) return;
    await SaveProfileContent(selectedID, content);
    await refreshProfiles();
    setIsEditing(false);
  }

  const effectiveSelectedID = selectedID || (isWide && profiles.length > 0 ? (activeID || profiles[0].id) : null);
  const selected = profiles.find((p) => p.id === effectiveSelectedID) ?? null;

  return (
    <div data-theme={theme} className="h-screen flex flex-col bg-bg text-fg-primary relative overflow-hidden">
      <Titlebar
        showBack={(!!selectedID) || isEditing || showSettings || showAnalytics}
        title={isEditing ? `${t.edit_config} ${selected?.name}` : showSettings ? t.settings : showAnalytics ? t.analytics : (selected?.name ?? "FireTunnel")}
        onBack={() => { isEditing ? setIsEditing(false) : showSettings ? setShowSettings(false) : showAnalytics ? setShowAnalytics(false) : setSelectedID(null) }}
        onSettings={() => { setShowSettings(true); setShowAnalytics(false); }}
        onAnalytics={() => { setShowAnalytics(true); setShowSettings(false); }}
        hideSettings={isEditing || showSettings || showAnalytics}
      />

      <div className="flex-1 flex overflow-hidden">
        {/* Left Pane (Profiles List) */}
        <div className={`w-full md:w-80 flex-shrink-0 md:block ${
          (showSettings || selectedID || isEditing) ? "hidden" : "block"
        } overflow-y-auto bg-bg-subtle`}>
          <ProfilesScreen
            profiles={profiles}
            activeID={activeID}
            onSelect={(id) => {
              setSelectedID(id);
              setIsEditing(false);
              setShowSettings(false);
            }}
            onImport={importProfile}
            onRefresh={refreshProfiles}
            onOpenDir={() => OpenProfilesDir()}
            onPingAll={handlePingAll}
            pingData={pingData}
          />
        </div>

        {/* Right Pane (Details or Settings) */}
        <div className={`flex-1 md:flex flex-col ${
          (showSettings || showAnalytics || selectedID || isEditing) ? "flex" : "hidden"
        } overflow-y-auto bg-transparent`}>
          <AnimatePresence mode="wait">
            {showSettings ? (
              <motion.div key="settings" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} className="h-full">
                <GlobalSettingsScreen onSave={(t) => { setShowSettings(false); if (t) setTheme(t); showToast("Settings saved", "info"); }} onCancel={() => setShowSettings(false)} />
              </motion.div>
            ) : showAnalytics ? (
              <motion.div key="analytics" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} className="h-full">
                <AnalyticsScreen history={speedHistory} onCancel={() => setShowAnalytics(false)} />
              </motion.div>
            ) : selected ? (
              isEditing ? (
                <motion.div key="edit" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} className="h-full">
                  <EditScreen
                    profile={selected}
                    onSave={handleSaveContent}
                    onCancel={() => setIsEditing(false)}
                  />
                </motion.div>
              ) : (
                <motion.div key="detail" initial={{ opacity: 0, scale: 0.98 }} animate={{ opacity: 1, scale: 1 }} exit={{ opacity: 0, scale: 0.98 }} className="h-full">
                  <DetailScreen
                    profile={selected}
                    snap={snap}
                    isActive={selected.id === activeID}
                    speedHistory={speedHistory}
                    onConnect={() => connect(selected.id)}
                    onDisconnect={disconnect}
                    onDelete={() => removeProfile(selected.id)}
                    onEdit={() => setIsEditing(true)}
                  />
                </motion.div>
              )
            ) : (
               <motion.div key="empty" initial={{ opacity: 0 }} animate={{ opacity: 1 }} exit={{ opacity: 0 }} className="flex-1 flex flex-col items-center justify-center text-fg-faint h-full p-8 text-center border-dashed border border-border m-8 rounded-xl opacity-50">
                 <Logo className="w-12 h-12 mb-4 text-fg-faint" />
                 <p className="text-sm">{t.select_sidebar}</p>
               </motion.div>
            )}
          </AnimatePresence>
        </div>
      </div>

      {toast && (
        <div className={`absolute bottom-4 left-4 right-4 animate-slide-up border px-3.5 py-2.5 rounded-lg flex items-start gap-2.5 text-sm shadow-lg backdrop-blur ${toast.type === "error" ? "bg-red-500/10 border-red-500/30 text-red-200" : "bg-emerald-500/10 border-emerald-500/30 text-emerald-200"}`}>
          {toast.type === "error" ? <Alert className="w-4 h-4 mt-0.5 shrink-0" /> : <Activity className="w-4 h-4 mt-0.5 shrink-0" />}
          <div className="flex-1 leading-tight break-words">{toast.msg}</div>
          <button onClick={() => setToast(null)} className="text-fg-muted hover:text-zinc-200 text-xs px-1">
            ✕
          </button>
        </div>
      )}
    </div>
  );
}

// ── Titlebar ─────────────────────────────────────────────────────────────────

function Titlebar(props: {
  showBack: boolean;
  onBack: () => void;
  title: string;
  onSettings: () => void;
  onAnalytics: () => void;
  hideSettings?: boolean;
}) {
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
            <button onClick={props.onAnalytics} className="p-1.5 rounded-full hover:bg-bg-elevated transition-colors text-fg-muted hover:text-fg-primary">
              <Activity className="w-4 h-4" />
            </button>
            <button onClick={props.onSettings} className="p-1.5 rounded-full hover:bg-bg-elevated transition-colors text-fg-muted hover:text-fg-primary">
              <SettingsIcon className="w-4 h-4" />
            </button>
          </>
        )}
      </div>
    </div>
  );
}

// ── Profiles list ────────────────────────────────────────────────────────────

function ProfilesScreen(props: {
  profiles: Profile[];
  activeID: string;
  onSelect: (id: string) => void;
  onImport: () => void;
  onRefresh: () => void;
  onOpenDir: () => void;
  onPingAll: () => void;
  pingData: Record<string, number>;
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
          <button
            onClick={props.onRefresh}
            title="Refresh"
            className="p-1.5 rounded text-fg-faint hover:text-fg-primary hover:bg-bg-elevated"
          >
            <Refresh className="w-3.5 h-3.5" />
          </button>
          <button
            onClick={props.onOpenDir}
            title="Open profiles folder"
            className="p-1.5 rounded text-fg-faint hover:text-fg-primary hover:bg-bg-elevated"
          >
            <Folder className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {props.profiles.length === 0 ? (
        <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} className="flex flex-col items-center text-center py-12 border border-dashed border-border rounded-lg">
          <Empty className="w-8 h-8 text-zinc-700 mb-3" />
          <p className="text-sm text-fg-secondary font-medium">{t.no_profiles}</p>
          <p className="text-xs text-fg-faint mt-1 max-w-[260px]">
            {t.import_hint}
          </p>
          <button onClick={props.onImport} className="btn-primary mt-4">
            <Plus className="w-3.5 h-3.5" /> {t.import_profile}
          </button>
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
                    onClick={() => props.onSelect(p.id)}
                    ping={props.pingData[p.id]}
                  />
                </motion.div>
              ))}
            </AnimatePresence>
          </div>
          <button onClick={props.onImport} className="btn-ghost w-full justify-center mt-2">
            <Plus className="w-3.5 h-3.5" /> {t.import_another}
          </button>
        </>
      )}
    </div>
  );
}

function ProfileRow(props: { profile: Profile; active: boolean; onClick: () => void; ping?: number }) {
  const { profile, active, ping } = props;
  return (
    <button
      onClick={props.onClick}
      className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg border transition-all duration-200 text-left
        ${
          active
            ? "bg-emerald-500/15 border-emerald-500/30 shadow-[0_0_15px_rgba(16,185,129,0.05)]"
            : "bg-transparent border-transparent hover:bg-zinc-500/10 hover:border-zinc-500/20"
        }`}
    >
      <div
        className={`w-2 h-2 rounded-full shrink-0 ${
          active ? "bg-emerald-400 animate-pulse" : "bg-zinc-700"
        }`}
      />
      <div className="min-w-0 flex-1">
        <div className="text-sm font-medium text-fg-primary truncate">{profile.name}</div>
        <div className="text-[11px] text-fg-faint truncate font-mono">
          {profile.username}@{profile.endpoint}
        </div>
      </div>
      {ping !== undefined && (
        <div className={`text-xs mr-2 font-mono ${ping > 150 ? "text-red-400" : ping > 80 ? "text-amber-400" : "text-emerald-400"}`}>
          {ping >= 0 ? `${ping}ms` : "err"}
        </div>
      )}
      <ChevronRight className="w-4 h-4 text-fg-faint shrink-0" />
    </button>
  );
}

// ── Detail / connection screen ───────────────────────────────────────────────

function DetailScreen(props: {
  profile: Profile;
  snap: Snapshot;
  isActive: boolean;
  speedHistory: { rx: number; tx: number }[];
  onConnect: () => void;
  onDisconnect: () => void;
  onDelete: () => void;
  onEdit: () => void;
}) {
  const { profile, snap, isActive, speedHistory } = props;
  const isOn = isActive && snap.state === "connected";
  const isPending = isActive && snap.state === "connecting";
  const { t } = useTranslation();
  const [showQR, setShowQR] = useState(false);
  const [qrData, setQrData] = useState("");

  async function handleShare() {
    if (showQR) {
      setShowQR(false);
      return;
    }
    try {
      const content = await ReadProfileContent(profile.id);
      setQrData(btoa(content));
      setShowQR(true);
    } catch (e) {
      console.error("Failed to read profile content", e);
    }
  }

  const currentRxSpeed = speedHistory.length > 0 ? speedHistory[speedHistory.length - 1].rx : 0;
  const currentTxSpeed = speedHistory.length > 0 ? speedHistory[speedHistory.length - 1].tx : 0;

  return (
    <div className="relative flex flex-col items-center min-h-full px-6 py-8 overflow-hidden">

      <div className="relative z-10 flex flex-col items-center w-full max-w-xs mt-4">
        <motion.button
          whileHover={{ scale: 1.05 }}
          whileTap={{ scale: 0.95 }}
          onClick={isOn || isPending ? props.onDisconnect : props.onConnect}
          disabled={isPending}
          className={`relative group flex items-center justify-center w-40 h-40 rounded-full transition-all duration-300
            ${
              isOn
                ? "bg-emerald-500/15 border-2 border-emerald-400 shadow-[0_0_60px_-10px_rgba(52,211,153,0.5)]"
                : isPending
                ? "bg-amber-500/15 border-2 border-amber-400/60"
                : "bg-bg-elevated border-2 border-zinc-700 hover:border-zinc-500"
            }`}
        >
          {isOn && (
            <span className="absolute inset-0 rounded-full border-2 border-emerald-400/40 animate-ping" />
          )}
          <Power
            className={`w-14 h-14 transition-colors ${
              isOn
                ? "text-emerald-400"
                : isPending
                ? "text-amber-400 animate-pulse"
                : "text-fg-faint group-hover:text-fg-secondary"
            }`}
          />
        </motion.button>

        <div className="mt-6 text-center">
          <motion.div
            key={isOn ? "on" : isPending ? "pending" : snap.state === "error" ? "err" : "off"}
            initial={{ opacity: 0, y: 5 }}
            animate={{ opacity: 1, y: 0 }}
            className={`text-lg font-medium ${
              isOn
                ? "text-emerald-400"
                : isPending
                ? "text-amber-400"
                : snap.state === "error" && isActive
                ? "text-red-400"
                : "text-fg-secondary"
            }`}
          >
            {isOn
              ? t.protected
              : isPending
              ? t.connecting
              : snap.state === "error" && isActive
              ? t.error
              : t.tap_to_connect}
          </motion.div>
          <div className="mt-1 text-xs text-fg-faint">{profile.username}@{profile.endpoint}</div>
        </div>
      </div>

      <AnimatePresence>
        {isOn && (
          <motion.div initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: 10 }} className="relative z-10 mt-7 w-full max-w-xs flex flex-col backdrop-blur-md bg-bg-subtle/30 p-2 rounded-xl border border-white/5 shadow-2xl">
            <StatRow label={t.uptime} value={fmtDuration(snap.uptime_sec)} />
            <StatRow label={t.tunnels} value={`${snap.active_conn} / ${snap.tunnels}`} />
            <StatRow label={t.download} value={`${fmtBytes(currentRxSpeed)}/s`} subValue={`${fmtBytes(snap.bytes_in)} ${t.total}`} color="text-emerald-400" />
            <StatRow label={t.upload} value={`${fmtBytes(currentTxSpeed)}/s`} subValue={`${fmtBytes(snap.bytes_out)} ${t.total}`} color="text-amber-400" />
            {snap.tun_name && <StatRow label={t.interface} value={snap.tun_name} mono />}
          </motion.div>
        )}
      </AnimatePresence>

      {/* Profile details */}
      <div className="relative z-10 mt-7 w-full max-w-xs">
        <div className="text-[11px] uppercase tracking-wide text-fg-faint font-medium px-1 mb-2">
          {t.profile}
        </div>
        <div className="space-y-2">
          <DetailRow label={t.address} value={profile.endpoint} mono />
          {profile.toml?.endpoint?.hostname && (
            <DetailRow label={t.hostname} value={profile.toml.endpoint.hostname} mono />
          )}
          {profile.toml?.endpoint?.upstream_protocol && (
            <DetailRow label={t.protocol} value={profile.toml.endpoint.upstream_protocol} />
          )}
          {profile.toml?.endpoint?.skip_verification && (
            <DetailRow label={t.tls_verify} value={t.skipped} warn />
          )}
          {profile.toml?.exclusions && profile.toml.exclusions.length > 0 && (
            <DetailRow label={t.exclusions} value={`${profile.toml.exclusions.length} ${t.rules}`} />
          )}
        </div>

        {!isActive && (
          <div className="flex gap-2 mt-5">
            <button
              onClick={handleShare}
              className="flex-[0.5] inline-flex items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-xs text-fg-secondary hover:text-fg-primary hover:bg-zinc-700/50 transition-colors"
              title={t.share}
            >
              <ShareIcon className="w-3.5 h-3.5" />
            </button>
            <button
              onClick={props.onEdit}
              className="flex-1 inline-flex items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-xs text-fg-secondary hover:text-fg-primary hover:bg-zinc-700/50 transition-colors"
            >
              {t.edit_config}
            </button>
            <button
              onClick={props.onDelete}
              className="flex-1 inline-flex items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-xs text-red-400 hover:text-red-300 hover:bg-red-500/10 transition-colors"
            >
              <Trash className="w-3.5 h-3.5" /> {t.delete}
            </button>
          </div>
        )}
      </div>

      {/* QR Code Modal Overlay inside DetailScreen */}
      <AnimatePresence>
        {showQR && (
          <motion.div
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: 20 }}
            className="absolute inset-0 z-50 flex flex-col items-center justify-center bg-bg/90 backdrop-blur-sm p-6"
          >
            <div className="bg-white p-4 rounded-xl shadow-2xl mb-4">
              <QRCodeSVG value={qrData} size={200} level="M" />
            </div>
            <p className="text-fg-secondary text-sm font-medium">{t.qr_hint}</p>
            <button onClick={() => setShowQR(false)} className="mt-6 btn-ghost px-6 py-2 rounded-full">
              {t.cancel}
            </button>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}

function StatRow(props: { label: string; value: string; subValue?: string; color?: string; mono?: boolean }) {
  return (
    <div className="flex items-center justify-between px-3 py-2 border-b border-white/5 last:border-0">
      <span className="text-xs text-fg-faint">{props.label}</span>
      <div className="text-right flex flex-col items-end">
        <span
          className={`text-sm tabular-nums ${props.color ?? "text-fg-primary"} ${
            props.mono ? "font-mono text-xs" : ""
          }`}
        >
          {props.value}
        </span>
        {props.subValue && <span className="text-[10px] text-fg-faint font-mono mt-0.5">{props.subValue}</span>}
      </div>
    </div>
  );
}

function DetailRow(props: { label: string; value: string; mono?: boolean; warn?: boolean }) {
  return (
    <div className="flex items-center justify-between gap-3 px-3 py-1.5">
      <span className="text-[11px] text-fg-faint">{props.label}</span>
      <span
        className={`text-xs truncate ${props.warn ? "text-amber-400" : "text-fg-secondary"} ${
          props.mono ? "font-mono" : ""
        }`}
      >
        {props.value}
      </span>
    </div>
  );
}

// ── helpers ──────────────────────────────────────────────────────────────────

function fmtBytes(n: number): string {
  n = Math.round(n);
  if (n < 1024) return `${n} B`;
  const u = ["KB", "MB", "GB", "TB"];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < u.length - 1) {
    v /= 1024;
    i++;
  }
  return `${v.toFixed(v < 10 ? 2 : 1)} ${u[i]}`;
}

function fmtDuration(s: number): string {
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${s % 60}s`;
  const h = Math.floor(m / 60);
  return `${h}h ${m % 60}m`;
}

function SpeedGraph({ history }: { history: { rx: number; tx: number }[] }) {
  if (history.length < 2) return null;
  const maxRx = Math.max(...history.map(h => h.rx), 1024 * 10);
  const maxTx = Math.max(...history.map(h => h.tx), 1024 * 10);
  const max = Math.max(maxRx, maxTx);
  
  const width = 1000;
  const height = 400; // large viewbox
  
  const toPoint = (val: number, i: number) => {
    const x = (i / 59) * width;
    const y = height - (val / max) * height * 0.6; // max height is 60%
    return `${x},${y}`;
  };

  const rxPath = history.map((h, i) => toPoint(h.rx, i)).join(" L ");
  const txPath = history.map((h, i) => toPoint(h.tx, i)).join(" L ");

  return (
    <svg viewBox={`0 0 ${width} ${height}`} className="absolute inset-0 w-full h-full pointer-events-none opacity-60 mix-blend-screen" preserveAspectRatio="none">
      <defs>
        <linearGradient id="gradRx" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#10b981" stopOpacity="0.5" />
          <stop offset="100%" stopColor="#10b981" stopOpacity="0" />
        </linearGradient>
        <linearGradient id="gradTx" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#f59e0b" stopOpacity="0.5" />
          <stop offset="100%" stopColor="#f59e0b" stopOpacity="0" />
        </linearGradient>
      </defs>
      
      {/* TX Area & Line */}
      <path
        d={`M 0,${height} L ${txPath} L ${(history.length-1)/59*width},${height} Z`}
        fill="url(#gradTx)"
      />
      <path
        d={`M ${txPath}`}
        fill="none"
        stroke="#f59e0b"
        strokeWidth="3"
        strokeLinecap="round"
        strokeLinejoin="round"
        className="drop-shadow-[0_0_12px_rgba(245,158,11,0.8)]"
      />

      {/* RX Area & Line */}
      <path
        d={`M 0,${height} L ${rxPath} L ${(history.length-1)/59*width},${height} Z`}
        fill="url(#gradRx)"
      />
      <path
        d={`M ${rxPath}`}
        fill="none"
        stroke="#10b981"
        strokeWidth="3"
        strokeLinecap="round"
        strokeLinejoin="round"
        className="drop-shadow-[0_0_12px_rgba(16,185,129,0.8)]"
      />
    </svg>
  );
}

function GlobalSettingsScreen(props: { onSave: (theme?: string) => void; onCancel: () => void }) {
  const { t, lang, setLang } = useTranslation();
  const [settings, setSettings] = useState<GlobalSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [rawExclusions, setRawExclusions] = useState("");

  useEffect(() => {
    GetGlobalSettings()
      .then((s: any) => {
        setSettings(s || { bypass_domains: false, global_exclusions: [], auto_connect: true, language: lang, theme: "glass", enable_adblock: false });
        setRawExclusions((s.global_exclusions || []).join("\n"));
      })
      .catch((e: any) => setError(String(e?.message ?? e)))
      .finally(() => setLoading(false));
  }, [lang]);

  async function handleSave() {
    if (!settings) return;
    setSaving(true);
    setError(null);
    try {
      const exclusions = rawExclusions.split("\n").map(s => s.trim()).filter(s => s.length > 0);
      const newSettings = {
        ...settings,
        global_exclusions: exclusions,
      };
      await SaveGlobalSettings(newSettings);
      setLang(newSettings.language || "en");
      props.onSave(newSettings.theme);
    } catch (e: any) {
      setError(String(e?.message ?? e));
      setSaving(false);
    }
  }

  if (loading) return <div className="p-4 text-center text-fg-faint text-sm">{t.loading}</div>;
  if (!settings) return <div className="p-4 text-center text-red-400 text-sm">{t.error}</div>;

  return (
    <div className="flex flex-col h-full bg-bg text-fg-primary p-4 animate-fade-in">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-xs uppercase tracking-wide text-fg-faint font-medium">{t.settings}</h2>
      </div>

      <div className="space-y-4">
        <div className="bg-bg-subtle border border-border rounded-lg p-4 space-y-4">
          <label className="flex items-center justify-between cursor-pointer group">
            <span className="text-sm font-medium group-hover:text-fg-primary text-fg-secondary transition-colors">{t.language}</span>
            <select
              value={settings.language || lang}
              onChange={(e) => setSettings({ ...settings, language: e.target.value })}
              className="bg-bg-elevated border border-border rounded text-sm px-2 py-1 outline-none focus:border-emerald-500 text-fg-secondary"
            >
              <option value="en">English</option>
              <option value="ru">Русский</option>
            </select>
          </label>

          <label className="flex items-center justify-between cursor-pointer group">
            <span className="text-sm font-medium group-hover:text-fg-primary text-fg-secondary transition-colors">Theme</span>
            <select
              value={settings.theme || "glass"}
              onChange={(e) => setSettings({ ...settings, theme: e.target.value })}
              className="bg-bg-elevated border border-border rounded text-sm px-2 py-1 outline-none focus:border-emerald-500 text-fg-secondary"
            >
              <option value="glass">Glass (macOS)</option>
              <option value="dark">Dark</option>
              <option value="light">Light</option>
              <option value="epic">Epic Style 🚀</option>
            </select>
          </label>

          <label className="flex items-center gap-3 cursor-pointer group">
            <input
              type="checkbox"
              checked={settings.auto_connect !== false}
              onChange={(e) => setSettings({ ...settings, auto_connect: e.target.checked })}
              className="w-4 h-4 rounded border-border bg-bg-elevated text-emerald-500 focus:ring-emerald-500 focus:ring-offset-0"
            />
            <span className="text-sm font-medium group-hover:text-fg-primary text-fg-secondary transition-colors">{t.auto_connect}</span>
          </label>

          <label className="flex items-center gap-3 cursor-pointer group">
            <input
              type="checkbox"
              checked={settings.enable_adblock}
              onChange={(e) => setSettings({ ...settings, enable_adblock: e.target.checked })}
              className="w-4 h-4 rounded border-border bg-bg-elevated text-emerald-500 focus:ring-emerald-500 focus:ring-offset-0"
            />
            <div className="flex flex-col">
              <span className="text-sm font-medium group-hover:text-fg-primary text-fg-secondary transition-colors">{t.adblock}</span>
              <span className="text-[11px] text-fg-faint">{t.adblock_desc}</span>
            </div>
          </label>

          <hr className="border-border" />

          <label className="flex items-center gap-3 cursor-pointer group">
            <input
              type="checkbox"
              checked={settings.bypass_domains}
              onChange={(e) => setSettings({ ...settings, bypass_domains: e.target.checked })}
              className="w-4 h-4 rounded border-border bg-bg-elevated text-emerald-500 focus:ring-emerald-500 focus:ring-offset-0"
            />
            <span className="text-sm font-medium group-hover:text-fg-primary text-fg-secondary transition-colors">{t.enable_bypass}</span>
          </label>
          
          <div className="pl-7 space-y-2">
            <textarea
              value={rawExclusions}
              onChange={(e) => setRawExclusions(e.target.value)}
              disabled={!settings.bypass_domains}
              placeholder="*.ru\n*.example.com\n192.168.1.0/24"
              className="w-full bg-bg-elevated border border-border rounded-md p-3 text-xs font-mono text-fg-secondary resize-none focus:outline-none focus:border-zinc-500 disabled:opacity-50 h-32"
              spellCheck={false}
            />
            <p className="text-[11px] text-fg-faint leading-relaxed">
              {t.bypass_hint}
            </p>
          </div>
        </div>
      </div>

      {error && <div className="text-red-400 text-xs mt-3 font-medium">{error}</div>}
      <div className="flex-1" />
      <div className="flex justify-end gap-2 mt-4">
        <button onClick={props.onCancel} disabled={saving} className="btn-ghost px-4 py-2 rounded-md text-xs font-medium">
          {t.cancel}
        </button>
        <motion.button whileTap={{ scale: 0.95 }} onClick={handleSave} disabled={saving} className="btn-primary px-4 py-2 rounded-md text-xs font-medium bg-emerald-500 text-zinc-900 hover:bg-emerald-400 disabled:opacity-50">
          {saving ? t.saving : t.save}
        </motion.button>
      </div>
    </div>
  );
}

function EditScreen(props: { profile: Profile; onSave: (content: string) => Promise<void>; onCancel: () => void }) {
  const [content, setContent] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    ReadProfileContent(props.profile.id)
      .then(setContent)
      .catch((e: any) => setError(String(e?.message ?? e)))
      .finally(() => setLoading(false));
  }, [props.profile.id]);

  async function handleSave() {
    setSaving(true);
    setError(null);
    try {
      await props.onSave(content);
    } catch (e: any) {
      setError(String(e?.message ?? e));
      setSaving(false);
    }
  }

  return (
    <div className="flex flex-col h-full bg-transparent text-fg-primary p-4 animate-fade-in">
      {loading ? (
        <div className="flex-1 flex items-center justify-center text-fg-faint text-sm">Loading...</div>
      ) : (
        <textarea
          value={content}
          onChange={(e) => setContent(e.target.value)}
          className="flex-1 w-full bg-bg-elevated border border-border rounded-lg p-3 text-xs font-mono text-fg-secondary resize-none focus:outline-none focus:border-zinc-500 mb-4"
          spellCheck={false}
        />
      )}
      {error && <div className="text-red-400 text-xs mb-3 font-medium">{error}</div>}
      <div className="flex justify-end gap-2">
        <button onClick={props.onCancel} disabled={saving} className="btn-ghost px-4 py-2 rounded-md text-xs font-medium">
          Cancel
        </button>
        <button onClick={handleSave} disabled={saving || loading} className="btn-primary px-4 py-2 rounded-md text-xs font-medium bg-emerald-500 text-zinc-900 hover:bg-emerald-400 disabled:opacity-50">
          {saving ? "Saving..." : "Save"}
        </button>
      </div>
    </div>
  );
}

function AnalyticsScreen({ history, onCancel }: { history: { rx: number; tx: number }[]; onCancel: () => void }) {
  const { t } = useTranslation();
  
  const data = history.map((h, i) => ({
    time: i,
    download: h.rx,
    upload: h.tx,
  }));

  return (
    <div className="flex flex-col h-full bg-transparent text-fg-primary p-6 animate-fade-in">
      <div className="flex items-center justify-between mb-2">
        <h2 className="text-xs uppercase tracking-wide text-fg-faint font-medium">{t.analytics}</h2>
      </div>
      <p className="text-xs text-fg-muted mb-6">{t.analytics_hint}</p>

      <div className="flex-1 w-full min-h-[250px] bg-bg-subtle/50 backdrop-blur-md border border-border rounded-xl p-4">
        {data.length < 2 ? (
          <div className="h-full flex items-center justify-center text-fg-faint text-sm">
            No data yet. Connect to a profile first.
          </div>
        ) : (
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={data} margin={{ top: 10, right: 0, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="colorRx" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#10b981" stopOpacity={0.8}/>
                  <stop offset="95%" stopColor="#10b981" stopOpacity={0}/>
                </linearGradient>
                <linearGradient id="colorTx" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#f59e0b" stopOpacity={0.8}/>
                  <stop offset="95%" stopColor="#f59e0b" stopOpacity={0}/>
                </linearGradient>
              </defs>
              <XAxis dataKey="time" hide />
              <YAxis hide domain={['auto', 'auto']} />
              <Tooltip 
                contentStyle={{ backgroundColor: '#181818', border: '1px solid #27272a', borderRadius: '8px', fontSize: '12px' }}
                formatter={(val: any) => [`${fmtBytes(Number(val) || 0)}/s`, '']}
                labelFormatter={() => ''}
              />
              <Area type="monotone" dataKey="download" stroke="#10b981" fillOpacity={1} fill="url(#colorRx)" />
              <Area type="monotone" dataKey="upload" stroke="#f59e0b" fillOpacity={1} fill="url(#colorTx)" />
            </AreaChart>
          </ResponsiveContainer>
        )}
      </div>

      <div className="flex justify-end gap-2 mt-6">
        <button onClick={onCancel} className="btn-primary px-6 py-2 rounded-full text-xs font-medium">
          Close
        </button>
      </div>
    </div>
  );
}
