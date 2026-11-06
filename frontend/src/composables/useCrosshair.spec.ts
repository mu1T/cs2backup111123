import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { CrosshairSaveResult } from '../types'

const h = vi.hoisted(() => ({
  getCrosshairState: vi.fn(),
}))

vi.mock('../utils/wails', () => ({
  getCrosshairState: h.getCrosshairState,
}))

import { useCrosshair } from './useCrosshair'

const CROSSHAIR = {
  format: 'cs2-v1',
  style: 4,
  follow_recoil: false,
  dot: false,
  t_style: false,
  outline_mode: 1,
  color: { r: 0, g: 255, b: 0, a: 255 },
  outline_color: { r: 0, g: 0, b: 0, a: 255 },
  gap: 4,
  length: 7,
  thickness: 2,
  spread_limit: 0,
  split_dist: 0,
  inner_alpha: 0.5,
  outer_alpha: 0.5,
  split_ratio: 0.5,
  screen_height: 1080,
  scope_dot_scale: 1,
  scope_dot_use_color: false,
}

function setup() {
  const showMessage = vi.fn()
  const logAction = vi.fn()
  const api = useCrosshair({ showMessage, logAction })
  return { api, showMessage, logAction }
}

function state(overrides: Record<string, unknown> = {}) {
  return {
    path: 'C:/cs2/game/csgo/cfg/autoexec.cfg',
    exists: true,
    share_code: 'CS-ABCD',
    parsed: {
      crosshair: CROSSHAIR,
      found_keys: [],
      missing_keys: [],
      display_toggle: '1',
      display_toggle_present: true,
    },
    ...overrides,
  }
}

describe('useCrosshair', () => {
  beforeEach(() => {
    h.getCrosshairState.mockReset()
  })

  it('初始状态：弹窗关着，三条摘要都空', () => {
    const { api } = setup()
    expect(api.modalVisible.value).toBe(false)
    expect(api.summary.value).toBe('')
    expect(api.shareCode.value).toBe('')
    expect(api.toggle.value).toBe('')
  })

  it('load 成功后写入摘要、分享码与显示开关', async () => {
    h.getCrosshairState.mockResolvedValue(state())
    const { api } = setup()

    await api.load()

    expect(api.summary.value).toContain('长 7')
    expect(api.shareCode.value).toBe('CS-ABCD')
    expect(api.toggle.value).toBe('1')
  })

  it('load 失败时清空三条摘要且不抛错', async () => {
    h.getCrosshairState.mockRejectedValue(new Error('steam path not configured'))
    const { api } = setup()

    await expect(api.load()).resolves.toBeUndefined()
    expect(api.summary.value).toBe('')
    expect(api.shareCode.value).toBe('')
    expect(api.toggle.value).toBe('')
  })

  it('load 失败会清掉上一次的成功值，不留旧数据', async () => {
    h.getCrosshairState.mockResolvedValueOnce(state())
    h.getCrosshairState.mockRejectedValueOnce(new Error('boom'))
    const { api } = setup()

    await api.load()
    expect(api.shareCode.value).toBe('CS-ABCD')

    await api.load()
    expect(api.summary.value).toBe('')
    expect(api.shareCode.value).toBe('')
  })

  it('open / close 控制弹窗', () => {
    const { api } = setup()
    api.open()
    expect(api.modalVisible.value).toBe(true)
    api.close()
    expect(api.modalVisible.value).toBe(false)
  })

  it('保存后重读摘要并弹提示，同时写一行日志', async () => {
    h.getCrosshairState.mockResolvedValue(state())
    const { api, showMessage, logAction } = setup()

    const result = {
      path: 'C:/cs2/game/csgo/cfg/autoexec.cfg',
      created: false,
      share_code: 'CS-ABCD',
      report: { replaced_keys: ['cl_crosshairgap'], appended_keys: [], commented_keys: [] },
    } as unknown as CrosshairSaveResult

    await api.handleSaved(result)

    expect(h.getCrosshairState).toHaveBeenCalledTimes(1)
    expect(showMessage).toHaveBeenCalledWith('准星设置已生效：已写入 autoexec.cfg，覆盖 1 项。重启 CS2 后加载。')
    expect(logAction).toHaveBeenCalledWith('准星设置已写入 autoexec.cfg')
  })

  it('首次写入时提示「已新建」，并把新增/注释的条数带上', async () => {
    h.getCrosshairState.mockResolvedValue(state())
    const { api, showMessage } = setup()

    await api.handleSaved({
      path: 'p',
      created: true,
      share_code: '',
      report: { replaced_keys: [], appended_keys: ['a', 'b'], commented_keys: ['c'] },
    } as unknown as CrosshairSaveResult)

    expect(showMessage).toHaveBeenCalledWith(
      '准星设置已生效：已新建 autoexec.cfg，新增 2 项，注释旧键 1 项。重启 CS2 后加载。',
    )
  })

  /**
   * 回归：三个清单是 null 时不能让 `.length` 抛出去。
   * 历史 bug——一次 TypeError 让 Vue 中止 patch，界面卡在「读取中…」，
   * 表现为「点了没反应」，查了三轮才定位到。
   */
  it('report 的三个清单是 null 时照样弹提示', async () => {
    h.getCrosshairState.mockResolvedValue(state())
    const { api, showMessage } = setup()

    await api.handleSaved({
      path: 'p',
      created: false,
      share_code: '',
      report: { replaced_keys: null, appended_keys: null, commented_keys: null },
    } as unknown as CrosshairSaveResult)

    expect(showMessage).toHaveBeenCalledWith('准星设置已生效：已写入 autoexec.cfg。重启 CS2 后加载。')
  })

  it('report 整个缺失时也不炸', async () => {
    h.getCrosshairState.mockResolvedValue(state())
    const { api, showMessage } = setup()

    await expect(
      api.handleSaved({ path: 'p', created: false, share_code: '' } as unknown as CrosshairSaveResult),
    ).resolves.toBeUndefined()
    expect(showMessage).toHaveBeenCalled()
  })

  it('保存后重读失败也不影响提示弹出', async () => {
    h.getCrosshairState.mockRejectedValue(new Error('boom'))
    const { api, showMessage } = setup()

    await api.handleSaved({
      path: 'p',
      created: false,
      share_code: '',
      report: { replaced_keys: [], appended_keys: [], commented_keys: [] },
    } as unknown as CrosshairSaveResult)

    expect(showMessage).toHaveBeenCalledWith('准星设置已生效：已写入 autoexec.cfg。重启 CS2 后加载。')
  })
})
