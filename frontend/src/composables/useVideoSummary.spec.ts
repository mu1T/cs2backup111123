import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { VideoPreset } from '../types'

const h = vi.hoisted(() => ({
  getVideoPresets: vi.fn(),
  getVideoSettings: vi.fn(),
  saveVideoSettings: vi.fn(),
  applyDefaultVideoPreset: vi.fn(),
}))

vi.mock('../utils/wails', () => h)

import { useVideoSummary } from './useVideoSummary'

function preset(over: Partial<VideoPreset> = {}): VideoPreset {
  return {
    id: 'p1',
    name: '竞技',
    created_at: '',
    is_default: true,
    file_path: '',
    settings: { 'setting.defaultres': '1920', 'setting.defaultresheight': '1080' },
    ...over,
  } as VideoPreset
}

function setup() {
  const showMessage = vi.fn()
  const logAction = vi.fn()
  const setBusy = vi.fn()
  const api = useVideoSummary({ showMessage, logAction, setBusy })
  return { api, showMessage, logAction, setBusy }
}

const SETTINGS = {
  read_only: true,
  settings: {
    'setting.defaultres': '1920',
    'setting.defaultresheight': '1080',
    'setting.aspectratiomode': '1',
    'setting.refreshrate_numerator': '144000',
    'setting.refreshrate_denominator': '1000',
  },
}

