import { describe, it, expect } from 'vitest'
// `?raw` 读原文而不是 import 成对象：2836 条的字面量类型会让 vue-tsc 慢得难受，
// 而且这里要的只是「生成结果和源数据是否一致」，不需要类型。
import rawJson from '../../scripts/data/cs2_commands.json?raw'
import { CS2_COMMANDS, CS2_CATEGORIES } from './cs2Commands.generated'

interface RawEntry {
  Name: string
}

const source = JSON.parse(rawJson) as RawEntry[]

/**
 * 生成物与源数据的一致性检查。
 *
 * 没有这组断言的话，最容易发生的退化是「改了 JSON 忘了跑生成器」或者
 * 「生成器被改坏，静默少了几百条」——界面照常工作，只是补不出那些指令，
 * 而且谁都不会发现。
 */
describe('cs2Commands.generated.ts 与源数据一致', () => {
  it('条数与源 JSON 一致', () => {
    expect(CS2_COMMANDS.length).toBe(source.length)
  })

  it('源数据里每一条都在表里', () => {
    const have = new Set(CS2_COMMANDS.map((r) => r[0].toLowerCase()))
    const missing = source.map((e) => e.Name).filter((n) => !have.has(n.toLowerCase()))
    expect(missing).toEqual([])
  })

  it('表里没有源数据之外的多余指令', () => {
    const want = new Set(source.map((e) => e.Name.toLowerCase()))
    const extra = CS2_COMMANDS.map((r) => r[0]).filter((n) => !want.has(n.toLowerCase()))
    expect(extra).toEqual([])
  })

  it('占位句没有被当成真描述（收录方换过三种措辞）', () => {
    const placeholder = /^游戏本身列表中没有描述的(设置|命令)[。.]?$|^暂无官方说明$/
    const bad = CS2_COMMANDS.filter((r) => placeholder.test(r[1]))
    expect(bad.map((r) => r[0])).toEqual([])
  })

  it('类别下标都指向真实类别', () => {
    const bad = CS2_COMMANDS.filter((r) => r[2] < 0 || r[2] >= CS2_CATEGORIES.length)
    expect(bad).toEqual([])
  })

  it('适用侧与类型只有约定内的取值', () => {
    const sides = new Set(CS2_COMMANDS.map((r) => r[3]))
    const kinds = new Set(CS2_COMMANDS.map((r) => r[4]))
    expect([...sides].sort()).toEqual(['both', 'client', 'server'])
    expect([...kinds].sort()).toEqual(['command', 'cvar'])
  })
})
