<script setup lang="ts">
import { ref, watch, computed, onMounted, onUnmounted } from 'vue'
import type { VideoPreset } from '../types'
import SvgIcon from './SvgIcon.vue'
import ConfirmModal from './ConfirmModal.vue'
import { useFocusTrap } from '../utils/a11y'
import type { DisplayMode } from '../types'
import {
  getVideoSettings,
  getDisplayModes,
  saveVideoSettings,
  selectVideoConfigFile,
  importVideoPreset,
  saveAsVideoPreset,
  getVideoPresets,
  applyVideoPreset,
} from '../utils/wails'

const props = defineProps<{
  visible: boolean
  accountId: string
  username: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'log-action', message: string): void
  (e: 'saved'): void
}>()

const dialogRef = ref<HTMLElement | null>(null)

useFocusTrap(
  computed(() => props.visible),
  dialogRef
)

const loading = ref(false)
const saving = ref(false)
const filePath = ref('')
const isReadOnly = ref(false)
const settings = ref<Record<string, string>>({})
const presets = ref<VideoPreset[]>([])
const selectedPresetId = ref('')
const newPresetName = ref('')
const isSavingPreset = ref(false)
const toastMsg = ref('')
const toastType = ref<'success' | 'error'>('success')

// Resolution helper state
const selectedResKey = ref('1280x960')
const customWidth = ref('')
const customHeight = ref('')

// Refresh rate helper state
const selectedHz = ref('auto')
const displayModes = ref<DisplayMode[]>([])

// 本机真实支持的刷新率（去重降序），最高档单独标记
const availableHz = computed(() => {
  const set = new Set<number>()
  displayModes.value.forEach((m) => {
    if (m.refresh_rate > 0) set.add(m.refresh_rate)
  })
  return [...set].sort((a, b) => b - a)
})

const maxHz = computed(() => availableHz.value[0] || 0)

const showToast = (msg: string, type: 'success' | 'error' = 'success') => {
  toastMsg.value = msg
  toastType.value = type
  setTimeout(() => {
    if (toastMsg.value === msg) {
      toastMsg.value = ''
    }
  }, 3500)
}

const detectedHz = computed(() => {
  const num = Number(settings.value['setting.refreshrate_numerator']) || 0
  const den = Number(settings.value['setting.refreshrate_denominator']) || 1000
  if (num <= 0) return ''
  return num > 1000 ? `${Math.round(num / den)} Hz` : `${num} Hz`
})

// MSAA 与 CMAA2 是两个独立开关，可以同时开启
const msaaMode = computed({
  get: () => settings.value['setting.msaa_samples'] || '0',
  set: (v: string) => {
    settings.value['setting.msaa_samples'] = v
  },
})

const cmaaEnabled = computed({
  get: () => (settings.value['setting.r_csgo_cmaa_enable'] || '0') === '1',
  set: (v: boolean) => {
    settings.value['setting.r_csgo_cmaa_enable'] = v ? '1' : '0'
  },
})

// 显卡信息由引擎写入，这里只做展示
const VENDOR_NAMES: Record<string, string> = {
  '4318': 'NVIDIA',
  '4098': 'AMD',
  '32902': 'Intel',
}

const gpuLabel = computed(() => {
  const vendor = settings.value['VendorID'] || ''
  const device = settings.value['DeviceID'] || ''
  if (!vendor && !device) return '未记录'
  const name = VENDOR_NAMES[vendor] || `厂商 ${vendor}`
  return device ? `${name} · DeviceID ${device}` : name
})

const syncResolutionFromSettings = () => {
  const w = settings.value['setting.defaultres'] || ''
  const h = settings.value['setting.defaultresheight'] || ''
  if (!w || !h) {
    selectedResKey.value = '1280x960'
    return
  }
  const key = `${w}x${h}`
  if (['1280x960', '1024x768', '1440x1080', '1920x1080', '2560x1440', '1680x1050'].includes(key)) {
    selectedResKey.value = key
  } else {
    selectedResKey.value = 'custom'
    customWidth.value = w
    customHeight.value = h
  }
}

