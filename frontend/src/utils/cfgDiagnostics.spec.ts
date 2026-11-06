import { describe, it, expect } from 'vitest'
import { analyzeCfg } from './cfgDiagnostics'

/**
 * 结构校验只做「不需要指令表就能判断」的三件事：
 * 引号配对、bind 缺参数、同一个键重复绑定。
 *
 * 测试重点在「不该报的别报」——一个满屏波浪线的校验器等于没有校验。
 */
describe('analyzeCfg', () => {
  it('干净的配置不报任何问题', () => {
    const text = [
      '// 我的配置',
      'bind MOUSE1 +attack',
      'bind "KEY_W" "+forward"',
      'cl_crosshairstyle 4',
      '',
      'sensitivity 2.5',
    ].join('\n')

    expect(analyzeCfg(text)).toEqual([])
  })

  it('空文本不报错', () => {
    expect(analyzeCfg('')).toEqual([])
  })

  describe('引号不配对', () => {
    it('奇数个双引号报 error，列号指向第一个引号', () => {
      const d = analyzeCfg('bind "MOUSE1 +attack')

      expect(d).toHaveLength(1)
      expect(d[0]).toMatchObject({ line: 1, column: 6, severity: 'error', code: 'unbalanced-quote' })
    })

    it('注释里的引号不参与配对判断', () => {
      // 引号在 // 之后，属于注释，不该被当成不配对
      expect(analyzeCfg('bind MOUSE1 +attack // 他说"这样"')).toEqual([])
    })

    it('引号里的 // 不是注释起点', () => {
      // 这一行整体是合法的：路径字符串里带 //
      const d = analyzeCfg('exec "cfg//test.cfg"')
      expect(d).toEqual([])
    })

    it('一行有两个引号就正常', () => {
      expect(analyzeCfg('bind "MOUSE1" "+attack"')).toEqual([])
    })

    it('不配对的那一行不再报 bind 相关的问题（避免一次报三条）', () => {
      const d = analyzeCfg('bind "MOUSE1')
      expect(d).toHaveLength(1)
      expect(d[0].code).toBe('unbalanced-quote')
    })
  })

  describe('bind 缺参数', () => {
    it('只有 bind 一个词', () => {
      const d = analyzeCfg('bind')
      expect(d).toHaveLength(1)
      expect(d[0]).toMatchObject({ code: 'bind-missing-args', severity: 'error' })
    })

    it('只有键没有指令', () => {
      const d = analyzeCfg('bind MOUSE1')
      expect(d[0].code).toBe('bind-missing-args')
    })

    it('大小写不影响识别', () => {
      const d = analyzeCfg('BIND MOUSE1')
      expect(d[0].code).toBe('bind-missing-args')
    })
  })

  describe('重复绑定', () => {
    it('同一个键绑两次，第二条报 warning 并指出前一行', () => {
      const d = analyzeCfg(['bind MOUSE1 +attack', 'bind MOUSE1 +jump'].join('\n'))

      expect(d).toHaveLength(1)
      expect(d[0]).toMatchObject({ line: 2, severity: 'warning', code: 'duplicate-bind' })
      expect(d[0].message).toContain('第 1 行')
    })

    it('键名大小写不同视为同一个键', () => {
      const d = analyzeCfg(['bind MOUSE1 +attack', 'bind mouse1 +jump'].join('\n'))
      expect(d[0].code).toBe('duplicate-bind')
    })

    it('引号包着的键也能识别重复', () => {
      const d = analyzeCfg(['bind "MOUSE1" "+attack"', 'bind MOUSE1 +jump'].join('\n'))
      expect(d[0].code).toBe('duplicate-bind')
    })

    it('不同的键不互相干扰', () => {
      const d = analyzeCfg(['bind MOUSE1 +attack', 'bind MOUSE2 +jump'].join('\n'))
      expect(d).toEqual([])
    })

    it('绑三次报两次（第二、第三条各报一次）', () => {
      const d = analyzeCfg(['bind MOUSE1 +a', 'bind MOUSE1 +b', 'bind MOUSE1 +c'].join('\n'))
      expect(d).toHaveLength(2)
      expect(d.every((x) => x.code === 'duplicate-bind')).toBe(true)
    })
  })

  describe('行号与排序', () => {
    it('CRLF 换行也能算对行号', () => {
      const d = analyzeCfg('bind MOUSE1 +attack\r\nbind "MOUSE1')
      expect(d[0].line).toBe(2)
    })

    it('多条问题按行号升序返回', () => {
      const d = analyzeCfg(['bind "X', 'bind MOUSE1 +a', 'bind MOUSE1 +b'].join('\n'))
      expect(d.map((x) => x.line)).toEqual([1, 3])
    })
  })

  /**
   * 用户的原话：「不然玩家不知道把 autoexec 里写只有 sv cheats1 才能用的指令呢？」
   *
   * 所以这里不只标「需要作弊」，还要解释为什么写在 autoexec 里没用——
   * 这条 cfg 正是 autoexec 时，警告才有意义。
   */
  describe('需要 sv_cheats 1 的指令', () => {
    it('没开 sv_cheats 就写 noclip：报 warning，并说清原因', () => {
      const d = analyzeCfg('noclip')

      expect(d).toHaveLength(1)
      expect(d[0]).toMatchObject({ line: 1, severity: 'warning', code: 'cheat-without-sv-cheats' })
      expect(d[0].message).toContain('sv_cheats 1')
      expect(d[0].hint).toContain('autoexec')
    })

    it('前面已经写了 sv_cheats 1 就不报', () => {
      const d = analyzeCfg(['sv_cheats 1', 'noclip', 'give weapon_ak47'].join('\n'))
      expect(d).toEqual([])
    })

    it('sv_cheats 1 写在后面不算——cfg 是顺序执行的', () => {
      const d = analyzeCfg(['noclip', 'sv_cheats 1'].join('\n'))
      expect(d.map((x) => x.code)).toEqual(['cheat-without-sv-cheats'])
    })

    it('sv_cheats 0 关掉之后又出现作弊指令，要重新报', () => {
      const d = analyzeCfg(['sv_cheats 1', 'sv_cheats 0', 'noclip'].join('\n'))
      expect(d).toHaveLength(1)
      expect(d[0].line).toBe(3)
    })

    it('sv_cheats 这一行本身不报（它不是「需要作弊」的指令）', () => {
      expect(analyzeCfg('sv_cheats 1')).toEqual([])
    })

    it('普通客户端指令与 bind 不报', () => {
      expect(analyzeCfg(['sensitivity 2.5', 'cl_showfps 1', 'bind MOUSE1 +attack'].join('\n'))).toEqual([])
    })

    it('注释里的作弊指令不报', () => {
      expect(analyzeCfg('// noclip 很有用')).toEqual([])
    })

    it('一堆作弊指令不会刷满屏：最多三条，剩下汇总一条', () => {
      const d = analyzeCfg(['noclip', 'give x', 'god', 'buddha', 'ent_fire'].join('\n'))

      const individual = d.filter((x) => x.code === 'cheat-without-sv-cheats')
      expect(individual).toHaveLength(3)

      const summary = d.filter((x) => x.code === 'cheat-without-sv-cheats-summary')
      expect(summary).toHaveLength(1)
      expect(summary[0].message).toContain('2')
    })

    /**
     * 练习 cfg 里作弊指令基本都是 `bind F noclip` 这种写法。
     * 只查每行第一个词的话，最该提醒的场景一条都报不出来。
     */
    it('bind 里绑的作弊指令也要报', () => {
      const d = analyzeCfg('bind F noclip')

      expect(d).toHaveLength(1)
      expect(d[0].code).toBe('cheat-without-sv-cheats')
      expect(d[0].message).toContain('noclip')
      expect(d[0].message).toContain('按')
    })

    it('bind 里绑普通动作不报', () => {
      expect(analyzeCfg('bind MOUSE1 +attack')).toEqual([])
    })

    it('同一条指令写了两处就报两处（练习 cfg 里最常见的漏报）', () => {
      // 以前按名字去重，`noclip` 和 `bind F noclip` 只提醒一行——而这两处
      // 都是玩家真正要改的地方
      const d = analyzeCfg(['noclip', 'bind F noclip'].join('\n'))
      expect(d.filter((x) => x.code === 'cheat-without-sv-cheats')).toHaveLength(2)
    })

    it('sv_cheats 0 关掉之后重新出现的作弊指令还要报', () => {
      const d = analyzeCfg(['noclip', 'sv_cheats 0', 'bind F noclip'].join('\n'))
      const hits = d.filter((x) => x.code === 'cheat-without-sv-cheats')
      expect(hits.map((x) => x.line)).toEqual([1, 3])
    })

    it('重复命中超过上限时，汇总文案里的名字不重复念', () => {
      const d = analyzeCfg(['bind F noclip', 'bind G noclip', 'bind H noclip', 'bind J noclip'].join('\n'))
      const summary = d.find((x) => x.code === 'cheat-without-sv-cheats-summary')!
      expect(summary.message).toContain('1 处')
      expect(summary.message).not.toContain('noclip、noclip')
    })
  })
})

