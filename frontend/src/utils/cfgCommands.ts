/**
 * CFG 补全用的指令表。
 *
 * 数据来自 `data/cs2Commands.generated.ts`——那是收录的 CS2 公开指令全量表
 * （2836 条）经 `scripts/gen-cs2-commands.mjs` 裁剪生成的，**不要再往这里手写指令**。
 * 手写时代只有 59 条，玩家敲 `mp_` 或 `sv_` 基本补不出来。
 *
 * 这里只做三件事：
 * 1. 把生成好的元组摊成对象，并建一份小写索引（补全和诊断都要按名字查）；
 * 2. 补上数据集给不了的东西：**取值提示**（JSON 只有默认值，看不出枚举范围）
 *    和 **bind 用的键名/动作**（那是键位表，不是指令表）；
 * 3. 提供前缀匹配。
 *
 * 关于「是否需要 sv_cheats」：生成器用的是「数据集字段 ∪ 作弊与跑图练习类别」。
 * 单看字段会漏——`noclip` / `give` / `god` / `impulse` 在数据集里都标着 false，
 * 但玩家在 autoexec 里写它们显然不会生效。并集才是玩家真正需要的那份名单。
 * 反过来 `sv_cheats` 自己在这个类别里，但它不是「需要作弊」的指令，生成器已豁免。
 */
import {
  CS2_COMMANDS,
  CS2_CATEGORIES,
  type Cs2Kind,
  type Cs2Side,
} from '../data/cs2Commands.generated'

export type CfgKind = Cs2Kind
export type CfgSide = Cs2Side

export interface CfgCommand {
  name: string
  /** 中文说明；空串表示游戏没提供（数据集里是「游戏本身列表中没有描述的设置。」）。 */
  desc: string
  /** 中文类别，例如「回合与比赛规则」。 */
  category: string
  /** 这条指令在哪一侧生效：服务器 / 客户端 / 两边都有。 */
  side: CfgSide
  /** cvar 是变量（可赋值），command 是执行一次的命令。 */
  kind: CfgKind
  /** 默认值；空串表示没有默认值或数据集未收录。 */
  def: string
  /** 是否需要 `sv_cheats 1` 才能生效。 */
  cheats: boolean
  /**
   * **人工确认过**的取值枚举；没有则为 undefined。
   *
   * 只有这张表里的值可以拿来校验（`value-not-allowed`）——它是人看过游戏行为
   * 才写下的。从默认值推导的 0/1 只用于补全提示，见 `boolHintOf`。
   */
  values?: string[]
}

/**
 * 人工取值覆盖层。
 *
 * 只写「数据集帮不上忙」的：JSON 里 `cl_crosshairstyle` 的默认值是 `Not listed`，
 * 看不出它接受 0-5 和 9。布尔类的不用写，见下面的 `derivedValues`。
 */
const VALUE_HINTS: Record<string, string[]> = {
  cl_crosshairstyle: ['0', '1', '2', '3', '4', '5', '9'],
  cl_crosshaircolor: ['0', '1', '2', '3', '4', '5'],
  cl_showfps: ['0', '1', '2', '3', '4', '5'],
  viewmodel_presetpos: ['1', '2', '3'],
}

/**
 * 默认值是 0/1/true/false 的 cvar，大概率是开关，补全时把 0/1 两个候选给出来。
 *
 * **只用于补全提示，绝不用于校验。** 默认值是 1 不等于「只接受 0/1」：
 * `cl_radar_scale_alternate` 默认值 1，描述里写着「有效值为 0.25 到 1.0」。
 * 之前校验直接用了这个推导值，于是 `1.00` 被判成非法——用户报的就是这个。
 */
const BOOL_DEFAULTS = new Set(['0', '1', 'true', 'false'])

/** 布尔类 cvar 的补全候选；不是枚举，别拿它做校验。 */
export function boolHintOf(c: CfgCommand): string[] | undefined {
  return BOOL_DEFAULTS.has(c.def) ? ['0', '1'] : undefined
}

