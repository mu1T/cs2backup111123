/**
 * 准星绘制几何。
 *
 * 纯函数，不碰 canvas，便于单测：把一份 Crosshair 换算成一组可绘制的图形，
 * 组件只负责按顺序画出来。
 *
 * 规格来源：docs/research/cs2-crosshair.md §6（逆向 client.dll 并与 1920×1080
 * 截图逐像素比对的结果）。几个容易画错的点都写在下面的注释里。
 */
import {
  CROSSHAIR_CIRCLE_STYLES,
  CROSSHAIR_CROSS_STYLES,
  CROSSHAIR_DYNAMIC_GAP_STYLES,
  CROSSHAIR_STYLES,
  type Crosshair,
} from '../types'

/** 矩形：横线、竖线、中心点、方形壁都用它。坐标单位是 canvas 像素。 */
export interface RectShape {
  kind: 'rect'
  x: number
  y: number
  w: number
  h: number
}

/** 圆环：静态圆形与动态圆形。 */
export interface RingShape {
  kind: 'ring'
  cx: number
  cy: number
  radius: number
  lineWidth: number
}

/** 扇环：静态四分的四段。角度为弧度。 */
export interface SectorShape {
  kind: 'sector'
  cx: number
  cy: number
  radius: number
  lineWidth: number
  start: number
  end: number
}

export type Shape = RectShape | RingShape | SectorShape

export interface GeometryOptions {
  /** 预览画布边长（CSS 像素）。仅在没有指定 centerX/centerY 时用于推导中心。 */
  viewSize: number
  /** 放大倍数。1 表示 1 个游戏像素 = 1 个 canvas 像素（真实比例）。 */
  zoom: number
  /** 预览所模拟的游戏画面高度，用于和 screenHeight 求比例。 */
  currentHeight: number
  /** 准星中心的 canvas 横坐标，默认 viewSize/2。地图预览里由平移量算出来。 */
  centerX?: number
  /** 准星中心的 canvas 纵坐标，默认 viewSize/2。 */
  centerY?: number
}

export interface CrosshairGeometry {
  /** 尺寸换算比例：当前画面高度 / 准星基准高度。 */
  scale: number
  /** 先画的描边图形。 */
  outlines: Shape[]
  /** 后画的填充图形。 */
  fills: Shape[]
}

/**
 * 尺寸换算比例。
 *
 * 只看高度，宽度不参与——4:3 拉伸下竖线比横线粗是游戏真实行为，不是 bug。
 */
export function computeScale(screenHeight: number, currentHeight: number): number {
  if (screenHeight <= 0 || currentHeight <= 0) return 1
  return currentHeight / screenHeight
}

/** 尺寸类数值换算：大于 0 的值不会被缩成 0。 */
export function scaleSize(value: number, scale: number): number {
  if (value <= 0) return 0
  return Math.max(1, Math.round(value * scale))
}

/**
 * 间距换算。
 *
 * 正数至少 1、负数最多 -1、0 保持 0；取整用 round half to even，
 * 对应游戏里的 cvtss2si 指令。
 */
export function scaleGap(gap: number, scale: number): number {
  if (gap === 0) return 0
  const scaled = Math.round(gap * scale)
  if (gap > 0) return Math.max(1, scaled)
  return Math.min(-1, scaled)
}

/**
 * 计算整份准星的绘制图形。
 *
 * gap 是「中心到线内边」的单边距离，两根线之间的总空隙是 2 × gap。
 * 第三方生成器常按总空隙画，导致预览比实际紧一半——这里按单边算。
 */
