/**
 * 展示层格式化工具。
 * 这些函数被多处复用（按文件备份、编辑器文件列表、快照列表），
 * 抽出来是为了能被单测直接驱动，而不必挂载整个组件。
 */

/** 字节数 → 人类可读体积，例如 657 → "657 B"、1280 → "1.3 KB"。 */
export const formatSize = (bytes: number): string => {
  if (!bytes || bytes <= 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
}

/**
 * ISO 时间 → 相对日期文案。
 * 今天/昨天/近 30 天用相对说法，更早用绝对日期；无法解析时原样返回。
 */
export const formatDate = (iso: string, now: number = Date.now()): string => {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d.getTime())) return iso
  const diffDays = Math.floor((now - d.getTime()) / 86400000)
  if (diffDays <= 0) return '今天'
  if (diffDays === 1) return '昨天'
  if (diffDays < 30) return `${diffDays} 天前`
  return d.toLocaleDateString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit' })
}
