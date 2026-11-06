/**
 * 全局自绘 tooltip。
 *
 * 应用里到处用原生 `title`，浏览器给的是系统默认样式（灰底小字、延迟不可控、
 * 深色主题下很扎眼）。这里用事件委托接管所有 `[title]`：
 *
 * 1. 首次悬停时把 `title` 摘下来存进 `data-tip` 并移除 `title`，避免原生提示同时弹出；
 * 2. 用一个固定定位的浮层自绘，样式跟 WinUI 主题一致，支持换行、贴边翻转、箭头指向；
 * 3. 滚动 / 点击 / Esc / 窗口失焦时立即收起。
 *
 * 不依赖任何组件改造：Vue 之后新增的 `title` 也会自动生效。
 */

const SHOW_DELAY = 400
const HIDE_DELAY = 60
const GAP = 10
const EDGE = 8

let layer: HTMLDivElement | null = null
let active: HTMLElement | null = null
let showTimer = 0
let hideTimer = 0

function layerEl(): HTMLDivElement {
  if (layer) return layer
  layer = document.createElement('div')
  layer.className = 'app-tooltip'
  layer.setAttribute('role', 'tooltip')
  document.body.appendChild(layer)
  return layer
}

/**
 * 取提示文本。
 *
 * 原生 `title` 在悬停时会被浏览器抢着画出来，所以读到之后立刻搬去 `data-tip`。
 * 之后再悬停就直接读 `data-tip`，不会重复搬。
 */
function takeText(el: HTMLElement): string {
  const native = el.getAttribute('title')
  if (native !== null) {
    if (native.trim()) el.setAttribute('data-tip', native)
    else el.removeAttribute('data-tip')
    el.removeAttribute('title')
  }
  return el.getAttribute('data-tip') ?? ''
}

function place(el: HTMLElement, text: string) {
  const tip = layerEl()
  tip.textContent = text
  tip.style.left = '0px'
  tip.style.top = '0px'
  tip.classList.add('is-visible')

  const anchor = el.getBoundingClientRect()
  const tw = tip.offsetWidth
  const th = tip.offsetHeight

  // 水平居中于锚点，再夹进视口；箭头跟着偏移，始终指着锚点中心。
  const anchorCenter = anchor.left + anchor.width / 2
  let left = anchorCenter - tw / 2
  left = Math.max(EDGE, Math.min(left, window.innerWidth - tw - EDGE))

  let top = anchor.top - th - GAP
  let placement: 'top' | 'bottom' = 'top'
  if (top < EDGE) {
    top = anchor.bottom + GAP
    placement = 'bottom'
  }

  tip.style.left = `${Math.round(left)}px`
  tip.style.top = `${Math.round(top)}px`
  tip.dataset.placement = placement

  const arrowX = Math.max(8, Math.min(anchorCenter - left, tw - 8))
  tip.style.setProperty('--arrow-x', `${Math.round(arrowX)}px`)
}

function show(el: HTMLElement) {
  const text = takeText(el)
  if (!text.trim()) return
  active = el
  place(el, text)
}

function hide() {
  window.clearTimeout(showTimer)
  window.clearTimeout(hideTimer)
  active = null
  layer?.classList.remove('is-visible')
}

function scheduleShow(el: HTMLElement) {
  window.clearTimeout(showTimer)
  window.clearTimeout(hideTimer)
  showTimer = window.setTimeout(() => show(el), SHOW_DELAY)
}

function scheduleHide(el: HTMLElement) {
  window.clearTimeout(showTimer)
  window.clearTimeout(hideTimer)
  hideTimer = window.setTimeout(() => {
    if (active === el || active === null) hide()
  }, HIDE_DELAY)
}

/** 找到最近的、带提示文本的元素。 */
function findAnchor(target: EventTarget | null): HTMLElement | null {
  if (!(target instanceof Element)) return null
  const el = target.closest<HTMLElement>('[title], [data-tip]')
  if (!el) return null
  // 禁用的控件不会派发鼠标事件，这里再挡一次，免得在别的容器上误弹。
  if (el instanceof HTMLButtonElement && el.disabled) return null
  return el
}

let installed = false

/** 安装全局 tooltip。重复调用无副作用。 */
export function installTooltip() {
  if (installed || typeof document === 'undefined') return
  installed = true

  document.addEventListener(
    'mouseover',
    (e) => {
      const anchor = findAnchor(e.target)
      if (!anchor) return
      if (anchor === active) return
      scheduleShow(anchor)
    },
    true
  )

  document.addEventListener(
    'mouseout',
    (e) => {
      const anchor = findAnchor(e.target)
      if (!anchor) return
      // 在同一个锚点内部移动（子元素之间）不算离开。
      const to = e.relatedTarget
      if (to instanceof Node && anchor.contains(to)) return
      scheduleHide(anchor)
    },
    true
  )

  // 滚动、点击、按 Esc、窗口失焦都立刻收起，避免浮层停在错位置。
  window.addEventListener('scroll', hide, true)
  document.addEventListener('pointerdown', hide, true)
  window.addEventListener('blur', hide)
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape') hide()
  })
}

/** 供测试与特殊场景手动收起。 */
export function hideTooltip() {
  hide()
}
