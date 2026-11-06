import type { AppInfo, LogEntry, LogLevel, UserCardInfo, SnapshotInfo, FileDiffItem, DiffResult, VideoConfigData, VideoPreset, LANServerStatus, LANPackageInfo, DiscoveredDevice, RemoteBackupItem, DownloadedBackupResult, GameCFGFile, DisplayMode, Crosshair, CrosshairState, CrosshairSaveResult, CrosshairPreset, CrosshairConvar, CrosshairCommandPreview } from '../types'
import { mockUsers, createMockGameCFGFiles, createMockUserCFGFiles, createMockSnapshots } from '../mocks/data'
// 生成的绑定层：每个函数就是 App.X 的薄封装。
// 用它而不是直接写 App.X，是为了让调用签名由 Go 侧决定——
// 后端改了参数或方法名，这里会编译失败，而不是运行时静默返回 undefined。
import * as App from '../../wailsjs/go/main/App'
import type { crosshair as WireCrosshair } from '../../wailsjs/go/models'

/**
 * 生成绑定与前端类型之间有两处固有摩擦，都在这一层一次性断言掉：
 *
 * 1. 生成的模型是 **class**（带 `convertValues()` 辅助方法），前端传的是普通
 *    对象字面量——参数位置传不进去；
 * 2. 生成的结构体字段是 `string`，前端有意收窄成字面量联合（如 `LogLevel`）
 *    ——返回位置对不上。
 *
 * 这两处都是编译期断言，运行时是恒等函数，不改变任何行为。
 * 之所以保留前端的窄类型：`LogLevel` / `FileDiffItem.status` 这些联合类型让
 * 分支收窄和穷尽检查成为可能，放宽成 `string` 会把保障换掉。
 */
const wire = <T>(v: unknown): T => v as T

/** 把前端的 Crosshair（普通对象）喂给 Go 侧参数（生成的 class）。 */
const crosshairArg = (ch: Crosshair) => wire<WireCrosshair.Crosshair>(ch)

/**
 * Checks whether the application is running inside the Wails desktop environment.
 */
export const isWailsAvailable = (): boolean => {
  return typeof window !== 'undefined' && typeof window.runtime !== 'undefined' && typeof window.go?.main?.App !== 'undefined'
}

export const isWailsRuntime = isWailsAvailable

/**
 * Retrieves application metadata from the Go backend, or falls back to defaults.
 */
export const getAppInfo = async (): Promise<AppInfo> => {
  if (window.go?.main?.App?.GetAppInfo) {
    try {
      return await App.GetAppInfo()
    } catch (e) {
      console.warn('Failed to fetch AppInfo from Wails backend:', e)
    }
  }
  return {
    name: 'CS2 Config Backup & Sync',
    version: '2.0.0 (Web/Dev)',
    description: 'CS2 个人配置备份与局域网同步工具',
    os: 'browser',
    arch: 'x64',
  }
}

/**
 * Fetches recent logs buffered in memory from the Go backend.
 */
export const getLogs = async (): Promise<LogEntry[]> => fetchLogs()
export const clearLogs = async (): Promise<void> => clearSystemLogs()
export const fetchLogs = async (): Promise<LogEntry[]> => {
  if (window.go?.main?.App?.GetLogs) {
    try {
      return wire<LogEntry[]>(await App.GetLogs())
    } catch (e) {
      console.warn('Failed to fetch logs from Wails backend:', e)
    }
  }
  return [
    {
      id: 1,
      timestamp: new Date().toTimeString().split(' ')[0],
      level: 'info',
      message: '系统前端界面初始化完成',
    },
  ]
}

/**
 * Clears the system log buffer in the Go backend.
 */
export const clearSystemLogs = async (): Promise<void> => {
  if (window.go?.main?.App?.ClearLogs) {
    try {
      await App.ClearLogs()
    } catch (e) {
      console.warn('Failed to clear logs via Wails backend:', e)
    }
  }
}

/**
 * Emits a log entry to the backend log buffer and triggers real-time broadcast.
 */
export const emitSystemLog = async (level: LogLevel, message: string): Promise<LogEntry | null> => {
  if (window.go?.main?.App?.EmitLog) {
    try {
      return wire<LogEntry>(await App.EmitLog(level, message))
    } catch (e) {
      console.warn('Failed to emit log to Wails backend:', e)
    }
  }
  return null
}

/**
 * Subscribes to real-time "log" events pushed by the Go backend.
 */
export const subscribeToLogs = (callback: (entry: LogEntry) => void): (() => void) => {
  if (window.runtime?.EventsOn) {
    return window.runtime.EventsOn('log', (entry: LogEntry) => {
      callback(entry)
    })
  }
  return () => {}
}

/**
 * Subscribes to real-time events emitted from Go.
 */
export const onWailsEvent = (eventName: string, callback: (...args: any[]) => void): (() => void) => {
  if (window.runtime?.EventsOn) {
    return window.runtime.EventsOn(eventName, callback)
  }
  return () => {}
}

/**
 * 订阅「某个账号的昵称 / 头像已在后台更新」事件。
 *
 * 后台抓取是异步的，不订阅的话用户得重开面板才看得到新名字。
 */
export const subscribeToUserUpdates = (callback: () => void): (() => void) => {
  if (window.runtime?.EventsOn) {
    return window.runtime.EventsOn('steam:users-updated', () => callback())
  }
  return () => {}
}

/**
 * Retrieves the currently saved Steam directory path.
 */
export const getSteamPath = async (): Promise<string> => {
  if (window.go?.main?.App?.GetSteamPath) {
    try {
      return await App.GetSteamPath()
    } catch (e) {
      console.warn('Failed to get Steam path:', e)
    }
  }
  return localStorage.getItem('cs2_mock_steam_path') || 'C:\\Steam'
}

/**
 * Updates and saves the Steam directory path.
 */
export const setSteamPath = async (path: string): Promise<void> => {
  if (window.go?.main?.App?.SetSteamPath) {
    return await App.SetSteamPath(path)
  }
  localStorage.setItem('cs2_mock_steam_path', path)
}

/**
 * Triggers backend auto-detection for the Steam path.
 */
export const detectSteamPath = async (): Promise<string> => {
  if (window.go?.main?.App?.DetectSteamPath) {
    return await App.DetectSteamPath()
  }
  return 'C:\\Program Files (x86)\\Steam'
}

/**
 * Prompts user for Steam directory via native dialog.
 */
export const selectSteamDirectory = async (): Promise<string> => {
  if (window.go?.main?.App?.SelectSteamDirectory) {
    return await App.SelectSteamDirectory()
  }
  return prompt('请输入本地 Steam 安装目录路径:', 'C:\\Steam') || ''
}

/**
 * Fetches all local Steam users discovered in loginusers.vdf / userdata.
 */
export const getUsers = async (): Promise<UserCardInfo[]> => {
  if (window.go?.main?.App?.GetUsers) {
    try {
      return await App.GetUsers()
    } catch (e) {
      console.warn('Failed to fetch Steam users from Wails backend:', e)
    }
  }

  return mockUsers
}

/**
 * Labels for downloaded remote backup archives (not Steam accounts).
 */
export const getRemoteBackups = async (): Promise<UserCardInfo[]> => {
  const api = window.go?.main?.App
  if (api?.GetRemoteBackups) {
    try {
      return await api.GetRemoteBackups()
    } catch {
      return []
    }
  }
  return []
}

/**
 * Persists a user's custom note.
 */
