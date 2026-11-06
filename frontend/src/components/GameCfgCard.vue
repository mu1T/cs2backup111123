<script setup lang="ts">
/**
 * 游戏通用配置卡片（game/csgo/cfg）。
 *
 * 只负责展示与事件上抛：文件清单、勾选集合、加载态、扫描动作全部由父级视图持有，
 * 这里不碰绑定层，也不自己扫描目录。
 */
import SvgIcon from './SvgIcon.vue'
import CfgFilePicker from './CfgFilePicker.vue'
import type { GameCFGFile } from '../types'

withDefaults(
  defineProps<{
    cs2CfgPath: string
    files: GameCFGFile[]
    selected: string[]
    quickFiles: GameCFGFile[]
    actionLoading?: boolean
    scanning?: boolean
    canRollback?: boolean
  }>(),
  {
    actionLoading: false,
    scanning: false,
    canRollback: false,
  }
)

defineEmits<{
  (e: 'backup-all'): void
  (e: 'backup-custom'): void
  (e: 'restore'): void
  (e: 'rollback'): void
  (e: 'open-dir'): void
  (e: 'open-path-settings'): void
  (e: 'edit-file', name: string): void
  (e: 'toggle-file', name: string): void
  (e: 'select-all'): void
  (e: 'select-none'): void
  (e: 'select-player'): void
  (e: 'refresh'): void
  (e: 'backup-checked'): void
}>()
</script>

<template>
  <div class="winui-card card-game-cfg">
    <div class="card-header">
      <div class="card-header-left">
        <h3 class="card-title">
          <SvgIcon name="gamepad" size="16" class="icon-dim" />
          <span>游戏通用配置 (game/csgo/cfg)</span>
        </h3>
        <p class="card-desc">备份或还原全局共享的 autoexec.cfg、键位及自定义配置文件</p>
      </div>
      <div class="card-header-right">
        <span class="cfg-path-indicator mono-font" :title="cs2CfgPath || '未检测到 CS2 路径'">
          {{ cs2CfgPath ? '已挂载' : '未检测到' }}
        </span>
      </div>
    </div>

    <div class="game-cfg-actions-row">
      <button
        class="btn-winui-standard"
        :disabled="actionLoading || !cs2CfgPath"
        title="全量备份 game/csgo/cfg 全部文件"
        @click="$emit('backup-all')"
      >
        <SvgIcon name="archive" size="14" />
        <span>备份全部</span>
      </button>
      <button
        class="btn-winui-standard"
        :disabled="actionLoading || !cs2CfgPath"
        title="剔除 Valve 默认配置，仅备份自定义 CFG 脚本"
        @click="$emit('backup-custom')"
      >
        <SvgIcon name="zap" size="14" />
        <span>仅备份自定义</span>
      </button>
      <button
        class="btn-winui-standard"
        :disabled="actionLoading || !cs2CfgPath"
        title="选择要还原的版本（最新一次或任意历史版本）"
        @click="$emit('restore')"
      >
        <SvgIcon name="restore" size="14" />
        <span>还原</span>
      </button>
      <button
        v-if="canRollback"
        class="btn-winui-danger-subtle"
        :disabled="actionLoading || !cs2CfgPath"
        title="撤销上一次通用 CFG 还原，回退至还原前自动备份的状态"
        @click="$emit('rollback')"
      >
        <SvgIcon name="refresh" size="14" />
        <span>撤销还原</span>
      </button>
      <button
        class="btn-winui-standard"
        :disabled="actionLoading || !cs2CfgPath"
        title="在 Windows 资源管理器中打开 CFG 目录"
        @click="$emit('open-dir')"
      >
        <SvgIcon name="folder" size="14" />
        <span>打开目录</span>
      </button>
    </div>

    <!-- 空状态：未检测到 CS2 安装目录时，解释按钮为何不可用 -->
    <div v-if="!cs2CfgPath" class="cfg-empty-hint">
      <SvgIcon name="alert-triangle" size="14" class="hint-icon" />
      <span class="hint-text">未检测到 CS2 安装目录，请先在路径设置中配置</span>
      <button type="button" class="hint-action" @click="$emit('open-path-settings')">打开路径设置</button>
    </div>

    <!-- 快速编辑：直接打开常见脚本 -->
    <div v-if="quickFiles.length > 0" class="quick-edit-row">
      <span class="quick-edit-label">
        <SvgIcon name="edit" size="12" />
        <span>快速编辑</span>
      </span>
      <button
        v-for="f in quickFiles"
        :key="f.name"
        class="quick-edit-chip"
        :disabled="!cs2CfgPath"
        :title="`在编辑器内打开 ${f.name}`"
        @click="$emit('edit-file', f.name)"
      >
        <span class="mono-font">{{ f.name }}</span>
      </button>
    </div>

    <!-- 按文件勾选备份 -->
    <CfgFilePicker
      :files="files"
      :selected="selected"
      :disabled="actionLoading || !cs2CfgPath"
      :path-missing="!cs2CfgPath"
      :loading="actionLoading"
      :scanning="scanning"
      @toggle="$emit('toggle-file', $event)"
      @select-all="$emit('select-all')"
      @select-none="$emit('select-none')"
      @select-player="$emit('select-player')"
      @refresh="$emit('refresh')"
      @backup="$emit('backup-checked')"
    />
  </div>
