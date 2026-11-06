<script setup lang="ts">
/**
 * 准星场景预览。
 *
 * 背景用三张真实地图参考图（dust2 / ancient / nuke，取自 B 站 cstool 的实现），
 * 每张按固定的 1080×288 区域裁切后等比铺满画布。准星按**真实比例**绘制：
 * 与背景图同一缩放系数，所以准星相对场景的大小就是游戏里的样子。
 *
 * 只做放大，不做拖动——准星永远压在画面中心，放大是为了看清那几个像素。
 */
import { ref, watch, onMounted, onUnmounted, computed } from 'vue'
import { CROSSHAIR_ROUND_STYLES, type Crosshair, type SceneMap } from '../types'
import { buildGeometry, rgbaString, computeScale, scaleSize, type Shape } from '../utils/crosshairGeometry'
import SvgIcon from './SvgIcon.vue'

import dust2Img from '../assets/maps/game-reference-dust2.png'
import ancientImg from '../assets/maps/game-reference-ancient.png'
import nukeImg from '../assets/maps/game-reference-nuke.png'
import mirageImg from '../assets/maps/game-reference-mirage.png'
import infernoImg from '../assets/maps/game-reference-inferno.png'

/**
 * 每张参考图里的有效区域。
 *
 * 三张图都留了白边，真正的画面是 1080×288 的一块，坐标取自 cstool 的
 * `me` 表，改动前请先确认裁切位置。
 */
const CROP: Record<SceneMap, { src: string; x: number; y: number; w: number; h: number }> = {
  dust2: { src: dust2Img, x: 92, y: 85, w: 1080, h: 288 },
  ancient: { src: ancientImg, x: 108, y: 98, w: 1080, h: 288 },
  nuke: { src: nukeImg, x: 110, y: 84, w: 1080, h: 288 },
  // 这两张是从工具截图里裁出来的：已经去掉箭头、底部控制条与边框，并统一缩放到 1080 宽，
  // 所以裁切起点是 0、整张即有效区域。宽度必须也是 1080，否则准星比例会与上面三张不一致。
  mirage: { src: mirageImg, x: 0, y: 0, w: 1080, h: 274 },
  inferno: { src: infernoImg, x: 0, y: 0, w: 1080, h: 274 },
}

const props = withDefaults(
  defineProps<{
    crosshair: Crosshair
    /** 视口高度（CSS 像素）。宽度自适应容器。 */
    height?: number
    /** 模拟的游戏画面高度，决定准星的缩放比例。 */
    currentHeight?: number
    /** 缩放倍数，1 = 真实比例。 */
    zoom?: number
    /** 用哪张地图做背景。 */
    map?: SceneMap
  }>(),
  {
    height: 220,
    currentHeight: 1080,
    zoom: 1,
    map: 'dust2',
  }
)

const emit = defineEmits<{
  (e: 'update:zoom', value: number): void
}>()

const canvasRef = ref<HTMLCanvasElement | null>(null)
const wrapRef = ref<HTMLElement | null>(null)
const measuredWidth = ref(600)
let ro: ResizeObserver | null = null

/** 已加载的图片缓存，避免切地图时闪白。 */
const images = new Map<SceneMap, HTMLImageElement>()
const loadedTick = ref(0)

/** 背景图铺满画布后的缩放系数（含用户缩放），准星用同一个系数。 */
const currentScale = ref(1)

function preload() {
  ;(Object.keys(CROP) as SceneMap[]).forEach((key) => {
    if (images.has(key)) return
    const img = new Image()
    img.onload = () => {
      images.set(key, img)
      loadedTick.value++
      render()
    }
    img.src = CROP[key].src
  })
}

/** 准星在游戏像素下的最大直径，用来提示「是不是太小了」。 */
const crosshairExtent = computed(() => {
  const scale = computeScale(props.crosshair.screen_height, props.currentHeight)
  const t = Math.max(1, scaleSize(props.crosshair.thickness, scale))
  const l = scaleSize(props.crosshair.length, scale)
  const g = props.crosshair.gap
  const round = CROSSHAIR_ROUND_STYLES.includes(props.crosshair.style)
  const reach = round ? Math.max(1, g) + t : Math.abs(g) + l + t
  return Math.max(1, reach * 2)
})

const tooSmall = computed(() => crosshairExtent.value * currentScale.value < 24)

const scaleLabel = computed(
  () => `准星实高约 ${Math.round(crosshairExtent.value)} px · 当前 ${props.zoom}x`
)

function drawShape(ctx: CanvasRenderingContext2D, shape: Shape, fill: string) {
  ctx.fillStyle = fill
  ctx.strokeStyle = fill
  if (shape.kind === 'rect') {
    ctx.fillRect(Math.round(shape.x), Math.round(shape.y), Math.round(shape.w), Math.round(shape.h))
    return
  }
  if (shape.kind === 'ring') {
    ctx.beginPath()
    ctx.lineWidth = Math.max(1, Math.round(shape.lineWidth))
    ctx.arc(shape.cx, shape.cy, Math.max(0.5, shape.radius), 0, Math.PI * 2)
    ctx.stroke()
    return
  }
  ctx.beginPath()
  ctx.lineWidth = Math.max(1, Math.round(shape.lineWidth))
  ctx.arc(shape.cx, shape.cy, Math.max(0.5, shape.radius), shape.start, shape.end)
  ctx.stroke()
}

