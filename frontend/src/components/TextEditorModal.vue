<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted, defineAsyncComponent } from 'vue'
import { formatSize } from '../utils/format'
import type { GameCFGFile } from '../types'
import SvgIcon from './SvgIcon.vue'
import ConfirmModal from './ConfirmModal.vue'
// 异步加载：CodeMirror 约 400 KB，编辑器只在用户点开时才用到，
// 不该拖慢首屏。拆成独立 chunk，第一次打开编辑器时再拉。
const CfgCodeEditor = defineAsyncComponent(() => import('./CfgCodeEditor.vue'))
import { useFocusTrap } from '../utils/a11y'
import { readConfigFile, saveConfigFileText, createConfigFile, getGameCFGFiles, getUserCFGFiles } from '../utils/wails'
import { cfgNameHint, finalCfgName } from '../utils/cfgName'

const props = withDefaults(
  defineProps<{
    visible: boolean
    accountId: string
    initialFile?: string
    initialScope?: 'user' | 'game'
  }>(),
  {
    initialFile: '',
    initialScope: 'user',
  }
)

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved', filename: string): void
}>()

// 常见文件的展示优先级（存在才排前面，不存在则忽略）
const PREFERRED_ORDER = [
  'cs2_video.txt',
  'cs2_user_keys_0_slot0.vcfg',
  'cs2_machine_convars.vcfg',
  'autoexec.cfg',
  'cs2_user_convars_0_slot0.vcfg',
]

// State
const currentScope = ref<'user' | 'game'>('user')
const selectedPresetPath = ref('')
const availableFiles = ref<GameCFGFile[]>([])
const filesLoading = ref(false)
const filesError = ref('')

const dialogRef = ref<HTMLElement | null>(null)

useFocusTrap(
  computed(() => props.visible),
  dialogRef
)

const fileContent = ref('')
const originalContent = ref('')
const loading = ref(false)
const saving = ref(false)

// Toast message
const toastMessage = ref('')
const toastType = ref<'success' | 'error'>('success')
let toastTimer: ReturnType<typeof setTimeout> | null = null

// Editor DOM refs for synchronized scroll

const showToast = (msg: string, type: 'success' | 'error' = 'success') => {
  if (toastTimer) clearTimeout(toastTimer)
  toastMessage.value = msg
  toastType.value = type
  toastTimer = setTimeout(() => {
    toastMessage.value = ''
  }, 3500)
}

