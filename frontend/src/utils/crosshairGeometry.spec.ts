import { describe, it, expect } from 'vitest'
import {
  computeScale,
  scaleSize,
  scaleGap,
  buildGeometry,
  describeCrosshair,
  alphaOf,
  rgbaString,
  type RectShape,
} from './crosshairGeometry'
import type { Crosshair } from '../types'

const base = (over: Partial<Crosshair> = {}): Crosshair => ({
  format: 'cs2-v1',
  style: 4,
  follow_recoil: true,
  dot: false,
  t_style: false,
  outline_mode: 0,
  color: { r: 0, g: 255, b: 0, a: 255 },
  outline_color: { r: 0, g: 0, b: 0, a: 255 },
  gap: 4,
  length: 8,
  thickness: 2,
  spread_limit: 255,
  split_dist: 3,
  inner_alpha: 0,
  outer_alpha: 1,
  split_ratio: 1,
  screen_height: 1080,
  scope_dot_scale: 1,
  scope_dot_use_color: false,
  ...over,
})

const opts = { viewSize: 300, zoom: 2, currentHeight: 1080 }

describe('computeScale', () => {
  it('按当前高度与基准高度的比例缩放', () => {
    expect(computeScale(1080, 1080)).toBe(1)
    expect(computeScale(1080, 2160)).toBe(2)
    expect(computeScale(1440, 1080)).toBe(0.75)
  })

  it('基准高度非法时退回 1，不产生 NaN', () => {
    expect(computeScale(0, 1080)).toBe(1)
    expect(computeScale(1080, 0)).toBe(1)
  })
})

describe('scaleSize', () => {
  it('大于 0 的值不会被缩成 0', () => {
    expect(scaleSize(1, 0.1)).toBe(1)
    expect(scaleSize(4, 0.1)).toBe(1)
  })

  it('0 保持 0', () => {
    expect(scaleSize(0, 2)).toBe(0)
  })

  it('按比例四舍五入', () => {
    expect(scaleSize(8, 1)).toBe(8)
    expect(scaleSize(8, 2)).toBe(16)
    expect(scaleSize(7, 0.5)).toBe(4)
  })
})

describe('scaleGap', () => {
  it('0 保持 0', () => {
    expect(scaleGap(0, 3)).toBe(0)
  })

  it('正数至少为 1', () => {
    expect(scaleGap(1, 0.1)).toBe(1)
  })

  it('负数最多为 -1', () => {
    expect(scaleGap(-1, 0.1)).toBe(-1)
  })

  it('按比例取整', () => {
    expect(scaleGap(4, 1)).toBe(4)
    expect(scaleGap(4, 2)).toBe(8)
  })
})

describe('buildGeometry 静态十字', () => {
  it('画四根线', () => {
    const g = buildGeometry(base(), opts)
    expect(g.fills).toHaveLength(4)
    expect(g.fills.every((s) => s.kind === 'rect')).toBe(true)
  })

  it('间距是单边距离，两根线之间是 2×gap', () => {
    const g = buildGeometry(base({ gap: 4, length: 8, thickness: 2 }), opts)
    const rects = g.fills as RectShape[]
    // zoom = 2 → gap 4 → 8px，length 8 → 16px
    const right = rects[0]
    const left = rects[1]
    expect(right.x).toBe(150 + 4 * 2)
    expect(right.w).toBe(16)
    expect(left.x + left.w).toBe(150 - 4 * 2)
    // 两根线之间的空隙 = 2 × gap × zoom
    expect(right.x - (left.x + left.w)).toBe(2 * 4 * 2)
  })

  it('T 型去掉上面那根', () => {
    const normal = buildGeometry(base({ t_style: false }), opts)
    const tStyle = buildGeometry(base({ t_style: true }), opts)
    expect(normal.fills).toHaveLength(4)
    expect(tStyle.fills).toHaveLength(3)
  })

  it('中心点画在填充层的最前面（在线之下）', () => {
    const g = buildGeometry(base({ dot: true }), opts)
    expect(g.fills).toHaveLength(5)
    const dot = g.fills[0] as RectShape
    expect(dot.w).toBe(2 * 2) // thickness × zoom
    expect(dot.h).toBe(2 * 2)
  })

  it('长度为 0 时退化为中心点', () => {
    const g = buildGeometry(base({ length: 0, dot: false }), opts)
    expect(g.fills).toHaveLength(1)
    expect(g.fills[0].kind).toBe('rect')
  })
})

