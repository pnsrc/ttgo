import { useState, useEffect, Fragment } from "react";
import { motion } from "framer-motion";
import { useTranslation } from "../i18n";
import { GetGlobalSettings, SaveGlobalSettings, FindConflictAdapters, DisableAdapter, GetBuildInfo, CheckForUpdate } from "../api";
import logoUrl from "../assets/logo.png";
import { GlobalSettings } from "../types";

export function GlobalSettingsScreen(props: { onSave: (theme?: string) => void; onCancel: () => void }) {
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
      const newSettings = { ...settings, global_exclusions: exclusions };
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
              <option value="epic">Epic Style</option>
            </select>
          </label>

          <label className="flex items-center gap-3 cursor-pointer group">
            <input type="checkbox" checked={settings.auto_connect !== false} onChange={(e) => setSettings({ ...settings, auto_connect: e.target.checked })} className="w-4 h-4 rounded border-border bg-bg-elevated text-emerald-500 focus:ring-emerald-500 focus:ring-offset-0" />
            <span className="text-sm font-medium group-hover:text-fg-primary text-fg-secondary transition-colors">{t.auto_connect}</span>
          </label>

          <label className="flex items-center gap-3 cursor-pointer group">
            <input type="checkbox" checked={settings.enable_adblock} onChange={(e) => setSettings({ ...settings, enable_adblock: e.target.checked })} className="w-4 h-4 rounded border-border bg-bg-elevated text-emerald-500 focus:ring-emerald-500 focus:ring-offset-0" />
            <div className="flex flex-col">
              <span className="text-sm font-medium group-hover:text-fg-primary text-fg-secondary transition-colors">{t.adblock}</span>
              <span className="text-[11px] text-fg-faint">{t.adblock_desc}</span>
            </div>
          </label>

          <div className="space-y-1.5">
            <label className="flex items-center justify-between group">
              <div className="flex flex-col">
                <span className="text-sm font-medium group-hover:text-fg-primary text-fg-secondary transition-colors">{t.dns_server}</span>
                <span className="text-[11px] text-fg-faint">{t.dns_server_desc}</span>
              </div>
            </label>
            <input
              type="text"
              value={settings.upstream_dns || ""}
              onChange={(e) => setSettings({ ...settings, upstream_dns: e.target.value })}
              placeholder="1.1.1.1"
              className="w-full bg-bg-elevated border border-border rounded px-3 py-2 text-xs font-mono text-fg-secondary focus:outline-none focus:border-emerald-500"
            />
          </div>

          <hr className="border-border" />

          <label className="flex items-center gap-3 cursor-pointer group">
            <input type="checkbox" checked={settings.bypass_domains} onChange={(e) => setSettings({ ...settings, bypass_domains: e.target.checked })} className="w-4 h-4 rounded border-border bg-bg-elevated text-emerald-500 focus:ring-emerald-500 focus:ring-offset-0" />
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
            <p className="text-[11px] text-fg-faint leading-relaxed">{t.bypass_hint}</p>
          </div>
        </div>

        <ConflictAdaptersSection />
        <AboutSection />
      </div>

      {error && <div className="text-red-400 text-xs mt-3 font-medium">{error}</div>}
      <div className="flex-1" />
      <div className="flex justify-end gap-2 mt-4">
        <button onClick={props.onCancel} disabled={saving} className="btn-ghost px-4 py-2 rounded-md text-xs font-medium">{t.cancel}</button>
        <motion.button whileTap={{ scale: 0.95 }} onClick={handleSave} disabled={saving} className="btn-primary px-4 py-2 rounded-md text-xs font-medium bg-emerald-500 text-zinc-900 hover:bg-emerald-400 disabled:opacity-50">
          {saving ? t.saving : t.save}
        </motion.button>
      </div>
    </div>
  );
}

