export type LogLevel = 'info' | 'success' | 'warning' | 'error'

export interface LogEntry {
  id: number
  timestamp: string
  level: LogLevel
  message: string
}

export interface AppInfo {
  name: string
  version: string
  description: string
  os: string
  arch: string
}

export interface UserCardInfo {
  account_id: string
  steam64: string
  username: string
  account_name?: string
  remember_password?: boolean
  is_recent: boolean
  avatar_url: string
  notes: string
}

export type ActiveTab = 'local' | 'lan'

// Snapshot types
export interface SnapshotInfo {
  id: string
  account_id: string
  created_at: string
  note: string
  total_files: number
  total_bytes: number
  file_hashes: Record<string, string>
}

// Diff types
export type DiffType = 'same' | 'add' | 'del' | 'mod'

export interface DiffLine {
  type: DiffType
  old_line_num: number
  new_line_num: number
  content: string
  old_content?: string
}

export interface DiffResult {
  lines: DiffLine[]
  add_count: number
  del_count: number
  mod_count: number
}

export interface FileDiffItem {
  rel_path: string
  status: 'modified' | 'added' | 'deleted' | 'identical'
  snapshot_hash?: string
  live_hash?: string
  diff?: DiffResult
}

// Display modes (Windows EnumDisplaySettings)
export interface DisplayMode {
  width: number
  height: number
  refresh_rate: number
  bits_per_pel: number
}

// Video Config & Preset types

/**
 * CFG 来源的**唯一**清单。
 * 类型、徽标元数据、计数初值、渲染顺序都从这里派生——
 * 加一种来源只需在这里加一项，其余地方会因为类型不全而编译报错，
 * 不会出现「后端加了一档、前端漏了徽标」的漂移。
 */
export const CFG_SOURCES = ['player', 'pwa', 'fivee', 'platform', 'valve'] as const
export type CfgSource = (typeof CFG_SOURCES)[number]

export interface GameCFGFile {
  name: string
  size: number
  modified: string
  ext: string
  source: CfgSource
  read_only: boolean
}
export interface VideoConfigData {
  file_path: string
  read_only: boolean
  settings: Record<string, string>
}

export interface VideoPreset {
  id: string
  name: string
  created_at: string
  is_default: boolean
  file_path: string
  settings: Record<string, string>
}

// LAN Sharing types
export interface LANServerStatus {
  running: boolean
  url: string
  all_urls: string[]
  port: number
  share_name: string
  pin: string
  pin_enabled: boolean
  shared_count: number
  connections: number
  shared_local_ids: string[]
  shared_zip_paths: string[]
  /** 出站限速（bytes/sec），0 表示不限。 */
  rate_limit: number
  /** 只提供服务的网卡地址，空串表示全部网卡。 */
  bind_address: string
  /** 本机所有可共享的地址，与运行状态无关——界面用它填「监听网卡」的选项。 */
  available_addresses: string[]
}

export interface LANPackageInfo {
  filename: string
  path: string
  size: number
}

export interface SharedBackupItem {
  id: string
  filename: string
  size: number
  backup_name: string
  created_at: string
  contains_cfg: boolean
  contains_userdata: boolean
  is_local: boolean
  account_id?: string
}


export interface DiscoveredDevice {
  ip: string
  port: number
  name: string
  version: string
  last_seen: string
  is_manual?: boolean
}

export type RemoteBackupItem = SharedBackupItem

export interface DownloadedBackupResult {
  account_id: string
  dest_dir: string
  total_files: number
  total_bytes: number
  source_name: string
  download_time: string
}

// Crosshair types
//
// 全部按 2026-09 准星系统重构后的「整数像素制」表达。旧键（cl_crosshairgap、
// cl_crosshaircolor 等）已从游戏删除，不在类型里留位置。

/** 准星分享码容器。legacy 只读兼容，导出永远走 cs2-v1。 */
export type CrosshairFormat = 'cs2-v1' | 'legacy-v1' | 'legacy-v3' | 'legacy-v4'

export interface CrosshairRGBA {
  r: number
  g: number
  b: number
  a: number
}

