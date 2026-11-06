import { ref } from 'vue'

export type ConfirmIcon = 'info' | 'warning' | 'danger' | 'restore' | 'switch'

/** ConfirmModal 需要的完整状态。 */
export interface ConfirmState {
  visible: boolean
  title: string
  message: string
  detail: string
  icon: ConfirmIcon
  confirmText: string
  cancelText: string
  isDanger: boolean
  showCancel: boolean
  prompt: boolean
  promptPlaceholder: string
  promptDefault: string
  onConfirm?: (val?: string) => void
  onCancel?: () => void
}

/** showConfirm 的入参：除 message 外都可省，省掉的有默认值。 */
export type ConfirmOptions = Partial<ConfirmState> & {
  message: string
  onConfirm?: (val?: string) => void
}

function blankState(): ConfirmState {
  return {
    visible: false,
    title: '',
    message: '',
    detail: '',
    icon: 'info',
    confirmText: '确定',
    cancelText: '取消',
    isDanger: false,
    showCancel: true,
    prompt: false,
    promptPlaceholder: '',
    promptDefault: '',
  }
}

/**
 * 一个确认框，全视图共用一个实例。
 *
 * 从 LocalBackupView 里搬出来的：那段状态有 14 个字段，全和别的 ref 混在
 * 一起，而调用方只关心「弹一下、回调跑不跑」。
 *
 * 回调存在状态里而不是返回值里，是因为调用方几乎都是
 * `await` 之后什么都不做、只等用户点确认。
 */
export function useConfirm() {
  const confirmState = ref<ConfirmState>(blankState())

  function showConfirm(opts: ConfirmOptions) {
    confirmState.value = {
      visible: true,
      title: opts.title || '确认操作',
      message: opts.message,
      detail: opts.detail || '',
      icon: opts.icon || 'info',
      confirmText: opts.confirmText || '确定',
      cancelText: opts.cancelText || '取消',
      isDanger: opts.isDanger ?? false,
      showCancel: opts.showCancel ?? true,
      prompt: opts.prompt ?? false,
      promptPlaceholder: opts.promptPlaceholder || '',
      promptDefault: opts.promptDefault || '',
      onConfirm: opts.onConfirm,
      onCancel: opts.onCancel,
    }
  }

  /** 用户点了确认。先关框再回调，避免回调里又开一个框时被这次关闭盖掉。 */
  function onConfirmModalConfirm(val?: string) {
    const cb = confirmState.value.onConfirm
    confirmState.value.visible = false
    if (cb) cb(val)
  }

  /** 用户点了取消（或按 Esc）。 */
  function onConfirmModalCancel() {
    const cb = confirmState.value.onCancel
    confirmState.value.visible = false
    if (cb) cb()
  }

  return { confirmState, showConfirm, onConfirmModalConfirm, onConfirmModalCancel }
}