// Normalize raw file path to scope & clean relative path
const normalizePath = (raw: string): { scope: 'user' | 'game'; path: string } => {
  let p = raw.replace(/\\/g, '/').trim()
  if (p.startsWith('game/csgo/cfg/')) {
    return { scope: 'game', path: p.replace(/^game\/csgo\/cfg\//, '') }
  }
  if (p.startsWith('csgo/cfg/')) {
    return { scope: 'game', path: p.replace(/^csgo\/cfg\//, '') }
  }
  if (p.startsWith('730/local/cfg/')) {
    return { scope: 'user', path: p.replace(/^730\/local\/cfg\//, '') }
  }
  if (p.startsWith('local/cfg/')) {
    return { scope: 'user', path: p.replace(/^local\/cfg\//, '') }
  }
  return { scope: props.initialScope || 'user', path: p }
}

// Effective Account ID for backend calls
const effectiveAccountId = computed(() => {
  return currentScope.value === 'game' ? 'game_cfg' : props.accountId
})

// Effective clean relative path
const currentRelPath = computed(() => normalizePath(selectedPresetPath.value.trim()).path)

// Dirty state check
const isDirty = computed(() => {
  return fileContent.value !== originalContent.value
})

// Line numbers and character metrics
const lines = computed(() => {
  return fileContent.value.split('\n')
})

const lineCount = computed(() => {
  return lines.value.length
})

const charCount = computed(() => {
  return fileContent.value.length
})

// Load file from backend
const loadFile = async (showSuccessNotice = false) => {
  const targetPath = currentRelPath.value
  if (!targetPath) {
    showToast('请输入有效的配置文件相对路径', 'error')
    return
  }

  loading.value = true
  try {
    const content = await readConfigFile(effectiveAccountId.value, targetPath)
    fileContent.value = content
    originalContent.value = content
    if (showSuccessNotice) {
      showToast(`已重新载入文件: ${targetPath}`)
    }
  } catch (err: any) {
    const errMsg = err?.message || String(err)
    showToast(`读取文件失败: ${errMsg}`, 'error')
    fileContent.value = ''
    originalContent.value = ''
  } finally {
    loading.value = false
  }
}

// Save file modifications to backend
const handleSave = async () => {
  const targetPath = currentRelPath.value
  if (!targetPath) {
    showToast('请输入有效的配置文件相对路径', 'error')
    return
  }

  saving.value = true
  try {
    await saveConfigFileText(effectiveAccountId.value, targetPath, fileContent.value)
    originalContent.value = fileContent.value
    showToast(`文件 [${targetPath}] 保存成功！`, 'success')
    emit('saved', targetPath)
  } catch (err: any) {
    const errMsg = err?.message || String(err)
    showToast(`保存文件失败: ${errMsg}`, 'error')
  } finally {
    saving.value = false
  }
}

// Unsaved changes confirmation dialog
const confirmUnsavedVisible = ref(false)
const confirmUnsavedMessage = ref('')
let pendingAction: (() => void) | null = null

const askConfirmDiscard = (message: string, action: () => void) => {
  if (!isDirty.value) {
    action()
    return
  }
  confirmUnsavedMessage.value = message
  pendingAction = action
  confirmUnsavedVisible.value = true
}

const onConfirmDiscard = () => {
  confirmUnsavedVisible.value = false
  if (pendingAction) {
    const act = pendingAction
    pendingAction = null
    act()
  }
}

// Scope switching
// 扫描当前作用域下真实存在的配置文件
const loadAvailableFiles = async (): Promise<GameCFGFile[]> => {
  filesLoading.value = true
  filesError.value = ''
  try {
    const list =
      currentScope.value === 'game'
        ? await getGameCFGFiles()
        : await getUserCFGFiles(props.accountId)

    // 按常见文件优先级排序，其余按名称
    const rank = (n: string) => {
      const i = PREFERRED_ORDER.indexOf(n.toLowerCase())
      return i === -1 ? 999 : i
    }
    list.sort((a, b) => {
      const ra = rank(a.name)
      const rb = rank(b.name)
      if (ra !== rb) return ra - rb
      return a.name.localeCompare(b.name)
    })

    availableFiles.value = list
    return list
  } catch (err: any) {
    availableFiles.value = []
    filesError.value = err?.message || '扫描配置目录失败'
    return []
  } finally {
    filesLoading.value = false
  }
}

const switchScope = async (scope: 'user' | 'game') => {
  if (currentScope.value === scope) return

  askConfirmDiscard('当前文件有未保存的修改，切换作用域将放弃修改，是否继续？', async () => {
    currentScope.value = scope
    const list = await loadAvailableFiles()
    if (list.length > 0) {
      selectedPresetPath.value = list[0].name
      await loadFile()
    } else {
      selectedPresetPath.value = ''
      fileContent.value = ''
      originalContent.value = ''
    }
  })
}

// 下拉切换文件
const handlePresetChange = async () => {
  if (!selectedPresetPath.value) return
  askConfirmDiscard('当前文件有未保存的修改，切换文件将放弃修改，是否继续？', async () => {
    await loadFile()
  })
}

/* ---------------- 新建配置文件 ---------------- */

const newFileOpen = ref(false)
const newFileName = ref('')
const newFileError = ref('')
const creatingFile = ref(false)
const newFileInput = ref<HTMLInputElement | null>(null)

/** 即时反馈；真正的判定在 Go 侧，规则见 utils/cfgName.ts。 */
const newFileHint = computed(() =>
  cfgNameHint(
    newFileName.value,
    availableFiles.value.map((f) => f.name),
  ),
)

const newFilePreview = computed(() => finalCfgName(newFileName.value))

function openNewFile() {
  newFileName.value = ''
  newFileError.value = ''
  newFileOpen.value = true
  nextTick(() => newFileInput.value?.focus())
}

function closeNewFile() {
  newFileOpen.value = false
}

async function submitNewFile() {
  if (creatingFile.value) return
  const raw = newFileName.value.trim()
  if (!raw) {
    newFileError.value = '先给文件起个名字'
    return
  }
  if (newFileHint.value) {
    newFileError.value = newFileHint.value
    return
  }

  // 新建会切换当前文件，先问一句别把未保存的改动丢了
  askConfirmDiscard('当前文件有未保存的修改，新建文件将放弃这些修改，是否继续？', async () => {
    creatingFile.value = true
    newFileError.value = ''
    try {
      const created = await createConfigFile(effectiveAccountId.value, raw)
      newFileOpen.value = false
      // 先刷新列表（下拉里得有这一项），再选中它并载入。
      // 不靠「列表里有没有」来决定要不要打开：我们刚刚建的就是这个文件，
      // 直接打开比依赖扫描结果稳。
      await loadAvailableFiles()
      selectedPresetPath.value = created
      await loadFile()
      showToast(`已新建 ${created}`, 'success')
    } catch (err: any) {
      // 后端的错误直接显示在弹窗里，不吞掉
      newFileError.value = err?.message || String(err)
    } finally {
      creatingFile.value = false
    }
  })
}

// 重新扫描
const handleRefreshFiles = async () => {
  const previous = selectedPresetPath.value
  const list = await loadAvailableFiles()
  if (previous && list.some((f) => f.name === previous)) {
    selectedPresetPath.value = previous
    showToast('已刷新文件列表')
  } else if (list.length > 0) {
    selectedPresetPath.value = list[0].name
    await loadFile()
  }
}

// Close dialog
const handleClose = () => {
  askConfirmDiscard('当前文件有未保存的修改，确定要关闭编辑器吗？', () => {
    emit('close')
  })
}

// Initialize when modal is shown or inputs change
const initialize = async () => {
  if (!props.visible) return

  // Determine initial scope and path
  let targetScope: 'user' | 'game' = props.initialScope || (props.accountId === 'game_cfg' ? 'game' : 'user')
  let targetFile = props.initialFile || ''

  if (targetFile) {
    const normalized = normalizePath(targetFile)
    targetScope = normalized.scope
    targetFile = normalized.path
  }

  currentScope.value = targetScope
  const list = await loadAvailableFiles()

  if (targetFile) {
    // 来自 Diff 等外部入口的文件可能不在扫描结果里，补一条以便直接打开
    if (!list.some((f) => f.name === targetFile)) {
      availableFiles.value = [
        {
          name: targetFile,
          size: 0,
          modified: '',
          ext: targetFile.slice(targetFile.lastIndexOf('.')).toLowerCase(),
          source: 'player',
          read_only: false,
        },
        ...list,
      ]
    }
    selectedPresetPath.value = targetFile
  } else if (list.length > 0) {
    selectedPresetPath.value = list[0].name
  } else {
    selectedPresetPath.value = ''
  }

  await loadFile()
}

// Watch visible state
watch(
  () => props.visible,
  (val) => {
    if (val) {
      initialize()
    } else {
      toastMessage.value = ''
    }
  },
  { immediate: true }
)

// Watch initialFile updates
watch(
  () => props.initialFile,
  (newFile) => {
    if (props.visible && newFile) {
      initialize()
    }
  }
)

const handleKeydown = (e: KeyboardEvent) => {
  if (e.key === 'Escape' && props.visible) {
    handleClose()
  }
  // Ctrl+S or Cmd+S shortcut to save
  if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's' && props.visible) {
    e.preventDefault()
    if (!saving.value && !loading.value) {
      handleSave()
    }
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
  if (toastTimer) clearTimeout(toastTimer)
})
</script>

<template>
  <div v-if="visible" class="modal-backdrop" @click.self="handleClose">
    <div ref="dialogRef" class="text-editor-dialog" role="dialog" aria-modal="true" aria-labelledby="editor-dialog-title" tabindex="-1">
      <!-- Modal Header -->
      <div class="editor-header">
        <div class="header-left">
          <div class="header-icon-box">
            <SvgIcon name="file-text" :size="18" class="text-accent" />
          </div>
          <div class="header-title-box">
            <div class="title-row">
              <h3 class="modal-title">配置文件编辑器</h3>
              <span v-if="isDirty" class="dirty-badge" title="当前内容与磁盘文件不一致">
                * 未保存修改
              </span>
            </div>
            <div class="header-meta">
              <span class="scope-indicator">
                {{ currentScope === 'game' ? '游戏通用配置' : `账号配置 (${accountId})` }}
              </span>
              <span class="file-path-badge font-mono">{{ currentRelPath }}</span>
            </div>
          </div>
        </div>

        <div class="header-right">
          <button class="btn-close" title="关闭 (Esc)" @click="handleClose">
            <SvgIcon name="close" :size="16" />
          </button>
        </div>
      </div>

      <!-- Toast Alert Banner -->
      <transition name="toast-fade">
        <div v-if="toastMessage" class="toast-banner" :class="toastType">
          <SvgIcon
            :name="toastType === 'success' ? 'check' : 'alert-triangle'"
            :size="15"
            class="toast-icon"
          />
          <span class="toast-text">{{ toastMessage }}</span>
        </div>
      </transition>

      <!-- Scope & File Selector Toolbar -->
      <div class="editor-toolbar">
        <!-- Scope Segmented Control -->
        <div class="scope-selector">
          <button
            type="button"
            class="scope-btn"
            :class="{ active: currentScope === 'user' }"
            title="查看或修改当前登录 Steam 账号的 730/local/cfg 配置"
            @click="switchScope('user')"
          >
            <SvgIcon name="user" :size="13" />
            <span>当前账号配置 (730/local/cfg)</span>
          </button>
          <button
            type="button"
            class="scope-btn"
            :class="{ active: currentScope === 'game' }"
            title="查看或修改 CS2 根目录下的 game/csgo/cfg 通用配置"
            @click="switchScope('game')"
          >
            <SvgIcon name="gamepad" :size="13" />
            <span>游戏通用配置 (game/csgo/cfg)</span>
          </button>
        </div>

        <!-- 文件选择：由后端扫描目录，用户直接挑，无需手输文件名 -->
        <div class="file-select-group">
          <label class="toolbar-label">选择文件:</label>
          <select
            v-model="selectedPresetPath"
            class="select-control preset-select font-mono"
            :disabled="filesLoading || availableFiles.length === 0"
            @change="handlePresetChange"
          >
            <option v-if="availableFiles.length === 0" value="">
              {{ filesLoading ? '正在扫描...' : '未扫描到配置文件' }}
            </option>
            <option v-for="f in availableFiles" :key="f.name" :value="f.name">
              {{ f.name }}{{ f.size ? `  ·  ${formatSize(f.size)}` : '' }}
            </option>
          </select>
          <button
            type="button"
            class="btn btn-secondary btn-sm file-refresh-btn"
            :disabled="filesLoading"
            title="重新扫描目录"
            aria-label="重新扫描目录"
            @click="handleRefreshFiles"
          >
            <SvgIcon name="refresh" :size="13" :class="{ spinning: filesLoading }" />
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-sm file-new-btn"
            :disabled="filesLoading || creatingFile"
            :title="`在 ${currentScope === 'game' ? 'game/csgo/cfg' : '730/local/cfg'} 下新建一个 .cfg 文件`"
            @click="openNewFile"
          >
            <SvgIcon name="plus" :size="13" />
            <span>新建</span>
          </button>
          <span class="file-count">{{ availableFiles.length }} 个文件</span>

          <!-- 新建弹层：只问名字，建完直接打开 -->
          <div v-if="newFileOpen" class="new-file-pop" @keydown.esc.stop="closeNewFile">
            <p class="new-file-title">
              新建到 {{ currentScope === 'game' ? 'game/csgo/cfg' : '730/local/cfg' }}
            </p>
            <input
              ref="newFileInput"
              v-model="newFileName"
              class="new-file-input font-mono"
              type="text"
              placeholder="例如 practice"
              @keydown.enter.prevent="submitNewFile"
            />
            <p v-if="newFileError || newFileHint" class="new-file-error">
              {{ newFileError || newFileHint }}
            </p>
            <p v-else class="new-file-tip">
              将创建 <strong>{{ newFilePreview || 'xxx.cfg' }}</strong>（后缀自动补全）
            </p>
            <div class="new-file-actions">
              <button type="button" class="btn btn-secondary btn-sm" @click="closeNewFile">取消</button>
              <button
                type="button"
                class="btn btn-primary btn-sm"
                :disabled="creatingFile || !newFileName.trim() || !!newFileHint"
                @click="submitNewFile"
              >
                {{ creatingFile ? '创建中…' : '创建' }}
              </button>
            </div>
          </div>
        </div>
      </div>

      <div v-if="filesError" class="files-error-bar">
        <SvgIcon name="alert-triangle" :size="14" />
        <span>{{ filesError }}</span>
      </div>

      <!-- Editor Core Workspace -->
      <div class="editor-workspace">
        <div v-if="loading" class="editor-loading-overlay">
          <div class="loading-spinner"></div>
          <span>正在读取配置文本...</span>
        </div>

        <div class="code-editor-container">
          <!-- CodeMirror：高亮 + 结构校验 + 补全。行号、滚动同步、Tab 缩进
               都由它自己处理，原来那套自绘 gutter 一并删掉了。 -->
          <CfgCodeEditor v-model="fileContent" />
        </div>
      </div>

      <!-- Editor Footer & Actions -->
      <div class="editor-footer">
        <div class="footer-status-bar">
          <span class="status-item">
            <SvgIcon name="file-text" :size="13" />
            <span>共 {{ lineCount }} 行</span>
          </span>
          <span class="status-separator">|</span>
          <span class="status-item">
            <span>{{ charCount.toLocaleString() }} 字符</span>
          </span>
          <span class="status-separator">|</span>
          <span class="status-item font-mono text-muted">UTF-8</span>
          <span v-if="isDirty" class="status-dirty-pill">
            未保存修改 (Ctrl+S)
          </span>
        </div>

        <div class="footer-actions">
          <button
            class="btn btn-secondary"
            :disabled="loading || saving || !isDirty"
            title="从本地磁盘重新载入文件，放弃当前编辑器内的所有修改"
            @click="() => loadFile(true)"
          >
            <SvgIcon name="refresh" :size="13" />
            <span>重新载入 / 放弃修改</span>
          </button>

          <button
            class="btn btn-secondary"
            @click="handleClose"
          >
            关闭
          </button>

          <button
            class="btn btn-primary"
            :disabled="loading || saving || !isDirty"
            title="保存修改至本地磁盘文件 (Ctrl+S)"
            @click="handleSave"
          >
            <SvgIcon v-if="!saving" name="check" :size="14" />
            <span v-if="saving" class="spinner-small"></span>
            <span>{{ saving ? '正在保存...' : '保存修改' }}</span>
          </button>
        </div>
      </div>
    </div>
    <!-- WinUI 3 Unsaved Changes Confirmation Modal -->
    <ConfirmModal
      :visible="confirmUnsavedVisible"
      title="未保存的修改"
      :message="confirmUnsavedMessage"
      detail="放弃后，当前文件所做的编辑修改将丢失，不会写入本地磁盘。"
      icon="warning"
      confirm-text="放弃修改"
      cancel-text="继续编辑"
      :is-danger="true"
      @confirm="onConfirmDiscard"
      @cancel="confirmUnsavedVisible = false"
    />
  </div>
</template>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  background-color: rgba(10, 12, 16, 0.85);
  backdrop-filter: blur(6px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1600;
  padding: 24px;
}

.text-editor-dialog {
  display: flex;
  flex-direction: column;
  width: 980px;
  max-width: 96vw;
  height: 85vh;
  max-height: 850px;
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-dialog);
  overflow: hidden;
  animation: dialogFadeIn 0.15s ease-out;
}

@keyframes dialogFadeIn {
  from {
    opacity: 0;
    transform: scale(0.98);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}

/* Header */
.editor-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 20px;
  background-color: var(--bg-surface-alt);
  border-bottom: 1px solid var(--border-color);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-icon-box {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  background: var(--accent-subtle);
  border-radius: var(--radius-md);
  border: 1px solid var(--tone-accent-mid);
}

.title-row {
  display: flex;
  align-items: center;
  gap: 8px;
}


.dirty-badge {
  font-size: 12px;
  font-weight: 600;
  color: var(--level-warning);
  background: rgba(252, 225, 0, 0.12);
  border: 1px solid rgba(252, 225, 0, 0.3);
  padding: 1px 6px;
  border-radius: var(--radius-sm);
}

.header-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 2px;
  font-size: 12px;
}

.scope-indicator {
  color: var(--color-text-secondary);
}

.file-path-badge {
  color: var(--accent);
  background: var(--bg-control);
  padding: 1px 6px;
  border-radius: var(--radius-xs);
  border: 1px solid var(--border-control);
  font-size: 12px;
}

.header-right {
  display: flex;
  align-items: center;
}

.btn-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  background: transparent;
  border: none;
  color: var(--color-text-secondary);
  cursor: pointer;
  border-radius: var(--radius-md);
  transition: all 0.15s ease;
}

.btn-close:hover {
  background-color: var(--bg-control-hover);
  color: #ffffff;
}

/* Toast Alert Banner */
.toast-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  font-size: 12px;
  border-bottom: 1px solid transparent;
}

.toast-banner.success {
  background-color: var(--tone-success-weak);
  color: var(--level-success);
  border-bottom-color: rgba(108, 203, 95, 0.3);
}

.toast-banner.error {
  background-color: var(--tone-error-weak);
  color: var(--level-error);
  border-bottom-color: var(--tone-error-strong);
}

.toast-icon {
  flex-shrink: 0;
}

.toast-fade-enter-active,
.toast-fade-leave-active {
  transition: all 0.2s ease;
}

.toast-fade-enter-from,
.toast-fade-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}

/* Toolbar */
.editor-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  background-color: #232323;
  border-bottom: 1px solid var(--border-color);
  gap: 16px;
  flex-wrap: wrap;
}

/* Scope Selector Segmented Button */
.scope-selector {
  display: flex;
  background-color: var(--bg-control);
  padding: 2px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-control);
}

.scope-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 12px;
  font-size: 12px;
  font-weight: 500;
  background: transparent;
  color: var(--color-text-secondary);
  border: none;
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
}

