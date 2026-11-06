<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import type { UserCardInfo, SnapshotInfo, VideoPreset, CrosshairSaveResult } from '../types'
import SvgIcon from '../components/SvgIcon.vue'
import DiffModal from '../components/DiffModal.vue'
import VideoSettingsModal from '../components/VideoSettingsModal.vue'
import CrosshairModal from '../components/CrosshairModal.vue'
import TextEditorModal from '../components/TextEditorModal.vue'
import ConfirmModal from '../components/ConfirmModal.vue'
import RestoreSourceModal from '../components/RestoreSourceModal.vue'
import PathSettingsModal from '../components/PathSettingsModal.vue'
import GameCfgCard from '../components/GameCfgCard.vue'
import CfgVersionPickerModal from '../components/CfgVersionPickerModal.vue'
import AccountConfigCard from '../components/AccountConfigCard.vue'
import defaultAvatar from '../assets/default_avatar.jpg'
import {
  subscribeToUserUpdates,
  openPathInExplorer,
  getCrosshairState,
  getBackupRetention,
  listGameCFGVersions,
} from '../utils/wails'
import { describeCrosshair } from '../utils/crosshairGeometry'
import { describeCrosshairToggle, type AccountSummary } from '../utils/accountSummary'
import { useToast } from '../composables/useToast'
import { useConfirm } from '../composables/useConfirm'
import { useCrosshair } from '../composables/useCrosshair'
import { usePaths } from '../composables/usePaths'
import { useSteamProcess } from '../composables/useSteamProcess'
import { useVideoSummary } from '../composables/useVideoSummary'
import { useGameCfg } from '../composables/useGameCfg'
import { useLaunchOptions } from '../composables/useLaunchOptions'
import { useBackup, type RestoreSourcePayload } from '../composables/useBackup'
import { useSnapshots } from '../composables/useSnapshots'
import { useAccounts } from '../composables/useAccounts'
import { useModals } from '../composables/useModals'

defineProps<{
  os?: string
  arch?: string
}>()

const emit = defineEmits<{
  (e: 'log-action', message: string): void
}>()

// State
const actionLoading = ref(false)

// 提示条与确认框各是一块独立状态，见 composables/。
const { errorMsg, successMsg, showMessage } = useToast()
const { confirmState, showConfirm, onConfirmModalConfirm, onConfirmModalCancel } = useConfirm()

// 准星摘要与编辑器弹窗：账号卡上只展示关键值与分享码，完整编辑在弹窗里。
const {
  modalVisible: crosshairModalVisible,
  summary: crosshairSummary,
  shareCode: crosshairShareCode,
  toggle: crosshairToggle,
  load: loadCrosshairSummary,
  open: openCrosshairModal,
  close: closeCrosshairModal,
  handleSaved: handleCrosshairSaved,
} = useCrosshair({
  showMessage,
  logAction: (msg) => emit('log-action', msg),
})

// 三个路径（Steam / 备份 / 手动 CS2 目录）与「打开资源管理器」。
const {
  steamPathInput,
  backupPathInput,
  cs2PathInput,
  cs2CfgPath,
  pathModalVisible,
  loadSteamPath,
  loadBackupPath,
  loadCS2CFGPath,
  loadCS2PathOverride,
  openPathSettings,
  handlePathSaved,
  handleAutoDetect,
  openSteamInExplorer: handleOpenSteamInExplorer,
  openBackupInExplorer: handleOpenBackupInExplorer,
} = usePaths({
  showMessage,
  logAction: (msg) => emit('log-action', msg),
  reloadAfterPathChange: async () => {
    await loadUsers()
    await loadCS2CFGPath()
    await loadBackedUpAccounts()
    await loadCfgFiles()
    await loadCrosshairSummary()
  },
})

// Steam 客户端进程与账号切换。
const {
  isSteamActive,
  steamCheckLoading,
  steamActionLoading,
  switchModalVisible,
  switchTargetUser,
  switchActionLoading,
  checkSteamStatus,
  shutdown: handleManualShutdownSteam,
  start: handleStartSteam,
  requestSwitch: handleSwitchAccount,
  confirmSwitch: confirmSwitchAccount,
  closeSwitchModal,
} = useSteamProcess({
  showMessage,
  logAction: (msg) => emit('log-action', msg),
  reloadUsers: () => loadUsers(),
})

// 账号卡片上的画质摘要与右侧两个快捷动作。纯计算在 utils/videoSummary.ts。
const video = useVideoSummary({
  showMessage,
  logAction: (msg) => emit('log-action', msg),
  setBusy: (busy) => {
    actionLoading.value = busy
  },
})

const {
  resolutionText: videoResolutionText,
  refreshRateText: videoRefreshRateText,
  summaryText: videoSummaryText,
  defaultPreset: videoDefaultPreset,
  defaultPresetName: videoDefaultPresetName,
  rawSettings: currentRawVideoSettings,
  readOnly: isVideoReadOnly,
  loading: videoSettingsLoading,
  isDefaultPresetApplied,
  load: loadAccountVideoSummary,
} = video

/** 摘要条上的「锁定 / 解锁」按钮：作用于当前选中账号。 */
const handleToggleReadOnlyLock = () => video.toggleReadOnly(selectedAccountId.value)

/** 摘要条上的「套用默认预设」按钮：没传账号时作用于当前选中账号。 */
const handleApplyDefaultVideoPresetQuick = (u?: UserCardInfo) => {
  const target = u || selectedUser.value
  if (!target) return
  return video.applyDefaultPresetTo(target.account_id, target.username)
}

// 共享游戏 CFG 目录的扫描结果与勾选。清单由后端扫描得出，不维护硬编码列表。
const {
  files: cfgFiles,
  selected: cfgSelected,
  scanning: cfgScanning,
  quickFiles: quickCfgFiles,
  load: loadCfgFiles,
  selectAll: selectAllCfgFiles,
  selectOnlyCustom: selectOnlyCustomCfg,
  toggle: toggleCfgFile,
  backupAll: handleBackupAllCFG,
  backupCustom: handleBackupCustomCFG,
  backupPicked: handleBackupSelectedCFG,
  backupChecked: handleBackupCheckedCFG,
  restore: restoreGameCFGNow,
  rollback: handleRollbackCFG,
} = useGameCfg({
  showMessage,
  logAction: (msg) => emit('log-action', msg),
  setBusy: (busy) => {
    actionLoading.value = busy
  },
  confirm: (opts) => showConfirm(opts),
  cfgDir: () => cs2CfgPath.value,
  afterRestore: async () => {
    await checkRollbackStatus()
    await loadGameCFGVersions()
  },
})

/** 通用 CFG 的历史版本，最新的在前。用来给「还原」提供版本选择。 */
const cfgVersions = ref<string[]>([])

async function loadGameCFGVersions() {
  try {
    cfgVersions.value = await listGameCFGVersions()
  } catch {
    cfgVersions.value = []
  }
}

/** 还原通用 CFG：先弹版本选择，选完再走确认框。 */
const cfgVersionPickerVisible = ref(false)

const handleRestoreCFG = async () => {
  await loadGameCFGVersions()
  cfgVersionPickerVisible.value = true
}

const confirmRestoreCFG = (version: string) => {
  cfgVersionPickerVisible.value = false
  restoreGameCFGNow(version || undefined)
}

// 画质设置弹窗与内置文本编辑器的开关状态。
const modals = useModals({
  showMessage,
  logAction: (msg) => emit('log-action', msg),
  selectedUser: () => selectedUser.value,
  selectedAccountID: () => selectedAccountId.value,
  reloadVideoSummary: (aid) => loadAccountVideoSummary(aid),
})

const {
  videoVisible: videoModalVisible,
  videoTargetAccountId,
  videoTargetUsername,
  textEditorVisible: isTextEditorOpen,
  textEditorFile: textEditorInitialFile,
  textEditorScope: textEditorInitialScope,
  openVideo: openVideoSettingsModal,
  openVideoGuarded: openVideoModal,
  closeVideo: closeVideoModal,
  handleVideoSaved: handleVideoSettingsSaved,
  openTextEditor,
  openEditorFromDiff: handleOpenEditorFromDiff,
  closeTextEditor: closeTextEditorModal,
  handleTextEditorSaved,
} = modals

// CS2 启动项：读取、快捷开关、编辑保存。纯文本处理在 utils/launchOptions.ts。
const launch = useLaunchOptions({
  showMessage,
  logAction: (msg) => emit('log-action', msg),
  setBusy: (busy) => {
    actionLoading.value = busy
  },
  copy: (text, onSuccess) => {
    void copyToClipboard(text, onSuccess)
  },
  markSteamStopped: () => {
    isSteamActive.value = false
  },
})

const {
  current: currentLaunchOptions,
  loading: launchOptionsLoading,
  editVisible: editLaunchOptionsVisible,
  editInput: editLaunchOptionsInput,
  steamPromptVisible: launchOptionsSteamPromptVisible,
  copySuccess: copyOptionsSuccess,
  load: loadLaunchOptions,
  copyCurrent: handleCopyLaunchOptions,
  openEdit: openEditLaunchOptions,
  isActive: isPresetActive,
  isActiveInModal: isPresetActiveInModal,
  addPreset,
  toggleInModal: togglePresetInModal,
} = launch

/** 摘要条上的快捷参数：作用于当前选中账号。 */
const handleToggleLaunchPreset = (preset: string) => launch.toggle(selectedAccountId.value, preset)
/** 编辑弹窗保存：作用于当前选中账号。 */
const handleSaveLaunchOptions = () => launch.save(selectedAccountId.value)
/** 「需要先退出 Steam」确认框：作用于当前选中账号。 */
const confirmSaveLaunchOptions = (shouldExitSteam: boolean) =>
  launch.confirmSave(selectedAccountId.value, shouldExitSteam)