</template>

<style scoped>

/* 卡片容器的背景/描边由父级 .winui-card.card-game-cfg 控制（scoped 会作用到子组件根元素），
   这里只负责卡片内部元素。 */
.card-game-cfg .card-title {
  font-size: 13px;
  color: rgba(255, 255, 255, 0.86);
}

.card-game-cfg .card-desc {
  color: rgba(255, 255, 255, 0.42);
}

.icon-dim {
  color: rgba(255, 255, 255, 0.4);
}

.cfg-path-indicator {
  font-size: 12px;
  padding: 2px 7px;
  border-radius: 4px;
  background: var(--surface-3);
  border: 1px solid var(--surface-4);
  color: rgba(255, 255, 255, 0.7);
}

.game-cfg-actions-row {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.game-cfg-actions-row > button {
  flex: 1 1 170px;
  min-width: 0;
}

.btn-winui-standard {
  height: 32px;
  padding: 0 12px;
  font-size: 13px;
  font-weight: 500;
  border-radius: 6px;
  background: var(--stroke-1);
  color: #ffffff;
  border: 1px solid var(--stroke-2);
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  cursor: pointer;
  transition: all 0.15s ease;
  user-select: none;
}

.btn-winui-standard:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.12);
  border-color: var(--stroke-3);
}

.btn-winui-standard:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

/* 空状态提示：路径未配置时解释按钮不可用 */
.cfg-empty-hint {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border-radius: 6px;
  background: rgba(252, 225, 0, 0.06);
  border: 1px dashed rgba(252, 225, 0, 0.24);
  color: rgba(255, 255, 255, 0.66);
  font-size: 12px;
}

.cfg-empty-hint .hint-icon {
  color: var(--level-warning);
  flex-shrink: 0;
}

.cfg-empty-hint .hint-text {
  flex: 1;
  min-width: 0;
}

.cfg-empty-hint .hint-action {
  flex-shrink: 0;
  height: 22px;
  padding: 0 8px;
  font-family: var(--font-sans);
  font-size: 12px;
  font-weight: 500;
  color: var(--accent-light);
  background: transparent;
  border: 1px solid var(--tone-info-stroke);
  border-radius: var(--radius-pill);
  cursor: pointer;
  transition: background-color var(--duration-fast) var(--ease-standard);
}

.cfg-empty-hint .hint-action:hover {
  background: var(--accent-subtle-hover);
}

/* 快速编辑行 */
.quick-edit-row {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.quick-edit-label {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  font-size: 12px;
  color: var(--color-text-muted);
  flex-shrink: 0;
}

.quick-edit-chip {
  height: 24px;
  padding: 0 8px;
  background: var(--bg-control);
  border: 1px solid var(--border-control);
  border-radius: var(--radius-pill);
  color: var(--color-text-secondary);
  font-family: var(--font-sans);
  font-size: 12px;
  cursor: pointer;
  transition: background-color var(--duration-fast) var(--ease-standard),
    color var(--duration-fast) var(--ease-standard),
    border-color var(--duration-fast) var(--ease-standard);
}

.quick-edit-chip:hover:not(:disabled) {
  background: var(--accent-subtle);
  border-color: var(--tone-info-stroke);
  color: var(--accent-light);
}

.quick-edit-chip:disabled {
  opacity: 0.4;
  cursor: default;
}

/* 矮窗口收尾覆盖：放在基础规则之后才能生效 */
@media (max-height: 760px) {
  .game-cfg-actions-row > button {
    flex: 1 1 150px;
    font-size: 12px;
    padding: 0 8px;
  }
}
</style>
