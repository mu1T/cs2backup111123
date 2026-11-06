import { describe, it, expect } from 'vitest'
import { planCompletion } from './cfgCompletion'

/**
 * 上一版把补全的触发条件收得太紧，导致菜单整个不弹。这组用例就是防那个回归：
 * **最常见的「敲两个字母就该有菜单」必须成立**，宽容规则是附加的，不能取代它。
 */
describe('planCompletion', () => {
  const labels = (p: ReturnType<typeof planCompletion>) => (p ? p.options.map((o) => o.label) : [])

  describe('最基本的触发', () => {
    it('敲 cl 就该弹出 cl_* 的候选', () => {
      const p = planCompletion('cl')
      expect(p).not.toBeNull()
      expect(labels(p).some((l) => l.startsWith('cl_'))).toBe(true)
    })

    it('替换范围只覆盖当前单词', () => {
      const p = planCompletion('cl_sho')
      expect(p).not.toBeNull()
      // from 应该是 'cl_sho' 的起点，即 0
      expect(p!.from).toBe(0)
    })

    it('行首有缩进时 from 落在单词起点', () => {
      const p = planCompletion('    cl')
      expect(p!.from).toBe(4)
    })

    it('空行不弹（一进编辑器就满屏候选是噪音）', () => {
      expect(planCompletion('')).toBeNull()
      expect(planCompletion('   ')).toBeNull()
    })
  })

  describe('bind 语境', () => {
    it('bind 之后弹键名', () => {
      const p = planCompletion('bind ')
      expect(labels(p)).toContain('MOUSE1')
    })

    it('bind MO 之后按键名过滤', () => {
      const p = planCompletion('bind MO')
      expect(labels(p).every((l) => l.startsWith('MO'))).toBe(true)
      expect(labels(p)).toContain('MOUSE1')
    })

    it('bind MOUSE1 之后弹 +动作', () => {
      const p = planCompletion('bind MOUSE1 ')
      expect(labels(p)).toContain('+attack')
    })

    it('bind MOUSE1 +a 之后按动作过滤', () => {
      const p = planCompletion('bind MOUSE1 +a')
      expect(labels(p)).toContain('+attack')
      expect(labels(p)).not.toContain('+jump')
    })
  })

  describe('空格当下划线', () => {
    it('敲 cl show 能命中 cl_showfps，且替换整段', () => {
      const p = planCompletion('cl show')
      expect(p).not.toBeNull()
      expect(labels(p)).toContain('cl_showfps')
      // 整段替换：从行首开始，这样接受后 'cl show' 会变成 'cl_showfps'
      expect(p!.from).toBe(0)
    })

    it('敲 cl crosshairstyle 能命中 cl_crosshairstyle', () => {
      const p = planCompletion('cl crosshairstyle')
      expect(labels(p)).toContain('cl_crosshairstyle')
    })
  })

  describe('不该弹的时候别弹', () => {
    it('已经带了数字参数', () => {
      expect(planCompletion('sensitivity 2.5')).toBeNull()
    })

    it('未知单词', () => {
      expect(planCompletion('zzzzz')).toBeNull()
    })

    it('注释行', () => {
      expect(planCompletion('// 这是注释')).toBeNull()
    })

    it('参数位置不补指令：写一句 echo 不该弹 hideconsole', () => {
      // 全量表之后 `hi` 是能命中 `hideconsole` 的，但那里是参数位置
      expect(planCompletion('echo hi')).toBeNull()
      expect(planCompletion('exec practice')).toBeNull()
    })
  })

  describe('取值补全', () => {
    it('指令名打全后连取值一起给', () => {
      const p = planCompletion('cl_crosshairstyle')
      expect(labels(p)).toContain('cl_crosshairstyle')
      expect(labels(p)).toContain('cl_crosshairstyle 4')
    })

    it('指令名打全 + 空格之后也给取值（玩家接受补全后习惯敲空格）', () => {
      expect(labels(planCompletion('sv_cheats '))).toEqual(['sv_cheats 0', 'sv_cheats 1'])
      expect(labels(planCompletion('cl_crosshairstyle '))).toContain('cl_crosshairstyle 4')
    })

    it('没有可枚举取值的指令，空格之后不弹', () => {
      expect(planCompletion('exec ')).toBeNull()
      expect(planCompletion('sensitivity ')).toBeNull()
    })
  })

  /**
   * 用户的原话：「不然玩家不知道把 autoexec 里写只有 sv cheats1 才能用的指令呢？」
   * 所以每条候选都得把「在哪生效 / 要不要作弊 / 这指令是干嘛的」说清楚。
   */
  describe('候选上的适用范围标注', () => {
    const opt = (line: string, label: string) =>
      planCompletion(line)?.options.find((o) => o.label === label)

    it('需要 sv_cheats 的指令在 detail 上标出来', () => {
      const o = opt('nocli', 'noclip')!
      expect(o.detail).toContain('需作弊')
      expect(o.cheats).toBe(true)
    })

    it('服务器指令标「仅服务器」', () => {
      expect(opt('mp_roundt', 'mp_roundtime')!.detail).toContain('仅服务器')
    })

    it('客户端指令标「仅客户端」', () => {
      expect(opt('sensitiv', 'sensitivity')!.detail).toContain('仅客户端')
    })

    it('两边都有的指令不加适用侧标签，但类别一定有', () => {
      const d = opt('mat_wire', 'mat_wireframe')!.detail!
      expect(d).not.toContain('仅')
      expect(d).toContain('图形渲染与帧率性能')
    })

    it('普通客户端指令不会被误标成需要作弊', () => {
      expect(opt('cl_showfp', 'cl_showfps')!.cheats).toBeFalsy()
    })
  })

  describe('选中后的详细说明（info）', () => {
    const info = (line: string, label: string) =>
      planCompletion(line)?.options.find((o) => o.label === label)?.info || ''

    it('中文说明 + 类别 + 适用侧 + 默认值', () => {
      const s = info('mp_roundt', 'mp_roundtime')
      expect(s).toContain('回合与比赛规则')
      expect(s).toContain('仅服务器')
      expect(s).toContain('默认 3')
      expect(s).toContain('分钟')
    })

    it('需要作弊的指令额外说明为什么写在 autoexec 里没用', () => {
      const s = info('nocli', 'noclip')
      expect(s).toContain('sv_cheats 1')
      expect(s).toContain('autoexec')
      expect(s).toContain('飞过墙壁')
    })

    it('数据集没给说明时，明说「没给」，而不是留空或显示占位句', () => {
      const s = info('mp_fraglimit', 'mp_fraglimit')
      expect(s).toContain('没有提供')
      expect(s).not.toContain('游戏本身列表中没有描述的设置')
    })

    it('命令类不写「默认值」', () => {
      expect(info('nocli', 'noclip')).not.toContain('默认')
    })
  })
})
