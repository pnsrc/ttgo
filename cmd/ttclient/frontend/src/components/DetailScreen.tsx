import { useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { useTranslation } from "../i18n";
import { ReadProfileContent } from "../api";
import { Profile, Snapshot } from "../types";
import { Power, Trash, ShareIcon } from "../icons";
import { fmtBytes, fmtDuration } from "../utils";
import { QRCodeSVG } from "qrcode.react";

export function DetailScreen(props: {
  profile: Profile;
  snap: Snapshot;
  isActive: boolean;
  speedHistory: { rx: number; tx: number }[];
  onConnect: () => void;
  onDisconnect: () => void;
  onDelete: () => void;
  onEdit: () => void;
  enrolled?: boolean;
}) {
  const { profile, snap, isActive, speedHistory } = props;
  const isOn = isActive && snap.state === "connected";
  const isPending = isActive && snap.state === "connecting";
  const { t } = useTranslation();
  const [showQR, setShowQR] = useState(false);
  const [qrData, setQrData] = useState("");

  async function handleShare() {
    if (showQR) { setShowQR(false); return; }
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
            ${isOn
              ? "bg-emerald-500/15 border-2 border-emerald-400 shadow-[0_0_60px_-10px_rgba(52,211,153,0.5)]"
              : isPending
              ? "bg-amber-500/15 border-2 border-amber-400/60"
              : "bg-bg-elevated border-2 border-zinc-700 hover:border-zinc-500"
            }`}
        >
          {isOn && <span className="absolute inset-0 rounded-full border-2 border-emerald-400/40 animate-ping" />}
          <Power className={`w-14 h-14 transition-colors ${isOn ? "text-emerald-400" : isPending ? "text-amber-400 animate-pulse" : "text-fg-faint group-hover:text-fg-secondary"}`} />
        </motion.button>

        <div className="mt-6 text-center">
          <motion.div
            key={isOn ? "on" : isPending ? "pending" : snap.state === "error" ? "err" : "off"}
            initial={{ opacity: 0, y: 5 }}
            animate={{ opacity: 1, y: 0 }}
            className={`text-lg font-medium ${isOn ? "text-emerald-400" : isPending ? "text-amber-400" : snap.state === "error" && isActive ? "text-red-400" : "text-fg-secondary"}`}
          >
            {isOn ? t.protected : isPending ? t.connecting : snap.state === "error" && isActive ? t.error : t.tap_to_connect}
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

      <div className="relative z-10 mt-7 w-full max-w-xs">
        <div className="text-[11px] uppercase tracking-wide text-fg-faint font-medium px-1 mb-2">{t.profile}</div>
        <div className="space-y-2">
          <DetailRow label={t.address} value={profile.endpoint} mono />
          {profile.toml?.endpoint?.hostname && <DetailRow label={t.hostname} value={profile.toml.endpoint.hostname} mono />}
          {profile.toml?.endpoint?.upstream_protocol && <DetailRow label={t.protocol} value={profile.toml.endpoint.upstream_protocol} />}
          {profile.toml?.endpoint?.skip_verification && <DetailRow label={t.tls_verify} value={t.skipped} warn />}
          {profile.toml?.exclusions && profile.toml.exclusions.length > 0 && <DetailRow label={t.exclusions} value={`${profile.toml.exclusions.length} ${t.rules}`} />}
        </div>

        {!isActive && (
          <div className="flex gap-2 mt-5">
            {!props.enrolled && (
              <button onClick={handleShare} className="flex-[0.5] inline-flex items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-xs text-fg-secondary hover:text-fg-primary hover:bg-zinc-700/50 transition-colors" title={t.share}>
                <ShareIcon className="w-3.5 h-3.5" />
              </button>
            )}
            {!props.enrolled && (
              <button onClick={props.onEdit} className="flex-1 inline-flex items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-xs text-fg-secondary hover:text-fg-primary hover:bg-zinc-700/50 transition-colors">
                {t.edit_config}
              </button>
            )}
            <button onClick={props.onDelete} className="flex-1 inline-flex items-center justify-center gap-1.5 rounded-md px-3 py-1.5 text-xs text-red-400 hover:text-red-300 hover:bg-red-500/10 transition-colors">
              <Trash className="w-3.5 h-3.5" /> {t.delete}
            </button>
          </div>
        )}
      </div>

      <AnimatePresence>
        {showQR && (
          <motion.div initial={{ opacity: 0, y: 20 }} animate={{ opacity: 1, y: 0 }} exit={{ opacity: 0, y: 20 }} className="absolute inset-0 z-50 flex flex-col items-center justify-center bg-bg/90 backdrop-blur-sm p-6">
            <div className="bg-white p-4 rounded-xl shadow-2xl mb-4">
              <QRCodeSVG value={qrData} size={200} level="M" />
            </div>
            <p className="text-fg-secondary text-sm font-medium">{t.qr_hint}</p>
            <button onClick={() => setShowQR(false)} className="mt-6 btn-ghost px-6 py-2 rounded-full">{t.cancel}</button>
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
        <span className={`text-sm tabular-nums ${props.color ?? "text-fg-primary"} ${props.mono ? "font-mono text-xs" : ""}`}>
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
      <span className={`text-xs truncate ${props.warn ? "text-amber-400" : "text-fg-secondary"} ${props.mono ? "font-mono" : ""}`}>
        {props.value}
      </span>
    </div>
  );
}