export interface Crosshair {
  format: CrosshairFormat
  style: number
  follow_recoil: boolean
  dot: boolean
  t_style: boolean
  outline_mode: number
  color: CrosshairRGBA
  outline_color: CrosshairRGBA
  gap: number
  length: number
  thickness: number
  spread_limit: number
  split_dist: number
  inner_alpha: number
  outer_alpha: number
  split_ratio: number
  screen_height: number
  scope_dot_scale: number
  scope_dot_use_color: boolean
}

export interface CrosshairParsedCFG {
  crosshair: Crosshair
  found_keys: string[]
  missing_keys: string[]
  display_toggle: string
  display_toggle_present: boolean
}

export interface CrosshairState {
  path: string
  exists: boolean
  parsed: CrosshairParsedCFG
  share_code: string
}

export interface CrosshairWriteReport {
  replaced_keys: string[]
  appended_keys: string[]
  commented_keys: string[]
  display_toggle: string
  preserved_toggle: boolean
}

export interface CrosshairSaveResult {
  path: string
  report: CrosshairWriteReport
  backup_path: string
  share_code: string
  created: boolean
}

export interface CrosshairPreset {
  id: string
  name: string
  description: string
  /** true = 内置职业选手预设；false = 用户自己存的。 */
  builtin: boolean
  crosshair: Crosshair
}

/** 一条准星指令，例如 `cl_crosshair_gap 4`。 */
export interface CrosshairConvar {
  key: string
  value: string
}

/** 编辑器里「看得见的键」：逐条指令 + 单行控制台串 + 分享码。 */
export interface CrosshairCommandPreview {
  lines: CrosshairConvar[]
  console: string
  share: string
}

/**
 * 样式清单，顺序与官方设置界面的下拉一致（静态在前）。
 *
 * `short` 用于十宫格里的按钮：窄窗口下每个格子只有 60px 左右，
 * 全称会折成三行把整行撑高，短名更稳；全称仍用于「当前：xxx」提示。
 */
export const CROSSHAIR_STYLES: { value: number; label: string; short: string }[] = [
  { value: 4, label: '静态十字', short: '静态十字' },
  { value: 3, label: '静态圆形', short: '静态圆形' },
  { value: 8, label: '静态方形', short: '静态方形' },
  { value: 6, label: '仅中心点', short: '仅中心点' },
  { value: 9, label: '静态四分', short: '静态四分' },
  { value: 0, label: '动态十字', short: '动态十字' },
  { value: 1, label: '动态圆形', short: '动态圆形' },
  { value: 2, label: '动态十字（经典）', short: '动态经典' },
  { value: 5, label: '静态十字（射击反馈）', short: '射击反馈' },
  { value: 7, label: '动态四分', short: '动态四分' },
]

/** 会用到动态分裂参数的样式：2 经典动态、9 静态四分。 */
export const CROSSHAIR_DYNAMIC_STYLES = [2, 9]

/**
 * 样式按几何形状分组。绘制、预览、描述三处都要用到这些分类，
 * 集中在这里定义，避免同一个集合散在多个文件里各写一份。
 */
export const CROSSHAIR_CROSS_STYLES = [0, 2, 4, 5, 7]
export const CROSSHAIR_CIRCLE_STYLES = [1, 3]
/** 圆环类几何（含静态四分的扇环），判断「准星占多大范围」时用。 */
export const CROSSHAIR_ROUND_STYLES = [1, 3, 9]
/** 静止态要用「给中心点让位」的间距、而不是用户设的 gap 的样式。 */
export const CROSSHAIR_DYNAMIC_GAP_STYLES = [0, 1]

export const CROSSHAIR_OUTLINE_MODES: { value: number; label: string }[] = [
  { value: 0, label: '无描边' },
  { value: 1, label: '完整描边' },
  { value: 2, label: '半描边' },
]

export const CROSSHAIR_SCREEN_HEIGHTS = [720, 768, 900, 960, 1080, 1440, 2160]

/** 场景预览可用的三张地图参考图。 */
export type SceneMap = 'dust2' | 'ancient' | 'nuke' | 'mirage' | 'inferno'
