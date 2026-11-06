import type { VideoPreset } from '../types'

/**
 * 画质摘要里的纯计算。
 *
 * 从 LocalBackupView 的 loadAccountVideoSummary 里拆出来的：那个函数把两次
 * 后端调用和「分辨率怎么显示」「刷新率怎么换算」混在一起，要断言只能同时
 * mock 两个接口。纯函数搬出来之后，边界情况（缺键、4:3 还是 16:9、
 * 刷新率是分数还是整数）都能单独测。
 */

/** 分辨率文案，例如 `1920 x 1080 (16:9)`。读不到就是「未检测到配置」。 */
export function describeResolution(settings: Record<string, string>): string {
  const w = settings['setting.defaultres'] || ''
  const h = settings['setting.defaultresheight'] || ''
  if (!w || !h) return '未检测到配置'

  const mode = settings['setting.aspectratiomode'] || ''
  let label: string
  if (mode === '0') {
    label = '4:3'
  } else if (mode === '1') {
    label = '16:9'
  } else if (mode === '2') {
    label = '16:10'
  } else {
    // 没写 aspectratiomode 的老配置，按宽高比猜。1.3~1.35 之间算 4:3，
    // 因为 4:3 的常见分辨率（1024x768、1280x960、1440x1080）都落在这个区间。
    const ratio = Number(w) / Number(h)
    label = ratio > 1.3 && ratio < 1.35 ? '4:3' : '16:9'
  }
  return `${w} x ${h} (${label})`
}

/**
 * 刷新率文案，例如 `144 Hz`。
 *
 * CS2 有两种写法：直接写 `setting.refreshrate` 的整数，或者写分子/分母
 * （`144000/1000`）。分母缺失时按 1000 算，这是游戏自己的默认。
 */
export function describeRefreshRate(settings: Record<string, string>): string {
  const raw = settings['setting.refreshrate_numerator'] || ''
  if (!raw) return '未检测到配置'

  const num = Number(raw)
  const den = Number(settings['setting.refreshrate_denominator']) || 1000
  if (!Number.isFinite(num) || num <= 0) return '未检测到配置'

  const hz = num > 1000 && den > 0 ? Math.round(num / den) : num
  return `${hz} Hz`
}

/**
 * 从预设列表里挑出「默认预设」。
 *
 * 优先 `is_default`；一个都没标默认时退回第一个——用户至少能一键套用某个
 * 预设，比按钮永远是灰的强。
 */
export function pickDefaultPreset(presets: VideoPreset[]): VideoPreset | null {
  if (!presets.length) return null
  return presets.find((p) => p.is_default) || presets[0]
}

/**
 * 当前画质是否与预设一致。
 *
 * 只比预设里出现的键：预设可能只存了用户关心的几项，缺的键不该算不一致。
 */
export function isPresetApplied(
  preset: VideoPreset | null,
  current: Record<string, string> | null | undefined,
): boolean {
  if (!preset || !preset.settings) return false
  if (!current) return false

  const presetKeys = Object.keys(preset.settings)
  if (!presetKeys.length) return false
  if (!Object.keys(current).length) return false

  return presetKeys.every((k) => current[k] === preset.settings[k])
}