.scope-btn:hover:not(.active) {
  background-color: var(--bg-control-hover);
  color: var(--color-text);
}

.scope-btn.active {
  background-color: var(--accent);
  color: #ffffff;
  font-weight: 600;
}

/* Preset & Custom Select Group */
.new-file-pop {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  width: 300px;
  padding: 10px 12px;
  background: var(--bg-card-solid);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.5);
  z-index: 30;
}

.new-file-title {
  margin: 0 0 8px;
  color: var(--color-text-muted);
  font-size: 11.5px;
}

.new-file-input {
  width: 100%;
  height: 32px;
  padding: 0 8px;
  box-sizing: border-box;
  background: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 6px;
  color: var(--color-text);
  font-size: 12.5px;
  outline: none;
}

.new-file-input:focus {
  border-color: var(--accent);
}

.new-file-tip,
.new-file-error {
  margin: 6px 0 0;
  font-size: 11px;
  line-height: 1.5;
}

.new-file-tip {
  color: var(--color-text-muted);
}

.new-file-error {
  color: var(--level-warning);
}

.new-file-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 10px;
}

.file-select-group {
  position: relative; /* 新建弹层挂在它下面 */
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  justify-content: flex-end;
}

.toolbar-label {
  font-size: 12px;
  color: var(--color-text-secondary);
  white-space: nowrap;
}