const onResolutionChange = () => {
  if (selectedResKey.value === 'custom') {
    if (customWidth.value) settings.value['setting.defaultres'] = customWidth.value
    if (customHeight.value) settings.value['setting.defaultresheight'] = customHeight.value
  } else {
    const [w, h] = selectedResKey.value.split('x')
    settings.value['setting.defaultres'] = w
    settings.value['setting.defaultresheight'] = h
    if (['1280x960', '1024x768', '1440x1080'].includes(selectedResKey.value)) {
      settings.value['setting.aspectratiomode'] = '0' // 4:3
    } else if (['1920x1080', '2560x1440'].includes(selectedResKey.value)) {
      settings.value['setting.aspectratiomode'] = '1' // 16:9
    } else if (selectedResKey.value === '1680x1050') {
      settings.value['setting.aspectratiomode'] = '2' // 16:10
    }
  }
}

const onCustomResInput = () => {
  if (selectedResKey.value === 'custom') {
    settings.value['setting.defaultres'] = customWidth.value.trim()
    settings.value['setting.defaultresheight'] = customHeight.value.trim()
  }
}

// 把配置里的刷新率映射到下拉选项（支持 239997/1000 这类有理数）
const syncHzFromSettings = () => {
  const num = Number(settings.value['setting.refreshrate_numerator']) || 0
  const den = Number(settings.value['setting.refreshrate_denominator']) || 1000
  if (num <= 0) {
    selectedHz.value = 'auto'
    return
  }
  const hz = Math.round(num > 1000 ? num / den : num)
  selectedHz.value = availableHz.value.includes(hz) ? String(hz) : 'auto'
}

const onHzChange = () => {
  if (selectedHz.value !== 'auto') {
    const val = Number(selectedHz.value)
    if (val > 0) {
      settings.value['setting.refreshrate_numerator'] = String(val * 1000)
      settings.value['setting.refreshrate_denominator'] = '1000'
    }
  }
}

const loadData = async () => {
  if (!props.accountId) return
  loading.value = true
  try {
    const data = await getVideoSettings(props.accountId)
    filePath.value = data.file_path
    isReadOnly.value = !!data.read_only
    settings.value = { ...data.settings }
    syncResolutionFromSettings()

    // 读取本机真实支持的显示模式，用于刷新率下拉
    try {
      displayModes.value = await getDisplayModes()
    } catch {
      displayModes.value = []
    }
    syncHzFromSettings()

    await loadPresets()
  } catch (err: any) {
    showToast(`加载视频配置失败: ${err?.message || err}`, 'error')
    emit('log-action', `加载视频配置失败 [AccountID ${props.accountId}]: ${err?.message || err}`)
  } finally {
    loading.value = false
  }
}

const loadPresets = async () => {
  try {
    const list = await getVideoPresets()
    presets.value = list
    if (list.length > 0 && !selectedPresetId.value) {
      const def = list.find((p) => p.is_default)
      selectedPresetId.value = def ? def.id : list[0].id
    }
  } catch (err: any) {
    console.warn('Failed to load video presets:', err)
  }
}

watch(
  () => props.visible,
  (val) => {
    if (val && props.accountId) {
      loadData()
    }
  },
  { immediate: true }
)

watch(
  () => props.accountId,
  (val) => {
    if (props.visible && val) {
      loadData()
    }
  }
)


const handleSaveSettings = async () => {
  if (!props.accountId) return
  saving.value = true
  try {
    await saveVideoSettings(props.accountId, settings.value, isReadOnly.value)
    showToast('视频设置保存成功！')
    emit('log-action', `已成功更新账号 [${props.accountId}] 视频配置 (只读锁定: ${isReadOnly.value ? '已开启' : '关闭'})`)
    emit('saved')
  } catch (err: any) {
    showToast(`保存失败: ${err?.message || err}`, 'error')
    emit('log-action', `保存视频设置失败 [AccountID ${props.accountId}]: ${err?.message || err}`)
  } finally {
    saving.value = false
  }
}

const importPromptVisible = ref(false)
const importPath = ref('')
const importDefaultName = ref('')

const handleImportPreset = async () => {
  try {
    const path = await selectVideoConfigFile()
    if (!path) return
    importPath.value = path
    importDefaultName.value = path.split('\\').pop()?.split('/').pop()?.replace('.txt', '') || '导入预设'
    importPromptVisible.value = true
  } catch (err: any) {
    showToast(`选择文件失败: ${err?.message || err}`, 'error')
  }
}

