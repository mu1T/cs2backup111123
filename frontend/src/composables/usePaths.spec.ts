import { describe, it, expect, vi, beforeEach } from 'vitest'

const h = vi.hoisted(() => ({
  getSteamPath: vi.fn(),
  getBackupPath: vi.fn(),
  getCS2Path: vi.fn(),
  getCS2CFGPath: vi.fn(),
  detectSteamPath: vi.fn(),
  openPathInExplorer: vi.fn(),
}))

vi.mock('../utils/wails', () => h)

import { usePaths } from './usePaths'

function setup(reloadAfterPathChange = vi.fn(async () => undefined)) {
  const showMessage = vi.fn()
  const logAction = vi.fn()
  const api = usePaths({ showMessage, logAction, reloadAfterPathChange })
  return { api, showMessage, logAction, reloadAfterPathChange }
}

describe('usePaths', () => {
  beforeEach(() => {
    Object.values(h).forEach((fn) => fn.mockReset())
  })

  it('初始四条路径都是空、弹窗关着、未在探测', () => {
    const { api } = setup()
    expect(api.steamPathInput.value).toBe('')
    expect(api.backupPathInput.value).toBe('')
    expect(api.cs2PathInput.value).toBe('')
    expect(api.cs2CfgPath.value).toBe('')
    expect(api.pathModalVisible.value).toBe(false)
    expect(api.detecting.value).toBe(false)
  })

  describe('读取', () => {
    it('loadSteamPath 读到值就写入', async () => {
      h.getSteamPath.mockResolvedValue('D:/Steam')
      const { api } = setup()
      await api.loadSteamPath()
      expect(api.steamPathInput.value).toBe('D:/Steam')
    })

    it('loadSteamPath 失败时保留原值，不清空', async () => {
      h.getSteamPath.mockResolvedValueOnce('D:/Steam')
      const { api } = setup()
      await api.loadSteamPath()

      h.getSteamPath.mockRejectedValueOnce(new Error('boom'))
      await api.loadSteamPath()
      // 用户可能刚手填了一个还没保存，读失败不该把它擦掉
      expect(api.steamPathInput.value).toBe('D:/Steam')
    })

    it('loadCS2CFGPath 失败时清空，避免旧目录继续被使用', async () => {
      h.getCS2CFGPath.mockResolvedValueOnce('D:/cs2/game/csgo/cfg')
      const { api } = setup()
      await api.loadCS2CFGPath()
      expect(api.cs2CfgPath.value).toBe('D:/cs2/game/csgo/cfg')

      h.getCS2CFGPath.mockRejectedValueOnce(new Error('steam path not set'))
      await api.loadCS2CFGPath()
      expect(api.cs2CfgPath.value).toBe('')
    })

    it('loadCS2PathOverride 失败时清空，空串代表自动探测', async () => {
      h.getCS2Path.mockRejectedValueOnce(new Error('boom'))
      const { api } = setup()
      await api.loadCS2PathOverride()
      expect(api.cs2PathInput.value).toBe('')
    })
  })

  it('打开路径设置前会先重读手动 CS2 路径', async () => {
    h.getCS2Path.mockResolvedValue('E:/cs2')
    const { api } = setup()

    await api.openPathSettings()

    expect(h.getCS2Path).toHaveBeenCalledTimes(1)
    expect(api.cs2PathInput.value).toBe('E:/cs2')
    expect(api.pathModalVisible.value).toBe(true)
  })

  describe('保存路径', () => {
    const payload = { steamPath: 'D:/Steam', backupPath: 'D:/bak', cs2Path: '' }

    it('三条路径都写入，并重读依赖它的东西', async () => {
      const { api, reloadAfterPathChange } = setup()
      await api.handlePathSaved(payload)

      expect(api.steamPathInput.value).toBe('D:/Steam')
      expect(api.backupPathInput.value).toBe('D:/bak')
      expect(api.cs2PathInput.value).toBe('')
      expect(reloadAfterPathChange).toHaveBeenCalledTimes(1)
    })

    it('没指定手动 CS2 目录时说「已保存」，指定了则点明「改为手动指定」', async () => {
      const auto = setup()
      await auto.api.handlePathSaved(payload)
      expect(auto.showMessage).toHaveBeenCalledWith('路径设置已保存')

      const manual = setup()
      await manual.api.handlePathSaved({ ...payload, cs2Path: 'E:/cs2' })
      expect(manual.showMessage).toHaveBeenCalledWith('路径设置已保存，CS2 目录已改为手动指定')
    })

    it('日志里带上 Steam 路径，便于事后排查', async () => {
      const { api, logAction } = setup()
      await api.handlePathSaved(payload)
      expect(logAction).toHaveBeenCalledWith('已更新路径设置 (Steam: D:/Steam)')
    })

    it('重读依赖发生在路径写入之后', async () => {
      const seen: string[] = []
      const { api } = setup(
        vi.fn(async () => {
          seen.push(api.steamPathInput.value)
        }),
      )
      await api.handlePathSaved(payload)
      expect(seen).toEqual(['D:/Steam'])
    })
  })

  describe('自动侦测', () => {
    it('成功时写入路径、记日志、重读依赖', async () => {
      h.detectSteamPath.mockResolvedValue('E:/Steam')
      const { api, logAction, reloadAfterPathChange } = setup()

      await api.handleAutoDetect()

      expect(api.steamPathInput.value).toBe('E:/Steam')
      expect(logAction).toHaveBeenCalledWith('自动侦测 Steam 目录成功: E:/Steam')
      expect(reloadAfterPathChange).toHaveBeenCalledTimes(1)
      expect(api.detecting.value).toBe(false)
    })

    it('失败时把原因写进提示条，路径保持原样', async () => {
      h.getSteamPath.mockResolvedValue('D:/Steam')
      const { api, showMessage } = setup()
      await api.loadSteamPath()

      h.detectSteamPath.mockRejectedValue(new Error('steam installation not found'))
      await api.handleAutoDetect()

      expect(api.steamPathInput.value).toBe('D:/Steam')
      expect(showMessage).toHaveBeenCalledWith('侦测失败: steam installation not found', true)
      expect(api.detecting.value).toBe(false)
    })

    it('失败时不会去重读依赖（免得把空路径传下去）', async () => {
      h.detectSteamPath.mockRejectedValue(new Error('nope'))
      const { api, reloadAfterPathChange } = setup()
      await api.handleAutoDetect()
      expect(reloadAfterPathChange).not.toHaveBeenCalled()
    })

    it('探测过程中 detecting 为 true，结束后复位（即使抛错）', async () => {
      let during = false
      h.detectSteamPath.mockImplementation(async () => {
        during = true
        throw new Error('nope')
      })
      const { api } = setup()
      await api.handleAutoDetect()
      expect(during).toBe(true)
      expect(api.detecting.value).toBe(false)
    })
  })

  describe('在资源管理器里打开', () => {
    it('Steam 路径为空时提示，不调后端', async () => {
      const { api, showMessage } = setup()
      await api.openSteamInExplorer()
      expect(showMessage).toHaveBeenCalledWith('Steam 安装路径为空', true)
      expect(h.openPathInExplorer).not.toHaveBeenCalled()
    })

    it('备份路径为空时提示，不调后端', async () => {
      const { api, showMessage } = setup()
      await api.openBackupInExplorer()
      expect(showMessage).toHaveBeenCalledWith('备份存储路径为空', true)
      expect(h.openPathInExplorer).not.toHaveBeenCalled()
    })

    it('路径首尾空白会被去掉再打开', async () => {
      h.getSteamPath.mockResolvedValue('  D:/Steam  ')
      const { api } = setup()
      await api.loadSteamPath()
      await api.openSteamInExplorer()
      expect(h.openPathInExplorer).toHaveBeenCalledWith('D:/Steam')
    })

    it('打开失败时报错文案带上原因', async () => {
      h.getBackupPath.mockResolvedValue('D:/bak')
      h.openPathInExplorer.mockRejectedValue(new Error('access denied'))
      const { api, showMessage } = setup()
      await api.loadBackupPath()
      await api.openBackupInExplorer()
      expect(showMessage).toHaveBeenCalledWith('打开路径失败: access denied', true)
    })
  })
})