describe('buildGeometry 圆形与方形', () => {
  it('静态圆形画成圆环', () => {
    const g = buildGeometry(base({ style: 3 }), opts)
    expect(g.fills).toHaveLength(1)
    expect(g.fills[0].kind).toBe('ring')
  })

  it('仅中心点只画一个方块', () => {
    const g = buildGeometry(base({ style: 6 }), opts)
    expect(g.fills).toHaveLength(1)
    expect(g.fills[0].kind).toBe('rect')
  })

  it('静态方形画四条壁', () => {
    const g = buildGeometry(base({ style: 8 }), opts)
    expect(g.fills).toHaveLength(4)
  })

  it('静态四分画四个扇区，角度随 split_ratio 收窄', () => {
    const wide = buildGeometry(base({ style: 9, split_ratio: 1 }), opts)
    const narrow = buildGeometry(base({ style: 9, split_ratio: 0.25 }), opts)
    expect(wide.fills).toHaveLength(4)
    expect(wide.fills[0].kind).toBe('sector')
    const wideSweep = (wide.fills[0] as { start: number; end: number }).end - (wide.fills[0] as { start: number; end: number }).start
    const narrowSweep = (narrow.fills[0] as { start: number; end: number }).end - (narrow.fills[0] as { start: number; end: number }).start
    expect(wideSweep).toBeGreaterThan(narrowSweep)
  })
})

describe('buildGeometry 动态样式', () => {
  it('动态十字静止态用「给中心点让位」的间距，不用用户 gap', () => {
    const withDot = buildGeometry(base({ style: 0, gap: 40, dot: true, thickness: 2 }), opts)
    const rects = withDot.fills.filter((s) => s.kind === 'rect') as RectShape[]
    // 中心点在最前，其后第一根线紧贴中心：间距 = thickness
    const right = rects[1]
    expect(right.x).toBe(150 + 2 * 2)
  })

  it('动态圆形缩成一条线（半径为 0 时不画成正常圆环）', () => {
    const g = buildGeometry(base({ style: 1, dot: false }), opts)
    expect(g.fills).toHaveLength(1)
    const ring = g.fills[0] as { radius: number }
    expect(ring.radius).toBe(0)
  })
})

describe('buildGeometry 描边', () => {
  it('无描边时不产生描边图形', () => {
    const g = buildGeometry(base({ outline_mode: 0 }), opts)
    expect(g.outlines).toHaveLength(0)
  })

  it('完整描边给每根线四边各一条', () => {
    const g = buildGeometry(base({ outline_mode: 1 }), opts)
    expect(g.outlines).toHaveLength(4 * 4)
  })

  it('半描边只压上方与左侧', () => {
    const g = buildGeometry(base({ outline_mode: 2 }), opts)
    expect(g.outlines).toHaveLength(4 * 2)
  })

  it('圆形描边画成外扩一圈的圆环', () => {
    const g = buildGeometry(base({ style: 3, outline_mode: 1 }), opts)
    expect(g.outlines).toHaveLength(1)
    expect(g.outlines[0].kind).toBe('ring')
  })
})

describe('颜色工具', () => {
  it('alpha 换算到 0-1 并夹取', () => {
    expect(alphaOf(255)).toBe(1)
    expect(alphaOf(0)).toBe(0)
    expect(alphaOf(300)).toBe(1)
    expect(alphaOf(-10)).toBe(0)
  })

  it('生成 canvas 可用的 rgba 字符串', () => {
    expect(rgbaString({ r: 208, g: 77, b: 6, a: 255 })).toBe('rgba(208, 77, 6, 1)')
  })
})

