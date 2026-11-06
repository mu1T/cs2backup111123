import { computed, ref } from 'vue'
import {
  getGameCFGFiles,
  backupGameCFG,
  restoreGameCFG,
  restoreGameCFGVersion,
  rollbackGameCFG,
  selectCFGFiles,
} from '../utils/wails'
import type { GameCFGFile } from '../types'

export interface UseGameCfgOptions {
  showMessage: (msg: string, isError?: boolean) => void
  logAction: (msg: string) => void
  /** 让「备份 / 还原」按钮转圈。视图把它接到全局 actionLoading 上。 */
  setBusy: (busy: boolean) => void
  /** 还原 / 撤销是不可逆操作，先问。 */
  confirm: (opts: {
    title: string
    message: string
    detail?: string
    icon?: 'info' | 'warning' | 'danger' | 'restore' | 'switch'
    confirmText?: string
    isDanger?: boolean
    onConfirm: () => Promise<void> | void
  }) => void
  /** 生效的 game/csgo/cfg 目录；勾选备份要用它拼完整路径。 */
  cfgDir: () => string
  /** 还原 / 撤销之后重查「能不能撤销」。 */
  afterRestore: () => Promise<void>
}

/**
 * 共享游戏 CFG 目录（`game/csgo/cfg`）的扫描结果与勾选状态。
 *
 * 清单由后端扫描得出，不再维护硬编码列表——平台（完美 / 5E）会自己往这个
 * 目录里丢文件，硬编码的清单永远跟不上。
 */

/** 快捷编辑入口的优先顺序：常见脚本排在前面，缺失的用玩家自定义补齐。 */
export const QUICK_CFG_PRIORITY = ['autoexec.cfg', 'practice.cfg', 'nadepractice.cfg', 'bot.cfg', 'server.cfg']

/** 快捷入口最多显示几个。 */
const QUICK_CFG_LIMIT = 4