const onConfirmImportName = async (name?: string) => {
  importPromptVisible.value = false
  const finalName = name?.trim() || importDefaultName.value
  if (!finalName || !importPath.value) return

  loading.value = true
  try {
    const p = await importVideoPreset(importPath.value, finalName, false)
    await loadPresets()
    selectedPresetId.value = p.id
    if (p.settings) {
      settings.value = { ...p.settings }
      syncResolutionFromSettings()
    }
    showToast(`已成功导入预设: ${p.name}`)
    emit('log-action', `已导入视频预设模板 [${p.name}]`)
  } catch (err: any) {
    showToast(`导入预设失败: ${err?.message || err}`, 'error')
    emit('log-action', `导入预设失败: ${err?.message || err}`)
  } finally {
    loading.value = false
  }
}

const handleSaveCurrentAsPreset = async () => {
  const name = newPresetName.value.trim()
  if (!name) {
    showToast('请输入预设名称', 'error')
    return
  }
  loading.value = true
  try {
    const p = await saveAsVideoPreset(props.accountId, name, false)
    await loadPresets()
    selectedPresetId.value = p.id
    newPresetName.value = ''
    isSavingPreset.value = false
    showToast(`已成功将当前画质存为预设: ${p.name}`)
    emit('log-action', `已将账号 [${props.accountId}] 当前配置存为新预设 [${p.name}]`)
  } catch (err: any) {
    showToast(`另存预设失败: ${err?.message || err}`, 'error')
    emit('log-action', `另存预设失败: ${err?.message || err}`)
  } finally {
    loading.value = false
  }
}

const handleSelectExistingPreset = async () => {
  if (!selectedPresetId.value) return
  const p = presets.value.find((x) => x.id === selectedPresetId.value)
  if (!p) return
  if (p.settings && Object.keys(p.settings).length > 0) {
    settings.value = { ...p.settings }
    syncResolutionFromSettings()
    showToast(`已载入预设 [${p.name}] 参数`)
  } else {
    try {
      loading.value = true
      await applyVideoPreset(props.accountId, p.id, isReadOnly.value)
      await loadData()
      showToast(`已载入预设 [${p.name}]`)
    } catch (err: any) {
      showToast(`载入预设失败: ${err?.message || err}`, 'error')
    } finally {
      loading.value = false
    }
  }
}

const close = () => {
  emit('close')
}

