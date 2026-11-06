import { ref } from 'vue'
import { getSteamPath, getBackupPath, getCS2Path, getCS2CFGPath, detectSteamPath, openPathInExplorer } from '../utils/wails'

export interface UsePathsOptions {
  showMessage: (msg: string, isError?: boolean) => void
  logAction: (msg: string) => void
  /**
   * 路径变了之后要重读的东西：账号列表、备份清单、cfg 列表、CS2 cfg 目录、准星摘要。
   *
   * 由视图注入而不是在这里直接调，是因为这几块分属不同的 module——
   * 让路径这块去 import 它们会把「谁依赖谁」重新搅成一团。
   */
  reloadAfterPathChange: () => Promise<void>
}

export interface PathPayload {
  steamPath: string
  backupPath: string
  cs2Path: string
}

/**
 * 三个路径：Steam 安装目录、备份仓库、手动指定的 CS2 配置目录。
 *
 * 从 LocalBackupView 里搬出来的。原来这些 ref 和四十多个别的 ref 挤在一起，
 * 「哪个函数会改路径」要翻遍全文件；而路径是几乎所有其它块的上游。
 */
export function usePaths(opts: UsePathsOptions) {
  const steamPathInput = ref('')
  const backupPathInput = ref('')
  /** 手动指定的 CS2 配置目录，空串表示自动探测。装在多份 CS2 时用它指定。 */
  const cs2PathInput = ref('')
  /** 实际生效的 game/csgo/cfg 目录，由后端算出来。 */
  const cs2CfgPath = ref('')

  const pathModalVisible = ref(false)
  const detecting = ref(false)

  async function loadSteamPath() {
    try {
      steamPathInput.value = await getSteamPath()
    } catch (err) {
      // 读不到就保持现状，不要清空——用户可能刚手填了一个还没保存。
      console.warn('Failed to load Steam path:', err)
    }
  }

  async function loadBackupPath() {
    try {
      backupPathInput.value = await getBackupPath()
    } catch (err) {
      console.warn('Failed to load Backup path:', err)
    }
  }

  /**
   * 重读生效的 CS2 cfg 目录。
   *
   * 失败时清空而不是留着旧值：旧值可能指向已经被删掉或换盘符的目录，
   * 留着它会让后续所有 cfg 操作打到一个不存在的地方。
   */
  async function loadCS2CFGPath() {
    try {
      cs2CfgPath.value = await getCS2CFGPath()
    } catch {
      cs2CfgPath.value = ''
    }
  }

  async function loadCS2PathOverride() {
    try {
      cs2PathInput.value = await getCS2Path()
    } catch {
      cs2PathInput.value = ''
    }
  }

  /** 打开路径设置弹窗前先拉一次手动 CS2 路径，避免界面显示旧值。 */
  async function openPathSettings() {
    await loadCS2PathOverride()
    pathModalVisible.value = true
  }

  /** 路径设置弹窗点了保存。 */
  async function handlePathSaved(payload: PathPayload) {
    steamPathInput.value = payload.steamPath
    backupPathInput.value = payload.backupPath
    cs2PathInput.value = payload.cs2Path

    opts.logAction(`已更新路径设置 (Steam: ${payload.steamPath})`)
    opts.showMessage(payload.cs2Path ? '路径设置已保存，CS2 目录已改为手动指定' : '路径设置已保存')

    await opts.reloadAfterPathChange()
  }

  /**
   * 空状态下的「自动侦测 Steam 目录」按钮。
   *
   * 探测会同时换掉 Steam 目录与 CS2 目录，所以收尾的刷新和「保存路径」走同一条路
   * ——以前这里只重读账号和 cfg 目录，准星摘要会留着旧目录那份。
   */
  async function handleAutoDetect() {
    detecting.value = true
    try {
      const detected = await detectSteamPath()
      steamPathInput.value = detected
      opts.logAction(`自动侦测 Steam 目录成功: ${detected}`)
      await opts.reloadAfterPathChange()
    } catch (err) {
      const msg = `侦测失败: ${(err as Error)?.message || err}`
      opts.showMessage(msg, true)
      opts.logAction(msg)
    } finally {
      detecting.value = false
    }
  }

  async function openSteamInExplorer() {
    const p = steamPathInput.value.trim()
    if (!p) {
      opts.showMessage('Steam 安装路径为空', true)
      return
    }
    try {
      await openPathInExplorer(p)
    } catch (err) {
      opts.showMessage(`打开路径失败: ${(err as Error)?.message || err}`, true)
    }
  }

  async function openBackupInExplorer() {
    const p = backupPathInput.value.trim()
    if (!p) {
      opts.showMessage('备份存储路径为空', true)
      return
    }
    try {
      await openPathInExplorer(p)
    } catch (err) {
      opts.showMessage(`打开路径失败: ${(err as Error)?.message || err}`, true)
    }
  }

  return {
    steamPathInput,
    backupPathInput,
    cs2PathInput,
    cs2CfgPath,
    pathModalVisible,
    detecting,
    loadSteamPath,
    loadBackupPath,
    loadCS2CFGPath,
    loadCS2PathOverride,
    openPathSettings,
    handlePathSaved,
    handleAutoDetect,
    openSteamInExplorer,
    openBackupInExplorer,
  }
}
