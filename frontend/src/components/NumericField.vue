<script setup lang="ts">
/**
 * 一个「标签 + 数字框 + 滑杆」的数值参数。
 *
 * 之前这段结构在准星编辑器里逐字符重复了九遍，差异只有范围、步长与小数位。
 * 收成一个 module 之后，范围来自 `utils/crosshairFields.ts` 的元数据表，
 * 夹取规则只写一遍，模板里每个参数是一行。
 *
 * 手输的行为有意分成两段：
 * - 输入过程中只接受合法值，超范围先不写回——避免打字打到一半被夹走；
 * - 失焦 / 回车时才夹取并格式化，保证模型里永远是合法值。
 */
const props = withDefaults(
  defineProps<{
    label: string
    /** hover 说明。 */
    tip?: string
    min: number
    max: number
    step?: number
    /** 写回模型时保留的小数位；0 表示取整。 */
    decimals?: number
  }>(),
  { tip: '', step: 1, decimals: 0 },
)

const model = defineModel<number>({ required: true })

function clampAndFormat(raw: number): number {
  const fallback = Number.isFinite(raw) ? raw : props.min
  const clamped = Math.min(props.max, Math.max(props.min, fallback))
  return props.decimals === 0 ? Math.round(clamped) : Number(clamped.toFixed(props.decimals))
}

function onInput(e: Event) {
  const el = e.target as HTMLInputElement
  // 清空输入框时不要写回：Number('') 是 0，会让用户「删掉旧值准备重输」
  // 的瞬间把参数改成 0（下界允许 0 的字段更是毫无提示地归零）。
  if (el.value.trim() === '') return
  const raw = Number(el.value)
  if (!Number.isFinite(raw)) return
  if (raw < props.min || raw > props.max) return
  model.value = raw
}

function onChange(e: Event) {
  const el = e.target as HTMLInputElement
  const v = clampAndFormat(Number(el.value))
  model.value = v
  // 模型值没变时 Vue 不会重渲染，这里手动把夹取结果写回输入框。
  el.value = String(v)
}
</script>

<template>
  <div class="field">
    <span class="field-label">
      <span :title="tip || undefined">{{ label }}</span>
      <input
        class="num-input"
        type="number"
        :min="min"
        :max="max"
        :step="step"
        :value="model"
        @input="onInput"
        @change="onChange"
      />
    </span>
    <input v-model.number="model" type="range" :min="min" :max="max" :step="step" class="slider" />
  </div>
</template>

<style scoped>
.field {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.field-label {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 11px;
  color: var(--color-text-secondary);
}

.num-input {
  width: 54px;
  height: 20px;
  padding: 0 5px;
  text-align: right;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: var(--radius-sm);
  color: var(--color-text);
  font-size: 11.5px;
  font-weight: 600;
  cursor: text;
  font-family: var(--font-mono);
  font-variant-numeric: tabular-nums;
  transition: var(--transition-fast);
}

.num-input:hover {
  background: var(--bg-control);
  border-color: var(--border-control);
}

.num-input:focus {
  outline: none;
  background: var(--bg-control-active);
  border-color: var(--accent);
}

.num-input::-webkit-inner-spin-button,
.num-input::-webkit-outer-spin-button {
  -webkit-appearance: none;
  margin: 0;
}

.slider {
  width: 100%;
  height: 18px;
  accent-color: var(--accent);
  cursor: pointer;
}
</style>
