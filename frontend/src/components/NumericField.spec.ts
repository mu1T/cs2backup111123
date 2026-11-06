import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import NumericField from './NumericField.vue'

/**
 * NumericField 是「标签 + 数字框 + 滑杆」的唯一实现，九个参数共用。
 *
 * 这里锁的是两段行为：
 * - 打字过程中超范围的值不写回（否则用户删掉 12 想输入 120，删到 1 就被夹走）；
 * - 失焦 / 回车时一定夹取并格式化（模型里永远合法）。
 */
function mountField(props: Record<string, unknown> = {}) {
  return mount(NumericField, {
    props: { modelValue: 5, label: '长度', min: 0, max: 64, step: 1, ...props },
  })
}

function lastModelValue(wrapper: ReturnType<typeof mountField>): number | undefined {
  const events = wrapper.emitted('update:modelValue')
  if (!events?.length) return undefined
  return events[events.length - 1][0] as number
}

describe('NumericField', () => {
  it('渲染标签、说明与数字框，范围与步长来自 props', () => {
    const wrapper = mountField({ tip: '单位是像素', min: -10, max: 64, step: 1 })
    expect(wrapper.text()).toContain('长度')
    expect(wrapper.find('.field-label span').attributes('title')).toBe('单位是像素')

    const num = wrapper.find('.num-input')
    expect(num.attributes('min')).toBe('-10')
    expect(num.attributes('max')).toBe('64')
    expect(num.attributes('step')).toBe('1')
    expect((num.element as HTMLInputElement).value).toBe('5')
  })

  it('没有说明时不渲染空 title', () => {
    const wrapper = mountField()
    expect(wrapper.find('.field-label span').attributes('title')).toBeUndefined()
  })

  it('输入范围内的值时立即写回', async () => {
    const wrapper = mountField()
    const num = wrapper.find('.num-input')
    ;(num.element as HTMLInputElement).value = '12'
    await num.trigger('input')
    expect(lastModelValue(wrapper)).toBe(12)
  })

  it('输入超范围的值时先不写回，避免打字打到一半被夹走', async () => {
    const wrapper = mountField()
    const num = wrapper.find('.num-input')
    ;(num.element as HTMLInputElement).value = '999'
    await num.trigger('input')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('输入非数字时不写回', async () => {
    const wrapper = mountField()
    const num = wrapper.find('.num-input')
    ;(num.element as HTMLInputElement).value = ''
    await num.trigger('input')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('失焦时把超范围的值夹到上界并写回输入框', async () => {
    const wrapper = mountField()
    const num = wrapper.find('.num-input')
    ;(num.element as HTMLInputElement).value = '999'
    await num.trigger('change')
    expect(lastModelValue(wrapper)).toBe(64)
    expect((num.element as HTMLInputElement).value).toBe('64')
  })

  it('失焦时把低于下界的值夹到下界', async () => {
    const wrapper = mountField({ min: -10 })
    const num = wrapper.find('.num-input')
    ;(num.element as HTMLInputElement).value = '-99'
    await num.trigger('change')
    expect(lastModelValue(wrapper)).toBe(-10)
  })

  it('空输入失焦时退回下界，而不是留成 NaN', async () => {
    const wrapper = mountField({ min: 0 })
    const num = wrapper.find('.num-input')
    ;(num.element as HTMLInputElement).value = ''
    await num.trigger('change')
    expect(lastModelValue(wrapper)).toBe(0)
  })

  it('按 decimals 保留小数位', async () => {
    const wrapper = mountField({ modelValue: 0.5, min: 0, max: 1, step: 0.01, decimals: 2 })
    const num = wrapper.find('.num-input')
    ;(num.element as HTMLInputElement).value = '0.333333'
    await num.trigger('change')
    expect(lastModelValue(wrapper)).toBe(0.33)
  })

  it('decimals 为 0 时取整', async () => {
    const wrapper = mountField({ modelValue: 5, min: 0, max: 64, step: 1, decimals: 0 })
    const num = wrapper.find('.num-input')
    ;(num.element as HTMLInputElement).value = '12.7'
    await num.trigger('change')
    expect(lastModelValue(wrapper)).toBe(13)
  })

  it('拖滑杆直接写回，不做夹取（浏览器已保证在范围内）', async () => {
    const wrapper = mountField()
    const slider = wrapper.find('.slider')
    ;(slider.element as HTMLInputElement).value = '20'
    await slider.trigger('input')
    expect(lastModelValue(wrapper)).toBe(20)
  })

  it('滑杆与数字框共享同一组范围属性', () => {
    const wrapper = mountField({ min: 0.3, max: 1, step: 0.01 })
    const slider = wrapper.find('.slider')
    expect(slider.attributes('min')).toBe('0.3')
    expect(slider.attributes('max')).toBe('1')
    expect(slider.attributes('step')).toBe('0.01')
  })
})