export function buildGeometry(ch: Crosshair, opts: GeometryOptions): CrosshairGeometry {
  const { viewSize, zoom, currentHeight } = opts
  const scale = computeScale(ch.screen_height, currentHeight)

  // 先在「游戏像素」里算出真实尺寸……
  const tGame = Math.max(1, scaleSize(ch.thickness, scale))
  const lGame = scaleSize(ch.length, scale)
  const isDynamic = CROSSHAIR_DYNAMIC_GAP_STYLES.includes(ch.style)
  // 动态样式的静止态要给中心点让位，用 0 或 t 而不是用户设的 gap。
  const gapSource = isDynamic ? (ch.dot ? tGame : 0) : ch.gap
  const gGame = isDynamic ? gapSource : scaleGap(gapSource, scale)

  // ……再换算到画布像素并**取整**。
  // 场景预览的缩放系数是「画布宽 / 参考图宽」，常常是 0.93 这种非整数；
  // 不取整的话 1px 的线会横跨两个像素、被抗锯齿糊成两三条灰边，
  // 小准星在 1x 下尤其明显。取整后每条线都落在整像素上，边缘是硬的。
  const t = Math.max(1, Math.round(tGame * zoom))
  const l = lGame > 0 ? Math.max(1, Math.round(lGame * zoom)) : 0
  const g = Math.round(gGame * zoom)

  const center = Math.floor(viewSize / 2)
  const centerX = Math.round(opts.centerX ?? center)
  const centerY = Math.round(opts.centerY ?? center)
  const fills: Shape[] = []
  const outlines: Shape[] = []

  const halfT = Math.floor(t / 2)
  const dotRect: RectShape = {
    kind: 'rect',
    x: centerX - halfT,
    y: centerY - halfT,
    w: t,
    h: t,
  }

  // 中心点也吃描边：CS2 里它是一根独立的「线」，同样有边框。
  const pushDot = () => {
    fills.push(dotRect)
    collectRectOutline(dotRect, ch.outline_mode, zoom, outlines)
  }

  // 中心点画在所有线之下。
  if (ch.dot && ch.style !== 6) {
    pushDot()
  }

  /** 偏移与尺寸都是画布像素，且全部为整数。 */
  const pushRect = (offsetX: number, offsetY: number, w: number, h: number) => {
    const rect: RectShape = {
      kind: 'rect',
      x: centerX + offsetX,
      y: centerY + offsetY,
      w,
      h,
    }
    fills.push(rect)
    collectRectOutline(rect, ch.outline_mode, zoom, outlines)
  }

  const pushRing = (radiusGame: number, lineWidthGame: number) => {
    const ring: RingShape = {
      kind: 'ring',
      cx: centerX,
      cy: centerY,
      radius: Math.round(radiusGame * zoom),
      lineWidth: Math.max(1, Math.round(lineWidthGame * zoom)),
    }
    fills.push(ring)
    collectRingOutline(ring, ch.outline_mode, zoom, outlines)
  }

  if (CROSSHAIR_CROSS_STYLES.includes(ch.style)) {
    // 左右两根：内边距中心 g，长 l，厚 t。
    if (l > 0) {
      pushRect(g, -halfT, l, t) // 右
      pushRect(-g - l, -halfT, l, t) // 左
      // cl_crosshair_t 去掉上面那根。
      if (!ch.t_style) pushRect(-halfT, -g - l, t, l) // 上
      pushRect(-halfT, g, t, l) // 下
    } else {
      // 长度为 0 时只保留中心点。
      if (!ch.dot) pushDot()
    }
  } else if (CROSSHAIR_CIRCLE_STYLES.includes(ch.style)) {
    const radius = isDynamic ? gapSource : Math.max(1, gGame) + tGame
    const lineWidth = isDynamic ? Math.max(1, tGame) : Math.max(1, tGame - 1)
    pushRing(radius, lineWidth)
  } else if (ch.style === 6) {
    // 仅中心点：边长 t 的方块。
    pushDot()
  } else if (ch.style === 8) {
    // 静态方形：边长 2g 的方孔，壁厚 t。
    const hole = Math.max(0, g)
    const span = hole + t
    pushRect(-span, -span, span * 2, t) // 上壁
    pushRect(-span, hole, span * 2, t) // 下壁
    pushRect(-span, -hole, t, hole * 2) // 左壁
    pushRect(hole, -hole, t, hole * 2) // 右壁
  } else if (ch.style === 9) {
    // 静态四分：圆环上四个 90° 扇区，每段占 splitRatio × 90°，以对角线为中心。
    const radius = Math.round((Math.max(1, gGame) + tGame) * zoom)
    const sweep = Math.max(0, Math.min(1, ch.split_ratio)) * (Math.PI / 2)
    for (let i = 0; i < 4; i++) {
      const diagonal = Math.PI / 4 + (i * Math.PI) / 2
      const sector: SectorShape = {
        kind: 'sector',
        cx: centerX,
        cy: centerY,
        radius,
        lineWidth: t,
        start: diagonal - sweep / 2,
        end: diagonal + sweep / 2,
      }
      fills.push(sector)
      collectSectorOutline(sector, ch.outline_mode, zoom, outlines)
    }
  } else {
    // 未知样式：退化为中心点，至少不会画出奇怪的东西。
    pushDot()
  }

  return { scale, outlines, fills }
}