// 用户配置的备份 / 还原 / 撤销还原。
const backupHook = useBackup({
  showMessage,
  logAction: (msg) => emit('log-action', msg),
  setBusy: (busy) => {
    actionLoading.value = busy
  },
  confirm: (opts) => showConfirm(opts),
  reloadSnapshots: (aid) => loadSnapshots(aid),
  reloadAccountDetail: async (aid) => {
    await loadAccountVideoSummary(aid)
    await loadLaunchOptions(aid)
  },
})

const {
  backedUpAccounts,
  remoteBackups,
  canRollbackUser,
  canRollbackGameCFG,
  restoreSourceModalVisible,
  loadBackedUpAccounts,
  closeRestoreSource: closeRestoreSourceModal,
  offerPrune,
} = backupHook

/** 备份当前选中账号。 */
const handleBackupSelectedUser = () => backupHook.backup(selectedAccountId.value)
/** 打开「还原来源」弹窗。 */
const openRestoreSourceModal = () => backupHook.openRestoreSource(selectedAccountId.value)
/** 「还原来源」弹窗点了确认。 */
const handleConfirmRestoreSource = (payload: RestoreSourcePayload) =>
  backupHook.confirmRestore(selectedAccountId.value, payload)
/** 撤销当前账号的上一次还原。 */
const handleRollbackUser = () => backupHook.rollbackUser(selectedAccountId.value)
/** 重查撤销状态。 */
const checkRollbackStatus = () => backupHook.checkRollbackStatus(selectedAccountId.value)

// 配置快照：列表、新建、还原、删除、差异比对。
const snaps = useSnapshots({
  showMessage,
  logAction: (msg) => emit('log-action', msg),
  setBusy: (busy) => {
    actionLoading.value = busy
  },
  confirm: (opts) => showConfirm(opts),
  reloadBackedUpAccounts: () => loadBackedUpAccounts(),
  reloadAccountDetail: async (aid) => {
    await checkRollbackStatus()
    await loadAccountVideoSummary(aid)
    await loadLaunchOptions(aid)
  },
})

const {
  list: snapshots,
  loading: snapshotsLoading,
  note: newSnapshotNote,
  modalVisible: snapshotsModalVisible,
  diffVisible: diffModalVisible,
  diffSnapshotId: diffModalSnapshotId,
  diffItems,
  diffLoading,
  load: loadSnapshots,
  close: closeSnapshotsModal,
  closeDiff: closeDiffModal,
} = snaps

/** 打开快照面板。 */
const openSnapshotsModal = () => snaps.open(selectedAccountId.value)
/** 为当前账号新建快照。 */
const handleCreateSnapshot = () => snaps.create(selectedAccountId.value)
/** 还原某个快照。 */
const handleRestoreSnapshot = (snap: SnapshotInfo) => snaps.restore(selectedAccountId.value, snap)
/** 删除某个快照。 */
const handleDeleteSnapshot = (snap: SnapshotInfo) => snaps.remove(selectedAccountId.value, snap)
/** 比对快照与本地实时配置。 */
const handleOpenDiff = (snap: SnapshotInfo) => snaps.openDiff(selectedAccountId.value, snap)

// Steam 账号列表：加载、选中、筛选、搜索、备注、复制标识符。这是工作台的中心块。
const accounts = useAccounts({
  showMessage,
  logAction: (msg) => emit('log-action', msg),
  setError: (msg) => {
    errorMsg.value = msg
  },
  copy: (text, onSuccess) => {
    void copyToClipboard(text, onSuccess)
  },
  backedUpAccounts,
  afterUsersLoaded: async (aid) => {
    await loadBackedUpAccounts()
    await loadCS2CFGPath()
    if (aid && aid !== 'game_cfg') {
      await loadSnapshots(aid)
      await loadLaunchOptions(aid)
      await loadAccountVideoSummary(aid)
    }
  },
})

const {
  users,
  loading,
  selectedAccountId,
  searchQuery,
  filterMode: accountFilterMode,
  editingAccountId,
  editingNoteText,
  copySuccessId,
  copySuccessSteam64,
  selectedUser,
  isSelectedUserBackedUp,
  backedUpCount,
  rememberedCount,
  filteredUsers,
  load: loadUsers,
  refreshIdentities: refreshUserIdentities,
  setFilter: setAccountFilter,
  startEditNote,
  saveNote,
  cancelEditNote,
  copyAccountId: handleCopyAccountId,
  copySteam64: handleCopySteam64,
} = accounts

// Rollback & Restore Source State

/**
 * 账号卡片要的两条摘要。拼字符串的活在这里做完，卡片只负责显示。
 * 原来这九个值各走一个 prop，父视图因此成了画质/准星显示细节的知情者。
 */
const accountSummary = computed<AccountSummary>(() => {
  const toggle = describeCrosshairToggle(crosshairToggle.value)
  return {
    video: {
      resolutionText: videoResolutionText.value,
      refreshRateText: videoRefreshRateText.value,
      readOnly: isVideoReadOnly.value,
      defaultPresetName: videoDefaultPreset.value?.name ?? null,
      defaultPresetApplied: isDefaultPresetApplied.value,
      loading: videoSettingsLoading.value,
    },
    crosshair: {
      text: crosshairSummary.value,
      shareCode: crosshairShareCode.value,
      toggleLabel: toggle.label,
      toggleOff: toggle.off,
    },
  }
})

// Account Filter & Quick Game CFG Presets
const resetFilters = () => {
  setAccountFilter('all')
  searchQuery.value = ''
}

const launchOptionPresets = [
  { label: '-novid', desc: '跳过开机动画' },
  { label: '-freq 240', desc: '240Hz 刷新率' },
  { label: '-freq 144', desc: '144Hz 刷新率' },
  { label: '+exec autoexec.cfg', desc: '自动执行 autoexec' },
  { label: '-tickrate 128', desc: '128 Tickrate' },
  { label: '+fps_max 0', desc: '解除帧率上限' },
  { label: '-high', desc: '高 CPU 优先级' },
  { label: '-allow_third_party_software', desc: '允许第三方录屏滤镜' },
]


// Snapshots and Config Editor modals state (WinUI 3 CommandBar hooks)
const openLaunchOptionsModal = () => {
  if (!selectedUser.value) {
    showMessage('请先选择一个 Steam 账号', true)
    return
  }
  openEditLaunchOptions()
}

// Methods

// User Actions
// CS2 Game CFG Actions
const selectUser = async (u: UserCardInfo) => {
  selectedAccountId.value = u.account_id
  emit('log-action', `选中账号: ${u.username || u.account_id} [ID: ${u.account_id}]`)
  await loadSnapshots(u.account_id)
  await loadLaunchOptions(u.account_id)
  await loadAccountVideoSummary(u.account_id)
  await checkRollbackStatus()
}

// Snapshot methods
const formatBytes = (bytes: number) => {
  if (!bytes || bytes <= 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

const formatDateTime = (iso: string) => {
  if (!iso) return ''
  try {
    const d = new Date(iso)
    if (isNaN(d.getTime())) return iso
    return d.toLocaleString('zh-CN', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
    })
  } catch {
    return iso
  }
}

const copyToClipboard = async (text: string, onSuccess?: () => void) => {
  try {
    await navigator.clipboard.writeText(text)
    if (onSuccess) onSuccess()
  } catch {
    showConfirm({
      title: '复制内容',
      message: '剪贴板权限受限，请手动复制以下内容：',
      prompt: true,
      promptDefault: text,
      showCancel: false,
      confirmText: '完成',
    })
  }
}

// Launch Options methods
// Video Settings Handlers
// 视图内自绘的弹层（快照 / 启动项 / 切号）缺少 Esc 关闭能力，这里统一兜底
const handleGlobalEsc = (e: KeyboardEvent) => {
  if (e.key !== 'Escape') return
  // 优先级更高的小弹窗自行处理 Esc，避免一次按键关闭两层
  if (confirmState.value.visible) return
  if (videoModalVisible.value || isTextEditorOpen.value || restoreSourceModalVisible.value || diffModalVisible.value) return

  if (launchOptionsSteamPromptVisible.value) {
    confirmSaveLaunchOptions(false)
  } else if (editLaunchOptionsVisible.value) {
    editLaunchOptionsVisible.value = false
  } else if (switchModalVisible.value) {
    closeSwitchModal()
  } else if (snapshotsModalVisible.value) {
    snapshotsModalVisible.value = false
  } else {
    return
  }
  e.preventDefault()
}

// 后台补到昵称 / 头像后只刷新卡片上的身份信息：
// 不动选中态、不重载快照与画质摘要，避免界面整体抖一下。
let identityTimer: number | undefined
let unsubscribeUserUpdates: (() => void) | null = null

onMounted(async () => {
  window.addEventListener('keydown', handleGlobalEsc)
  unsubscribeUserUpdates = subscribeToUserUpdates(() => {
    if (identityTimer) window.clearTimeout(identityTimer)
    identityTimer = window.setTimeout(refreshUserIdentities, 600)
  })
  await loadSteamPath()
  await loadBackupPath()
  await loadBackedUpAccounts()
  await loadUsers()
  await loadGameCFGVersions()
  await checkRollbackStatus()
  // 上限是这一版才加的：存量可能远超，先问一次再收敛。
  try {
    await offerPrune(selectedAccountId.value, await getBackupRetention())
  } catch {
    // 收敛是锦上添花，失败不该影响首屏
  }
  await checkSteamStatus()
  await loadCfgFiles()
  await loadCrosshairSummary()
  await loadCS2PathOverride()
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleGlobalEsc)
  unsubscribeUserUpdates?.()
  if (identityTimer) window.clearTimeout(identityTimer)
})
</script>

