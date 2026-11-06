import { ref } from 'vue'
import { isSteamRunning, startSteam, shutdownSteam, switchSteamAccount } from '../utils/wails'
import type { UserCardInfo } from '../types'

export interface UseSteamProcessOptions {
  showMessage: (msg: string, isError?: boolean) => void
  logAction: (msg: string) => void
  /** 切号成功后账号列表要重读（remember_password 等状态会变）。 */
  reloadUsers: () => Promise<void>
}

/**
 * Steam 客户端进程与账号切换。
 *
 * 从 LocalBackupView 里搬出来的：六个状态标志 + 五个动作，和另外几十个 ref
 * 混在一起时，「切号到底刷新了哪些东西」要靠通读整个文件才能回答。
 */
export function useSteamProcess(opts: UseSteamProcessOptions) {
  const isSteamActive = ref(false)
  const steamCheckLoading = ref(false)
  const steamActionLoading = ref(false)

  const switchModalVisible = ref(false)
  const switchTargetUser = ref<UserCardInfo | null>(null)
  const switchActionLoading = ref(false)

  /** 查一下 Steam 是否在跑。查不到就保持上一次的判断，不把界面闪成「未运行」。 */
  async function checkSteamStatus() {
    steamCheckLoading.value = true
    try {
      isSteamActive.value = await isSteamRunning()
    } catch (err) {
      console.warn('Failed to check steam status:', err)
    } finally {
      steamCheckLoading.value = false
    }
  }

  async function shutdown() {
    steamActionLoading.value = true
    opts.logAction('正在退出 Steam 客户端...')
    try {
      await shutdownSteam()
      isSteamActive.value = false
      opts.showMessage('Steam 客户端已安全退出')
      opts.logAction('Steam 客户端已成功退出')
    } catch (err) {
      const msg = (err as Error)?.message || err
      opts.showMessage(`退出 Steam 失败: ${msg}`, true)
      opts.logAction(`退出 Steam 失败: ${msg}`)
    } finally {
      steamActionLoading.value = false
    }
  }

  /**
   * 启动 Steam。
   *
   * @param launchGame true 表示连 CS2 一起拉起来（-applaunch 730），
   *   false 只后台启动客户端——切号之后走的是后者。
   */
  async function start(launchGame = true) {
    steamActionLoading.value = true
    opts.logAction(launchGame ? '正在启动 Steam 并运行 CS2...' : '正在后台启动 Steam 客户端...')
    try {
      await startSteam(launchGame)
      isSteamActive.value = true
      opts.showMessage(launchGame ? 'Steam 已启动并拉起 CS2！' : 'Steam 客户端已启动！')
      opts.logAction(launchGame ? 'Steam 及 CS2 启动成功' : 'Steam 客户端启动成功')
    } catch (err) {
      const msg = (err as Error)?.message || err
      opts.showMessage(`启动 Steam 失败: ${msg}`, true)
      opts.logAction(`启动 Steam 失败: ${msg}`)
    } finally {
      steamActionLoading.value = false
    }
  }

  /** 点账号卡上的「切换」：先开确认框，不直接切。 */
  function requestSwitch(u: UserCardInfo) {
    switchTargetUser.value = u
    switchModalVisible.value = true
  }

  /** 确认框里点了确认。切号会让 Steam 重启，所以成功后要重查进程状态。 */
  async function confirmSwitch() {
    const u = switchTargetUser.value
    if (!u) return

    const targetAcc = u.account_name || u.account_id
    switchActionLoading.value = true
    try {
      opts.logAction(`正在切换 Steam 账号至 [${u.username}] (${targetAcc})...`)
      await switchSteamAccount(targetAcc)
      switchModalVisible.value = false
      opts.showMessage('切换成功，Steam 正在重新启动...')
      opts.logAction(`已成功切换 Steam 账号至 [${u.username}] (${targetAcc})，Steam 正在重新启动`)
      await opts.reloadUsers()
      await checkSteamStatus()
    } catch (err) {
      const msg = (err as Error)?.message || err
      opts.showMessage(`切换 Steam 账号失败: ${msg}`, true)
      opts.logAction(`切换 Steam 账号失败: ${msg}`)
    } finally {
      switchActionLoading.value = false
    }
  }

  function closeSwitchModal() {
    switchModalVisible.value = false
  }

  return {
    isSteamActive,
    steamCheckLoading,
    steamActionLoading,
    switchModalVisible,
    switchTargetUser,
    switchActionLoading,
    checkSteamStatus,
    shutdown,
    start,
    requestSwitch,
    confirmSwitch,
    closeSwitchModal,
  }
}
