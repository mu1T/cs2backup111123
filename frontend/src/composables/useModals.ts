import { ref } from 'vue'
import type { UserCardInfo } from '../types'

export interface UseModalsOptions {
  showMessage: (msg: string, isError?: boolean) => void
  logAction: (msg: string) => void
  /** 当前选中的账号，画质弹窗没显式传人时用它。 */
  selectedUser: () => UserCardInfo | null
  /** 当前选中的账号 ID，画质保存后要重读摘要。 */
  selectedAccountID: () => string
  /** 重读某个账号的画质摘要。 */
  reloadVideoSummary: (accountID: string) => Promise<void>
}

/** 文本编辑器要打开的文件属于哪个作用域。 */
export type EditorScope = 'user' | 'game'

/**
 * 两个弹窗的开关状态：画质设置弹窗、内置文本编辑器。
 *
 * 从 LocalBackupView 里搬出来的最后一块。它们没有业务逻辑，只有「开着还是
 * 关着」和「打开时带什么参数」，但混在四十多个业务状态里时，没人能一眼看出
 * 界面上同时可能开着几个弹层。
 */
export function useModals(opts: UseModalsOptions) {
  const videoVisible = ref(false)
  const videoTargetAccountId = ref('')
  const videoTargetUsername = ref('')

  const textEditorVisible = ref(false)
  const textEditorFile = ref('')
  const textEditorScope = ref<EditorScope>('user')

  /**
   * 打开画质设置弹窗。
   *
   * 没传人就用当前选中账号；两个都没有就静默返回——调用方自己决定要不要
   * 提示（`openVideoGuarded` 会提示）。
   */
  function openVideo(u?: UserCardInfo) {
    const target = u ?? opts.selectedUser()
    if (!target) return
    videoTargetAccountId.value = target.account_id
    videoTargetUsername.value = target.username
    videoVisible.value = true
  }

  /** 带「请先选择账号」提示的入口，供空状态下的按钮用。 */
  function openVideoGuarded() {
    const target = opts.selectedUser()
    if (!target) {
      opts.showMessage('请先选择一个 Steam 账号', true)
      return
    }
    openVideo(target)
  }

  function closeVideo() {
    videoVisible.value = false
  }

  /** 画质弹窗保存完：提示 + 重读当前账号的画质摘要。 */
  async function handleVideoSaved() {
    opts.showMessage('CS2 视频配置已成功保存')
    const aid = opts.selectedAccountID()
    if (aid && aid !== 'game_cfg') {
      await opts.reloadVideoSummary(aid)
    }
  }

  /**
   * 打开文本编辑器。
   *
   * 第一个参数是 `string | Event`，因为模板里既会 `@click="openTextEditor('a.cfg')"`
   * 也会 `@click="openTextEditor"`（后者把事件对象传进来）。这里统一忽略非字符串。
   */
  function openTextEditor(file?: string | Event, scope: EditorScope = 'user') {
    textEditorFile.value = typeof file === 'string' ? file : ''
    textEditorScope.value = scope
    textEditorVisible.value = true
  }

  /**
   * 从差异弹窗点某个文件打开编辑器。
   *
   * 作用域靠前缀判断：`game_cfg` 账号，或文件名以 `game` / `csgo` 开头的，
   * 都属于游戏全局 CFG，而不是某个账号的 userdata。
   */
  function openEditorFromDiff(payload: { accountId: string; file: string }) {
    const isGame =
      payload.accountId === 'game_cfg' ||
      payload.file.startsWith('game') ||
      payload.file.startsWith('csgo')
    openTextEditor(payload.file, isGame ? 'game' : 'user')
  }

  function closeTextEditor() {
    textEditorVisible.value = false
  }

  function handleTextEditorSaved(filename: string) {
    opts.showMessage(`配置文件 [${filename}] 保存成功！`)
    opts.logAction(`已在内置编辑器中修改并保存配置文件: ${filename}`)
  }

  return {
    videoVisible,
    videoTargetAccountId,
    videoTargetUsername,
    textEditorVisible,
    textEditorFile,
    textEditorScope,
    openVideo,
    openVideoGuarded,
    closeVideo,
    handleVideoSaved,
    openTextEditor,
    openEditorFromDiff,
    closeTextEditor,
    handleTextEditorSaved,
  }
}
