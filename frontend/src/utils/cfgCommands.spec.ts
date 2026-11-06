import { describe, it, expect } from 'vitest'
import { matchCommands, findCommand, searchCommands, boolHintOf, CFG_COMMANDS, BIND_ACTIONS, BIND_KEYS } from './cfgCommands'

describe('指令表（来自收录的全量 JSON）', () => {
  it('规模是全量，不是手写的那几十条', () => {
    expect(CFG_COMMANDS.length).toBeGreaterThan(2800)
  })

  it('表里没有重复的指令名', () => {
    const names = CFG_COMMANDS.map((c) => c.name.toLowerCase())
    expect(new Set(names).size).toBe(names.length)
  })

  it('中文描述、类别、适用侧、默认值都带上了', () => {
    const c = findCommand('mp_roundtime')!
    expect(c).toBeDefined()
    expect(c.desc).toContain('分钟')
    expect(c.category).toBe('回合与比赛规则')
    expect(c.side).toBe('server')
    expect(c.kind).toBe('cvar')
    expect(c.def).toBe('3')
  })

  it('客户端指令标成 client，通用指令标成 both', () => {
    expect(findCommand('sensitivity')!.side).toBe('client')
    expect(findCommand('bind')!.side).toBe('both')
  })

  it('command 类没有默认值，空串而不是 "Command" 这种占位', () => {
    const c = findCommand('noclip')!
    expect(c.kind).toBe('command')
    expect(c.def).toBe('')
  })

  it('占位描述被清空，不把「游戏本身列表中没有描述的设置」当说明', () => {
    const c = findCommand('mp_fraglimit')!
    expect(c.desc).toBe('')
  })
})

describe('需要 sv_cheats 的判定', () => {
  it('noclip 需要（数据集里它的 RequiresCheats 是 false，靠类别补上）', () => {
    expect(findCommand('noclip')!.cheats).toBe(true)
  })

  it('give / god / impulse 同样需要', () => {
    for (const n of ['give', 'god', 'impulse']) {
      expect(findCommand(n)?.cheats, n).toBe(true)
    }
  })

  it('cl_showpos 需要（数据集字段漏标，类别补上）', () => {
    expect(findCommand('cl_showpos')!.cheats).toBe(true)
  })

  it('sv_cheats 自己不算「需要 sv_cheats」', () => {
    expect(findCommand('sv_cheats')!.cheats).toBe(false)
  })

  it('普通客户端指令不需要', () => {
    for (const n of ['sensitivity', 'cl_showfps', 'bind', 'exec']) {
      expect(findCommand(n)?.cheats, n).toBe(false)
    }
  })

  it('数据集显式标记的也认（mat_wireframe）', () => {
    expect(findCommand('mat_wireframe')!.cheats).toBe(true)
  })
})

describe('取值提示', () => {
  it('布尔类 cvar 默认值是 0/1 时，补全给 0 和 1 两个候选', () => {
    expect(boolHintOf(findCommand('developer')!)).toEqual(['0', '1'])
  })

  it('默认值是 true/false 的也算布尔（数据集里这类有 700 多条）', () => {
    expect(boolHintOf(findCommand('con_enable')!)).toEqual(['0', '1'])
  })

  it('推导出来的 0/1 不写进 values——values 只装人工确认的枚举', () => {
    // 这条边界很关键：校验只认 values，混进推导值就会把
    // cl_radar_scale_alternate 1.00 判成非法（用户报过）
    expect(findCommand('developer')!.values).toBeUndefined()
    expect(findCommand('cl_crosshairstyle')!.values).toContain('9')
  })

  it('多值枚举用人工覆盖层（数据集只给默认值，看不出取值范围）', () => {
    expect(findCommand('cl_crosshairstyle')!.values).toContain('9')
    expect(findCommand('cl_showfps')!.values).toEqual(['0', '1', '2', '3', '4', '5'])
  })

  it('没有可枚举取值的指令不给 values', () => {
    expect(findCommand('sensitivity')!.values).toBeUndefined()
  })
})

