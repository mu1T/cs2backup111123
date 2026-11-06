import { describe, it, expect } from 'vitest'
import { describeResolution, describeRefreshRate, pickDefaultPreset, isPresetApplied } from './videoSummary'
import type { VideoPreset } from '../types'

function preset(over: Partial<VideoPreset> = {}): VideoPreset {
  return {
    id: 'p1',
    name: '竞技',
    created_at: '',
    is_default: false,
    file_path: '',
    settings: { 'setting.defaultres': '1920', 'setting.defaultresheight': '1080' },
    ...over,
  } as VideoPreset
}

describe('describeResolution', () => {
  it('缺任一维就报「未检测到配置」', () => {
    expect(describeResolution({})).toBe('未检测到配置')
    expect(describeResolution({ 'setting.defaultres': '1920' })).toBe('未检测到配置')
    expect(describeResolution({ 'setting.defaultresheight': '1080' })).toBe('未检测到配置')
  })

  it('aspectratiomode 明确时直接用', () => {
    const base = { 'setting.defaultres': '1280', 'setting.defaultresheight': '960' }
    expect(describeResolution({ ...base, 'setting.aspectratiomode': '0' })).toBe('1280 x 960 (4:3)')
    expect(describeResolution({ ...base, 'setting.aspectratiomode': '1' })).toBe('1280 x 960 (16:9)')
    expect(describeResolution({ ...base, 'setting.aspectratiomode': '2' })).toBe('1280 x 960 (16:10)')
  })

  it('没有 aspectratiomode 时按宽高比猜：4:3 落在 1.3~1.35', () => {
    // 1024/768 = 1.333
    expect(describeResolution({ 'setting.defaultres': '1024', 'setting.defaultresheight': '768' })).toBe(
      '1024 x 768 (4:3)',
    )
    // 1440/1080 = 1.333
    expect(describeResolution({ 'setting.defaultres': '1440', 'setting.defaultresheight': '1080' })).toBe(
      '1440 x 1080 (4:3)',
    )
    // 1920/1080 = 1.777
    expect(describeResolution({ 'setting.defaultres': '1920', 'setting.defaultresheight': '1080' })).toBe(
      '1920 x 1080 (16:9)',
    )
  })

  it('未知的 aspectratiomode 也走宽高比猜测，而不是显示原值', () => {
    expect(
      describeResolution({
        'setting.defaultres': '1024',
        'setting.defaultresheight': '768',
        'setting.aspectratiomode': '9',
      }),
    ).toBe('1024 x 768 (4:3)')
  })
})

describe('describeRefreshRate', () => {
  it('没有分子就报「未检测到配置」', () => {
    expect(describeRefreshRate({})).toBe('未检测到配置')
    expect(describeRefreshRate({ 'setting.refreshrate_denominator': '1000' })).toBe('未检测到配置')
  })

  it('大于 1000 的值按分子/分母换算', () => {
    expect(
      describeRefreshRate({
        'setting.refreshrate_numerator': '144000',
        'setting.refreshrate_denominator': '1000',
      }),
    ).toBe('144 Hz')
    expect(
      describeRefreshRate({
        'setting.refreshrate_numerator': '239760',
        'setting.refreshrate_denominator': '1001',
      }),
    ).toBe('240 Hz')
  })

  it('小于等于 1000 的值当成直接的赫兹数', () => {
    expect(describeRefreshRate({ 'setting.refreshrate_numerator': '144' })).toBe('144 Hz')
    expect(describeRefreshRate({ 'setting.refreshrate_numerator': '60' })).toBe('60 Hz')
  })

  it('分母缺失时按 1000 算', () => {
    expect(describeRefreshRate({ 'setting.refreshrate_numerator': '165000' })).toBe('165 Hz')
  })

  it('分子不是数字时报「未检测到配置」，不吐 NaN Hz', () => {
    expect(describeRefreshRate({ 'setting.refreshrate_numerator': 'abc' })).toBe('未检测到配置')
    expect(describeRefreshRate({ 'setting.refreshrate_numerator': '0' })).toBe('未检测到配置')
  })
})

describe('pickDefaultPreset', () => {
  it('空列表返回 null', () => {
    expect(pickDefaultPreset([])).toBeNull()
  })

  it('优先取标了 is_default 的那个', () => {
    const a = preset({ id: 'a' })
    const b = preset({ id: 'b', is_default: true })
    expect(pickDefaultPreset([a, b])?.id).toBe('b')
  })

  it('一个都没标默认时退回第一个', () => {
    const a = preset({ id: 'a' })
    const b = preset({ id: 'b' })
    expect(pickDefaultPreset([a, b])?.id).toBe('a')
  })
})

describe('isPresetApplied', () => {
  it('没有预设或没有当前设置时都是 false', () => {
    expect(isPresetApplied(null, { 'setting.defaultres': '1920' })).toBe(false)
    expect(isPresetApplied(preset(), null)).toBe(false)
    expect(isPresetApplied(preset(), {})).toBe(false)
  })

  it('预设为空设置时算不一致（避免空预设显示成「已套用」）', () => {
    expect(isPresetApplied(preset({ settings: {} }), { 'setting.defaultres': '1920' })).toBe(false)
  })

  it('预设里出现的键全部相等才算一致', () => {
    const p = preset()
    expect(isPresetApplied(p, { 'setting.defaultres': '1920', 'setting.defaultresheight': '1080' })).toBe(true)
    expect(isPresetApplied(p, { 'setting.defaultres': '1280', 'setting.defaultresheight': '1080' })).toBe(false)
  })

  it('只比预设里出现的键：当前设置多出来的键不影响判定', () => {
    const p = preset({ settings: { 'setting.defaultres': '1920' } })
    expect(isPresetApplied(p, { 'setting.defaultres': '1920', 'setting.msaa_samples': '8' })).toBe(true)
  })
})
