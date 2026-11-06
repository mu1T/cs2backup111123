<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import type { FileDiffItem, DiffResult } from '../types'
import SvgIcon from './SvgIcon.vue'
import { useFocusTrap } from '../utils/a11y'

const props = defineProps<{
  visible: boolean
  accountId: string
  snapshotId: string
  diffItems: FileDiffItem[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'open-editor', payload: { accountId: string; file: string }): void
}>()

const dialogRef = ref<HTMLElement | null>(null)

useFocusTrap(
  computed(() => props.visible),
  dialogRef
)

const selectedFileIndex = ref(0)
const viewMode = ref<'side-by-side' | 'unified'>('side-by-side')

// Watch for items change and reset selection
watch(
  () => props.diffItems,
  (newItems) => {
    if (newItems && newItems.length > 0) {
      // Prefer first modified or added/deleted file
      const changedIdx = newItems.findIndex((it) => it.status !== 'identical')
      selectedFileIndex.value = changedIdx >= 0 ? changedIdx : 0
    } else {
      selectedFileIndex.value = 0
    }
  },
  { immediate: true }
)

const currentFile = computed<FileDiffItem | null>(() => {
  if (!props.diffItems || props.diffItems.length === 0) return null
  return props.diffItems[selectedFileIndex.value] || props.diffItems[0]
})

const currentDiff = computed<DiffResult | null>(() => {
  return currentFile.value?.diff || null
})

const totalChanges = computed(() => {
  if (!currentDiff.value) return { add: 0, del: 0, mod: 0 }
  return {
    add: currentDiff.value.add_count,
    del: currentDiff.value.del_count,
    mod: currentDiff.value.mod_count,
  }
})