describe('matchCommands', () => {
  it('正常前缀匹配', () => {
    const names = matchCommands('cl_crosshairstyle').map((c) => c.name)
    expect(names).toContain('cl_crosshairstyle')
  })

  it('空格当下划线：敲 cl show 也能命中 cl_showfps', () => {
    const names = matchCommands('cl show').map((c) => c.name)
    expect(names).toContain('cl_showfps')
  })

  it('大小写不敏感', () => {
    expect(matchCommands('CL_SHOWFPS').map((c) => c.name)).toContain('cl_showfps')
  })

  it('空前缀不返回任何候选（否则一进编辑器就弹满屏）', () => {
    expect(matchCommands('')).toEqual([])
    expect(matchCommands('   ')).toEqual([])
  })

  it('带参数（含空格且不在指令名里）时不硬猜', () => {
    // sensitivity_2.5 不存在于表里，所以返回空——不会刷一堆无关候选
    expect(matchCommands('sensitivity 2.5')).toEqual([])
  })

  it('全量表下前缀匹配依然只回相关项', () => {
    const names = matchCommands('mp_freezetime').map((c) => c.name)
    expect(names).toContain('mp_freezetime')
    expect(names.length).toBeLessThan(20)
  })

  /**
   * 全量表之后排序变成了一件正经事：`cl_show` 能匹配到
   * `cl_show_clan_in_death_notice` 这种冷门指令，字母序会把它排第一。
   */
  it('完全相同的名字排第一', () => {
    expect(matchCommands('mp_freezetime')[0].name).toBe('mp_freezetime')
  })

  it('短名字优先：敲 cl_show 时第一项是 cl_showfps 而不是冷门长名字', () => {
    expect(matchCommands('cl_show')[0].name).toBe('cl_showfps')
  })
})

describe('bind 用的键名与动作（JSON 里没有，仍然是手写表）', () => {
  it('键名与动作都还在', () => {
    expect(BIND_KEYS).toContain('MOUSE1')
    expect(BIND_ACTIONS).toContain('+attack')
  })
})

/**
 * 用户的原话：「有些玩家肯定会有一种情况，就是只知道指令的描述却不知道具体指令」。
 * 所以搜索必须覆盖 名字 / 类别 / 说明 三个字段，而且中文说明要能直接搜到。
 */
describe('searchCommands', () => {
  it('按中文说明搜：冻结 → mp_freezetime', () => {
    const names = searchCommands('冻结').map((c) => c.name)
    expect(names).toContain('mp_freezetime')
  })

  it('按类别搜：准星 → 一批 cl_crosshair*', () => {
    const names = searchCommands('准星').map((c) => c.name)
    expect(names.some((n) => n.startsWith('cl_crosshair'))).toBe(true)
  })

  it('按名字搜：完全相同的排第一', () => {
    expect(searchCommands('noclip')[0].name).toBe('noclip')
  })

  it('名字前缀比说明命中更靠前', () => {
    const hits = searchCommands('crosshair')
    expect(hits[0].name.startsWith('cl_crosshair') || hits[0].name.startsWith('crosshair')).toBe(true)
  })

  it('多个词是「都要命中」', () => {
    const names = searchCommands('准星 颜色').map((c) => c.name)
    expect(names.length).toBeGreaterThan(0)
    expect(names.some((n) => n.includes('color'))).toBe(true)
  })

  it('大小写不敏感', () => {
    expect(searchCommands('NOCLIP')[0].name).toBe('noclip')
  })

  it('空查询给出前若干条，不能是空列表也不能是全表', () => {
    const hits = searchCommands('')
    expect(hits.length).toBeGreaterThan(0)
    expect(hits.length).toBeLessThan(100)
  })

  it('搜不到就是空', () => {
    expect(searchCommands('zzzzzz')).toEqual([])
  })

  it('润色把「机器人」改成「Bot」之后，按中文搜仍然能搜到', () => {
    // 2026-11-06 那版润色把 72 条描述里的「机器人」统一改成了「Bot」，
    // 按字面匹配会一条都搜不到——所以加了一层同义词
    const names = searchCommands('机器人').map((c) => c.name)
    expect(names.length).toBeGreaterThan(0)
    expect(names).toContain('bot_add')
  })

  it('按「Bot」搜是同一批结果', () => {
    const names = searchCommands('bot').map((c) => c.name)
    expect(names).toContain('bot_add')
  })

  it('同义词不影响「多个词都要命中」的语义', () => {
    const names = searchCommands('机器人 难度').map((c) => c.name)
    expect(names).toContain('bot_add')
  })

  it('结果里不重复', () => {
    const names = searchCommands('cl').map((c) => c.name)
    expect(new Set(names).size).toBe(names.length)
  })
})
