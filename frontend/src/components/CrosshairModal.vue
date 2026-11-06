<script setup lang="ts">
/**
 * CS2 准星设置。
 *
 * 版面：顶部一张真实地图场景（准星按真实比例压在上面），下面三栏参数，
 * 页脚放动作。状态信息收进标题栏的小徽标，不再单独占一块面板。
 *
 * 指令不常驻：悬停页脚左侧两个按钮时浮出 tooltip，3 秒圆环倒计时后常驻，
 * 点面板外或右上角 × 才收起。「写入 autoexec.cfg」不触发它。
 *
 * 数据流：打开时从 autoexec.cfg 读现状 → 本地编辑 → 保存时整份写回。
 * 分享码编解码、cfg 写入、预设存取都在 Go 侧，这里只负责调用与展示。
 */
import { ref, watch, computed, onMounted, onUnmounted } from 'vue'
import type { Crosshair, CrosshairPreset, CrosshairState, CrosshairSaveResult, CrosshairCommandPreview } from '../types'
import { CROSSHAIR_STYLES, CROSSHAIR_OUTLINE_MODES, CROSSHAIR_SCREEN_HEIGHTS, CROSSHAIR_DYNAMIC_STYLES } from '../types'
import type { SceneMap } from '../types'
import SvgIcon from './SvgIcon.vue'
import NumericField from './NumericField.vue'
import CrosshairScenePreview from './CrosshairScenePreview.vue'
import { fieldsOfGroup } from '../utils/crosshairFields'
import { useFocusTrap } from '../utils/a11y'
import {
  getCrosshairState,
  saveCrosshair,
  decodeCrosshairCode,
  encodeCrosshairCode,
  exportCrosshairCFG,
  getCrosshairConsole,
  getCrosshairPresets,
  getHiddenCrosshairPresets,
  saveCrosshairPreset,
  deleteCrosshairPreset,
  setCrosshairPresetHidden,
  getCrosshairCommands,
  defaultCrosshair,
} from '../utils/wails'

const props = defineProps<{
  visible: boolean
  accountId: string
  username: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'log-action', message: string): void
  (e: 'saved', result: CrosshairSaveResult): void
}>()

const dialogRef = ref<HTMLElement | null>(null)
useFocusTrap(
  computed(() => props.visible),
  dialogRef
)

const model = ref<Crosshair>(defaultCrosshair())
const loaded = ref<CrosshairState | null>(null)
const presets = ref<CrosshairPreset[]>([])
const hiddenPresets = ref<CrosshairPreset[]>([])
const loading = ref(false)
const saving = ref(false)
const exporting = ref(false)
const importing = ref(false)

const shareCode = ref('')
const commands = ref<CrosshairCommandPreview>({ lines: [], console: '', share: '' })
const importText = ref('')
const showImport = ref(false)
const showHidden = ref(false)
const showAdvanced = ref(false)

const presetName = ref('')
const showSavePreset = ref(false)

const sceneZoom = ref(1)
const previewHeight = ref(1080)
const sceneMap = ref<SceneMap>('dust2')

/** 场景预览可选的地图，顺序按职业比赛出场率排。 */
const SCENE_MAPS: { value: SceneMap; label: string }[] = [
  { value: 'dust2', label: 'Dust II' },
  { value: 'mirage', label: 'Mirage' },
  { value: 'inferno', label: 'Inferno' },
  { value: 'ancient', label: 'Ancient' },
  { value: 'nuke', label: 'Nuke' },
]

const toastMsg = ref('')
const toastType = ref<'success' | 'error'>('success')

// ---- 指令 tooltip ----
const TIP_PIN_MS = 3000
const RING_CIRCUMFERENCE = 2 * Math.PI * 5.5
const tipVisible = ref(false)
const tipPinned = ref(false)
const tipProgress = ref(0)
const tipRef = ref<HTMLElement | null>(null)
let tipRaf = 0
let tipStart = 0

function tickTip(now: number) {
  const p = Math.min(1, (now - tipStart) / TIP_PIN_MS)
  tipProgress.value = p
  if (p >= 1) {
    tipPinned.value = true
    return
  }
  tipRaf = requestAnimationFrame(tickTip)
}

function startTip() {
  if (tipVisible.value) return
  tipVisible.value = true
  tipPinned.value = false
  tipProgress.value = 0
  tipStart = performance.now()
  cancelAnimationFrame(tipRaf)
  tipRaf = requestAnimationFrame(tickTip)
}

/** 倒计时走完之前离开就收起；常驻之后离开不再收。 */
function leaveTip() {
  if (tipPinned.value) return
  hideTip()
}

function hideTip() {
  tipVisible.value = false
  tipPinned.value = false
  tipProgress.value = 0
  cancelAnimationFrame(tipRaf)
}

/** 点 tooltip 与触发按钮之外的任何地方都收起。 */
function onDocPointerDown(e: PointerEvent) {
  if (!tipVisible.value) return
  const target = e.target as Node | null
  if (!target) return
  if (tipRef.value?.contains(target)) return
  if ((target as HTMLElement).closest?.('.tip-trigger')) return
  hideTip()
}

