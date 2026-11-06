<script setup lang="ts">
import { ref, computed, watch, nextTick, onMounted, onUnmounted } from 'vue'
import SvgIcon from './SvgIcon.vue'
import { useFocusTrap } from '../utils/a11y'

interface Props {
  visible: boolean
  title?: string
  message: string
  detail?: string
  icon?: 'info' | 'warning' | 'danger' | 'restore' | 'switch'
  confirmText?: string
  cancelText?: string
  isDanger?: boolean
  showCancel?: boolean
  prompt?: boolean
  promptPlaceholder?: string
  promptDefault?: string
}

const props = withDefaults(defineProps<Props>(), {
  title: '确认操作',
  icon: 'info',
  confirmText: '确定',
  cancelText: '取消',
  isDanger: false,
  showCancel: true,
  prompt: false,
  promptPlaceholder: '',
  promptDefault: '',
})

const emit = defineEmits<{
  (e: 'confirm', value?: string): void
  (e: 'cancel'): void
}>()

const inputVal = ref('')
const inputRef = ref<HTMLInputElement | null>(null)
const boxRef = ref<HTMLElement | null>(null)

useFocusTrap(
  computed(() => props.visible),
  boxRef
)

watch(
  () => props.visible,
  (val) => {
    if (val) {
      inputVal.value = props.promptDefault || ''
      nextTick(() => {
        if (props.prompt) {
          inputRef.value?.focus()
          inputRef.value?.select()
        } else {
          boxRef.value?.focus()
        }
      })
    }
  }
)

// 覆盖层本身通常不持有焦点，仅靠 @keydown 无法响应 Esc，这里监听 window
const handleGlobalKeydown = (e: KeyboardEvent) => {
  if (!props.visible) return
  if (e.key === 'Escape') {
    e.preventDefault()
    handleCancel()
  }
}

onMounted(() => window.addEventListener('keydown', handleGlobalKeydown))
onUnmounted(() => window.removeEventListener('keydown', handleGlobalKeydown))

const handleConfirm = () => {
  emit('confirm', props.prompt ? inputVal.value : undefined)
}

const handleCancel = () => {
  emit('cancel')
}
</script>

<template>
  <Teleport to="body">
    <div
      v-if="visible"
      class="confirm-modal-overlay"
      @click.self="handleCancel"
      @keydown.enter.prevent="handleConfirm"
    >
      <div
        ref="boxRef"
        class="confirm-modal-box"
        role="dialog"
        aria-modal="true"
        aria-labelledby="confirm-modal-title"
        tabindex="-1"
      >
        <!-- Header / Icon -->
        <div class="confirm-header">
          <div class="confirm-icon-wrap" :class="`icon-${icon}`">
            <SvgIcon
              v-if="icon === 'danger' || icon === 'warning'"
              name="warning"
              :size="18"
            />
            <SvgIcon
              v-else-if="icon === 'restore'"
              name="restore"
              :size="18"
            />
            <SvgIcon
              v-else-if="icon === 'switch'"
              name="user"
              :size="18"
            />
            <SvgIcon
              v-else
              name="info"
              :size="18"
            />
          </div>
          <div class="confirm-title-area">
            <h3 id="confirm-modal-title" class="confirm-title">{{ title }}</h3>
            <p class="confirm-message">{{ message }}</p>
          </div>
        </div>

        <!-- Detail message if present -->
        <div v-if="detail" class="confirm-detail-box">
          <span>{{ detail }}</span>
        </div>

        <!-- Prompt input if needed -->
        <div v-if="prompt" class="confirm-prompt-area">
          <input
            ref="inputRef"
            v-model="inputVal"
            class="input-control confirm-input"
            :placeholder="promptPlaceholder"
            @keydown.enter.prevent.stop="handleConfirm"
          />
        </div>

        <!-- Footer Actions -->
        <div class="confirm-actions">
          <button
            v-if="showCancel"
            class="btn btn-secondary btn-cancel"
            @click="handleCancel"
          >
            {{ cancelText }}
          </button>
          <button
            class="btn"
            :class="isDanger ? 'btn-danger' : 'btn-primary'"
            @click="handleConfirm"
          >
            {{ confirmText }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.confirm-modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background-color: rgba(0, 0, 0, 0.5);
  backdrop-filter: blur(2px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}

.confirm-modal-box {
  width: 100%;
  max-width: 420px;
  background-color: #2c2c2c;
  border: 1px solid rgba(255, 255, 255, 0.09);
  border-radius: var(--radius-card);
  box-shadow: var(--shadow-dialog);
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  outline: none;
  animation: modalPop var(--duration-normal) var(--ease-standard);
}

@keyframes modalPop {
  from {
    opacity: 0;
    transform: scale(0.96) translateY(6px);
  }
  to {
    opacity: 1;
    transform: scale(1) translateY(0);
  }
}

.confirm-header {
  display: flex;
  align-items: flex-start;
  gap: 14px;
}

.confirm-icon-wrap {
  width: 36px;
  height: 36px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.icon-info {
  background-color: rgba(0, 120, 212, 0.15);
  color: var(--accent);
}

.icon-warning {
  background-color: rgba(255, 185, 0, 0.15);
  color: #ffb900;
}

.icon-danger {
  background-color: rgba(232, 17, 35, 0.15);
  color: #e81123;
}

.icon-restore {
  background-color: rgba(16, 124, 65, 0.15);
  color: #107c41;
}

.icon-switch {
  background-color: rgba(0, 120, 212, 0.15);
  color: var(--accent);
}

.confirm-title-area {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.confirm-title {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--color-text);
  line-height: 1.3;
}

.confirm-message {
  margin: 0;
  font-size: 13px;
  color: var(--color-text-secondary);
  line-height: 1.5;
  white-space: pre-wrap;
}

.confirm-detail-box {
  background-color: var(--bg-card);
  border: 1px solid var(--border-color);
  border-radius: var(--radius-md);
  padding: 10px 12px;
  font-size: 12px;
  color: var(--color-text-muted);
  line-height: 1.4;
}

.confirm-prompt-area {
  display: flex;
  flex-direction: column;
}

.confirm-input {
  width: 100%;
  box-sizing: border-box;
}

.confirm-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 4px;
}

.btn-danger {
  background-color: #d13438;
  color: #ffffff;
}

.btn-danger:hover {
  background-color: #e81123;
}
</style>
