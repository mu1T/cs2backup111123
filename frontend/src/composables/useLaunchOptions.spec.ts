import { describe, it, expect, vi, beforeEach } from 'vitest'

const h = vi.hoisted(() => ({
  getLaunchOptions: vi.fn(),
  setLaunchOptions: vi.fn(),
  isSteamRunning: vi.fn(),
  shutdownSteam: vi.fn(),
}))

vi.mock('../utils/wails', () => h)

import { useLaunchOptions } from './useLaunchOptions'

function setup() {
  const showMessage = vi.fn()
  const logAction = vi.fn()
  const setBusy = vi.fn()
  const markSteamStopped = vi.fn()
  const copy = vi.fn((_text: string, onSuccess: () => void) => onSuccess())
  const api = useLaunchOptions({ showMessage, logAction, setBusy, copy, markSteamStopped })
  return { api, showMessage, logAction, setBusy, markSteamStopped, copy }
}

describe('useLaunchOptions', () => {
  beforeEach(() => {
    Object.values(h).forEach((fn) => fn.mockReset())
  })

  describe('读取', () => {
    it('读到值就写入，loading 复位', async () => {
      h.getLaunchOptions.mockResolvedValue('-novid -high')
      const { api } = setup()

      await api.load('123')

      expect(api.current.value).toBe('-novid -high')
      expect(api.loading.value).toBe(false)
    })

    it('game_cfg 与空 ID 直接清空，不问后端', async () => {
      const { api } = setup()
      await api.load('game_cfg')
      await api.load('')
      expect(h.getLaunchOptions).not.toHaveBeenCalled()
      expect(api.current.value).toBe('')
    })

    it('读取失败时清空而不是留着上一个账号的值', async () => {
      h.getLaunchOptions.mockResolvedValueOnce('-novid')
      const { api } = setup()
      await api.load('123')
      expect(api.current.value).toBe('-novid')

      h.getLaunchOptions.mockResolvedValueOnce(null as unknown as string)
      await api.load('456')
      expect(api.current.value).toBeNull()
    })
  })

  describe('复制', () => {
    it('空启动项不复制', () => {
      const { api, copy } = setup()
      api.copyCurrent()
      expect(copy).not.toHaveBeenCalled()
    })

    it('复制成功后亮 2 秒标记并提示', () => {
      const { api, copy, showMessage } = setup()
      h.getLaunchOptions.mockResolvedValue('-novid')
      api.current.value = '-novid'

      api.copyCurrent()

      expect(copy).toHaveBeenCalledWith('-novid', expect.any(Function))
      expect(api.copySuccess.value).toBe(true)
      expect(showMessage).toHaveBeenCalledWith('CS2 启动项已成功复制至剪贴板')
    })
  })

  describe('编辑弹窗', () => {
    it('打开时用当前值填充输入框', () => {
      const { api } = setup()
      api.current.value = '-novid'
      api.openEdit()
      expect(api.editInput.value).toBe('-novid')
      expect(api.editVisible.value).toBe(true)
    })

    it('快捷参数在弹窗里切换：加 / 减', () => {
      const { api } = setup()
      api.openEdit()
      api.toggleInModal('-novid')
      expect(api.editInput.value).toBe('-novid')
      api.toggleInModal('-novid')
      expect(api.editInput.value).toBe('')
    })

    it('isActive / isActiveInModal 分别看当前值与输入框', () => {
      const { api } = setup()
      api.current.value = '-novid'
      api.editInput.value = '-high'
      expect(api.isActive('-novid')).toBe(true)
      expect(api.isActive('-high')).toBe(false)
      expect(api.isActiveInModal('-high')).toBe(true)
    })
  })

  describe('摘要条上直接切换参数', () => {
    it('追加后写回后端并提示「已追加」', async () => {
      h.setLaunchOptions.mockResolvedValue(undefined)
      const { api, showMessage, setBusy } = setup()
      api.current.value = '-novid'

      await api.toggle('123', '-high')

      expect(h.setLaunchOptions).toHaveBeenCalledWith('123', '-novid -high')
      expect(api.current.value).toBe('-novid -high')
      expect(showMessage).toHaveBeenCalledWith('已追加至启动项: -high')
      expect(setBusy).toHaveBeenLastCalledWith(false)
    })

    it('移除后写回后端并提示「已移除」', async () => {
      h.setLaunchOptions.mockResolvedValue(undefined)
      const { api, showMessage } = setup()
      api.current.value = '-novid -high'

      await api.toggle('123', '-high')

      expect(h.setLaunchOptions).toHaveBeenCalledWith('123', '-novid')
      expect(showMessage).toHaveBeenCalledWith('已从启动项移除: -high')
    })

    it('-freq 是替换而不是叠加', async () => {
      h.setLaunchOptions.mockResolvedValue(undefined)
      const { api } = setup()
      api.current.value = '-novid -freq 60'

      await api.toggle('123', '-freq 144')

      expect(h.setLaunchOptions).toHaveBeenCalledWith('123', '-novid -freq 144')
    })

    it('失败时报错且不改本地值', async () => {
      h.setLaunchOptions.mockRejectedValue(new Error('steam locked'))
      const { api, showMessage, setBusy } = setup()
      api.current.value = '-novid'

      await api.toggle('123', '-high')

      expect(api.current.value).toBe('-novid')
      expect(showMessage).toHaveBeenCalledWith('更新启动项失败: steam locked', true)
      expect(setBusy).toHaveBeenLastCalledWith(false)
    })

    it('game_cfg 与空 ID 不写后端', async () => {
      const { api } = setup()
      await api.toggle('game_cfg', '-novid')
      await api.toggle('', '-novid')
      expect(h.setLaunchOptions).not.toHaveBeenCalled()
    })
  })

  describe('保存', () => {
    it('Steam 没在跑时直接写', async () => {
      h.isSteamRunning.mockResolvedValue(false)
      h.setLaunchOptions.mockResolvedValue(undefined)
      const { api, showMessage } = setup()
      api.editInput.value = '-novid -high'

      await api.save('123')

      expect(h.setLaunchOptions).toHaveBeenCalledWith('123', '-novid -high')
      expect(api.editVisible.value).toBe(false)
      expect(showMessage).toHaveBeenCalledWith('CS2 启动项保存成功！')
    })

    it('Steam 在跑时弹确认框，不写', async () => {
      h.isSteamRunning.mockResolvedValue(true)
      const { api } = setup()

      await api.save('123')

      expect(api.steamPromptVisible.value).toBe(true)
      expect(h.setLaunchOptions).not.toHaveBeenCalled()
    })

    it('确认「先退出 Steam」时先关 Steam 再写，并把进程状态标记为已停', async () => {
      h.shutdownSteam.mockResolvedValue(undefined)
      h.setLaunchOptions.mockResolvedValue(undefined)
      const { api, markSteamStopped } = setup()
      api.steamPromptVisible.value = true
      api.editInput.value = '-novid'

      await api.confirmSave('123', true)

      expect(api.steamPromptVisible.value).toBe(false)
      expect(h.shutdownSteam).toHaveBeenCalledTimes(1)
      expect(markSteamStopped).toHaveBeenCalledTimes(1)
      expect(h.setLaunchOptions).toHaveBeenCalledWith('123', '-novid')
    })

    it('确认「不退出」时只关框，什么都不写', async () => {
      const { api } = setup()
      api.steamPromptVisible.value = true

      await api.confirmSave('123', false)

      expect(api.steamPromptVisible.value).toBe(false)
      expect(h.shutdownSteam).not.toHaveBeenCalled()
      expect(h.setLaunchOptions).not.toHaveBeenCalled()
    })

    it('查不到 Steam 状态时按「没在跑」处理，直接写', async () => {
      h.isSteamRunning.mockRejectedValue(new Error('boom'))
      h.setLaunchOptions.mockResolvedValue(undefined)
      const { api } = setup()
      api.editInput.value = '-novid'

      await api.save('123')

      expect(h.setLaunchOptions).toHaveBeenCalledWith('123', '-novid')
    })

    it('保存前会 trim 掉首尾空白', async () => {
      h.isSteamRunning.mockResolvedValue(false)
      h.setLaunchOptions.mockResolvedValue(undefined)
      const { api } = setup()
      api.editInput.value = '   -novid  -high   '

      await api.save('123')

      expect(h.setLaunchOptions).toHaveBeenCalledWith('123', '-novid  -high')
    })

    it('保存失败时报错，弹窗不关（用户能重试）', async () => {
      h.isSteamRunning.mockResolvedValue(false)
      h.setLaunchOptions.mockRejectedValue(new Error('write failed'))
      const { api, showMessage } = setup()
      api.editVisible.value = true
      api.editInput.value = '-novid'

      await api.save('123')

      expect(api.editVisible.value).toBe(true)
      expect(showMessage).toHaveBeenCalledWith('保存启动项失败: write failed', true)
    })
  })
})