/**
 * Go 的 nil 切片会被 encoding/json 写成 null，前端直接 `.length` 会抛
 * "Cannot read properties of null"，把整个弹窗的渲染搞崩（历史 bug 的根因）。
 * 后端已改成返回空切片，这里再做一层兜底：任何来源的 null 都收敛成空数组。
 */
function asArray<T>(v: T[] | null | undefined): T[] {
  return Array.isArray(v) ? v : []
}

const showToast = (msg: string, type: 'success' | 'error' = 'success') => {
  toastMsg.value = msg
  toastType.value = type
  window.setTimeout(() => {
    if (toastMsg.value === msg) toastMsg.value = ''
  }, 4200)
}

const isDynamicStyle = computed(() => CROSSHAIR_DYNAMIC_STYLES.includes(model.value.style))

const styleLabel = computed(
  () => CROSSHAIR_STYLES.find((s) => s.value === model.value.style)?.label ?? `样式 ${model.value.style}`
)

const foundCount = computed(() => loaded.value?.parsed.found_keys.length ?? 0)
const totalCount = computed(
  () => (loaded.value?.parsed.found_keys.length ?? 0) + (loaded.value?.parsed.missing_keys.length ?? 0)
)

const toggleLabel = computed(() => {
  const parsed = loaded.value?.parsed
  if (!parsed || !parsed.display_toggle_present) return '未设置'
  return parsed.display_toggle
})

const builtinPresets = computed(() => presets.value.filter((p) => p.builtin))
const userPresets = computed(() => presets.value.filter((p) => !p.builtin))

const colorHex = computed({
  get: () => toHex(model.value.color),
  set: (v: string) => {
    const rgb = fromHex(v)
    if (rgb) model.value.color = { ...rgb, a: model.value.color.a }
  },
})

const outlineColorHex = computed({
  get: () => toHex(model.value.outline_color),
  set: (v: string) => {
    const rgb = fromHex(v)
    if (rgb) model.value.outline_color = { ...rgb, a: model.value.outline_color.a }
  },
})

/**
 * 九个数值参数的元数据来自 `utils/crosshairFields.ts`，这里只按块取出来。
 * 范围、步长、小数位、说明文案都在那张表里，模板里每个参数是一行。
 */
const sizeFields = fieldsOfGroup('size')
const advancedFields = fieldsOfGroup('advanced')
const dynamicFields = fieldsOfGroup('dynamic')

function toHex(c: { r: number; g: number; b: number }): string {
  const part = (v: number) => Math.max(0, Math.min(255, Math.round(v))).toString(16).padStart(2, '0')
  return `#${part(c.r)}${part(c.g)}${part(c.b)}`
}

function fromHex(hex: string): { r: number; g: number; b: number } | null {
  const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim())
  if (!m) return null
  const n = parseInt(m[1], 16)
  return { r: (n >> 16) & 0xff, g: (n >> 8) & 0xff, b: n & 0xff }
}

/** 分享码与指令刷新做防抖：拖滑块时不必每一帧都过一趟 Go。 */
/**
 * 拉一次「指令 + 分享码」。两处调用（打开/导入时立即刷新、拖滑块时防抖刷新）
 * 共用同一段逻辑，避免两份实现慢慢走偏。
 */
async function pullCommands(fallbackToEncode: boolean) {
  try {
    const preview = await getCrosshairCommands(model.value)
    commands.value = {
      lines: asArray(preview?.lines),
      console: preview?.console ?? '',
      share: preview?.share ?? '',
    }
    if (commands.value.share) {
      shareCode.value = commands.value.share
    } else if (fallbackToEncode) {
      shareCode.value = await encodeCrosshairCode(model.value)
    }
  } catch {
    // 指令拿不到就保持旧内容；只有防抖路径才把分享码清空（避免显示过期码）
    if (fallbackToEncode) shareCode.value = ''
  }
}

let refreshTimer: number | undefined
function scheduleRefresh() {
  if (refreshTimer) window.clearTimeout(refreshTimer)
  refreshTimer = window.setTimeout(() => pullCommands(true), 350)
}

async function loadState() {
  loading.value = true
  try {
    const state = await getCrosshairState()
    loaded.value = {
      ...state,
      parsed: {
        ...state.parsed,
        found_keys: asArray(state.parsed?.found_keys),
        missing_keys: asArray(state.parsed?.missing_keys),
      },
    }
    model.value = { ...(state.parsed?.crosshair ?? defaultCrosshair()) }
    shareCode.value = state.share_code ?? ''
    await refreshCommands()
  } catch (e) {
    showToast(e instanceof Error ? e.message : '读取准星设置失败', 'error')
  } finally {
    loading.value = false
  }
}

async function refreshCommands() {
  await pullCommands(false)
}

async function loadPresets() {
  try {
    presets.value = asArray(await getCrosshairPresets())
    hiddenPresets.value = asArray(await getHiddenCrosshairPresets())
  } catch {
    presets.value = []
    hiddenPresets.value = []
  }
}

