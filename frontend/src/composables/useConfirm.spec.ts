import { describe, it, expect, vi } from 'vitest'
import { useConfirm } from './useConfirm'

describe('useConfirm', () => {
  it('初始不可见', () => {
    const { confirmState } = useConfirm()
    expect(confirmState.value.visible).toBe(false)
  })

  it('只给 message 时其余字段取默认值', () => {
    const { confirmState, showConfirm } = useConfirm()
    showConfirm({ message: '要备份吗？' })
    const s = confirmState.value
    expect(s.visible).toBe(true)
    expect(s.message).toBe('要备份吗？')
    expect(s.title).toBe('确认操作')
    expect(s.confirmText).toBe('确定')
    expect(s.cancelText).toBe('取消')
    expect(s.isDanger).toBe(false)
    expect(s.showCancel).toBe(true)
    expect(s.prompt).toBe(false)
  })

  it('显式传 false 的布尔项不会被默认值顶掉', () => {
    const { confirmState, showConfirm } = useConfirm()
    showConfirm({ message: 'x', showCancel: false, isDanger: false })
    expect(confirmState.value.showCancel).toBe(false)
    expect(confirmState.value.isDanger).toBe(false)
  })

  it('点确认时先关框再回调，回调能拿到输入值', () => {
    const { confirmState, showConfirm, onConfirmModalConfirm } = useConfirm()
    const onConfirm = vi.fn()
    showConfirm({ message: 'x', prompt: true, onConfirm })

    let visibleWhenCalled: boolean | null = null
    onConfirm.mockImplementation(() => {
      visibleWhenCalled = confirmState.value.visible
    })

    onConfirmModalConfirm('新备注')
    expect(onConfirm).toHaveBeenCalledWith('新备注')
    // 回调里若又开一个框，不该被这次关闭盖掉
    expect(visibleWhenCalled).toBe(false)
    expect(confirmState.value.visible).toBe(false)
  })

  it('点取消只跑 onCancel，不跑 onConfirm', () => {
    const { showConfirm, onConfirmModalCancel } = useConfirm()
    const onConfirm = vi.fn()
    const onCancel = vi.fn()
    showConfirm({ message: 'x', onConfirm, onCancel })

    onConfirmModalCancel()
    expect(onCancel).toHaveBeenCalledTimes(1)
    expect(onConfirm).not.toHaveBeenCalled()
  })

  it('没有回调时点确认也不炸', () => {
    const { showConfirm, onConfirmModalConfirm, onConfirmModalCancel } = useConfirm()
    showConfirm({ message: 'x' })
    expect(() => onConfirmModalConfirm()).not.toThrow()
    expect(() => onConfirmModalCancel()).not.toThrow()
  })

  it('两次 showConfirm 不共享回调', () => {
    const { showConfirm, onConfirmModalConfirm } = useConfirm()
    const first = vi.fn()
    const second = vi.fn()
    showConfirm({ message: 'a', onConfirm: first })
    showConfirm({ message: 'b', onConfirm: second })

    onConfirmModalConfirm()
    expect(first).not.toHaveBeenCalled()
    expect(second).toHaveBeenCalledTimes(1)
  })
})