<template>
  <div class="local-backup-view">
    <!-- Global Floating Toast Notification Container (Non-intrusive, zero-shift) -->
    <transition-group name="toast-slide" tag="div" class="toast-floating-container">
      <div v-if="successMsg" key="success-toast" class="toast-item success-toast">
        <div class="toast-icon-wrap">
          <SvgIcon name="check" size="16" />
        </div>
        <div class="toast-body">
          <span class="toast-title">操作成功</span>
          <span class="toast-msg">{{ successMsg }}</span>
        </div>
        <button class="toast-close-btn" title="关闭通知" @click="successMsg = ''">
          <SvgIcon name="close" size="12" />
        </button>
      </div>

      <div v-if="errorMsg" key="error-toast" class="toast-item error-toast">
        <div class="toast-icon-wrap">
          <SvgIcon name="alert-triangle" size="16" />
        </div>
        <div class="toast-body">
          <span class="toast-title">操作提示与警告</span>
          <span class="toast-msg">{{ errorMsg }}</span>
        </div>
        <button class="toast-close-btn" title="关闭通知" @click="errorMsg = ''">
          <SvgIcon name="close" size="12" />
        </button>
      </div>
    </transition-group>

    <!-- Master-Detail Split Workbench Layout -->
    <div class="workbench-layout">
      <!-- Master Panel (Left, fixed ~360px) -->
      <aside class="workbench-master">
        <!-- Section 1: 环境状态（只读概览 + 设置入口，避免与右侧重复编辑） -->
        <div class="master-env-section">
          <div class="steam-status-pill" :class="{ 'status-active': isSteamActive }">
            <div class="status-indicator-group">
              <span class="status-dot" :class="{ 'pulse-active': isSteamActive }"></span>
              <span class="status-name">{{ isSteamActive ? 'Steam 运行中' : 'Steam 未运行' }}</span>
            </div>
            <div class="status-btn-group">
              <button
                v-if="isSteamActive"
                class="btn-status-pill btn-shutdown"
                :disabled="steamActionLoading"
                title="退出 Steam 客户端"
                @click="handleManualShutdownSteam"
              >
                <SvgIcon name="power" size="12" />
                <span>退出</span>
              </button>
              <button
                v-else
                class="btn-status-pill btn-start"
                :disabled="steamActionLoading"
                title="启动 Steam 客户端"
                @click="() => handleStartSteam(false)"
              >
                <SvgIcon name="play" size="12" />
                <span>启动</span>
              </button>
              <button
                class="btn-status-refresh"
                :disabled="steamCheckLoading"
                title="刷新 Steam 运行状态"
                @click="checkSteamStatus"
              >
                <SvgIcon name="refresh" size="12" :class="{ spinning: steamCheckLoading }" />
              </button>
            </div>
          </div>

          <div class="env-summary">
            <button class="env-line" title="在资源管理器中打开 Steam 目录" @click="handleOpenSteamInExplorer">
              <SvgIcon name="folder" size="12" class="env-line-icon" />
              <span class="env-line-text mono-font">{{ steamPathInput || '未配置 Steam 目录' }}</span>
            </button>
            <button class="env-line" title="在资源管理器中打开备份仓库" @click="handleOpenBackupInExplorer">
              <SvgIcon name="archive" size="12" class="env-line-icon" />
              <span class="env-line-text mono-font">{{ backupPathInput || '未配置备份仓库' }}</span>
            </button>
          </div>

          <button class="env-settings-btn" @click="openPathSettings">
            <SvgIcon name="settings" size="13" />
            <span>路径设置</span>
          </button>
        </div>

        <!-- Section 2: Account Search & Filter Bar -->
        <div class="master-search-bar master-header-panel">
          <div class="search-input-box">
            <SvgIcon name="search" size="14" class="search-box-icon" />
            <input
              v-model="searchQuery"
              type="text"
              class="search-text-input"
              placeholder="搜索玩家、SteamID、备注..."
            />
            <button
              v-if="searchQuery"
              class="btn-clear-search"
              title="清空搜索"
              @click="searchQuery = ''"
            >
              <SvgIcon name="close" size="11" />
            </button>
          </div>

          <div class="filter-segments" role="group" aria-label="账号筛选">
            <button
              class="filter-seg"
              :class="{ active: accountFilterMode === 'all' }"
              @click="setAccountFilter('all')"
            >
              全部 <span class="seg-count">{{ users.length }}</span>
            </button>
            <button
              class="filter-seg"
              :class="{ active: accountFilterMode === 'remembered' }"
              title="仅显示已记住密码、可免密切换的账号"
              @click="setAccountFilter('remembered')"
            >
              可登录 <span class="seg-count">{{ rememberedCount }}</span>
            </button>
            <button
              class="filter-seg"
              :class="{ active: accountFilterMode === 'backed_up' }"
              title="仅显示本地已建立备份的账号"
              @click="setAccountFilter('backed_up')"
            >
              已备份 <span class="seg-count">{{ backedUpCount }}</span>
            </button>
            <button
              class="btn-refresh-users"
              :disabled="loading"
              title="重新扫描 Steam 账号"
              @click="loadUsers"
            >
              <SvgIcon name="refresh" size="12" :class="{ spinning: loading }" />
            </button>
          </div>
        </div>

        <!-- Section 3: Master Accounts List (Scrollable) -->
        <div class="master-accounts-scroll">
          <!-- Loading State -->
          <div v-if="loading && users.length === 0" class="master-loading-card">
            <div class="spinner-small"></div>
            <span>正在扫描本地 Steam 账号...</span>
          </div>

          <!-- Empty State (No accounts or missing steam path) -->
          <div v-else-if="users.length === 0" class="master-empty-card">
            <div class="empty-icon-circle">
              <SvgIcon name="folder" size="24" />
            </div>
            <div class="empty-title">未检测到 Steam 账号</div>
            <p class="empty-desc">
              未在当前 Steam 目录下找到有效的 userdata 数据。请确认上方路径是否正确，或点击下方按钮自动侦测。
            </p>
            <button class="btn btn-primary btn-sm btn-auto-detect-empty" @click="handleAutoDetect">
              <SvgIcon name="zap" size="13" />
              <span>自动侦测 Steam 目录</span>
            </button>
          </div>

          <!-- No Filter Results -->
          <div v-else-if="filteredUsers.length === 0" class="master-no-results-card">
            <SvgIcon name="search" size="20" class="no-res-icon" />
            <div class="no-res-text">
              {{ accountFilterMode === 'remembered' ? '当前没有记住密码的账号' : accountFilterMode === 'backed_up' ? '当前没有已备份的账号' : `无匹配「${searchQuery}」的账号` }}
            </div>
            <button class="btn-clear-link" @click="resetFilters">重置筛选条件</button>
          </div>

          <!-- Accounts List -->
          <div v-else class="account-items-list">
            <div
              v-for="u in filteredUsers"
              :key="u.account_id"
              class="account-card"
              :class="{
                'selected-account': selectedAccountId === u.account_id,
                'recent-account': u.is_recent,
              }"
              @click="selectUser(u)"
            >
              <!-- Avatar -->
              <div class="account-card-left">
                <div class="avatar-container">
                  <!-- 没有头像时用内置默认头像兜底，避免列表里出现一排字母方块 -->
                  <img
                    :src="u.avatar_url || defaultAvatar"
                    :alt="u.username"
                    class="user-avatar-img"
                  />
                  <span v-if="u.is_recent" class="avatar-active-dot" title="当前登录账号"></span>
                </div>
              </div>

              <!-- Card Body -->
              <div class="account-card-body">
                <div class="account-header-line">
                  <span class="account-nickname" :title="u.username">
                    {{ u.username || '未知玩家' }}
                  </span>
                  <span v-if="u.is_recent" class="badge-active-recent" title="当前登录玩家 (loginusers.vdf)">
                    <span class="dot-pulse"></span>
                    <span>当前登录</span>
                  </span>
                  <span class="account-status-icons">
                    <SvgIcon
                      v-if="u.remember_password"
                      name="key"
                      size="12"
                      class="status-icon"
                      title="本地已记住密码，支持免密切换"
                    />
                    <SvgIcon
                      v-if="backedUpAccounts.includes(u.account_id)"
                      name="check"
                      size="12"
                      class="status-icon status-icon-ok"
                      title="本地已建立配置备份"
                    />
                  </span>
                  <button
                    v-if="!u.is_recent"
                    class="btn-switch-account"
                    :class="{ 'btn-switch-quick': u.remember_password }"
                    :title="u.remember_password ? '本地已记住密码，点击直接免密切换并启动 Steam' : '切换至此账号（Steam 将以该账号启动）'"
                    @click.stop="handleSwitchAccount(u)"
                  >
                    <SvgIcon :name="u.remember_password ? 'zap' : 'user'" size="11" />
                    <span>{{ u.remember_password ? '免密登录' : '登录此号' }}</span>
                  </button>
                </div>

                <div class="account-ids-line">
                  <span class="id-tag mono-font" :title="`AccountID: ${u.account_id}`">
                    <span class="id-tag-label">ID:</span>{{ u.account_id }}
                  </span>
                  <button
                    class="btn-copy-id"
                    :title="copySuccessId === u.account_id ? '已复制！' : '复制 Account ID'"
                    @click="handleCopyAccountId(u.account_id, u.account_id, $event)"
                  >
                    <SvgIcon :name="copySuccessId === u.account_id ? 'check' : 'copy'" size="11" />
                  </button>
                  <span class="steam64-tag mono-font" :title="`SteamID64: ${u.steam64}`">
                    {{ u.steam64 }}
                  </span>
                </div>

                <!-- 备注区不再拦截整行点击：点卡片任意位置都能选中账号，编辑走独立按钮 -->
                <div class="account-note-line">
                  <div v-if="editingAccountId === u.account_id" class="note-edit-box" @click.stop>
                    <input
                      v-model="editingNoteText"
                      type="text"
                      class="note-edit-input"
                      placeholder="备注..."
                      autofocus
                      @keyup.enter="saveNote(u, $event)"
                      @keyup.esc="cancelEditNote($event)"
                    />
                    <button class="btn-note-ok" title="保存备注" @click="saveNote(u, $event)">
                      <SvgIcon name="check" size="11" />
                    </button>
                    <button class="btn-note-cancel" title="取消编辑" @click="cancelEditNote($event)">
                      <SvgIcon name="close" size="11" />
                    </button>
                  </div>
                  <div v-else class="note-view-box">
                    <span v-if="u.notes" class="note-content" :title="u.notes">
                      <SvgIcon name="file-text" size="11" class="note-icon" />
                      <span class="note-str">{{ u.notes }}</span>
                    </span>
                    <span v-else class="note-empty-action">未添加备注</span>
                    <button
                      class="btn-note-edit"
                      :title="u.notes ? '编辑备注' : '添加备注'"
                      :aria-label="u.notes ? '编辑备注' : '添加备注'"
                      @click.stop="startEditNote(u, $event)"
                    >
                      <SvgIcon name="edit" size="10" />
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

      </aside>

      <!-- Detail Panel (Right, flex: 1) -->
      <section class="workbench-detail">
        <!-- Top WinUI 3 CommandBar -->
        <div class="winui-command-bar">
          <button
            class="command-bar-btn"
            :disabled="!selectedUser"
            title="打开 CS2 画质精细调校与 4:3 比例控制"
            @click="openVideoModal"
          >
            <SvgIcon name="monitor" size="14" />
            <span>画质设置</span>
          </button>

          <button
            class="command-bar-btn"
            :disabled="!selectedUser"
            title="修改 CS2 命令行启动项参数"
            @click="openLaunchOptionsModal"
          >
            <SvgIcon name="terminal" size="14" />
            <span>修改启动项</span>
          </button>

          <button
            class="command-bar-btn"
            :disabled="!selectedUser"
            title="在软件内查看与修改当前账号的配置文件"
            @click="() => openTextEditor(undefined, 'user')"
          >
            <SvgIcon name="file-text" size="14" />
            <span>编辑配置文件</span>
          </button>

          <button
            class="command-bar-btn"
            :disabled="!selectedUser"
            title="查看历史备份快照时间线与差异比对"
            @click="openSnapshotsModal"
          >
            <SvgIcon name="clock" size="14" />
            <span>历史备份与对比</span>
            <span v-if="snapshots.length > 0" class="command-badge">{{ snapshots.length }}</span>
          </button>

        </div>

        <!-- 3 Clean Section Cards Stack -->
        <div class="workbench-cards-stack">
          <!-- Card 1: 当前账号配置 -->
          <AccountConfigCard
            v-if="selectedUser"
            :user="selectedUser"
            :is-backed-up="isSelectedUserBackedUp"
            :launch-options="currentLaunchOptions"
            :copy-success-id="copySuccessId"
            :copy-success-steam64="copySuccessSteam64"
            :copy-options-success="copyOptionsSuccess"
            :action-loading="actionLoading"
            :can-rollback="canRollbackUser"
            :summary="accountSummary"
            @switch-account="handleSwitchAccount(selectedUser)"
            @copy-account-id="(id: string) => handleCopyAccountId(id, id)"
            @copy-steam64="(id: string) => handleCopySteam64(id)"
            @copy-launch-options="handleCopyLaunchOptions"
            @toggle-readonly="handleToggleReadOnlyLock"
            @apply-default-preset="handleApplyDefaultVideoPresetQuick()"
            @backup="handleBackupSelectedUser"
            @restore="openRestoreSourceModal"
            @rollback="handleRollbackUser"
            @open-crosshair="openCrosshairModal"
          />

          <!-- Dedicated Shared Game CFG Workbench Card (When Shared Game CFG is selected) -->
          <!-- Empty account placeholder if no user is selected -->
          <div v-else class="winui-card detail-empty-placeholder">
            <div class="placeholder-icon-wrap">
              <SvgIcon name="user" size="32" />
            </div>
            <h4 class="placeholder-title">未选中 Steam 账号</h4>
            <p class="placeholder-desc">请在左侧列表选择一个 Steam 账号，或使用下方路径设置自动检测 Steam 安装目录。</p>
          </div>

          <!-- Card 2: 游戏通用 CFG -->
          <GameCfgCard
            :cs2-cfg-path="cs2CfgPath"
            :files="cfgFiles"
            :selected="cfgSelected"
            :quick-files="quickCfgFiles"
            :action-loading="actionLoading"
            :scanning="cfgScanning"
            :can-rollback="canRollbackGameCFG"
            @backup-all="handleBackupAllCFG"
            @backup-custom="handleBackupCustomCFG"
            @restore="handleRestoreCFG"
            @rollback="handleRollbackCFG"
            @open-dir="openPathInExplorer(cs2CfgPath)"
            @open-path-settings="openPathSettings"
            @edit-file="(n: string) => openTextEditor(n, 'game')"
            @toggle-file="toggleCfgFile"
            @select-all="selectAllCfgFiles(true)"
            @select-none="selectAllCfgFiles(false)"
            @select-player="selectOnlyCustomCfg"
            @refresh="loadCfgFiles"
            @backup-checked="handleBackupCheckedCFG"
          />

        </div>
      </section>
    </div>

    <!-- Diff Comparison Modal -->
    <DiffModal
      :visible="diffModalVisible"
      :account-id="selectedAccountId"
      :snapshot-id="diffModalSnapshotId"
      :diff-items="diffItems"
      @close="diffModalVisible = false"
      @open-editor="handleOpenEditorFromDiff"
    />

    <!-- Video Settings Modal (Mounted for Ticket 03/05 integration) -->
    <VideoSettingsModal
      :visible="videoModalVisible"
      :account-id="videoTargetAccountId"
      :username="videoTargetUsername"
      @close="closeVideoModal"
      @saved="handleVideoSettingsSaved"
    />

    <!-- 通用 CFG 还原：版本选择 -->
    <CfgVersionPickerModal
      :visible="cfgVersionPickerVisible"
      :versions="cfgVersions"
      @close="cfgVersionPickerVisible = false"
      @confirm="confirmRestoreCFG"
    />

    <!-- Crosshair Settings Modal -->
    <CrosshairModal
      :visible="crosshairModalVisible"
      :account-id="selectedAccountId"
      :username="selectedUser?.username || ''"
      @close="closeCrosshairModal"
      @saved="handleCrosshairSaved"
    />


    <!-- Snapshots & Diff Modal Dialog (Opened from CommandBar) -->
    <div v-if="snapshotsModalVisible" class="modal-overlay" @click.self="snapshotsModalVisible = false">
      <div class="snapshots-modal-container" role="dialog" aria-modal="true" aria-labelledby="snapshots-modal-title">
        <div class="snapshots-modal-header">
          <div class="snapshots-modal-title">
            <SvgIcon name="clock" size="16" class="text-accent" />
            <span id="snapshots-modal-title">历史备份与快照对比</span>
            <span class="snapshots-modal-account mono-font" v-if="selectedUser">({{ selectedUser.username }} - {{ selectedAccountId }})</span>
          </div>
          <button class="shield-close-btn" title="关闭" @click="snapshotsModalVisible = false">
            <SvgIcon name="close" size="14" />
          </button>
        </div>

        <div class="snapshots-modal-toolbar">
          <div class="create-snap-box">
            <input
              v-model="newSnapshotNote"
              type="text"
              class="winui-input snap-note-input"
              placeholder="备注（可选）"
              @keyup.enter="handleCreateSnapshot"
            />
            <button
              class="btn-winui-primary btn-sm"
              :disabled="actionLoading"
              @click="handleCreateSnapshot"
            >
              <SvgIcon name="plus" size="13" />
              <span>创建新快照</span>
            </button>
          </div>
        </div>

        <div class="snapshots-modal-body">
          <div v-if="snapshotsLoading" class="snapshot-loading-box">
            <div class="spinner-small"></div>
            <span>正在读取历史快照时间线...</span>
          </div>
          <div v-else-if="snapshots.length === 0" class="snapshot-empty-box">
            <SvgIcon name="clock" size="28" class="text-muted" />
            <h5>暂无历史快照节点</h5>
            <p>在上方输入备注并点击「创建新快照」，随时可保存历史配置版本。</p>
          </div>
          <div v-else class="snapshots-list">
            <div
              v-for="(snap, index) in snapshots"
              :key="snap.id"
              class="snapshot-item-card"
            >
              <div class="snap-info-col">
                <div class="snap-id-line">
                  <span class="snap-id-badge mono-font">{{ snap.id }}</span>
                  <span v-if="index === 0" class="snap-latest-badge" title="最近一次备份，局域网共享给对方的就是这一版">最近</span>
                  <span class="snap-time">{{ formatDateTime(snap.created_at) }}</span>
                  <span class="snap-meta">{{ snap.total_files }} 个文件 · {{ formatBytes(snap.total_bytes) }}</span>
                </div>
                <div class="snap-note-line">
                  <SvgIcon :name="snap.note ? 'file-text' : 'backup'" size="12" class="text-muted" />
                  <span>{{ snap.note || '自动备份（无备注）' }}</span>
                </div>
              </div>
              <div class="snap-actions-row">
                <button
                  class="btn-winui-ctrl btn-sm"
                  :disabled="actionLoading || diffLoading"
                  title="与本地实时配置比对差异"
                  @click="handleOpenDiff(snap)"
                >
                  <SvgIcon name="columns" size="12" />
                  <span>对比差异</span>
                </button>
                <button
                  class="btn-winui-ctrl btn-sm"
                  :disabled="actionLoading"
                  title="还原此快照版本"
                  @click="handleRestoreSnapshot(snap)"
                >
                  <SvgIcon name="restore" size="12" />
                  <span>还原</span>
                </button>
                <button
                  class="btn-winui-ctrl btn-sm btn-danger-ctrl"
                  :disabled="actionLoading"
                  title="删除此快照"
                  @click="handleDeleteSnapshot(snap)"
                >
                  <SvgIcon name="trash" size="12" />
                  <span>删除</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- In-App Config Text Editor Modal (Ticket 05) -->
    <TextEditorModal
      :visible="isTextEditorOpen"
      :account-id="selectedAccountId"
      :initial-file="textEditorInitialFile"
      :initial-scope="textEditorInitialScope"
      @close="closeTextEditorModal"
      @saved="handleTextEditorSaved"
    />

    <!-- Edit Launch Options Modal Dialog -->
    <div v-if="editLaunchOptionsVisible" class="modal-overlay" @click.self="editLaunchOptionsVisible = false">
      <div class="launch-modal-container" role="dialog" aria-modal="true" aria-labelledby="launch-modal-title">
        <div class="launch-modal-header">
          <div class="launch-modal-title">
            <SvgIcon name="terminal" size="16" class="text-accent" />
            <span id="launch-modal-title">修改 CS2 启动项 (AppID 730)</span>
          </div>
          <button class="shield-close-btn" title="关闭" aria-label="关闭启动项设置" @click="editLaunchOptionsVisible = false">
            <SvgIcon name="close" size="14" />
          </button>
        </div>

        <div class="launch-modal-body">
          <div class="launch-modal-meta">
            <span>目标账号: <strong>{{ selectedUser?.username }}</strong> <span class="mono-font">({{ selectedAccountId }})</span></span>
            <span class="launch-modal-sub">修改结果将以无分配流式 VDF 安全写入 localconfig.vdf</span>
          </div>

          <!-- Quick Preset Chips -->
          <div class="preset-chips-section">
            <span class="chips-label">常用预设快捷添加:</span>
            <div class="chips-grid">
              <button
                v-for="preset in launchOptionPresets"
                :key="preset.label"
                type="button"
                class="preset-chip"
                :class="{ active: isPresetActiveInModal(preset.label) }"
                :title="preset.desc"
                @click="togglePresetInModal(preset.label)"
              >
                <div class="chip-main-row">
                  <SvgIcon :name="isPresetActiveInModal(preset.label) ? 'check' : 'plus'" size="11" class="chip-status-icon" />
                  <span class="chip-label mono-font">{{ preset.label }}</span>
                </div>
                <span class="chip-desc">{{ preset.desc }}</span>
              </button>
            </div>
          </div>

          <!-- Launch Options Textarea -->
          <div class="launch-input-group">
            <label class="launch-input-label">启动项命令行参数 (自动转义引号与反斜杠):</label>
            <textarea
              v-model="editLaunchOptionsInput"
              class="launch-textarea mono-font"
              rows="4"
              placeholder="例如: -novid -freq 240 +exec autoexec.cfg -tickrate 128 +fps_max 0"
            ></textarea>
          </div>
        </div>

        <div class="launch-modal-footer">
          <button
            class="btn btn-primary"
            :disabled="actionLoading"
            @click="handleSaveLaunchOptions"
          >
            <SvgIcon name="check" size="14" />
            <span>保存启动项</span>
          </button>
          <button
            class="btn btn-secondary"
            @click="editLaunchOptionsVisible = false"
          >
            取消
          </button>
        </div>
      </div>
    </div>

    <!-- Launch Options Steam Exit Check Confirmation Modal -->
    <div
      v-if="launchOptionsSteamPromptVisible"
      class="modal-overlay"
      @click.self="confirmSaveLaunchOptions(false)"
    >
      <div class="steam-confirm-dialog">
        <div class="confirm-dialog-header">
          <div class="confirm-dialog-title">
            <SvgIcon name="shield" size="16" class="text-accent" />
            <span>Steam 正在运行</span>
          </div>
          <button class="shield-close-btn" title="关闭" @click="confirmSaveLaunchOptions(false)">
            <SvgIcon name="close" size="14" />
          </button>
        </div>
        <div class="confirm-dialog-body">
          <p class="confirm-text">
            Steam 正在运行中。修改启动项需要关闭 Steam 才能写入生效。是否立即退出 Steam 并保存？
          </p>
        </div>
        <div class="confirm-dialog-footer">
          <button
            class="btn btn-secondary"
            :disabled="actionLoading"
            @click="confirmSaveLaunchOptions(false)"
          >
            暂不修改
          </button>
          <button
            class="btn btn-primary"
            :disabled="actionLoading"
            @click="confirmSaveLaunchOptions(true)"
          >
            <SvgIcon v-if="!actionLoading" name="check" size="14" />
            <span v-if="actionLoading" class="spinner-small"></span>
            <span>{{ actionLoading ? '正在处理...' : '退出 Steam 并保存' }}</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Switch Account WinUI 3 Confirmation Modal -->
    <div
      v-if="switchModalVisible"
      class="modal-overlay"
      @click.self="closeSwitchModal"
    >
      <div class="switch-modal-container" role="dialog" aria-modal="true" aria-labelledby="switch-modal-title">
        <div class="switch-modal-header">
          <div class="switch-modal-title">
            <SvgIcon name="user" size="16" class="text-accent" />
            <span id="switch-modal-title">切换 Steam 账号</span>
          </div>
          <button class="shield-close-btn" title="关闭" @click="closeSwitchModal">
            <SvgIcon name="close" size="14" />
          </button>
        </div>
        <div class="switch-modal-body">
          <p class="switch-confirm-msg">
            确认切换 Steam 账号至 <strong>{{ switchTargetUser?.username || switchTargetUser?.account_name }}</strong> 吗？Steam 将自动退出并以该账号重新登录。
          </p>
          <div v-if="switchTargetUser" class="switch-account-detail-box">
            <div class="switch-detail-row">
              <span class="detail-label">玩家昵称:</span>
              <span class="detail-value">{{ switchTargetUser.username }}</span>
            </div>
            <div class="switch-detail-row">
              <span class="detail-label">SteamID:</span>
              <span class="detail-value mono-font">{{ switchTargetUser.account_id }} ({{ switchTargetUser.steam64 }})</span>
            </div>
            <div v-if="switchTargetUser.account_name" class="switch-detail-row">
              <span class="detail-label">登录名:</span>
              <span class="detail-value mono-font">{{ switchTargetUser.account_name }}</span>
            </div>
          </div>
        </div>
        <div class="switch-modal-footer">
          <button
            class="btn btn-primary"
            :disabled="switchActionLoading"
            @click="confirmSwitchAccount"
          >
            <SvgIcon v-if="!switchActionLoading" name="check" size="14" />
            <span v-if="switchActionLoading" class="spinner-small"></span>
            <span>{{ switchActionLoading ? '正在切换...' : '确认切换' }}</span>
          </button>
          <button
            class="btn btn-secondary"
            :disabled="switchActionLoading"
            @click="closeSwitchModal"
          >
            取消
          </button>
        </div>
      </div>
    </div>
    <!-- Restore Source Modal -->
    <RestoreSourceModal
      :visible="restoreSourceModalVisible"
      :target-account-id="selectedAccountId"
      :target-username="selectedUser?.username"
      :backed-up-accounts="backedUpAccounts"
      :snapshots="snapshots"
      :users="users"
      :remote-backups="remoteBackups"
      @confirm="handleConfirmRestoreSource"
      @close="restoreSourceModalVisible = false"
    />

    <!-- Path Settings Dialog -->
    <PathSettingsModal
      :visible="pathModalVisible"
      :steam-path="steamPathInput"
      :backup-path="backupPathInput"
      :cs2-path="cs2PathInput"
      @close="pathModalVisible = false"
      @saved="handlePathSaved"
    />

    <!-- Global WinUI 3 Confirm Modal -->
    <ConfirmModal
      :visible="confirmState.visible"
      :title="confirmState.title"
      :message="confirmState.message"
      :detail="confirmState.detail"
      :icon="confirmState.icon"
      :confirm-text="confirmState.confirmText"
      :cancel-text="confirmState.cancelText"
      :is-danger="confirmState.isDanger"
      :show-cancel="confirmState.showCancel"
      :prompt="confirmState.prompt"
      :prompt-placeholder="confirmState.promptPlaceholder"
      :prompt-default="confirmState.promptDefault"
      @confirm="onConfirmModalConfirm"
      @cancel="onConfirmModalCancel"
    />
  </div>
