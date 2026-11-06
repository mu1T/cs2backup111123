import { ref } from 'vue'
import {
  previewBackupPrune,
  applyBackupPrune,
  getBackedUpAccounts,
  getRemoteBackups,
  backupUser,
  restoreUser,
  restoreSnapshot,
  rollbackUserRestore,
  hasUserRollback,
  hasGameCFGRollback,
} from '../utils/wails'
import type { UserCardInfo } from '../types'

export interface UseBackupOptions {
  showMessage: (msg: string, isError?: boolean) => void
  logAction: (msg: string) => void
  setBusy: (busy: boolean) => void
  /** 弹确认框。撤销还原是危险操作，必须先问。 */
  confirm: (opts: {
    title: string
    message: string
    detail?: string
    icon?: 'info' | 'warning' | 'danger' | 'restore' | 'switch'
    confirmText?: string
    isDanger?: boolean
    onConfirm: () => Promise<void> | void
  }) => void
  /** 备份成功后要重读该账号的快照列表。 */
  reloadSnapshots: (accountID: string) => Promise<void>
  /** 还原 / 回滚之后要重读该账号的画质与启动项摘要。 */
  reloadAccountDetail: (accountID: string) => Promise<void>
}

/** 「还原来源」弹窗选了什么。 */
export interface RestoreSourcePayload {
  type: 'self' | 'other' | 'snapshot'
  sourceId: string
}

/**
 * 用户配置的备份、还原、撤销还原。
 *
 * 从 LocalBackupView 里搬出来的。这块的状态（哪些账号有备份、有哪些远程备份、
 * 能不能撤销）只服务这一个职责，却和另外几十个 ref 挤在一起。
 *
 * 动作显式收 accountID，不去读「当前选中账号」——那样这块就没法单独测。
 */
