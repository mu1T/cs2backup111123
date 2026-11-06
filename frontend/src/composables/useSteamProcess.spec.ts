import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { UserCardInfo } from '../types'

const h = vi.hoisted(() => ({
  isSteamRunning: vi.fn(),
  startSteam: vi.fn(),
  shutdownSteam: vi.fn(),
  switchSteamAccount: vi.fn(),
}))

vi.mock('../utils/wails', () => h)

import { useSteamProcess } from './useSteamProcess'

const USER: UserCardInfo = {
  account_id: '123456789',
  steam64: '76561198083722517',
  username: 'ZywOo_CS2',
  account_name: 'zywoo_official',
  remember_password: true,
  is_recent: false,
  avatar_url: '',
  notes: '',
}

function setup() {
  const showMessage = vi.fn()
  const logAction = vi.fn()
  const reloadUsers = vi.fn(async () => undefined)
  const api = useSteamProcess({ showMessage, logAction, reloadUsers })
  return { api, showMessage, logAction, reloadUsers }
}

describe('useSteamProcess', () => {
  beforeEach(() => {
    Object.values(h).forEach((fn) => fn.mockReset())
  })

  it('初始状态：Steam 未运行、三个 loading 都关着、切号框关着', () => {
    const { api } = setup()
    expect(api.isSteamActive.value).toBe(false)
    expect(api.steamCheckLoading.value).toBe(false)
    expect(api.steamActionLoading.value).toBe(false)
    expect(api.switchModalVisible.value).toBe(false)
    expect(api.switchTargetUser.value).toBeNull()
  })

  describe('进程状态', () => {
    it('checkSteamStatus 读到在跑就置为 true', async () => {
      h.isSteamRunning.mockResolvedValue(true)
      const { api } = setup()
      await api.checkSteamStatus()
      expect(api.isSteamActive.value).toBe(true)
      expect(api.steamCheckLoading.value).toBe(false)
    })

    it('查询失败时保留上一次的判断，不闪成「未运行」', async () => {
      h.isSteamRunning.mockResolvedValueOnce(true)
      const { api } = setup()
      await api.checkSteamStatus()

      h.isSteamRunning.mockRejectedValueOnce(new Error('boom'))
      await api.checkSteamStatus()
      expect(api.isSteamActive.value).toBe(true)
      expect(api.steamCheckLoading.value).toBe(false)
    })
  })

  describe('启停 Steam', () => {
    it('shutdown 成功后置为未运行并提示', async () => {
      h.shutdownSteam.mockResolvedValue(undefined)
      const { api, showMessage } = setup()

      await api.shutdown()

      expect(api.isSteamActive.value).toBe(false)
      expect(showMessage).toHaveBeenCalledWith('Steam 客户端已安全退出')
      expect(api.steamActionLoading.value).toBe(false)
    })

    it('shutdown 失败时报错文案带原因，状态不动', async () => {
      h.shutdownSteam.mockRejectedValue(new Error('steam not running'))
      const { api, showMessage } = setup()

      await api.shutdown()

      expect(showMessage).toHaveBeenCalledWith('退出 Steam 失败: steam not running', true)
      expect(api.steamActionLoading.value).toBe(false)
    })

    it('start(true) 连 CS2 一起拉起，文案说「拉起 CS2」', async () => {
      h.startSteam.mockResolvedValue(undefined)
      const { api, showMessage } = setup()

      await api.start(true)

      expect(h.startSteam).toHaveBeenCalledWith(true)
      expect(api.isSteamActive.value).toBe(true)
      expect(showMessage).toHaveBeenCalledWith('Steam 已启动并拉起 CS2！')
    })

    it('start(false) 只启客户端，文案不提 CS2', async () => {
      h.startSteam.mockResolvedValue(undefined)
      const { api, showMessage } = setup()

      await api.start(false)

      expect(h.startSteam).toHaveBeenCalledWith(false)
      expect(showMessage).toHaveBeenCalledWith('Steam 客户端已启动！')
    })

    it('start 失败时报错且不把状态改成运行中', async () => {
      h.startSteam.mockRejectedValue(new Error('exe not found'))
      const { api, showMessage } = setup()

      await api.start(true)

      expect(showMessage).toHaveBeenCalledWith('启动 Steam 失败: exe not found', true)
      expect(api.isSteamActive.value).toBe(false)
      expect(api.steamActionLoading.value).toBe(false)
    })
  })

  describe('切号', () => {
    it('requestSwitch 只开框，不调后端', () => {
      const { api } = setup()
      api.requestSwitch(USER)
      expect(api.switchModalVisible.value).toBe(true)
      // ref 会把对象转成响应式代理，用 toEqual 比内容
      expect(api.switchTargetUser.value).toEqual(USER)
      expect(h.switchSteamAccount).not.toHaveBeenCalled()
    })

    it('confirmSwitch 用 account_name 切号，成功后关框、重读账号、重查进程', async () => {
      h.switchSteamAccount.mockResolvedValue(undefined)
      h.isSteamRunning.mockResolvedValue(true)
      const { api, showMessage, reloadUsers } = setup()

      api.requestSwitch(USER)
      await api.confirmSwitch()

      expect(h.switchSteamAccount).toHaveBeenCalledWith('zywoo_official')
      expect(api.switchModalVisible.value).toBe(false)
      expect(showMessage).toHaveBeenCalledWith('切换成功，Steam 正在重新启动...')
      expect(reloadUsers).toHaveBeenCalledTimes(1)
      expect(api.isSteamActive.value).toBe(true)
      expect(api.switchActionLoading.value).toBe(false)
    })

    it('没有 account_name 时退回用 account_id 切号', async () => {
      h.switchSteamAccount.mockResolvedValue(undefined)
      h.isSteamRunning.mockResolvedValue(true)
      const { api } = setup()

      api.requestSwitch({ ...USER, account_name: '' })
      await api.confirmSwitch()

      expect(h.switchSteamAccount).toHaveBeenCalledWith('123456789')
    })

    it('切号失败时框保持打开，让用户能重试', async () => {
      h.switchSteamAccount.mockRejectedValue(new Error('remember password off'))
      const { api, showMessage } = setup()

      api.requestSwitch(USER)
      await api.confirmSwitch()

      expect(api.switchModalVisible.value).toBe(true)
      expect(showMessage).toHaveBeenCalledWith('切换 Steam 账号失败: remember password off', true)
      expect(api.switchActionLoading.value).toBe(false)
    })

    it('没有目标用户时 confirmSwitch 什么都不做', async () => {
      const { api } = setup()
      await api.confirmSwitch()
      expect(h.switchSteamAccount).not.toHaveBeenCalled()
    })
  })
})
