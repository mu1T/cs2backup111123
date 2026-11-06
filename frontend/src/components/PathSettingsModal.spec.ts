import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'

/**
 * 路径设置弹窗：重点验证「CS2 安装目录」这条新增项。
 * 留空 = 自动探测，这个契约靠测试锁住，免得以后被改成「空值也写一次」。
 */
const h = vi.hoisted(() => ({
  setSteamPath: vi.fn(async () => undefined),
  setBackupPath: vi.fn(async () => undefined),
  setCS2Path: vi.fn(async () => undefined),
  selectSteamDirectory: vi.fn(async () => ''),
  selectBackupDirectory: vi.fn(async () => ''),
  selectCS2Directory: vi.fn(async () => ''),
  detectSteamPath: vi.fn(async () => ''),
  openPathInExplorer: vi.fn(async () => undefined),
  getBackupRetention: vi.fn(async () => 10),
  setBackupRetention: vi.fn(async () => undefined),
}))

vi.mock('../utils/wails', () => ({
  setSteamPath: h.setSteamPath,
  setBackupPath: h.setBackupPath,
  setCS2Path: h.setCS2Path,
  selectSteamDirectory: h.selectSteamDirectory,
  selectBackupDirectory: h.selectBackupDirectory,
  selectCS2Directory: h.selectCS2Directory,
  detectSteamPath: h.detectSteamPath,
  openPathInExplorer: h.openPathInExplorer,
  getBackupRetention: h.getBackupRetention,
  setBackupRetention: h.setBackupRetention,
}))

import PathSettingsModal from './PathSettingsModal.vue'

const mountModal = (props: Record<string, unknown> = {}) =>
  mount(PathSettingsModal, {
    props: {
      visible: true,
      steamPath: 'C:/Steam',
      backupPath: 'D:/backup',
      cs2Path: '',
      ...props,
    },
    global: { stubs: { Teleport: true } },
  })

describe('PathSettingsModal 的 CS2 目录项', () => {
  beforeEach(() => {
    Object.values(h).forEach((fn) => fn.mockClear())
  })

  it('渲染出三行：Steam / CS2 / 备份仓库', async () => {
    const wrapper = mountModal()
    await flushPromises()
    const labels = wrapper.findAll('.setting-row .label-text').map((n) => n.text())
    expect(labels).toEqual(['Steam 安装目录', 'CS2 安装目录', '备份仓库目录'])
    wrapper.unmount()
  })

  it('说明里写清了「留空即自动探测」', async () => {
    const wrapper = mountModal()
    await flushPromises()
    const hint = wrapper.findAll('.setting-row .label-hint')[1].text()
    expect(hint).toContain('留空则自动探测')
    wrapper.unmount()
  })

  it('填了 CS2 目录就写回后端', async () => {
    const wrapper = mountModal()
    await flushPromises()
    await wrapper.findAll('.setting-row input')[1].setValue('E:/CS2')
    await wrapper.find('.dialog-footer .btn-primary').trigger('click')
    await flushPromises()
    expect(h.setCS2Path).toHaveBeenCalledWith('E:/CS2')
    wrapper.unmount()
  })

  it('点「自动」清空后保存，传的是空串（恢复自动探测）', async () => {
    const wrapper = mountModal({ cs2Path: 'E:/CS2' })
    await flushPromises()
    // 第二行的「自动」按钮
    const autoBtn = wrapper.findAll('.setting-row')[1].findAll('.control-actions button')[1]
    await autoBtn.trigger('click')
    await wrapper.find('.dialog-footer .btn-primary').trigger('click')
    await flushPromises()
    expect(h.setCS2Path).toHaveBeenCalledWith('')
    wrapper.unmount()
  })

  it('没改动就不重复写', async () => {
    const wrapper = mountModal({ cs2Path: 'E:/CS2' })
    await flushPromises()
    await wrapper.find('.dialog-footer .btn-primary').trigger('click')
    await flushPromises()
    expect(h.setCS2Path).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('浏览按钮把选中的目录填进输入框', async () => {
    h.selectCS2Directory.mockResolvedValueOnce('E:/CS2/game/csgo/cfg')
    const wrapper = mountModal()
    await flushPromises()
    const browseBtn = wrapper.findAll('.setting-row')[1].findAll('.control-actions button')[0]
    await browseBtn.trigger('click')
    await flushPromises()
    const input = wrapper.findAll('.setting-row input')[1].element as HTMLInputElement
    expect(input.value).toBe('E:/CS2/game/csgo/cfg')
    wrapper.unmount()
  })
})
