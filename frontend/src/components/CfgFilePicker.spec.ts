import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import CfgFilePicker from './CfgFilePicker.vue'
import type { GameCFGFile } from '../types'

const file = (name: string, source: GameCFGFile['source'], size = 100): GameCFGFile => ({
  name,
  size,
  modified: new Date().toISOString(),
  ext: name.slice(name.lastIndexOf('.')),
  source,
  read_only: false,
})

const FILES: GameCFGFile[] = [
  file('autoexec.cfg', 'player', 9232),
  file('pwa.cfg', 'pwa', 53),
  file('f5eautoexec.cfg', 'fivee', 657),
  file('5e_bind.cfg', 'fivee', 412),
  file('gamemode_casual.cfg', 'valve', 4545),
]

const mountPicker = (props: Record<string, unknown> = {}) =>
  mount(CfgFilePicker, {
    props: { files: [], selected: [], ...props },
  })

describe('CfgFilePicker 来源统计', () => {
  it('按五个来源分类计数，5E 与第三方平台各自独立', () => {
    const wrapper = mountPicker({ files: FILES })
    const chips = wrapper.findAll('.source-chip').map((c) => c.text().replace(/\s+/g, ' '))
    expect(chips).toEqual([
      '自定义 1',
      '完美平台 1',
      '5E 平台 2',
      '第三方平台 0',
      '官方 1',
    ])
  })

  it('5E 文件的行内徽标显示为「5E 平台」而不是「第三方平台」', () => {
    const wrapper = mountPicker({ files: FILES })
    const row = wrapper.findAll('.picker-row').find((r) => r.text().includes('f5eautoexec.cfg'))
    expect(row).toBeTruthy()
    expect(row!.find('.row-source').text()).toBe('5E 平台')
  })
})

describe('CfgFilePicker 空状态', () => {
  it('未配置路径时提示去路径设置，而不是让用户检查路径', () => {
    const wrapper = mountPicker({ pathMissing: true })
    expect(wrapper.find('.picker-empty').text()).toContain('未检测到 CS2 安装目录，请先在路径设置中配置')
  })

  it('路径已配置但没有文件时提示检查安装路径', () => {
    const wrapper = mountPicker({ pathMissing: false })
    expect(wrapper.find('.picker-empty').text()).toContain('未扫描到配置文件')
  })
})

describe('CfgFilePicker 交互与禁用态', () => {
  it('没有勾选时「备份选中」不可点', () => {
    const wrapper = mountPicker({ files: FILES, selected: [] })
    expect(wrapper.find('.picker-submit').attributes('disabled')).toBeDefined()
  })

  it('勾选后「备份选中」可点，并显示已选数量', () => {
    const wrapper = mountPicker({ files: FILES, selected: ['autoexec.cfg'] })
    expect(wrapper.find('.picker-submit').attributes('disabled')).toBeUndefined()
    expect(wrapper.find('.toggle-meta').text()).toContain('1')
  })

  it('点击行复选框抛出 toggle 事件并带上文件名', async () => {
    const wrapper = mountPicker({ files: FILES })
    await wrapper.findAll('.row-check')[0].trigger('change')
    expect(wrapper.emitted('toggle')?.[0]).toEqual(['autoexec.cfg'])
  })

  it('扫描中禁用批量操作按钮', () => {
    const wrapper = mountPicker({ files: FILES, scanning: true })
    const tools = wrapper.findAll('.tool-btn')
    expect(tools[0].attributes('disabled')).toBeDefined()
    expect(wrapper.find('.scanning-hint').exists()).toBe(true)
  })
})