const handleKeydown = (e: KeyboardEvent) => {
  if (e.key === 'Escape' && props.visible) {
    close()
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div v-if="visible" class="modal-backdrop" @click.self="close">
    <div ref="dialogRef" class="winui-dialog" role="dialog" aria-modal="true" aria-labelledby="video-dialog-title" tabindex="-1">
      <!-- Header -->
      <div class="winui-dialog-header">
        <div class="header-content">
          <div class="header-icon-box">
            <SvgIcon name="monitor" :size="18" />
          </div>
          <div class="header-titles">
            <div class="header-title-row">
              <h3 id="video-dialog-title" class="modal-title">CS2 画质设置</h3>
              <span class="user-badge mono-font">
                <SvgIcon name="user" :size="11" />
                <span>{{ username }}</span>
                <span class="account-id">({{ accountId }})</span>
              </span>
            </div>
            <p class="modal-subtitle">调整 CS2 视频画质与显示设置</p>
          </div>
        </div>
        <button class="winui-close-btn" title="关闭 (Esc)" @click="close">
          <SvgIcon name="close" :size="14" />
        </button>
      </div>

      <!-- Toast Alert Banner -->
      <transition name="toast-fade">
        <div v-if="toastMsg" class="toast-banner" :class="toastType">
          <SvgIcon :name="toastType === 'success' ? 'check' : 'alert-triangle'" :size="14" />
          <span>{{ toastMsg }}</span>
        </div>
      </transition>

      <!-- Custom Presets Toolbar -->
      <div class="preset-section">
        <div class="preset-bar">
          <!-- Secondary Actions -->
          <div class="preset-aux-actions">
            <button
              type="button"
              class="btn-aux"
              title="从本地 .txt 格式文件导入预设"
              @click="handleImportPreset"
            >
              <SvgIcon name="download" :size="12" />
              <span>从本地导入 .txt</span>
            </button>
            <button
              type="button"
              class="btn-aux"
              title="将当前各项设置存为新预设模板"
              @click="isSavingPreset = !isSavingPreset"
            >
              <SvgIcon name="plus" :size="12" />
              <span>存为新预设</span>
            </button>
            <select
              v-if="presets.length > 0"
              v-model="selectedPresetId"
              class="winui-select-compact"
              @change="handleSelectExistingPreset"
            >
              <option value="" disabled>选择已存预设...</option>
              <option v-for="p in presets" :key="p.id" :value="p.id">
                {{ p.name }} {{ p.is_default ? '[默认]' : '' }}
              </option>
            </select>
          </div>
        </div>

        <!-- Inline Save Preset Form -->
        <div v-if="isSavingPreset" class="inline-save-row">
          <SvgIcon name="edit" :size="13" class="inline-save-icon" />
          <input
            v-model="newPresetName"
            type="text"
            class="winui-input inline-name-input"
            placeholder="输入预设名称（如 1280×960）"
            @keyup.enter="handleSaveCurrentAsPreset"
          />
          <button class="btn btn-primary btn-sm" :disabled="loading" @click="handleSaveCurrentAsPreset">
            确认保存
          </button>
          <button class="btn btn-secondary btn-sm" @click="isSavingPreset = false">
            取消
          </button>
        </div>
      </div>

      <!-- Main Body: WinUI 3 Settings Cards List -->
      <div class="winui-dialog-body">
        <div v-if="loading && !filePath" class="dialog-loading">
          <div class="spinner"></div>
          <p>正在读取目标账号 CS2 视频参数...</p>
        </div>

        <div v-else class="settings-cards-list">
          <div class="group-label">显示</div>

          <!-- 分辨率 -->
          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">分辨率</div>
            </div>
            <div class="card-right res-card-right">
              <select v-model="selectedResKey" class="winui-select" @change="onResolutionChange">
                <option value="1280x960">1280 × 960</option>
                <option value="1024x768">1024 × 768</option>
                <option value="1440x1080">1440 × 1080</option>
                <option value="1920x1080">1920 × 1080</option>
                <option value="2560x1440">2560 × 1440</option>
                <option value="1680x1050">1680 × 1050</option>
                <option value="custom">自定义</option>
              </select>
              <div v-if="selectedResKey === 'custom'" class="custom-res-inputs">
                <input
                  v-model="customWidth"
                  type="text"
                  class="winui-input custom-res-box mono-font"
                  placeholder="宽"
                  @input="onCustomResInput"
                />
                <span class="res-x">×</span>
                <input
                  v-model="customHeight"
                  type="text"
                  class="winui-input custom-res-box mono-font"
                  placeholder="高"
                  @input="onCustomResInput"
                />
              </div>
            </div>
          </div>

          <!-- 宽高比 -->
          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">宽高比</div>
            </div>
            <div class="card-right">
              <select v-model="settings['setting.aspectratiomode']" class="winui-select">
                <option value="0">4:3</option>
                <option value="1">16:9</option>
                <option value="2">16:10</option>
              </select>
            </div>
          </div>

          <!-- 显示模式 -->
          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">显示模式</div>
            </div>
            <div class="card-right">
              <select v-model="settings['setting.fullscreen']" class="winui-select">
                <option value="1">全屏</option>
                <option value="2">全屏窗口</option>
                <option value="0">窗口</option>
              </select>
            </div>
          </div>

          <!-- 刷新率 -->
          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">
                <span>刷新率</span>
                <span v-if="detectedHz" class="badge-tag">{{ detectedHz }}</span>
              </div>
            </div>
            <div class="card-right">
              <select v-model="selectedHz" class="winui-select" @change="onHzChange">
                <option value="auto">自动</option>
                <option v-for="hz in availableHz" :key="hz" :value="String(hz)">
                  {{ hz }} Hz{{ hz === maxHz ? '（最高）' : '' }}
                </option>
              </select>
            </div>
          </div>

          <!-- 显示器 -->
          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">显示器</div>
            </div>
            <div class="card-right">
              <select v-model="settings['setting.monitor_index']" class="winui-select">
                <option value="0">显示器 1</option>
                <option value="1">显示器 2</option>
                <option value="2">显示器 3</option>
                <option value="3">显示器 4</option>
              </select>
            </div>
          </div>

          <div class="group-label">画质</div>

          <!-- 全局阴影质量 -->
          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">全局阴影质量</div>
            </div>
            <div class="card-right">
              <select v-model="settings['setting.videocfg_shadow_quality']" class="winui-select">
                <option value="0">低</option>
                <option value="1">中</option>
                <option value="2">高</option>
                <option value="3">超高</option>
              </select>
            </div>
          </div>

          <!-- 动态阴影 -->
          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">动态阴影</div>
            </div>
            <div class="card-right">
              <select v-model="settings['setting.videocfg_dynamic_shadows']" class="winui-select">
                <option value="1">全部</option>
                <option value="0">仅太阳</option>
              </select>
            </div>
          </div>

          <!-- 模型 / 贴图细节 -->
          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">模型 / 贴图细节</div>
            </div>
            <div class="card-right">
              <select v-model="settings['setting.videocfg_texture_detail']" class="winui-select">
                <option value="0">低</option>
                <option value="1">中</option>
                <option value="2">高</option>
              </select>
            </div>
          </div>

          <!-- 粒子细节 -->
          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">粒子细节</div>
            </div>
            <div class="card-right">
              <select v-model="settings['setting.videocfg_particle_detail']" class="winui-select">
                <option value="0">低</option>
                <option value="1">中</option>
                <option value="2">高</option>
                <option value="3">超高</option>
              </select>
            </div>
          </div>

          <!-- 光影细节 -->
          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">光影细节</div>
            </div>
            <div class="card-right">
              <select v-model="settings['setting.shaderquality']" class="winui-select">
                <option value="0">低</option>
                <option value="1">高</option>
              </select>
            </div>
          </div>

          <!-- 环境光遮蔽（合法值 0 / 2 / 3） -->
          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">环境光遮蔽</div>
            </div>
            <div class="card-right">
              <select v-model="settings['setting.videocfg_ao_detail']" class="winui-select">
                <option value="0">禁用</option>
                <option value="2">中</option>
                <option value="3">高</option>
              </select>
            </div>
          </div>

          <!-- 高动态范围（3 性能 / -1 质量） -->
          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">高动态范围</div>
            </div>
            <div class="card-right">
              <select v-model="settings['setting.videocfg_hdr_detail']" class="winui-select">
                <option value="3">性能</option>
                <option value="-1">质量</option>
              </select>
            </div>
          </div>

          <!-- FidelityFX 超分辨率 -->
          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">FidelityFX 超分辨率</div>
            </div>
            <div class="card-right">
              <select v-model="settings['setting.videocfg_fsr_detail']" class="winui-select">
                <option value="0">关</option>
                <option value="1">超高品质</option>
                <option value="2">品质</option>
                <option value="3">平衡</option>
                <option value="4">性能</option>
              </select>
            </div>
          </div>

          <!-- 抗锯齿：MSAA 与 CMAA2 可同时开启 -->
          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">多重采样抗锯齿 (MSAA)</div>
            </div>
            <div class="card-right">
              <select v-model="msaaMode" class="winui-select">
                <option value="0">关</option>
                <option value="2">2x</option>
                <option value="4">4x</option>
                <option value="8">8x</option>
              </select>
            </div>
          </div>

          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">形态抗锯齿 (CMAA2)</div>
            </div>
            <div class="card-right toggle-right">
              <span class="toggle-status-label" :class="{ active: cmaaEnabled }">
                {{ cmaaEnabled ? '开' : '关' }}
              </span>
              <button
                type="button"
                role="switch"
                :aria-checked="cmaaEnabled"
                class="winui-toggle-switch"
                :class="{ checked: cmaaEnabled }"
                title="切换 CMAA2（可与 MSAA 同时开启）"
                @click="cmaaEnabled = !cmaaEnabled"
              >
                <span class="toggle-thumb"></span>
              </button>
            </div>
          </div>

          <!-- 贴图过滤模式 -->
          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">贴图过滤模式</div>
            </div>
            <div class="card-right">
              <select v-model="settings['setting.r_texturefilteringquality']" class="winui-select">
                <option value="0">双线性</option>
                <option value="1">三线性</option>
                <option value="2">2x</option>
                <option value="3">4x</option>
                <option value="4">8x</option>
                <option value="5">16x</option>
              </select>
            </div>
          </div>

          <!-- 垂直同步 -->
          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">垂直同步</div>
            </div>
            <div class="card-right">
              <select v-model="settings['setting.mat_vsync']" class="winui-select">
                <option value="0">关</option>
                <option value="1">开</option>
              </select>
            </div>
          </div>

          <!-- 低延迟 -->
          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">低延迟</div>
            </div>
            <div class="card-right">
              <select v-model="settings['setting.r_low_latency']" class="winui-select">
                <option value="0">关</option>
                <option value="1">开</option>
                <option value="2">开 + Boost</option>
              </select>
            </div>
          </div>

          <div class="group-label">硬件优先级</div>
          <p class="group-hint">
            数值越小越偏性能：0 的优先级最高（特效最少、帧数最高），3 的优先级最低（画质最全）。
            这三项是 CS2 遗留的硬件档位键，游戏已改用上方「画质」各项，改动通常不生效。
          </p>

          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">CPU 优先级</div>
            </div>
            <div class="card-right">
              <select v-model="settings['setting.cpu_level']" class="winui-select">
                <option value="0">0 · 性能优先</option>
                <option value="1">1 · 偏性能</option>
                <option value="2">2 · 偏画质</option>
                <option value="3">3 · 画质优先</option>
              </select>
            </div>
          </div>

          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">GPU 优先级</div>
            </div>
            <div class="card-right">
              <select v-model="settings['setting.gpu_level']" class="winui-select">
                <option value="0">0 · 性能优先</option>
                <option value="1">1 · 偏性能</option>
                <option value="2">2 · 偏画质</option>
                <option value="3">3 · 画质优先</option>
              </select>
            </div>
          </div>

          <div class="winui-settings-card">
            <div class="card-left">
              <div class="card-title">显存优先级</div>
            </div>
            <div class="card-right">
              <select v-model="settings['setting.gpu_mem_level']" class="winui-select">
                <option value="0">0 · 性能优先</option>
                <option value="1">1 · 偏性能</option>
                <option value="2">2 · 偏画质</option>
                <option value="3">3 · 画质优先</option>
              </select>
            </div>
          </div>

          <div class="group-label">其他</div>

          <!-- 显卡（只读） -->
          <div class="winui-settings-card readonly-card">
            <div class="card-left">
              <div class="card-title">显卡</div>
            </div>
            <div class="card-right">
              <span class="readonly-value mono-font">{{ gpuLabel }}</span>
            </div>
          </div>

          <!-- 配置文件只读保护 -->
          <div class="winui-settings-card lock-protect-card">
            <div class="card-left">
              <div class="card-title">
                <SvgIcon name="lock" :size="14" class="text-accent" />
                <span>配置文件只读保护</span>
              </div>
            </div>
            <div class="card-right toggle-right">
              <span class="toggle-status-label" :class="{ active: isReadOnly }">
                {{ isReadOnly ? '已锁定' : '未锁定' }}
              </span>
              <button
                type="button"
                role="switch"
                :aria-checked="isReadOnly"
                class="winui-toggle-switch"
                :class="{ checked: isReadOnly }"
                title="切换 cs2_video.txt 只读属性"
                @click="isReadOnly = !isReadOnly"
              >
                <span class="toggle-thumb"></span>
              </button>
            </div>
          </div>
        </div>
      </div>

      <!-- Footer -->
      <div class="winui-dialog-footer">
        <div class="footer-left">
          <div class="file-path-indicator" :title="filePath">
            <SvgIcon name="file-text" :size="12" class="file-icon" />
            <span class="file-name mono-font">{{ filePath || 'cs2_video.txt' }}</span>
          </div>
        </div>
        <div class="footer-right">
          <button class="btn btn-secondary" @click="close">
            取消
          </button>
          <button
            class="btn btn-primary"
            :disabled="saving || loading"
            @click="handleSaveSettings"
          >
            <span v-if="saving" class="spinner-small"></span>
            <SvgIcon v-if="!saving" name="check" :size="14" />
            <span>{{ saving ? '正在保存...' : '应用设置' }}</span>
          </button>
        </div>
      </div>
    </div>
    <!-- WinUI 3 Import Preset Name Prompt Modal -->
    <ConfirmModal
      :visible="importPromptVisible"
      title="导入视频预设模板"
      message="请输入该导入配置的显示名称："
      icon="info"
      confirm-text="确认导入"
      cancel-text="取消"
      :prompt="true"
      prompt-placeholder="例如: 某主播画质 / 1080P高刷"
      :prompt-default="importDefaultName"
      @confirm="onConfirmImportName"
      @cancel="importPromptVisible = false"
    />
  </div>
</template>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  background-color: rgba(10, 12, 16, 0.7);
  backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1400;
  padding: 24px;
}

.winui-dialog {
  display: flex;
  flex-direction: column;
  width: 860px;
  max-width: 95vw;
  height: 86vh;
  max-height: 840px;
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

.winui-dialog-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 24px;
  border-bottom: 1px solid var(--border-color);
  background: var(--surface-1);
  flex-shrink: 0;
}

.header-content {
  display: flex;
  align-items: center;
  gap: 14px;
}

.header-icon-box {
  width: 36px;
  height: 36px;
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
  gap: 2px;
}

.header-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
}

