/**
 * 补全的决策逻辑：给定「光标前的这一行」，算出该弹什么候选、替换哪一段。
 *
 * 抽成纯函数而不是写在组件里，是因为上一版把它埋进补全回调后，出问题只能靠
 * 手敲验证——结果就出了「菜单整个不弹」这种低级故障。
 *
 * 两条规则按优先级：
 * 1. **整行（空格当 `_`）能匹配到** → 替换整行。
 *    这是「空格当下划线」的宽容：敲 `cl show` 能命中 `cl_showfps`，
 *    接受后整段被替换成正确写法。
 * 2. **当前单词能匹配到** → 替换当前单词。这是最常见的情况：敲 `cl` 就该弹 `cl_*`。
 *
 * 顺序不能反。指令表从手写 59 条换成全量 2836 条之后，`show` 这种词能命中
 * `show_loadout_toggle` 之类的指令，于是「当前单词」规则会把 `cl show` 抢走，
 * 弹出跟用户意图无关的东西。行内有空格时，整行是更具体的那个查询，先试它。
 *
 * 两条都不中就不弹——`sensitivity 2.5` 这种已经带参数的，不该刷一堆无关候选。
 */

import { BIND_ACTIONS, BIND_KEYS, CFG_WORD_AT_END, boolHintOf, findCommand, matchCommands, type CfgCommand } from './cfgCommands'

export interface CompletionOption {
  label: string
  type: 'keyword' | 'constant' | 'variable'
  /** 行内短标签：需作弊 / 仅服务器 / 类别。 */
  detail?: string
  /** 选中后侧边浮层里的详细说明（多行纯文本）。 */
  info?: string
  /** 需要 sv_cheats 1：组件据此给这一行加标记样式。 */
  cheats?: boolean
}

export interface CompletionPlan {
  /** 相对「光标前这一行」的起始偏移，调用方加上行首位置即可。 */
  from: number
  options: CompletionOption[]
}


const SIDE_LABEL: Record<CfgCommand['side'], string> = {
  server: '仅服务器',
  client: '仅客户端',
  both: '',
}

/** 行内短标签：`需作弊 · 仅服务器 · 回合与比赛规则`。 */
export function detailOf(c: CfgCommand): string {
  return joinParts(c.cheats && '需作弊', SIDE_LABEL[c.side], c.category)
}

/** 数据集没给说明时的占位文案；界面三处共用，别各写一遍。 */
export const NO_DESC = '游戏没有提供这条指令的说明。'

/** 把若干段用 ` · ` 连起来，空段自动跳过。 */
function joinParts(...parts: (string | false)[]): string {
  return parts.filter(Boolean).join(' · ')
}

/** 一条指令的「元信息」行：类别 · 适用侧 · 类型 · 默认值。 */
export function metaOf(c: CfgCommand): string {
  return joinParts(c.category, SIDE_LABEL[c.side], c.kind === 'cvar' ? 'cvar' : '命令', c.def && `默认 ${c.def}`)
}

/**
 * 选中后的详细说明。
 *
 * 用户的原话是「不然玩家不知道把 autoexec 里写只有 sv_cheats 1 才能用的指令呢」，
 * 所以需要作弊的那批除了标出来，还要直说为什么写在 autoexec 里没用。
 *
 * 段落之间用空行分隔：`white-space: pre-line` 会把空行原样渲染出来，
 * 三行挤在一起读起来太累。
 *
 * 不用 `⚠` 这类符号：ADR-0008 要求全站零 Emoji，Windows 上它是彩色字形，
 * 和周围的线性矢量图标不是一套。要表达警示就用中文。
 */
export function infoOf(c: CfgCommand): string {
  const parts = [metaOf(c), c.desc || NO_DESC]

  if (c.cheats) {
    parts.push('注意：需要 sv_cheats 1，而且只在本地/自建服务器里能开。写进 autoexec 连官方服务器时，这条不起作用。')
  }
  return parts.join('\n\n')
}

/**
 * 补全用的取值候选：人工枚举优先，否则用布尔推导。
 *
 * 注意这是**补全**的取值来源，比校验宽——校验只认 `c.values`（见 cfgDiagnostics）。
 */
function valuesOf(c: CfgCommand): string[] | undefined {
  return c.values ?? boolHintOf(c)
}