function applyPreset(p: CrosshairPreset) {
  // 预设自带参考高（职业选手在不同分辨率下调的），一并套用才还原真实观感。
  model.value = { ...p.crosshair }
  showToast(`已套用「${p.name}」`)
}

async function hidePreset(p: CrosshairPreset) {
  try {
    await setCrosshairPresetHidden(p.id, true)
    await loadPresets()
    showToast(`已隐藏预设「${p.name}」`)
  } catch (e) {
    showToast(e instanceof Error ? e.message : '隐藏失败', 'error')
  }
}

async function restorePreset(p: CrosshairPreset) {
  try {
    await setCrosshairPresetHidden(p.id, false)
    await loadPresets()
    showToast(`已恢复预设「${p.name}」`)
  } catch (e) {
    showToast(e instanceof Error ? e.message : '恢复失败', 'error')
  }
}

async function removeUserPreset(p: CrosshairPreset) {
  try {
    await deleteCrosshairPreset(p.id)
    await loadPresets()
    showToast(`已删除预设「${p.name}」`)
  } catch (e) {
    showToast(e instanceof Error ? e.message : '删除失败', 'error')
  }
}

async function handleSavePreset() {
  const name = presetName.value.trim()
  if (!name) {
    showToast('请先给预设起个名字', 'error')
    return
  }
  try {
    await saveCrosshairPreset(name, model.value)
    presetName.value = ''
    showSavePreset.value = false
    await loadPresets()
    showToast(`已保存预设「${name}」`)
  } catch (e) {
    showToast(e instanceof Error ? e.message : '保存预设失败', 'error')
  }
}

async function handleImport() {
  const code = importText.value.trim()
  if (!code) {
    showToast('请先粘贴分享码', 'error')
    return
  }
  if (importing.value) return
  importing.value = true
  try {
    const decoded = await decodeCrosshairCode(props.accountId, code)
    model.value = { ...decoded }
    importText.value = ''
    showImport.value = false
    await refreshCommands()
    if (decoded.format !== 'cs2-v1') {
      showToast('已导入旧版分享码，尺寸按 1080 基准近似换算，请核对预览')
    } else {
      showToast('已导入分享码')
    }
  } catch (e) {
    // 失败时保留输入框里的内容，方便用户改一个字符重试。
    showToast(e instanceof Error ? e.message : '分享码解析失败', 'error')
  } finally {
    importing.value = false
  }
}

async function copyText(text: string, okMsg: string) {
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    showToast(okMsg)
  } catch {
    showToast('复制失败，请手动选中复制', 'error')
  }
}

async function handleCopyShareCode() {
  shareCode.value = await encodeCrosshairCode(model.value)
  await copyText(shareCode.value, '分享码已复制')
}

async function handleCopyConsole() {
  const line = commands.value.console || (await getCrosshairConsole(model.value))
  await copyText(line, '控制台指令已复制，可直接粘进游戏控制台')
}

async function handleExport() {
  exporting.value = true
  try {
    const path = await exportCrosshairCFG(model.value)
    showToast(`已导出 crosshair.cfg：${path}`)
    emit('log-action', `导出准星配置到 ${path}`)
  } catch (e) {
    showToast(e instanceof Error ? e.message : '导出失败', 'error')
  } finally {
    exporting.value = false
  }
}

async function handleSave() {
  saving.value = true
  try {
    const res = await saveCrosshair(model.value)
    shareCode.value = res.share_code
    showToast('准星已写入 autoexec.cfg')
    emit('log-action', `准星已写入 autoexec.cfg（${res.share_code}）`)
    // 交给视图层弹软件内的侧边提示，与其它操作的反馈保持一致。
    emit('saved', res)
    await loadState()
  } catch (e) {
    showToast(e instanceof Error ? e.message : '写入失败', 'error')
  } finally {
    saving.value = false
  }
}

function resetToDefault() {
  model.value = defaultCrosshair()
  showToast('已恢复为默认准星')
}

const close = () => {
  hideTip()
  emit('close')
}

const handleKeydown = (e: KeyboardEvent) => {
  if (e.key === 'Escape' && props.visible) close()
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
  document.addEventListener('pointerdown', onDocPointerDown, true)
  loadPresets()
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
  document.removeEventListener('pointerdown', onDocPointerDown, true)
  if (refreshTimer) window.clearTimeout(refreshTimer)
  cancelAnimationFrame(tipRaf)
})

watch(
  () => props.visible,
  (v) => {
    if (v) {
      loadState()
      previewHeight.value = 1080
      sceneZoom.value = 1
    } else {
      hideTip()
    }
  },
  { immediate: true }
)

watch(model, scheduleRefresh, { deep: true })
</script>

