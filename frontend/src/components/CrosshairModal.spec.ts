import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import type { Crosshair } from '../types'

/**
 * 用桩替掉整个绑定层：这组用例只验证编辑器自身的结构行为
 * （弹窗形态、预设显隐、指令 tooltip、保存后的事件），不碰 Go。
 *
 * vi.mock 会被提升到文件顶部，所以桩里用到的数据必须放进 vi.hoisted，
 * 否则会撞上「Cannot access before initialization」。
 */
const h = vi.hoisted(() => {
  const baseCrosshair = (over: Record<string, unknown> = {}) => ({
    format: 'cs2-v1',
    style: 4,
    follow_recoil: true,
    dot: false,
    t_style: false,
    outline_mode: 0,
    color: { r: 0, g: 255, b: 0, a: 255 },
    outline_color: { r: 0, g: 0, b: 0, a: 255 },
    gap: 2,
    length: 2,
    thickness: 2,
    spread_limit: 255,
    split_dist: 3,
    inner_alpha: 0,
    outer_alpha: 1,
    split_ratio: 1,
    screen_height: 960,
    scope_dot_scale: 1,
    scope_dot_use_color: false,
    ...over,
  })

  const proPresets = [
    { id: 'pro-zywoo', name: 'ZywOo', description: 'Vitality', builtin: true, crosshair: baseCrosshair() },
    {
      id: 'pro-donk',
      name: 'donk',
      description: 'Spirit',
      builtin: true,
      crosshair: baseCrosshair({ color: { r: 37, g: 255, b: 0, a: 255 } }),
    },
  ]

  return { baseCrosshair, proPresets, hidden: [], userPresets: [], saveCrosshair: vi.fn(), nullArrays: false }
})

vi.mock('../utils/wails', () => ({
  getCrosshairState: async () => ({
    path: 'C:/Steam/game/csgo/cfg/autoexec.cfg',
    exists: true,
    parsed: {
      crosshair: h.baseCrosshair(),
      // nullArrays 用来复现「Go 的 nil 切片变成 JSON null」这个历史 bug
      found_keys: h.nullArrays ? null : ['cl_crosshairstyle'],
      missing_keys: h.nullArrays ? null : ['cl_crosshair_gap'],
      display_toggle: '1',
      display_toggle_present: true,
    },
    share_code: 'CStest',
  }),
  saveCrosshair: () => h.saveCrosshair(),
  decodeCrosshairCode: async (_accountID: string, _code: string) => h.baseCrosshair(),
  encodeCrosshairCode: async () => 'CStest',
  exportCrosshairCFG: async () => 'C:/out/crosshair.cfg',
  getCrosshairConsole: async () => 'cl_crosshairstyle 4',
  getCrosshairPresets: async () => [...h.proPresets, ...h.userPresets],
  getHiddenCrosshairPresets: async () => h.hidden,
  saveCrosshairPreset: vi.fn(async () => h.userPresets[0]),
  deleteCrosshairPreset: vi.fn(async () => undefined),
  setCrosshairPresetHidden: vi.fn(async () => undefined),
  getCrosshairCommands: async (c: Crosshair) => ({
    lines: [
      { key: 'cl_crosshairstyle', value: String(c.style) },
      { key: 'cl_crosshair_length', value: String(c.length) },
      { key: 'cl_crosshair_gap', value: String(c.gap) },
    ],
    console: `cl_crosshairstyle ${c.style}; cl_crosshair_length ${c.length}`,
    share: 'CStest',
  }),
  defaultCrosshair: () => h.baseCrosshair(),
}))

import CrosshairModal from './CrosshairModal.vue'

const mountModal = () =>
  mount(CrosshairModal, {
    props: { visible: true, accountId: '123456789', username: 'ZywOo_CS2' },
    global: { stubs: { CrosshairScenePreview: true } },
  })