export const CFG_COMMANDS: CfgCommand[] = CS2_COMMANDS.map((row) => {
  const [name, desc, category, side, kind, def, cheats] = row
  const cmd: CfgCommand = {
    name,
    desc,
    category: CS2_CATEGORIES[category] ?? '未分类',
    side,
    kind,
    def,
    cheats: cheats === 1,
  }
  // 只认人工枚举；推导出来的 0/1 走 boolHintOf，留在补全层
  const values = VALUE_HINTS[name]
  if (values) cmd.values = values
  return cmd
})

/** 按小写名字查，补全和诊断都用它。 */
const COMMAND_INDEX = new Map(CFG_COMMANDS.map((c) => [c.name.toLowerCase(), c]))

/** 查一条指令；表里没有返回 undefined。 */
export function findCommand(name: string): CfgCommand | undefined {
  return COMMAND_INDEX.get(name.trim().toLowerCase())
}

/**
 * 指令名允许的字符集，三处共用：补全的「光标前那个词」、搜索面板插入时的
 * 「光标前残缺的词」、语法高亮的分词器。以前三处各写一份（高亮那份还多了个
 * `$`），改「什么算一个词」要同时动三个文件。
 */
export const CFG_WORD_AT_END = /[\w+\-._]*$/
export const CFG_WORD_AT_START = /^[\w+\-._]+/

/** 常见的 bind 键名，补全 `bind ` 之后用。键位表不在 JSON 里，只能手写。 */
export const BIND_KEYS: string[] = [
  'MOUSE1', 'MOUSE2', 'MOUSE3', 'MOUSE4', 'MOUSE5',
  'W', 'A', 'S', 'D', 'Q', 'E', 'R', 'F', 'G', 'C', 'V', 'B', 'T', 'X', 'Z',
  'SPACE', 'SHIFT', 'CTRL', 'ALT', 'TAB', 'ESC', 'ENTER', 'BACKSPACE',
  '1', '2', '3', '4', '5', '6', '7', '8', '9', '0',
  'F1', 'F2', 'F3', 'F4', 'F5', 'F6', 'F7', 'F8', 'F9', 'F10', 'F11', 'F12',
  'KP_END', 'KP_DOWNARROW', 'KP_PGDN', 'KP_LEFTARROW', 'KP_5', 'KP_RIGHTARROW',
  'KP_HOME', 'KP_UPARROW', 'KP_PGUP', 'KP_ENTER', 'KP_INS', 'KP_DEL',
]

/** 常见的 +动作，补全 `bind <键> ` 之后用。同样不在 JSON 里。 */
export const BIND_ACTIONS: string[] = [
  '+attack', '+attack2', '+forward', '+back', '+moveleft', '+moveright',
  '+jump', '+duck', '+use', '+reload', '+speed', '+walk',
  '-attack', '-attack2', '-forward', '-back', '-moveleft', '-moveright',
  '-jump', '-duck', '-use', '-reload', '-speed', '-walk',
]

/**
 * 按前缀匹配指令名。
 *
 * **空格当作下划线**：用户可能顺手敲 `cl showfps`，而真实指令是 `cl_showfps`。
 * 这里先把空格换成 `_` 再比，于是 `cl show` 能命中 `cl_showfps`，接受补全后
 * 整段被替换成正确写法。
 *
 * **排序规则**（全量表之后才有意义）：完全相同的排最前 → 名字短的优先 →
 * 字母序。原因很实际：敲 `cl_show` 时字母序会把
 * `cl_show_clan_in_death_notice` 排到 `cl_showfps` 前面，而前者几乎没人用。
 * 没有「常用度」数据可用，名字长度是唯一能自动拿到的相关性代理——通用的
 * 指令名字短，冷门的名字长。
 *
 * 抽成纯函数是为了能测这两条规则——藏在补全回调里的话，只能靠手敲验证。
 */