export const saveUserNote = async (accountID: string, note: string): Promise<void> => {
  if (window.go?.main?.App?.SaveUserNote) {
    return await App.SaveUserNote(accountID, note)
  }
  const notes = JSON.parse(localStorage.getItem('cs2_mock_notes') || '{}')
  notes[accountID] = note
  localStorage.setItem('cs2_mock_notes', JSON.stringify(notes))
}

/**
 * Resolves a username for an account ID.
 */
export const resolveUserName = async (accountID: string): Promise<string> => {
  if (window.go?.main?.App?.ResolveUserName) {
    return await App.ResolveUserName(accountID)
  }
  return accountID
}

/**
 * Retrieves the currently saved backup repository path.
 */
export const getBackupPath = async (): Promise<string> => {
  if (window.go?.main?.App?.GetBackupPath) {
    try {
      return await App.GetBackupPath()
    } catch (e) {
      console.warn('Failed to get backup path:', e)
    }
  }
  return localStorage.getItem('cs2_mock_backup_path') || 'backup'
}

/**
 * Updates and saves the backup repository path.
 */
export const setBackupPath = async (path: string): Promise<void> => {
  if (window.go?.main?.App?.SetBackupPath) {
    return await App.SetBackupPath(path)
  }
  localStorage.setItem('cs2_mock_backup_path', path)
}

/**
 * Prompts user for backup directory via native dialog.
 */
export const selectBackupDirectory = async (): Promise<string> => {
  if (window.go?.main?.App?.SelectBackupDirectory) {
    return await App.SelectBackupDirectory()
  }
  return prompt('请输入备份存放目录路径:', 'D:\\CS2_Backups') || ''
}

/**
 * Opens a directory or file in Windows Explorer.
 */
export const openPathInExplorer = async (path: string): Promise<void> => {
  if (window.go?.main?.App?.OpenPathInExplorer) {
    return await App.OpenPathInExplorer(path)
  }
  console.log(`[Mock] Opened path in Explorer: ${path}`)
}

/**
 * Returns detected CS2 game CFG path.
 */
export const getCS2CFGPath = async (): Promise<string> => {
  if (window.go?.main?.App?.GetCS2CFGPath) {
    return await App.GetCS2CFGPath()
  }
  return 'C:\\Steam\\steamapps\\common\\Counter-Strike Global Offensive\\game\\csgo\\cfg'
}

/**
 * Backs up userdata for the given account ID.
 */
/**
 * 手动指定的 CS2 配置目录（game/csgo/cfg）。空串表示交给自动探测。
 */
export const getCS2Path = async (): Promise<string> => {
  if (window.go?.main?.App?.GetCS2Path) {
    return await App.GetCS2Path()
  }
  return localStorage.getItem('cs2_mock_cs2_path') || ''
}

/**
 * 设置 CS2 配置目录。后端会把任意层级归一成 game/csgo/cfg；传空串恢复自动探测。
 */
export const setCS2Path = async (path: string): Promise<void> => {
  if (window.go?.main?.App?.SetCS2Path) {
    await App.SetCS2Path(path)
    return
  }
  if (path.trim()) localStorage.setItem('cs2_mock_cs2_path', path.trim())
  else localStorage.removeItem('cs2_mock_cs2_path')
}

/**
 * 打开目录选择器挑选 CS2 安装目录，返回归一化后的 cfg 目录。
 */
export const selectCS2Directory = async (): Promise<string> => {
  if (window.go?.main?.App?.SelectCS2Directory) {
    return await App.SelectCS2Directory()
  }
  return ''
}

export const backupUser = async (accountID: string): Promise<void> => {
  if (window.go?.main?.App?.BackupUser) {
    return await App.BackupUser(accountID)
  }
  const backed = JSON.parse(localStorage.getItem('cs2_mock_backed_up_users') || '[]')
  if (!backed.includes(accountID)) {
    backed.push(accountID)
    localStorage.setItem('cs2_mock_backed_up_users', JSON.stringify(backed))
  }
}

/**
 * Restores userdata for the given target account ID.
 */
export const restoreUser = async (targetAccountID: string, sourceAccountID?: string): Promise<void> => {
  if (window.go?.main?.App?.RestoreUser) {
    return await App.RestoreUser(targetAccountID, sourceAccountID || targetAccountID)
  }
}

/**
 * Backs up CS2 game config files.
 */
export const backupGameCFG = async (mode: string, selectedFiles: string[] = []): Promise<number> => {
  if (window.go?.main?.App?.BackupGameCFG) {
    return await App.BackupGameCFG(mode, selectedFiles)
  }
  return mode === 'custom' ? 2 : (mode === 'selected' ? selectedFiles.length : 5)
}

/**
 * Restores CS2 game config files.
 */
/** 通用 CFG 的历史版本，最新的在前。 */
export const listGameCFGVersions = async (): Promise<string[]> => {
  if (window.go?.main?.App?.ListGameCFGVersions) {
    return await App.ListGameCFGVersions()
  }
  return []
}

/**
 * 把通用 CFG 还原到指定历史版本。
 *
 * 与 restoreGameCFG 的区别：那个只认「最新备份」，这个能回到任意一版。
 */
export const restoreGameCFGVersion = async (versionID: string): Promise<number> => {
  if (window.go?.main?.App?.RestoreGameCFGVersion) {
    return await App.RestoreGameCFGVersion(versionID)
  }
  return 0
}

export const restoreGameCFG = async (): Promise<number> => {
  if (window.go?.main?.App?.RestoreGameCFG) {
    return await App.RestoreGameCFG()
  }
  return 5
}

/**
 * Opens file picker dialog for CFG files.
 */
export const selectCFGFiles = async (): Promise<string[]> => {
  if (window.go?.main?.App?.SelectCFGFiles) {
    return await App.SelectCFGFiles()
  }
  return ['C:\\Steam\\steamapps\\common\\Counter-Strike Global Offensive\\game\\csgo\\cfg\\autoexec.cfg']
}

/**
 * Scans game/csgo/cfg and returns every backup-able file with its origin classification.
 */
export const getGameCFGFiles = async (): Promise<GameCFGFile[]> => {
  const api = window.go?.main?.App
  if (api?.ListGameCFGFiles) {
    return await api.ListGameCFGFiles()
  }
  // 桌面端存在后端却没有该接口：说明客户端版本过旧，直接报错而不是回退到假数据
  if (api) {
    throw new Error('当前客户端不支持 CFG 目录扫描，请重新构建后运行')
  }
  // 浏览器 / 开发态：样例 + 本次会话里新建出来的文件
  return mergeMockCreated(createMockGameCFGFiles(), 'game_cfg')
}

/**
 * Scans userdata/<accountID>/730/local/cfg for the config editor.
 */
export const getUserCFGFiles = async (accountID: string): Promise<GameCFGFile[]> => {
  const api = window.go?.main?.App
  if (api?.ListUserCFGFiles) {
    return await api.ListUserCFGFiles(accountID)
  }
  if (api) {
    throw new Error('当前客户端不支持账号配置目录扫描，请重新构建后运行')
  }
  return mergeMockCreated(createMockUserCFGFiles(), accountID)
}

/**
 * Retrieves the list of accounts with existing backups.
 */
export const getBackedUpAccounts = async (): Promise<string[]> => {
  if (window.go?.main?.App?.GetBackedUpAccounts) {
    try {
      return await App.GetBackedUpAccounts()
    } catch (e) {
      console.warn('Failed to get backed up accounts:', e)
    }
  }
  return JSON.parse(localStorage.getItem('cs2_mock_backed_up_users') || '["123456789"]')
}