function commandOptions(commands: CfgCommand[], exact: string): CompletionOption[] {
  return commands.flatMap((c) => {
    const items: CompletionOption[] = [
      {
        label: c.name,
        type: c.kind === 'cvar' ? 'constant' : 'keyword',
        detail: detailOf(c),
        info: infoOf(c),
        cheats: c.cheats,
      },
    ]
    // 指令名已经打全了，就把已知取值一起给出来（人工枚举优先，否则用布尔推导）
    if (c.name.toLowerCase() === exact) {
      for (const v of valuesOf(c) ?? []) {
        items.push({
          label: `${c.name} ${v}`,
          type: 'constant',
          detail: '取值',
          info: infoOf(c),
          cheats: c.cheats,
        })
      }
    }
    return items
  })
}

/**
 * 计算补全方案。
 *
 * @param lineBeforeCursor 光标前的整行文本（不含光标后的部分）
 */
export function planCompletion(lineBeforeCursor: string): CompletionPlan | null {
  const lead = lineBeforeCursor.length - lineBeforeCursor.trimStart().length
  const trimmed = lineBeforeCursor.trimStart()
  if (!trimmed) return null

  const endsWithSpace = /\s$/.test(lineBeforeCursor)
  const tokens = trimmed.split(/\s+/).filter(Boolean)

  // ---- bind 语境 ----
  if (tokens[0]?.toLowerCase() === 'bind') {
    // 刚敲完 `bind `：还什么都没有，把键名全给出来
    if (tokens.length === 1 && endsWithSpace) {
      const options = BIND_KEYS.map((k) => ({ label: k, type: 'variable' as const }))
      return { from: lineBeforeCursor.length, options }
    }
    // 正在打键名：只有 bind + 一个词，且还没打空格
    if (tokens.length === 2 && !endsWithSpace) {
      const word = tokens[1]
      const options = BIND_KEYS.filter((k) => k.toLowerCase().startsWith(word.toLowerCase())).map(
        (k) => ({ label: k, type: 'variable' as const }),
      )
      return options.length ? { from: lineBeforeCursor.length - word.length, options } : null
    }
    // 打动作：bind <键> 之后
    if (tokens.length >= 2) {
      const word = endsWithSpace ? '' : tokens[tokens.length - 1]
      const options = BIND_ACTIONS.filter((a) =>
        a.toLowerCase().startsWith(word.toLowerCase()),
      ).map((a) => ({ label: a, type: 'keyword' as const }))
      return options.length ? { from: lineBeforeCursor.length - word.length, options } : null
    }
  }

  // ---- 指令名打全 + 一个空格：给取值 ----
  //
  // 接受补全之后玩家通常直接敲空格，这时候该给 `sv_cheats 1` 这种候选，
  // 而不是什么都不弹。替换范围是整行，接受后空格不会多出来。
  // 尾随空格会被 filter(Boolean) 吃掉，所以「指令名 + 空格」是 1 个 token
  if (tokens.length === 1 && endsWithSpace) {
    const cmd = findCommand(tokens[0])
    const values = cmd && valuesOf(cmd)
    if (cmd && values?.length) {
      return {
        from: lead,
        options: values.map((v) => ({
          label: `${cmd.name} ${v}`,
          type: 'constant' as const,
          detail: '取值',
          info: infoOf(cmd),
          cheats: cmd.cheats,
        })),
      }
    }
  }

  // ---- 规则 1：整行（空格当 _）做前缀，只在行内有空格时才试 ----
  if (tokens.length > 1) {
    const byLine = matchCommands(trimmed)
    if (byLine.length) {
      const query = trimmed.replace(/\s+/g, '_').toLowerCase()
      return { from: lead, options: commandOptions(byLine, query) }
    }
  }

  // ---- 规则 2：当前单词做前缀，但只有「这个词是行首第一个词」时才可能是指令 ----
  //
  // 全量表之后这条限制是必须的：`echo hi` 里的 `hi` 会命中 `hideconsole`，
  // 于是写一句 echo 就弹出无关候选。指令只在行首才有意义，参数位置不补指令。
  if (tokens.length > 1) return null

  const wordMatch = CFG_WORD_AT_END.exec(lineBeforeCursor)
  const word = wordMatch ? wordMatch[0] : ''

  if (word) {
    const byWord = matchCommands(word)
    if (byWord.length) {
      return { from: lineBeforeCursor.length - word.length, options: commandOptions(byWord, word.toLowerCase()) }
    }
  }

  return null
}