<template>
  <div v-if="visible" class="modal-backdrop" @click.self="close">
    <div
      ref="dialogRef"
      class="winui-dialog"
      role="dialog"
      aria-modal="true"
      aria-labelledby="crosshair-dialog-title"
      tabindex="-1"
    >
      <div class="dialog-header">
        <div class="header-left">
          <div class="header-icon-box">
            <SvgIcon name="sliders" :size="17" />
          </div>
          <div class="header-titles">
            <div class="header-title-row">
              <h3 id="crosshair-dialog-title" class="modal-title">准星设置</h3>
              <span class="user-badge mono-font">{{ username || '未选择账号' }}</span>
              <span class="scope-badge">全局设置</span>
            </div>
            <p class="modal-subtitle">写入 game/csgo/cfg/autoexec.cfg，重启 CS2 生效，或控制台输入 exec autoexec 生效</p>
          </div>
        </div>
        <div class="header-right">
          <span class="status-chip" :class="{ warning: loaded && !loaded.exists }">
            <SvgIcon :name="loaded && !loaded.exists ? 'alert-triangle' : 'check'" :size="11" />
            <span v-if="loading">读取中…</span>
            <span v-else-if="loaded && !loaded.exists">autoexec.cfg 待创建</span>
            <span v-else>已读取 {{ foundCount }}/{{ totalCount }} 项</span>
          </span>
          <span class="status-chip" title="准星显示开关由 autoexec.cfg 决定，工具只读不改">
            <SvgIcon name="eye" :size="11" />
            <span>crosshair {{ toggleLabel }}</span>
          </span>
          <button class="winui-close-btn" title="关闭 (Esc)" @click="close">
            <SvgIcon name="close" :size="14" />
          </button>
        </div>
      </div>

      <transition name="toast-fade">
        <div v-if="toastMsg" class="toast-banner" :class="toastType">
          <SvgIcon :name="toastType === 'success' ? 'check' : 'alert-triangle'" :size="14" />
          <span>{{ toastMsg }}</span>
        </div>
      </transition>

      <!-- 真实地图场景 + 真实比例准星 -->
      <div class="scene-strip">
        <CrosshairScenePreview
          v-model:zoom="sceneZoom"
          :crosshair="model"
          :height="196"
          :current-height="previewHeight"
          :map="sceneMap"
        />
        <div class="scene-toolbar">
          <div class="segmented">
            <button
              v-for="z in [1, 2, 4, 8]"
              :key="z"
              type="button"
              class="segment"
              :class="{ active: sceneZoom === z }"
              @click="sceneZoom = z"
            >
              {{ z }}x
            </button>
          </div>
          <div class="segmented">
            <button
              v-for="m in SCENE_MAPS"
              :key="m.value"
              type="button"
              class="segment"
              :class="{ active: sceneMap === m.value }"
              @click="sceneMap = m.value"
            >
              {{ m.label }}
            </button>
          </div>
          <select v-model.number="previewHeight" class="select-compact" title="模拟分辨率">
            <option v-for="h in CROSSHAIR_SCREEN_HEIGHTS" :key="h" :value="h">
              {{ Math.round((h * 16) / 9) }} × {{ h }}
            </option>
          </select>
        </div>
      </div>

      <div class="dialog-body">
        <!-- 栏一：预设 -->
        <section class="param-col">
          <div class="param-group">
            <div class="group-head">
              <h4 class="group-title">职业选手预设</h4>
              <button type="button" class="btn-link" @click="resetToDefault">恢复默认</button>
            </div>
            <div class="preset-grid">
              <div v-for="p in builtinPresets" :key="p.id" class="preset-chip" :title="p.description">
                <button type="button" class="preset-apply" @click="applyPreset(p)">{{ p.name }}</button>
                <button type="button" class="preset-action" title="隐藏这个预设" @click.stop="hidePreset(p)">
                  <SvgIcon name="close" :size="9" />
                </button>
              </div>
              <span v-if="!builtinPresets.length" class="group-hint">职业选手预设已全部隐藏</span>
            </div>
            <div v-if="hiddenPresets.length" class="hidden-row">
              <button type="button" class="btn-link" @click="showHidden = !showHidden">
                {{ showHidden ? '收起' : `已隐藏 ${hiddenPresets.length} 个` }}
              </button>
              <div v-if="showHidden" class="preset-grid">
                <div v-for="p in hiddenPresets" :key="p.id" class="preset-chip muted">
                  <span class="preset-apply static">{{ p.name }}</span>
                  <button type="button" class="preset-action" title="恢复这个预设" @click.stop="restorePreset(p)">
                    <SvgIcon name="plus" :size="9" />
                  </button>
                </div>
              </div>
            </div>
          </div>

          <div class="param-group">
            <div class="group-head">
              <h4 class="group-title">我的预设</h4>
              <button type="button" class="btn-link" @click="showSavePreset = !showSavePreset">
                {{ showSavePreset ? '收起' : '存为预设' }}
              </button>
            </div>
            <div v-if="showSavePreset" class="save-preset-row">
              <input
                v-model="presetName"
                class="input-control"
                placeholder="预设名称"
                maxlength="40"
                @keyup.enter="handleSavePreset"
              />
              <button type="button" class="btn btn-primary btn-sm" @click="handleSavePreset">保存</button>
            </div>
            <div class="preset-grid">
              <div v-for="p in userPresets" :key="p.id" class="preset-chip" :title="p.description">
                <button type="button" class="preset-apply" @click="applyPreset(p)">{{ p.name }}</button>
                <button type="button" class="preset-action danger" title="删除这个预设" @click.stop="removeUserPreset(p)">
                  <SvgIcon name="trash" :size="9" />
                </button>
              </div>
              <span v-if="!userPresets.length" class="group-hint">还没有自定义预设</span>
            </div>
          </div>
        </section>

        <!-- 栏二：样式与尺寸 -->
        <section class="param-col">
          <div class="param-group">
            <h4 class="group-title">样式</h4>
            <div class="style-grid">
              <button
                v-for="s in CROSSHAIR_STYLES"
                :key="s.value"
                type="button"
                class="style-tile"
                :class="{ active: model.style === s.value }"
                :title="s.label"
                @click="model.style = s.value"
              >
                {{ s.short }}
              </button>
            </div>
            <p class="group-hint">{{ styleLabel }} · cl_crosshairstyle {{ model.style }}</p>
          </div>

          <div class="param-group">
            <h4 class="group-title">尺寸</h4>
            <NumericField
              v-for="f in sizeFields"
              :key="f.key"
              v-model="model[f.key]"
              :label="f.label"
              :tip="f.tip"
              :min="f.min"
              :max="f.max"
              :step="f.step"
              :decimals="f.decimals"
            />
            <p class="group-hint">间距是「中心到线内边」的单边距离，允许负值</p>
          </div>

          <div class="param-group">
            <h4 class="group-title">形状</h4>
            <div class="switch-row">
              <label class="switch-item"><input v-model="model.dot" type="checkbox" /><span>中心点</span></label>
              <label class="switch-item"><input v-model="model.t_style" type="checkbox" /><span>T 型</span></label>
              <label class="switch-item"><input v-model="model.follow_recoil" type="checkbox" /><span>跟随后坐力</span></label>
            </div>
          </div>
        </section>

        <!-- 栏三：颜色、描边、高级、分享码 -->
        <section class="param-col">
          <div class="param-group">
            <h4 class="group-title">颜色</h4>
            <div class="color-row">
              <input v-model="colorHex" type="color" class="color-input" title="点击取色；右侧 R/G/B/A 可直接填数值" />
              <div class="rgba-inputs">
                <label class="mini-field"><span>R</span><input v-model.number="model.color.r" type="number" min="0" max="255" /></label>
                <label class="mini-field"><span>G</span><input v-model.number="model.color.g" type="number" min="0" max="255" /></label>
                <label class="mini-field"><span>B</span><input v-model.number="model.color.b" type="number" min="0" max="255" /></label>
                <label class="mini-field"><span>A</span><input v-model.number="model.color.a" type="number" min="0" max="255" /></label>
              </div>
            </div>
          </div>

          <div class="param-group">
            <h4 class="group-title">描边</h4>
            <div class="segmented wide">
              <button
                v-for="m in CROSSHAIR_OUTLINE_MODES"
                :key="m.value"
                type="button"
                class="segment"
                :class="{ active: model.outline_mode === m.value }"
                @click="model.outline_mode = m.value"
              >
                {{ m.label }}
              </button>
            </div>
            <div v-if="model.outline_mode > 0" class="color-row">
              <input v-model="outlineColorHex" type="color" class="color-input" title="描边颜色，与准星颜色相互独立" />
              <div class="rgba-inputs">
                <label class="mini-field"><span>R</span><input v-model.number="model.outline_color.r" type="number" min="0" max="255" /></label>
                <label class="mini-field"><span>G</span><input v-model.number="model.outline_color.g" type="number" min="0" max="255" /></label>
                <label class="mini-field"><span>B</span><input v-model.number="model.outline_color.b" type="number" min="0" max="255" /></label>
                <label class="mini-field"><span>A</span><input v-model.number="model.outline_color.a" type="number" min="0" max="255" /></label>
              </div>
            </div>
          </div>

          <div class="param-group">
            <button type="button" class="advanced-toggle" @click="showAdvanced = !showAdvanced">
              <SvgIcon :name="showAdvanced ? 'chevron-down' : 'chevron-right'" :size="12" />
              <span>高级</span>
              <span class="advanced-summary mono-font">基准高 {{ model.screen_height }}</span>
            </button>

            <template v-if="showAdvanced">
              <div class="field-inline">
                <span class="field-label">基准屏幕高度</span>
                <select
                  v-model.number="model.screen_height"
                  class="select-compact"
                  title="你是在哪个屏幕高度下调的准星；游戏按当前分辨率等比缩放"
                >
                  <option v-for="h in CROSSHAIR_SCREEN_HEIGHTS" :key="h" :value="h">{{ h }}</option>
                </select>
              </div>
              <NumericField
                v-for="f in advancedFields"
                :key="f.key"
                v-model="model[f.key]"
                :label="f.label"
                :tip="f.tip"
                :min="f.min"
                :max="f.max"
                :step="f.step"
                :decimals="f.decimals"
              />
              <label class="switch-item">
                <input v-model="model.scope_dot_use_color" type="checkbox" />
                <span>瞄准镜中心点使用准星颜色</span>
              </label>

              <template v-if="isDynamicStyle">
                <NumericField
                  v-for="f in dynamicFields"
                  :key="f.key"
                  v-model="model[f.key]"
                  :label="f.label"
                  :tip="f.tip"
                  :min="f.min"
                  :max="f.max"
                  :step="f.step"
                  :decimals="f.decimals"
                />
              </template>
            </template>
          </div>

          <div class="param-group">
            <div class="group-head">
              <h4 class="group-title">分享码</h4>
              <button type="button" class="btn-link" @click="showImport = !showImport">
                {{ showImport ? '收起' : '导入' }}
              </button>
            </div>
            <div class="share-row">
              <input :value="shareCode" readonly class="input-control mono-font share-input" />
              <button type="button" class="btn btn-secondary btn-sm" @click="handleCopyShareCode">复制</button>
            </div>
            <div v-if="showImport" class="import-row">
              <input
                v-model="importText"
                class="input-control mono-font"
                placeholder="CS… 或 CSGO-xxxxx-…"
                @keyup.enter="handleImport"
              />
              <button type="button" class="btn btn-primary btn-sm" :disabled="importing" @click="handleImport">
                <span v-if="importing" class="spinner-small" />
                <span v-else>导入</span>
              </button>
            </div>
          </div>
        </section>
      </div>

      <!-- 指令 tooltip：悬停页脚左侧两个按钮时浮出，3 秒倒计时后常驻 -->
      <transition name="tip-fade">
        <div v-if="tipVisible" ref="tipRef" class="cmd-tip" role="tooltip">
          <div class="tip-head">
            <span class="tip-title">当前准星控制台指令（{{ commands.lines.length }} 条）</span>
            <div class="tip-head-right">
              <span class="tip-countdown" :class="{ pinned: tipPinned }">
                <svg width="14" height="14" viewBox="0 0 14 14" aria-hidden="true">
                  <circle cx="7" cy="7" r="5.5" fill="none" stroke="rgba(255,255,255,0.18)" stroke-width="2" />
                  <circle
                    cx="7"
                    cy="7"
                    r="5.5"
                    fill="none"
                    stroke="var(--accent-light)"
                    stroke-width="2"
                    :stroke-dasharray="RING_CIRCUMFERENCE"
                    :stroke-dashoffset="RING_CIRCUMFERENCE * (1 - tipProgress)"
                    transform="rotate(-90 7 7)"
                  />
                </svg>
                <span>{{ tipPinned ? '已常驻' : '常驻中…' }}</span>
              </span>
              <button type="button" class="tip-close" title="关闭" @click="hideTip">
                <SvgIcon name="close" :size="10" />
              </button>
            </div>
          </div>
          <div class="tip-body mono-font">
            <div v-for="line in commands.lines" :key="line.key" class="tip-row">
              <span class="tip-key">{{ line.key }}</span>
              <span class="tip-value">{{ line.value }}</span>
            </div>
          </div>
          <div class="tip-foot">
            <span class="tip-hint">点面板外或右上角关闭</span>
            <button type="button" class="btn btn-secondary btn-sm" @click="handleCopyConsole">复制整串</button>
          </div>
        </div>
      </transition>

      <div class="dialog-footer">
        <div class="footer-left">
          <button
            type="button"
            class="btn btn-secondary tip-trigger"
            @mouseenter="startTip"
            @mouseleave="leaveTip"
            @click="handleCopyConsole"
          >
            <SvgIcon name="terminal" :size="13" />
            <span>复制控制台指令</span>
          </button>
          <button
            type="button"
            class="btn btn-secondary tip-trigger"
            @mouseenter="startTip"
            @mouseleave="leaveTip"
            @click="handleExport"
          >
            <SvgIcon name="download" :size="13" />
            <span>导出 crosshair.cfg</span>
          </button>
        </div>
        <div class="footer-right">
          <button type="button" class="btn btn-secondary" @click="close">关闭</button>
          <button type="button" class="btn btn-primary" :disabled="saving || loading" @click="handleSave">
            <span v-if="saving" class="spinner-small" />
            <span v-else>写入 autoexec.cfg</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  background-color: rgba(10, 12, 16, 0.72);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1400;
  padding: 16px;
}