</template>

<style scoped>
.local-backup-view {
  display: flex;
  flex-direction: column;
  gap: 12px;
  height: 100%;
  width: 100%;
}

/* Global Floating Toast Container */
.toast-floating-container {
  position: fixed;
  bottom: 46px;
  right: 20px;
  z-index: 2000;
  display: flex;
  flex-direction: column;
  gap: 10px;
  max-width: 440px;
  pointer-events: none;
}

.toast-item {
  pointer-events: auto;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 10px 12px;
  border-radius: var(--radius-card);
  background-color: #2c2c2c;
  border: 1px solid rgba(255, 255, 255, 0.09);
  box-shadow: var(--shadow-flyout);
  color: #ffffff;
  font-size: 13px;
  box-sizing: border-box;
}

.toast-item.success-toast {
  border-color: var(--status-success-stroke);
}

.toast-item.success-toast .toast-icon-wrap {
  color: var(--level-success);
  background-color: var(--status-success-fill);
}

.toast-item.error-toast {
  border-color: var(--status-error-stroke);
}

.toast-item.error-toast .toast-icon-wrap {
  color: var(--level-error);
  background-color: var(--status-error-fill);
}

.toast-icon-wrap {
  width: 28px;
  height: 28px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.toast-body {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
  min-width: 0;
}

.toast-title {
  font-size: 13px;
  font-weight: 600;
  color: #ffffff;
}

.toast-msg {
  font-size: 12px;
  color: var(--color-text-secondary);
  line-height: 1.4;
  word-break: break-all;
}

.toast-close-btn {
  background: transparent;
  border: none;
  color: var(--color-text-muted);
  cursor: pointer;
  padding: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
  transition: all 0.15s ease;
}

.toast-close-btn:hover {
  color: #ffffff;
  background-color: var(--stroke-2);
}

.toast-slide-enter-active,
.toast-slide-leave-active {
  transition: all 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}

.toast-slide-enter-from {
  opacity: 0;
  transform: translateX(30px);
}

.toast-slide-leave-to {
  opacity: 0;
  transform: translateX(30px) scale(0.95);
}

/* Master-Detail Split Workbench Layout */
.workbench-layout {
  display: flex;
  gap: 18px;
  flex: 1;
  min-height: 0;
  height: 100%;
  width: 100%;
  overflow: hidden;
}

/* 矮窗口（如 1080x680 最小尺寸）：压缩留白，保证卡片首屏完整 */
@media (max-height: 760px) {
  .winui-card {
    padding: 12px 14px;
    gap: 10px;
  }

  /* 通用配置卡片带了更高特异性的内边距，矮窗口下需要同步压缩 */
  .winui-card.card-game-cfg {
    padding: 12px 14px;
    gap: 10px;
  }

  .workbench-cards-stack {
    gap: 10px;
    padding-bottom: 0;
  }

  .workbench-detail {
    gap: 10px;
  }

}

/* ============================================================
   Master Panel (Left, fixed ~380px)
   ============================================================ */
.workbench-master {
  width: 344px;
  min-width: 320px;
  max-width: 380px;
  flex-shrink: 0;
  height: 100%;
  display: flex;
  flex-direction: column;
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  padding: 14px;
  box-sizing: border-box;
  overflow: hidden;
  box-shadow: var(--shadow-card);
}

/* Master Section 1: 环境状态 */
.master-env-section {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding-bottom: 10px;
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
}

.steam-status-pill {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  height: var(--control-height);
  padding: 0 6px 0 10px;
  background-color: var(--bg-control);
  border: 1px solid var(--border-control);
  border-radius: var(--radius-control);
  transition: background-color var(--duration-fast) var(--ease-standard),
    border-color var(--duration-fast) var(--ease-standard);
}

.steam-status-pill.status-active {
  border-color: var(--status-success-stroke);
  background-color: var(--status-success-fill);
}

.status-indicator-group {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
}

.status-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background-color: var(--color-text-muted);
  flex-shrink: 0;
}

