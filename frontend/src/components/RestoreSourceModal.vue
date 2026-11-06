<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import type { SnapshotInfo, UserCardInfo } from '../types'
import SvgIcon from './SvgIcon.vue'
import { useFocusTrap } from '../utils/a11y'

interface Props {
  visible: boolean
  targetAccountId: string
  targetUsername?: string
  backedUpAccounts: string[]
  snapshots?: SnapshotInfo[]
  users?: UserCardInfo[]
  remoteBackups?: UserCardInfo[]
}

const props = withDefaults(defineProps<Props>(), {
  targetUsername: '',
  snapshots: () => [],
  users: () => [],
  remoteBackups: () => [],
})

const emit = defineEmits<{
  (e: 'confirm', payload: { type: 'self' | 'other' | 'snapshot'; sourceId: string }): void
  (e: 'close'): void
}>()

export type RestoreSourceType = 'self' | 'other' | 'snapshot'

const selectedType = ref<RestoreSourceType>('self')
const selectedOtherAccount = ref('')
const selectedSnapshotId = ref('')
const dialogRef = ref<HTMLElement | null>(null)

useFocusTrap(
  computed(() => props.visible),
  dialogRef
)

const hasSelfBackup = computed(() => {
  return props.backedUpAccounts.includes(props.targetAccountId)
})

const otherAccounts = computed(() => {
  return props.backedUpAccounts.filter((id) => id !== props.targetAccountId)
})

watch(
  () => props.visible,
  (val) => {
    if (val) {
      if (hasSelfBackup.value) {
        selectedType.value = 'self'
      } else if (props.snapshots.length > 0) {
        selectedType.value = 'snapshot'
        selectedSnapshotId.value = props.snapshots[0].id
      } else if (otherAccounts.value.length > 0) {
        selectedType.value = 'other'
        selectedOtherAccount.value = otherAccounts.value[0]
      } else {
        selectedType.value = 'self'
      }

      if (otherAccounts.value.length > 0 && !selectedOtherAccount.value) {
        selectedOtherAccount.value = otherAccounts.value[0]
      }
      if (props.snapshots.length > 0 && !selectedSnapshotId.value) {
        selectedSnapshotId.value = props.snapshots[0].id
      }
    }
  }
)

// 账号 ID → 昵称，避免下拉里只有一串数字
const displayName = (accountId: string): string => {
  const u = props.users.find((x) => x.account_id === accountId)
  if (u) return u.username || accountId

  // 远程备份不是 Steam 账号，用下载时记录的信息代替原始 ID
  const remote = props.remoteBackups.find((x) => x.account_id === accountId)
  if (remote) return remote.username || '远程备份'

  // 兜底：remote_YYYYMMDD_HHMMSS
  const m = /^remote_(\d{8})_(\d{6})$/.exec(accountId)
  if (m) {
    const d = m[1]
    const t = m[2]
    return `远程备份 ${d.slice(0, 4)}/${d.slice(4, 6)}/${d.slice(6, 8)} ${t.slice(0, 2)}:${t.slice(2, 4)}`
  }
  return accountId
}

const accountHint = (accountId: string): string => {
  const remote = props.remoteBackups.find((x) => x.account_id === accountId)
  if (remote?.notes) return remote.notes
  return ''
}

