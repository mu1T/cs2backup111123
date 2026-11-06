/**
 * 账号卡片上的两条摘要：画质与准星。
 *
 * 这两个摘要原本是九个独立 prop，穿过 seam 只为在卡片里拼两行字：
 * 父视图因此成了「画质怎么显示」「准星开关怎么翻译成人话」的唯一知情者，
 * 卡片的 spec 也要准备九项 fixture 才能渲染。
 *
 * 现在拼字符串的活在这一层做完，卡片只收一个对象。
 * 新增一条摘要信息 = 这里加字段 + 卡片加一行，父视图不用动。
 */

/** 画质摘要条。 */
export interface VideoSummary {
  /** 例如「1920×1080」。空串表示没读到配置。 */
  resolutionText: string
  /** 例如「144 Hz」。空串表示未知。 */
  refreshRateText: string
  /** cs2_video.txt 是否被设为只读（防游戏重置画质）。 */
  readOnly: boolean
  /** 默认预设的名字；null 表示还没设默认预设。 */
  defaultPresetName: string | null
  /** 当前画质是否与默认预设一致。 */
  defaultPresetApplied: boolean
  /** 正在读取画质设置。 */
  loading: boolean
}

/** 准星摘要条。 */
export interface CrosshairSummary {
  /** 例如「静态十字 · 长 7 · 粗 2 · 间距 4」。空串表示没读到。 */
  text: string
  /** 当前准星对应的分享码，空串表示尚未读取。 */
  shareCode: string
  /** 显示开关的人话描述。 */
  toggleLabel: string
  /** 显示开关是否处于关闭状态，供徽标换色用。 */
  toggleOff: boolean
}

export interface AccountSummary {
  video: VideoSummary
  crosshair: CrosshairSummary
}

export function emptyAccountSummary(): AccountSummary {
  return {
    video: {
      resolutionText: '',
      refreshRateText: '',
      readOnly: false,
      defaultPresetName: null,
      defaultPresetApplied: false,
      loading: false,
    },
    crosshair: { text: '', shareCode: '', toggleLabel: '', toggleOff: false },
  }
}

/**
 * 把 autoexec.cfg 里 `crosshair` 的原始值翻成人话。
 *
 * 三态：没写 = 跟随游戏内设置，0/false = 已关闭，其余 = 已开启。
 */
export function describeCrosshairToggle(raw: string): { label: string; off: boolean } {
  const v = (raw || '').trim().toLowerCase()
  if (!v) return { label: '跟随游戏内设置', off: false }
  if (v === '0' || v === 'false') return { label: '已关闭', off: true }
  return { label: '已开启', off: false }
}