.modal-title {
  letter-spacing: -0.2px;
}

.modal-subtitle {
  font-size: 12px;
  color: var(--color-text-secondary);
  margin: 0;
}

.user-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  font-weight: 600;
  color: var(--accent);
  background: var(--accent-subtle);
  border: 1px solid var(--tone-accent-mid);
  padding: 2px 8px;
  border-radius: var(--radius-sm, 4px);
}

.account-id {
  color: var(--color-text-muted);
}

.winui-close-btn {
  width: 32px;
  height: 32px;
  border-radius: var(--radius-md, 6px);
  background: transparent;
  border: none;
  color: var(--color-text-muted);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s ease;
}

.winui-close-btn:hover {
  background: var(--surface-4);
  color: var(--color-text);
}

.toast-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 24px;
  font-size: 12px;
  font-weight: 500;
  flex-shrink: 0;
}

.toast-banner.success {
  background: var(--tone-success-weak);
  color: var(--level-success);
  border-bottom: 1px solid rgba(108, 203, 95, 0.3);
}

.toast-banner.error {
  background: var(--tone-error-weak);
  color: var(--level-error);
  border-bottom: 1px solid var(--tone-error-strong);
}

.preset-section {
  padding: 12px 24px;
  border-bottom: 1px solid var(--border-color);
  background: rgba(255, 255, 255, 0.015);
  display: flex;
  flex-direction: column;
  gap: 10px;
  flex-shrink: 0;
}