const formatSnapshotTime = (iso: string): string => {
  if (!iso) return ''
  const d = new Date(iso)
  if (isNaN(d.getTime())) return iso
  return d.toLocaleString('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

const canConfirm = computed(() => {
  if (selectedType.value === 'self') {
    return hasSelfBackup.value
  }
  if (selectedType.value === 'other') {
    return Boolean(selectedOtherAccount.value)
  }
  if (selectedType.value === 'snapshot') {
    return Boolean(selectedSnapshotId.value)
  }
  return false
})

const handleConfirm = () => {
  if (!canConfirm.value) return

  let sourceId = props.targetAccountId
  if (selectedType.value === 'other') {
    sourceId = selectedOtherAccount.value
  } else if (selectedType.value === 'snapshot') {
    sourceId = selectedSnapshotId.value
  }

  emit('confirm', {
    type: selectedType.value,
    sourceId,
  })
}

const handleClose = () => {
  emit('close')
}

// 覆盖层默认不持有焦点，Esc 需在 window 上监听
const handleGlobalKeydown = (e: KeyboardEvent) => {
  if (props.visible && e.key === 'Escape') {
    e.preventDefault()
    handleClose()
  }
}

onMounted(() => window.addEventListener('keydown', handleGlobalKeydown))
onUnmounted(() => window.removeEventListener('keydown', handleGlobalKeydown))
</script>

<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="restore-source-overlay"
      @click.self="handleClose"
    >
      <div ref="dialogRef" class="restore-source-box acrylic-card" role="dialog" aria-modal="true" aria-labelledby="restore-source-title" tabindex="-1">
        <!-- Header -->
        <div class="modal-header">
          <div class="header-title-row">
            <SvgIcon name="restore" :size="20" class="header-icon" />
            <div class="header-text">
              <h3 id="restore-source-title" class="modal-title">选择配置还原来源</h3>
              <p class="modal-subtitle">
                目标账号: <span class="highlight">{{ targetUsername || targetAccountId }}</span> (ID: {{ targetAccountId }})
              </p>
            </div>
          </div>
          <button class="btn-icon-close" title="关闭" @click="handleClose">
            <SvgIcon name="close" :size="14" />
          </button>
        </div>

        <!-- Body: Sources List -->
        <div class="modal-body">
          <div class="source-options">
            <!-- Option 1: Self Backup -->
            <label
              class="source-option-card"
              :class="{
                selected: selectedType === 'self',
                disabled: !hasSelfBackup,
              }"
            >
              <div class="option-radio">
                <input
                  v-model="selectedType"
                  type="radio"
                  value="self"
                  :disabled="!hasSelfBackup"
                />
              </div>
              <div class="option-content">
                <div class="option-title-row">
                  <span class="option-title">当前账号的基础最新备份</span>
                  <span v-if="hasSelfBackup" class="badge-status badge-ready">可用</span>
                  <span v-else class="badge-status badge-none">暂无备份</span>
                </div>
                <p class="option-desc">
                  从本地 userdata_backup/{{ targetAccountId }} 还原该账号之前备份的默认配置。
                </p>
              </div>
            </label>

            <!-- Option 2: Clone from other account -->
            <label
              class="source-option-card"
              :class="{
                selected: selectedType === 'other',
                disabled: otherAccounts.length === 0,
              }"
            >
              <div class="option-radio">
                <input
                  v-model="selectedType"
                  type="radio"
                  value="other"
                  :disabled="otherAccounts.length === 0"
                />
              </div>
              <div class="option-content">
                <div class="option-title-row">
                  <span class="option-title">从其他已备份账号克隆配置 (跨号还原)</span>
                  <span v-if="otherAccounts.length > 0" class="badge-status badge-count">
                    {{ otherAccounts.length }} 个账号可用
                  </span>
                  <span v-else class="badge-status badge-none">无其他备份</span>
                </div>
                <p class="option-desc">
                  将另一已备份账号的配置复制并还原至当前账号（适合小号克隆大号键位与画质）。
                </p>

                <!-- Dropdown for other accounts -->
                <div
                  v-if="selectedType === 'other' && otherAccounts.length > 0"
                  class="option-subcontrol"
                  @click.stop
                >
                  <label class="subcontrol-label">选择来源账号:</label>
                  <select
                    v-model="selectedOtherAccount"
                    class="select-control subcontrol-select"
                  >
                    <option
                      v-for="accId in otherAccounts"
                      :key="accId"
                      :value="accId"
                    >
                      {{ displayName(accId) }}{{ accountHint(accId) ? ` · ${accountHint(accId)}` : ` · ID ${accId}` }}
                    </option>
                  </select>
                </div>
              </div>
            </label>

            <!-- Option 3: Historical Snapshots -->
            <label
              class="source-option-card"
              :class="{
                selected: selectedType === 'snapshot',
                disabled: snapshots.length === 0,
              }"
            >
              <div class="option-radio">
                <input
                  v-model="selectedType"
                  type="radio"
                  value="snapshot"
                  :disabled="snapshots.length === 0"
                />
              </div>
              <div class="option-content">
                <div class="option-title-row">
                  <span class="option-title">从历史快照版本恢复</span>
                  <span v-if="snapshots.length > 0" class="badge-status badge-count">
                    {{ snapshots.length }} 个快照版本
                  </span>
                  <span v-else class="badge-status badge-none">暂无快照</span>
                </div>
                <p class="option-desc">
                  从该账号历史创建的时间戳快照副本中还原指定时刻的配置存档。
                </p>

                <!-- Dropdown for snapshots -->
                <div
                  v-if="selectedType === 'snapshot' && snapshots.length > 0"
                  class="option-subcontrol"
                  @click.stop
                >
                  <label class="subcontrol-label">选择快照版本:</label>
                  <select
                    v-model="selectedSnapshotId"
                    class="select-control subcontrol-select"
                  >
                    <option
                      v-for="snap in snapshots"
                      :key="snap.id"
                      :value="snap.id"
                    >
                      {{ formatSnapshotTime(snap.created_at) || snap.id }} {{ snap.note ? `· ${snap.note}` : '' }} · {{ snap.total_files }} 个文件
                    </option>
                  </select>
                </div>
              </div>
            </label>
          </div>

          <!-- Rollback Notice Banner -->
          <div class="rollback-tip-banner">
            <SvgIcon name="info" :size="14" class="tip-icon" />
            <span>还原前系统将自动保留当前配置的回退备份点，还原后可在卡片上随时一键【撤销还原】。</span>
          </div>
        </div>

        <!-- Footer -->
        <div class="modal-footer">
          <button class="btn btn-secondary" @click="handleClose">
            取消
          </button>
          <button
            class="btn btn-primary"
            :disabled="!canConfirm"
            @click="handleConfirm"
          >
            <SvgIcon name="restore" :size="13" />
            <span>确认还原</span>
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.restore-source-overlay {
  position: fixed;
  inset: 0;
  z-index: 9998;
  background-color: rgba(0, 0, 0, 0.65);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}

