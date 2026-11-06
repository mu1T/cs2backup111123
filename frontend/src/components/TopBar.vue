<script setup lang="ts">
import type { ActiveTab } from '../types'
import SvgIcon from './SvgIcon.vue'

withDefaults(
  defineProps<{
    activeTab: ActiveTab
    version?: string
    lanRunning?: boolean
  }>(),
  {
    version: '',
    lanRunning: false,
  }
)

const emit = defineEmits<{
  (e: 'change-tab', tab: ActiveTab): void
  (e: 'open-lan'): void
}>()
</script>

<template>
  <header class="topbar">
    <div class="brand">
      <img src="/app_icon.ico" alt="" class="brand-app-icon" aria-hidden="true" />
      <span v-if="version" class="version-badge mono-font">v{{ version }}</span>
    </div>

    <nav class="nav-tabs" role="tablist">
      <button
        class="nav-tab"
        :class="{ active: activeTab === 'local' }"
        role="tab"
        :aria-selected="activeTab === 'local'"
        @click="emit('change-tab', 'local')"
      >
        <SvgIcon name="archive" :size="16" />
        <span class="tab-label">本地备份</span>
      </button>

      <button
        class="nav-tab"
        :class="{ active: activeTab === 'lan' }"
        role="tab"
        :aria-selected="activeTab === 'lan'"
        @click="emit('change-tab', 'lan')"
      >
        <SvgIcon name="wifi" :size="16" />
        <span class="tab-label">局域网共享</span>
      </button>
    </nav>

    <div class="status-section">
      <button
        class="lan-status-pill"
        :class="{ online: lanRunning }"
        :title="lanRunning ? '本机局域网共享服务正在运行，点击查看' : '本机局域网共享服务未启动，点击开启'"
        @click="emit('open-lan')"
      >
        <span class="status-dot" />
        <span class="status-text">{{ lanRunning ? '共享服务运行中' : '共享服务未启动' }}</span>
      </button>
    </div>
  </header>
</template>

<style scoped>
.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  height: 48px;
  padding: 0 16px;
  background-color: var(--bg-app);
  border-bottom: 1px solid var(--border-color);
  flex-shrink: 0;
  z-index: 10;
}

.brand {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}

.brand-app-icon {
  width: 24px;
  height: 24px;
  border-radius: var(--radius-sm);
  object-fit: contain;
  flex-shrink: 0;
}

.version-badge {
  font-size: 12px;
  color: var(--color-text-muted);
  background: var(--bg-control);
  padding: 1px 6px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border-control);
  flex-shrink: 0;
}

.nav-tabs {
  display: flex;
  align-items: stretch;
  gap: 4px;
  height: 100%;
}

.nav-tab {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  height: 100%;
  padding: 0 14px;
  background: transparent;
  border: none;
  border-radius: 0;
  color: var(--color-text-secondary);
  font-family: var(--font-sans);
  font-size: 13px;
  font-weight: 400;
  cursor: pointer;
  transition: color var(--duration-fast) var(--ease-standard),
    background-color var(--duration-fast) var(--ease-standard);
}

.nav-tab:hover {
  color: var(--color-text);
  background: var(--bg-card-hover);
}

.nav-tab.active {
  color: var(--color-text);
  font-weight: 600;
}

.nav-tab.active::after {
  content: '';
  position: absolute;
  left: 14px;
  right: 14px;
  bottom: 0;
  height: 3px;
  border-radius: 2px 2px 0 0;
  background-color: var(--accent);
}

.status-section {
  display: flex;
  align-items: center;
  flex-shrink: 0;
}

.lan-status-pill {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 28px;
  padding: 0 12px;
  background: var(--bg-control);
  border: 1px solid var(--border-control);
  border-radius: var(--radius-pill);
  color: var(--color-text-secondary);
  font-family: var(--font-sans);
  font-size: 12px;
  cursor: pointer;
  transition: background-color var(--duration-fast) var(--ease-standard),
    border-color var(--duration-fast) var(--ease-standard);
}

.lan-status-pill:hover {
  background: var(--bg-control-hover);
  border-color: var(--border-control-hover);
  color: var(--color-text);
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background-color: var(--color-text-muted);
  flex-shrink: 0;
}

.lan-status-pill.online {
  border-color: var(--status-success-stroke);
  background: var(--status-success-fill);
  color: #b6e8af;
}

.lan-status-pill.online .status-dot {
  background-color: var(--level-success);
}
</style>
