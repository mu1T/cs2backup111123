import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import type { UserCardInfo } from '../types'

const h = vi.hoisted(() => ({
  getUsers: vi.fn(),
  saveUserNote: vi.fn(),
}))

vi.mock('../utils/wails', () => h)

import { useAccounts } from './useAccounts'

function user(over: Partial<UserCardInfo> = {}): UserCardInfo {
  return {
    account_id: '1',
    steam64: '76561198000000001',
    username: 'alpha',
    account_name: 'alpha',
    remember_password: true,
    is_recent: false,
    avatar_url: '',
    notes: '',
    ...over,
  } as UserCardInfo
}

function setup(backedUp: string[] = []) {
  const showMessage = vi.fn()
  const logAction = vi.fn()
  const setError = vi.fn()
  const copy = vi.fn((_t: string, onSuccess: () => void) => onSuccess())
  const backedUpAccounts = ref<string[]>(backedUp)
  const afterUsersLoaded = vi.fn(async () => undefined)
  const api = useAccounts({ showMessage, logAction, setError, copy, backedUpAccounts, afterUsersLoaded })
  return { api, showMessage, logAction, setError, copy, backedUpAccounts, afterUsersLoaded }
}

describe('useAccounts', () => {
  beforeEach(() => {
    Object.values(h).forEach((fn) => fn.mockReset())
  })

  describe('加载与选中', () => {
    it('首次加载选中最近登录的账号', async () => {
      h.getUsers.mockResolvedValue([user({ account_id: 'a' }), user({ account_id: 'b', is_recent: true })])
      const { api, afterUsersLoaded } = setup()

      await api.load()

      expect(api.users.value).toHaveLength(2)
      expect(api.selectedAccountId.value).toBe('b')
      expect(afterUsersLoaded).toHaveBeenCalledWith('b')
      expect(api.loading.value).toBe(false)
    })

    it('没有 is_recent 时选第一个', async () => {
      h.getUsers.mockResolvedValue([user({ account_id: 'a' }), user({ account_id: 'b' })])
      const { api } = setup()
      await api.load()
      expect(api.selectedAccountId.value).toBe('a')
    })

    it('已选中的账号仍在列表里时不动选中态', async () => {
      h.getUsers.mockResolvedValueOnce([user({ account_id: 'a' }), user({ account_id: 'b' })])
      const { api } = setup()
      await api.load()
      api.select('b')

      h.getUsers.mockResolvedValueOnce([user({ account_id: 'a' }), user({ account_id: 'b', is_recent: true })])
      await api.load()

      expect(api.selectedAccountId.value).toBe('b')
    })

    it('用户主动选了 game_cfg 时，重新加载不会把它顶掉', async () => {
      h.getUsers.mockResolvedValueOnce([user({ account_id: 'a' })])
      const { api } = setup()
      await api.load()
      api.select('game_cfg')

      h.getUsers.mockResolvedValueOnce([user({ account_id: 'a', is_recent: true })])
      await api.load()

      expect(api.selectedAccountId.value).toBe('game_cfg')
    })

    it('选中的账号消失了就退回最近登录的', async () => {
      h.getUsers.mockResolvedValueOnce([user({ account_id: 'a' }), user({ account_id: 'b' })])
      const { api } = setup()
      await api.load()
      api.select('b')

      h.getUsers.mockResolvedValueOnce([user({ account_id: 'a', is_recent: true })])
      await api.load()

      expect(api.selectedAccountId.value).toBe('a')
    })

    it('加载失败时把原因写进提示条并复位 loading', async () => {
      h.getUsers.mockRejectedValue(new Error('steam not found'))
      const { api, setError } = setup()

      await api.load()

      expect(setError).toHaveBeenCalledWith('steam not found')
      expect(api.loading.value).toBe(false)
    })
  })

  describe('身份刷新', () => {
    it('只合并昵称与头像，不动其它字段', async () => {
      h.getUsers.mockResolvedValueOnce([user({ account_id: 'a', notes: '我的备注' })])
      const { api } = setup()
      await api.load()

      h.getUsers.mockResolvedValueOnce([
        user({ account_id: 'a', username: '新昵称', avatar_url: 'http://x/a.png', notes: '' }),
      ])
      await api.refreshIdentities()

      expect(api.users.value[0].username).toBe('新昵称')
      expect(api.users.value[0].avatar_url).toBe('http://x/a.png')
      // 备注是本地数据，不能被接口返回值覆盖
      expect(api.users.value[0].notes).toBe('我的备注')
    })

    it('刷新失败时保持现状', async () => {
      h.getUsers.mockResolvedValueOnce([user({ account_id: 'a', username: 'old' })])
      const { api } = setup()
      await api.load()

      h.getUsers.mockRejectedValueOnce(new Error('boom'))
      await api.refreshIdentities()

      expect(api.users.value[0].username).toBe('old')
    })

    it('新出现的账号不会因为身份刷新被加进列表', async () => {
      h.getUsers.mockResolvedValueOnce([user({ account_id: 'a' })])
      const { api } = setup()
      await api.load()

      h.getUsers.mockResolvedValueOnce([user({ account_id: 'a' }), user({ account_id: 'b' })])
      await api.refreshIdentities()

      expect(api.users.value).toHaveLength(1)
    })
  })

  describe('筛选与搜索', () => {
    async function seeded() {
      h.getUsers.mockResolvedValue([
        user({ account_id: '1', username: 'alpha', remember_password: true, notes: '大号' }),
        user({ account_id: '2', username: 'beta', remember_password: false, notes: '' }),
        user({ account_id: '3', username: 'gamma', remember_password: true, notes: '' }),
      ])
      const ctx = setup(['2'])
      await ctx.api.load()
      return ctx
    }

    it('默认显示全部', async () => {
      const { api } = await seeded()
      expect(api.filteredUsers.value).toHaveLength(3)
    })

    it('只看记住密码的账号', async () => {
      const { api } = await seeded()
      api.setFilter('remembered')
      expect(api.filteredUsers.value.map((u) => u.account_id)).toEqual(['1', '3'])
    })

    it('只看有备份的账号', async () => {
      const { api } = await seeded()
      api.setFilter('backed_up')
      expect(api.filteredUsers.value.map((u) => u.account_id)).toEqual(['2'])
    })

    it('搜索匹配昵称 / 账号 ID / Steam64 / 备注', async () => {
      const { api } = await seeded()
      api.searchQuery.value = '大号'
      expect(api.filteredUsers.value.map((u) => u.account_id)).toEqual(['1'])

      api.searchQuery.value = 'GAMMA'
      expect(api.filteredUsers.value.map((u) => u.account_id)).toEqual(['3'])
    })

    it('筛选与搜索叠加生效', async () => {
      const { api } = await seeded()
      api.setFilter('remembered')
      api.searchQuery.value = 'gamma'
      expect(api.filteredUsers.value.map((u) => u.account_id)).toEqual(['3'])
    })
  })

  describe('统计', () => {
    it('backedUpCount / rememberedCount 跟着数据走', async () => {
      h.getUsers.mockResolvedValue([
        user({ account_id: '1', remember_password: true }),
        user({ account_id: '2', remember_password: false }),
      ])
      const { api } = setup(['1'])
      await api.load()

      expect(api.backedUpCount.value).toBe(1)
      expect(api.rememberedCount.value).toBe(1)
      expect(api.isSelectedUserBackedUp.value).toBe(true)
    })
  })

  describe('备注', () => {
    it('保存备注时就地改，不整表重载', async () => {
      h.getUsers.mockResolvedValue([user({ account_id: '1', notes: '' })])
      h.saveUserNote.mockResolvedValue(undefined)
      const { api, logAction } = setup()
      await api.load()

      api.startEditNote(api.users.value[0])
      api.editingNoteText.value = '  主号  '
      await api.saveNote(api.users.value[0])

      expect(h.saveUserNote).toHaveBeenCalledWith('1', '主号')
      expect(api.users.value[0].notes).toBe('主号')
      expect(api.editingAccountId.value).toBeNull()
      expect(logAction).toHaveBeenCalledWith('已更新账号 1 备注为: 主号')
    })

    it('保存失败时报错，编辑态保留（用户能重试）', async () => {
      h.getUsers.mockResolvedValue([user({ account_id: '1' })])
      h.saveUserNote.mockRejectedValue(new Error('locked'))
      const { api, showMessage } = setup()
      await api.load()

      api.startEditNote(api.users.value[0])
      await api.saveNote(api.users.value[0])

      expect(showMessage).toHaveBeenCalledWith('保存备注失败: locked', true)
      expect(api.editingAccountId.value).toBe('1')
    })

    it('取消编辑清空编辑态', () => {
      const { api } = setup()
      api.startEditNote(user({ notes: 'x' }))
      expect(api.editingNoteText.value).toBe('x')
      api.cancelEditNote()
      expect(api.editingAccountId.value).toBeNull()
      expect(api.editingNoteText.value).toBe('')
    })
  })

  describe('复制', () => {
    it('复制账号 ID 后亮标记', () => {
      const { api, copy } = setup()
      api.copyAccountId('123456789', '123456789')

      expect(copy).toHaveBeenCalledWith('123456789', expect.any(Function))
      expect(api.copySuccessId.value).toBe('123456789')
    })

    it('复制 SteamID64 后亮标记', () => {
      const { api, copy } = setup()
      api.copySteam64('76561198')
      expect(copy).toHaveBeenCalledWith('76561198', expect.any(Function))
      expect(api.copySuccessSteam64.value).toBe(true)
    })

    it('复制时阻止事件冒泡，避免顺带触发选中', () => {
      const { api } = setup()
      const ev = { stopPropagation: vi.fn() } as unknown as Event
      api.copyAccountId('1', '1', ev)
      expect(ev.stopPropagation).toHaveBeenCalled()
    })
  })
})
