<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import type { LogEntry } from '../types'
import SvgIcon from './SvgIcon.vue'

const props = defineProps<{
  logs: LogEntry[]
}>()

const emit = defineEmits<{
  (e: 'clear'): void
}>()

const EXPAND_KEY = 'cs2_log_drawer_expanded'

// 默认收起：日志抽屉固定占位会挤压主工作台，首屏应优先保证卡片完整可见
const isExpanded = ref(localStorage.getItem(EXPAND_KEY) === '1')
const logContainerRef = ref<HTMLElement | null>(null)

const toggleExpand = () => {
  isExpanded.value = !isExpanded.value
  localStorage.setItem(EXPAND_KEY, isExpanded.value ? '1' : '0')
  if (isExpanded.value) {
    scrollToBottom()
  }
}

const scrollToBottom = () => {
  nextTick(() => {
    if (logContainerRef.value) {
      logContainerRef.value.scrollTop = logContainerRef.value.scrollHeight
    }
  })
}

const isCopied = ref(false)

const handleCopyAllLogs = async () => {
  if (props.logs.length === 0) return
  const fullText = props.logs
    .map((e) => `[${e.timestamp}] [${e.level.toUpperCase()}] ${e.message}`)
    .join('\n')
  try {
    await navigator.clipboard.writeText(fullText)
    isCopied.value = true
    setTimeout(() => {
      isCopied.value = false
    }, 1800)
  } catch (err) {
    console.warn('Failed to copy logs to clipboard:', err)
  }
}

watch(
  () => props.logs.length,
  () => {
    if (isExpanded.value) {
      scrollToBottom()
    }
  }
)
</script>

<template>
  <aside class="log-drawer" :class="{ collapsed: !isExpanded }">
    <!-- Drawer Header -->
    <div class="drawer-header" @click.self="toggleExpand">
      <div class="header-left" @click="toggleExpand">
        <SvgIcon name="terminal" :size="14" class="drawer-icon" />
        <h3 class="drawer-title">运行日志</h3>
        <span class="log-count-pill">{{ logs.length }} 条</span>
      </div>

      <div class="header-actions">
        <button
          class="action-btn copy-btn"
          :title="isCopied ? '已复制！' : '复制全量运行日志'"
          :disabled="logs.length === 0"
          @click.stop="handleCopyAllLogs"
        >
          <SvgIcon :name="isCopied ? 'check' : 'copy'" :size="13" />
          <span>{{ isCopied ? '已复制' : '复制' }}</span>
        </button>

        <button
          class="action-btn clear-btn"
          title="清空运行日志"
          @click.stop="emit('clear')"
        >
          <SvgIcon name="trash" :size="13" />
          <span>清空</span>
        </button>

        <button
          class="action-btn toggle-btn"
          :title="isExpanded ? '收起日志' : '展开日志'"
          @click.stop="toggleExpand"
        >
          <span class="toggle-text">{{ isExpanded ? '收起' : '展开' }}</span>
          <SvgIcon :name="isExpanded ? 'chevron-down' : 'chevron-up'" :size="12" />
        </button>
      </div>
    </div>

    <!-- Drawer Body -->
    <div v-show="isExpanded" ref="logContainerRef" class="drawer-body">
      <div v-if="logs.length === 0" class="empty-logs">
        <span>暂无日志输出</span>
      </div>

      <div v-else class="log-list">
        <div
          v-for="entry in logs"
          :key="entry.id"
          class="log-row"
          :class="`level-${entry.level}`"
        >
          <span class="log-time font-mono">[{{ entry.timestamp }}]</span>
          <span class="log-level" :class="`badge-${entry.level}`">
            {{ entry.level.toUpperCase() }}
          </span>
          <span class="log-message">{{ entry.message }}</span>
        </div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.log-drawer {
  display: flex;
  flex-direction: column;
  background-color: var(--bg-app-alt);
  border-top: 1px solid var(--border-color);
  transition: height var(--duration-normal) var(--ease-standard);
  height: 176px;
  flex-shrink: 0;
  z-index: 20;
}

.log-drawer.collapsed {
  height: 34px;
}

.drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 34px;
  padding: 0 16px;
  background-color: transparent;
  border-bottom: 1px solid var(--border-color);
  user-select: none;
  cursor: pointer;
  flex-shrink: 0;
}

.log-drawer.collapsed .drawer-header {
  border-bottom-color: transparent;
}

.drawer-header:hover {
  background-color: var(--bg-card-hover);
}

.header-left {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
}

.drawer-icon {
  color: var(--accent);
}

.drawer-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
  letter-spacing: 0.3px;
}

.log-count-pill {
  font-size: 12px;
  color: var(--color-text-muted);
  background-color: var(--bg-control);
  border: 1px solid var(--border-control);
  padding: 1px 7px;
  border-radius: var(--radius-sm);
  font-family: var(--font-mono);
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 6px;
}

.action-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 26px;
  padding: 0 8px;
  background-color: var(--bg-control);
  border: 1px solid var(--border-control);
  border-radius: var(--radius-sm);
  color: var(--color-text-secondary);
  font-size: 12px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.action-btn:hover {
  background-color: var(--bg-control-hover);
  color: var(--color-text);
  border-color: var(--border-control-hover);
}

.clear-btn:hover {
  color: var(--level-error);
  border-color: rgba(255, 153, 164, 0.4);
  background-color: rgba(255, 153, 164, 0.08);
}

.toggle-btn {
  font-weight: 500;
  color: var(--accent);
}

.toggle-btn:hover {
  border-color: rgba(0, 120, 212, 0.4);
  background-color: var(--accent-subtle);
}

.drawer-body {
  flex: 1;
  overflow-y: auto;
  padding: 8px 16px;
  background-color: var(--bg-app-alt);
  font-family: var(--font-mono);
  font-size: 12px;
  user-select: text !important;
  -webkit-user-select: text !important;
}

.log-row {
  user-select: text !important;
  -webkit-user-select: text !important;
}

.log-message {
  user-select: text !important;
  -webkit-user-select: text !important;
}

.empty-logs {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--color-text-muted);
  font-size: 12px;
}

.log-list {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.log-row {
  display: flex;
  align-items: baseline;
  gap: 8px;
  line-height: 1.6;
  word-break: break-all;
}

.log-time {
  color: var(--color-text-muted);
  flex-shrink: 0;
  font-size: 12px;
}

.log-level {
  display: inline-block;
  padding: 0 4px;
  border-radius: var(--radius-xs);
  font-size: 12px;
  font-weight: 700;
  letter-spacing: 0.5px;
  flex-shrink: 0;
  text-align: center;
  min-width: 52px;
}

.badge-info {
  background-color: var(--tone-info-weak);
  color: var(--level-info);
  border: 1px solid rgba(96, 205, 255, 0.25);
}

.badge-success {
  background-color: rgba(108, 203, 95, 0.12);
  color: var(--level-success);
  border: 1px solid rgba(108, 203, 95, 0.25);
}

.badge-warning {
  background-color: rgba(252, 225, 0, 0.12);
  color: var(--level-warning);
  border: 1px solid rgba(252, 225, 0, 0.25);
}

.badge-error {
  background-color: rgba(255, 153, 164, 0.12);
  color: var(--level-error);
  border: 1px solid rgba(255, 153, 164, 0.25);
}

.log-message {
  color: var(--color-text);
  flex: 1;
}

.level-error .log-message {
  color: #ff99a4;
}

.level-warning .log-message {
  color: #fce100;
}

.level-success .log-message {
  color: #b6e8af;
}
</style>