/**
 * 矩形的描边：完整模式四周各外扩 1 像素；半描边只压上方与左侧。
 *
 * 描边要单独收集并先画，否则一根线的描边会盖住另一根线的填充。
 * 厚度是「1 个游戏像素」，所以要跟着 zoom 放大，否则放大预览时描边会比线细一截。
 */
function collectRectOutline(rect: RectShape, mode: number, zoom: number, out: Shape[]) {
  if (mode <= 0) return
  const thickness = Math.max(1, Math.round(zoom))
  // 上
  out.push({ kind: 'rect', x: rect.x - thickness, y: rect.y - thickness, w: rect.w + thickness * 2, h: thickness })
  // 左
  out.push({ kind: 'rect', x: rect.x - thickness, y: rect.y, w: thickness, h: rect.h })
  if (mode >= 2) return
  // 下
  out.push({ kind: 'rect', x: rect.x - thickness, y: rect.y + rect.h, w: rect.w + thickness * 2, h: thickness })
  // 右
  out.push({ kind: 'rect', x: rect.x + rect.w, y: rect.y, w: thickness, h: rect.h })
}

/** 扇环的描边：同样外扩一圈，角度保持与填充一致。 */
function collectSectorOutline(sector: SectorShape, mode: number, zoom: number, out: Shape[]) {
  if (mode <= 0) return
  const edge = Math.max(1, Math.round(zoom))
  out.push({
    kind: 'sector',
    cx: sector.cx,
    cy: sector.cy,
    radius: sector.radius + edge,
    lineWidth: sector.lineWidth + edge * 2,
    start: sector.start,
    end: sector.end,
  })
}

/** 圆环的描边：完整模式外扩一圈；半描边只画上半与左半。 */
function collectRingOutline(ring: RingShape, mode: number, zoom: number, out: Shape[]) {
  if (mode <= 0) return
  const edge = Math.max(1, Math.round(zoom))
  const outer: RingShape = {
    kind: 'ring',
    cx: ring.cx,
    cy: ring.cy,
    radius: ring.radius + edge,
    lineWidth: ring.lineWidth + edge * 2,
  }
  if (mode >= 2) {
    // 半描边：只保留上半与左半，用两个扇环近似。
    out.push({
      kind: 'sector',
      cx: outer.cx,
      cy: outer.cy,
      radius: outer.radius,
      lineWidth: outer.lineWidth,
      start: Math.PI,
      end: Math.PI * 2,
    })
    out.push({
      kind: 'sector',
      cx: outer.cx,
      cy: outer.cy,
      radius: outer.radius,
      lineWidth: outer.lineWidth,
      start: Math.PI / 2,
      end: (Math.PI * 3) / 2,
    })
    return
  }
  out.push(outer)
}

/** 把 0-255 的 alpha 换成 canvas 用的 0-1。 */
export function alphaOf(a: number): number {
  return Math.max(0, Math.min(1, a / 255))
}

/** 生成 CSS 颜色字符串。 */
export function rgbaString(color: { r: number; g: number; b: number; a: number }): string {
  return `rgba(${color.r}, ${color.g}, ${color.b}, ${alphaOf(color.a)})`
}

/** 样式值 → 中文名。直接从 CROSSHAIR_STYLES 派生，不再手抄一份。 */
const STYLE_NAMES: Record<number, string> = Object.fromEntries(
  CROSSHAIR_STYLES.map((s) => [s.value, s.label])
)

/**
 * 一行摘要，用于工作台账号卡的准星条。
 *
 * 只挑用户最关心的四项：样式、长度、粗细、间距，避免把整份设置塞进卡片。
 */
export function describeCrosshair(ch: Crosshair): string {
  const name = STYLE_NAMES[ch.style] ?? `样式 ${ch.style}`
  const parts = [name, `长 ${ch.length}`, `粗 ${ch.thickness}`, `间距 ${ch.gap}`]
  if (ch.dot) parts.push('中心点')
  if (ch.t_style) parts.push('T 型')
  return parts.join(' · ')
}
