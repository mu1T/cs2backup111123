#!/usr/bin/env node
/**
 * 把 `scripts/data/cs2_commands.json`（收录的 CS2 全量指令）编译成前端用的紧凑数据模块。
 *
 * 为什么要生成而不是运行时直接 import 那份 JSON：
 * - 原文件 1.1 MB，9 个字段里 Description_EN 与 Category_EN 前端用不到，
 *   光这两项就占了将近一半体积；
 * - 运行时只 import 一个定好类型的数组，省掉 2836 次对象字面量的解析。
 *
 * 生成规则（每一处都是刻意的，改之前先想清楚）：
 * - `Default` 为 `Command` / `Not listed` 时置空 —— 那不是默认值，是「没有默认值」的占位；
 * - 描述等于占位串「游戏本身列表中没有描述的设置。」时置空 —— 949 条都是这句，
 *   留着只会在界面上刷屏，空串让 UI 自己去说「游戏未提供说明」；
 * - `RequiresCheats` 与数据集自带的「作弊与跑图练习」类别取并集。**单看字段不够**：
 *   noclip / give / god / impulse 都被标成 false，但它们显然要 sv_cheats 1；
 *   而这个类别里的 106 条就是练习/作弊指令，并集才是玩家真正需要的那份名单；
 * - 按名称排序，保证每次生成结果稳定（否则 diff 一片红）。
 *
 * 用法：node scripts/gen-cs2-commands.mjs
 */
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

const HERE = dirname(fileURLToPath(import.meta.url))
const SRC = resolve(HERE, 'data/cs2_commands.json')
const OUT = resolve(HERE, '../src/data/cs2Commands.generated.ts')

/**
 * 数据集里「没有描述」的占位句，不是真的描述。
 *
 * 收录方换过几次措辞，现在同时存在三种（2026-11-06 那份数据里分别是 949 / 1302 / 46 条）：
 * `游戏本身列表中没有描述的设置。`、`游戏本身列表中没有描述的命令。`、`暂无官方说明`。
 * 用正则兜住「设置/命令」的变体，别每换一次措辞就漏一批。
 */
const DESC_PLACEHOLDER = /^游戏本身列表中没有描述的(设置|命令)[。.]?$|^暂无官方说明$/
/** 需要作弊的类别（中文名来自数据集本身，不是我们编的）。 */
const CHEAT_CATEGORY = '作弊与跑图练习'
/** 自己就在开作弊开关，不能标成「需要作弊」——否则会对着 sv_cheats 1 这一行报警。 */
const CHEAT_EXEMPT = new Set(['sv_cheats'])

const SIDE = { Server: 'server', Client: 'client', Both: 'both' }
const KIND = { cvar: 'cvar', command: 'command' }

const raw = JSON.parse(readFileSync(SRC, 'utf8'))
if (!Array.isArray(raw) || raw.length === 0) throw new Error('源数据不是非空数组')

const problems = []
const categories = []
const catIndex = new Map()

function categoryId(name) {
  const key = (name || '').trim() || '未分类'
  if (!catIndex.has(key)) {
    catIndex.set(key, categories.length)
    categories.push(key)
  }
  return catIndex.get(key)
}

const rows = raw.map((e, i) => {
  const name = String(e.Name || '').trim()
  if (!/^[A-Za-z0-9_.+-]+$/.test(name)) problems.push(`#${i} 指令名异常: ${JSON.stringify(name)}`)

  const side = SIDE[e.Side]
  if (!side) problems.push(`#${i} ${name} 的 Side 未知: ${JSON.stringify(e.Side)}`)
  const kind = KIND[e.Type]
  if (!kind) problems.push(`#${i} ${name} 的 Type 未知: ${JSON.stringify(e.Type)}`)

  let desc = String(e.Description || '').trim()
  if (DESC_PLACEHOLDER.test(desc)) desc = ''

  let def = String(e.Default || '').trim()
  if (def === 'Command' || def === 'Not listed') def = ''

  const cat = String(e.Category || '').trim()
  const flagged = e.RequiresCheats !== false && e.RequiresCheats !== 0
  const cheats = (flagged || cat === CHEAT_CATEGORY) && !CHEAT_EXEMPT.has(name)

  return [name, desc, categoryId(cat), side || 'both', kind || 'cvar', def, cheats ? 1 : 0]
})

const dupes = rows.map((r) => r[0].toLowerCase()).filter((n, i, a) => a.indexOf(n) !== i)
if (dupes.length) problems.push(`重名指令: ${[...new Set(dupes)].slice(0, 5).join(', ')}`)

rows.sort((a, b) => a[0].toLowerCase().localeCompare(b[0].toLowerCase()))

const q = (s) => JSON.stringify(s)
const lines = rows.map((r) => `  [${q(r[0])}, ${q(r[1])}, ${r[2]}, ${q(r[3])}, ${q(r[4])}, ${q(r[5])}, ${r[6]}],`)

const out = `/**
 * **自动生成，不要手改。** 改数据请改 \`scripts/data/cs2_commands.json\`，
 * 然后跑 \`node scripts/gen-cs2-commands.mjs\`。
 *
 * 源数据：收录的 CS2 公开指令表（${raw.length} 条），字段已裁剪到前端实际用得到的部分。
 * 元组顺序：[名称, 中文描述, 类别下标, 适用侧, 类型, 默认值, 是否需 sv_cheats]
 * - 描述为空串表示「游戏没有提供说明」，不是漏抓；
 * - 默认值空串表示该指令没有默认值（command 类）或数据源未收录。
 */

export type Cs2Side = 'server' | 'client' | 'both'
export type Cs2Kind = 'cvar' | 'command'
export type Cs2CommandRow = readonly [
  name: string,
  desc: string,
  category: number,
  side: Cs2Side,
  kind: Cs2Kind,
  def: string,
  cheats: 0 | 1,
]

export const CS2_CATEGORIES: readonly string[] = [
${categories.map((c) => `  ${q(c)},`).join('\n')}
]

export const CS2_COMMANDS: readonly Cs2CommandRow[] = [
${lines.join('\n')}
]
`

mkdirSync(dirname(OUT), { recursive: true })
writeFileSync(OUT, out, 'utf8')

const kb = (Buffer.byteLength(out, 'utf8') / 1024).toFixed(1)
console.log(`已生成 ${OUT}`)
console.log(`  指令 ${rows.length} 条 · 类别 ${categories.length} 个 · ${kb} KB`)
console.log(`  需 sv_cheats 的 ${rows.filter((r) => r[6]).length} 条 · 无描述的 ${rows.filter((r) => !r[1]).length} 条`)
if (problems.length) {
  console.error(`\n数据有问题（${problems.length} 处）：`)
  for (const p of problems.slice(0, 20)) console.error('  - ' + p)
  process.exitCode = 1
}
