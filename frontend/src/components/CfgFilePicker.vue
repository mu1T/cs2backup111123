<script setup lang="ts">
import { ref, computed } from 'vue'
import { CFG_SOURCES } from '../types'
import type { GameCFGFile, CfgSource } from '../types'
import { formatSize, formatDate } from '../utils/format'
import SvgIcon from './SvgIcon.vue'

const props = withDefaults(
  defineProps<{
    files: GameCFGFile[]
    selected: string[]
    disabled?: boolean
    loading?: boolean
    scanning?: boolean
    pathMissing?: boolean
    defaultOpen?: boolean
  }>(),
  {
    disabled: false,
    loading: false,
    scanning: false,
    pathMissing: false,
    defaultOpen: false,
  }
)

const emit = defineEmits<{
  (e: 'toggle', name: string): void
  (e: 'select-all'): void
  (e: 'select-none'): void
  (e: 'select-player'): void
  (e: 'backup'): void
  (e: 'refresh'): void
}>()

const isOpen = ref(props.defaultOpen)

const SOURCE_META: Record<CfgSource, { label: string; hint: string }> = {
  player: { label: '自定义', hint: '玩家自己编写或导入的脚本' },
  pwa: { label: '完美平台', hint: '完美世界竞技平台（PWA）生成，非玩家自定义' },
  fivee: { label: '5E 平台', hint: '5E 对战平台（5EPlay）注入，f5e / 5e 前缀' },
  platform: { label: '第三方平台', hint: 'FACEIT 等其他对战平台生成（当前无已知前缀）' },
  valve: { label: '官方', hint: 'Valve 随游戏下发的默认配置' },
}

const selectedSet = computed(() => new Set(props.selected))
const playerCount = computed(() => props.files.filter((f) => f.source === 'player').length)

const sourceCounts = computed(() => {
  const counts = Object.fromEntries(CFG_SOURCES.map((s) => [s, 0])) as Record<CfgSource, number>
  props.files.forEach((f) => {
    counts[f.source] = (counts[f.source] || 0) + 1
  })
  return counts
})
</script>

<template>
  <div class="cfg-picker" :class="{ open: isOpen }">
    <button
      type="button"
      class="picker-toggle"
      :aria-expanded="isOpen"
      aria-controls="cfg-picker-body"
      @click="isOpen = !isOpen"
    >
      <SvgIcon :name="isOpen ? 'chevron-down' : 'chevron-right'" :size="13" class="toggle-icon" />
      <span class="toggle-label">按文件备份</span>
      <span class="toggle-meta">
        <span v-if="scanning" class="scanning-hint">正在扫描目录...</span>
        <template v-else>
          已选 <strong>{{ selected.length }}</strong> / {{ files.length }} 个文件
        </template>
      </span>
    </button>

    <div v-show="isOpen" id="cfg-picker-body" class="picker-body">
      <div class="picker-toolbar">
        <button type="button" class="tool-btn" :disabled="disabled || scanning" @click="emit('select-all')">
          全选
        </button>
        <button type="button" class="tool-btn" :disabled="disabled || scanning" @click="emit('select-none')">
          清空
        </button>
        <button
          type="button"
          class="tool-btn"
          :disabled="disabled || scanning"
          :title="`仅勾选玩家自定义脚本（当前 ${playerCount} 个）`"
          @click="emit('select-player')"
        >
          仅自定义 ({{ playerCount }})
        </button>
        <button
          type="button"
          class="tool-btn tool-icon"
          :disabled="scanning"
          title="重新扫描 game/csgo/cfg"
          aria-label="重新扫描 CFG 目录"
          @click="emit('refresh')"
        >
          <SvgIcon name="refresh" :size="12" :class="{ spinning: scanning }" />
        </button>
        <button
          type="button"
          class="btn btn-primary btn-sm picker-submit"
          :disabled="disabled || scanning || selected.length === 0"
          :title="selected.length === 0 ? '请先勾选需要备份的配置文件' : `备份已勾选的 ${selected.length} 个文件`"
          @click="emit('backup')"
        >
          <span v-if="loading" class="spinner-small"></span>
          <SvgIcon v-else name="archive" :size="13" />
          <span>{{ loading ? '正在备份...' : '备份选中' }}</span>
        </button>
      </div>

      <div class="source-summary">
        <span
          v-for="s in CFG_SOURCES"
          :key="s"
          class="source-chip"
          :class="`chip-${s}`"
          :title="SOURCE_META[s].hint"
        >
          {{ SOURCE_META[s].label }}
          <strong>{{ sourceCounts[s] || 0 }}</strong>
        </span>
      </div>

      <ul v-if="files.length > 0" class="picker-list">
        <li v-for="f in files" :key="f.name" class="picker-row" :class="{ checked: selectedSet.has(f.name) }">
          <label class="row-main">
            <input
              type="checkbox"
              class="row-check"
              :checked="selectedSet.has(f.name)"
              :disabled="disabled"
              :aria-label="`选择 ${f.name}`"
              @change="emit('toggle', f.name)"
            />
            <span class="row-name mono-font" :title="f.name">{{ f.name }}</span>
          </label>
          <span class="row-source" :class="`src-${f.source}`">{{ SOURCE_META[f.source].label }}</span>
          <span v-if="f.read_only" class="row-lock" title="文件带只读属性">
            <SvgIcon name="lock" :size="11" />
          </span>
          <span class="row-meta mono-font">{{ formatSize(f.size) }}</span>
          <span class="row-meta row-date">{{ formatDate(f.modified) }}</span>
        </li>
      </ul>

      <div v-else class="picker-empty">
        <SvgIcon :name="pathMissing ? 'alert-triangle' : 'folder'" :size="16" />
        <span v-if="pathMissing">未检测到 CS2 安装目录，请先在路径设置中配置</span>
        <span v-else>未扫描到配置文件，请检查 CS2 安装路径是否正确</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.cfg-picker {
  border: 1px solid var(--border-color);
  border-radius: var(--radius-control);
  background: rgba(0, 0, 0, 0.14);
  overflow: hidden;
}