/**
 * Snapshot & Diff API
 */

export const getSnapshots = async (accountID: string): Promise<SnapshotInfo[]> => {
  if (window.go?.main?.App?.GetSnapshots) {
    try {
      return await App.GetSnapshots(accountID)
    } catch (e) {
      console.warn('Failed to get snapshots:', e)
      throw e
    }
  }
  const key = `cs2_mock_snapshots_${accountID}`
  const cached = localStorage.getItem(key)
  if (cached) {
    return JSON.parse(cached)
  }
  return createMockSnapshots(accountID)
}

export const createSnapshot = async (accountID: string, note: string): Promise<SnapshotInfo> => {
  if (window.go?.main?.App?.CreateSnapshot) {
    return await App.CreateSnapshot(accountID, note)
  }
  const key = `cs2_mock_snapshots_${accountID}`
  const existing: SnapshotInfo[] = JSON.parse(localStorage.getItem(key) || '[]')
  const newSnap: SnapshotInfo = {
    id: new Date().toISOString().replace(/[-:T]/g, '').slice(0, 15),
    account_id: accountID,
    created_at: new Date().toISOString(),
    note: note || '用户手动快照',
    total_files: 3,
    total_bytes: 8420,
    file_hashes: {},
  }
  existing.unshift(newSnap)
  localStorage.setItem(key, JSON.stringify(existing))
  return newSnap
}

export const restoreSnapshot = async (accountID: string, snapshotID: string): Promise<void> => {
  if (window.go?.main?.App?.RestoreSnapshot) {
    return await App.RestoreSnapshot(accountID, snapshotID)
  }
}

export const deleteSnapshot = async (accountID: string, snapshotID: string): Promise<void> => {
  if (window.go?.main?.App?.DeleteSnapshot) {
    return await App.DeleteSnapshot(accountID, snapshotID)
  }
  const key = `cs2_mock_snapshots_${accountID}`
  const existing: SnapshotInfo[] = JSON.parse(localStorage.getItem(key) || '[]')
  const filtered = existing.filter((s) => s.id !== snapshotID)
  localStorage.setItem(key, JSON.stringify(filtered))
}

export const compareSnapshot = async (accountID: string, snapshotID: string): Promise<FileDiffItem[]> => {
  if (window.go?.main?.App?.CompareSnapshot) {
    return wire<FileDiffItem[]>(await App.CompareSnapshot(accountID, snapshotID))
  }
  return [
    {
      rel_path: 'local/cfg/cs2_video.txt',
      status: 'modified',
      diff: {
        lines: [
          { type: 'same', old_line_num: 1, new_line_num: 1, content: '"VideoConfig"' },
          { type: 'same', old_line_num: 2, new_line_num: 2, content: '{' },
          { type: 'mod', old_line_num: 3, new_line_num: 3, content: '\t"setting.csm_quality_level"\t\t"3"', old_content: '\t"setting.csm_quality_level"\t\t"1"' },
          { type: 'same', old_line_num: 4, new_line_num: 4, content: '\t"setting.defaultres"\t\t"1920"' },
          { type: 'same', old_line_num: 5, new_line_num: 5, content: '\t"setting.defaultresheight"\t\t"1080"' },
          { type: 'add', old_line_num: 0, new_line_num: 6, content: '\t"setting.high_dpi"\t\t"1"' },
          { type: 'same', old_line_num: 6, new_line_num: 7, content: '}' },
        ],
        add_count: 1,
        del_count: 0,
        mod_count: 1,
      },
    },
    {
      rel_path: 'local/cfg/cs2_user_keys_0_slot0.vcfg',
      status: 'identical',
      diff: {
        lines: [
          { type: 'same', old_line_num: 1, new_line_num: 1, content: '"config"' },
          { type: 'same', old_line_num: 2, new_line_num: 2, content: '{' },
          { type: 'same', old_line_num: 3, new_line_num: 3, content: '\t"bindings"' },
          { type: 'same', old_line_num: 4, new_line_num: 4, content: '\t{' },
          { type: 'same', old_line_num: 5, new_line_num: 5, content: '\t\t"w"\t\t"+forward"' },
          { type: 'same', old_line_num: 6, new_line_num: 6, content: '\t}' },
          { type: 'same', old_line_num: 7, new_line_num: 7, content: '}' },
        ],
        add_count: 0,
        del_count: 0,
        mod_count: 0,
      },
    },
  ]
}

export const getTextDiff = async (textA: string, textB: string): Promise<DiffResult> => {
  if (window.go?.main?.App?.GetTextDiff) {
    return wire<DiffResult>(await App.GetTextDiff(textA, textB))
  }
  return {
    lines: [],
    add_count: 0,
    del_count: 0,
    mod_count: 0,
  }
}

let mockSteamRunning = false;
const mockLaunchOptions: Record<string, string> = {
  '123456789': '-novid -tickrate 128 +fps_max 0',
};

export const isSteamRunning = async (): Promise<boolean> => {
  if (window.go?.main?.App?.IsSteamRunning) {
    return await App.IsSteamRunning();
  }
  return mockSteamRunning;
};

export const shutdownSteam = async (): Promise<void> => {
  if (window.go?.main?.App?.ShutdownSteam) {
    await App.ShutdownSteam();
    return;
  }
  mockSteamRunning = false;
};

export const startSteam = async (launchGame: boolean = false): Promise<void> => {
  if (window.go?.main?.App?.StartSteam) {
    await App.StartSteam(!!launchGame);
    return;
  }
  mockSteamRunning = true;
};

export const startCS2Game = async (): Promise<void> => {
  if (window.go?.main?.App?.StartCS2Game) {
    await App.StartCS2Game();
    return;
  }
  return startSteam(true);
};

export const switchSteamAccount = async (accountName: string): Promise<void> => {
  if (window.go?.main?.App?.SwitchSteamAccount) {
    await App.SwitchSteamAccount(accountName);
    return;
  }
  mockSteamRunning = true;
};

export const getLaunchOptions = async (accountID: string): Promise<string> => {
  if (window.go?.main?.App?.GetLaunchOptions) {
    return await App.GetLaunchOptions(accountID);
  }
  return mockLaunchOptions[accountID] || '';
};

export const setLaunchOptions = async (accountID: string, options: string): Promise<void> => {
  if (window.go?.main?.App?.SetLaunchOptions) {
    await App.SetLaunchOptions(accountID, options);
    return;
  }
  mockLaunchOptions[accountID] = options;
};

// Video Settings and Presets mock & implementations

const mockVideoConfig: Record<string, VideoConfigData> = {
  '123456789': {
    file_path: 'C:\\Program Files (x86)\\Steam\\userdata\\123456789\\730\\local\\cfg\\cs2_video.txt',
    read_only: false,
    settings: {
      Version: '16',
      VendorID: '4318',
      DeviceID: '9373',
      'setting.defaultres': '1280',
      'setting.defaultresheight': '960',
      'setting.aspectratiomode': '0',
      'setting.fullscreen': '1',
      'setting.nowindowborder': '0',
      'setting.fullscreen_min_on_focus_loss': '1',
      'setting.monitor_index': '0',
      'setting.high_dpi': '0',
      'setting.refreshrate_numerator': '240',
      'setting.refreshrate_denominator': '1',
      'setting.cpu_level': '0',
      'setting.gpu_level': '0',
      'setting.gpu_mem_level': '0',
      'setting.msaa_samples': '4',
      'setting.r_csgo_cmaa_enable': '0',
      'setting.videocfg_fsr_detail': '0',
      'setting.r_texturefilteringquality': '1',
      'setting.r_low_latency': '1',
      'setting.mat_vsync': '0',
      'setting.videocfg_ao_detail': '0',
      'setting.videocfg_dynamic_shadows': '1',
      'setting.videocfg_shadow_quality': '1',
      'setting.videocfg_particle_detail': '0',
      'setting.videocfg_texture_detail': '0',
      'setting.shaderquality': '0',
      'setting.videocfg_hdr_detail': '3',
    },
  },
}

