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

const w = (window as any).go.main.App;

export const EnrollDevice = (url: string) => w.EnrollDevice(url);
export const GetPendingDeepLink = () => w.GetPendingDeepLink() as Promise<string>;
export const ReadClipboard = () => w.ReadClipboard() as Promise<string>;
export const ReadLogs = (lines: number) => w.ReadLogs(lines) as Promise<string>;
export const ImportProfileFromText = (content: string, name: string) => w.ImportProfileFromText(content, name) as Promise<any>;
export const GetConnections = () => w.GetConnections() as Promise<any[]>;
export const AddExclusion = (domain: string) => w.AddExclusion(domain) as Promise<void>;
export const CheckWintun = () => w.CheckWintun() as Promise<boolean>;
export const DownloadWintun = () => w.DownloadWintun() as Promise<void>;
export const FindConflictAdapters = () => w.FindConflictAdapters() as Promise<any[]>;
export const DisableAdapter = (name: string) => w.DisableAdapter(name) as Promise<void>;
export const GetEnrollments = () => w.GetEnrollments() as Promise<any[]>;
export const GetVersion = () => w.GetVersion() as Promise<string>;
export const CheckForUpdate = () => w.CheckForUpdate() as Promise<{ available: boolean; version: string; url: string; release_notes: string }>;
export const GetBuildInfo = () => w.GetBuildInfo() as Promise<{ version: string; build_date: string; git_commit: string; git_branch: string; go_version: string; os: string; arch: string }>;

export {
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
};
