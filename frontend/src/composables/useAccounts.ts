import { computed, ref, type Ref } from 'vue'
import { getUsers, saveUserNote } from '../utils/wails'
import type { UserCardInfo } from '../types'

export type AccountFilterMode = 'all' | 'remembered' | 'backed_up'

export interface UseAccountsOptions {
  showMessage: (msg: string, isError?: boolean) => void
  logAction: (msg: string) => void
  /** 写提示条的错误位（账号列表加载失败时用）。 */
  setError: (msg: string) => void
  /** 复制到剪贴板，成功后回调。视图那边有一层权限失败降级。 */
  copy: (text: string, onSuccess: () => void) => void
  /** 哪些账号有本地备份，由 useBackup 提供。 */
  backedUpAccounts: Ref<string[]>
  /**
   * 账号列表刷新后，把跟「选中账号」相关的东西也刷一遍：
   * 备份清单、CS2 cfg 目录、快照、启动项、画质摘要。
   */
  afterUsersLoaded: (accountID: string) => Promise<void>
}

/**
 * Steam 账号列表：加载、选中、筛选、搜索、备注、复制标识符。
 *
 * 这是工作台的中心块——其余几块都要读 `selectedAccountId`。放在最后搬，
 * 是因为前面几块搬完之后它的依赖关系才清楚。
 */
export function useAccounts(opts: UseAccountsOptions) {
  const users = ref<UserCardInfo[]>([])
  const loading = ref(false)
  const selectedAccountId = ref('')

  const searchQuery = ref('')
  const filterMode = ref<AccountFilterMode>('all')

  const editingAccountId = ref<string | null>(null)
  const editingNoteText = ref('')

  const copySuccessId = ref<string | null>(null)
  const copySuccessSteam64 = ref(false)

  const selectedUser = computed(
    () => users.value.find((u) => u.account_id === selectedAccountId.value) ?? null,
  )

  const isSelectedUserBackedUp = computed(() =>
    opts.backedUpAccounts.value.includes(selectedAccountId.value),
  )

  const backedUpCount = computed(
    () => users.value.filter((u) => opts.backedUpAccounts.value.includes(u.account_id)).length,
  )

  const rememberedCount = computed(() => users.value.filter((u) => u.remember_password).length)

  const filteredUsers = computed(() => {
    let list = users.value
    if (filterMode.value === 'remembered') {
      list = list.filter((u) => u.remember_password)
    } else if (filterMode.value === 'backed_up') {
      list = list.filter((u) => opts.backedUpAccounts.value.includes(u.account_id))
    }

    const q = searchQuery.value.trim().toLowerCase()
    if (!q) return list
    return list.filter(
      (u) =>
        u.username.toLowerCase().includes(q) ||
        u.account_id.toLowerCase().includes(q) ||
        u.steam64.toLowerCase().includes(q) ||
        u.notes.toLowerCase().includes(q),
    )
  })

  /**
   * 重新加载账号列表。
   *
   * 选中态的处理有意保守：用户主动选了 `game_cfg`（共享游戏配置）时不动；
   * 当前选中还在列表里时不动；只有在「没选」或「选的那个消失了」时，
   * 才退回到最近登录的账号，再退到第一个。
   */
  async function load() {
    loading.value = true
    opts.setError('')
    try {
      const list = await getUsers()
      users.value = list

      if (list.length > 0) {
        const keepCurrent =
          selectedAccountId.value === 'game_cfg' ||
          list.some((u) => u.account_id === selectedAccountId.value)
        if (!keepCurrent || !selectedAccountId.value) {
          const recent = list.find((u) => u.is_recent)
          selectedAccountId.value = recent ? recent.account_id : list[0].account_id
        }
      }

      await opts.afterUsersLoaded(selectedAccountId.value)
      opts.logAction(`已加载 ${list.length} 个本地 Steam 账号`)
    } catch (err) {
      const msg = (err as Error)?.message || String(err)
      opts.setError(msg)
      opts.logAction(`加载 Steam 账号失败: ${msg}`)
    } finally {
      loading.value = false
    }
  }

  /**
   * 只刷新卡片上的身份信息（昵称、头像）。
   *
   * 后台补到昵称 / 头像后会推事件过来，这里只合并这两个字段：不动选中态、
   * 不重载快照与画质摘要，避免界面整体抖一下。失败就保持现状——锦上添花。
   */
  async function refreshIdentities() {
    try {
      const fresh = await getUsers()
      const byId = new Map(fresh.map((u) => [u.account_id, u]))
      users.value = users.value.map((u) => {
        const f = byId.get(u.account_id)
        return f ? { ...u, username: f.username, avatar_url: f.avatar_url } : u
      })
    } catch {
      // 保持现状
    }
  }

  function select(accountID: string) {
    selectedAccountId.value = accountID
  }

  function setFilter(mode: AccountFilterMode) {
    filterMode.value = mode
  }

  function startEditNote(u: UserCardInfo, event?: Event) {
    if (event) event.stopPropagation()
    editingAccountId.value = u.account_id
    editingNoteText.value = u.notes || ''
  }

  async function saveNote(u: UserCardInfo, event?: Event) {
    if (event) event.stopPropagation()
    const note = editingNoteText.value.trim()
    try {
      await saveUserNote(u.account_id, note)
      // 就地改，不整表重载：重载会让正在编辑的那张卡闪一下。
      u.notes = note
      editingAccountId.value = null
      opts.logAction(`已更新账号 ${u.account_id} 备注为: ${note || '(空)'}`)
    } catch (err) {
      opts.showMessage(`保存备注失败: ${(err as Error)?.message || err}`, true)
    }
  }

  function cancelEditNote(event?: Event) {
    if (event) event.stopPropagation()
    editingAccountId.value = null
    editingNoteText.value = ''
  }

  function copyAccountId(text: string, accountId: string, event?: Event) {
    if (event) event.stopPropagation()
    opts.copy(text, () => {
      copySuccessId.value = accountId
      setTimeout(() => {
        // 期间复制了别的账号的话，不要把新的标记清掉
        if (copySuccessId.value === accountId) copySuccessId.value = null
      }, 1800)
      opts.logAction(`已复制 Account ID: ${text}`)
    })
  }

  function copySteam64(text: string, event?: Event) {
    if (event) event.stopPropagation()
    opts.copy(text, () => {
      copySuccessSteam64.value = true
      setTimeout(() => {
        copySuccessSteam64.value = false
      }, 1800)
      opts.logAction(`已复制 SteamID64: ${text}`)
    })
  }

  return {
    users,
    loading,
    selectedAccountId,
    searchQuery,
    filterMode,
    editingAccountId,
    editingNoteText,
    copySuccessId,
    copySuccessSteam64,
    selectedUser,
    isSelectedUserBackedUp,
    backedUpCount,
    rememberedCount,
    filteredUsers,
    load,
    refreshIdentities,
    select,
    setFilter,
    startEditNote,
    saveNote,
    cancelEditNote,
    copyAccountId,
    copySteam64,
  }
}
