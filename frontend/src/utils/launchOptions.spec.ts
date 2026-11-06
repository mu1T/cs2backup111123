import { describe, it, expect } from 'vitest'
import { hasFlag, addFlag, removeFlag, toggleFlag } from './launchOptions'

describe('hasFlag', () => {
  it('空参数或空 flag 都是 false', () => {
    expect(hasFlag('', '-novid')).toBe(false)
    expect(hasFlag('-novid', '')).toBe(false)
  })

  it('按词边界匹配，不会命中更长的参数', () => {
    expect(hasFlag('-novid', '-novid')).toBe(true)
    expect(hasFlag('-novid2', '-novid')).toBe(false)
    expect(hasFlag('-novidx', '-novid')).toBe(false)
  })

  it('大小写无关', () => {
    expect(hasFlag('-NOViD', '-novid')).toBe(true)
  })

  it('带空格的参数也能匹配（如 -freq 144）', () => {
    expect(hasFlag('-novid -freq 144 -high', '-freq 144')).toBe(true)
    expect(hasFlag('-novid -freq 60 -high', '-freq 144')).toBe(false)
  })

  it('正则元字符被转义，不会误伤', () => {
    expect(hasFlag('+cl_showfps 1', '+cl_showfps 1')).toBe(true)
    expect(hasFlag('xcl_showfps 1', '+cl_showfps 1')).toBe(false)
  })
})

describe('addFlag', () => {
  it('空串时直接放进去', () => {
    expect(addFlag('', '-novid')).toBe('-novid')
    expect(addFlag('   ', '-novid')).toBe('-novid')
  })

  it('追加到末尾并压掉多余空白', () => {
    expect(addFlag('-novid', '-high')).toBe('-novid -high')
    expect(addFlag('  -novid   -high  ', '-threads 4')).toBe('-novid -high -threads 4')
  })

  it('已经有了就不重复追加', () => {
    expect(addFlag('-novid -high', '-novid')).toBe('-novid -high')
    expect(addFlag('-NOViD', '-novid')).toBe('-NOViD')
  })

  it('-freq 是互斥的：先清掉旧的再追加', () => {
    expect(addFlag('-novid -freq 60 -high', '-freq 144')).toBe('-novid -high -freq 144')
  })

  it('只有 -freq 一个参数时，替换后不留多余空格', () => {
    expect(addFlag('-freq 60', '-freq 144')).toBe('-freq 144')
  })

  it('-freq 与其它参数混排时也能干净替换', () => {
    expect(addFlag('-freq 60 -novid', '-freq 144')).toBe('-novid -freq 144')
  })

  it('空 flag 不做任何事', () => {
    expect(addFlag('-novid', '   ')).toBe('-novid')
  })
})

describe('removeFlag', () => {
  it('移除中间的参数不留双空格', () => {
    expect(removeFlag('-novid -high -threads 4', '-high')).toBe('-novid -threads 4')
  })

  it('移除开头或结尾的参数', () => {
    expect(removeFlag('-novid -high', '-novid')).toBe('-high')
    expect(removeFlag('-novid -high', '-high')).toBe('-novid')
  })

  it('移除唯一参数后得到空串', () => {
    expect(removeFlag('-novid', '-novid')).toBe('')
  })

  it('不存在的参数原样返回（已规范化）', () => {
    expect(removeFlag('-novid', '-high')).toBe('-novid')
    expect(removeFlag('  -novid   -high ', '-x')).toBe('-novid -high')
  })

  it('不会误删更长的参数', () => {
    expect(removeFlag('-novid2 -novid', '-novid')).toBe('-novid2')
  })

  it('大小写无关地移除', () => {
    expect(removeFlag('-NOViD -high', '-novid')).toBe('-high')
  })
})

describe('toggleFlag', () => {
  it('没有就加上，wasActive 为 false', () => {
    expect(toggleFlag('-novid', '-high')).toEqual({ next: '-novid -high', wasActive: false })
  })

  it('有就去掉，wasActive 为 true', () => {
    expect(toggleFlag('-novid -high', '-high')).toEqual({ next: '-novid', wasActive: true })
  })

  it('对 -freq 也成立：切换会替换而不是叠加', () => {
    expect(toggleFlag('-freq 60', '-freq 144')).toEqual({ next: '-freq 144', wasActive: false })
    expect(toggleFlag('-freq 144', '-freq 144')).toEqual({ next: '', wasActive: true })
  })

  it('空串上切换 = 添加', () => {
    expect(toggleFlag('', '-novid')).toEqual({ next: '-novid', wasActive: false })
  })
})
