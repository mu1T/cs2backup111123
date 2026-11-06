<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import TopBar from './components/TopBar.vue'
import LogDrawer from './components/LogDrawer.vue'
import LocalBackupView from './views/LocalBackupView.vue'
import LanSharingView from './views/LanSharingView.vue'
import LanWebShare from './views/LanWebShare.vue'
import type { ActiveTab, LogEntry, AppInfo } from './types'
import {
  getAppInfo,
  getLogs,
  clearLogs,
  subscribeToLogs,
  isWailsAvailable,
  getLANServerStatus,
} from './utils/wails'

// 运行宿主：Wails 桌面端呈现完整工作台；局域网浏览器访问时呈现轻量共享中心（ADR-0004）
// 开发态（vite dev）默认仍预览桌面工作台，可用 ?mode=share 强制预览共享中心
const query = new URLSearchParams(window.location.search)
const isDesktop = query.get('mode') !== 'share' && (isWailsAvailable() || import.meta.env.DEV)

const activeTab = ref<ActiveTab>('local')
const logs = ref<LogEntry[]>([])
const lanRunning = ref(false)
const appInfo = ref<AppInfo>({
  name: 'CS2 Config Backup & Sync',
  version: '2.0.0',
  description: 'CS2 配置备份与同步',
  os: 'windows',
  arch: 'amd64',
})

let unsubscribeLogs: (() => void) | null = null
let lanPollTimer: number | null = null

const handleTabChange = (tab: ActiveTab) => {
  activeTab.value = tab
}

const addLog = (message: string, level: LogEntry['level'] = 'info') => {
  const newLog: LogEntry = {
    id: Date.now() + Math.random(),
    timestamp: new Date().toLocaleTimeString('zh-CN', { hour12: false }),
    level,
    message,
  }
  logs.value.push(newLog)
  if (logs.value.length > 500) {
    logs.value.shift()
  }
}

const handleClearLogs = async () => {
  await clearLogs()
  logs.value = []
}

const refreshLanStatus = async () => {
  try {
    const s = await getLANServerStatus()
    lanRunning.value = !!s.running
  } catch {
    lanRunning.value = false
  }
}

onMounted(async () => {
  if (!isDesktop) return

  try {
    appInfo.value = await getAppInfo()
    logs.value = await getLogs()
    addLog(`系统初始化完成: ${appInfo.value.name} v${appInfo.value.version}`)
  } catch (e: any) {
    addLog(`获取系统信息失败: ${e?.message || e}`, 'error')
  }

  // 订阅 Go 后端实时日志事件，避免运行日志只能看到前端本地动作
  unsubscribeLogs = subscribeToLogs((entry) => {
    if (!entry) return
    logs.value.push({ ...entry, id: entry.id || Date.now() + Math.random() })
    if (logs.value.length > 500) {
      logs.value.shift()
    }
  })

  await refreshLanStatus()
  lanPollTimer = window.setInterval(refreshLanStatus, 5000)
})

onUnmounted(() => {
  if (unsubscribeLogs) unsubscribeLogs()
  if (lanPollTimer) window.clearInterval(lanPollTimer)
})
</script>

<template>
  <!-- 局域网网页端：同一份静态资源，浏览器访问时呈现共享中心 -->
  <LanWebShare v-if="!isDesktop" />

  <div v-else class="app-layout">
    <TopBar
      :active-tab="activeTab"
      :version="appInfo.version"
      :lan-running="lanRunning"
      @change-tab="handleTabChange"
      @open-lan="handleTabChange('lan')"
    />

    <main class="main-content">
      <transition name="fade" mode="out-in">
        <LocalBackupView
          v-if="activeTab === 'local'"
          key="local"
          @log-action="(msg) => addLog(msg, 'info')"
        />
        <LanSharingView
          v-else-if="activeTab === 'lan'"
          key="lan"
          @log-action="(msg) => addLog(msg, 'info')"
        />
      </transition>
    </main>

    <LogDrawer :logs="logs" @clear="handleClearLogs" />
  </div>
</template>

<style scoped>
.app-layout {
  display: flex;
  flex-direction: column;
  width: 100vw;
  height: 100vh;
  background-color: var(--bg-app);
  overflow: hidden;
}

.main-content {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding: 12px 16px 0;
  background-color: var(--bg-app);
}

.fade-enter-active,
.fade-leave-active {
  transition: opacity var(--duration-fast) var(--ease-standard);
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
