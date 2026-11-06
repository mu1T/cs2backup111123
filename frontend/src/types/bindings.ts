/**
 * Wails 绑定层的类型契约。
 *
 * 从 `utils/wails.ts` 里抽出来，是为了让 mock 数据也能引用同一份类型：
 * mock 只要写成 `BindingResult<'GetUsers'>`，后端签名一变就会在类型检查阶段报错，
 * 而不是等到浏览器预览时才发现假数据已经过期。
 */
import type {
  AppInfo,
  LogEntry,
  UserCardInfo,
  SnapshotInfo,
  FileDiffItem,
  DiffResult,
  VideoConfigData,
  VideoPreset,
  LANServerStatus,
  LANPackageInfo,
  DiscoveredDevice,
  RemoteBackupItem,
  DownloadedBackupResult,
  GameCFGFile,
  DisplayMode,
  Crosshair,
  CrosshairState,
  CrosshairSaveResult,
  CrosshairPreset,
  CrosshairCommandPreview,
} from './index'

export interface AppBindings {
  GetAppInfo: () => Promise<AppInfo>
  GetLogs: () => Promise<LogEntry[]>
  ClearLogs: () => Promise<void>
  EmitLog: (level: string, message: string) => Promise<LogEntry>
  GetSteamPath: () => Promise<string>
  SetSteamPath: (path: string) => Promise<void>
  DetectSteamPath: () => Promise<string>
  SelectSteamDirectory: () => Promise<string>
  GetUsers: () => Promise<UserCardInfo[]>
  GetRemoteBackups: () => Promise<UserCardInfo[]>
  SaveUserNote: (accountID: string, note: string) => Promise<void>
  ResolveUserName: (accountID: string) => Promise<string>
  GetBackupPath: () => Promise<string>
  SetBackupPath: (path: string) => Promise<void>
  SelectBackupDirectory: () => Promise<string>
  GetCS2CFGPath: () => Promise<string>
  GetCS2Path: () => Promise<string>
  SetCS2Path: (path: string) => Promise<void>
  SelectCS2Directory: () => Promise<string>
  BackupUser: (accountID: string) => Promise<void>
  RestoreUser: (targetAccountID: string, sourceAccountID: string) => Promise<void>
  BackupGameCFG: (mode: string, selectedFiles: string[]) => Promise<number>
  RestoreGameCFG: () => Promise<number>
  ListGameCFGFiles: () => Promise<GameCFGFile[]>
  ListUserCFGFiles: (accountID: string) => Promise<GameCFGFile[]>
  SelectCFGFiles: () => Promise<string[]>
  GetBackedUpAccounts: () => Promise<string[]>
  GetSnapshots: (accountID: string) => Promise<SnapshotInfo[]>
  CreateSnapshot: (accountID: string, note: string) => Promise<SnapshotInfo>
  RestoreSnapshot: (accountID: string, snapshotID: string) => Promise<void>
  DeleteSnapshot: (accountID: string, snapshotID: string) => Promise<void>
  CompareSnapshot: (accountID: string, snapshotID: string) => Promise<FileDiffItem[]>
  GetTextDiff: (textA: string, textB: string) => Promise<DiffResult>
  IsSteamRunning: () => Promise<boolean>
  ShutdownSteam: () => Promise<void>
  StartSteam: (launchGame?: boolean) => Promise<void>
  StartCS2Game: () => Promise<void>
  GetLaunchOptions: (accountID: string) => Promise<string>
  SetLaunchOptions: (accountID: string, options: string) => Promise<void>
  GetVideoSettings: (accountID: string) => Promise<VideoConfigData>
  GetDisplayModes: () => Promise<DisplayMode[]>
  SaveVideoSettings: (accountID: string, settings: Record<string, string>, readOnly: boolean) => Promise<void>
  SelectVideoConfigFile: () => Promise<string>
  ImportVideoPreset: (filePath: string, name: string, setAsDefault: boolean) => Promise<VideoPreset>
  SaveAsVideoPreset: (accountID: string, name: string, setAsDefault: boolean) => Promise<VideoPreset>
  GetVideoPresets: () => Promise<VideoPreset[]>
  SetDefaultVideoPreset: (presetID: string) => Promise<void>
  DeleteVideoPreset: (presetID: string) => Promise<void>
  ApplyVideoPreset: (accountID: string, presetID: string, setReadOnly: boolean) => Promise<void>
  ApplyDefaultVideoPreset: (accountID: string, setReadOnly: boolean) => Promise<void>
  StartLANServer: (port: number, name: string) => Promise<void>
  StopLANServer: () => Promise<void>
  GetLANServerStatus: () => Promise<LANServerStatus>
  ToggleLANPin: (enabled: boolean) => Promise<void>
  RegenerateLANPin: () => Promise<string>
  SetLANSharedBackups: (accountIDs: string[], zipPaths: string[]) => Promise<void>
  CreateLANPackage: (accountID: string, name: string, includeCfg: boolean, includeUserdata: boolean) => Promise<string>
  GetLANPackages: () => Promise<LANPackageInfo[]>
  StartDiscovery: () => Promise<void>
  StopDiscovery: () => Promise<void>
  GetDiscoveredDevices: () => Promise<DiscoveredDevice[]>
  AddManualDevice: (ip: string, port: number) => Promise<void>
  ConnectRemoteDevice: (ip: string, port: number, pin: string) => Promise<RemoteBackupItem[]>
  DownloadRemoteBackup: (ip: string, port: number, pin: string, backupID: string) => Promise<DownloadedBackupResult>
  SwitchSteamAccount: (accountName: string) => Promise<void>
  ReadConfigFile: (accountID: string, relPath: string) => Promise<string>
  SaveConfigFileText: (accountID: string, relPath: string, content: string) => Promise<void>
  CreateConfigFile: (accountID: string, name: string) => Promise<string>
  OpenPathInExplorer: (targetPath: string) => Promise<void>
  HasUserRollback: (accountID: string) => Promise<boolean>
  RollbackUserRestore: (accountID: string) => Promise<void>
  HasGameCFGRollback: () => Promise<boolean>
  RollbackGameCFG: () => Promise<number>
  GetCrosshairState: () => Promise<CrosshairState>
  SaveCrosshair: (crosshair: Crosshair) => Promise<CrosshairSaveResult>
  DecodeCrosshairCode: (accountID: string, code: string) => Promise<Crosshair>
  EncodeCrosshairCode: (crosshair: Crosshair) => Promise<string>
  ExportCrosshairCFG: (crosshair: Crosshair) => Promise<string>
  GetCrosshairConsole: (crosshair: Crosshair) => Promise<string>
  GetCrosshairPresets: () => Promise<CrosshairPreset[]>
  GetHiddenCrosshairPresets: () => Promise<CrosshairPreset[]>
  SaveCrosshairPreset: (name: string, crosshair: Crosshair) => Promise<CrosshairPreset>
  DeleteCrosshairPreset: (id: string) => Promise<void>
  SetCrosshairPresetHidden: (id: string, hidden: boolean) => Promise<void>
  GetCrosshairCommands: (crosshair: Crosshair) => Promise<CrosshairCommandPreview>
  SetLANRateLimit: (bytesPerSec: number) => Promise<void>
  SetLANBindAddress: (addr: string) => Promise<void>
  GetBackupRetention: () => Promise<number>
  SetBackupRetention: (n: number) => Promise<void>
  RestoreVersion: (accountID: string, versionID: string) => Promise<void>
  PreviewBackupPrune: (accountID: string) => Promise<string[]>
  ApplyBackupPrune: (accountID: string) => Promise<string[]>
  ListGameCFGVersions: () => Promise<string[]>
  RestoreGameCFGVersion: (versionID: string) => Promise<number>
}

/** 取某个绑定的返回值类型，例如 `BindingResult<'GetUsers'>` = `UserCardInfo[]`。 */
export type BindingResult<K extends keyof AppBindings> = Awaited<ReturnType<AppBindings[K]>>

declare global {
  interface Window {
    runtime?: {
      EventsOn: (eventName: string, callback: (...args: any[]) => void) => () => void
      EventsEmit: (eventName: string, ...args: any[]) => void
    }
    go?: {
      main?: {
        App?: AppBindings
      }
    }
  }
}