.picker-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  height: 34px;
  padding: 0 12px;
  background: transparent;
  border: none;
  color: var(--color-text-secondary);
  font-family: var(--font-sans);
  font-size: 12px;
  cursor: pointer;
  text-align: left;
  transition: background-color var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard);
}

.picker-toggle:hover {
  background: var(--bg-control-hover);
  color: var(--color-text);
}

.toggle-icon {
  flex-shrink: 0;
}

.toggle-label {
  font-weight: 600;
}

.toggle-meta {
  margin-left: auto;
  color: var(--color-text-muted);
  font-size: 12px;
}

.toggle-meta strong {
  color: var(--accent-light);
  font-family: var(--font-mono);
  font-weight: 600;
}

.scanning-hint {
  color: var(--accent-light);
}

.picker-body {
  border-top: 1px solid var(--border-color);
  padding: 8px 10px 10px;
}

.picker-toolbar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding-bottom: 8px;
}

.tool-btn {
  height: 24px;
  padding: 0 8px;
  background: var(--bg-control);
  border: 1px solid var(--border-control);
  border-radius: var(--radius-sm);
  color: var(--color-text-secondary);
  font-family: var(--font-sans);
  font-size: 12px;
  cursor: pointer;
  transition: background-color var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard);
}

.tool-btn:hover:not(:disabled) {
  background: var(--bg-control-hover);
  color: var(--color-text);
}

.tool-btn:disabled {
  opacity: 0.4;
  cursor: default;
}

.tool-icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  padding: 0;
}

.picker-submit {
  margin-left: auto;
}

.source-summary {
  display: flex;
  align-items: center;
  gap: 6px;
  padding-bottom: 8px;
  flex-wrap: wrap;
}

.source-chip {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 18px;
  padding: 0 7px;
  border-radius: var(--radius-pill);
  font-size: 12px;
  color: var(--color-text-muted);
  background: var(--bg-control);
  border: 1px solid var(--border-control);
  white-space: nowrap;
}

.source-chip strong {
  font-family: var(--font-mono);
  font-weight: 600;
  color: var(--color-text-secondary);
}

.chip-player {
  color: var(--accent-light);
  border-color: rgba(96, 205, 255, 0.3);
  background: var(--accent-subtle);
}

.chip-player strong {
  color: var(--accent-light);
}

.chip-pwa {
  color: #f0b8c0;
  border-color: rgba(255, 153, 164, 0.28);
  background: rgba(255, 153, 164, 0.08);
}

.chip-pwa strong {
  color: #f0b8c0;
}

.chip-fivee {
  color: #f2cf8c;
  border-color: rgba(252, 225, 0, 0.28);
  background: rgba(252, 225, 0, 0.08);
}

.chip-fivee strong {
  color: #f2cf8c;
}

.picker-list {
  list-style: none;
  margin: 0;
  padding: 0;
  max-height: 216px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.picker-row {
  display: flex;
  align-items: center;
  gap: 8px;
  min-height: 28px;
  padding: 0 6px;
  border-radius: var(--radius-sm);
  transition: background-color var(--duration-fast) var(--ease-standard);
}

.picker-row:hover {
  background: var(--bg-control-hover);
}

.picker-row.checked {
  background: var(--accent-subtle);
}

.row-main {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
  cursor: pointer;
}

.row-check {
  width: 14px;
  height: 14px;
  accent-color: var(--accent);
  cursor: pointer;
  flex-shrink: 0;
}

.row-name {
  font-size: 12px;
  color: var(--color-text);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.row-source {
  flex-shrink: 0;
  font-size: 12px;
  line-height: 16px;
  padding: 0 6px;
  border-radius: var(--radius-pill);
  border: 1px solid var(--border-control);
  color: var(--color-text-muted);
  white-space: nowrap;
}

.src-player {
  color: var(--accent-light);
  border-color: var(--tone-info-stroke);
  background: var(--accent-subtle);
}

.src-pwa {
  color: #f0b8c0;
  border-color: rgba(255, 153, 164, 0.28);
  background: rgba(255, 153, 164, 0.08);
}

.src-fivee {
  color: #f2cf8c;
  border-color: rgba(252, 225, 0, 0.28);
  background: rgba(252, 225, 0, 0.08);
}

.row-lock {
  display: inline-flex;
  color: var(--color-text-muted);
  flex-shrink: 0;
}

.row-meta {
  flex-shrink: 0;
  width: 54px;
  text-align: right;
  font-size: 12px;
  color: var(--color-text-muted);
}

.row-date {
  width: 62px;
}

.picker-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 18px 10px;
  color: var(--color-text-muted);
  font-size: 12px;
}

</style>