describe('CrosshairModal 弹窗形态', () => {
  beforeEach(() => {
    h.userPresets.length = 0
    h.hidden.length = 0
  })

  it('是居中弹窗（保留遮罩），不再是右侧抽屉', async () => {
    const wrapper = mountModal()
    await flushPromises()
    expect(wrapper.find('.modal-backdrop').exists()).toBe(true)
    expect(wrapper.find('.winui-dialog').exists()).toBe(true)
    expect(wrapper.find('.crosshair-sheet').exists()).toBe(false)
    wrapper.unmount()
  })

  it('顶部是地图场景预览，不再是几倍几倍的方框', async () => {
    const wrapper = mountModal()
    await flushPromises()
    expect(wrapper.find('.scene-strip').exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'CrosshairScenePreview' }).exists()).toBe(true)
    wrapper.unmount()
  })

  it('展示职业选手预设', async () => {
    const wrapper = mountModal()
    await flushPromises()
    const names = wrapper.findAll('.preset-apply').map((n) => n.text())
    expect(names).toContain('ZywOo')
    expect(names).toContain('donk')
    wrapper.unmount()
  })

  it('每个内置预设都带隐藏按钮', async () => {
    const wrapper = mountModal()
    await flushPromises()
    expect(wrapper.findAll('.preset-chip .preset-action').length).toBe(h.proPresets.length)
    wrapper.unmount()
  })

  it('没有用户预设时给出空状态提示', async () => {
    const wrapper = mountModal()
    await flushPromises()
    expect(wrapper.text()).toContain('还没有自定义预设')
    wrapper.unmount()
  })

  it('可见时把账号名与「全局设置」标出来', async () => {
    const wrapper = mountModal()
    await flushPromises()
    expect(wrapper.find('.user-badge').text()).toContain('ZywOo_CS2')
    expect(wrapper.find('.scope-badge').text()).toBe('全局设置')
    wrapper.unmount()
  })
})

describe('CrosshairModal 指令 tooltip', () => {
  beforeEach(() => {
    h.userPresets.length = 0
    h.hidden.length = 0
  })

  it('默认不显示，指令不再常驻占用版面', async () => {
    const wrapper = mountModal()
    await flushPromises()
    expect(wrapper.find('.cmd-tip').exists()).toBe(false)
    expect(wrapper.find('.commands-panel').exists()).toBe(false)
    wrapper.unmount()
  })

  it('悬停页脚按钮时浮出，并列出每个键', async () => {
    const wrapper = mountModal()
    await flushPromises()
    await wrapper.findAll('.tip-trigger')[0].trigger('mouseenter')
    await flushPromises()

    expect(wrapper.find('.cmd-tip').exists()).toBe(true)
    const text = wrapper.find('.tip-body').text()
    expect(text).toContain('cl_crosshairstyle')
    expect(text).toContain('cl_crosshair_gap')
    wrapper.unmount()
  })

  it('倒计时未结束时移开就收起', async () => {
    const wrapper = mountModal()
    await flushPromises()
    const trigger = wrapper.findAll('.tip-trigger')[0]
    await trigger.trigger('mouseenter')
    await flushPromises()
    await trigger.trigger('mouseleave')
    await flushPromises()
    expect(wrapper.find('.cmd-tip').exists()).toBe(false)
    wrapper.unmount()
  })

  it('关闭按钮可主动收起', async () => {
    const wrapper = mountModal()
    await flushPromises()
    await wrapper.findAll('.tip-trigger')[0].trigger('mouseenter')
    await flushPromises()
    await wrapper.find('.tip-close').trigger('click')
    await flushPromises()
    expect(wrapper.find('.cmd-tip').exists()).toBe(false)
    wrapper.unmount()
  })

  it('只有页脚左侧两个按钮触发 tooltip', async () => {
    const wrapper = mountModal()
    await flushPromises()
    expect(wrapper.findAll('.dialog-footer .tip-trigger').length).toBe(2)
    expect(wrapper.findAll('.dialog-footer button').length).toBe(4)
    wrapper.unmount()
  })

  it('写入 autoexec.cfg 与关闭都不触发 tooltip', async () => {
    const wrapper = mountModal()
    await flushPromises()
    const buttons = wrapper.findAll('.dialog-footer button')
    // 后两个是「关闭」和「写入 autoexec.cfg」
    await buttons[2].trigger('mouseenter')
    await buttons[3].trigger('mouseenter')
    await flushPromises()
    expect(wrapper.find('.cmd-tip').exists()).toBe(false)
    wrapper.unmount()
  })
})

