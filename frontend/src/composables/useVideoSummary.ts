import { computed, ref } from 'vue'
import { getVideoPresets, getVideoSettings, saveVideoSettings, applyDefaultVideoPreset } from '../utils/wails'
import { describeResolution, describeRefreshRate, pickDefaultPreset, isPresetApplied } from '../utils/videoSummary'
import type { VideoPreset } from '../types'

export interface UseVideoSummaryOptions {
  showMessage: (msg: string, isError?: boolean) => void
  logAction: (msg: string) => void
  /** 让卡片上的按钮转圈。视图把它接到全局 actionLoading 上。 */
  setBusy: (busy: boolean) => void
}

/**
 * 账号卡片上的画质摘要，以及摘要条右侧那两个快捷动作。
 *
 * 从 LocalBackupView 里搬出来的。纯计算部分（分辨率文案、刷新率换算、
 * 预设是否已套用）已经单独放进 utils/videoSummary.ts，这里只管状态与 IO。
 *
 * 动作都显式收 accountID，不去读「当前选中账号」——那样这一块就又被
 * 账号列表绑住了，也没法单独测。
 */
export function useVideoSummary(opts: UseVideoSummaryOptions) {
  const resolutionText = ref('')
  const refreshRateText = ref('')
  /** 与 resolutionText 同值，供画质弹窗复用；两者历史上是分开的，先保留。 */
  const summaryText = ref('')
  const defaultPreset = ref<VideoPreset | null>(null)
  const defaultPresetName = ref('')
  const rawSettings = ref<Record<string, string>>({})
  const readOnly = ref(false)
  const loading = ref(false)

  const isDefaultPresetApplied = computed(() => isPresetApplied(defaultPreset.value, rawSettings.value))

  function reset() {
    summaryText.value = ''
    resolutionText.value = ''
    refreshRateText.value = ''
    defaultPresetName.value = ''
    defaultPreset.value = null
    rawSettings.value = {}
    readOnly.value = false
  }

  /**
   * 读某个账号的画质摘要。
   *
   * game_cfg（共享游戏配置）与空 ID 直接清空：画质是每个账号自己的，
   * 没有「账号」这个概念时显示上一个人的画质会误导。
   */
  async function load(accountID: string) {
    if (!accountID || accountID === 'game_cfg') {
      reset()
      return
    }

    loading.value = true
    try {
      // 默认预设拿不到不影响摘要，只是「套用默认预设」按钮会灰掉。
      try {
        const presets = await getVideoPresets()
        const def = pickDefaultPreset(presets)
        defaultPreset.value = def
        defaultPresetName.value = def ? def.name : '未配置默认预设'
      } catch {
        defaultPreset.value = null
        defaultPresetName.value = '未配置默认预设'
      }

      const data = await getVideoSettings(accountID)
      readOnly.value = !!data.read_only
      const settings = data.settings || {}
      rawSettings.value = settings

      resolutionText.value = describeResolution(settings)
      summaryText.value = resolutionText.value
      refreshRateText.value = describeRefreshRate(settings)
    } catch (err) {
      console.warn(`Failed to load video summary for ${accountID}:`, err)
      rawSettings.value = {}
      summaryText.value = '未检测到配置'
      resolutionText.value = '未检测到配置'
      refreshRateText.value = '未检测到配置'
      readOnly.value = false
    } finally {
      loading.value = false
    }
  }

  /** 切换 cs2_video.txt 的只读锁。摘要里没有原始设置时先补读一次。 */
  async function toggleReadOnly(accountID: string) {
    if (!accountID || accountID === 'game_cfg') return

    const target = !readOnly.value
    opts.setBusy(true)
    opts.logAction(
      target
        ? `正在为账号 [${accountID}] 的 cs2_video.txt 开启只读锁定...`
        : `正在为账号 [${accountID}] 的 cs2_video.txt 解除只读锁定...`,
    )
    try {
      let settings = rawSettings.value
      if (!settings || Object.keys(settings).length === 0) {
        try {
          const data = await getVideoSettings(accountID)
          settings = data.settings || {}
          rawSettings.value = settings
        } catch {
          settings = {}
        }
      }

      await saveVideoSettings(accountID, settings, target)
      readOnly.value = target

      const msg = target
        ? '已成功开启 cs2_video.txt 只读锁定，防止游戏重置画质！'
        : '已成功解除 cs2_video.txt 只读锁定！'
      opts.showMessage(msg)
      opts.logAction(msg)
      await load(accountID)
    } catch (err) {
      const detail = (err as Error)?.message || err
      opts.showMessage(`切换只读锁定失败: ${detail}`, true)
      opts.logAction(`切换只读锁定失败: ${detail}`)
    } finally {
      opts.setBusy(false)
    }
  }

  /** 把全局默认预设套到某个账号，并顺手开启只读锁。 */
  async function applyDefaultPresetTo(accountID: string, username: string) {
    if (!accountID) return

    opts.setBusy(true)
    opts.logAction(
      `正在将全局默认视频预设套用至账号 [${username}] (${accountID}) 并自动开启只读锁定...`,
    )
    try {
      await applyDefaultVideoPreset(accountID, true)
      await load(accountID)
      opts.showMessage(`成功为账号 [${username}] 套用默认视频预设 (已开启只读锁定)`)
      opts.logAction(`账号 [${accountID}] 默认视频预设套用成功 (只读防篡改锁定已生效)`)
    } catch (err) {
      const detail = (err as Error)?.message || err
      opts.showMessage(`套用默认视频预设失败: ${detail}`, true)
      opts.logAction(`套用默认视频预设失败: ${detail}`)
    } finally {
      opts.setBusy(false)
    }
  }

  return {
    resolutionText,
    refreshRateText,
    summaryText,
    defaultPreset,
    defaultPresetName,
    rawSettings,
    readOnly,
    loading,
    isDefaultPresetApplied,
    load,
    toggleReadOnly,
    applyDefaultPresetTo,
  }
}