.file-refresh-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  flex-shrink: 0;
}

.file-count {
  font-size: 12px;
  color: var(--color-text-muted);
  white-space: nowrap;
  font-family: var(--font-mono);
}

.files-error-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 16px 8px;
  padding: 8px 12px;
  border-radius: var(--radius-control);
  background: var(--status-error-fill);
  border: 1px solid var(--status-error-stroke);
  color: var(--level-error);
  font-size: 12px;
}

.preset-select {
  min-width: 280px;
  max-width: 360px;
}

.custom-path-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
}

.custom-path-input {
  width: 220px;
}

/* Editor Core Workspace */
.editor-workspace {
  flex: 1;
  position: relative;
  overflow: hidden;
  background-color: #1a1a1a;
  display: flex;
}

.editor-loading-overlay {
  position: absolute;
  inset: 0;
  background-color: rgba(26, 26, 26, 0.85);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 12px;
  z-index: 10;
  color: var(--color-text-secondary);
  font-size: 13px;
}

.loading-spinner {
  width: 24px;
  height: 24px;
  border: 2px solid var(--stroke-3);
  border-top-color: var(--accent);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

.code-editor-container {
  display: flex;
  width: 100%;
  height: 100%;
  overflow: hidden;
}

/* Footer */
.editor-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 10px 16px;
  background-color: var(--bg-surface-alt);
  border-top: 1px solid var(--border-color);
  gap: 16px;
}

.footer-status-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
  color: var(--color-text-secondary);
}

.status-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.status-separator {
  color: var(--border-color);
}

.status-dirty-pill {
  font-size: 12px;
  font-weight: 600;
  color: var(--level-warning);
  background: rgba(252, 225, 0, 0.12);
  border: 1px solid rgba(252, 225, 0, 0.25);
  padding: 1px 8px;
  border-radius: var(--radius-sm);
}

.footer-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.spinner-small {
  --spinner-size: 13px;
  --spinner-border: rgba(255, 255, 255, 0.3);
}
</style>