function ConflictAdaptersSection() {
  const { t } = useTranslation();
  const [adapters, setAdapters] = useState<any[] | null>(null);
  const [scanning, setScanning] = useState(false);
  const [disabled, setDisabled] = useState<Set<string>>(new Set());
  const isWindows = /win/i.test(navigator.userAgent) && !/darwin/i.test(navigator.userAgent);

  if (!isWindows) return null;

  async function scan() {
    setScanning(true);
    try {
      const list = await FindConflictAdapters();
      setAdapters(list || []);
    } catch {
      setAdapters([]);
    } finally {
      setScanning(false);
    }
  }

  async function handleDisable(name: string) {
    try {
      await DisableAdapter(name);
      setDisabled((prev) => new Set(prev).add(name));
    } catch {}
  }

  return (
    <div className="bg-bg-subtle border border-border rounded-lg p-4 space-y-3">
      <div className="flex flex-col">
        <span className="text-sm font-medium text-fg-secondary">{t.conflict_adapters}</span>
        <span className="text-[11px] text-fg-faint">{t.conflict_adapters_desc}</span>
      </div>
      <button onClick={scan} disabled={scanning} className="btn-ghost px-3 py-1.5 rounded-md text-xs font-medium">
        {scanning ? t.conflict_scanning : t.conflict_scan}
      </button>
      {adapters !== null && (
        adapters.length === 0 ? (
          <div className="text-xs text-fg-faint py-2">{t.conflict_none}</div>
        ) : (
          <div className="space-y-1.5">
            {adapters.map((a: any) => (
              <div key={a.name} className="flex items-center justify-between gap-2 bg-bg-elevated rounded-md px-3 py-2 border border-border">
                <div className="flex-1 min-w-0">
                  <div className="text-xs font-medium text-fg-secondary truncate">{a.name}</div>
                  <div className="text-[10px] text-fg-faint truncate">{a.description}</div>
                </div>
                <span className={`text-[10px] px-1.5 py-0.5 rounded ${a.status === "Up" ? "bg-emerald-500/20 text-emerald-400" : "bg-zinc-500/20 text-fg-faint"}`}>
                  {a.status}
                </span>
                {disabled.has(a.name) ? (
                  <span className="text-[10px] text-amber-400">{t.conflict_disabled}</span>
                ) : a.status === "Up" ? (
                  <button onClick={() => handleDisable(a.name)} className="text-[10px] px-2 py-1 rounded bg-red-500/10 text-red-400 hover:bg-red-500/20 transition-colors">
                    {t.conflict_disable}
                  </button>
                ) : null}
              </div>
            ))}
          </div>
        )
      )}
    </div>
  );
}

function AboutSection() {
  const { t } = useTranslation();
  const [build, setBuild] = useState<any>(null);
  const [update, setUpdate] = useState<{ available: boolean; version: string; url: string } | null>(null);
  const [checking, setChecking] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    GetBuildInfo().then(setBuild);
  }, []);

  async function checkUpdate() {
    setChecking(true);
    setError("");
    setUpdate(null);
    try {
      const info = await CheckForUpdate();
      setUpdate(info);
    } catch {
      setError(t.update_error);
    } finally {
      setChecking(false);
    }
  }

  const infoRows = build ? [
    { label: t.version_current, value: `v${build.version}` },
    { label: t.build_date, value: build.build_date === "dev" ? "Development" : new Date(build.build_date).toLocaleDateString() },
    { label: t.git_commit, value: build.git_commit },
    { label: t.git_branch, value: build.git_branch },
    { label: t.go_version, value: build.go_version },
    { label: t.platform, value: `${build.os}/${build.arch}` },
  ] : [];

  return (
    <div className="bg-bg-subtle border border-border rounded-lg p-4 space-y-4">
      <div className="flex items-center gap-3">
        <img src={logoUrl} alt="FireTunnel" className="w-12 h-12 rounded-xl shadow-lg" />
        <div className="flex flex-col">
          <span className="text-sm font-bold text-fg-primary">FireTunnel</span>
          <span className="text-[11px] text-fg-faint">Desktop client for TrustTunnel VPN</span>
        </div>
      </div>

      {build && (
        <div className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5">
          {infoRows.map(row => (
            <Fragment key={row.label}>
              <span className="text-[11px] text-fg-faint">{row.label}</span>
              <span className="text-[11px] font-mono text-fg-secondary truncate">{row.value}</span>
            </Fragment>
          ))}
        </div>
      )}

      <div className="flex items-center gap-2">
        <button onClick={checkUpdate} disabled={checking} className="btn-ghost px-3 py-1.5 rounded-md text-xs font-medium">
          {checking ? t.checking_update : t.check_update}
        </button>
      </div>

      {update && (
        update.available ? (
          <div className="flex items-center justify-between bg-emerald-500/10 border border-emerald-500/20 rounded-md px-3 py-2">
            <span className="text-xs font-medium text-emerald-400">{t.update_available}: v{update.version}</span>
            <button onClick={() => window.open(update.url)} className="text-xs px-3 py-1 rounded bg-emerald-500 text-zinc-900 hover:bg-emerald-400 font-medium">
              {t.update_download}
            </button>
          </div>
        ) : (
          <div className="text-xs text-fg-faint py-1">{t.update_latest}</div>
        )
      )}
      {error && <div className="text-xs text-red-400">{error}</div>}
    </div>
  );
}
