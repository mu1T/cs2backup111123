import { describe, it, expect } from 'vitest'
import { cfgNameHint, finalCfgName } from './cfgName'

/**
 * 前端这层只做即时反馈，规则要跟 Go 侧 `NormalizeNewCFGName` 对齐。
 * 后端仍然会再判一次（Windows 保留名、控制字符、长度在那边），
 * 这里测的是「用户边打字能看到的那些」。
 */
describe('finalCfgName', () => {
  it('没写后缀就补 .cfg', () => {
    expect(finalCfgName('practice')).toBe('practice.cfg')
  })

  it('已经有后缀就不重复补，大小写不敏感', () => {
    expect(finalCfgName('practice.cfg')).toBe('practice.cfg')
    expect(finalCfgName('Practice.CFG')).toBe('Practice.CFG')
  })

  it('空输入返回空串', () => {
    expect(finalCfgName('   ')).toBe('')
  })
})

describe('cfgNameHint', () => {
  const existing = ['autoexec.cfg', 'practice.cfg']

  it('合法名字不给提示', () => {
    expect(cfgNameHint('surf', existing)).toBe('')
  })

  it('同名（含只差大小写）提示已存在', () => {
    expect(cfgNameHint('practice', existing)).toContain('已存在')
    expect(cfgNameHint('AUTOEXEC', existing)).toContain('已存在')
  })

  it('带非法字符直接说清楚', () => {
    expect(cfgNameHint('a/b', existing)).toContain('不能包含')
    expect(cfgNameHint('a:b', existing)).toContain('不能包含')
  })

  it('空输入不提示（还没开始打字，别一上来就报错）', () => {
    expect(cfgNameHint('', existing)).toBe('')
    expect(cfgNameHint('   ', existing)).toBe('')
  })
})