.restore-source-box {
  width: 100%;
  max-width: 540px;
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.6);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  animation: modalIn 0.15s cubic-bezier(0, 0, 0.2, 1);
}

@keyframes modalIn {
  from {
    opacity: 0;
    transform: scale(0.97) translateY(8px);
  }
  to {
    opacity: 1;
    transform: scale(1) translateY(0);
  }
}

.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-color);
  background-color: var(--bg-surface);
}

.header-title-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.header-icon {
  color: var(--accent);
}

.header-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
}


.modal-subtitle {
  margin: 0;
  font-size: 12px;
  color: var(--color-text-secondary);
}

.modal-subtitle .highlight {
  color: var(--accent);
  font-weight: 600;
}

.btn-icon-close {
  background: transparent;
  border: none;
  color: var(--color-text-muted);
  width: 28px;
  height: 28px;
  border-radius: var(--radius-sm);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-icon-close:hover {
  background-color: var(--bg-control-hover);
  color: var(--color-text);
}

.modal-body {
  padding: 18px 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  max-height: 70vh;
  overflow-y: auto;
}

.source-options {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.source-option-card {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px 14px;
  background-color: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all 0.15s ease;
}

.source-option-card:hover:not(.disabled) {
  background-color: var(--bg-card-hover);
  border-color: var(--stroke-3);
}

.source-option-card.selected {
  border-color: var(--accent);
  background-color: var(--tone-accent-weak);
}

.source-option-card.disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.option-radio {
  margin-top: 3px;
}

.option-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.option-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.option-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
}

.option-desc {
  margin: 0;
  font-size: 12px;
  color: var(--color-text-muted);
  line-height: 1.4;
}

.badge-status {
  font-size: 12px;
  padding: 1px 7px;
  border-radius: var(--radius-sm);
  font-weight: 500;
}

.badge-ready {
  background-color: rgba(16, 124, 65, 0.15);
  color: #107c41;
  border: 1px solid rgba(16, 124, 65, 0.3);
}

.badge-count {
  background-color: rgba(0, 120, 212, 0.12);
  color: var(--accent);
  border: 1px solid var(--tone-accent-mid);
}

.badge-none {
  background-color: rgba(255, 255, 255, 0.05);
  color: var(--color-text-muted);
  border: 1px solid var(--surface-4);
}

.option-subcontrol {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px dashed var(--border-color);
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.subcontrol-label {
  font-size: 12px;
  color: var(--color-text-secondary);
}

.subcontrol-select {
  width: 100%;
}

.rollback-tip-banner {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  border-radius: var(--radius-sm);
  background-color: var(--tone-accent-weak);
  border: 1px solid rgba(0, 120, 212, 0.2);
  color: var(--color-text-secondary);
  font-size: 12px;
  line-height: 1.4;
}

.tip-icon {
  color: var(--accent);
  flex-shrink: 0;
}

.modal-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  padding: 14px 20px;
  border-top: 1px solid var(--border-color);
  background-color: var(--bg-surface);
}
</style>
