import { ref, onUnmounted } from 'vue'

/**
 * 工作台顶部那条提示条。
 *
 * 从 LocalBackupView 里搬出来的：原来它和 66 个别的 ref 挤在同一个作用域，
 * 谁也说不清哪几个是它自己的。现在这条提示只属于这里。
 *
 * 两种消息互斥——出错了就不该同时挂着上一条成功提示。
 */
export function useToast() {
  const errorMsg = ref('')
  const successMsg = ref('')

  let timer: ReturnType<typeof setTimeout> | null = null

  function clearTimer() {
    if (timer !== null) {
      clearTimeout(timer)
      timer = null
    }
  }

  /**
   * 弹一条提示。成功提示 4 秒后自动消失；错误提示常驻，等下一次提示把它顶掉。
   */
  function showMessage(msg: string, isError = false) {
    clearTimer()
    if (isError) {
      errorMsg.value = msg
      successMsg.value = ''
      return
    }
    successMsg.value = msg
    errorMsg.value = ''
    timer = setTimeout(() => {
      // 期间又弹过别的提示的话，不要把它清掉。
      if (successMsg.value === msg) {
        successMsg.value = ''
      }
      timer = null
    }, 4000)
  }

  function clearToast() {
    clearTimer()
    errorMsg.value = ''
    successMsg.value = ''
  }

  // 视图卸载时把定时器收掉，避免它回调到一个已经不在的组件。
  onUnmounted(clearTimer)

  return { errorMsg, successMsg, showMessage, clearToast }
}
