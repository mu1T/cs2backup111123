<script setup lang="ts">
import { ref, watch } from 'vue'
import SvgIcon from './SvgIcon.vue'
/**
 * 选择要还原的通用 CFG 版本。
 *
 * 为什么是「点了还原才弹」而不是在卡片上常驻下拉：还原是低频操作，
 * 常驻一个下拉会一直占着按钮区，而用户 99% 的情况就是还原最新一次。
 * 点还原 → 看到「最新 + 各历史版本」→ 选一个 → 确认。
 */
const props = withDefaults(
  defineProps<{
    visible: boolean
    /** 历史版本 ID，最新的在前。 */
    versions: string[]
    /** 正在执行还原。 */
    loading?: boolean
  }>(),
  { loading: false },
)

const emit = defineEmits<{
  (e: 'close'): void
  /** version 为空串表示「最新一次备份」。 */
  (e: 'confirm', version: string): void
}>()

/** 空串代表「最新一次备份」这个特殊选项。 */
const chosen = ref('')

watch(
  () => props.visible,
  (val) => {
    if (val) chosen.value = ''
  },
)

const submit = () => {
  if (props.loading) return
  emit('confirm', chosen.value)
}
</script>

<template>
  <Teleport to="body">
    <div v-if="visible" class="picker-overlay" @click.self="emit('close')">
      <div class="picker-dialog" role="dialog" aria-modal="true" aria-labelledby="cfg-version-title">
        <header class="picker-header">
          <h3 id="cfg-version-title">还原通用 CFG</h3>
          <button class="picker-close" title="关闭" aria-label="关闭" @click="emit('close')">
            <SvgIcon name="x" :size="14" />
          </button>
        </header>

        <div class="picker-body">
          <p class="picker-hint">选择要还原到哪一版。还原前系统会自动把当前配置留一份，随时可以撤销。</p>

          <label class="picker-option">
            <input v-model="chosen" type="radio" value="" />
            <span class="option-label">最新一次备份</span>
            <span class="option-note">（推荐）</span>
          </label>

          <label v-for="v in versions" :key="v" class="picker-option">
            <input v-model="chosen" type="radio" :value="v" />
            <span class="option-label mono-font">{{ v }}</span>
          </label>

          <p v-if="versions.length === 0" class="picker-empty">
            还没有历史版本。先「全量备份」一次再还原。
          </p>
        </div>

        <footer class="picker-footer">
          <button class="picker-btn" :disabled="loading" @click="emit('close')">取消</button>
          <button class="picker-btn picker-btn-primary" :disabled="loading" @click="submit">
            {{ loading ? '还原中…' : '确认还原' }}
          </button>
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.picker-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1200;
}

.picker-dialog {
  width: 420px;
  max-width: 90vw;
  max-height: 80vh;
  display: flex;
  flex-direction: column;
  /* 对话框要用实色：--bg-card 是 5% 白，铺在遮罩上等于透明 */
  background: var(--bg-card-solid);
  border: 1px solid var(--border-color);
  border-radius: 10px;
  box-shadow: var(--shadow-dialog);
}

.picker-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 18px;
  border-bottom: 1px solid var(--border-color);
}

.picker-header h3 {
  font-size: 14px;
  font-weight: 600;
}

.picker-close {
  background: transparent;
  border: none;
  color: var(--color-text-muted);
  cursor: pointer;
  padding: 4px;
  display: flex;
}

.picker-close:hover {
  color: var(--color-text);
}

.picker-body {
  padding: 14px 18px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.picker-hint {
  font-size: 12px;
  color: var(--color-text-muted);
  line-height: 1.6;
  margin-bottom: 6px;
}

.picker-option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 10px;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  font-size: 12.5px;
  cursor: pointer;
}

.picker-option:hover {
  background: var(--bg-control-hover);
}

.picker-option input {
  accent-color: var(--accent);
  cursor: pointer;
}

.option-label {
  color: var(--color-text);
}

.option-note {
  font-size: 11px;
  color: var(--color-text-muted);
}

.picker-empty {
  font-size: 12px;
  color: var(--color-text-muted);
  padding: 10px 0;
}

/* 按钮样式写在这里而不是复用 .btn-winui-*：那套类散落在各组件自己的
   scoped 样式里（.btn-winui-standard 只在 GameCfgCard 里定义），组件之间
   拿不到，复用会退化成浏览器原生按钮——边框又细又丑。 */
.picker-btn {
  height: 32px;
  padding: 0 14px;
  font-size: 13px;
  font-weight: 500;
  font-family: inherit;
  border-radius: 6px;
  background: var(--stroke-1, rgba(255, 255, 255, 0.08));
  color: var(--color-text);
  border: 1px solid var(--stroke-2, rgba(255, 255, 255, 0.14));
  cursor: pointer;
  transition: all 0.15s ease;
  user-select: none;
}

.picker-btn:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.12);
  border-color: var(--stroke-3, rgba(255, 255, 255, 0.24));
}

.picker-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.picker-btn-primary {
  background: var(--accent);
  color: var(--accent-text, #ffffff);
  border-color: var(--accent);
}

.picker-btn-primary:hover:not(:disabled) {
  background: var(--accent-hover, var(--accent));
  border-color: var(--accent-hover, var(--accent));
}

.picker-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 12px 18px;
  border-top: 1px solid var(--border-color);
}
</style>