.winui-dialog {
  position: relative;
  display: flex;
  flex-direction: column;
  width: 1060px;
  max-width: 96vw;
  /* 高度跟内容走：内容不满时不留一大片空白，超了才滚动。 */
  height: auto;
  max-height: 90vh;
  background-color: var(--bg-surface, #262626);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg, 8px);
  box-shadow: var(--shadow-dialog, 0 16px 32px rgba(0, 0, 0, 0.36));
  overflow: hidden;
  animation: dialog-pop 0.18s cubic-bezier(0, 0, 0, 1);
}

@keyframes dialog-pop {
  from {
    opacity: 0;
    transform: scale(0.97);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}

/* ---- 标题栏：状态收成两个小徽标，不再单独占一块 ---- */
.dialog-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 12px 18px;
  border-bottom: 1px solid var(--border-color);
  background: var(--surface-1);
  flex-shrink: 0;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
  min-width: 0;
}

.header-icon-box {
  width: 34px;
  height: 34px;
  border-radius: var(--radius-md, 6px);
  background: var(--accent-subtle, rgba(0, 120, 212, 0.12));
  border: 1px solid var(--tone-accent-mid);
  color: var(--accent, #0078d4);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.header-titles {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}

.header-title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.modal-title {
  letter-spacing: -0.2px;
}

.modal-subtitle {
  font-size: 11.5px;
  color: var(--color-text-muted);
  margin: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.user-badge,
.scope-badge {
  display: inline-flex;
  align-items: center;
  padding: 1px 7px;
  border-radius: var(--radius-pill);
  background: var(--surface-3);
  border: 1px solid var(--stroke-1);
  font-size: 10.5px;
  color: var(--color-text-secondary);
  white-space: nowrap;
}

.scope-badge {
  color: var(--accent-light);
  background: var(--tone-info-weak);
  border-color: var(--tone-info-stroke);
}

.header-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.status-chip {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: var(--radius-pill);
  background: var(--surface-2);
  border: 1px solid var(--stroke-1);
  font-size: 10.5px;
  color: var(--color-text-muted);
  white-space: nowrap;
}

.status-chip.warning {
  color: var(--level-warning);
  background: var(--status-warning-fill);
  border-color: var(--status-warning-stroke);
}

.winui-close-btn {
  width: 30px;
  height: 30px;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: var(--radius-control);
  background: transparent;
  color: var(--color-text-secondary);
  cursor: pointer;
  transition: var(--transition-fast);
}

.winui-close-btn:hover {
  background: var(--surface-4);
  color: var(--color-text);
}

.toast-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 8px 18px 0;
  padding: 7px 12px;
  border-radius: var(--radius-control);
  font-size: 12px;
  flex-shrink: 0;
}

.toast-banner.success {
  background: var(--status-success-fill);
  border: 1px solid var(--status-success-stroke);
  color: var(--level-success);
}

.toast-banner.error {
  background: var(--status-error-fill);
  border: 1px solid var(--status-error-stroke);
  color: var(--level-error);
}

.toast-fade-enter-active,
.toast-fade-leave-active,
.tip-fade-enter-active,
.tip-fade-leave-active {
  transition: opacity var(--duration-normal) var(--ease-standard);
}

.toast-fade-enter-from,
.toast-fade-leave-to,
.tip-fade-enter-from,
.tip-fade-leave-to {
  opacity: 0;
}

/* ---- 场景 ---- */
.scene-strip {
  padding: 10px 18px 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  flex-shrink: 0;
  border-bottom: 1px solid var(--border-color);
}

.scene-toolbar {
  display: flex;
  align-items: center;
  gap: 10px;
}

.scene-toolbar .select-compact {
  margin-left: auto;
}

.segmented {
  display: inline-flex;
  padding: 2px;
  gap: 2px;
  background: var(--surface-2);
  border: 1px solid var(--stroke-1);
  border-radius: var(--radius-control);
}

.segmented.wide {
  width: 100%;
}

.segmented.wide .segment {
  flex: 1;
}

.segment {
  border: none;
  background: transparent;
  color: var(--color-text-secondary);
  font-size: 11.5px;
  padding: 3px 9px;
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: var(--transition-fast);
  white-space: nowrap;
}

.segment:hover {
  background: var(--surface-4);
  color: var(--color-text);
}

.segment.active {
  background: var(--accent);
  color: var(--accent-text);
}

.select-compact {
  height: 24px;
  padding: 0 6px;
  background: var(--bg-control);
  border: 1px solid var(--border-control);
  border-radius: var(--radius-control);
  color: var(--color-text);
  font-size: 11.5px;
  color-scheme: dark;
}

/* ---- 三栏参数 ---- */
.dialog-body {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 22px;
  flex: 0 1 auto;
  min-height: 0;
  padding: 14px 18px 16px;
  overflow-y: auto;
  align-content: start;
}

.param-col {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}

.param-group {
  display: flex;
  flex-direction: column;
  gap: 7px;
}

.group-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.group-title {
  margin: 0;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
  letter-spacing: 0.3px;
  text-transform: uppercase;
}

.group-hint {
  margin: 0;
  font-size: 10.5px;
  color: var(--color-text-muted);
  line-height: 14px;
}

.btn-link {
  border: none;
  background: transparent;
  color: var(--accent-light);
  font-size: 11px;
  cursor: pointer;
  padding: 0;
  white-space: nowrap;
}

.btn-link:hover {
  text-decoration: underline;
}

.preset-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}

.preset-chip {
  display: inline-flex;
  align-items: stretch;
  background: var(--surface-2);
  border: 1px solid var(--stroke-1);
  border-radius: var(--radius-pill);
  overflow: hidden;
  transition: var(--transition-fast);
}

.preset-chip:hover {
  background: var(--surface-4);
  border-color: var(--stroke-3);
}

.preset-chip.muted {
  opacity: 0.6;
}

.preset-apply {
  border: none;
  background: transparent;
  color: var(--color-text-secondary);
  font-size: 11px;
  padding: 4px 3px 4px 9px;
  cursor: pointer;
  white-space: nowrap;
}

.preset-chip:hover .preset-apply {
  color: var(--color-text);
}

.preset-apply.static {
  cursor: default;
  padding-right: 3px;
}

.preset-action {
  border: none;
  background: transparent;
  color: var(--color-text-muted);
  padding: 0 6px 0 2px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  opacity: 0;
  transition: var(--transition-fast);
}

.preset-chip:hover .preset-action {
  opacity: 1;
}

.preset-action:hover,
.preset-action.danger:hover {
  color: var(--level-error);
}

.hidden-row {
  display: flex;
  flex-direction: column;
  gap: 5px;
}

.save-preset-row {
  display: flex;
  gap: 6px;
}

.save-preset-row .input-control {
  flex: 1;
  min-width: 0;
  font-size: 11.5px;
  height: 28px;
}

.style-grid {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 5px;
}

.style-tile {
  padding: 7px 2px;
  font-size: 10.5px;
  line-height: 13px;
  color: var(--color-text-secondary);
  background: var(--surface-2);
  border: 1px solid var(--stroke-1);
  border-radius: var(--radius-control);
  cursor: pointer;
  transition: var(--transition-fast);
}

.style-tile:hover {
  background: var(--surface-4);
  border-color: var(--stroke-3);
  color: var(--color-text);
}

.style-tile.active {
  background: var(--accent-subtle);
  border-color: var(--accent);
  color: var(--accent-light);
  font-weight: 600;
}

/* 数值参数的「标签 + 数字框 + 滑杆」已收进 NumericField.vue，
   这里的 .field-label 只服务下面「基准屏幕高度」那一行。 */
.field-label {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 11px;
  color: var(--color-text-secondary);
}

.field-inline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  font-size: 11px;
  color: var(--color-text-secondary);
}

.switch-row {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
}

.switch-item {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 11.5px;
  color: var(--color-text-secondary);
  cursor: pointer;
}

.switch-item input {
  accent-color: var(--accent);
  cursor: pointer;
}

.color-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.color-input {
  width: 36px;
  height: 28px;
  padding: 2px;
  background: var(--bg-control);
  border: 1px solid var(--border-control);
  border-radius: var(--radius-control);
  cursor: pointer;
  flex-shrink: 0;
}

.rgba-inputs {
  display: flex;
  gap: 4px;
  flex: 1;
}

.mini-field {
  display: flex;
  align-items: center;
  gap: 3px;
  flex: 1;
  font-size: 10px;
  color: var(--color-text-muted);
}

.mini-field input {
  width: 100%;
  min-width: 0;
  height: 24px;
  padding: 0 4px;
  background: var(--bg-control);
  border: 1px solid var(--border-control);
  border-radius: var(--radius-control);
  color: var(--color-text);
  font-size: 11px;
  font-family: var(--font-mono);
}

.advanced-toggle {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--color-text-secondary);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.3px;
  text-transform: uppercase;
  cursor: pointer;
}