.status-active .status-dot {
  background-color: var(--level-success);
}

.status-name {
  font-size: 12px;
  font-weight: 500;
  color: var(--color-text-secondary);
  white-space: nowrap;
}

.status-active .status-name {
  color: var(--color-text);
}

.status-btn-group {
  display: flex;
  align-items: center;
  gap: 4px;
  flex-shrink: 0;
}

.btn-status-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 24px;
  padding: 0 7px;
  font-family: var(--font-sans);
  font-size: 12px;
  font-weight: 500;
  border-radius: var(--radius-sm);
  cursor: pointer;
  border: 1px solid transparent;
  transition: background-color var(--duration-fast) var(--ease-standard);
}

.btn-shutdown {
  background: var(--status-error-fill);
  color: var(--level-error);
  border-color: var(--status-error-stroke);
}

.btn-shutdown:hover:not(:disabled) {
  background: rgba(255, 153, 164, 0.2);
  color: #ffffff;
}

/* WinUI 中“启动/主操作”使用强调色，而非自定义绿色 */
.btn-start {
  background: var(--accent);
  color: #ffffff;
  border-color: var(--surface-4);
}

.btn-start:hover:not(:disabled) {
  background: var(--accent-hover);
}

.btn-status-refresh {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  background: transparent;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  color: var(--color-text-muted);
  cursor: pointer;
  transition: background-color var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard);
}

