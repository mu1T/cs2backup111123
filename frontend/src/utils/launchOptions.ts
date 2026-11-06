/**
 * CS2 启动项的纯文本处理。
 *
 * 从 LocalBackupView 里拆出来的。这段逻辑原来散在五个函数里（`checkPresetActive`
 * / `isPresetActive` / `isPresetActiveInModal` / `addPreset` / `togglePresetInModal`
 * / `handleToggleLaunchPreset`），其中两处是同一套正则抄了两遍，还有一处用
 * 「子串包含」而另一处用「词边界匹配」——不一致但没有测试能发现。
 *
 * 这里统一成四个函数，全部对「空白分隔的一串参数」做词边界操作。
 */

/** 转义正则元字符，参数里可能出现 `-` `.` `(` 这类字符。 */
function escapeRegExp(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

/** 把多余空白压成一个，并去掉首尾空白。 */
function normalize(args: string): string {
  return args.replace(/\s+/g, ' ').trim()
}

/** 某个参数是否已经在这串启动项里。按词边界匹配，`-novid` 不会命中 `-novid2`。 */
export function hasFlag(args: string, flag: string): boolean {
  if (!args || !flag) return false
  const regex = new RegExp(`(^|\\s)${escapeRegExp(flag.trim())}(\\s|$)`, 'i')
  return regex.test(args)
}

/**
 * 追加一个参数。
 *
 * `-freq` 是互斥的：游戏只认最后一个，所以先清掉已有的再追加，
 * 否则会攒出 `-freq 60 -freq 144` 这种自己都看不懂的东西。
 */
export function addFlag(args: string, flag: string): string {
  const cur = normalize(args)
  const f = flag.trim()
  if (!f) return cur
  if (hasFlag(cur, f)) return cur

  if (f.startsWith('-freq ')) {
    const withoutFreq = normalize(cur.replace(/-freq\s+\d+/gi, ''))
    return withoutFreq ? `${withoutFreq} ${f}` : f
  }
  return cur ? `${cur} ${f}` : f
}

/** 移除一个参数。 */
export function removeFlag(args: string, flag: string): string {
  const f = flag.trim()
  if (!f) return normalize(args)
  const regex = new RegExp(`(^|\\s)${escapeRegExp(f)}(\\s|$)`, 'gi')
  return normalize(args.replace(regex, ' '))
}

/**
 * 有就去掉，没有就加上。
 *
 * 返回 `next` 与 `wasActive`，调用方据此决定提示文案（「已追加」还是「已移除」）。
 */
export function toggleFlag(args: string, flag: string): { next: string; wasActive: boolean } {
  const wasActive = hasFlag(args, flag)
  return {
    next: wasActive ? removeFlag(args, flag) : addFlag(args, flag),
    wasActive,
  }
}
