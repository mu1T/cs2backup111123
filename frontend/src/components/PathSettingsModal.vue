<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import SvgIcon from './SvgIcon.vue'
import { useFocusTrap } from '../utils/a11y'
import {
  setSteamPath,
  setBackupPath,
  selectSteamDirectory,
  selectBackupDirectory,
  detectSteamPath,
  openPathInExplorer,
  setCS2Path,
  selectCS2Directory,
  getBackupRetention,
  setBackupRetention,
} from '../utils/wails'

const props = defineProps<{
  visible: boolean
  steamPath: string
  backupPath: string
  /** 手动指定的 CS2 配置目录；空串表示自动探测。 */
  cs2Path: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved', payload: { steamPath: string; backupPath: string; cs2Path: string }): void
}>()

const steamInput = ref('')
const backupInput = ref('')
const cs2Input = ref('')
const retentionInput = ref(10)
const saving = ref(false)
const detecting = ref(false)
const errorText = ref('')
const dialogRef = ref<HTMLElement | null>(null)

useFocusTrap(
  computed(() => props.visible),
  dialogRef
)

watch(
  () => props.visible,
  (val) => {
    if (val) {
      steamInput.value = props.steamPath || ''
      backupInput.value = props.backupPath || ''
      cs2Input.value = props.cs2Path || ''
      errorText.value = ''
      void loadRetention()
    }
  },
  { immediate: true }
)

const handleBrowseSteam = async () => {
  try {
    const picked = await selectSteamDirectory()
    if (picked) steamInput.value = picked
  } catch (err: any) {
    errorText.value = `选择目录失败: ${err?.message || err}`
  }
}

const handleDetectSteam = async () => {
  detecting.value = true
  errorText.value = ''
  try {
    const detected = await detectSteamPath()
    if (detected) steamInput.value = detected
  } catch (err: any) {
    errorText.value = `自动侦测失败: ${err?.message || err}`
  } finally {
    detecting.value = false
  }
}

const handleBrowseBackup = async () => {
  try {
    const picked = await selectBackupDirectory()
    if (picked) backupInput.value = picked
  } catch (err: any) {
    errorText.value = `选择目录失败: ${err?.message || err}`
  }
}

const handleBrowseCS2 = async () => {
  try {
    const picked = await selectCS2Directory()
    if (picked) cs2Input.value = picked
  } catch (err: any) {
    errorText.value = `选择目录失败: ${err?.message || err}`
  }
}

/** 清空即恢复自动探测（后端会读 Steam 库配置，覆盖装在非默认库的 CS2）。 */
const handleClearCS2 = () => {
  cs2Input.value = ''
}

const handleOpen = async (path: string) => {
  if (!path) return
  try {
    await openPathInExplorer(path)
  } catch (err: any) {
    errorText.value = `打开目录失败: ${err?.message || err}`
  }
}

// Esc 关闭（覆盖层不可聚焦，需在 window 上监听）
const handleGlobalKeydown = (e: KeyboardEvent) => {
  if (!props.visible) return
  if (e.key === 'Escape') {
    e.preventDefault()
    emit('close')
  }
}

onMounted(() => window.addEventListener('keydown', handleGlobalKeydown))
onUnmounted(() => window.removeEventListener('keydown', handleGlobalKeydown))

/**
 * 读当前的历史版本上限。读不到就用默认 10，不报错打断用户。
 *
 * 用函数声明而不是 const 箭头函数：上面那个 `watch(..., { immediate: true })`
 * 在 setup 期间就会调它，const 会踩 TDZ。
 */
async function loadRetention() {
  try {
    const n = await getBackupRetention()
    retentionInput.value = n > 0 ? n : 10
  } catch {
    retentionInput.value = 10
  }
}