.btn-status-refresh:hover:not(:disabled) {
  color: var(--color-text);
  background: var(--bg-control-hover);
}

/* 路径概览：只读展示，点击直达资源管理器，编辑统一收敛到路径设置对话框 */
.env-summary {
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.env-line {
  display: flex;
  align-items: center;
  gap: 6px;
  width: 100%;
  height: 24px;
  padding: 0 6px;
  background: transparent;
  border: none;
  border-radius: var(--radius-sm);
  color: var(--color-text-muted);
  font-size: 12px;
  text-align: left;
  cursor: pointer;
  transition: background-color var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard);
}

.env-line:hover {
  background: var(--bg-control-hover);
  color: var(--color-text-secondary);
}

.env-line-icon {
  flex-shrink: 0;
}

.env-line-text {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.env-settings-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  height: 28px;
  margin-top: 2px;
  background: transparent;
  border: 1px solid var(--border-control);
  border-radius: var(--radius-control);
  color: var(--color-text-secondary);
  font-family: var(--font-sans);
  font-size: 12px;
  cursor: pointer;
  transition: background-color var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard);
}

.env-settings-btn:hover {
  background: var(--bg-control-hover);
  color: var(--color-text);
}

/* Master Section 2: 搜索与筛选 */
.master-search-bar {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 10px 0 8px;
  flex-shrink: 0;
}

.filter-segments {
  display: flex;
  align-items: center;
  gap: 4px;
}

.filter-seg {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 26px;
  padding: 0 10px;
  background: transparent;
  border: 1px solid var(--border-control);
  border-radius: var(--radius-pill);
  color: var(--color-text-secondary);
  font-family: var(--font-sans);
  font-size: 12px;
  cursor: pointer;
  transition: background-color var(--duration-fast) var(--ease-standard),
    border-color var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard);
}

.filter-seg:hover {
  background: var(--bg-control-hover);
  color: var(--color-text);
}

.filter-seg.active {
  background: var(--accent-subtle);
  border-color: rgba(0, 120, 212, 0.45);
  color: var(--accent-light);
}

.seg-count {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--color-text-muted);
}

.filter-seg.active .seg-count {
  color: var(--accent-light);
}

.filter-segments .btn-refresh-users {
  margin-left: auto;
}

.search-input-box {
  display: flex;
  align-items: center;
  flex: 1;
  height: var(--control-height);
  background-color: var(--bg-control);
  border: 1px solid var(--border-control);
  border-radius: var(--radius-md);
  padding: 0 8px;
  transition: border-color 0.15s ease, background-color 0.15s ease;
}

.search-input-box:focus-within {
  border-color: var(--accent);
  background-color: var(--bg-control-active);
}

.search-box-icon {
  color: var(--color-text-muted);
  margin-right: 6px;
  flex-shrink: 0;
}

.search-text-input {
  flex: 1;
  background: transparent;
  border: none;
  outline: none;
  font-size: 12px;
  font-family: var(--font-sans);
  color: var(--color-text);
  padding: 0;
}

.search-text-input::placeholder {
  color: var(--color-text-muted);
}

.btn-clear-search {
  background: transparent;
  border: none;
  color: var(--color-text-muted);
  cursor: pointer;
  padding: 2px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-xs);
  transition: color 0.15s ease;
}

.btn-clear-search:hover {
  color: var(--color-text);
}

.btn-refresh-users {
  height: 26px;
  background: var(--bg-control);
  border: 1px solid var(--border-control);
  color: var(--color-text-secondary);
  border-radius: var(--radius-sm);
  padding: 0 7px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s ease;
}

.btn-refresh-users:hover:not(:disabled) {
  color: #ffffff;
  background: var(--bg-control-hover);
  border-color: var(--border-control-hover);
}