const handleOpenEditor = () => {
  if (!currentFile.value) return
  emit('open-editor', {
    accountId: props.accountId,
    file: currentFile.value.rel_path,
  })
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

const getStatusBadge = (status: string) => {
  switch (status) {
    case 'modified':
      return { text: '已修改', class: 'badge-mod' }
    case 'added':
      return { text: '新增文件', class: 'badge-add' }
    case 'deleted':
      return { text: '已删除', class: 'badge-del' }
    case 'identical':
      return { text: '无变动', class: 'badge-same' }
    default:
      return { text: status, class: 'badge-same' }
  }
}
</script>

<template>
  <div v-if="visible" class="diff-modal-backdrop" @click.self="close">
    <div ref="dialogRef" class="diff-modal-window" role="dialog" aria-modal="true" aria-labelledby="diff-modal-title" tabindex="-1">
      <!-- Modal Header -->
      <div class="diff-modal-header">
        <div class="header-left">
          <div class="modal-tag">配置文件对比</div>
          <h3 id="diff-modal-title" class="modal-title">
            配置对比:
            <span class="highlight-id">{{ snapshotId }}</span>
            <span class="text-muted font-normal"> (备份)</span>
            <span class="compare-arrow"> ⇄ </span>
            <span class="highlight-live">本地配置</span>
            <span class="account-tag">账号: {{ accountId }}</span>
          </h3>
        </div>

        <div class="header-right">
          <!-- View Mode Toggle -->
          <div class="view-toggle">
            <button
              class="toggle-btn"
              :class="{ active: viewMode === 'side-by-side' }"
              title="双栏对比模式"
              @click="viewMode = 'side-by-side'"
            >
              <SvgIcon name="columns" :size="13" /> 双栏对比
            </button>
            <button
              class="toggle-btn"
              :class="{ active: viewMode === 'unified' }"
              title="单栏对比模式"
              @click="viewMode = 'unified'"
            >
              <SvgIcon name="list" :size="13" /> 单栏对比
            </button>
          </div>

          <button class="btn-close" title="关闭 (Esc)" @click="close">
            <SvgIcon name="close" :size="16" />
          </button>
        </div>
      </div>

      <!-- File Navigation & Stats Toolbar -->
      <div class="diff-toolbar">
        <div class="file-selector-group">
          <label class="file-label">对比文件:</label>
          <select v-model="selectedFileIndex" class="file-select font-mono">
            <option
              v-for="(item, idx) in diffItems"
              :key="item.rel_path"
              :value="idx"
            >
              {{ item.rel_path }} ({{ getStatusBadge(item.status).text }})
            </option>
          </select>

          <span
            v-if="currentFile"
            class="status-pill"
            :class="getStatusBadge(currentFile.status).class"
          >
            {{ getStatusBadge(currentFile.status).text }}
          </span>

          <!-- WinUI 3 styled [在软件内编辑] Button in Toolbar -->
          <button
            v-if="currentFile"
            class="btn-edit-inline"
            title="在软件内置文本编辑器中查看并编辑此文件"
            @click="handleOpenEditor"
          >
            <SvgIcon name="edit" :size="13" />
            <span>在软件内编辑</span>
          </button>
        </div>

        <!-- Summary Counters -->
        <div class="stats-counter-group">
          <span class="stat-pill pill-add" title="新增行数">
            +{{ totalChanges.add }} 增
          </span>
          <span class="stat-pill pill-del" title="删除行数">
            -{{ totalChanges.del }} 删
          </span>
          <span class="stat-pill pill-mod" title="修改行数">
            ~{{ totalChanges.mod }} 改
          </span>
        </div>
      </div>

      <!-- Diff Content Area -->
      <div class="diff-body">
        <!-- Empty / No file -->
        <div v-if="!currentFile" class="diff-empty">
          <p>暂无文件对比数据</p>
        </div>

        <!-- Identical / No Changes -->
        <div
          v-else-if="currentFile.status === 'identical' || (currentDiff && currentDiff.add_count === 0 && currentDiff.del_count === 0 && currentDiff.mod_count === 0)"
          class="diff-identical-notice"
        >
          <div class="notice-icon">
            <SvgIcon name="check" :size="32" :stroke-width="2.5" />
          </div>
          <h4>两端配置内容完全相同</h4>
          <p>当前本地配置文件与备份内容完全一致，没有差异。</p>
          <button
            class="btn btn-secondary btn-sm"
            style="margin-top: 10px;"
            title="在软件内置文本编辑器中打开此文件"
            @click="handleOpenEditor"
          >
            <SvgIcon name="edit" :size="13" />
            <span>在软件内编辑此文件</span>
          </button>
        </div>

        <!-- Side-by-Side Diff View -->
        <div
          v-else-if="viewMode === 'side-by-side' && currentDiff"
          class="diff-view side-by-side"
        >
          <div class="sbs-header-row">
            <div class="sbs-col-header left">
              <span>历史备份 ({{ snapshotId }})</span>
            </div>
            <div class="sbs-col-header right">
              <span>本地当前配置 (Steam 目录)</span>
              <button
                v-if="currentFile"
                class="btn-edit-header"
                title="在内置编辑器中修改此本地文件"
                @click="handleOpenEditor"
              >
                <SvgIcon name="edit" :size="12" />
                <span>在软件内编辑</span>
              </button>
            </div>
          </div>

          <div class="sbs-table-wrap">
            <table class="sbs-table">
              <tbody>
                <tr
                  v-for="(line, idx) in currentDiff.lines"
                  :key="idx"
                  class="diff-row"
                  :class="`row-${line.type}`"
                >
                  <!-- Left: Old / Snapshot Column -->
                  <td class="line-num old-num">
                    {{ line.old_line_num > 0 ? line.old_line_num : '' }}
                  </td>
                  <td class="line-content left-content">
                    <span v-if="line.type === 'mod'" class="content-text">{{ line.old_content }}</span>
                    <span v-else-if="line.type === 'add'" class="content-placeholder"></span>
                    <span v-else class="content-text">{{ line.content }}</span>
                  </td>

                  <!-- Right: New / Live Column -->
                  <td class="line-num new-num">
                    {{ line.new_line_num > 0 ? line.new_line_num : '' }}
                  </td>
                  <td class="line-content right-content">
                    <span v-if="line.type === 'del'" class="content-placeholder"></span>
                    <span v-else class="content-text">{{ line.content }}</span>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- Unified Diff View -->
        <div
          v-else-if="viewMode === 'unified' && currentDiff"
          class="diff-view unified"
        >
          <div class="unified-table-wrap">
            <table class="unified-table">
              <thead>
                <tr>
                  <th style="width: 50px;">旧行</th>
                  <th style="width: 50px;">新行</th>
                  <th style="width: 30px; text-align: center;">±</th>
                  <th>内容</th>
                </tr>
              </thead>
              <tbody>
                <template v-for="(line, idx) in currentDiff.lines" :key="idx">
                  <!-- For Mod in unified view: show old deleted line then new added line -->
                  <template v-if="line.type === 'mod'">
                    <tr class="diff-row row-del">
                      <td class="line-num">{{ line.old_line_num }}</td>
                      <td class="line-num"></td>
                      <td class="diff-sign">-</td>
                      <td class="line-content">{{ line.old_content }}</td>
                    </tr>
                    <tr class="diff-row row-add">
                      <td class="line-num"></td>
                      <td class="line-num">{{ line.new_line_num }}</td>
                      <td class="diff-sign">+</td>
                      <td class="line-content">{{ line.content }}</td>
                    </tr>
                  </template>

                  <!-- Standard same / add / del -->
                  <tr
                    v-else
                    class="diff-row"
                    :class="`row-${line.type}`"
                  >
                    <td class="line-num">
                      {{ line.old_line_num > 0 ? line.old_line_num : '' }}
                    </td>
                    <td class="line-num">
                      {{ line.new_line_num > 0 ? line.new_line_num : '' }}
                    </td>
                    <td class="diff-sign">
                      {{ line.type === 'add' ? '+' : line.type === 'del' ? '-' : ' ' }}
                    </td>
                    <td class="line-content">
                      {{ line.content }}
                    </td>
                  </tr>
                </template>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- Modal Footer -->
      <div class="diff-modal-footer">
        <span class="footer-hint">提示：绿色为新增参数，红色为已删除参数，黄色为修改参数。点击【在软件内编辑】可直接修改。</span>
        <div class="footer-buttons">
          <button
            v-if="currentFile"
            class="btn btn-secondary btn-sm"
            @click="handleOpenEditor"
          >
            <SvgIcon name="edit" :size="13" />
            <span>在软件内编辑</span>
          </button>
          <button class="btn btn-secondary btn-sm" @click="close">关闭</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.diff-modal-backdrop {
  position: fixed;
  inset: 0;
  background-color: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(2px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1500;
  padding: 24px;
  animation: fadeIn 0.15s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.diff-modal-window {
  width: 94vw;
  max-width: 1240px;
  height: 86vh;
  max-height: 880px;
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-dialog);
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* Header */
.diff-modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 20px;
  background-color: var(--bg-surface-alt);
  border-bottom: 1px solid var(--border-color);
}

.modal-tag {
  font-size: 12px;
  font-weight: 600;
  color: var(--accent);
  margin-bottom: 2px;
}

.modal-title {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.highlight-id {
  color: var(--level-info);
  font-family: var(--font-mono);
}

.highlight-live {
  color: var(--level-success);
}

.compare-arrow {
  color: var(--color-text-secondary);
}

.account-tag {
  font-size: 12px;
  font-weight: normal;
  padding: 2px 8px;
  background-color: var(--bg-control);
  border: 1px solid var(--border-control);
  border-radius: var(--radius-sm);
  color: var(--color-text-secondary);
  font-family: var(--font-mono);
}

.header-right {
  display: flex;
  align-items: center;
  gap: 10px;
}

.view-toggle {
  display: flex;
  background-color: var(--bg-control);
  padding: 2px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-control);
}

.toggle-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 12px;
  font-size: 12px;
  font-weight: 500;
  background: transparent;
  color: var(--color-text-secondary);
  border: none;
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: all 0.15s ease;
}

.toggle-btn.active {
  background-color: var(--accent);
  color: #ffffff;
  font-weight: 600;
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

/* Toolbar */
.diff-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 20px;
  background-color: #232323;
  border-bottom: 1px solid var(--border-color);
  gap: 12px;
  flex-wrap: wrap;
}

.file-selector-group {
  display: flex;
  align-items: center;
  gap: 10px;
}

.file-label {
  font-size: 13px;
  color: var(--color-text-secondary);
  font-weight: 500;
}

.file-select {
  background-color: var(--bg-control);
  color: var(--color-text);
  border: 1px solid var(--border-control);
  padding: 4px 10px;
  border-radius: var(--radius-md);
  font-size: 12px;
  outline: none;
  cursor: pointer;
}

.file-select:focus {
  border-color: var(--accent);
}

.status-pill {
  font-size: 12px;
  font-weight: 600;
  padding: 2px 7px;
  border-radius: var(--radius-sm);
}

.badge-mod {
  background-color: rgba(252, 225, 0, 0.15);
  color: var(--level-warning);
  border: 1px solid rgba(252, 225, 0, 0.3);
}

.badge-add {
  background-color: var(--tone-success-weak);
  color: var(--level-success);
  border: 1px solid rgba(108, 203, 95, 0.3);
}

.badge-del {
  background-color: var(--tone-error-weak);
  color: var(--level-error);
  border: 1px solid var(--tone-error-strong);
}

.badge-same {
  background-color: var(--bg-control);
  color: var(--color-text-secondary);
  border: 1px solid var(--border-control);
}

/* WinUI 3 styled [在软件内编辑] Button */
.btn-edit-inline {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 28px;
  padding: 0 10px;
  background-color: var(--accent-subtle);
  color: var(--level-info);
  border: 1px solid rgba(0, 120, 212, 0.35);
  border-radius: var(--radius-md);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
}

.btn-edit-inline:hover {
  background-color: var(--accent);
  color: #ffffff;
  border-color: var(--accent);
}

.btn-edit-header {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  background-color: var(--accent-subtle);
  color: var(--level-info);
  border: 1px solid var(--tone-accent-strong);
  border-radius: var(--radius-sm);
  font-size: 12px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.15s ease;
}

.btn-edit-header:hover {
  background-color: var(--accent);
  color: #ffffff;
  border-color: var(--accent);
}

.stats-counter-group {
  display: flex;
  gap: 8px;
}

.stat-pill {
  font-size: 12px;
  font-family: var(--font-mono);
  font-weight: 600;
  padding: 2px 7px;
  border-radius: var(--radius-sm);
}

.pill-add {
  background-color: var(--tone-success-weak);
  color: var(--level-success);
}

.pill-del {
  background-color: var(--tone-error-weak);
  color: var(--level-error);
}

.pill-mod {
  background-color: rgba(252, 225, 0, 0.15);
  color: var(--level-warning);
}

/* Diff Body */
.diff-body {
  flex: 1;
  overflow: auto;
  background-color: #1a1a1a;
  position: relative;
}

.diff-empty,
.diff-identical-notice {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: var(--color-text-secondary);
  gap: 8px;
  text-align: center;
}

.notice-icon {
  color: var(--level-success);
  background: rgba(108, 203, 95, 0.12);
  width: 56px;
  height: 56px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 8px;
}

.diff-identical-notice h4 {
  font-size: 15px;
  color: #ffffff;
}

/* Side by Side Diff */
.side-by-side {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.sbs-header-row {
  display: flex;
  background-color: #232323;
  border-bottom: 1px solid var(--border-color);
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
  position: sticky;
  top: 0;
  z-index: 10;
}

.sbs-col-header {
  flex: 1;
  padding: 6px 14px;
}

.sbs-col-header.left {
  border-right: 1px solid var(--border-color);
}

.sbs-col-header.right {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.sbs-table-wrap,
.unified-table-wrap {
  flex: 1;
  overflow-y: auto;
}

.sbs-table,
.unified-table {
  width: 100%;
  border-collapse: collapse;
  font-family: var(--font-mono);
  font-size: 12px;
  line-height: 1.6;
}

.diff-row {
  border-bottom: 1px solid rgba(255, 255, 255, 0.03);
}

.line-num {
  width: 45px;
  text-align: right;
  padding: 2px 8px;
  color: var(--color-text-muted);
  user-select: none;
  font-size: 12px;
  background-color: rgba(0, 0, 0, 0.25);
}

.line-content {
  padding: 2px 10px;
  white-space: pre-wrap;
  word-break: break-all;
}

.left-content {
  border-right: 1px solid var(--border-color);
  width: 48%;
}

.right-content {
  width: 48%;
}

.content-placeholder {
  display: block;
  min-height: 1.6em;
  background-color: var(--surface-1);
}

/* Highlighting styles */
.row-add {
  background-color: rgba(108, 203, 95, 0.1);
}
.row-add .right-content {
  background-color: rgba(108, 203, 95, 0.18);
  color: #b6e8af;
}

.row-del {
  background-color: rgba(255, 153, 164, 0.1);
}
.row-del .left-content {
  background-color: rgba(255, 153, 164, 0.18);
  color: #ff99a4;
}

.row-mod {
  background-color: rgba(252, 225, 0, 0.08);
}
.row-mod .left-content {
  background-color: var(--tone-error-weak);
  color: #ff99a4;
}
.row-mod .right-content {
  background-color: var(--tone-success-weak);
  color: #b6e8af;
}

.diff-sign {
  width: 24px;
  text-align: center;
  font-weight: bold;
  user-select: none;
}
.row-add .diff-sign { color: var(--level-success); }
.row-del .diff-sign { color: var(--level-error); }

/* Modal Footer */
.diff-modal-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 20px;
  background-color: var(--bg-surface-alt);
  border-top: 1px solid var(--border-color);
  gap: 12px;
}

.footer-hint {
  font-size: 12px;
  color: var(--color-text-secondary);
}

.footer-buttons {
  display: flex;
  align-items: center;
  gap: 8px;
}
</style>
