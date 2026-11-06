import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { useToast } from './useToast'

/**
 * 提示条的两条规矩：
 * - 两种消息互斥，出错了不该同时挂着上一条成功提示；
 * - 成功提示 4 秒后自动消失，但如果期间又弹了别的，不能把新的清掉。
 */
function mountToast() {
  let api!: ReturnType<typeof useToast>
  const wrapper = mount(
    defineComponent({
      setup() {
        api = useToast()
        return () => h('div')
      },
    }),
  )
  return { api, wrapper }
}

describe('useToast', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('初始状态两条都是空的', () => {
    const { api } = mountToast()
    expect(api.successMsg.value).toBe('')
    expect(api.errorMsg.value).toBe('')
  })

  it('成功提示会清掉错误提示', () => {
    const { api } = mountToast()
    api.showMessage('出错了', true)
    api.showMessage('成功了')
    expect(api.successMsg.value).toBe('成功了')
    expect(api.errorMsg.value).toBe('')
  })

  it('错误提示会清掉成功提示，且不会自动消失', () => {
    const { api } = mountToast()
    api.showMessage('成功了')
    api.showMessage('出错了', true)
    expect(api.errorMsg.value).toBe('出错了')
    expect(api.successMsg.value).toBe('')

    vi.advanceTimersByTime(10_000)
    expect(api.errorMsg.value).toBe('出错了')
  })

  it('成功提示 4 秒后自动消失', () => {
    const { api } = mountToast()
    api.showMessage('成功了')
    vi.advanceTimersByTime(3_999)
    expect(api.successMsg.value).toBe('成功了')
    vi.advanceTimersByTime(2)
    expect(api.successMsg.value).toBe('')
  })

  it('连续两条成功提示时，旧定时器不会把新的清掉', () => {
    const { api } = mountToast()
    api.showMessage('第一条')
    vi.advanceTimersByTime(3_000)
    api.showMessage('第二条')
    // 第一条的 4 秒到了
    vi.advanceTimersByTime(1_500)
    expect(api.successMsg.value).toBe('第二条')
    // 第二条自己的 4 秒到了
    vi.advanceTimersByTime(3_000)
    expect(api.successMsg.value).toBe('')
  })

  it('clearToast 清空两条并停掉定时器', () => {
    const { api } = mountToast()
    api.showMessage('成功了')
    api.clearToast()
    expect(api.successMsg.value).toBe('')
    vi.advanceTimersByTime(5_000)
    expect(api.successMsg.value).toBe('')
  })

  it('组件卸载后定时器不再触发', () => {
    const { api, wrapper } = mountToast()
    api.showMessage('成功了')
    wrapper.unmount()
    expect(() => vi.advanceTimersByTime(5_000)).not.toThrow()
  })
})