/* Master Section 3: Master Accounts List (Scrollable) */
.master-accounts-scroll {
  flex: 1;
  overflow-y: auto;
  scroll-behavior: smooth;
  min-height: 0;
  padding-right: 2px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.master-accounts-scroll::-webkit-scrollbar,
.workbench-detail::-webkit-scrollbar {
  width: 5px;
}

.master-accounts-scroll::-webkit-scrollbar-track,
.workbench-detail::-webkit-scrollbar-track {
  background: transparent;
}

.master-accounts-scroll::-webkit-scrollbar-thumb,
.workbench-detail::-webkit-scrollbar-thumb {
  background: var(--stroke-3);
  border-radius: 4px;
}

.master-accounts-scroll::-webkit-scrollbar-thumb:hover,
.workbench-detail::-webkit-scrollbar-thumb:hover {
  background: rgba(255, 255, 255, 0.3);
}

.account-items-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.account-card {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 10px 11px;
  background-color: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: all 0.15s ease;
  position: relative;
}

.account-card:hover {
  background-color: var(--bg-card-hover);
  border-color: rgba(255, 255, 255, 0.14);
}

.account-card.selected-account {
  background-color: var(--bg-card-hover);
  border-color: var(--accent);
  box-shadow: 0 0 0 1px var(--accent);
}

.account-card.recent-account.selected-account {
  border-color: var(--accent);
  box-shadow: 0 0 0 1px var(--accent);
}

.account-card-left {
  flex-shrink: 0;
}

.avatar-container {
  position: relative;
  width: 38px;
  height: 38px;
  border-radius: var(--radius-md);
  overflow: visible;
}

.user-avatar-img {
  width: 100%;
  height: 100%;
  border-radius: var(--radius-md);
  object-fit: cover;
  background-color: var(--bg-control);
}


.avatar-active-dot {
  position: absolute;
  top: -2px;
  right: -2px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background-color: var(--level-success);
  border: 2px solid var(--bg-surface);
}

.account-card-body {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.account-header-line {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.account-nickname {
  flex: 1 1 auto;
  min-width: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.account-status-icons {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  flex-shrink: 0;
  color: var(--color-text-muted);
}

.status-icon {
  flex-shrink: 0;
}

.status-icon-ok {
  color: var(--level-success);
}

.badge-active-recent {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 19px;
  padding: 0 5px;
  font-size: 12px;
  font-weight: 500;
  line-height: 1;
  color: #b6e8af;
  background-color: var(--status-success-fill);
  border: 1px solid var(--status-success-stroke);
  border-radius: var(--radius-pill);
  white-space: nowrap;
  flex-shrink: 0;
}

.dot-pulse {
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background-color: var(--level-success);
}

.btn-switch-account {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 22px;
  padding: 0 7px;
  font-size: 12px;
  font-weight: 500;
  color: var(--accent-light);
  background-color: transparent;
  border: 1px solid var(--tone-info-stroke);
  border-radius: var(--radius-pill);
  white-space: nowrap;
  cursor: pointer;
  flex-shrink: 0;
  transition: background-color var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard);
}

.btn-switch-account:hover {
  background-color: var(--accent);
  color: #ffffff;
  border-color: var(--accent);
}

.account-ids-line {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

.id-tag {
  color: #e2e8f0;
}

.id-tag-label {
  color: var(--color-text-muted);
  margin-right: 2px;
}

.btn-copy-id {
  background: transparent;
  border: none;
  color: var(--color-text-muted);
  cursor: pointer;
  padding: 1px 3px;
  display: inline-flex;
  align-items: center;
  border-radius: 3px;
  transition: all 0.15s ease;
}

.btn-copy-id:hover {
  color: #ffffff;
  background-color: var(--stroke-2);
}

.steam64-tag {
  color: var(--color-text-muted);
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 130px;
}

.account-note-line {
  margin-top: 2px;
}

.note-view-box {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  font-size: 12px;
  color: var(--color-text-muted);
}

.note-content {
  flex: 1;
  min-width: 0;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--color-text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.btn-note-edit {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  flex-shrink: 0;
  padding: 0;
  background: transparent;
  border: 1px solid transparent;
  border-radius: var(--radius-sm);
  color: var(--color-text-muted);
  cursor: pointer;
  opacity: 0.45;
  transition: opacity var(--duration-fast) var(--ease-standard),
    background-color var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard);
}

.account-card:hover .btn-note-edit,
.selected-account .btn-note-edit {
  opacity: 1;
}

.btn-note-edit:hover {
  background: var(--bg-control-hover);
  border-color: var(--border-control);
  color: var(--color-text);
}

.note-icon {
  flex-shrink: 0;
  color: #3b82f6;
}

.note-str {
  overflow: hidden;
  text-overflow: ellipsis;
}

.note-empty-action {
  flex: 1;
  min-width: 0;
  font-size: 12px;
  color: var(--color-text-disabled);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.note-edit-box {
  display: flex;
  align-items: center;
  gap: 4px;
}

.note-edit-input {
  flex: 1;
  background-color: #0b0c10;
  border: 1px solid #3b82f6;
  border-radius: 4px;
  color: #ffffff;
  font-size: 12px;
  padding: 2px 6px;
  outline: none;
}

.btn-note-ok,
.btn-note-cancel {
  background: transparent;
  border: 1px solid var(--stroke-3);
  color: var(--color-text-secondary);
  border-radius: 4px;
  padding: 2px 5px;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
}

.btn-note-ok:hover {
  color: #10b981;
  border-color: #10b981;
}

.btn-note-cancel:hover {
  color: #ef4444;
  border-color: #ef4444;
}

/* Master State Placeholders */
.master-loading-card,
.master-empty-card,
.master-no-results-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 24px 16px;
  background-color: #151820;
  border: 1px dashed var(--stroke-2);
  border-radius: 8px;
  gap: 8px;
}

.empty-icon-circle {
  width: 44px;
  height: 44px;
  border-radius: 50%;
  background-color: rgba(255, 255, 255, 0.05);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--accent-cs2);
}

.empty-title {
  font-size: 13px;
  font-weight: 600;
  color: #ffffff;
  margin: 0;
}

.empty-desc {
  font-size: 12px;
  color: var(--color-text-secondary);
  line-height: 1.4;
  margin: 0;
}

.btn-auto-detect-empty {
  margin-top: 6px;
}

.no-res-icon {
  color: var(--color-text-muted);
}

.no-res-text {
  font-size: 12px;
  color: var(--color-text-secondary);
}

.btn-clear-link {
  background: transparent;
  border: none;
  color: #3b82f6;
  font-size: 12px;
  cursor: pointer;
  text-decoration: underline;
  padding: 0;
}

/* ============================================================
   Detail Panel (Right, flex: 1)
   ============================================================ */
.workbench-detail {
  flex: 1;
  min-width: 0;
  height: 100%;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding-right: 4px;
}

/* Detail Placeholder */
.detail-empty-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  min-height: 360px;
  background-color: #12141a;
  border: 1px dashed var(--stroke-2);
  border-radius: 10px;
  text-align: center;
  padding: 40px 20px;
  gap: 12px;
}

.placeholder-icon-wrap {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  background-color: rgba(255, 255, 255, 0.05);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #64748b;
}

.placeholder-title {
  font-size: 16px;
  font-weight: 600;
  color: #ffffff;
  margin: 0;
}

.placeholder-desc {
  font-size: 13px;
  color: var(--color-text-secondary);
  max-width: 420px;
  line-height: 1.5;
  margin: 0;
}

.chip-label {
  color: var(--color-text-muted);
  font-weight: 600;
}

@media (max-width: 980px) {
  .workbench-layout {
    flex-direction: column;
    height: auto;
    overflow: visible;
  }

  .workbench-master {
    width: 100%;
    max-width: 100%;
    min-width: 0;
    height: auto;
    max-height: 420px;
  }

  .master-header-panel {
    position: sticky;
    top: 0;
    z-index: 20;
    background-color: #12141c;
  }

  .workbench-detail {
    height: auto;
    overflow-y: visible;
  }
}

.snapshot-loading-box,
.snapshot-empty-box {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 20px;
  background-color: rgba(0, 0, 0, 0.2);
  border-radius: 6px;
  color: var(--color-text-secondary);
  font-size: 13px;
  flex: 1;
  justify-content: center;
  text-align: center;
}

.snapshot-item-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  background-color: #171a24;
  border: 1px solid var(--surface-3);
  border-radius: 6px;
  transition: all 0.15s ease;
}

.snapshot-item-card:hover {
  background-color: #1b1f2c;
  border-color: rgba(255, 255, 255, 0.12);
}

.snapshot-item-card.item-legacy {
  border-left: 3px solid #f59e0b;
}

.snap-latest-badge {
  font-size: 10px;
  font-weight: 600;
  letter-spacing: 0.3px;
  color: var(--accent);
  background: color-mix(in srgb, var(--accent) 14%, transparent);
  border: 1px solid color-mix(in srgb, var(--accent) 40%, transparent);
  border-radius: 999px;
  padding: 0 6px;
  line-height: 16px;
  flex-shrink: 0;
}

.snap-id-badge {
  font-size: 12px;
  font-weight: 700;
  color: #93c5fd;
  background-color: rgba(59, 130, 246, 0.15);
  border: 1px solid rgba(59, 130, 246, 0.3);
  padding: 1px 5px;
  border-radius: 4px;
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.snap-id-badge.legacy-badge {
  color: #fbbf24;
  background-color: rgba(245, 158, 11, 0.15);
  border-color: rgba(245, 158, 11, 0.35);
}

/* ============================================================
   Buttons & Common Components
   ============================================================ */
.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
  border: 1px solid transparent;
}

.btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.btn-primary {
  background: var(--accent);
  color: #ffffff;
}

.btn-primary:hover:not(:disabled) {
  opacity: 0.9;
  transform: translateY(-1px);
}

.btn-secondary {
  background: #1f2330;
  border-color: var(--stroke-2);
  color: var(--color-text-secondary);
}

.btn-secondary:hover:not(:disabled) {
  background: #282d3d;
  color: #ffffff;
  border-color: rgba(255, 255, 255, 0.2);
}

.btn-sm {
  padding: 4px 9px;
  font-size: 12px;
}

/* Utility Animations & Typography */
.mono-font {
  font-family: var(--font-mono);
}

.text-accent {
  color: var(--accent-cs2);
}

.spinning {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

.spinner-small {
  --spinner-border: rgba(255, 255, 255, 0.2);
  --spinner-top: var(--accent-cs2);
}

/* Modals & Dialogs */
.modal-overlay {
  position: fixed;
  inset: 0;
  padding: 24px;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
  backdrop-filter: blur(2px);
}

.launch-modal-container {
  width: 90%;
  max-width: 560px;
  background-color: #2c2c2c;
  border: 1px solid rgba(255, 255, 255, 0.09);
  border-radius: var(--radius-card);
  box-shadow: var(--shadow-dialog);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.launch-modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  background-color: rgba(0, 0, 0, 0.25);
  border-bottom: 1px solid var(--surface-4);
}

.launch-modal-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  font-weight: 700;
  color: #ffffff;
}

.shield-close-btn {
  background: none;
  border: none;
  color: var(--color-text-secondary);
  font-size: 16px;
  cursor: pointer;
  padding: 4px;
  display: flex;
  align-items: center;
}

.shield-close-btn:hover {
  color: #ffffff;
}

.launch-modal-body {
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.launch-modal-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  padding: 14px 18px;
  background-color: rgba(0, 0, 0, 0.2);
  border-top: 1px solid var(--surface-4);
}

/* Preset Chips */
.preset-chips-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.chips-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.chips-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.preset-chip {
  display: inline-flex;
  flex-direction: column;
  align-items: flex-start;
  padding: 4px 8px;
  background-color: rgba(255, 255, 255, 0.05);
  border: 1px solid var(--stroke-2);
  border-radius: 4px;
  cursor: pointer;
  transition: all 0.15s;
}

.preset-chip:hover {
  background-color: rgba(99, 102, 241, 0.2);
  border-color: rgba(99, 102, 241, 0.4);
}

.preset-chip.active {
  background: rgba(59, 130, 246, 0.2);
  border-color: #3b82f6;
}

.preset-chip.active .chip-label {
  color: #60a5fa;
  font-weight: 700;
}

.preset-chip .chip-main-row {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}

.preset-chip .chip-status-icon {
  color: var(--color-text-muted);
  flex-shrink: 0;
}

.preset-chip.active .chip-status-icon {
  color: #60a5fa;
}

.chip-label {
  font-family: var(--font-mono);
  font-size: 12px;
  font-weight: 600;
  color: #93c5fd;
}

.chip-desc {
  font-size: 12px;
  color: var(--color-text-secondary);
}

.launch-input-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.launch-input-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.launch-textarea {
  width: 100%;
  padding: 8px 10px;
  background-color: rgba(0, 0, 0, 0.35);
  border: 1px solid var(--stroke-3);
  border-radius: 6px;
  color: #ffffff;
  font-size: 12px;
  resize: vertical;
}

.launch-textarea:focus {
  outline: none;
  border-color: #6366f1;
}

.toast-title {
  font-size: 14px;
  font-weight: 700;
  color: #ffffff;
}

.fade-slide-enter-active,
.fade-slide-leave-active {
  transition: all 0.3s ease;
}

.fade-slide-enter-from,
.fade-slide-leave-to {
  opacity: 0;
  transform: translateY(16px);
}

/* Switch Account Modal */
.switch-modal-container {
  width: 90%;
  max-width: 440px;
  background-color: #2c2c2c;
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-dialog);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  animation: modalScaleIn 0.2s cubic-bezier(0.16, 1, 0.3, 1);
}