.preset-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}

.preset-segmented-group {
  display: inline-flex;
  background: rgba(0, 0, 0, 0.25);
  padding: 3px;
  border-radius: var(--radius-md, 6px);
  border: 1px solid var(--border-color);
  gap: 4px;
}

.preset-segment-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 28px;
  padding: 0 12px;
  font-size: 12px;
  font-weight: 500;
  border-radius: 4px;
  border: none;
  background: transparent;
  color: var(--color-text-secondary);
  cursor: pointer;
  transition: all 0.15s ease;
}

.preset-segment-btn:hover {
  background: var(--surface-3);
  color: #fff;
}

.preset-segment-btn.active {
  background: var(--accent);
  color: #ffffff;
  font-weight: 600;
  box-shadow: 0 2px 6px rgba(0, 120, 212, 0.4);
}

.preset-aux-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.btn-aux {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 28px;
  padding: 0 10px;
  background: var(--bg-control);
  border: 1px solid var(--border-control);
  border-radius: var(--radius-sm, 4px);
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-secondary);
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-aux:hover {
  background: var(--bg-control-hover);
  color: #fff;
  border-color: var(--border-control-hover);
}

.winui-select-compact {
  height: 28px;
  background: var(--bg-control);
  border: 1px solid var(--border-control);
  border-radius: var(--radius-sm, 4px);
  color: var(--color-text-secondary);
  font-size: 12px;
  padding: 0 8px;
  outline: none;
  cursor: pointer;
}