/**
 * 取值校验：用户原始需求里的「非法 cl_ 值 + 修复建议」。
 *
 * 只校验枚举明确的指令——不从「默认值 0/1」反推布尔，`fps_max` 就是反例。
 */
describe('取值不在允许范围内', () => {
  it('cl_crosshairstyle 7 报出来，并给出改成 0 的建议', () => {
    const d = analyzeCfg('cl_crosshairstyle 7')

    expect(d).toHaveLength(1)
    expect(d[0]).toMatchObject({ line: 1, severity: 'warning', code: 'value-not-allowed' })
    expect(d[0].message).toContain('cl_crosshairstyle')
    expect(d[0].message).toContain('7')
    expect(d[0].fix).toEqual({ value: '0' })
  })

  it('只标出问题的那一段，不标整行', () => {
    const d = analyzeCfg('cl_crosshairstyle 7')
    // 'cl_crosshairstyle 7' 里 7 在第 19 个字符
    expect(d[0].column).toBe(19)
    expect(d[0].length).toBe(1)
  })

  it('合法取值不报', () => {
    expect(analyzeCfg('cl_crosshairstyle 4')).toEqual([])
    expect(analyzeCfg('cl_showfps 3')).toEqual([])
  })

  it('非法取值在 cl_showfps 上也能报', () => {
    const d = analyzeCfg('cl_showfps 9')
    expect(d[0].code).toBe('value-not-allowed')
    expect(d[0].fix).toEqual({ value: '0' })
  })

  it('非数字的取值也报（打错字的情况）', () => {
    const d = analyzeCfg('cl_crosshairstyle abc')
    expect(d[0].code).toBe('value-not-allowed')
  })

  it('只写指令不给值时不报', () => {
    expect(analyzeCfg('cl_crosshairstyle')).toEqual([])
  })

  it('不拿「默认值 0/1」反推布尔：fps_max 300 是合法的', () => {
    // fps_max 默认值是 0，但它接受任意数字。反推会把这条常见写法判成错的。
    expect(analyzeCfg('fps_max 300')).toEqual([])
  })

  it('注释里的非法取值不报', () => {
    expect(analyzeCfg('// cl_crosshairstyle 7')).toEqual([])
  })
})

