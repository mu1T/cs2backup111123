import { ref } from 'vue'
import { getSnapshots, createSnapshot, restoreVersion, deleteSnapshot, compareSnapshot } from '../utils/wails'
import type { SnapshotInfo, FileDiffItem } from '../types'

export interface UseSnapshotsOptions {
  showMessage: (msg: string, isError?: boolean) => void
  logAction: (msg: string) => void
  setBusy: (busy: boolean) => void
  confirm: (opts: {
    title: string
    message: string
    detail?: string
    icon?: 'info' | 'warning' | 'danger' | 'restore' | 'switch'
    confirmText?: string
    isDanger?: boolean
    onConfirm: () => Promise<void> | void
  }) => void
  /** 新建快照后账号「有备份」状态会变。 */
  reloadBackedUpAccounts: () => Promise<void>
  /** 还原后要重查撤销状态并重读该账号的画质 / 启动项摘要。 */
  reloadAccountDetail: (accountID: string) => Promise<void>
}

/**
 * 配置快照：列表、新建、还原、删除、与本地实时配置比对。
 *
 * 从 LocalBackupView 里搬出来的。快照这块有九份状态（列表、加载中、备注、
 * 差异弹窗、差异项……）全散在顶层，而它只服务「快照」这一个职责。
 */
export function useSnapshots(opts: UseSnapshotsOptions) {
  const list = ref<SnapshotInfo[]>([])
  const loading = ref(false)
  const note = ref('')
  const modalVisible = ref(false)

  const diffVisible = ref(false)
  const diffSnapshotId = ref('')
  const diffItems = ref<FileDiffItem[]>([])
  const diffLoading = ref(false)

  async function load(accountID: string) {
    if (!accountID || accountID === 'game_cfg') {
      list.value = []
      return
    }
    loading.value = true
    try {
      list.value = await getSnapshots(accountID)
    } catch (err) {
      // 读不到就保留原列表，不要把用户眼前的快照清空成「一个都没有」。
      console.warn(`Failed to load snapshots for ${accountID}:`, err)
    } finally {
      loading.value = false
    }
  }

  /** 打开快照面板前先刷一次列表，避免显示上一次的。 */
  async function open(accountID: string) {
    if (!accountID || accountID === 'game_cfg') {
      opts.showMessage('请先选择一个 Steam 账号', true)
      return
    }
    await load(accountID)
    modalVisible.value = true
  }

  function close() {
    modalVisible.value = false
  }

  async function create(accountID: string) {
    if (!accountID || accountID === 'game_cfg') {
      opts.showMessage('请先选择要创建快照的账号', true)
      return
    }

    opts.setBusy(true)
    try {
      const text = note.value.trim()
      opts.logAction(`正在为账号 [${accountID}] 创建配置备份 (备注: ${text || '无'})...`)
      const snap = await createSnapshot(accountID, text)
      note.value = ''
      await load(accountID)
      await opts.reloadBackedUpAccounts()
      opts.showMessage(`成功为账号 [${accountID}] 创建快照 [${snap.id}]！`)
      opts.logAction(`快照 [${snap.id}] 创建成功 (哈希去重已应用)`)
    } catch (err) {
      opts.showMessage(`创建快照失败: ${(err as Error)?.message || err}`, true)
    } finally {
      opts.setBusy(false)
    }
  }

  /** 还原某个快照。不可逆（虽然系统会自动留一份 restore_originals），先问。 */
  function restore(accountID: string, snap: SnapshotInfo) {
    if (!accountID || accountID === 'game_cfg') return

    opts.confirm({
      title: '还原快照版本',
      message: `确认将快照 [${snap.id}] (${snap.note || '无备注'}) 还原至本地账号 [${accountID}] 吗？`,
      detail: '还原前系统将自动备份当前配置至 restore_originals 目录。',
      icon: 'restore',
      confirmText: '确认还原',
      onConfirm: async () => {
        opts.setBusy(true)
        try {
          opts.logAction(`正在还原账号 [${accountID}] 的备份 [${snap.id}]...`)
          await restoreVersion(accountID, snap.id)
          opts.showMessage(`快照 [${snap.id}] 已安全恢复至本地账号 [${accountID}]！`)
          opts.logAction(`账号 [${accountID}] 快照版本 [${snap.id}] 恢复成功`)
          await opts.reloadAccountDetail(accountID)
        } catch (err) {
          opts.showMessage(`恢复快照失败: ${(err as Error)?.message || err}`, true)
        } finally {
          opts.setBusy(false)
        }
      },
    })
  }

  function remove(accountID: string, snap: SnapshotInfo) {
    if (!accountID || accountID === 'game_cfg') return

    opts.confirm({
      title: '删除快照',
      message: `确认删除快照 [${snap.id}] 吗？`,
      detail: '此操作将永久删除该快照存档及其关联的元数据，无法撤销。',
      icon: 'danger',
      confirmText: '确认删除',
      isDanger: true,
      onConfirm: async () => {
        opts.setBusy(true)
        try {
          opts.logAction(`正在删除账号 [${accountID}] 的快照 [${snap.id}]...`)
          await deleteSnapshot(accountID, snap.id)
          await load(accountID)
          opts.showMessage(`快照 [${snap.id}] 已成功删除`)
          opts.logAction(`账号 [${accountID}] 的快照 [${snap.id}] 已删除`)
        } catch (err) {
          opts.showMessage(`删除快照失败: ${(err as Error)?.message || err}`, true)
        } finally {
          opts.setBusy(false)
        }
      },
    })
  }

  /** 比对快照与本地实时配置。差异弹窗自己管理开关，失败时不开。 */
  async function openDiff(accountID: string, snap: SnapshotInfo) {
    diffLoading.value = true
    try {
      opts.logAction(`正在比对账号 [${accountID}] 快照 [${snap.id}] 与本地实时配置差异...`)
      const items = await compareSnapshot(accountID, snap.id)
      diffItems.value = items
      diffSnapshotId.value = snap.id
      diffVisible.value = true
      const changed = items.filter((i) => i.status !== 'identical').length
      opts.logAction(`比对完成，发现 ${changed} 处配置文件变动`)
    } catch (err) {
      opts.showMessage(`比对失败: ${(err as Error)?.message || err}`, true)
    } finally {
      diffLoading.value = false
    }
  }

  function closeDiff() {
    diffVisible.value = false
  }

  return {
    list,
    loading,
    note,
    modalVisible,
    diffVisible,
    diffSnapshotId,
    diffItems,
    diffLoading,
    load,
    open,
    close,
    create,
    restore,
    remove,
    openDiff,
    closeDiff,
  }
}
