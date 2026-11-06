/**
 * 新建 cfg 文件时的**即时反馈**规则。
 *
 * 注意：真正说了算的是 Go 侧的 `NormalizeNewCFGName`（它还管 Windows 保留名、
 * 控制字符、长度这些）。这里只做用户边打字边能看到的几条，让「已存在」这类
 * 问题不用等提交才知道。两处规则不一致时，以后端返回的错误为准——组件会把
 * 后端错误原样显示出来。
 */

/** 文件名里不许出现的字符（和后端 illegalNameChars 保持一致）。 */
const ILLEGAL = /[/\\:*?"<>|]/

/** 用户输入 → 最终文件名（后缀自动补 .cfg，大小写不敏感）。 */
export function finalCfgName(raw: string): string {
  const name = raw.trim()
  if (!name) return ''
  return name.toLowerCase().endsWith('.cfg') ? name : `${name}.cfg`
}

/**
 * 返回一条提示；空串表示「看起来没问题」。
 *
 * @param existing 当前作用域下已存在的文件名，用于查重（大小写不敏感：
 *                 Windows 文件系统不区分大小写，`Autoexec.cfg` 和
 *                 `autoexec.cfg` 是同一个文件）
 */
export function cfgNameHint(raw: string, existing: readonly string[]): string {
  const name = raw.trim()
  if (!name) return ''
  if (ILLEGAL.test(name)) return '不能包含 / \\ : * ? " < > |'

  const finalName = finalCfgName(name)
  if (existing.some((n) => n.toLowerCase() === finalName.toLowerCase())) {
    return `${finalName} 已存在，换个名字`
  }
  return ''
}
