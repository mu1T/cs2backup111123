/**
 * CS2 `.cfg` 的校验。
 *
 * 两类检查：
 * 1. **不需要指令表**的：引号配对、`bind` 缺参数、同一个键重复绑定。
 * 2. **需要指令表**的：写了需要 `sv_cheats 1` 的指令，却没在它之前开作弊。
 *    这是玩家最容易踩的坑——把 `noclip`、`give` 写进 autoexec，连官方服务器时
 *    一条都不生效，而游戏不会给任何提示。
 *
 * 刻意不做的事：
 * - **不判断指令是否存在**。全量指令表（`cfgCommands.ts`）里有的才算已知，
 *   表里没有的既不报「不存在」也不报「未知」——指令表本身是收录来的，
 *   拿它当「不存在」的判据会误伤新指令。
 * - **不判断取值是否合法**（需要类型/范围表）。
 * - **不判断「仅服务器」指令写在客户端 cfg 里**。自建服务器跑 practice cfg 时
 *   写 `mp_*` 完全合法，这条规则会天天误报。
 * - **不试图修复**。这里只报告位置与原因，改不改由调用方决定。
 *
 * `bind <键> <指令>` 里绑的指令也会查：练习 cfg 里作弊指令基本都这么写，
 * 只查每行第一个词的话，最该提醒的场景一条都报不出来。
 *
 * 纯函数、不碰 DOM、不碰网络，所以边界情况可以毫秒级测。
 */

import { findCommand } from './cfgCommands'

export type DiagnosticSeverity = 'error' | 'warning'

export type DiagnosticCode =
  | 'unbalanced-quote'
  | 'bind-missing-args'
  | 'duplicate-bind'
  | 'cheat-without-sv-cheats'
  | 'cheat-without-sv-cheats-summary'
  | 'value-not-allowed'

export interface Diagnostic {
  /** 1 起算，和编辑器行号一致。 */
  line: number
  /** 1 起算的列号，指向问题起始位置。 */
  column: number
  severity: DiagnosticSeverity
  code: DiagnosticCode
  message: string
  /** 给用户的下一步建议；没有合适建议时为空。 */
  hint?: string
  /**
   * 从 `column` 起要标红多少个字符。缺省表示整行。
   * 取值类问题只标那一段，别把整行画成红的。
   */
  length?: number
  /**
   * 一键修复的目标值。编辑器据此生成一个「改成 X」的 action。
   * 只有取值类问题会给——别的规则改法不唯一，不该替用户做主。
   */
  fix?: { value: string }
}

/** 一条被解析出来的 `bind`，用于查重。 */
interface BindRecord {
  key: string
  line: number
}

/** 一条「需要作弊却没开」的命中，攒够了一起决定怎么报。 */
interface CheatHit {
  line: number
  column: number
  name: string
  /** 直接写的指令，还是 `bind <键> <指令>` 里绑的那个。 */
  via: 'line' | 'bind'
}

/** 逐条报最多报几条；再多就汇总成一条，免得满屏波浪线。 */
const CHEAT_WARN_LIMIT = 3

/**
 * 两个取值是否等价：字符串相同，或者都能解析成同一个数。
 *
 * `1.00` 和 `1` 在游戏眼里是同一个值，按字符串比会误报——用户报的就是这个。
 */
function sameValue(a: string, b: string): boolean {
  if (a === b) return true
  if (!a.trim() || !b.trim()) return false
  const na = Number(a)
  const nb = Number(b)
  return Number.isFinite(na) && Number.isFinite(nb) && na === nb
}

/** `sv_cheats 1` / `true` 视为开，其余视为关。 */
function isOn(value: string | undefined): boolean {
  const v = (value || '').toLowerCase()
  return v === '1' || v === 'true'
}

/** 去掉行内注释（`//` 之后的部分），保留引号里的内容。 */
function stripComment(line: string): string {
  let inQuote = false
  for (let i = 0; i < line.length; i++) {
    const ch = line[i]
    if (ch === '"') {
      inQuote = !inQuote
    } else if (!inQuote && ch === '/' && line[i + 1] === '/') {
      return line.slice(0, i)
    }
  }
  return line
}

/** 统计一行里未转义的引号数量。CS2 的 cfg 不支持反斜杠转义，所以直接数。 */
function countQuotes(line: string): number {
  let n = 0
  for (const ch of line) {
    if (ch === '"') n++
  }
  return n
}

/** 按空白切词，引号内的空白不切。返回去引号后的词。 */
function tokenize(line: string): string[] {
  const out: string[] = []
  let cur = ''
  let inQuote = false
  let started = false

  for (const ch of line) {
    if (ch === '"') {
      inQuote = !inQuote
      started = true
      continue
    }
    if (!inQuote && /\s/.test(ch)) {
      if (started) {
        out.push(cur)
        cur = ''
        started = false
      }
      continue
    }
    cur += ch
    started = true
  }
  if (started) out.push(cur)
  return out
}

/**
 * 扫描整份 cfg，返回按行号排好序的诊断列表。
 *
 * 空行与纯注释行会被跳过——它们不该产生任何提示。
 */
