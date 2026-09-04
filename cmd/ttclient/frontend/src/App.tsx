import { useEffect, useState, useCallback, useRef } from "react";
import { useTranslation } from "./i18n";
import { motion, AnimatePresence } from "framer-motion";
import { EventsOn } from "../wailsjs/runtime/runtime";
import { StatusPayload, Snapshot, Profile } from "./types";
import { Logo, Alert, Activity } from "./icons";
import {
  ConnectProfile, Disconnect, Status, Profiles, ImportProfile,
  DeleteProfile, OpenProfilesDir, SaveProfileContent,
  GetGlobalSettings, PingAll, EnrollDevice, GetPendingDeepLink,
  ImportProfileFromText, CheckWintun, DownloadWintun, GetEnrollments,
} from "./api";

import { Titlebar } from "./components/Titlebar";
import { ProfilesScreen } from "./components/ProfilesScreen";
import { DetailScreen } from "./components/DetailScreen";
import { ConnectionsScreen } from "./components/ConnectionsScreen";
import { GlobalSettingsScreen } from "./components/SettingsScreen";
import { LogsScreen } from "./components/LogsScreen";
import { EditScreen } from "./components/EditScreen";
import { PasteImportScreen } from "./components/PasteImportScreen";
import { AnalyticsScreen } from "./components/AnalyticsScreen";

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
  const [showLogs, setShowLogs] = useState(false);
  const [showConns, setShowConns] = useState(false);
  const [showPasteImport, setShowPasteImport] = useState(false);
  const [showEnroll, setShowEnroll] = useState(false);
  const [enrollURL, setEnrollURL] = useState("");
  const [enrolling, setEnrolling] = useState(false);
  const [pingData, setPingData] = useState<Record<string, number>>({});
  const [isWide, setIsWide] = useState(window.innerWidth >= 768);
  const { t } = useTranslation();
  const [theme, setTheme] = useState("glass");
  const [wintunMissing, setWintunMissing] = useState(false);
  const [wintunDownloading, setWintunDownloading] = useState(false);

  const [speedHistory, setSpeedHistory] = useState<{ rx: number; tx: number }[]>([]);
  const lastBytes = useRef({ rx: 0, tx: 0, time: 0 });

  useEffect(() => {
    CheckWintun().then((ok) => { if (!ok) setWintunMissing(true); }).catch(() => {});
  }, []);

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
      setSpeedHistory((prev) => [...prev, { rx: rxSpeed, tx: txSpeed }].slice(-60));
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
    if (isMac) document.body.classList.add("platform-macos");
    return () => { document.body.classList.remove("platform-macos"); };
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

    GetPendingDeepLink().then((link) => {
      if (link) {
        setEnrollURL(link);
        handleEnroll(link);
      }
    }).catch(() => {});

    const unsubDeeplink = EventsOn("deeplink", (url: string) => {
      if (url) handleEnroll(url);
    });

    return () => { unsub(); unsubDeeplink(); };
  }, [refreshProfiles]);

  useEffect(() => {
    if (activeID) setSelectedID(activeID);
  }, [activeID]);

  useEffect(() => {
    if (snap.state === "error" && snap.error) showToast(snap.error, "error");
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
      if (p) { await refreshProfiles(); setSelectedID(p.id); }
    } catch (e: any) {
      showToast(String(e?.message ?? e), "error");
    }
  }

  async function handleEnroll(url?: string) {
    const link = url || enrollURL.trim();
    if (!link) return;
    setEnrolling(true);
    try {
      const result = await EnrollDevice(link) as any;
      if (result?.ok) {
        showToast(t.enroll_success, "info");
        await refreshProfiles();
        if (result.profile_id) setSelectedID(result.profile_id);
        setShowEnroll(false);
        setEnrollURL("");
      } else if (result?.revoked) {
        showToast(result.message || t.enroll_revoked, "error");
        await refreshProfiles();
      } else {
        showToast(result?.message || result?.error || "Enrollment failed", "error");
      }
    } catch (e: any) {
      showToast(String(e?.message ?? e), "error");
    } finally {
      setEnrolling(false);
    }
  }

  async function handlePasteImport(content: string, name: string) {
    try {
      const p = await ImportProfileFromText(content, name);
      if (p) {
        await refreshProfiles();
        setSelectedID(p.id);
        setShowPasteImport(false);
        showToast(t.settings_saved, "info");
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
    try { await ConnectProfile(id); } catch (e: any) { showToast(String(e?.message ?? e), "error"); }
  }

  async function disconnect() {
    try { await Disconnect(); } catch (e: any) { showToast(String(e?.message ?? e), "error"); }
  }

  async function handleSaveContent(content: string) {
    if (!selectedID) return;
    await SaveProfileContent(selectedID, content);
    await refreshProfiles();
    setIsEditing(false);
  }

  const enrollStates = useRef<Record<string, boolean>>({});
  useEffect(() => {
    GetEnrollments().then((states) => {
      const map: Record<string, boolean> = {};
      (states || []).forEach((s: any) => { if (s.profile_id) map[s.profile_id] = true; });
      enrollStates.current = map;
    }).catch(() => {});
  }, [profiles]);

  const isEnrolled = (id: string) => !!enrollStates.current[id];

  const effectiveSelectedID = selectedID || (isWide && profiles.length > 0 ? (activeID || profiles[0].id) : null);
  const selected = profiles.find((p) => p.id === effectiveSelectedID) ?? null;

  return (
    <div data-theme={theme} className="h-screen flex flex-col bg-bg text-fg-primary relative overflow-hidden">
      <Titlebar
        showBack={(!!selectedID) || isEditing || showSettings || showAnalytics || showLogs || showConns || showPasteImport}
        title={isEditing ? `${t.edit_config} ${selected?.name}` : showSettings ? t.settings : showAnalytics ? t.analytics : showLogs ? t.logs : showConns ? t.active_conns : showPasteImport ? t.paste_config : (selected?.name ?? "FireTunnel")}
        onBack={() => { isEditing ? setIsEditing(false) : showSettings ? setShowSettings(false) : showAnalytics ? setShowAnalytics(false) : showLogs ? setShowLogs(false) : showConns ? setShowConns(false) : showPasteImport ? setShowPasteImport(false) : setSelectedID(null) }}
        onSettings={() => { setShowSettings(true); setShowAnalytics(false); setShowLogs(false); setShowConns(false); setShowPasteImport(false); }}
        onAnalytics={() => { setShowAnalytics(true); setShowSettings(false); setShowLogs(false); setShowConns(false); setShowPasteImport(false); }}
        onLogs={() => { setShowLogs(true); setShowSettings(false); setShowAnalytics(false); setShowConns(false); setShowPasteImport(false); }}
        onConns={() => { setShowConns(true); setShowLogs(false); setShowSettings(false); setShowAnalytics(false); setShowPasteImport(false); }}
        hideSettings={isEditing || showSettings || showAnalytics || showLogs || showConns || showPasteImport}
      />

      <AnimatePresence>
        {wintunMissing && (
          <motion.div initial={{ opacity: 0, height: 0 }} animate={{ opacity: 1, height: "auto" }} exit={{ opacity: 0, height: 0 }} className="overflow-hidden">
            <div className="bg-amber-500/10 border-b border-amber-500/30 px-4 py-3 flex items-center gap-3">
              <Alert className="w-5 h-5 text-amber-400 shrink-0" />
              <div className="flex-1 min-w-0">
                <div className="text-sm font-medium text-amber-300">{t.wintun_missing}</div>
                <div className="text-xs text-amber-400/70 mt-0.5">{t.wintun_desc}</div>
              </div>
              <button
                onClick={async () => {
                  setWintunDownloading(true);
                  try {
                    await DownloadWintun();
                    setWintunMissing(false);
                    showToast(t.wintun_installed, "info");
                  } catch (e: any) {
                    showToast(String(e?.message ?? e), "error");
                  } finally {
                    setWintunDownloading(false);
                  }
                }}
                disabled={wintunDownloading}
                className="shrink-0 px-3 py-1.5 rounded-md text-xs font-medium bg-amber-500 text-zinc-900 hover:bg-amber-400 disabled:opacity-50"
              >
                {wintunDownloading ? t.wintun_downloading : t.wintun_download}
              </button>
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      <div className="flex-1 flex overflow-hidden">
        <div className={`w-full md:w-80 flex-shrink-0 md:block ${
          (showSettings || showLogs || showConns || showPasteImport || selectedID || isEditing) ? "hidden" : "block"
        } overflow-y-auto bg-bg-subtle`}>
          <ProfilesScreen
            profiles={profiles}
            activeID={activeID}
            onSelect={(id) => { setSelectedID(id); setIsEditing(false); setShowSettings(false); }}
            onImport={importProfile}
            onRefresh={refreshProfiles}
            onOpenDir={() => OpenProfilesDir()}
            onPingAll={handlePingAll}
            pingData={pingData}
            showEnroll={showEnroll}
            enrollURL={enrollURL}
            enrolling={enrolling}
            onToggleEnroll={() => setShowEnroll(!showEnroll)}
            onEnrollURLChange={setEnrollURL}
            onEnroll={() => handleEnroll()}
            onPasteImport={() => { setShowPasteImport(true); setSelectedID(null); }}
            onDeleteProfile={removeProfile}
            connecting={snap.state === "connecting"}
          />
        </div>

        <div className={`flex-1 md:flex flex-col ${
          (showSettings || showAnalytics || showLogs || showConns || showPasteImport || selectedID || isEditing) ? "flex" : "hidden"
        } overflow-y-auto bg-transparent`}>
          <AnimatePresence mode="wait">
            {showSettings ? (
              <motion.div key="settings" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} className="h-full">
                <GlobalSettingsScreen onSave={(t) => { setShowSettings(false); if (t) setTheme(t); showToast("Settings saved", "info"); }} onCancel={() => setShowSettings(false)} />
              </motion.div>
            ) : showLogs ? (
              <motion.div key="logs" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} className="h-full">
                <LogsScreen onClose={() => setShowLogs(false)} />
              </motion.div>
            ) : showConns ? (
              <motion.div key="conns" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} className="h-full">
                <ConnectionsScreen onClose={() => setShowConns(false)} />
              </motion.div>
            ) : showPasteImport ? (
              <motion.div key="paste" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} className="h-full">
                <PasteImportScreen onImport={handlePasteImport} onCancel={() => setShowPasteImport(false)} />
              </motion.div>
            ) : showAnalytics ? (
              <motion.div key="analytics" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} className="h-full">
                <AnalyticsScreen history={speedHistory} onCancel={() => setShowAnalytics(false)} />
              </motion.div>
            ) : selected ? (
              isEditing ? (
                <motion.div key="edit" initial={{ opacity: 0, y: 10 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: -10 }} className="h-full">
                  <EditScreen profile={selected} onSave={handleSaveContent} onCancel={() => setIsEditing(false)} />
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
                    enrolled={isEnrolled(selected.id)}
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
          <button onClick={() => setToast(null)} className="text-fg-muted hover:text-zinc-200 text-xs px-1">✕</button>
        </div>
      )}
    </div>
  );
}
