import { ref } from 'vue'
import { describeCrosshair } from '../utils/crosshairGeometry'
import { getCrosshairState } from '../utils/wails'
import type { CrosshairSaveResult } from '../types'

export interface UseCrosshairOptions {
  /** 弹一条提示条。 */
  showMessage: (msg: string, isError?: boolean) => void
  /** 往日志面板写一行。 */
  logAction: (msg: string) => void
}

/**
 * 准星摘要条与准星编辑器弹窗。
 *
 * 准星是全局 `game/csgo/cfg/autoexec.cfg` 里的设置，与选中账号无关，
 * 所以加载时机跟画质摘要一致：进页面一次、保存后一次、换 CS2 目录后一次。
 *
 * 从 LocalBackupView 里搬出来的：它原来和另外六十多个 ref 挤在同一个作用域，
 * 「准星摘要什么时候刷新」这件事要翻遍整个文件才能确认。
 */
export function useCrosshair(opts: UseCrosshairOptions) {
  const modalVisible = ref(false)
  const summary = ref('')
  const shareCode = ref('')
  const toggle = ref('')

  /** 重读 autoexec.cfg 里的准星设置。读不到不算错——没配 Steam 路径时本来就读不到。 */
  async function load() {
    try {
      const state = await getCrosshairState()
      summary.value = describeCrosshair(state.parsed.crosshair)
      shareCode.value = state.share_code
      toggle.value = state.parsed.display_toggle
    } catch (err) {
      summary.value = ''
      shareCode.value = ''
      toggle.value = ''
      console.warn('Failed to load crosshair state:', err)
    }
  }

  function open() {
    modalVisible.value = true
  }

  function close() {
    modalVisible.value = false
  }

  /**
   * 编辑器保存完之后的反馈。
   *
   * 三个清单都可能为空。Go 侧已经保证是空数组而不是 null，但这里仍然兜一层：
   * 历史上就是因为一次 `null.length` 抛了 TypeError，Vue 中止 patch，
   * 整个界面卡在「读取中…」，看起来像「点了没反应」。
   */
  async function handleSaved(result: CrosshairSaveResult) {
    await load()

    const replaced = result.report?.replaced_keys?.length ?? 0
    const appended = result.report?.appended_keys?.length ?? 0
    const commented = result.report?.commented_keys?.length ?? 0

    const parts: string[] = [result.created ? '已新建 autoexec.cfg' : '已写入 autoexec.cfg']
    if (replaced) parts.push(`覆盖 ${replaced} 项`)
    if (appended) parts.push(`新增 ${appended} 项`)
    if (commented) parts.push(`注释旧键 ${commented} 项`)

    opts.showMessage(`准星设置已生效：${parts.join('，')}。重启 CS2 后加载。`)
    opts.logAction('准星设置已写入 autoexec.cfg')
  }

  return { modalVisible, summary, shareCode, toggle, load, open, close, handleSaved }
}
