import { describe, it, expect } from 'vitest'
import { formatSize, formatDate } from './format'

describe('formatSize', () => {
  it('把 0 与非法值显示为 0 B', () => {
    expect(formatSize(0)).toBe('0 B')
    expect(formatSize(-5)).toBe('0 B')
    expect(formatSize(NaN)).toBe('0 B')
  })

  it('按 1024 进制换算并保留一位小数', () => {
    expect(formatSize(657)).toBe('657 B')
    expect(formatSize(1024)).toBe('1 KB')
    expect(formatSize(1536)).toBe('1.5 KB')
    expect(formatSize(1024 * 1024 * 2.5)).toBe('2.5 MB')
  })
})

describe('formatDate', () => {
  const now = new Date('2026-11-05T12:00:00Z').getTime()
  const daysAgo = (n: number) => new Date(now - n * 86400000).toISOString()

  it('空值与非法日期原样返回', () => {
    expect(formatDate('', now)).toBe('')
    expect(formatDate('不是日期', now)).toBe('不是日期')
  })

  it('今天与昨天用相对说法', () => {
    expect(formatDate(daysAgo(0), now)).toBe('今天')
    expect(formatDate(daysAgo(1), now)).toBe('昨天')
  })

  it('近 30 天用「N 天前」', () => {
    expect(formatDate(daysAgo(2), now)).toBe('2 天前')
    expect(formatDate(daysAgo(29), now)).toBe('29 天前')
  })

  it('超过 30 天回落到绝对日期', () => {
    const result = formatDate(daysAgo(60), now)
    expect(result).not.toContain('天前')
    expect(result).toMatch(/\d{4}/)
  })
})
