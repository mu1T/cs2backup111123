import { watch, onBeforeUnmount, type Ref } from 'vue'

const FOCUSABLE_SELECTOR = [
  'a[href]',
  'button:not([disabled])',
  'textarea:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',')

/**
 * 对话框焦点管理：
 * 1. 打开时把焦点移入对话框（优先 [data-autofocus]，其次首个可聚焦元素）
 * 2. Tab / Shift+Tab 在对话框内循环，不会跑到背景内容
 * 3. 关闭时把焦点归还给打开前的元素
 */
export function useFocusTrap(visible: Ref<boolean>, container: Ref<HTMLElement | null>) {
  let lastActive: HTMLElement | null = null

  const getFocusable = (): HTMLElement[] => {
    const root = container.value
    if (!root) return []
    return Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(
      (el) => el.offsetParent !== null || el === document.activeElement
    )
  }

  const onKeydown = (e: KeyboardEvent) => {
    if (e.key !== 'Tab') return
    const root = container.value
    if (!root) return
    const nodes = getFocusable()
    if (nodes.length === 0) {
      e.preventDefault()
      root.focus()
      return
    }
    const first = nodes[0]
    const last = nodes[nodes.length - 1]
    const active = document.activeElement as HTMLElement | null
    const inside = active ? root.contains(active) : false

    if (e.shiftKey) {
      if (!inside || active === first) {
        e.preventDefault()
        last.focus()
      }
    } else if (!inside || active === last) {
      e.preventDefault()
      first.focus()
    }
  }

  watch(visible, (isVisible) => {
    if (isVisible) {
      lastActive = (document.activeElement as HTMLElement) || null
      document.addEventListener('keydown', onKeydown, true)
      requestAnimationFrame(() => {
        const root = container.value
        if (!root) return
        const preferred = root.querySelector<HTMLElement>('[data-autofocus]')
        const nodes = getFocusable()
        ;(preferred || nodes[0] || root).focus?.()
      })
    } else {
      document.removeEventListener('keydown', onKeydown, true)
      if (lastActive && typeof lastActive.focus === 'function') {
        lastActive.focus()
      }
      lastActive = null
    }
  })

  onBeforeUnmount(() => {
    document.removeEventListener('keydown', onKeydown, true)
  })
}