.advanced-toggle:hover {
  color: var(--color-text);
}

.advanced-summary {
  margin-left: auto;
  font-size: 10.5px;
  font-weight: 400;
  color: var(--color-text-muted);
  text-transform: none;
  letter-spacing: 0;
}

.share-row,
.import-row {
  display: flex;
  gap: 6px;
}

.share-input,
.import-row .input-control {
  flex: 1;
  min-width: 0;
  font-size: 11px;
  height: 28px;
}

/* ---- 指令 tooltip ---- */
.cmd-tip {
  position: absolute;
  left: 50%;
  bottom: 58px;
  transform: translateX(-50%);
  width: min(620px, calc(100% - 36px));
  background: var(--bg-surface-alt, #2d2d2d);
  border: 1px solid var(--stroke-3);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-flyout, 0 8px 16px rgba(0, 0, 0, 0.28));
  z-index: 20;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.tip-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 7px 12px;
  border-bottom: 1px solid var(--stroke-1);
  background: var(--surface-1);
}

.tip-title {
  font-size: 11.5px;
  font-weight: 600;
  color: var(--color-text);
}

.tip-head-right {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tip-countdown {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 10.5px;
  color: var(--color-text-muted);
}

.tip-countdown.pinned {
  color: var(--level-success);
}

.tip-close {
  width: 18px;
  height: 18px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--color-text-muted);
  cursor: pointer;
}

.tip-close:hover {
  background: var(--surface-4);
  color: var(--color-text);
}

.tip-body {
  max-height: 210px;
  overflow-y: auto;
  padding: 6px 12px;
  font-size: 11px;
  line-height: 17px;
  user-select: text;
}

.tip-row {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  white-space: nowrap;
}

.tip-key {
  color: #93c5fd;
}

.tip-value {
  color: var(--color-text);
  font-variant-numeric: tabular-nums;
}

.tip-foot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 6px 12px;
  border-top: 1px solid var(--stroke-1);
}

.tip-hint {
  font-size: 10.5px;
  color: var(--color-text-muted);
}

.dialog-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 11px 18px;
  border-top: 1px solid var(--border-color);
  background: var(--surface-1);
  flex-shrink: 0;
}

.footer-left,
.footer-right {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