.switch-modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  background-color: var(--surface-1);
  border-bottom: 1px solid var(--border-color);
}

.switch-modal-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text);
}

.switch-modal-body {
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.switch-confirm-msg {
  font-size: 13px;
  line-height: 1.5;
  color: var(--color-text-secondary);
}

.switch-confirm-msg strong {
  color: #ffffff;
}

.switch-account-detail-box {
  display: flex;
  flex-direction: column;
  gap: 6px;
  padding: 10px 12px;
  background: var(--bg-control);
  border: 1px solid var(--border-control);
  border-radius: var(--radius-md);
}

.switch-detail-row {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
}

.switch-detail-row .detail-label {
  color: var(--color-text-muted);
  width: 70px;
  flex-shrink: 0;
}

.switch-detail-row .detail-value {
  color: var(--color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.switch-modal-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  padding: 12px 18px;
  background-color: rgba(0, 0, 0, 0.15);
  border-top: 1px solid var(--border-color);
}

/* ============================================================
   WinUI 3 Right Workbench Layout & Top CommandBar
   ============================================================ */
.workbench-detail {
  flex: 1;
  min-width: 0;
  height: 100%;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding-right: 4px;
}

.winui-command-bar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  padding: 8px 12px;
  background: var(--surface-2);
  border: 1px solid var(--stroke-1);
  border-radius: 8px;
  flex-shrink: 0;
}

.command-bar-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  height: 32px;
  padding: 0 12px;
  white-space: nowrap;
  flex-shrink: 0;
  font-size: 13px;
  font-weight: 500;
  color: #ffffff;
  background: var(--surface-3);
  border: 1px solid var(--surface-4);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.15s ease;
  user-select: none;
}

.command-bar-btn:hover:not(:disabled) {
  background: var(--stroke-2);
  border-color: rgba(255, 255, 255, 0.16);
}

.command-bar-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.command-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 20px;
  height: 19px;
  padding: 0 4px;
  border-radius: 10px;
  font-size: 12px;
  font-weight: 600;
  line-height: 1;
  background: rgba(0, 103, 192, 0.3);
  color: #60a5fa;
  border: 1px solid rgba(0, 103, 192, 0.4);
}

/* 3 Section Cards Stack */
.workbench-cards-stack {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding-bottom: 20px;
}

.winui-card {
  background: var(--surface-2);
  border: 1px solid var(--stroke-1);
  border-radius: 8px;
  padding: 16px 18px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  box-sizing: border-box;
  transition: border-color 0.15s ease;
}

.winui-card.card-highlighted {
  border-color: rgba(0, 103, 192, 0.5);
  box-shadow: 0 0 0 1px rgba(0, 103, 192, 0.3);
}

/* 卡片主次：账号卡片（主）压住通用配置卡片（次） */
.winui-card.card-account-config {
  background: rgba(255, 255, 255, 0.066);
  border-color: rgba(255, 255, 255, 0.105);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.22);
}

.winui-card.card-game-cfg {
  background: rgba(255, 255, 255, 0.018);
  border-color: rgba(255, 255, 255, 0.045);
  padding: 14px 16px;
  gap: 12px;
}

/* 空状态提示：路径未配置时解释按钮不可用 */
/* Card 1: Account Profile Row */
/* Launch Options Preview Strip */
/* 快速编辑行 */
/* 画质摘要条 */
/* Card 1: Core Action Buttons */
/* 出现第三个按钮（撤销还原）时独占一行，避免右侧留空 */
/* Card 2: Game CFG */
.winui-input {
  flex: 1;
  min-width: 0;
  height: 32px;
  padding: 0 10px;
  font-size: 12px;
  color: #ffffff;
  background: rgba(0, 0, 0, 0.3);
  border: 1px solid var(--stroke-2);
  border-radius: 6px;
  outline: none;
  transition: all 0.15s ease;
}

.winui-input:focus {
  border-color: #0078d4;
  box-shadow: 0 0 0 1px #0078d4;
  background: rgba(0, 0, 0, 0.4);
}

.btn-winui-ctrl {
  height: 32px;
  padding: 0 10px;
  font-size: 12px;
  font-weight: 500;
  color: #ffffff;
  background: var(--surface-3);
  border: 1px solid rgba(255, 255, 255, 0.09);
  border-radius: 6px;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  cursor: pointer;
  white-space: nowrap;
  flex-shrink: 0;
  transition: all 0.15s ease;
}

.btn-winui-ctrl:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.11);
  border-color: rgba(255, 255, 255, 0.16);
}

.btn-winui-ctrl:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.btn-danger-ctrl {
  color: #f87171;
}

.btn-danger-ctrl:hover:not(:disabled) {
  background: rgba(239, 68, 68, 0.15);
  border-color: rgba(239, 68, 68, 0.3);
}

/* Modals */
.snapshots-modal-container {
  width: 90%;
  max-width: 720px;
  background: #2c2c2c;
  border: 1px solid rgba(255, 255, 255, 0.09);
  border-radius: var(--radius-card);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: var(--shadow-dialog);
}

.snapshots-modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid var(--surface-4);
}

.snapshots-modal-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  font-weight: 600;
  color: #ffffff;
}

.snapshots-modal-account {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.5);
  font-weight: normal;
}

.snapshots-modal-toolbar {
  padding: 12px 18px;
  border-bottom: 1px solid var(--surface-3);
  background: var(--surface-1);
}

.create-snap-box {
  display: flex;
  align-items: center;
  gap: 8px;
}

.snap-note-input {
  flex: 1;
}

.btn-sm {
  height: 30px;
  padding: 0 10px;
  font-size: 12px;
}

.snapshots-modal-body {
  padding: 14px 18px;
  max-height: 400px;
  overflow-y: auto;
}

.snapshot-loading-box,
.snapshot-empty-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 30px 10px;
  gap: 8px;
  color: rgba(255, 255, 255, 0.6);
  font-size: 13px;
  text-align: center;
}

.snapshot-empty-box h5 {
  font-size: 14px;
  font-weight: 600;
  color: #ffffff;
  margin: 0;
}

.snapshot-empty-box p {
  font-size: 12px;
  color: rgba(255, 255, 255, 0.5);
  margin: 0;
}

.snapshots-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.snapshot-item-card {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 10px 12px;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid var(--surface-3);
  border-radius: 6px;
  transition: all 0.15s ease;
}

.snapshot-item-card:hover {
  background: var(--surface-3);
  border-color: var(--stroke-2);
}

.snap-info-col {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.snap-id-line {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 12px;
}

.snap-id-badge {
  font-weight: 600;
  color: #60a5fa;
  background: rgba(59, 130, 246, 0.15);
  padding: 1px 5px;
  border-radius: 4px;
  font-size: 12px;
}

.snap-time {
  color: rgba(255, 255, 255, 0.5);
}

.snap-meta {
  color: rgba(255, 255, 255, 0.4);
}

.snap-note-line {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.7);
}

.snap-actions-row {
  display: flex;
  align-items: center;
  gap: 6px;
}

/* Steam Confirm Dialog (WinUI 3 style) */
.steam-confirm-dialog {
  width: 440px;
  max-width: 90vw;
  background: var(--bg-surface, #262626);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg, 8px);
  box-shadow: var(--shadow-dialog, 0 16px 32px rgba(0, 0, 0, 0.36));
  overflow: hidden;
  animation: modal-fade-in 0.2s cubic-bezier(0, 0, 0, 1);
}

.confirm-dialog-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid var(--border-color);
  background: var(--surface-1);
}

.confirm-dialog-title {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text);
}

.confirm-dialog-body {
  padding: 20px 18px;
}

.confirm-text {
  font-size: 13px;
  line-height: 1.6;
  color: var(--color-text-secondary);
}

.confirm-dialog-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  padding: 14px 18px;
  border-top: 1px solid var(--border-color);
  background: var(--surface-1);
}
/* ============================================================
   Shared Game CFG 专注卡片（此前误置于样式表之外，从未生效）
   ============================================================ */
.btn-switch-account.btn-switch-quick {
  border-color: rgba(96, 205, 255, 0.45);
  color: var(--accent-light);
  font-weight: 600;
}

.btn-switch-account.btn-switch-quick:hover {
  background-color: var(--accent);
  border-color: var(--accent);
  color: #ffffff;
}
/* 矮窗口收尾覆盖：必须放在基础规则之后才能生效 */
</style>
