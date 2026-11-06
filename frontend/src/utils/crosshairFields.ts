/**
 * 准星数值参数的唯一真值表。
 *
 * 这九个参数原本在 CrosshairModal 里各写一遍「标签 + 数字框 + 滑杆」，
 * 范围值散在数字框属性、@input 实参、@change 实参、滑杆属性四处——
 * 加一个参数要改五处，改漏一处就是「滑杆能拖到 64，手输却被夹到 32」这种 bug。
 *
 * 现在范围、步长、小数位、说明文案都在这里，渲染交给 NumericField。
 * 新增参数：这里加一行 + `types/index.ts` 的 Crosshair 加字段 + Go 侧默认值。
 */

/** 可以在界面上用「数字框 + 滑杆」编辑的字段。 */
export type CrosshairNumericKey =
  | 'length'
  | 'thickness'
  | 'gap'
  | 'split_dist'
  | 'spread_limit'
  | 'scope_dot_scale'
  | 'inner_alpha'
  | 'outer_alpha'
  | 'split_ratio'

/** 参数出现在界面的哪一块，决定渲染位置与是否随样式显隐。 */
export type CrosshairFieldGroup = 'size' | 'advanced' | 'dynamic'

export interface CrosshairFieldMeta {
  key: CrosshairNumericKey
  /** 数字框左侧的短标签。 */
  label: string
  /** hover 说明，搬运到 NumericField 的 title。 */
  tip: string
  min: number
  max: number
  step: number
  /** 写回模型时保留的小数位；0 表示取整。 */
  decimals: number
  group: CrosshairFieldGroup
}

export const CROSSHAIR_FIELDS: readonly CrosshairFieldMeta[] = [
  {
    key: 'length',
    label: '长度',
    tip: '每根准星线的长度，单位是像素（按基准屏幕高度等比缩放）',
    min: 0,
    max: 64,
    step: 1,
    decimals: 0,
    group: 'size',
  },
  {
    key: 'thickness',
    label: '粗细',
    tip: '线（或圆环）的厚度，单位是像素；0 会按 1 像素渲染',
    min: 0,
    max: 32,
    step: 1,
    decimals: 0,
    group: 'size',
  },
  {
    key: 'gap',
    label: '间距',
    tip: '中心到线内边的单边距离，单位是像素；负值会让线在中心交叠',
    min: -10,
    max: 64,
    step: 1,
    decimals: 0,
    group: 'size',
  },
  {
    key: 'scope_dot_scale',
    label: '瞄准镜中心点大小',
    tip: '开镜时中心点的大小倍率，1 为默认',
    min: 0.1,
    max: 2,
    step: 0.01,
    decimals: 2,
    group: 'advanced',
  },
  {
    key: 'split_dist',
    label: '分裂距离',
    tip: '动态样式在扩散时，内线向外移动的距离',
    min: 0,
    max: 127,
    step: 1,
    decimals: 0,
    group: 'dynamic',
  },
  {
    key: 'inner_alpha',
    label: '内线透明度',
    tip: '扩散时内线的透明度系数，0 完全透明、1 不透明',
    min: 0,
    max: 1,
    step: 0.01,
    decimals: 2,
    group: 'dynamic',
  },
  {
    key: 'outer_alpha',
    label: '外线透明度',
    tip: '扩散时外线的透明度系数，最低 0.3',
    min: 0.3,
    max: 1,
    step: 0.01,
    decimals: 2,
    group: 'dynamic',
  },
  {
    key: 'split_ratio',
    label: '内外线长度比',
    tip: '扩散后内线与外线的长度占比；静态四分用它控制每段扇区的角度',
    min: 0,
    max: 1,
    step: 0.01,
    decimals: 2,
    group: 'dynamic',
  },
  {
    key: 'spread_limit',
    label: '扩散上限',
    tip: '动态元素在 128 像素基线之外最多还能扩散多远',
    min: 0,
    max: 255,
    step: 1,
    decimals: 0,
    group: 'dynamic',
  },
]

/** 按 group 取字段，保持表里的声明顺序。 */
export function fieldsOfGroup(group: CrosshairFieldGroup): CrosshairFieldMeta[] {
  return CROSSHAIR_FIELDS.filter((f) => f.group === group)
}

/** 取单个字段的元数据；键写错时在开发期就抛，而不是渲染成 undefined 范围。 */
export function fieldOf(key: CrosshairNumericKey): CrosshairFieldMeta {
  const found = CROSSHAIR_FIELDS.find((f) => f.key === key)
  if (!found) throw new Error(`未定义的准星数值字段: ${key}`)
  return found
}