const handleSave = async () => {
  const sp = steamInput.value.trim()
  const bp = backupInput.value.trim()
  if (!sp) {
    errorText.value = '请填写 Steam 安装目录'
    return
  }
  if (!bp) {
    errorText.value = '请填写备份仓库目录'
    return
  }
  saving.value = true
  errorText.value = ''
  try {
    if (sp !== props.steamPath) await setSteamPath(sp)
    if (bp !== props.backupPath) await setBackupPath(bp)
    // CS2 目录允许留空：留空 = 自动探测
    const cp = cs2Input.value.trim()
    if (cp !== props.cs2Path) await setCS2Path(cp)
    await setBackupRetention(Math.max(0, Math.floor(retentionInput.value) || 0))
    emit('saved', { steamPath: sp, backupPath: bp, cs2Path: cp })
    emit('close')
  } catch (err: any) {
    errorText.value = `保存路径失败: ${err?.message || err}`
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <Teleport to="body">
    <div v-if="visible" class="path-overlay" @click.self="emit('close')">
      <div ref="dialogRef" class="path-dialog" role="dialog" aria-modal="true" aria-labelledby="path-dialog-title" tabindex="-1">
        <header class="dialog-header">
          <div class="header-text">
            <h3 id="path-dialog-title" class="dialog-title">路径设置</h3>
            <p class="dialog-subtitle">配置 Steam 客户端安装目录与备份数据存放位置</p>
          </div>
          <button class="btn-close" title="关闭" @click="emit('close')">
            <SvgIcon name="close" :size="14" />
          </button>
        </header>

        <div class="dialog-body">
          <div class="setting-row">
            <div class="setting-label">
              <span class="label-text">Steam 安装目录</span>
              <span class="label-hint">用于读取 userdata 与 config 下的账号信息</span>
            </div>
            <div class="setting-control">
              <input v-model="steamInput" type="text" class="input-control mono-font" data-autofocus placeholder="例如 C:\Program Files (x86)\Steam" />
              <div class="control-actions">
                <button class="btn btn-secondary btn-sm" title="浏览并选择 Steam 安装根目录" @click="handleBrowseSteam">
                  <SvgIcon name="folder" :size="13" /><span>浏览</span>
                </button>
                <button class="btn btn-secondary btn-sm" :disabled="detecting" title="自动查找本机的 Steam 安装目录" @click="handleDetectSteam">
                  <SvgIcon name="zap" :size="13" :class="{ spinning: detecting }" /><span>自动侦测</span>
                </button>
                <button class="btn btn-secondary btn-sm" :disabled="!steamInput" title="在资源管理器中打开" @click="handleOpen(steamInput)">
                  <SvgIcon name="external-link" :size="13" />
                </button>
              </div>
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-label">
              <span class="label-text">CS2 安装目录</span>
              <span class="label-hint">
                留空则自动探测。装了多份 CS2、或游戏不在 Steam 默认库时在这里指定
              </span>
            </div>
            <div class="setting-control">
              <input
                v-model="cs2Input"
                type="text"
                class="input-control mono-font"
                placeholder="留空自动探测；可填 CS2 安装目录或 game/csgo/cfg"
                title="选到哪一层都行：CS2 安装目录、Counter-Strike Global Offensive、game/csgo 或 cfg，保存时会自动归一到 game/csgo/cfg"
              />
              <div class="control-actions">
                <button
                  class="btn btn-secondary btn-sm"
                  title="浏览并选择 CS2 安装目录（任意层级都会自动归一）"
                  @click="handleBrowseCS2"
                >
                  <SvgIcon name="folder" :size="13" /><span>浏览</span>
                </button>
                <button
                  class="btn btn-secondary btn-sm"
                  :disabled="!cs2Input"
                  title="清空并恢复自动探测"
                  @click="handleClearCS2"
                >
                  <SvgIcon name="refresh" :size="13" /><span>自动</span>
                </button>
                <button
                  class="btn btn-secondary btn-sm"
                  :disabled="!cs2Input"
                  title="在资源管理器中打开"
                  @click="handleOpen(cs2Input)"
                >
                  <SvgIcon name="external-link" :size="13" />
                </button>
              </div>
            <div class="form-col">
              <label class="field-label">历史版本上限:</label>
              <input
                v-model.number="retentionInput"
                type="number"
                min="1"
                max="100"
                class="input-control"
                title="每个账号保留多少个历史备份版本；超出时自动清理最旧的。填 0 表示不清理"
              />
            </div>
            </div>
          </div>

          <div class="setting-row">
            <div class="setting-label">
              <span class="label-text">备份仓库目录</span>
              <span class="label-hint">快照、通用 CFG 与局域网安装包的存放位置</span>
            </div>
            <div class="setting-control">
              <input v-model="backupInput" type="text" class="input-control mono-font" placeholder="例如 D:\CS2_Backups" />
              <div class="control-actions">
                <button class="btn btn-secondary btn-sm" title="浏览并选择备份仓库目录" @click="handleBrowseBackup">
                  <SvgIcon name="folder" :size="13" /><span>浏览</span>
                </button>
                <button class="btn btn-secondary btn-sm" :disabled="!backupInput" title="在资源管理器中打开" @click="handleOpen(backupInput)">
                  <SvgIcon name="external-link" :size="13" />
                </button>
              </div>
            </div>
          </div>

          <div v-if="errorText" class="error-banner">
            <SvgIcon name="alert-triangle" :size="14" />
            <span>{{ errorText }}</span>
          </div>
        </div>

        <footer class="dialog-footer">
          <button class="btn btn-secondary" title="放弃本次修改，不会写入任何配置" @click="emit('close')">取消</button>
          <button class="btn btn-primary" :disabled="saving" title="校验并保存三条路径" @click="handleSave">
            <span v-if="saving" class="spinner-small"></span>
            <span>{{ saving ? '正在保存...' : '保存' }}</span>
          </button>
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.path-overlay {
  position: fixed;
  inset: 0;
  z-index: 9998;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background-color: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(2px);
}

.path-dialog {
  width: 100%;
  max-width: 560px;
  background-color: #2c2c2c;
  border: 1px solid rgba(255, 255, 255, 0.09);
  border-radius: var(--radius-card);
  box-shadow: var(--shadow-dialog);
  display: flex;
  flex-direction: column;
  animation: dialogIn var(--duration-normal) var(--ease-standard);
}

@keyframes dialogIn {
  from {
    opacity: 0;
    transform: translateY(8px) scale(0.99);
  }
  to {
    opacity: 1;
    transform: none;
  }
}

.dialog-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 20px 24px 12px;
}

.dialog-title {
  font-family: var(--font-display);
  font-size: 18px;
  font-weight: 600;
  line-height: 24px;
}

.dialog-subtitle {
  margin-top: 2px;
  font-size: 12px;
  color: var(--color-text-muted);
}

.btn-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  background: transparent;
  border: none;
  border-radius: var(--radius-control);
  color: var(--color-text-secondary);
  cursor: pointer;
  transition: background-color var(--duration-fast) var(--ease-standard);
}

.btn-close:hover {
  background: var(--bg-control-hover);
  color: var(--color-text);
}

.dialog-body {
  display: flex;
  flex-direction: column;
  gap: 18px;
  padding: 4px 24px 20px;
}

.setting-row {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.setting-label {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.label-text {
  font-size: 13px;
  font-weight: 600;
}

.label-hint {
  font-size: 12px;
  color: var(--color-text-muted);
}

.setting-control {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.control-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.error-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: var(--radius-control);
  background: var(--status-error-fill);
  border: 1px solid var(--status-error-stroke);
  color: var(--level-error);
  font-size: 12px;
}

.dialog-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 16px 24px;
  border-top: 1px solid var(--border-color);
}

</style>
