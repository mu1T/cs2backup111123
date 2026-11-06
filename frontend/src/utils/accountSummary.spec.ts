import { describe, it, expect } from 'vitest'
import { emptyAccountSummary, describeCrosshairToggle } from './accountSummary'

/**
 * 准星显示开关在 autoexec.cfg 里有三态，卡片上的徽标文案全靠这一个函数。
 * 三态里「没写」和「写了 0」在界面上必须能区分开——前者是跟随游戏内设置，
 * 后者是明确关掉了。
 */
describe('describeCrosshairToggle', () => {
  it('文件里没写这个键时是「跟随游戏内设置」', () => {
    expect(describeCrosshairToggle('')).toEqual({ label: '跟随游戏内设置', off: false })
    expect(describeCrosshairToggle('   ')).toEqual({ label: '跟随游戏内设置', off: false })
  })

  it('0 与 false 都算关闭，且大小写无关', () => {
    for (const raw of ['0', 'false', 'FALSE', ' False ']) {
      expect(describeCrosshairToggle(raw), raw).toEqual({ label: '已关闭', off: true })
    }
  })

  it('1 与 true 算开启', () => {
    for (const raw of ['1', 'true', 'TRUE']) {
      expect(describeCrosshairToggle(raw), raw).toEqual({ label: '已开启', off: false })
    }
  })

  it('无法识别的值按开启处理，而不是当成关闭', () => {
    expect(describeCrosshairToggle('2')).toEqual({ label: '已开启', off: false })
  })
})

describe('emptyAccountSummary', () => {
  it('返回两份空摘要，字段齐全且可安全读取', () => {
    const s = emptyAccountSummary()
    expect(s.video.resolutionText).toBe('')
    expect(s.video.defaultPresetName).toBeNull()
    expect(s.video.loading).toBe(false)
    expect(s.crosshair.text).toBe('')
    expect(s.crosshair.toggleOff).toBe(false)
  })

  it('每次返回新对象，避免多处共享同一份引用', () => {
    const a = emptyAccountSummary()
    const b = emptyAccountSummary()
    a.video.readOnly = true
    expect(b.video.readOnly).toBe(false)
  })
})
