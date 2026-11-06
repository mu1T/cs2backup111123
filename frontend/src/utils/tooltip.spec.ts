import { describe, it, expect, beforeAll, afterEach, vi } from 'vitest'
import { installTooltip } from './tooltip'

/**
 * 全局 tooltip 接管的是 document 上的委托监听，所以只装一次。
 * 这里用假定时器验证「延迟出现 / 延迟消失 / 原生 title 被摘走」三件事。
 */
beforeAll(() => {
  installTooltip()
})

afterEach(() => {
  // 只清掉测试造的按钮：浮层节点由模块持有，整个 body 清空会让它脱离文档，
  // 后续测试就再也查不到它了。
  document.querySelectorAll('button').forEach((b) => b.remove())
  document.querySelector('.app-tooltip')?.classList.remove('is-visible')
  vi.useRealTimers()
})

const makeAnchor = (title: string, attrs: Record<string, string> = {}) => {
  const btn = document.createElement('button')
  btn.setAttribute('title', title)
  for (const [k, v] of Object.entries(attrs)) btn.setAttribute(k, v)
  document.body.appendChild(btn)
  return btn
}

const tip = () => document.querySelector('.app-tooltip') as HTMLElement | null

describe('全局 tooltip', () => {
  it('悬停后延迟出现，并把原生 title 摘走', () => {
    vi.useFakeTimers()
    const btn = makeAnchor('写入 autoexec.cfg')

    btn.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }))
    // 还没到延迟时间，不该出现
    expect(tip()?.classList.contains('is-visible') ?? false).toBe(false)

    vi.advanceTimersByTime(500)
    expect(tip()?.classList.contains('is-visible')).toBe(true)
    expect(tip()?.textContent).toBe('写入 autoexec.cfg')
    // 原生提示必须摘掉，否则浏览器会同时再画一个
    expect(btn.hasAttribute('title')).toBe(false)
    expect(btn.getAttribute('data-tip')).toBe('写入 autoexec.cfg')
  })

  it('移开后延迟收起', () => {
    vi.useFakeTimers()
    const btn = makeAnchor('导出 crosshair.cfg')
    btn.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }))
    vi.advanceTimersByTime(500)
    expect(tip()?.classList.contains('is-visible')).toBe(true)

    btn.dispatchEvent(new MouseEvent('mouseout', { bubbles: true }))
    vi.advanceTimersByTime(200)
    expect(tip()?.classList.contains('is-visible')).toBe(false)
  })

  it('在锚点内部移动不算离开', () => {
    vi.useFakeTimers()
    const btn = makeAnchor('带子元素的按钮')
    const child = document.createElement('span')
    btn.appendChild(child)

    btn.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }))
    vi.advanceTimersByTime(500)
    expect(tip()?.classList.contains('is-visible')).toBe(true)

    // 从子元素移到按钮本身：relatedTarget 仍在锚点内
    child.dispatchEvent(new MouseEvent('mouseout', { bubbles: true, relatedTarget: btn }))
    vi.advanceTimersByTime(200)
    expect(tip()?.classList.contains('is-visible')).toBe(true)
  })

  it('空 title 不弹', () => {
    vi.useFakeTimers()
    const btn = makeAnchor('   ')
    btn.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }))
    vi.advanceTimersByTime(500)
    expect(tip()?.classList.contains('is-visible') ?? false).toBe(false)
  })

  it('禁用按钮不弹', () => {
    vi.useFakeTimers()
    const btn = makeAnchor('禁用的按钮')
    ;(btn as HTMLButtonElement).disabled = true
    btn.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }))
    vi.advanceTimersByTime(500)
    expect(tip()?.classList.contains('is-visible') ?? false).toBe(false)
  })

  it('Esc 立刻收起', () => {
    vi.useFakeTimers()
    const btn = makeAnchor('按 Esc 收起')
    btn.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }))
    vi.advanceTimersByTime(500)
    expect(tip()?.classList.contains('is-visible')).toBe(true)

    document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
    expect(tip()?.classList.contains('is-visible')).toBe(false)
  })

  it('点击立刻收起', () => {
    vi.useFakeTimers()
    const btn = makeAnchor('点一下就收')
    btn.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }))
    vi.advanceTimersByTime(500)
    expect(tip()?.classList.contains('is-visible')).toBe(true)

    document.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true }))
    expect(tip()?.classList.contains('is-visible')).toBe(false)
  })

  it('已经搬过的 data-tip 仍然可用（不依赖 title）', () => {
    vi.useFakeTimers()
    const btn = makeAnchor('第二次悬停')
    btn.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }))
    vi.advanceTimersByTime(500)
    btn.dispatchEvent(new MouseEvent('mouseout', { bubbles: true }))
    vi.advanceTimersByTime(200)

    // 此时 title 已被摘走，只剩 data-tip
    expect(btn.hasAttribute('title')).toBe(false)
    btn.dispatchEvent(new MouseEvent('mouseover', { bubbles: true }))
    vi.advanceTimersByTime(500)
    expect(tip()?.textContent).toBe('第二次悬停')
  })
})
