import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { GameCFGFile } from '../types'

const h = vi.hoisted(() => ({
  getGameCFGFiles: vi.fn(),
  backupGameCFG: vi.fn(),
  restoreGameCFG: vi.fn(),
  rollbackGameCFG: vi.fn(),
  selectCFGFiles: vi.fn(),
}))

vi.mock('../utils/wails', () => h)

import { useGameCfg } from './useGameCfg'

function file(name: string, source: GameCFGFile['source'] = 'player'): GameCFGFile {
  return { name, source, size: 0, modified: '' } as unknown as GameCFGFile
}

function setup(cfgDir = '') {
  const showMessage = vi.fn()
  const logAction = vi.fn()
  const setBusy = vi.fn()
  const confirm = vi.fn()
  const afterRestore = vi.fn(async () => undefined)
  const api = useGameCfg({ showMessage, logAction, setBusy, confirm, cfgDir: () => cfgDir, afterRestore })
  return { api, showMessage, logAction, setBusy, confirm, afterRestore }
}

describe('useGameCfg', () => {
  beforeEach(() => {
    Object.values(h).forEach((fn) => fn.mockReset())
  })

  describe('扫描', () => {
    it('首次扫描只勾玩家自定义的文件', async () => {
      h.getGameCFGFiles.mockResolvedValue([
        file('autoexec.cfg', 'player'),
        file('gamemode.cfg', 'valve'),
        file('5e.cfg', 'fivee'),
      ])
      const { api, logAction } = setup()

      await api.load()

      expect(api.files.value).toHaveLength(3)
      expect(api.selected.value).toEqual(['autoexec.cfg'])
      expect(logAction).toHaveBeenCalledWith('已扫描 CS2 CFG 目录，共 3 个配置文件')
      expect(api.scanning.value).toBe(false)
    })

    it('再次扫描保留原勾选，但丢掉已经消失的文件', async () => {
      h.getGameCFGFiles.mockResolvedValueOnce([file('a.cfg'), file('b.cfg')])
      const { api } = setup()
      await api.load()
      api.selectAll(true)
      expect(api.selected.value).toEqual(['a.cfg', 'b.cfg'])

      // b.cfg 被删了，c.cfg 新增
      h.getGameCFGFiles.mockResolvedValueOnce([file('a.cfg'), file('c.cfg')])
      await api.load()

      expect(api.selected.value).toEqual(['a.cfg'])
    })

    it('再次扫描不会把新出现的玩家文件自动勾上', async () => {
      h.getGameCFGFiles.mockResolvedValueOnce([file('a.cfg')])
      const { api } = setup()
      await api.load()

      h.getGameCFGFiles.mockResolvedValueOnce([file('a.cfg'), file('new.cfg')])
      await api.load()

      expect(api.selected.value).toEqual(['a.cfg'])
    })

    it('扫描失败时清空清单并报错，scanning 复位', async () => {
      // 用「后端返回 null」来触发失败：Go 侧 nil 切片序列化出来就是 null，
      // 这是真实会发生的形态。用 mock 抛错会让 vitest 把它记成 unhandled
      // error（即便调用方已经 await 并捕获），干扰断言。
      h.getGameCFGFiles.mockResolvedValue(null as unknown as GameCFGFile[])
      const { api, showMessage } = setup()

      await api.load()

      expect(api.files.value).toEqual([])
      expect(showMessage).toHaveBeenCalledWith(
        expect.stringContaining('扫描 CFG 目录失败'),
        true,
      )
      expect(api.scanning.value).toBe(false)
    })
  })

  describe('勾选', () => {
    async function withFiles() {
      h.getGameCFGFiles.mockResolvedValue([
        file('a.cfg', 'player'),
        file('b.cfg', 'valve'),
        file('c.cfg', 'pwa'),
      ])
      const ctx = setup()
      await ctx.api.load()
      return ctx
    }

    it('全选 / 全不选', async () => {
      const { api } = await withFiles()
      api.selectAll(true)
      expect(api.selected.value).toEqual(['a.cfg', 'b.cfg', 'c.cfg'])
      api.selectAll(false)
      expect(api.selected.value).toEqual([])
    })

    it('只选玩家自定义', async () => {
      const { api } = await withFiles()
      api.selectAll(true)
      api.selectOnlyCustom()
      expect(api.selected.value).toEqual(['a.cfg'])
    })

    it('toggle 单个文件：没勾就加，勾了就删', async () => {
      const { api } = await withFiles()
      api.toggle('b.cfg')
      expect(api.selected.value).toContain('b.cfg')
      api.toggle('b.cfg')
      expect(api.selected.value).not.toContain('b.cfg')
    })
  })

  describe('快捷入口', () => {
    it('优先表里命中的按表顺序排，不受扫描顺序影响', async () => {
      h.getGameCFGFiles.mockResolvedValue([
        file('server.cfg'),
        file('autoexec.cfg'),
        file('practice.cfg'),
      ])
      const { api } = setup()
      await api.load()

      expect(api.quickFiles.value.map((f) => f.name)).toEqual(['autoexec.cfg', 'practice.cfg', 'server.cfg'])
    })

    it('不足四个时用玩家自定义文件补齐，且不与已选的重复', async () => {
      h.getGameCFGFiles.mockResolvedValue([
        file('autoexec.cfg'),
        file('x.cfg'),
        file('y.cfg'),
        file('z.cfg'),
        file('w.cfg'),
      ])
      const { api } = setup()
      await api.load()

      const names = api.quickFiles.value.map((f) => f.name)
      expect(names).toHaveLength(4)
      expect(names[0]).toBe('autoexec.cfg')
      expect(new Set(names).size).toBe(4)
    })

    it('补齐时不拿平台或官方文件充数', async () => {
      h.getGameCFGFiles.mockResolvedValue([file('autoexec.cfg'), file('valve.cfg', 'valve')])
      const { api } = setup()
      await api.load()

      expect(api.quickFiles.value.map((f) => f.name)).toEqual(['autoexec.cfg'])
    })

    it('空目录时快捷入口为空', () => {
      const { api } = setup()
      expect(api.quickFiles.value).toEqual([])
    })
  })
})

