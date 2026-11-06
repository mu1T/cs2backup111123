import { ref } from 'vue'
import { getLaunchOptions, setLaunchOptions, isSteamRunning, shutdownSteam } from '../utils/wails'
import { hasFlag, addFlag, toggleFlag } from '../utils/launchOptions'

export interface UseLaunchOptionsOptions {
  showMessage: (msg: string, isError?: boolean) => void
  logAction: (msg: string) => void
  setBusy: (busy: boolean) => void
  /** 复制到剪贴板，成功后回调。视图那边有一层权限失败降级。 */
  copy: (text: string, onSuccess: () => void) => void
  /** 为了写启动项而退出 Steam 之后，把进程状态标记成未运行。 */
  markSteamStopped: () => void
}

/**
 * CS2 启动项：读取、快捷开关、编辑保存。
 *
 * 纯文本处理（词边界匹配、`-freq` 互斥替换）在 utils/launchOptions.ts，
 * 这里只管状态与 IO。
 *
 * 保存流程有一个分支：Steam 正在跑的时候写启动项可能不生效（它会在退出时
 * 把 localconfig 写回去），所以要问用户「要不要先退出 Steam」。
 */
export function useLaunchOptions(opts: UseLaunchOptionsOptions) {
  const current = ref('')
  const loading = ref(false)
  const editVisible = ref(false)
  const editInput = ref('')
  /** 保存时发现 Steam 在跑，弹「需要先退出」的确认框。 */
  const steamPromptVisible = ref(false)
  const copySuccess = ref(false)

  async function load(accountID: string) {
    if (!accountID || accountID === 'game_cfg') {
      current.value = ''
      return
    }
    loading.value = true
    try {
      current.value = await getLaunchOptions(accountID)
    } catch (err) {
      console.warn(`Failed to load launch options for ${accountID}:`, err)
      current.value = ''
    } finally {
      loading.value = false
    }
  }

  function copyCurrent() {
    if (!current.value) return
    opts.copy(current.value, () => {
      copySuccess.value = true
      setTimeout(() => {
        copySuccess.value = false
      }, 2000)
      opts.logAction(`已复制 CS2 启动项: ${current.value}`)
      opts.showMessage('CS2 启动项已成功复制至剪贴板')
    })
  }

  function openEdit() {
    editInput.value = current.value
    editVisible.value = true
  }

  function closeEdit() {
    editVisible.value = false
  }

  function isActive(preset: string) {
    return hasFlag(current.value, preset)
  }

  function isActiveInModal(preset: string) {
    return hasFlag(editInput.value, preset)
  }

  /** 编辑弹窗里的「追加」按钮。 */
  function addPreset(preset: string) {
    editInput.value = addFlag(editInput.value, preset)
  }

  /** 编辑弹窗里点某个快捷参数：有就去掉，没有就加上。 */
  function toggleInModal(preset: string) {
    editInput.value = toggleFlag(editInput.value, preset).next
  }

  /** 摘要条上直接点某个快捷参数：算好新值就立刻写回后端。 */
  async function toggle(accountID: string, preset: string) {
    if (!accountID || accountID === 'game_cfg') return

    const { next, wasActive } = toggleFlag(current.value, preset)
    opts.setBusy(true)
    opts.logAction(
      wasActive
        ? `正在从账号 [${accountID}] 启动项中移除 [${preset}]...`
        : `正在向账号 [${accountID}] 启动项中追加 [${preset}]...`,
    )
    try {
      await setLaunchOptions(accountID, next)
      current.value = next
      opts.showMessage(wasActive ? `已从启动项移除: ${preset}` : `已追加至启动项: ${preset}`)
      opts.logAction(`账号 [${accountID}] CS2 启动项已更新为: ${next || '(空)'}`)
    } catch (err) {
      const detail = (err as Error)?.message || err
      opts.showMessage(`更新启动项失败: ${detail}`, true)
      opts.logAction(`更新启动项失败: ${detail}`)
    } finally {
      opts.setBusy(false)
    }
  }

  /** 编辑弹窗点保存：先看 Steam 在不在跑，在跑就问一句。 */
  async function save(accountID: string) {
    if (!accountID || accountID === 'game_cfg') return

    try {
      if (await isSteamRunning()) {
        steamPromptVisible.value = true
        return
      }
    } catch (err) {
      // 查不到就按「没在跑」处理：多问一句比写不进去强。
      console.warn('Steam running check failed:', err)
    }

    await executeSave(accountID, false)
  }

  /** 用户在「需要先退出 Steam」的确认框里做了选择。 */
  async function confirmSave(accountID: string, shouldExitSteam: boolean) {
    steamPromptVisible.value = false
    if (shouldExitSteam) {
      await executeSave(accountID, true)
    }
  }

  async function executeSave(accountID: string, exitSteam: boolean) {
    if (!accountID || accountID === 'game_cfg') return

    opts.setBusy(true)
    try {
      if (exitSteam) {
        opts.logAction('正在退出 Steam 以便使启动项写入生效...')
        await shutdownSteam()
        opts.markSteamStopped()
      }

      const value = editInput.value.trim()
      opts.logAction(`正在更新账号 [${accountID}] 的 CS2 启动项...`)
      await setLaunchOptions(accountID, value)
      current.value = value
      editVisible.value = false
      opts.showMessage('CS2 启动项保存成功！')
      opts.logAction(`账号 [${accountID}] CS2 启动项更新成功: ${value}`)
    } catch (err) {
      const detail = (err as Error)?.message || err
      opts.showMessage(`保存启动项失败: ${detail}`, true)
      opts.logAction(`保存启动项失败: ${detail}`)
    } finally {
      opts.setBusy(false)
    }
  }

  return {
    current,
    loading,
    editVisible,
    editInput,
    steamPromptVisible,
    copySuccess,
    load,
    copyCurrent,
    openEdit,
    closeEdit,
    isActive,
    isActiveInModal,
    addPreset,
    toggleInModal,
    toggle,
    save,
    confirmSave,
    executeSave,
  }
}
