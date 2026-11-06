import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

/**
 * 绑定层（`utils/wails.ts`）的唯一直接测试。
 *
 * 这个文件是所有前后端通信的入口，之前只被上层 composable 用 `vi.mock` 整个
 * 替换掉，从来没有真正跑过它自己的两条分支：
 * - 桌面端：走生成的绑定（`wailsjs/go/main/App`）
 * - 浏览器预览：走 mock 兜底
 *
 * 把调用从 `window.go.main.App.X` 换成生成函数之后，这两条分支都必须还活着。
 */

const h = vi.hoisted(() => ({
  GetAppInfo: vi.fn(),
  GetUsers: vi.fn(),
  GetLogs: vi.fn(),
}))

vi.mock('../../wailsjs/go/main/App', () => h)

import { isWailsAvailable, getAppInfo, getUsers, getLogs } from './wails'

/**
 * 装一个最小的 Wails 运行时桩。
 *
 * `isWailsAvailable` 要求 `window.runtime` 和 `window.go.main.App` **同时**存在
 * ——只装 go 会被判定成浏览器环境，这条一开始就踩了。
 */
function installWailsStub() {
  ;(window as unknown as { runtime?: unknown }).runtime = {
    EventsOn: () => () => undefined,
    EventsEmit: () => undefined,
  }
  ;(window as unknown as { go?: unknown }).go = {
    main: {
      App: {
        GetAppInfo: () => undefined,
        GetUsers: () => undefined,
        GetLogs: () => undefined,
      },
    },
  }
}

function removeWailsStub() {
  delete (window as unknown as { go?: unknown }).go
  delete (window as unknown as { runtime?: unknown }).runtime
}

describe('绑定层', () => {
  beforeEach(() => {
    Object.values(h).forEach((fn) => fn.mockReset())
    removeWailsStub()
  })
  afterEach(() => {
    removeWailsStub()
  })

  describe('环境判定', () => {
    it('没有 window.go 时判定为浏览器环境', () => {
      expect(isWailsAvailable()).toBe(false)
    })

    it('注入了 window.go.main.App 之后判定为桌面环境', () => {
      installWailsStub()
      expect(isWailsAvailable()).toBe(true)
    })
  })

  describe('桌面端：走生成的绑定', () => {
    beforeEach(installWailsStub)

    it('getAppInfo 把结果原样透传', async () => {
      const info = { name: 'CS2', version: '9.9.9', description: '', os: 'windows', arch: 'amd64' }
      h.GetAppInfo.mockResolvedValue(info)

      await expect(getAppInfo()).resolves.toEqual(info)
      expect(h.GetAppInfo).toHaveBeenCalledTimes(1)
    })

    it('getUsers 走生成的 GetUsers', async () => {
      h.GetUsers.mockResolvedValue([{ account_id: '1' }])

      const users = await getUsers()
      expect(users).toHaveLength(1)
      expect(h.GetUsers).toHaveBeenCalledTimes(1)
    })

    it('getLogs 走生成的 GetLogs', async () => {
      h.GetLogs.mockResolvedValue([{ id: 1, timestamp: '', level: 'info', message: 'hi' }])

      const logs = await getLogs()
      expect(logs[0].message).toBe('hi')
      expect(h.GetLogs).toHaveBeenCalledTimes(1)
    })

    it('后端抛错时退回 mock，而不是把异常抛给调用方', async () => {
      h.GetAppInfo.mockRejectedValue(new Error('bridge down'))

      const info = await getAppInfo()
      // 兜底值来自 mock，不是后端
      expect(info.name).toBe('CS2 Config Backup & Sync')
      expect(info.os).toBe('browser')
    })

    it('后端抛错时 getUsers 退回 mock 账号列表', async () => {
      h.GetUsers.mockRejectedValue(new Error('bridge down'))

      const users = await getUsers()
      expect(Array.isArray(users)).toBe(true)
      expect(users.length).toBeGreaterThan(0)
    })
  })

  describe('浏览器预览：走 mock 兜底', () => {
    it('没有 window.go 时不调生成绑定，直接返回兜底值', async () => {
      const info = await getAppInfo()

      expect(h.GetAppInfo).not.toHaveBeenCalled()
      expect(info.name).toBe('CS2 Config Backup & Sync')
    })

    it('没有 window.go 时 getUsers 返回 mock 账号', async () => {
      const users = await getUsers()

      expect(h.GetUsers).not.toHaveBeenCalled()
      expect(users.length).toBeGreaterThan(0)
    })

    it('window.go 存在但缺某个方法时，该方法单独退回 mock', async () => {
      // 只装 GetAppInfo，不装 GetUsers —— 模拟「旧版后端没有新绑定」
      ;(window as unknown as { go?: unknown }).go = {
        main: { App: { GetAppInfo: () => undefined } },
      }
      h.GetAppInfo.mockResolvedValue({ name: 'from-backend', version: '', description: '', os: '', arch: '' })

      const info = await getAppInfo()
      expect(info.name).toBe('from-backend')

      const users = await getUsers()
      expect(h.GetUsers).not.toHaveBeenCalled()
      expect(users.length).toBeGreaterThan(0)
    })
  })
})