let mockPresets: VideoPreset[] = [
  {
    id: 'preset_default_competitive',
    name: '默认画质预设 (1280×960)',
    created_at: '2026-11-04 18:00:00',
    is_default: true,
    file_path: 'config/video_presets/preset_default_competitive.txt',
    settings: { ...mockVideoConfig['123456789'].settings },
  },
]

export const getVideoSettings = async (accountID: string): Promise<VideoConfigData> => {
  if (window.go?.main?.App?.GetVideoSettings) {
    return await App.GetVideoSettings(accountID)
  }
  if (!mockVideoConfig[accountID]) {
    mockVideoConfig[accountID] = {
      file_path: `C:\\Steam\\userdata\\${accountID}\\730\\local\\cfg\\cs2_video.txt`,
      read_only: false,
      settings: { ...mockVideoConfig['123456789'].settings },
    }
  }
  return JSON.parse(JSON.stringify(mockVideoConfig[accountID]))
}

export const saveVideoSettings = async (
  accountID: string,
  settings: Record<string, string>,
  readOnly: boolean
): Promise<void> => {
  if (window.go?.main?.App?.SaveVideoSettings) {
    await App.SaveVideoSettings(accountID, settings, readOnly)
    return
  }
  if (!mockVideoConfig[accountID]) {
    mockVideoConfig[accountID] = {
      file_path: `C:\\Steam\\userdata\\${accountID}\\730\\local\\cfg\\cs2_video.txt`,
      read_only: readOnly,
      settings: { ...settings },
    }
  } else {
    mockVideoConfig[accountID].settings = { ...mockVideoConfig[accountID].settings, ...settings }
    mockVideoConfig[accountID].read_only = readOnly
  }
}

/**
 * Enumerates display modes supported by the primary monitor (Windows).
 */
export const getDisplayModes = async (): Promise<DisplayMode[]> => {
  const api = window.go?.main?.App
  if (api?.GetDisplayModes) {
    try {
      return await api.GetDisplayModes()
    } catch {
      return []
    }
  }
  // 浏览器 / 开发态样例
  return [
    { width: 1920, height: 1080, refresh_rate: 240, bits_per_pel: 32 },
    { width: 1920, height: 1080, refresh_rate: 144, bits_per_pel: 32 },
    { width: 1920, height: 1080, refresh_rate: 60, bits_per_pel: 32 },
    { width: 1280, height: 960, refresh_rate: 240, bits_per_pel: 32 },
    { width: 1280, height: 960, refresh_rate: 144, bits_per_pel: 32 },
    { width: 1024, height: 768, refresh_rate: 144, bits_per_pel: 32 },
  ]
}

export const selectVideoConfigFile = async (): Promise<string> => {
  if (window.go?.main?.App?.SelectVideoConfigFile) {
    return await App.SelectVideoConfigFile()
  }
  return 'C:\\Downloads\\pro_video.txt'
}

export const importVideoPreset = async (
  filePath: string,
  name: string,
  setAsDefault: boolean
): Promise<VideoPreset> => {
  if (window.go?.main?.App?.ImportVideoPreset) {
    return await App.ImportVideoPreset(filePath, name, setAsDefault)
  }
  const id = `preset_${Date.now()}`
  if (setAsDefault) {
    mockPresets.forEach((p) => (p.is_default = false))
  }
  const newPreset: VideoPreset = {
    id,
    name: name || '导入预设',
    created_at: new Date().toLocaleString(),
    is_default: setAsDefault,
    file_path: `config/video_presets/${id}.txt`,
    settings: { ...mockVideoConfig['123456789'].settings },
  }
  mockPresets.push(newPreset)
  return newPreset
}

export const saveAsVideoPreset = async (
  accountID: string,
  name: string,
  setAsDefault: boolean
): Promise<VideoPreset> => {
  if (window.go?.main?.App?.SaveAsVideoPreset) {
    return await App.SaveAsVideoPreset(accountID, name, setAsDefault)
  }
  const id = `preset_${Date.now()}`
  if (setAsDefault) {
    mockPresets.forEach((p) => (p.is_default = false))
  }
  const cfg = await getVideoSettings(accountID)
  const newPreset: VideoPreset = {
    id,
    name: name || `账号 ${accountID} 预设`,
    created_at: new Date().toLocaleString(),
    is_default: setAsDefault,
    file_path: `config/video_presets/${id}.txt`,
    settings: { ...cfg.settings },
  }
  mockPresets.push(newPreset)
  return newPreset
}

export const getVideoPresets = async (): Promise<VideoPreset[]> => {
  if (window.go?.main?.App?.GetVideoPresets) {
    return await App.GetVideoPresets()
  }
  return JSON.parse(JSON.stringify(mockPresets))
}

export const setDefaultVideoPreset = async (presetID: string): Promise<void> => {
  if (window.go?.main?.App?.SetDefaultVideoPreset) {
    await App.SetDefaultVideoPreset(presetID)
    return
  }
  mockPresets.forEach((p) => {
    p.is_default = p.id === presetID
  })
}

export const deleteVideoPreset = async (presetID: string): Promise<void> => {
  if (window.go?.main?.App?.DeleteVideoPreset) {
    await App.DeleteVideoPreset(presetID)
    return
  }
  mockPresets = mockPresets.filter((p) => p.id !== presetID)
}

export const applyVideoPreset = async (
  accountID: string,
  presetID: string,
  setReadOnly: boolean
): Promise<void> => {
  if (window.go?.main?.App?.ApplyVideoPreset) {
    await App.ApplyVideoPreset(accountID, presetID, setReadOnly)
    return
  }
  const p = mockPresets.find((x) => x.id === presetID)
  if (!p) throw new Error(`预设不存在: ${presetID}`)
  await saveVideoSettings(accountID, p.settings, setReadOnly)
}

export const applyDefaultVideoPreset = async (
  accountID: string,
  setReadOnly: boolean
): Promise<void> => {
  if (window.go?.main?.App?.ApplyDefaultVideoPreset) {
    await App.ApplyDefaultVideoPreset(accountID, setReadOnly)
    return
  }
  const p = mockPresets.find((x) => x.is_default) || mockPresets[0]
  if (!p) throw new Error('未设置默认预设')
  await saveVideoSettings(accountID, p.settings, setReadOnly)
}

// LAN Sharing bindings and mocks

let mockLANStatus: LANServerStatus = {
  running: false,
  url: 'http://192.168.1.100:8765',
  all_urls: ['http://192.168.1.100:8765'],
  port: 8765,
  share_name: 'CS2-DESKTOP-LAN',
  pin: '839201',
  pin_enabled: false,
  shared_count: 1,
  connections: 0,
  shared_local_ids: ['123456789'],
  shared_zip_paths: [],
  rate_limit: 0,
  bind_address: '',
  available_addresses: [],
}

let mockLANPackages: LANPackageInfo[] = [
  {
    filename: 'CS2Backup_ZywOo_20261104.zip',
    path: 'backup/lan_packages/CS2Backup_ZywOo_20261104.zip',
    size: 4520384,
  },
]