.winui-select-compact:focus {
  border-color: var(--accent);
}

.inline-save-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 10px;
  background: rgba(0, 0, 0, 0.2);
  border: 1px solid var(--tone-accent-strong);
  border-radius: var(--radius-md, 6px);
}

.inline-save-icon {
  color: var(--accent);
}

.inline-name-input {
  flex: 1;
  height: 28px;
  font-size: 12px;
}

.winui-dialog-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 24px;
}

.dialog-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 60px 0;
  gap: 16px;
  color: var(--color-text-muted);
}

.settings-cards-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.group-label {
  margin: 10px 0 2px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.4px;
  color: var(--color-text-muted);
}

.group-label:first-child {
  margin-top: 0;
}

/* 分组说明：解释反直觉的取值方向（0 = 性能优先） */
.group-hint {
  margin: 0 0 2px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--color-text-muted);
}

.winui-settings-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 6px 12px;
  background-color: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-control);
  min-height: 42px;
  transition: background-color var(--duration-fast) var(--ease-standard),
    border-color var(--duration-fast) var(--ease-standard);
}

.winui-settings-card:hover {
  background-color: var(--bg-card-hover);
  border-color: rgba(255, 255, 255, 0.12);
}

.card-left {
  flex: 1;
  min-width: 0;
  padding-right: 16px;
}