function render() {
  const canvas = canvasRef.value
  if (!canvas) return

  const dpr = window.devicePixelRatio || 1
  const w = measuredWidth.value
  const h = props.height
  canvas.width = Math.round(w * dpr)
  canvas.height = Math.round(h * dpr)
  canvas.style.width = `${w}px`
  canvas.style.height = `${h}px`

  const ctx = canvas.getContext('2d')
  if (!ctx) return
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  ctx.clearRect(0, 0, w, h)
  ctx.imageSmoothingEnabled = true

  const crop = CROP[props.map] ?? CROP.dust2
  const img = images.get(props.map)

  // 铺满画布：取「填满」的缩放系数，多出来的方向居中裁掉，不拉伸。
  const baseScale = Math.max(w / crop.w, h / crop.h)
  const scale = baseScale * props.zoom
  currentScale.value = scale

  ctx.fillStyle = '#101214'
  ctx.fillRect(0, 0, w, h)

  if (img) {
    const dw = crop.w * scale
    const dh = crop.h * scale
    ctx.drawImage(img, crop.x, crop.y, crop.w, crop.h, (w - dw) / 2, (h - dh) / 2, dw, dh)
  }

  // 准星画在画布中心，缩放系数与背景图一致 → 相对大小即游戏里的真实比例。
  const geom = buildGeometry(props.crosshair, {
    viewSize: Math.min(w, h),
    zoom: scale,
    currentHeight: props.currentHeight,
    centerX: w / 2,
    centerY: h / 2,
  })

  const outlineColor = rgbaString(props.crosshair.outline_color)
  geom.outlines.forEach((s) => drawShape(ctx, s, outlineColor))
  const fillColor = rgbaString(props.crosshair.color)
  geom.fills.forEach((s) => drawShape(ctx, s, fillColor))

  // 真实比例下准星可能只有几像素，加一圈极淡的定位环，避免用户找不到它。
  if (tooSmall.value) {
    ctx.beginPath()
    ctx.strokeStyle = 'rgba(255,255,255,0.45)'
    ctx.lineWidth = 1
    ctx.setLineDash([3, 3])
    ctx.arc(w / 2, h / 2, 16, 0, Math.PI * 2)
    ctx.stroke()
    ctx.setLineDash([])
  }
}

function onWheel(e: WheelEvent) {
  e.preventDefault()
  const next = e.deltaY < 0 ? props.zoom * 1.5 : props.zoom / 1.5
  const clamped = Math.min(16, Math.max(1, Number(next.toFixed(2))))
  if (clamped !== props.zoom) emit('update:zoom', clamped)
}

onMounted(() => {
  if (wrapRef.value) {
    measuredWidth.value = wrapRef.value.clientWidth || 600
    ro = new ResizeObserver((entries) => {
      const w = Math.round(entries[0].contentRect.width)
      if (w > 0 && w !== measuredWidth.value) {
        measuredWidth.value = w
        render()
      }
    })
    ro.observe(wrapRef.value)
  }
  preload()
  render()
})

watch(
  () => [props.crosshair, props.height, props.zoom, props.currentHeight, props.map, loadedTick.value],
  render,
  { deep: true }
)

onUnmounted(() => {
  ro?.disconnect()
  ro = null
})
</script>

<template>
  <div ref="wrapRef" class="scene-wrap" :style="{ height: `${height}px` }">
    <canvas
      ref="canvasRef"
      class="scene-canvas"
      title="缩放比例预览：滚轮可缩放，下方切换地图与模拟分辨率"
      @wheel="onWheel"
    />

    <div class="scene-hud">
      <span class="hud-chip mono-font">{{ scaleLabel }}</span>
    </div>

    <div v-if="tooSmall" class="scene-hint">
      <SvgIcon name="search" :size="11" />
      <span>真实比例下准星只有几像素，滚轮或左下角放大</span>
    </div>
  </div>
</template>

<style scoped>
.scene-wrap {
  position: relative;
  border-radius: var(--radius-lg);
  border: 1px solid var(--stroke-2);
  overflow: hidden;
  flex-shrink: 0;
  background: #101214;
}

.scene-canvas {
  display: block;
  cursor: zoom-in;
  touch-action: none;
}

.scene-hud {
  position: absolute;
  left: 8px;
  bottom: 8px;
  display: flex;
  align-items: center;
  gap: 6px;
  pointer-events: none;
}

.hud-chip {
  font-size: 10.5px;
  color: rgba(255, 255, 255, 0.9);
  background: rgba(0, 0, 0, 0.5);
  border-radius: var(--radius-sm);
  padding: 2px 7px;
}

.scene-hint {
  position: absolute;
  right: 8px;
  bottom: 8px;
  display: flex;
  align-items: center;
  gap: 5px;
  font-size: 10.5px;
  color: rgba(255, 255, 255, 0.86);
  background: rgba(0, 0, 0, 0.5);
  border-radius: var(--radius-sm);
  padding: 3px 7px;
  pointer-events: none;
}
</style>