export function useBackup(opts: UseBackupOptions) {
  const backedUpAccounts = ref<string[]>([])
  /** 远程备份只用于「还原来源」的展示，不进入账号列表。 */
  const remoteBackups = ref<UserCardInfo[]>([])
  const canRollbackUser = ref(false)
  const canRollbackGameCFG = ref(false)
  const restoreSourceModalVisible = ref(false)
  /** 已经问过收敛的账号——同一个会话里不重复打扰。 */
  const askedPrune = new Set<string>()

  async function loadBackedUpAccounts() {
    try {
      backedUpAccounts.value = await getBackedUpAccounts()
    } catch (err) {
      // 读不到就当没有备份，不要清空成 undefined——模板里直接 includes()。
      console.warn('Failed to load backed up accounts:', err)
    }
    try {
      remoteBackups.value = await getRemoteBackups()
    } catch {
      remoteBackups.value = []
    }
  }

  /**
   * 查「能不能撤销还原」。
   *
   * 账号的撤销和通用 CFG 的撤销是两件独立的事：账号那条只在选了真实账号时查，
   * 通用 CFG 那条任何时候都查。
   */
  async function checkRollbackStatus(accountID: string) {
    if (accountID && accountID !== 'game_cfg') {
      try {
        canRollbackUser.value = await hasUserRollback(accountID)
      } catch {
        canRollbackUser.value = false
      }
    } else {
      canRollbackUser.value = false
    }

    try {
      canRollbackGameCFG.value = await hasGameCFGRollback()
    } catch {
      canRollbackGameCFG.value = false
    }
  }

  async function backup(accountID: string) {
    if (!accountID || accountID === 'game_cfg') {
      opts.showMessage('请先选择要备份的账号', true)
      return
    }

    opts.setBusy(true)
    try {
      opts.logAction(`正在流式备份用户 [${accountID}] 的 CS2 Userdata 配置...`)
      await backupUser(accountID)
      await loadBackedUpAccounts()
      await opts.reloadSnapshots(accountID)
      opts.showMessage(`用户 [${accountID}] 备份成功！`)
      opts.logAction(`用户 [${accountID}] CS2 Userdata 备份成功`)
    } catch (err) {
      opts.showMessage(`备份用户失败: ${(err as Error)?.message || err}`, true)
    } finally {
      opts.setBusy(false)
    }
  }

  function openRestoreSource(accountID: string) {
    if (!accountID || accountID === 'game_cfg') {
      opts.showMessage('请先选择要恢复的目标账号', true)
      return
    }
    restoreSourceModalVisible.value = true
  }

  function closeRestoreSource() {
    restoreSourceModalVisible.value = false
  }

  /**
   * 「还原来源」弹窗点了确认。
   *
   * 三种来源走不同的后端调用，但收尾一样：重查撤销状态、重读该账号的画质与
   * 启动项摘要——还原会整份覆盖 userdata，摘要必然过期。
   */
  async function confirmRestore(accountID: string, payload: RestoreSourcePayload) {
    restoreSourceModalVisible.value = false

    opts.setBusy(true)
    try {
      if (payload.type === 'snapshot') {
        opts.logAction(`正在将快照 [${payload.sourceId}] 还原至账号 [${accountID}]...`)
        await restoreSnapshot(accountID, payload.sourceId)
        opts.showMessage(`快照 [${payload.sourceId}] 已成功还原至账号 [${accountID}]！`)
        opts.logAction(`账号 [${accountID}] 快照 [${payload.sourceId}] 还原成功`)
      } else if (payload.type === 'other') {
        opts.logAction(`正在将账号 [${payload.sourceId}] 的配置克隆还原至账号 [${accountID}]...`)
        // 签名顺序：RestoreUser(目标账号, 来源账号)——目标是被覆盖的一方
        await restoreUser(accountID, payload.sourceId)
        opts.showMessage(`已成功将账号 [${payload.sourceId}] 的配置克隆还原至账号 [${accountID}]！`)
        opts.logAction(`账号 [${accountID}] 跨账号克隆还原成功 (源: ${payload.sourceId})`)
      } else {
        opts.logAction(`正在将账号 [${accountID}] 本地备份还原...`)
        await restoreUser(accountID, accountID)
        opts.showMessage(`账号 [${accountID}] 配置已还原，原配置已备份至 restore_originals。`)
        opts.logAction(`用户 [${accountID}] 配置恢复成功`)
      }

      await checkRollbackStatus(accountID)
      await opts.reloadAccountDetail(accountID)
    } catch (err) {
      opts.showMessage(`还原配置失败: ${(err as Error)?.message || err}`, true)
    } finally {
      opts.setBusy(false)
    }
  }

  /**
   * 存量收敛：检测到历史版本超过上限就弹一次确认。
   *
   * 上限是这一版才加的，用户机器上可能已经堆了几十个历史。直接删是破坏性的，
   * 所以先让他看清「保留几个、删几个、删哪些」，确认后才动。
   *
   * 同一个账号在一个会话里只问一次——用户点了取消就是「知道了，别烦我」。
   */
  async function offerPrune(accountID: string, limit: number) {
    if (!accountID || accountID === 'game_cfg' || limit <= 0) return
    if (askedPrune.has(accountID)) return

    let doomed: string[]
    try {
      doomed = await previewBackupPrune(accountID)
    } catch {
      return
    }
    if (doomed.length === 0) return

    askedPrune.add(accountID)
    opts.confirm({
      title: '历史版本超出上限',
      message: `账号 [${accountID}] 有 ${doomed.length + limit} 个历史版本，超过上限 ${limit} 个。`,
      detail: `将保留最近 ${limit} 个，删除最旧的 ${doomed.length} 个：${doomed.join('、')}`,
      icon: 'warning',
      confirmText: '确认清理',
      isDanger: true,
      onConfirm: async () => {
        opts.setBusy(true)
        try {
          const dropped = await applyBackupPrune(accountID)
          await loadBackedUpAccounts()
          opts.showMessage(`已清理 ${dropped.length} 个旧版本`)
          opts.logAction(`账号 [${accountID}] 历史版本已收敛，清理: ${dropped.join(', ') || '(无)'}`)
        } catch (err) {
          opts.showMessage(`清理历史版本失败: ${(err as Error)?.message || err}`, true)
        } finally {
          opts.setBusy(false)
        }
      },
    })
  }

  /** 撤销上一次还原。危险操作，先弹确认框。 */
  function rollbackUser(accountID: string) {
    if (!accountID || accountID === 'game_cfg') return

    opts.confirm({
      title: '撤销还原 (回退原配置)',
      message: `确认将账号 [${accountID}] 回退至上次还原操作之前的原始配置吗？`,
      detail: '系统将从 restore_originals/user_730 提取历史备份并覆盖当前配置。',
      icon: 'restore',
      confirmText: '确认撤销',
      isDanger: true,
      onConfirm: async () => {
        opts.setBusy(true)
        try {
          opts.logAction(`正在撤销账号 [${accountID}] 的还原操作...`)
          await rollbackUserRestore(accountID)
          opts.showMessage(`账号 [${accountID}] 已成功回退至还原前的原始配置！`)
          opts.logAction(`账号 [${accountID}] 撤销还原成功`)
          await checkRollbackStatus(accountID)
          await opts.reloadAccountDetail(accountID)
        } catch (err) {
          opts.showMessage(`撤销还原失败: ${(err as Error)?.message || err}`, true)
        } finally {
          opts.setBusy(false)
        }
      },
    })
  }

  return {
    backedUpAccounts,
    remoteBackups,
    canRollbackUser,
    canRollbackGameCFG,
    restoreSourceModalVisible,
    loadBackedUpAccounts,
    checkRollbackStatus,
    backup,
    openRestoreSource,
    closeRestoreSource,
    confirmRestore,
    rollbackUser,
    offerPrune,
  }
}