export function matchCommands(prefix: string): CfgCommand[] {
  // 先判空再换空格：全空白的输入会被换成 `_`，而表里真有 `_record` 这种指令，
  // 于是「光标停在一行空白上」就会弹出一个候选——这个顺序不能反。
  if (!prefix.trim()) return []
  const query = prefix.replace(/\s+/g, '_').trim().toLowerCase()

  const matched = CFG_COMMANDS.filter((c) => c.name.toLowerCase().startsWith(query))
  return matched.sort((a, b) => {
    const aName = a.name.toLowerCase()
    const bName = b.name.toLowerCase()
    if (aName === query || bName === query) return aName === query ? -1 : 1
    if (a.name.length !== b.name.length) return a.name.length - b.name.length
    return aName < bName ? -1 : aName > bName ? 1 : 0
  })
}

/**
 * 中文别名 → 数据集里的实际用词。
 *
 * 2026-11-06 那版润色把 72 条描述里的「机器人」统一改成了「Bot」，于是玩家按
 * 中文搜「机器人」一条都搜不到——搜索是按字面匹配的。这里补一层同义词：
 * 一个词命中任意一个变体就算命中。
 *
 * 只收录「润色后确实搜不到」的词，不是通用词表。
 */
const SEARCH_SYNONYMS: Record<string, string[]> = {
  机器人: ['bot'],
  人机: ['bot'],
  电脑人: ['bot'],
}

/** 单个词对一条指令的得分；0 表示没命中。 */
function scoreTerm(name: string, category: string, desc: string, term: string): number {
  if (name === term) return 1000
  if (name.startsWith(term)) return 600
  if (name.includes(term)) return 300
  if (category.includes(term)) return 120
  if (desc.includes(term)) return 80
  return 0
}

/**
 * 按「名字 / 类别 / 中文说明」搜指令。
 *
 * 存在的理由：玩家经常知道效果（「每轮开始冻结几秒」）却不知道指令叫什么。
 * 补全只能按名字前缀匹配，帮不上这种忙。
 *
 * 打分（越大越靠前）：
 * - 名字完全相同 1000，名字前缀 600，名字包含 300
 * - 类别命中 120，说明命中 80
 * - 再给短名字一点加成：同样的命中，短名字更可能是玩家要的那条
 *
 * 多个词是「都要命中」（AND），这样「准星 颜色」能收到 cl_crosshaircolor 这类。
 */
export function searchCommands(query: string, limit = 50): CfgCommand[] {
  const q = query.trim().toLowerCase()
  if (!q) return CFG_COMMANDS.slice(0, limit)

  const terms = q.split(/\s+/).filter(Boolean)
  const scored: { cmd: CfgCommand; score: number }[] = []

  for (const cmd of CFG_COMMANDS) {
    const name = cmd.name.toLowerCase()
    const category = cmd.category.toLowerCase()
    const desc = cmd.desc.toLowerCase()
    let score = 0
    let all = true

    for (const term of terms) {
      // 一个词可以有多个写法：中文别名和数据集里的实际用词，命中任意一个即可
      let best = scoreTerm(name, category, desc, term)
      for (const variant of SEARCH_SYNONYMS[term] ?? []) {
        best = Math.max(best, scoreTerm(name, category, desc, variant))
      }
      if (best === 0) {
        all = false
        break
      }
      score += best
    }
    if (!all) continue

    // 名字越短越可能是正主，但别让它盖过命中等级的差距
    score += Math.max(0, 40 - cmd.name.length)
    scored.push({ cmd, score })
  }

  scored.sort((a, b) => b.score - a.score || a.cmd.name.length - b.cmd.name.length || (a.cmd.name < b.cmd.name ? -1 : 1))
  return scored.slice(0, limit).map((s) => s.cmd)
}