.card-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
}

.card-subtitle {
  font-size: 12px;
  color: var(--color-text-secondary);
  margin-top: 2px;
  line-height: 1.4;
}

.readonly-card {
  background: transparent;
}

.readonly-value {
  font-size: 12px;
  color: var(--color-text-secondary);
}

.badge-tag {
  font-size: 12px;
  font-weight: 600;
  color: var(--accent);
  background: var(--accent-subtle);
  padding: 1px 6px;
  border-radius: 4px;
  border: 1px solid var(--tone-accent-mid);
}

.card-right {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.res-card-right {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  gap: 6px;
}

.custom-res-inputs {
  display: flex;
  align-items: center;
  gap: 6px;
}

.custom-res-box {
  width: 80px;
  height: 28px;
  font-size: 12px;
  text-align: center;
}

.res-x {
  color: var(--color-text-muted);
  font-size: 12px;
}

.winui-select {
  height: var(--control-height, 32px);
  min-width: 160px;
  max-width: 220px;
  padding: 0 10px;
  background-color: var(--bg-control);
  border: 1px solid var(--border-control);
  border-radius: var(--radius-md, 6px);
  color: var(--color-text);
  font-size: 12px;
  outline: none;
  cursor: pointer;
  transition: all 0.15s ease;
}

.winui-select:focus {
  border-color: var(--accent);
  background-color: var(--bg-control-active);
}

.winui-input {
  background-color: var(--bg-control);
  border: 1px solid var(--border-control);
  border-radius: var(--radius-md, 6px);
  color: var(--color-text);
  padding: 4px 10px;
  outline: none;
  transition: all 0.15s ease;
}

.winui-input:focus {
  border-color: var(--accent);
  background-color: var(--bg-control-active);
}

/* WinUI 3 Toggle Switch */
.toggle-right {
  display: flex;
  align-items: center;
  gap: 12px;
}

.toggle-status-label {
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-muted);
  transition: color 0.15s ease;
}

.toggle-status-label.active {
  color: var(--level-success);
  font-weight: 600;
}

.winui-toggle-switch {
  width: 44px;
  height: 22px;
  border-radius: 11px;
  background: rgba(255, 255, 255, 0.12);
  border: 1px solid rgba(255, 255, 255, 0.25);
  position: relative;
  cursor: pointer;
  padding: 0;
  outline: none;
  transition: all 0.2s cubic-bezier(0.1, 0.9, 0.2, 1);
}

.winui-toggle-switch:hover {
  background: rgba(255, 255, 255, 0.18);
  border-color: rgba(255, 255, 255, 0.35);
}

.winui-toggle-switch.checked {
  background: var(--accent, #0078d4);
  border-color: var(--accent, #0078d4);
}

.winui-toggle-switch.checked:hover {
  background: var(--accent-hover, #1886d9);
  border-color: var(--accent-hover, #1886d9);
}

.toggle-thumb {
  position: absolute;
  top: 3px;
  left: 3px;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: #ffffff;
  transition: transform 0.2s cubic-bezier(0.1, 0.9, 0.2, 1);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.3);
}

.winui-toggle-switch.checked .toggle-thumb {
  transform: translateX(22px);
}

.lock-protect-card {
  border-left: 3px solid var(--accent);
}

/* Footer */
.winui-dialog-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 24px;
  border-top: 1px solid var(--border-color);
  background: var(--surface-1);
  flex-shrink: 0;
}

.footer-left {
  display: flex;
  align-items: center;
  min-width: 0;
  flex: 1;
  padding-right: 16px;
}

.file-path-indicator {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--color-text-muted);
  max-width: 100%;
}

.file-icon {
  flex-shrink: 0;
  color: var(--color-text-secondary);
}

.file-name {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.footer-right {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-shrink: 0;
}

.spinner-small {
  --spinner-size: 13px;
  --spinner-border: rgba(255, 255, 255, 0.3);
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