describe('useGameCfg 写入侧', () => {
  beforeEach(() => {
    Object.values(h).forEach((fn) => fn.mockReset())
  })

  describe('备份', () => {
    it('全量备份传 all，并提示条数', async () => {
      h.backupGameCFG.mockResolvedValue(12)
      const { api, showMessage, setBusy } = setup()

      await api.backupAll()

      expect(h.backupGameCFG).toHaveBeenCalledWith('all', [])
      expect(showMessage).toHaveBeenCalledWith('成功全量备份 12 个 CS2 CFG 配置文件！')
      expect(setBusy).toHaveBeenLastCalledWith(false)
    })

    it('只备份自定义时传 custom', async () => {
      h.backupGameCFG.mockResolvedValue(3)
      const { api, showMessage } = setup()

      await api.backupCustom()

      expect(h.backupGameCFG).toHaveBeenCalledWith('custom', [])
      expect(showMessage).toHaveBeenCalledWith('成功备份 3 个玩家自定义 CS2 配置文件！')
    })

    it('备份失败时报错并复位忙碌状态', async () => {
      h.backupGameCFG.mockRejectedValue(new Error('disk full'))
      const { api, showMessage, setBusy } = setup()

      await api.backupAll()

      expect(showMessage).toHaveBeenCalledWith('备份 CS2 CFG 失败: disk full', true)
      expect(setBusy).toHaveBeenLastCalledWith(false)
      // 失败时不该再弹「成功备份 N 个」
      expect(showMessage).toHaveBeenCalledTimes(1)
    })

    it('文件选择器里取消了就什么都不做', async () => {
      h.selectCFGFiles.mockResolvedValue([])
      const { api, logAction } = setup()

      await api.backupPicked()

      expect(h.backupGameCFG).not.toHaveBeenCalled()
      expect(logAction).not.toHaveBeenCalled()
    })

    it('文件选择器抛错时提示，不继续备份', async () => {
      h.selectCFGFiles.mockRejectedValue(new Error('dialog failed'))
      const { api, showMessage } = setup()

      await api.backupPicked()

      expect(showMessage).toHaveBeenCalledWith('选择性备份失败: dialog failed', true)
      expect(h.backupGameCFG).not.toHaveBeenCalled()
    })

    it('挑了文件就把完整路径传给后端', async () => {
      h.selectCFGFiles.mockResolvedValue(['D:/a.cfg', 'D:/b.cfg'])
      h.backupGameCFG.mockResolvedValue(2)
      const { api, showMessage } = setup()

      await api.backupPicked()

      expect(h.backupGameCFG).toHaveBeenCalledWith('selected', ['D:/a.cfg', 'D:/b.cfg'])
      expect(showMessage).toHaveBeenCalledWith('成功备份选定的 2 个配置文件！')
    })

    it('没勾选任何文件时提示，不调后端', async () => {
      const { api, showMessage } = setup()

      await api.backupChecked()

      expect(showMessage).toHaveBeenCalledWith('请先勾选需要备份的配置文件', true)
      expect(h.backupGameCFG).not.toHaveBeenCalled()
    })

    it('勾选备份用生效的 cfg 目录拼完整路径，反斜杠统一成正斜杠', async () => {
      h.getGameCFGFiles.mockResolvedValue([file('a.cfg'), file('b.cfg')])
      h.backupGameCFG.mockResolvedValue(2)
      // 用 charCode 拼反斜杠，避免在测试里写转义时被工具链吃掉
      const bs = String.fromCharCode(92)
      const { api } = setup(`D:${bs}Steam${bs}game${bs}csgo${bs}cfg`)
      await api.load()
      api.selectAll(true)

      await api.backupChecked()

      expect(h.backupGameCFG).toHaveBeenCalledWith('selected', [
        'D:/Steam/game/csgo/cfg/a.cfg',
        'D:/Steam/game/csgo/cfg/b.cfg',
      ])
    })

    it('拿不到 cfg 目录时退回只传文件名', async () => {
      h.getGameCFGFiles.mockResolvedValue([file('a.cfg')])
      h.backupGameCFG.mockResolvedValue(1)
      const { api } = setup('')
      await api.load()
      api.selectAll(true)

      await api.backupChecked()

      expect(h.backupGameCFG).toHaveBeenCalledWith('selected', ['a.cfg'])
    })
  })

  describe('还原与撤销', () => {
    it('还原先弹确认框，不直接调后端', () => {
      const { api, confirm } = setup()
      api.restore()

      expect(confirm).toHaveBeenCalledTimes(1)
      expect(confirm.mock.calls[0][0]).toMatchObject({ icon: 'restore', confirmText: '确认还原' })
      expect(h.restoreGameCFG).not.toHaveBeenCalled()
    })

    it('确认后才还原，并重查撤销状态', async () => {
      h.restoreGameCFG.mockResolvedValue(7)
      const { api, confirm, showMessage, afterRestore } = setup()

      api.restore()
      await confirm.mock.calls[0][0].onConfirm()

      expect(h.restoreGameCFG).toHaveBeenCalledTimes(1)
      expect(showMessage).toHaveBeenCalledWith('成功还原 7 个 CS2 游戏配置文件！')
      expect(afterRestore).toHaveBeenCalledTimes(1)
    })

    it('还原失败时报错，仍然复位忙碌状态', async () => {
      h.restoreGameCFG.mockRejectedValue(new Error('no backup'))
      const { api, confirm, showMessage, setBusy } = setup()

      api.restore()
      await confirm.mock.calls[0][0].onConfirm()

      expect(showMessage).toHaveBeenCalledWith('还原 CS2 CFG 失败: no backup', true)
      expect(setBusy).toHaveBeenLastCalledWith(false)
    })

    it('撤销还原标为危险操作，确认后调后端', async () => {
      h.rollbackGameCFG.mockResolvedValue(5)
      const { api, confirm, showMessage, afterRestore } = setup()

      api.rollback()
      expect(confirm.mock.calls[0][0]).toMatchObject({ isDanger: true, icon: 'restore' })

      await confirm.mock.calls[0][0].onConfirm()

      expect(h.rollbackGameCFG).toHaveBeenCalledTimes(1)
      expect(showMessage).toHaveBeenCalledWith('已成功回退 5 个通用配置文件至还原前的状态！')
      expect(afterRestore).toHaveBeenCalledTimes(1)
    })

    it('撤销失败时报错', async () => {
      h.rollbackGameCFG.mockRejectedValue(new Error('nothing to rollback'))
      const { api, confirm, showMessage } = setup()

      api.rollback()
      await confirm.mock.calls[0][0].onConfirm()

      expect(showMessage).toHaveBeenCalledWith('撤销还原通用 CFG 失败: nothing to rollback', true)
    })
  })
})