describe('useVideoSummary', () => {
  beforeEach(() => {
    Object.values(h).forEach((fn) => fn.mockReset())
  })

  describe('load', () => {
    it('读到配置后算出分辨率、刷新率、只读状态', async () => {
      h.getVideoPresets.mockResolvedValue([preset()])
      h.getVideoSettings.mockResolvedValue(SETTINGS)
      const { api } = setup()

      await api.load('123')

      expect(api.resolutionText.value).toBe('1920 x 1080 (16:9)')
      expect(api.refreshRateText.value).toBe('144 Hz')
      expect(api.readOnly.value).toBe(true)
      expect(api.defaultPresetName.value).toBe('竞技')
      expect(api.loading.value).toBe(false)
    })

    it('预设已套用时 isDefaultPresetApplied 为 true', async () => {
      h.getVideoPresets.mockResolvedValue([preset()])
      h.getVideoSettings.mockResolvedValue(SETTINGS)
      const { api } = setup()

      await api.load('123')
      expect(api.isDefaultPresetApplied.value).toBe(true)
    })

    it('当前设置与预设不一致时为 false', async () => {
      h.getVideoPresets.mockResolvedValue([preset()])
      h.getVideoSettings.mockResolvedValue({
        read_only: false,
        settings: { 'setting.defaultres': '1280', 'setting.defaultresheight': '960' },
      })
      const { api } = setup()

      await api.load('123')
      expect(api.isDefaultPresetApplied.value).toBe(false)
    })

    it('game_cfg 与空 ID 直接清空，不去问后端', async () => {
      h.getVideoPresets.mockResolvedValue([preset()])
      h.getVideoSettings.mockResolvedValue(SETTINGS)
      const { api } = setup()

      await api.load('123')
      expect(api.resolutionText.value).not.toBe('')

      await api.load('game_cfg')
      expect(api.resolutionText.value).toBe('')
      expect(api.readOnly.value).toBe(false)
      expect(api.defaultPreset.value).toBeNull()

      await api.load('')
      expect(h.getVideoSettings).toHaveBeenCalledTimes(1)
    })

    it('预设接口挂了不影响摘要，只是默认预设为空', async () => {
      h.getVideoPresets.mockRejectedValue(new Error('boom'))
      h.getVideoSettings.mockResolvedValue(SETTINGS)
      const { api } = setup()

      await api.load('123')

      expect(api.resolutionText.value).toBe('1920 x 1080 (16:9)')
      expect(api.defaultPresetName.value).toBe('未配置默认预设')
      expect(api.defaultPreset.value).toBeNull()
    })

    it('设置接口挂了时三条文案都变成「未检测到配置」', async () => {
      h.getVideoPresets.mockResolvedValue([preset()])
      h.getVideoSettings.mockRejectedValue(new Error('boom'))
      const { api } = setup()

      await api.load('123')

      expect(api.resolutionText.value).toBe('未检测到配置')
      expect(api.refreshRateText.value).toBe('未检测到配置')
      expect(api.readOnly.value).toBe(false)
      expect(api.loading.value).toBe(false)
    })
  })

  describe('toggleReadOnly', () => {
    it('从只读切到可写：保存时传 false，并重读摘要', async () => {
      h.getVideoPresets.mockResolvedValue([preset()])
      h.getVideoSettings.mockResolvedValue(SETTINGS)
      h.saveVideoSettings.mockResolvedValue(undefined)
      const { api, showMessage } = setup()

      await api.load('123')
      await api.toggleReadOnly('123')

      expect(h.saveVideoSettings).toHaveBeenCalledWith('123', SETTINGS.settings, false)
      expect(showMessage).toHaveBeenCalledWith('已成功解除 cs2_video.txt 只读锁定！')
      expect(h.getVideoSettings).toHaveBeenCalledTimes(2)
    })

    it('摘要里没有原始设置时先补读一次再保存', async () => {
      h.getVideoSettings.mockResolvedValue(SETTINGS)
      h.saveVideoSettings.mockResolvedValue(undefined)
      const { api } = setup()

      // 没 load 过，rawSettings 为空
      await api.toggleReadOnly('123')

      // 一次补读 + 保存后重读一次
      expect(h.getVideoSettings).toHaveBeenCalledTimes(2)
      expect(h.saveVideoSettings).toHaveBeenCalledWith('123', SETTINGS.settings, true)
    })

    it('补读也失败时用空设置去保存，而不是直接放弃', async () => {
      h.getVideoSettings.mockRejectedValue(new Error('boom'))
      h.saveVideoSettings.mockResolvedValue(undefined)
      const { api } = setup()

      await api.toggleReadOnly('123')

      expect(h.saveVideoSettings).toHaveBeenCalledWith('123', {}, true)
    })

    it('失败时报错文案带原因，并复位忙碌状态', async () => {
      h.getVideoSettings.mockResolvedValue(SETTINGS)
      h.saveVideoSettings.mockRejectedValue(new Error('access denied'))
      const { api, showMessage, setBusy } = setup()

      await api.toggleReadOnly('123')

      expect(showMessage).toHaveBeenCalledWith('切换只读锁定失败: access denied', true)
      expect(setBusy).toHaveBeenLastCalledWith(false)
    })

    it('game_cfg 与空 ID 不做任何事', async () => {
      const { api } = setup()
      await api.toggleReadOnly('game_cfg')
      await api.toggleReadOnly('')
      expect(h.saveVideoSettings).not.toHaveBeenCalled()
    })
  })

  describe('applyDefaultPresetTo', () => {
    it('套用后重读摘要并提示', async () => {
      h.applyDefaultVideoPreset.mockResolvedValue(undefined)
      h.getVideoPresets.mockResolvedValue([preset()])
      h.getVideoSettings.mockResolvedValue(SETTINGS)
      const { api, showMessage, setBusy } = setup()

      await api.applyDefaultPresetTo('123', 'ZywOo_CS2')

      expect(h.applyDefaultVideoPreset).toHaveBeenCalledWith('123', true)
      expect(showMessage).toHaveBeenCalledWith('成功为账号 [ZywOo_CS2] 套用默认视频预设 (已开启只读锁定)')
      expect(h.getVideoSettings).toHaveBeenCalledTimes(1)
      expect(setBusy).toHaveBeenLastCalledWith(false)
    })

    it('失败时报错且复位忙碌状态', async () => {
      h.applyDefaultVideoPreset.mockRejectedValue(new Error('no default preset'))
      const { api, showMessage, setBusy } = setup()

      await api.applyDefaultPresetTo('123', 'ZywOo_CS2')

      expect(showMessage).toHaveBeenCalledWith('套用默认视频预设失败: no default preset', true)
      expect(setBusy).toHaveBeenLastCalledWith(false)
    })

    it('没有 accountID 时什么都不做', async () => {
      const { api } = setup()
      await api.applyDefaultPresetTo('', 'x')
      expect(h.applyDefaultVideoPreset).not.toHaveBeenCalled()
    })
  })
})
