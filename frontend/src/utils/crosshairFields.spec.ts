import { describe, it, expect } from 'vitest'
import { CROSSHAIR_FIELDS, fieldsOfGroup, fieldOf } from './crosshairFields'
import { defaultCrosshair } from './wails'

/**
 * 这张表是九个数值参数的唯一真值。它一旦和 Crosshair 对不上，
 * 界面上就会出现「滑杆能拖到的值，模型里没有对应字段」这类静默问题。
 */
describe('准星数值参数元数据表', () => {
  it('每个字段都能在 Crosshair 里找到同名字段，且默认值是合法数字', () => {
    const defaults = defaultCrosshair() as unknown as Record<string, unknown>
    for (const f of CROSSHAIR_FIELDS) {
      expect(defaults, `Crosshair 缺少字段 ${f.key}`).toHaveProperty(f.key)
      expect(typeof defaults[f.key], `${f.key} 不是数字`).toBe('number')
      expect(Number.isFinite(defaults[f.key] as number)).toBe(true)
    }
  })

  it('默认值落在各自范围内', () => {
    const defaults = defaultCrosshair() as unknown as Record<string, unknown>
    for (const f of CROSSHAIR_FIELDS) {
      const v = defaults[f.key] as number
      expect(v, `${f.key} 默认值 ${v} 低于 min ${f.min}`).toBeGreaterThanOrEqual(f.min)
      expect(v, `${f.key} 默认值 ${v} 高于 max ${f.max}`).toBeLessThanOrEqual(f.max)
    }
  })

  it('key 不重复', () => {
    const keys = CROSSHAIR_FIELDS.map((f) => f.key)
    expect(new Set(keys).size).toBe(keys.length)
  })

  it('范围与步长自洽：min < max，step > 0，且 step 能整除区间', () => {
    for (const f of CROSSHAIR_FIELDS) {
      expect(f.min, `${f.key} min 不小于 max`).toBeLessThan(f.max)
      expect(f.step, `${f.key} step 非正`).toBeGreaterThan(0)
      const steps = (f.max - f.min) / f.step
      expect(Number.isInteger(Number(steps.toFixed(6))), `${f.key} 区间不是 step 的整数倍`).toBe(true)
    }
  })

  it('小数位与步长一致：step 小于 1 的字段必须保留小数', () => {
    for (const f of CROSSHAIR_FIELDS) {
      if (f.step < 1) {
        expect(f.decimals, `${f.key} 步长 ${f.step} 却取整，拖滑杆会卡住`).toBeGreaterThan(0)
      } else {
        expect(f.decimals, `${f.key} 步长为整数却保留小数`).toBe(0)
      }
    }
  })

  it('每个字段都有标签与说明，九个字段全部被某个分组覆盖', () => {
    for (const f of CROSSHAIR_FIELDS) {
      expect(f.label.length).toBeGreaterThan(0)
      expect(f.tip.length).toBeGreaterThan(0)
    }
    const grouped = [...fieldsOfGroup('size'), ...fieldsOfGroup('advanced'), ...fieldsOfGroup('dynamic')]
    expect(grouped.length).toBe(CROSSHAIR_FIELDS.length)
  })

  it('fieldOf 对未定义的键抛错，而不是返回 undefined', () => {
    expect(fieldOf('gap').label).toBe('间距')
    expect(() => fieldOf('nope' as never)).toThrow(/未定义的准星数值字段/)
  })
})
