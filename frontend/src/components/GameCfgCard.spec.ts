import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import GameCfgCard from './GameCfgCard.vue'
import type { GameCFGFile } from '../types'

const file = (name: string, source: GameCFGFile['source']): GameCFGFile => ({
  name,
  size: 100,
  modified: new Date().toISOString(),
  ext: name.slice(name.lastIndexOf('.')),
  source,
  read_only: false,
})

const FILES = [file('autoexec.cfg', 'player'), file('f5eautoexec.cfg', 'fivee')]

const mountCard = (props: Record<string, unknown> = {}) =>
  mount(GameCfgCard, {
    props: { cs2CfgPath: '', files: [], selected: [], quickFiles: [], ...props },
  })

describe('GameCfgCard 路径缺失', () => {
  it('未配置路径时给出提示与直达路径设置的入口', () => {
    const wrapper = mountCard()
    expect(wrapper.find('.cfg-empty-hint').text()).toContain('未检测到 CS2 安装目录，请先在路径设置中配置')
    expect(wrapper.find('.cfg-path-indicator').text()).toBe('未检测到')
  })

  it('点「打开路径设置」抛出事件而不是自己弹窗', async () => {
    const wrapper = mountCard()
    await wrapper.find('.hint-action').trigger('click')
    expect(wrapper.emitted('open-path-settings')).toHaveLength(1)
  })

  it('路径缺失时动作按钮全部禁用', () => {
    const wrapper = mountCard()
    const buttons = wrapper.findAll('.btn-winui-standard')
    expect(buttons.length).toBeGreaterThan(0)
    buttons.forEach((b) => expect(b.attributes('disabled')).toBeDefined())
  })
})

describe('GameCfgCard 正常态', () => {
  const ready = { cs2CfgPath: 'C:\\Steam\\game\\csgo\\cfg', files: FILES, selected: [] }

  it('已挂载时显示状态徽标且按钮可点', () => {
    const wrapper = mountCard(ready)
    expect(wrapper.find('.cfg-path-indicator').text()).toBe('已挂载')
    expect(wrapper.find('.cfg-empty-hint').exists()).toBe(false)
    wrapper.findAll('.btn-winui-standard').forEach((b) => expect(b.attributes('disabled')).toBeUndefined())
  })

  it('加载中禁用所有动作', () => {
    const wrapper = mountCard({ ...ready, actionLoading: true })
    wrapper.findAll('.btn-winui-standard').forEach((b) => expect(b.attributes('disabled')).toBeDefined())
  })

  it('只有存在回滚归档时才出现「撤销还原」', () => {
    expect(mountCard(ready).find('.btn-winui-danger-subtle').exists()).toBe(false)
    expect(mountCard({ ...ready, canRollback: true }).find('.btn-winui-danger-subtle').exists()).toBe(true)
  })

  it('动作按钮抛出对应事件', async () => {
    const wrapper = mountCard(ready)
    const [backupAll, backupCustom, restore, openDir] = wrapper.findAll('.btn-winui-standard')
    await backupAll.trigger('click')
    await backupCustom.trigger('click')
    await restore.trigger('click')
    await openDir.trigger('click')
    expect(wrapper.emitted('backup-all')).toHaveLength(1)
    expect(wrapper.emitted('backup-custom')).toHaveLength(1)
    expect(wrapper.emitted('restore')).toHaveLength(1)
    expect(wrapper.emitted('open-dir')).toHaveLength(1)
  })

  it('快速编辑 chips 按文件名抛出 edit-file', async () => {
    const wrapper = mountCard({ ...ready, quickFiles: FILES })
    const chips = wrapper.findAll('.quick-edit-chip')
    expect(chips).toHaveLength(2)
    await chips[1].trigger('click')
    expect(wrapper.emitted('edit-file')?.[0]).toEqual(['f5eautoexec.cfg'])
  })
})