describe('CrosshairModal 保存', () => {
  beforeEach(() => {
    h.saveCrosshair.mockReset()
    h.saveCrosshair.mockResolvedValue({
      path: 'C:/Steam/game/csgo/cfg/autoexec.cfg',
      report: {
        replaced_keys: [],
        appended_keys: [],
        commented_keys: [],
        display_toggle: '1',
        preserved_toggle: true,
      },
      backup_path: '',
      share_code: 'CSsaved',
      created: false,
    })
  })

  it('保存后向父级抛出写入结果', async () => {
    const wrapper = mountModal()
    await flushPromises()
    await wrapper.find('.dialog-footer .btn-primary').trigger('click')
    await flushPromises()
    const emitted = wrapper.emitted('saved')
    expect(emitted).toBeTruthy()
    const payload = emitted![0][0] as { share_code: string }
    expect(payload.share_code).toBe('CSsaved')
    wrapper.unmount()
  })
})

describe('CrosshairModal 数值可直接输入', () => {
  it('每个滑块旁都有一个数字输入框', async () => {
    const wrapper = mountModal()
    await flushPromises()
    // 长度 / 粗细 / 间距 三个常驻；瞄准镜中心点在「高级」里；分裂五件套只在样式 2/9 时出现。
    expect(wrapper.findAll('input.num-input').length).toBe(3)

    await wrapper.find('.advanced-toggle').trigger('click')
    await flushPromises()
    expect(wrapper.findAll('input.num-input').length).toBe(4)

    // 切到「动态经典」（style 2）后，分裂参数才展开
    const styleTiles = wrapper.findAll('.style-tile')
    const dynamicClassic = styleTiles.find((t) => t.text().includes('动态经典'))
    await dynamicClassic!.trigger('click')
    await flushPromises()
    expect(wrapper.findAll('input.num-input').length).toBe(9)
    wrapper.unmount()
  })

  it('输入合法值会写回模型', async () => {
    const wrapper = mountModal()
    await flushPromises()
    const lengthInput = wrapper.findAll('input.num-input')[0]
    await lengthInput.setValue('20')
    await flushPromises()
    const range = wrapper.findAll('input[type="range"]')[0]
    expect((range.element as HTMLInputElement).value).toBe('20')
    wrapper.unmount()
  })

  it('输入越界值在失焦时被夹取', async () => {
    const wrapper = mountModal()
    await flushPromises()
    const lengthInput = wrapper.findAll('input.num-input')[0]
    await lengthInput.setValue('999')
    await lengthInput.trigger('change')
    await flushPromises()
    expect((lengthInput.element as HTMLInputElement).value).toBe('64')
    wrapper.unmount()
  })

  it('输入非数字时回落到下界而不是 NaN', async () => {
    const wrapper = mountModal()
    await flushPromises()
    const lengthInput = wrapper.findAll('input.num-input')[0]
    await lengthInput.setValue('')
    await lengthInput.trigger('change')
    await flushPromises()
    expect((lengthInput.element as HTMLInputElement).value).toBe('0')
    wrapper.unmount()
  })
})

describe('CrosshairModal 对 Go 的 null 切片免疫', () => {
  beforeEach(() => {
    h.userPresets.length = 0
    h.hidden.length = 0
    h.nullArrays = false
  })

  it('found_keys / missing_keys 为 null 时仍能正常渲染', async () => {
    h.nullArrays = true
    const wrapper = mountModal()
    await flushPromises()
    // 渲染没崩：弹窗与三栏都在
    expect(wrapper.find('.winui-dialog').exists()).toBe(true)
    expect(wrapper.findAll('.param-col').length).toBe(3)
    // 计数按 0 处理，不抛异常
    expect(wrapper.find('.status-chip').text()).toContain('已读取 0/0 项')
    wrapper.unmount()
  })
})

describe('CrosshairModal 场景名', () => {
  it('用英文地图名，不用中文别名', async () => {
    const wrapper = mountModal()
    await flushPromises()
    const labels = wrapper.findAll('.scene-toolbar .segment').map((b) => b.text())
    expect(labels).toContain('Dust II')
    expect(labels).toContain('Nuke')
    expect(labels.join('')).not.toContain('沙城')
    expect(labels.join('')).not.toContain('核子')
    wrapper.unmount()
  })
})