export const startLANServer = async (port: number, name: string): Promise<void> => {
  if (window.go?.main?.App?.StartLANServer) {
    await App.StartLANServer(port, name)
    return
  }
  mockLANStatus.running = true
  mockLANStatus.port = port || 8765
  mockLANStatus.share_name = name || 'CS2-DEV-LAN'
  mockLANStatus.url = `http://192.168.1.100:${mockLANStatus.port}`
  mockLANStatus.all_urls = [mockLANStatus.url]
}

/**
 * 设置局域网共享的出站限速（bytes/sec）。传 0 表示不限。
 *
 * 限速挂在服务端写出层，所以对桌面端和浏览器端同时生效。
 */
/**
 * 指定局域网共享只在哪张网卡的地址上提供服务。传空串表示全部网卡。
 *
 * 重启共享服务后才生效——监听地址是在启动时绑定的。
 */
export const setLANBindAddress = async (addr: string): Promise<void> => {
  if (window.go?.main?.App?.SetLANBindAddress) {
    await App.SetLANBindAddress(addr)
    return
  }
  mockLANStatus.bind_address = addr
}

export const setLANRateLimit = async (bytesPerSec: number): Promise<void> => {
  if (window.go?.main?.App?.SetLANRateLimit) {
    await App.SetLANRateLimit(bytesPerSec)
    return
  }
  mockLANStatus.rate_limit = bytesPerSec
}

/** 每个账号保留多少个历史版本；0 表示不清理。 */
/**
 * 把账号还原到指定历史版本。
 *
 * 与 restoreUser 的区别：那个只认「最新备份」镜像，这个能回到任意一个版本，
 * 并且还原前会自动留一个「还原前自动留底」的版本。
 */
export const restoreVersion = async (accountID: string, versionID: string): Promise<void> => {
  if (window.go?.main?.App?.RestoreVersion) {
    await App.RestoreVersion(accountID, versionID)
  }
}

/** 收敛时会删掉哪些历史版本——只查不删，给确认框用。 */
export const previewBackupPrune = async (accountID: string): Promise<string[]> => {
  if (window.go?.main?.App?.PreviewBackupPrune) {
    return await App.PreviewBackupPrune(accountID)
  }
  return []
}

/** 按当前上限收敛历史版本，返回被删的 ID。 */
export const applyBackupPrune = async (accountID: string): Promise<string[]> => {
  if (window.go?.main?.App?.ApplyBackupPrune) {
    return await App.ApplyBackupPrune(accountID)
  }
  return []
}

export const getBackupRetention = async (): Promise<number> => {
  if (window.go?.main?.App?.GetBackupRetention) {
    return await App.GetBackupRetention()
  }
  return 10
}

/**
 * 设置历史版本保留上限，0 表示不清理。
 *
 * 只影响之后的备份：改小不会立刻删掉现有历史，等下一次备份时才收敛。
 */
export const setBackupRetention = async (n: number): Promise<void> => {
  if (window.go?.main?.App?.SetBackupRetention) {
    await App.SetBackupRetention(n)
  }
}

export const stopLANServer = async (): Promise<void> => {
  if (window.go?.main?.App?.StopLANServer) {
    await App.StopLANServer()
    return
  }
  mockLANStatus.running = false
}

export const getLANServerStatus = async (): Promise<LANServerStatus> => {
  if (window.go?.main?.App?.GetLANServerStatus) {
    return await App.GetLANServerStatus()
  }
  return JSON.parse(JSON.stringify(mockLANStatus))
}

export const toggleLANPin = async (enabled: boolean): Promise<void> => {
  if (window.go?.main?.App?.ToggleLANPin) {
    await App.ToggleLANPin(enabled)
    return
  }
  mockLANStatus.pin_enabled = enabled
}

export const regenerateLANPin = async (): Promise<string> => {
  if (window.go?.main?.App?.RegenerateLANPin) {
    return await App.RegenerateLANPin()
  }
  const randomPIN = Math.floor(100000 + Math.random() * 900000).toString()
  mockLANStatus.pin = randomPIN
  return randomPIN
}

export const setLANSharedBackups = async (accountIDs: string[], zipPaths: string[]): Promise<void> => {
  if (window.go?.main?.App?.SetLANSharedBackups) {
    await App.SetLANSharedBackups(accountIDs, zipPaths)
    return
  }
  mockLANStatus.shared_local_ids = [...accountIDs]
  mockLANStatus.shared_zip_paths = [...zipPaths]
  mockLANStatus.shared_count = accountIDs.length + zipPaths.length
}

export const createLANPackage = async (
  accountID: string,
  name: string,
  includeCfg: boolean,
  includeUserdata: boolean
): Promise<string> => {
  if (window.go?.main?.App?.CreateLANPackage) {
    return await App.CreateLANPackage(accountID, name, includeCfg, includeUserdata)
  }
  const filename = `CS2Backup_${name || accountID}_${Date.now()}.zip`
  const pkgPath = `backup/lan_packages/${filename}`
  mockLANPackages.unshift({
    filename,
    path: pkgPath,
    size: 3200000,
  })
  if (!mockLANStatus.shared_zip_paths.includes(pkgPath)) {
    mockLANStatus.shared_zip_paths.push(pkgPath)
    mockLANStatus.shared_count++
  }
  return pkgPath
}

export const getLANPackages = async (): Promise<LANPackageInfo[]> => {
  if (window.go?.main?.App?.GetLANPackages) {
    return await App.GetLANPackages()
  }
  return JSON.parse(JSON.stringify(mockLANPackages))
}


/**
 * UDP Discovery & Remote Backup Sync API
 */

const mockDiscoveredDevices: DiscoveredDevice[] = [
  {
    ip: '192.168.1.108',
    port: 8765,
    name: 'ZywOo-Gaming-Laptop',
    version: '2.0.0',
    last_seen: new Date().toISOString(),
    is_manual: false,
  },
  {
    ip: '192.168.1.115',
    port: 8765,
    name: 's1mple-Desktop-PC',
    version: '2.0.0',
    last_seen: new Date().toISOString(),
    is_manual: false,
  },
]

const mockRemoteBackups: RemoteBackupItem[] = [
  {
    id: 'mock_remote_1',
    filename: 'CS2Backup_ZywOo_20261104.zip',
    size: 2850000,
    backup_name: '职业竞技配置 (ZywOo)',
    created_at: new Date().toISOString(),
    contains_cfg: true,
    contains_userdata: true,
    is_local: false,
    account_id: '123456789',
  },
  {
    id: 'mock_remote_2',
    filename: 'CS2Backup_Local_666000333.zip',
    size: 1420000,
    backup_name: '日常排位个人配置',
    created_at: new Date().toISOString(),
    contains_cfg: true,
    contains_userdata: false,
    is_local: true,
    account_id: '666000333',
  },
]

export const startDiscovery = async (): Promise<void> => {
  if (window.go?.main?.App?.StartDiscovery) {
    return await App.StartDiscovery()
  }
}

export const stopDiscovery = async (): Promise<void> => {
  if (window.go?.main?.App?.StopDiscovery) {
    return await App.StopDiscovery()
  }
}

export const getDiscoveredDevices = async (): Promise<DiscoveredDevice[]> => {
  if (window.go?.main?.App?.GetDiscoveredDevices) {
    return await App.GetDiscoveredDevices()
  }
  return JSON.parse(JSON.stringify(mockDiscoveredDevices))
}