describe('describeCrosshair', () => {
  it('摘要含样式名与三项尺寸', () => {
    const text = describeCrosshair(base({ style: 4, length: 7, thickness: 2, gap: 4 }))
    expect(text).toContain('静态十字')
    expect(text).toContain('长 7')
    expect(text).toContain('粗 2')
    expect(text).toContain('间距 4')
  })

  it('开关项只在开启时出现', () => {
    expect(describeCrosshair(base())).not.toContain('中心点')
    expect(describeCrosshair(base({ dot: true }))).toContain('中心点')
    expect(describeCrosshair(base({ t_style: true }))).toContain('T 型')
  })

  it('未知样式不抛错', () => {
    expect(describeCrosshair(base({ style: 42 }))).toContain('样式 42')
  })
})

describe('buildGeometry 静态四分的描边', () => {
  it('每个扇区都带自己的外扩描边', () => {
    const g = buildGeometry(base({ style: 9, outline_mode: 1 }), opts)
    expect(g.fills).toHaveLength(4)
    expect(g.outlines).toHaveLength(4)
    expect(g.outlines.every((s) => s.kind === 'sector')).toBe(true)
  })

  it('无描边时不产生扇区描边', () => {
    const g = buildGeometry(base({ style: 9, outline_mode: 0 }), opts)
    expect(g.outlines).toHaveLength(0)
  })
})

describe('buildGeometry 像素对齐（1x 发糊的回归护栏）', () => {
  // 场景预览的缩放系数是「画布宽 / 参考图宽」，常是 0.926 这类非整数。
  // 只要有一个坐标或尺寸带小数，浏览器就会抗锯齿，1px 的线被糊成两三条灰边。
  const oddZoom = { viewSize: 1000, zoom: 0.926, currentHeight: 1080 }

  it('非整数缩放下矩形坐标与尺寸全为整数', () => {
    const g = buildGeometry(base({ gap: 4, length: 8, thickness: 2, outline_mode: 1 }), oddZoom)
    const all = [...g.fills, ...g.outlines]
    expect(all.length).toBeGreaterThan(0)
    for (const s of all) {
      if (s.kind === 'rect') {
        expect(Number.isInteger(s.x)).toBe(true)
        expect(Number.isInteger(s.y)).toBe(true)
        expect(Number.isInteger(s.w)).toBe(true)
        expect(Number.isInteger(s.h)).toBe(true)
      } else {
        expect(Number.isInteger(s.cx)).toBe(true)
        expect(Number.isInteger(s.cy)).toBe(true)
        expect(Number.isInteger(s.radius)).toBe(true)
        expect(Number.isInteger(s.lineWidth)).toBe(true)
      }
    }
  })

  it('左右两根线关于中心对称', () => {
    const g = buildGeometry(base({ gap: 4, length: 8, thickness: 2 }), oddZoom)
    const rects = g.fills as RectShape[]
    const right = rects[0]
    const left = rects[1]
    // viewSize 1000 → 中心 500。左线右端到中心的距离应等于右线左端到中心的距离。
    const centerX = 500
    expect(right.x - centerX).toBe(centerX - (left.x + left.w))
    expect(right.w).toBe(left.w)
  })

  it('线宽至少 1 像素，不会因为缩放变成 0', () => {
    const g = buildGeometry(base({ thickness: 1, length: 2, gap: 0 }), { viewSize: 400, zoom: 0.4, currentHeight: 1080 })
    for (const s of g.fills) {
      if (s.kind === 'rect') {
        expect(s.w).toBeGreaterThanOrEqual(1)
        expect(s.h).toBeGreaterThanOrEqual(1)
      }
    }
  })
})