export function analyzeCfg(text: string): Diagnostic[] {
  const diagnostics: Diagnostic[] = []
  const firstBindLine = new Map<string, number>()
  /** 当前位置 sv_cheats 是否开着；cfg 是顺序执行的，所以要边走边记。 */
  let cheatsOn = false
  const cheatHits: CheatHit[] = []

  // 统一换行，行号才对得上编辑器
  const lines = text.split(/\r\n|\r|\n/)

  lines.forEach((rawLine, idx) => {
    const lineNo = idx + 1
    const code = stripComment(rawLine)

    // 1. 引号不配对
    const quotes = countQuotes(code)
    if (quotes % 2 !== 0) {
      diagnostics.push({
        line: lineNo,
        column: code.indexOf('"') + 1,
        severity: 'error',
        code: 'unbalanced-quote',
        message: `引号没有配对（这一行有 ${quotes} 个双引号）`,
        hint: '这一行之后的所有内容都会被当成字符串的一部分，游戏会整行忽略。',
      })
      // 引号都不配对，后面的 token 解析没有意义，跳过这一行
      return
    }

    const tokens = tokenize(code)
    if (tokens.length === 0) return

    const command = tokens[0].toLowerCase()
    const column = code.length - code.trimStart().length + 1

    // 4. 需要 sv_cheats 1 的指令：记状态，顺带攒命中
    if (command === 'sv_cheats') {
      // 开关自己不是「需要作弊」的指令，只更新状态
      cheatsOn = isOn(tokens[1])
      return
    }
    if (!cheatsOn) {
      // 直接写的指令
      const direct = findCommand(command)
      if (direct?.cheats) {
        cheatHits.push({ line: lineNo, column, name: direct.name, via: 'line' })
      }
      // `bind <键> <指令>` 里绑的那个。练习 cfg 里作弊指令基本都是这种写法，
      // 只查每行第一个词的话，最该提醒的场景反而一条都不报。
      const bound = command === 'bind' && tokens[2] ? findCommand(tokens[2]) : undefined
      if (bound?.cheats) {
        cheatHits.push({ line: lineNo, column, name: bound.name, via: 'bind' })
      }
    }

    /*
     * 5. 取值不在已知枚举里。
     *
     * **只校验人工确认过的枚举**（`CfgCommand.values`，目前是准星样式/颜色、
     * cl_showfps、持枪视角预设）。从默认值推导的 0/1 只用于补全提示，不进这里：
     * `cl_radar_scale_alternate` 默认值是 1，但它的有效范围是 0.25–1.0，
     * 拿推导值校验就会把 `1.00` 判成非法——宁可少报，不可误报。
     *
     * 比较按数值等价：`1.00` 等于 `1`。
     */
    const known = findCommand(command)
    if (known?.values?.length && tokens.length >= 2) {
      const value = tokens[1]
      if (!known.values.some((v) => sameValue(v, value))) {
        const at = code.indexOf(value, code.indexOf(tokens[0]) + tokens[0].length)
        const start = at >= 0 ? at + 1 : column
        const fixTo = known.values.find((v) => sameValue(v, known.def)) ?? known.values[0]
        diagnostics.push({
          line: lineNo,
          column: start,
          length: value.length,
          severity: 'warning',
          code: 'value-not-allowed',
          message: `${known.name} 的取值只能是 ${known.values.join(' / ')}，这里是 ${value}`,
          hint: `改成 ${fixTo} 通常是安全的；如果这条指令在游戏里能接受别的值，说明这张表该更新了，或者或许存在例外情况。`,
          fix: { value: fixTo },
        })
      }
    }

    // 2. bind 缺参数
    if (command === 'bind') {
      if (tokens.length < 3) {
        diagnostics.push({
          line: lineNo,
          column,
          severity: 'error',
          code: 'bind-missing-args',
          message: 'bind 需要两个参数：键名与要执行的指令',
          hint: '写法是 bind <键> <指令>，例如 bind MOUSE1 +attack',
        })
        return
      }

      // 3. 同一个键重复绑定
      const key = tokens[1].toLowerCase()
      const prev = firstBindLine.get(key)
      if (prev !== undefined) {
        diagnostics.push({
          line: lineNo,
          column,
          severity: 'warning',
          code: 'duplicate-bind',
          message: `键 ${tokens[1]} 在第 ${prev} 行已经绑定过，这里的绑定会覆盖它`,
          hint: '如果这是有意的覆盖，可以忽略；否则删掉其中一条。',
        })
      } else {
        firstBindLine.set(key, lineNo)
      }
    }
  })

  // 4. 汇总「需要作弊却没开」的命中
  for (const [i, hit] of cheatHits.entries()) {
    if (i < CHEAT_WARN_LIMIT) {
      diagnostics.push({
        line: hit.line,
        column: hit.column,
        severity: 'warning',
        code: 'cheat-without-sv-cheats',
        message:
          hit.via === 'bind'
            ? `绑定的 ${hit.name} 需要 sv_cheats 1，按下这个键也不会有反应`
            : `${hit.name} 需要 sv_cheats 1 才能生效`,
        hint: 'autoexec 是在连接服务器时执行的，那里开不了作弊；这条只在自己建的本地服务器里配合 sv_cheats 1 有用。',
      })
    } else {
      const rest = cheatHits.slice(CHEAT_WARN_LIMIT)
      // 名字去重再展示：同一个 noclip 绑了 5 个键时，别把「noclip、noclip、noclip」念出来
      const names = [...new Set(rest.map((h) => h.name))]
      diagnostics.push({
        line: hit.line,
        column: hit.column,
        severity: 'warning',
        code: 'cheat-without-sv-cheats-summary',
        message: `还有 ${rest.length} 处需要 sv_cheats 1（${names.slice(0, 3).join('、')}${names.length > 3 ? '…' : ''}），同样不会生效`,
        hint: '整份文件都要跑图/练习的话，在最前面加一行 sv_cheats 1，并在本地服务器里执行。',
      })
      break
    }
  }

  return diagnostics.sort((a, b) => a.line - b.line || a.column - b.column)
}