export const addManualDevice = async (ip: string, port: number): Promise<void> => {
  if (window.go?.main?.App?.AddManualDevice) {
    return await App.AddManualDevice(ip, port)
  }
  mockDiscoveredDevices.unshift({
    ip,
    port,
    name: `手动添加设备 (${ip}:${port})`,
    version: 'manual',
    last_seen: new Date().toISOString(),
    is_manual: true,
  })
}

export const connectRemoteDevice = async (ip: string, port: number, pin: string): Promise<RemoteBackupItem[]> => {
  if (window.go?.main?.App?.ConnectRemoteDevice) {
    return await App.ConnectRemoteDevice(ip, port, pin)
  }
  if (pin && pin !== '123456' && pin !== '666888') {
    throw new Error('PIN 码错误，无法连接到该设备')
  }
  return JSON.parse(JSON.stringify(mockRemoteBackups))
}

export const downloadRemoteBackup = async (
  ip: string,
  port: number,
  pin: string,
  backupID: string
): Promise<DownloadedBackupResult> => {
  if (window.go?.main?.App?.DownloadRemoteBackup) {
    return await App.DownloadRemoteBackup(ip, port, pin, backupID)
  }
  const timestamp = new Date().toISOString().replace(/[-:T.]/g, '').slice(0, 15)
  return {
    account_id: `remote_${timestamp}`,
    dest_dir: `backup/userdata_backup/remote_${timestamp}`,
    total_files: 8,
    total_bytes: 2850000,
    source_name: `Remote-${ip}`,
    download_time: new Date().toLocaleString(),
  }
}

// In-App Config Text Editor bindings and mocks
const mockConfigFiles: Record<string, string> = {
  '123456789:cs2_user_keys.vcfg': `"config"\n{\n\t"bindings"\n\t{\n\t\t"w"\t\t"+forward"\n\t\t"s"\t\t"+back"\n\t\t"a"\t\t"+left"\n\t\t"d"\t\t"+right"\n\t\t"space"\t\t"+jump"\n\t\t"mwheeldown"\t\t"+jump"\n\t\t"mouse1"\t\t"+attack"\n\t\t"mouse2"\t\t"+attack2"\n\t\t"e"\t\t"+use"\n\t\t"r"\t\t"+reload"\n\t\t"shift"\t\t"+sprint"\n\t\t"ctrl"\t\t"+duck"\n\t}\n}`,
  '123456789:cs2_video.txt': `"video.cfg"\n{\n\t"setting.defaultres"\t\t"1280"\n\t"setting.defaultresheight"\t\t"960"\n\t"setting.refreshrate_numerator"\t\t"240"\n\t"setting.refreshrate_denominator"\t\t"1"\n\t"setting.aspectratiomode"\t\t"0"\n\t"setting.fullscreen"\t\t"1"\n\t"setting.nowindowborder"\t\t"0"\n}`,
  '123456789:cs2_machine_convars.vcfg': `"config"\n{\n\t"convars"\n\t{\n\t\t"cl_crosshaircolor"\t\t"1"\n\t\t"cl_crosshairsize"\t\t"2.5"\n\t\t"cl_crosshairgap"\t\t"-2"\n\t\t"sensitivity"\t\t"1.25"\n\t\t"volume"\t\t"0.6"\n\t\t"fps_max"\t\t"0"\n\t}\n}`,
  '123456789:autoexec.cfg': `// CS2 个人配置 autoexec.cfg (SteamID: 123456789)\n// 每次游戏启动自动载入\nfps_max 0\nbind mwheeldown +jump\nbind mwheelup +jump\nsensitivity 1.25\nvolume 0.65\necho "User autoexec.cfg loaded successfully!"\n`,
  'game_cfg:autoexec.cfg': `// CS2 游戏通用配置 game/csgo/cfg/autoexec.cfg\n// 对本机所有启动 CS2 的玩家全局生效\nrate 1000000\ncl_updaterate 128\ncl_interp 0.015625\ncl_crosshair_friendly_warning 0\necho "CS2 Global Game CFG loaded!"\n`,
  'game_cfg:gamemode_competitive.cfg': `// CS2 竞技模式默认参数重载\nmp_roundtime 1.92\nmp_freezetime 15\nmp_maxrounds 24\nmp_overtime_enable 1\nmp_c4timer 40\n`,
  'game_cfg:gamemode_casual.cfg': `// CS2 休闲模式默认参数\nmp_roundtime 2.25\nmp_freezetime 5\nmp_maxrounds 12\nmp_c4timer 45\n`,
}

/**
 * 把「本次会话里新建的文件」并进 mock 列表。
 *
 * 不这么做的话，浏览器里新建完文件，下拉列表里没有它——看起来像功能坏了，
 * 而真实后端是扫目录的，新建的文件一定在。
 */
function mergeMockCreated(list: GameCFGFile[], accountID: string): GameCFGFile[] {
  const prefix = `${accountID}:`
  const known = new Set(list.map((f) => f.name.toLowerCase()))
  const extra: GameCFGFile[] = []

  const addName = (name: string, content: string) => {
    if (known.has(name.toLowerCase())) return
    known.add(name.toLowerCase())
    extra.push({
      name,
      size: content.length,
      modified: new Date().toISOString(),
      ext: '.cfg',
      source: 'player',
      read_only: false,
    })
  }

  for (const key of Object.keys(mockConfigFiles)) {
    if (key.startsWith(prefix)) addName(key.slice(prefix.length), mockConfigFiles[key])
  }
  for (let i = 0; i < localStorage.length; i++) {
    const key = localStorage.key(i) || ''
    if (key.startsWith(`cs2_mock_cfg_${prefix}`)) {
      const name = key.slice(`cs2_mock_cfg_${prefix}`.length)
      addName(name, localStorage.getItem(key) || '')
    }
  }

  return extra.length ? [...list, ...extra] : list
}

