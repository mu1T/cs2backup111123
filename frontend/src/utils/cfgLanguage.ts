/**
 * CS2 `.cfg` 的语法高亮。
 *
 * 为什么需要这个：CodeMirror 的 `basicSetup` **不含任何 cfg 语法**——没有语言
 * 定义就只是纯文本，一个字都不会变色。这是接编辑器时最容易漏的一步。
 *
 * 配色与分组照着 VSCode 扩展 `yunkong.cs2-cfg-highlighter` 的 TextMate 语法
 * 来（MIT），但换成手写分词器而不是搬 `.tmLanguage.json`：那份语法有十几条
 * 正则互斥的 `patterns`，靠 VSCode 的匹配优先级决定谁赢；CodeMirror 的
 * StreamLanguage 是按顺序取第一个命中的，直接搬会打架。
 *
 * 分词优先级（从高到低）：注释 → 引号字符串 → 数字 → 键名/动作 → 指令 → 普通词。
 */
import { StreamLanguage, HighlightStyle, syntaxHighlighting } from '@codemirror/language'
import { tags } from '@lezer/highlight'
import { CFG_COMMANDS, CFG_WORD_AT_START, BIND_KEYS, BIND_ACTIONS } from './cfgCommands'

const COMMAND_SET = new Set(CFG_COMMANDS.map((c) => c.name.toLowerCase()))
const KEY_SET = new Set(BIND_KEYS.map((k) => k.toLowerCase()))
const ACTION_SET = new Set(BIND_ACTIONS.map((a) => a.toLowerCase()))

/** 一行里第一个词才可能是「指令」，后面的是参数。 */
interface CfgState {
  /** 本行是否已经出现过非空白 token。 */
  sawToken: boolean
}

const cfgStream = StreamLanguage.define<CfgState>({
  name: 'cs2cfg',

  startState: () => ({ sawToken: false }),

  token(stream, state) {
    // 换行时重置「本行是否已有 token」
    if (stream.sol()) state.sawToken = false

    if (stream.eatSpace()) return null

    // 注释：整行剩余部分
    if (stream.match('//')) {
      stream.skipToEnd()
      return 'comment'
    }

    // 引号字符串：不吃到行尾，避免不配对的引号把整篇染成字符串
    if (stream.peek() === '"') {
      stream.next()
      while (!stream.eol()) {
        const ch = stream.next()
        if (ch === '"') break
      }
      return 'string'
    }

    // 数字：0-255、2.5、-10、1e3
    if (stream.match(/^[+-]?\d*\.?\d+([eE][+-]?\d+)?/)) {
      return 'number'
    }

    // 键名与 +动作：只在 bind 语境下有意义，但按形状判更简单也不会误伤
    const word = stream.match(CFG_WORD_AT_START) as RegExpMatchArray | null
    if (word) {
      const raw = word[0]
      const lower = raw.toLowerCase()

      if (ACTION_SET.has(lower) || /^[+-][a-z_]+$/.test(lower)) return 'atom'
      if (KEY_SET.has(lower)) return 'variableName'

      const isFirst = !state.sawToken
      state.sawToken = true

      // 第一个词且是指令表里的 → 指令色
      if (isFirst && COMMAND_SET.has(lower)) return 'keyword'

      // 形如 cl_xxx / sv_xxx / r_xxx 的，即使不在表里也按指令色——用户最需要
      // 一眼看出「这是个控制台指令」而不是普通单词
      if (/^(cl|sv|r|mp|host|mat|viewmodel|fps|voice|con|m)_/.test(lower)) return 'keyword'

      return 'variableName'
    }

    stream.next()
    return null
  },

  languageData: {
    commentTokens: { line: '//' },
  },
})

/**
 * 配色。
 *
 * 注释、正文用应用已有的主题变量；字符串/数字/指令/键名这四类走语法高亮
 * 专用色（主题里没有对应语义的 token，硬凑会跟状态色撞车）。
 */
const cfgHighlight = HighlightStyle.define([
  { tag: tags.comment, color: 'var(--color-text-muted)', fontStyle: 'italic' },
  { tag: tags.string, color: '#a5d6a7' },
  { tag: tags.number, color: '#ffcc80' },
  { tag: tags.keyword, color: '#82b1ff', fontWeight: '600' },
  { tag: tags.atom, color: '#f48fb1' },
  { tag: tags.variableName, color: 'var(--color-text)' },
])

/** 语言 + 配色的完整扩展，挂到编辑器上。 */
export const cfgLanguage = [cfgStream, syntaxHighlighting(cfgHighlight)]