export function useGameCfg(opts: UseGameCfgOptions) {
  const files = ref<GameCFGFile[]>([])
  const selected = ref<string[]>([])
  const scanning = ref(false)
  /** 首次扫描是否已经做过——决定「默认勾选玩家自定义」还是「保留原勾选」。 */
  const initialized = ref(false)

  const quickFiles = computed<GameCFGFile[]>(() => {
    const byName = new Map(files.value.map((f) => [f.name, f]))
    const picked: GameCFGFile[] = []

    for (const name of QUICK_CFG_PRIORITY) {
      const hit = byName.get(name)
      if (hit) picked.push(hit)
    }

    if (picked.length < QUICK_CFG_LIMIT) {
      files.value
        .filter((f) => f.source === 'player' && !picked.some((p) => p.name === f.name))
        .slice(0, QUICK_CFG_LIMIT - picked.length)
        .forEach((f) => picked.push(f))
    }
    return picked
  })

  /**
   * 重新扫描目录。
   *
   * 首次扫描默认只勾玩家自定义的文件——平台与官方文件（5E / 完美 / Valve）
   * 不该被「备份共享 CFG」默认带上。之后的扫描保留原勾选，但丢掉已经消失的文件。
   */
  async function load() {
    scanning.value = true
    try {
      const list = await getGameCFGFiles()
      files.value = list
      const names = new Set(list.map((f) => f.name))

      if (!initialized.value) {
        selected.value = list.filter((f) => f.source === 'player').map((f) => f.name)
        initialized.value = true
      } else {
        selected.value = selected.value.filter((n) => names.has(n))
      }

      opts.logAction(`已扫描 CS2 CFG 目录，共 ${list.length} 个配置文件`)
    } catch (err) {
      files.value = []
      opts.showMessage(`扫描 CFG 目录失败: ${(err as Error)?.message || err}`, true)
    } finally {
      scanning.value = false
    }
  }

  function selectAll(all: boolean) {
    selected.value = all ? files.value.map((f) => f.name) : []
  }

  function selectOnlyCustom() {
    selected.value = files.value.filter((f) => f.source === 'player').map((f) => f.name)
  }

  function toggle(name: string) {
    const idx = selected.value.indexOf(name)
    if (idx >= 0) {
      selected.value.splice(idx, 1)
    } else {
      selected.value.push(name)
    }
  }

  // ---- 写入侧：备份 / 还原 / 撤销 ----

  /**
   * 备份。
   *
   * mode 决定后端怎么筛文件：
   * - `all`      整个目录
   * - `custom`   只挑玩家自定义（剔除 Valve 官方与平台文件）
   * - `selected` 按传入的完整路径挑
   */
  async function backup(mode: 'all' | 'custom' | 'selected', paths: string[] = []) {
    opts.setBusy(true)
    try {
      const count = await backupGameCFG(mode, paths)
      // 备份会产生新版本，「还原」的版本下拉要跟着变。
      await opts.afterRestore()
      return count
    } catch (err) {
      opts.showMessage(`备份 CS2 CFG 失败: ${(err as Error)?.message || err}`, true)
      return null
    } finally {
      opts.setBusy(false)
    }
  }

  /** 全量备份。 */
  async function backupAll() {
    opts.logAction('正在执行游戏公共通用 CFG (Game CFG) 全量备份...')
    const count = await backup('all')
    if (count === null) return
    opts.showMessage(`成功全量备份 ${count} 个 CS2 CFG 配置文件！`)
    opts.logAction(`全量备份完成，共备份 ${count} 个配置文件至 cs2_cfg/`)
  }

  /** 只备份玩家自定义文件（剔除 Valve 官方默认）。 */
  async function backupCustom() {
    opts.logAction('正在筛选备份玩家自定义 CFG（已剔除 Valve 官方默认文件）...')
    const count = await backup('custom')
    if (count === null) return
    opts.showMessage(`成功备份 ${count} 个玩家自定义 CS2 配置文件！`)
    opts.logAction(`玩家 CFG 过滤备份完成，共写入 ${count} 个自定义配置`)
  }

  /** 弹系统文件选择器，挑几个备份。用户取消就什么都不做。 */
  async function backupPicked() {
    let paths: string[]
    try {
      paths = await selectCFGFiles()
    } catch (err) {
      opts.showMessage(`选择性备份失败: ${(err as Error)?.message || err}`, true)
      return
    }
    if (!paths || paths.length === 0) return

    opts.logAction(`已挑选 ${paths.length} 个配置文件，正在执行选择性备份...`)
    const count = await backup('selected', paths)
    if (count === null) return
    opts.showMessage(`成功备份选定的 ${count} 个配置文件！`)
    opts.logAction(`选择性备份完成，共备份 ${count} 个配置文件`)
  }

  /**
   * 备份当前勾选的文件。
   *
   * 勾选的是文件名，后端要完整路径——用生效的 cfg 目录拼出来。
   * 目录拿不到就退回只传文件名（后端会按相对路径解析）。
   */
  async function backupChecked() {
    const names = selected.value
    if (names.length === 0) {
      opts.showMessage('请先勾选需要备份的配置文件', true)
      return
    }

    const dir = (opts.cfgDir() || '').replace(/\\/g, '/')
    const paths = names.map((n) => (dir ? `${dir}/${n}` : n))

    opts.logAction(`正在备份勾选的 ${names.length} 个 CS2 配置文件...`)
    const count = await backup('selected', paths)
    if (count === null) return
    opts.showMessage(`成功备份选中的 ${count} 个 CS2 游戏配置文件！`)
    opts.logAction(`勾选备份完成，共备份 ${count} 个配置文件至 cs2_cfg/`)
  }

  /**
   * 还原通用 CFG。还原前系统会自动留一份，所以这里只做一次确认。
   *
   * `versionID` 为空表示还原「最新一次备份」（走镜像）；给了就还原到那个历史版本。
   */
  function restore(versionID?: string) {
    const target = versionID ? `历史版本 [${versionID}]` : '最新一次备份'
    opts.confirm({
      title: '还原通用 CFG',
      message: `确认将 ${target} 还原到 game/csgo/cfg 吗？`,
      detail: '系统将在还原前自动将当前配置备份至 restore_originals/cs2_cfg_global，支持随时回退。',
      icon: 'restore',
      confirmText: '确认还原',
      onConfirm: async () => {
        opts.setBusy(true)
        try {
          opts.logAction(`正在还原 CS2 游戏通用 CFG（${target}）...`)
          const count = versionID
            ? await restoreGameCFGVersion(versionID)
            : await restoreGameCFG()
          opts.showMessage(`成功还原 ${count} 个 CS2 游戏配置文件！`)
          opts.logAction(`游戏通用 CFG 还原成功（${target}），共还原 ${count} 个文件 (原配置已备份)`)
          await opts.afterRestore()
        } catch (err) {
          opts.showMessage(`还原 CS2 CFG 失败: ${(err as Error)?.message || err}`, true)
        } finally {
          opts.setBusy(false)
        }
      },
    })
  }

  /** 撤销上一次通用 CFG 还原。 */
  function rollback() {
    opts.confirm({
      title: '撤销通用 CFG 还原',
      message: '确认将游戏通用 CFG 回退至上次还原操作之前的配置吗？',
      detail: '系统将从 restore_originals/cs2_cfg_global 恢复所有通用配置文件。',
      icon: 'restore',
      confirmText: '确认撤销',
      isDanger: true,
      onConfirm: async () => {
        opts.setBusy(true)
        try {
          opts.logAction('正在撤销游戏通用 CFG 的还原操作...')
          const count = await rollbackGameCFG()
          opts.showMessage(`已成功回退 ${count} 个通用配置文件至还原前的状态！`)
          opts.logAction(`通用 CFG 撤销还原成功，已恢复 ${count} 个文件`)
          await opts.afterRestore()
        } catch (err) {
          opts.showMessage(`撤销还原通用 CFG 失败: ${(err as Error)?.message || err}`, true)
        } finally {
          opts.setBusy(false)
        }
      },
    })
  }

  return {
    files,
    selected,
    scanning,
    initialized,
    quickFiles,
    load,
    selectAll,
    selectOnlyCustom,
    toggle,
    backupAll,
    backupCustom,
    backupPicked,
    backupChecked,
    restore,
    rollback,
  }
}