function cleanMockRelPath(relPath: string): string {
  return relPath.replace(/\\/g, '/').replace(/^(730\/)?local\/cfg\//, '').replace(/^(game\/)?csgo\/cfg\//, '')
}

export const readConfigFile = async (accountId: string, relPath: string): Promise<string> => {
  const cleanRel = cleanMockRelPath(relPath)
  if (window.go?.main?.App?.ReadConfigFile) {
    return await App.ReadConfigFile(accountId, cleanRel)
  }
  const key = `${accountId}:${cleanRel}`
  const cached = localStorage.getItem(`cs2_mock_cfg_${key}`)
  if (cached !== null) {
    return cached
  }
  if (mockConfigFiles[key] !== undefined) {
    return mockConfigFiles[key]
  }
  for (const k of Object.keys(mockConfigFiles)) {
    if (k.endsWith(`:${cleanRel}`)) {
      return mockConfigFiles[k]
    }
  }
  return `// ${cleanRel} (${accountId})\n// [DEV MODE] 自动生成模拟配置文本\n`
}

/**
 * 新建一个空的 cfg 文件，返回最终文件名（后端会补 .cfg 后缀）。
 *
 * 与保存的区别：这个**只能新建**，同名已存在会抛错。别拿 saveConfigFileText
 * 来实现「新建」——那是覆盖写，用户输个 autoexec 就把原配置清空了。
 */
export const createConfigFile = async (accountId: string, name: string): Promise<string> => {
  const cleanName = name.trim()
  if (window.go?.main?.App?.CreateConfigFile) {
    return await App.CreateConfigFile(accountId, cleanName)
  }

  // 浏览器里跑（开发/探针）：按后端的规则做一份最简模拟，好让界面能验
  const finalName = cleanName.toLowerCase().endsWith('.cfg') ? cleanName : `${cleanName}.cfg`
  const key = `${accountId}:${finalName}`
  const taken = Object.keys(mockConfigFiles).some((k) => k.toLowerCase() === key.toLowerCase())
  if (taken || localStorage.getItem(`cs2_mock_cfg_${key}`) !== null) {
    throw new Error(`${finalName} 已存在，换个名字`)
  }
  mockConfigFiles[key] = ''
  localStorage.setItem(`cs2_mock_cfg_${key}`, '')
  return finalName
}

export const saveConfigFileText = async (accountId: string, relPath: string, content: string): Promise<void> => {
  const cleanRel = cleanMockRelPath(relPath)
  if (window.go?.main?.App?.SaveConfigFileText) {
    await App.SaveConfigFileText(accountId, cleanRel, content)
    return
  }
  const key = `${accountId}:${cleanRel}`
  mockConfigFiles[key] = content
  localStorage.setItem(`cs2_mock_cfg_${key}`, content)
}


/**
 * Rollback & Restore Reversion APIs
 */
export const hasUserRollback = async (accountId: string): Promise<boolean> => {
  if (window.go?.main?.App?.HasUserRollback) {
    try {
      return await App.HasUserRollback(accountId);
    } catch {
      return false;
    }
  }
  return false;
};

export const rollbackUserRestore = async (accountId: string): Promise<void> => {
  if (window.go?.main?.App?.RollbackUserRestore) {
    await App.RollbackUserRestore(accountId);
    return;
  }
};

export const hasGameCFGRollback = async (): Promise<boolean> => {
  if (window.go?.main?.App?.HasGameCFGRollback) {
    try {
      return await App.HasGameCFGRollback();
    } catch {
      return false;
    }
  }
  return false;
};

export const rollbackGameCFG = async (): Promise<number> => {
  if (window.go?.main?.App?.RollbackGameCFG) {
    return await App.RollbackGameCFG();
  }
  return 0;
};

/**
 * 准星设置 APIs
 *
 * 浏览器预览（无后端）下用 localStorage 顶一份内存状态，行为与后端一致：
 * 保存后再次读取能拿到刚写的值，分享码走本地实现的同款编解码。
 */

const CROSSHAIR_MOCK_KEY = 'cs2_mock_crosshair'

/** 与 Go 侧 crosshair.Default() 保持一致的默认准星。 */
export const defaultCrosshair = (): Crosshair => ({
  format: 'cs2-v1',
  style: 7,
  follow_recoil: true,
  dot: false,
  t_style: false,
  outline_mode: 1,
  color: { r: 0, g: 255, b: 0, a: 255 },
  outline_color: { r: 0, g: 0, b: 0, a: 255 },
  gap: 4,
  length: 8,
  thickness: 2,
  spread_limit: 255,
  split_dist: 3,
  inner_alpha: 0,
  outer_alpha: 1,
  split_ratio: 1,
  screen_height: 1080,
  scope_dot_scale: 1,
  scope_dot_use_color: false,
})

const readMockCrosshair = (): Crosshair => {
  try {
    const raw = localStorage.getItem(CROSSHAIR_MOCK_KEY)
    if (!raw) return defaultCrosshair()
    return { ...defaultCrosshair(), ...(JSON.parse(raw) as Partial<Crosshair>) }
  } catch {
    return defaultCrosshair()
  }
}

export const getCrosshairState = async (): Promise<CrosshairState> => {
  if (window.go?.main?.App?.GetCrosshairState) {
    return wire<CrosshairState>(await App.GetCrosshairState())
  }
  const crosshair = readMockCrosshair()
  const allKeys = Object.keys(crosshair).filter((k) => k !== 'format')
  return {
    path: 'C://Steam//steamapps//common//Counter-Strike Global Offensive\\game\\csgo\\cfg\\autoexec.cfg',
    exists: localStorage.getItem(CROSSHAIR_MOCK_KEY) !== null,
    parsed: {
      crosshair,
      found_keys: [],
      missing_keys: allKeys,
      display_toggle: '1',
      display_toggle_present: true,
    },
    share_code: await encodeCrosshairCode(crosshair),
  }
}

export const saveCrosshair = async (crosshair: Crosshair): Promise<CrosshairSaveResult> => {
  if (window.go?.main?.App?.SaveCrosshair) {
    return wire<CrosshairSaveResult>(await App.SaveCrosshair(crosshairArg(crosshair)))
  }
  localStorage.setItem(CROSSHAIR_MOCK_KEY, JSON.stringify(crosshair))
  return {
    path: 'C://Steam//steamapps//common//Counter-Strike Global Offensive\\game\\csgo\\cfg\\autoexec.cfg',
    report: {
      replaced_keys: [],
      appended_keys: [],
      commented_keys: [],
      display_toggle: '1',
      preserved_toggle: true,
    },
    backup_path: '',
    share_code: await encodeCrosshairCode(crosshair),
    created: false,
  }
}

/**
 * 解析分享码。accountID 用来取该账号的 CS2 分辨率——旧版码的尺寸是比例值，
 * 换算成像素要知道「当前高度」，传空则退回 1080。
 */
export const decodeCrosshairCode = async (accountID: string, code: string): Promise<Crosshair> => {
  if (window.go?.main?.App?.DecodeCrosshairCode) {
    return wire<Crosshair>(await App.DecodeCrosshairCode(accountID, code))
  }
  throw new Error('浏览器预览模式不支持解析分享码，请在桌面端使用')
}

export const encodeCrosshairCode = async (crosshair: Crosshair): Promise<string> => {
  if (window.go?.main?.App?.EncodeCrosshairCode) {
    return await App.EncodeCrosshairCode(crosshairArg(crosshair))
  }
  return 'CS（浏览器预览模式不生成分享码）'
}

export const exportCrosshairCFG = async (crosshair: Crosshair): Promise<string> => {
  if (window.go?.main?.App?.ExportCrosshairCFG) {
    return await App.ExportCrosshairCFG(crosshairArg(crosshair))
  }
  localStorage.setItem('cs2_mock_crosshair_cfg', JSON.stringify(crosshair))
  return 'C://Steam//steamapps//common//Counter-Strike Global Offensive\\game\\csgo\\cfg\\crosshair.cfg'
}

export const getCrosshairConsole = async (crosshair: Crosshair): Promise<string> => {
  if (window.go?.main?.App?.GetCrosshairConsole) {
    return await App.GetCrosshairConsole(crosshairArg(crosshair))
  }
  return ''
}

const CROSSHAIR_USER_KEY = 'cs2_mock_crosshair_user_presets'
const CROSSHAIR_HIDDEN_KEY = 'cs2_mock_crosshair_hidden_presets'

interface MockUserPreset {
  id: string
  name: string
  created_at: string
  crosshair: Crosshair
}

/** 浏览器预览用的职业选手预设，取值与 Go 侧 BuiltinPresets() 一致。 */
const MOCK_PRO_PRESETS: CrosshairPreset[] = [
  ['pro-zywoo', 'ZywOo', 2, 2, 2, 768, 0, 233, 255, 200],
  ['pro-donk', 'donk', 2, 2, 1, 960, 37, 255, 0, 255],
  ['pro-monesy', 'm0NESY', 3, 1, 2, 960, 255, 255, 255, 255],
  ['pro-kscerato', 'kscerato', 1, 2, 1, 864, 0, 255, 226, 255],
  ['pro-sh1ro', 'sh1ro', 2, 1, 1, 960, 0, 255, 0, 200],
  ['pro-ropz', 'ropz', 4, 1, 2, 1080, 0, 255, 0, 255],
  ['pro-molodoy', 'molodoy', 2, 2, 2, 768, 16, 255, 255, 200],
  ['pro-spinx', 'spinx', 2, 2, 1, 960, 0, 255, 0, 255],
  ['pro-makazze', 'makazze', 2, 2, 1, 960, 0, 255, 255, 255],
  ['pro-niko', 'NiKo', 2, 2, 2, 960, 0, 255, 145, 255],
  ['pro-kyousuke', 'kyousuke', 2, 2, 1, 960, 0, 255, 0, 200],
  ['pro-b1t', 'b1t', 2, 2, 1, 960, 0, 255, 0, 200],
  ['pro-torzsi', 'torzsi', 2, 2, 1, 960, 0, 255, 0, 255],
  ['pro-910', '910', 2, 2, 1, 960, 255, 255, 255, 200],
  ['pro-flamez', 'flameZ', 3, 1, 2, 960, 0, 229, 255, 255],
  ['pro-yekindar', 'YEKINDAR', 2, 2, 1, 1050, 38, 241, 27, 255],
].map(([id, name, len, thick, gap, h, r, g, b, a]) => ({
  id: id as string,
  name: name as string,
  description: '职业选手预设',
  builtin: true,
  crosshair: {
    ...defaultCrosshair(),
    style: 4,
    length: len as number,
    thickness: thick as number,
    gap: gap as number,
    screen_height: h as number,
    outline_mode: 0,
    dot: false,
    t_style: false,
    color: { r: r as number, g: g as number, b: b as number, a: a as number },
  },
}))

const readMockHidden = (): string[] => {
  try {
    const raw = localStorage.getItem(CROSSHAIR_HIDDEN_KEY)
    return raw ? (JSON.parse(raw) as string[]) : []
  } catch {
    return []
  }
}

const readMockUserPresets = (): MockUserPreset[] => {
  try {
    const raw = localStorage.getItem(CROSSHAIR_USER_KEY)
    return raw ? (JSON.parse(raw) as MockUserPreset[]) : []
  } catch {
    return []
  }
}

const toPreset = (u: MockUserPreset): CrosshairPreset => ({
  id: u.id,
  name: u.name,
  description: `我的预设 · ${u.created_at}`,
  builtin: false,
  crosshair: u.crosshair,
})

export const getCrosshairPresets = async (): Promise<CrosshairPreset[]> => {
  if (window.go?.main?.App?.GetCrosshairPresets) {
    return wire<CrosshairPreset[]>(await App.GetCrosshairPresets())
  }
  const hidden = readMockHidden()
  const builtin = MOCK_PRO_PRESETS.filter((p) => !hidden.includes(p.id))
  return [...builtin, ...readMockUserPresets().map(toPreset)]
}

export const getHiddenCrosshairPresets = async (): Promise<CrosshairPreset[]> => {
  if (window.go?.main?.App?.GetHiddenCrosshairPresets) {
    return wire<CrosshairPreset[]>(await App.GetHiddenCrosshairPresets())
  }
  const hidden = readMockHidden()
  return MOCK_PRO_PRESETS.filter((p) => hidden.includes(p.id))
}

export const saveCrosshairPreset = async (name: string, crosshair: Crosshair): Promise<CrosshairPreset> => {
  if (window.go?.main?.App?.SaveCrosshairPreset) {
    return wire<CrosshairPreset>(await App.SaveCrosshairPreset(name, crosshairArg(crosshair)))
  }
  const preset: MockUserPreset = {
    id: `user-${Date.now()}`,
    name: name.trim() || '我的准星',
    created_at: new Date().toLocaleString(),
    crosshair,
  }
  const all = readMockUserPresets()
  all.push(preset)
  localStorage.setItem(CROSSHAIR_USER_KEY, JSON.stringify(all))
  return toPreset(preset)
}

export const deleteCrosshairPreset = async (id: string): Promise<void> => {
  if (window.go?.main?.App?.DeleteCrosshairPreset) {
    await App.DeleteCrosshairPreset(id)
    return
  }
  const all = readMockUserPresets().filter((p) => p.id !== id)
  localStorage.setItem(CROSSHAIR_USER_KEY, JSON.stringify(all))
}

export const setCrosshairPresetHidden = async (id: string, hidden: boolean): Promise<void> => {
  if (window.go?.main?.App?.SetCrosshairPresetHidden) {
    await App.SetCrosshairPresetHidden(id, hidden)
    return
  }
  const cur = readMockHidden()
  const next = hidden ? [...new Set([...cur, id])] : cur.filter((x) => x !== id)
  localStorage.setItem(CROSSHAIR_HIDDEN_KEY, JSON.stringify(next))
}

export const getCrosshairCommands = async (crosshair: Crosshair): Promise<CrosshairCommandPreview> => {
  if (window.go?.main?.App?.GetCrosshairCommands) {
    return wire<CrosshairCommandPreview>(await App.GetCrosshairCommands(crosshairArg(crosshair)))
  }
  // 浏览器预览下按后端同一顺序拼一份，保证面板不是空的。
  const c = crosshair
  const b = (v: boolean) => (v ? '1' : '0')
  const lines: CrosshairConvar[] = [
    { key: 'cl_crosshairstyle', value: String(c.style) },
    { key: 'cl_crosshair_length', value: String(c.length) },
    { key: 'cl_crosshair_thickness', value: String(c.thickness) },
    { key: 'cl_crosshair_gap', value: String(c.gap) },
    { key: 'cl_crosshairdot', value: b(c.dot) },
    { key: 'cl_crosshair_t', value: b(c.t_style) },
    { key: 'cl_crosshair_recoil', value: b(c.follow_recoil) },
    { key: 'cl_crosshaircolor_r', value: String(c.color.r) },
    { key: 'cl_crosshaircolor_g', value: String(c.color.g) },
    { key: 'cl_crosshaircolor_b', value: String(c.color.b) },
    { key: 'cl_crosshaircolor_a', value: String(c.color.a) },
    { key: 'cl_crosshair_drawoutline', value: String(c.outline_mode) },
    { key: 'cl_crosshairoutline_r', value: String(c.outline_color.r) },
    { key: 'cl_crosshairoutline_g', value: String(c.outline_color.g) },
    { key: 'cl_crosshairoutline_b', value: String(c.outline_color.b) },
    { key: 'cl_crosshairoutline_a', value: String(c.outline_color.a) },
    { key: 'cl_crosshair_dynamic_spread_limit', value: String(c.spread_limit) },
    { key: 'cl_crosshair_dynamic_splitdist', value: String(c.split_dist) },
    { key: 'cl_crosshair_dynamic_splitalpha_innermod', value: String(c.inner_alpha) },
    { key: 'cl_crosshair_dynamic_splitalpha_outermod', value: String(c.outer_alpha) },
    { key: 'cl_crosshair_dynamic_maxdist_splitratio', value: String(c.split_ratio) },
    { key: 'cl_ironsight_dot_scale', value: String(c.scope_dot_scale) },
    { key: 'cl_ironsight_usecrosshaircolor', value: b(c.scope_dot_use_color) },
    { key: 'cl_crosshair_screen_height', value: String(c.screen_height) },
  ]
  return {
    lines,
    console: lines.map((l) => `${l.key} ${l.value}`).join('; '),
    share: '',
  }
}