/**
 * 用户报的：「cl_radar_scale_alternate 1.00 为什么提示要改成 1？1.00≠1 吗？」
 *
 * 两个根因叠在一起：
 * 1. 校验用了「从默认值推出来的 0/1」——这条指令默认值是 1，于是被判成布尔，
 *    而它描述里写着「有效值为 0.25 到 1.0」，根本不是布尔；
 * 2. 取值是字符串比较，`1.00` 和 `1` 不相等。
 */
describe('取值校验的误报', () => {
  it('1.00 等于 1，不该报', () => {
    expect(analyzeCfg('cl_radar_scale_alternate 1.00')).toEqual([])
  })

  it('雷达缩放的其它合法值也不该报', () => {
    expect(analyzeCfg('cl_radar_scale 0.33')).toEqual([])
    expect(analyzeCfg('cl_radar_scale_alternate 0.25')).toEqual([])
  })

  it('人工枚举也要按数值比：cl_showfps 3.0 就是 3', () => {
    expect(analyzeCfg('cl_showfps 3.0')).toEqual([])
    expect(analyzeCfg('cl_crosshairstyle 4.0')).toEqual([])
  })

  it('但真的越界还是要报', () => {
    expect(analyzeCfg('cl_crosshairstyle 7')[0].code).toBe('value-not-allowed')
    expect(analyzeCfg('cl_showfps 9')[0].code).toBe('value-not-allowed')
  })

  it('从默认值推出来的 0/1 不参与校验（否则会把浮点参数判错）', () => {
    // developer 默认值 0，但游戏里接受 0/1/2；这里只是确认它不会因为「不是 0/1」
    // 就被判错——校验只认人工确认过的枚举
    expect(analyzeCfg('developer 2')).toEqual([])
  })
})
