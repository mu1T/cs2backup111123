<script setup lang="ts">
/**
 * CFG 代码编辑器：高亮 + 结构校验 + 补全 + 指令搜索 + 指令说明栏。
 *
 * 换掉原来的 `<textarea>`：那套要自己画行号栏、自己同步滚动、自己插 Tab，
 * 而这三件事 CodeMirror 都是现成的。更重要的是补全与诊断需要「知道光标在哪、
 * 当前行是什么」——在 textarea 上做这些会写出一堆边界 bug。
 *
 * 扩展分工：
 * - `basicSetup`：行号、撤销栈、括号匹配、搜索
 * - `linter` + `analyzeCfg`：把结构校验画成波浪线 + 行号栏警告点
 * - `autocompletion` + `cfgCommands`：指令/键名/动作补全
 * - 指令搜索面板（Ctrl+K）：按名字/类别/中文说明搜，解决「知道效果不知道指令名」
 * - 底部说明栏：光标落在已有指令上就显示它是干嘛的（读别人的 cfg 时用）
 *
 * 曾经做过一个「鼠标悬停在指令上弹注释」的浮层（`hoverTooltip`），已经删了：
 * 鼠标在正文上移动就冒泡，影响正常阅读；底部说明栏常驻、不挡视线，够了。
 */
import { ref, computed, watch, nextTick, onMounted, onBeforeUnmount } from 'vue'
import { EditorView, keymap, tooltips } from '@codemirror/view'
import { EditorState } from '@codemirror/state'
import { basicSetup } from 'codemirror'
import { linter, lintGutter, type Diagnostic as CmDiagnostic } from '@codemirror/lint'
import {
  autocompletion,
  acceptCompletion,
  completionStatus,
  selectedCompletionIndex,
  setSelectedCompletion,
  type CompletionContext,
  type CompletionResult,
} from '@codemirror/autocomplete'
import { defaultKeymap, indentWithTab } from '@codemirror/commands'
import { analyzeCfg } from '../utils/cfgDiagnostics'
import { cfgLanguage } from '../utils/cfgLanguage'
import { planCompletion, metaOf, NO_DESC, type CompletionOption } from '../utils/cfgCompletion'
import { CFG_WORD_AT_END, findCommand, searchCommands, type CfgCommand } from '../utils/cfgCommands'

const props = defineProps<{
  modelValue: string
  readonly?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

const host = ref<HTMLDivElement | null>(null)
let view: EditorView | null = null
/** 自己发出去的变化不要回灌，否则光标会被重置。 */
let applyingExternal = false

/** 光标/选区当前落在哪条已知指令上；底部说明栏用它。 */
const currentCommand = ref<CfgCommand | null>(null)

/* ---------------- 指令搜索面板 ---------------- */

const searchOpen = ref(false)
const searchQuery = ref('')
const searchIndex = ref(0)
const searchInput = ref<HTMLInputElement | null>(null)
const searchPanel = ref<HTMLDivElement | null>(null)

/** 搜名字 / 类别 / 中文说明；空查询给前 50 条，不至于一片空白。 */
const searchHits = computed(() => searchCommands(searchQuery.value, 60))

// 换查询词时选中项回到第一条，否则会停在一个不存在的位置上
watch(searchQuery, () => {
  searchIndex.value = 0
})

function openSearch() {
  if (props.readonly) return
  searchOpen.value = true
  searchQuery.value = ''
  searchIndex.value = 0
  nextTick(() => searchInput.value?.focus())
}

function closeSearch() {
  if (!searchOpen.value) return
  searchOpen.value = false
  view?.focus()
}

/** 把选中的指令插到光标处，替换掉光标前那个残缺的词。 */
function insertCommand(name: string) {
  if (!view) return
  const pos = view.state.selection.main.head
  const line = view.state.doc.lineAt(pos)
  const before = line.text.slice(0, pos - line.from)
  const word = CFG_WORD_AT_END.exec(before)
  const from = pos - (word ? word[0].length : 0)

  view.dispatch({
    changes: { from, to: pos, insert: name },
    selection: { anchor: from + name.length },
    scrollIntoView: true,
  })
  closeSearch()
}

function onSearchKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') {
    e.preventDefault()
    closeSearch()
    return
  }
  if (e.key === 'k' && (e.ctrlKey || e.metaKey)) {
    e.preventDefault()
    closeSearch()
    return
  }
  if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
    e.preventDefault()
    const step = e.key === 'ArrowDown' ? 1 : -1
    const last = searchHits.value.length - 1
    searchIndex.value = Math.max(0, Math.min(last, searchIndex.value + step))
    nextTick(() => {
      searchPanel.value?.querySelector('.cfg-hit.is-active')?.scrollIntoView({ block: 'nearest' })
    })
    return
  }
  if (e.key === 'Enter') {
    e.preventDefault()
    const hit = searchHits.value[searchIndex.value]
    if (hit) insertCommand(hit.name)
  }
}

/** 结构校验 → CodeMirror 的波浪线。行号/列号转成文档偏移。 */
const cfgLinter = linter(
  (v) => {
    const doc = v.state.doc
    const out: CmDiagnostic[] = []

    for (const d of analyzeCfg(doc.toString())) {
      // 行号越界时（文件正在被改）跳过，不要让整条 lint 挂掉
      if (d.line < 1 || d.line > doc.lines) continue
      const line = doc.line(d.line)
      const from = Math.min(line.from + Math.max(0, d.column - 1), line.to)
      // 取值类问题只标那一段；其他规则标整行
      const to = d.length ? Math.min(from + d.length, line.to) : line.to > from ? line.to : from
      const fix = d.fix

      out.push({
        from,
        to,
        severity: d.severity,
        message: d.hint ? `${d.message}\n${d.hint}` : d.message,
        // 「修复建议」落到 CodeMirror 的 action 上：波浪线上会出一个「改成 X」
        actions: fix
          ? [
              {
                name: `改成 ${fix.value}`,
                apply: (v, actFrom, actTo) => {
                  v.dispatch({ changes: { from: actFrom, to: actTo, insert: fix.value } })
                },
              },
            ]
          : undefined,
      })
    }
    return out
  },
  // 停止输入 400ms 再算，敲字时不闪
  { delay: 400 },
)

/**
 * 补全入口。真正的决策在 `utils/cfgCompletion.ts` 里，那边有 14 条用例——
 * 上一版把这段逻辑埋在这里，出了「菜单整个不弹」的故障却只能靠手敲验证。
 *
 * `filter: false`：候选已经自己筛过了，且要支持「空格当下划线」这种
 * CodeMirror 默认过滤处理不了的规则。
 */
function cfgCompletion(ctx: CompletionContext): CompletionResult | null {
  const line = ctx.state.doc.lineAt(ctx.pos)
  const before = line.text.slice(0, ctx.pos - line.from)
  const plan = planCompletion(before)
  if (!plan || plan.options.length === 0) return null
  return { from: line.from + plan.from, options: plan.options, filter: false }
}

/**
 * 光标/选区落在一条已知指令上时，返回它的词条（底部说明栏用它）。
 *
 * 优先看选区：用户选中一段文字时，那段文字本身就是他想查的东西。
 */
function commandAtSelection(state: EditorState): CfgCommand | null {
  const sel = state.selection.main

  if (!sel.empty) {
    const picked = state.doc.sliceString(sel.from, sel.to).trim()
    const hit = findCommand(picked)
    if (hit) return hit
  }

  const word = state.wordAt(sel.head)
  if (!word) return null
  return findCommand(state.doc.sliceString(word.from, word.to)) ?? null
}


/** 深色主题：颜色全部取应用已有的 CSS 变量，跟界面一致。 */
const appTheme = EditorView.theme(
  {
    '&': { backgroundColor: 'transparent', color: 'var(--color-text)', fontSize: '12.5px' },
    '.cm-content': { fontFamily: 'var(--font-mono)', caretColor: 'var(--accent)' },
    '.cm-gutters': {
      backgroundColor: 'transparent',
      color: 'var(--color-text-muted)',
      border: 'none',
    },
    '.cm-activeLine': { backgroundColor: 'rgba(255, 255, 255, 0.03)' },
    '.cm-activeLineGutter': { backgroundColor: 'transparent' },
    '&.cm-focused': { outline: 'none' },
    '.cm-selectionBackground, ::selection': { backgroundColor: 'rgba(255, 255, 255, 0.15)' },
    '.cm-tooltip': {
      backgroundColor: 'var(--bg-card-solid)',
      border: '1px solid var(--border-color)',
      borderRadius: '6px',
    },
    '.cm-tooltip-autocomplete > ul > li[aria-selected]': {
      backgroundColor: 'var(--accent)',
      color: 'var(--accent-text, #fff)',
    },
  },
  { dark: true },
)

function buildState(doc: string) {
  return EditorState.create({
    doc,
    extensions: [
      basicSetup,
      /*
       * Tab 接受补全。
       *
       * CodeMirror 的 `completionKeymap` 只绑了 Enter（Ctrl-Space 开、Escape 关、
       * 上下键选），**没有 Tab**。而我们又显式加了 `indentWithTab`，于是 Tab 直接
       * 被拿去缩进——菜单明明开着，按下去只是多了两个空格。
       *
       * `acceptCompletion` 在没有补全时返回 false，事件会继续往下走到
       * `indentWithTab`，所以「菜单开着就补全、没开就缩进」两件事一条绑定就够。
       */
      keymap.of([
        { key: 'Tab', run: acceptCompletion },
        // Ctrl+K 打开指令搜索。CodeMirror 自带的是 Ctrl+F（文内查找），不冲突。
        { key: 'Mod-k', run: () => { openSearch(); return true } },
        ...defaultKeymap,
        indentWithTab,
      ]),
      appTheme,
      cfgLanguage,
      cfgLinter,
      // 行号栏里的警告圆点。光靠文字底下的波浪线太不显眼——2px、贴着字底，
      // 用户的原话就是「你说的波浪线警告并没有」。
      lintGutter(),
      // 挂到 body：编辑器容器有圆角和边框，tooltip 放在里面会被裁掉
      tooltips({ parent: document.body }),
      autocompletion({
        override: [cfgCompletion],
        maxRenderedOptions: 12,
        // 需要 sv_cheats 1 的候选加一个类，样式上跟普通指令区分开
        optionClass: (c) => ((c as CompletionOption).cheats ? 'cm-cfg-cheat' : ''),
      }),
      EditorView.updateListener.of((u) => {
        // 光标一动就更新底部说明栏：这就是「选中就能看注释」
        if (u.docChanged || u.selectionSet) currentCommand.value = commandAtSelection(u.state)
        if (!u.docChanged || applyingExternal) return
        emit('update:modelValue', u.state.doc.toString())
      }),
      EditorState.readOnly.of(!!props.readonly),
    ],
  })
}

onMounted(() => {
  if (!host.value) return
  view = new EditorView({ state: buildState(props.modelValue), parent: host.value })
  document.addEventListener('mousemove', onListHover)
})

onBeforeUnmount(() => {
  document.removeEventListener('mousemove', onListHover)
  view?.destroy()
  view = null
})

/**
 * 鼠标停在候选上时把那一项选中，右侧说明浮层跟着走。
 *
 * CodeMirror 的补全列表**自己只处理 mousedown**，没有 mousemove——所以默认
 * 鼠标划过去毫无反应，说明浮层只跟着键盘选中项。`setSelectedCompletion` 是
 * 官方导出的 effect，正好用来把「悬停」接成「选中」。
 *
 * 两个刻意的选择：
 * - 监听 `mousemove` 而不是 `mouseover`：菜单恰好弹在静止的光标下面时，
 *   浏览器也会派发 mouseover，那会把默认选中的第一项抢掉；`mousemove` 只在
 *   指针真的移动时才来。
 * - 先用 `completionStatus` 挡一道：绝大多数鼠标移动发生在没有补全的时候，
 *   没必要每次都去 `closest()` 找列表。
 *
 * 列表挂在 document.body 上（`tooltips({ parent })`），不在编辑器 DOM 里，
 * 所以只能挂在 document 上监听。`li.id` 的结尾就是它在候选数组里的下标。
 */
function onListHover(e: MouseEvent) {
  if (!view || completionStatus(view.state) !== 'active') return

  const target = e.target as HTMLElement | null
  const li = target?.closest?.('.cm-tooltip-autocomplete li') as HTMLElement | null
  if (!li?.id) return

  const m = /-(\d+)$/.exec(li.id)
  if (!m) return
  const index = Number(m[1])
  if (index === selectedCompletionIndex(view.state)) return

  view.dispatch({ effects: setSelectedCompletion(index) })
}

// 外部换文件（切到另一个 cfg）时整篇替换，不逐字符 diff
watch(
  () => props.modelValue,
  (val) => {
    if (!view) return
    const current = view.state.doc.toString()
    if (current === val) return

    applyingExternal = true
    view.dispatch({
      changes: { from: 0, to: current.length, insert: val },
      selection: { anchor: 0 },
    })
    applyingExternal = false
  },
)
</script>

<template>
  <div class="cfg-editor-wrap">
    <div ref="host" class="cfg-code-editor" @mousedown="closeSearch"></div>

    <!--
      搜索指令的入口。放在右上角而不是行号栏：行号栏是「每行一个」的位置，
      每行都挂个灯泡太吵，只在光标行挂又依赖 gutter 的重绘时机（改文档才重绘，
      单纯移光标不保证）。固定按钮反而不会被漏掉。
    -->
    <button
      v-if="!readonly"
      class="cfg-search-btn"
      type="button"
      title="搜索指令（Ctrl+K）：知道效果但不知道指令名时用"
      @click="openSearch"
    >
      <svg viewBox="0 0 24 24" width="15" height="15" fill="none" stroke="currentColor" stroke-width="1.8">
        <path d="M9 18h6M10 21h4" stroke-linecap="round" />
        <path d="M12 3a6 6 0 0 0-3.6 10.8c.5.4.8 1 .8 1.6V16h5.6v-.6c0-.6.3-1.2.8-1.6A6 6 0 0 0 12 3Z" stroke-linejoin="round" />
      </svg>
    </button>

    <div v-if="searchOpen" ref="searchPanel" class="cfg-search-panel">
      <input
        ref="searchInput"
        v-model="searchQuery"
        class="cfg-search-input"
        type="text"
        placeholder="搜指令：名字 / 类别 / 说明，例如「冻结」「准星」「noclip」"
        @keydown="onSearchKeydown"
      />
      <div class="cfg-search-list">
        <p v-if="searchHits.length === 0" class="cfg-search-empty">没有匹配的指令</p>
        <button
          v-for="(hit, i) in searchHits"
          :key="hit.name"
          type="button"
          class="cfg-hit"
          :class="{ 'is-active': i === searchIndex }"
          @mouseenter="searchIndex = i"
          @click="insertCommand(hit.name)"
        >
          <span class="cfg-hit-name">{{ hit.name }}</span>
          <span class="cfg-hit-cat">{{ hit.category }}</span>
          <span v-if="hit.cheats" class="cfg-hit-cheat">需作弊</span>
          <span class="cfg-hit-desc">{{ hit.desc || NO_DESC }}</span>
        </button>
      </div>
      <p class="cfg-search-foot">↑↓ 选择 · Enter 插入 · Esc 关闭</p>
    </div>

    <!--
      底部说明栏：光标停在已有指令上、或者选中它，这里就显示这条指令是干嘛的。
      场景是「打开一个别人写的 cfg，看不懂里面某行」——补全帮不上忙，那些字
      已经写在文件里了。
    -->
    <div class="cfg-doc-bar" :class="{ 'is-empty': !currentCommand }">
      <template v-if="currentCommand">
        <span class="cfg-doc-name">{{ currentCommand.name }}</span>
        <span class="cfg-doc-meta">{{ metaOf(currentCommand) }}</span>
        <span v-if="currentCommand.cheats" class="cfg-doc-cheat">需 sv_cheats 1</span>
        <span class="cfg-doc-text">{{ currentCommand.desc || NO_DESC }}</span>
      </template>
      <span v-else class="cfg-doc-hint">把光标放在指令上，就能看到这条指令是干嘛的</span>
    </div>
  </div>
</template>

<style scoped>
/* 外壳：给搜索按钮和面板提供定位基准 */
.cfg-editor-wrap {
  position: relative;
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  min-width: 0;
}

.cfg-code-editor {
  flex: 1;
  min-height: 0;
  min-width: 0;
  overflow: hidden;
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--bg-surface);
}

/* CodeMirror 自己撑满容器并内部滚动 */
.cfg-code-editor :deep(.cm-editor) {
  height: 100%;
}

.cfg-code-editor :deep(.cm-scroller) {
  overflow: auto;
}

/* ---- 底部说明栏 ---- */
.cfg-doc-bar {
  flex: none;
  display: flex;
  align-items: baseline;
  gap: 8px;
  min-height: 26px;
  padding: 4px 10px 5px;
  border-top: 1px solid var(--border-color);
  background: rgba(255, 255, 255, 0.02);
  font-size: 11.5px;
  line-height: 1.5;
  overflow: hidden;
  white-space: nowrap;
}

.cfg-doc-name {
  flex: none;
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--accent);
}

.cfg-doc-meta {
  flex: none;
  color: var(--color-text-muted);
}

.cfg-doc-cheat {
  flex: none;
  color: var(--level-warning);
}

.cfg-doc-text {
  flex: 1;
  min-width: 0;
  color: var(--color-text);
  overflow: hidden;
  text-overflow: ellipsis;
}

.cfg-doc-hint {
  color: var(--color-text-muted);
  opacity: 0.7;
}

/* ---- 搜索按钮 ---- */
.cfg-search-btn {
  position: absolute;
  top: 7px;
  right: 16px;
  width: 26px;
  height: 26px;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  /*
   * 常驻底色 + 边框，而不是「悬停才出现」。
   * 它是这个功能的唯一入口，第一眼看不见就等于没有——第一版做成纯透明图标，
   * 在深色编辑器右上角基本看不出来。
   */
  border: 1px solid var(--border-color);
  border-radius: 6px;
  background: var(--bg-card-solid);
  color: var(--color-text-muted);
  cursor: pointer;
  opacity: 0.8;
  transition: color 0.15s, opacity 0.15s, border-color 0.15s;
}

.cfg-search-btn:hover {
  opacity: 1;
  color: var(--accent);
  border-color: var(--accent);
}

/* ---- 搜索面板 ---- */
.cfg-search-panel {
  position: absolute;
  top: 38px;
  right: 16px;
  width: 460px;
  max-width: calc(100% - 32px);
  max-height: calc(100% - 52px);
  display: flex;
  flex-direction: column;
  background: var(--bg-card-solid);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.5);
  overflow: hidden;
  z-index: 5;
}

.cfg-search-input {
  flex: none;
  height: 32px;
  padding: 0 12px;
  border: none;
  border-bottom: 1px solid var(--border-color);
  background: transparent;
  color: var(--color-text);
  font-size: 13px;
  outline: none;
}

.cfg-search-input::placeholder {
  color: var(--color-text-muted);
}

.cfg-search-list {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 4px;
}

.cfg-search-empty {
  margin: 0;
  padding: 14px 10px;
  color: var(--color-text-muted);
  font-size: 12.5px;
  text-align: center;
}

.cfg-hit {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 2px 8px;
  width: 100%;
  padding: 6px 8px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.cfg-hit.is-active {
  background: var(--accent);
}

.cfg-hit-name {
  font-family: var(--font-mono);
  font-size: 12.5px;
  color: var(--color-text);
}

.cfg-hit-cat {
  font-size: 11px;
  color: var(--color-text-muted);
  text-align: right;
}

.cfg-hit-cheat {
  font-size: 10.5px;
  color: var(--level-warning);
}

.cfg-hit-desc {
  grid-column: 1 / -1;
  font-size: 11.5px;
  color: var(--color-text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* 选中行反白：里面的文字全部转深色，否则在 accent 底上糊成一团 */
.cfg-hit.is-active .cfg-hit-name,
.cfg-hit.is-active .cfg-hit-cat,
.cfg-hit.is-active .cfg-hit-desc,
.cfg-hit.is-active .cfg-hit-cheat {
  color: var(--accent-text, #fff);
}

.cfg-search-foot {
  flex: none;
  margin: 0;
  padding: 6px 12px;
  border-top: 1px solid var(--border-color);
  color: var(--color-text-muted);
  font-size: 11px;
}
</style>

<!--
  非 scoped：补全菜单与诊断气泡挂在 document.body 上，scoped 样式选不中。
  层级必须显式抬到弹窗遮罩之上，否则菜单「弹了但看不见」——见 .cm-tooltip 的注释。
-->
<style>
/* ---- CodeMirror 补全菜单与诊断气泡 ----
 * 放在这个组件的**非 scoped** style 块里：这些元素被 `tooltips({ parent: document.body })`
 * 挂到了 body 上，不在组件 DOM 子树里，scoped 的属性选择器选不中它们。
 * 之所以从全局 style.css 挪过来：它们只有这个组件会用到，而且和下面的补全逻辑
 * 放在一起，改层级、改配色时不会漏掉一半。
 */
.cm-tooltip {
  /*
   * 必须显式抬层级，否则菜单是「隐形」的。
   *
   * CodeMirror 自己的基础样式把 z-index 定在 500；一旦把 tooltip 挂到 body 上，
   * 它就和弹窗遮罩成了同级兄弟——而 TextEditorModal 的遮罩是 1600。于是菜单
   * 被压在 85% 不透明的遮罩下面，用户看到的就是「补全菜单根本没弹出来」。
   *
   * 取 2500：压过编辑弹窗（1600）和 toast 浮层（2000），又低于确认框（9999）
   * 与全局提示（10000）——那两者该盖住补全菜单。
   */
  z-index: 2500 !important;
  background: var(--bg-card-solid) !important;
  border: 1px solid var(--border-color) !important;
  border-radius: 8px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.45);
  color: var(--color-text);
  font-size: 12.5px;
  /*
   * 这里**不能**写 overflow: hidden。
   *
   * 选中候选时右侧那块说明浮层（`.cm-completionInfo`）是补全菜单的**子元素**，
   * 用 `left: 100%` 定位在菜单框外面。给菜单裁一刀，说明浮层就整个被裁掉——
   * 它的 getBoundingClientRect 仍然报 340×118，看起来「存在」，但一个像素都
   * 画不出来。这个 bug 用 jsdom 或 rect 检查都发现不了，只有真浏览器截图能看见。
   *
   * 圆角裁切交给下面的 ul 自己做（它本来就有 overflow: hidden auto）。
   */
  overflow: visible;
}

.cm-tooltip.cm-tooltip-autocomplete {
  min-width: 280px;
  max-width: 460px;
}

.cm-tooltip-autocomplete > ul {
  max-height: 240px;
  font-family: var(--font-mono);
  /* 菜单本体的圆角裁切放在这里，别放到 .cm-tooltip 上（会连说明浮层一起裁） */
  border-radius: 8px;
}

.cm-tooltip-autocomplete > ul > li {
  padding: 5px 10px;
  line-height: 1.5;
  display: flex;
  align-items: center;
  gap: 8px;
}

.cm-tooltip-autocomplete > ul > li[aria-selected] {
  background: var(--accent);
  color: var(--accent-text, #ffffff);
}

.cm-tooltip-autocomplete .cm-completionDetail {
  color: var(--color-text-muted);
  font-style: normal;
  font-size: 11px;
  margin-left: auto;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.cm-tooltip-autocomplete > ul > li[aria-selected] .cm-completionDetail {
  color: rgba(255, 255, 255, 0.78);
}

/* 诊断气泡：结构校验的波浪线悬停时出现 */
.cm-tooltip.cm-tooltip-lint {
  max-width: 380px;
  padding: 8px 10px;
  line-height: 1.6;
}

/*
 * 需要 sv_cheats 1 的候选：左侧一条橙线。
 *
 * 光靠 detail 里那三个字不够快——玩家在列表里扫的时候，颜色比字先到眼睛。
 */
.cm-tooltip-autocomplete > ul > li.cm-cfg-cheat {
  box-shadow: inset 2px 0 0 var(--level-warning);
}

/*
 * 选中后右侧的详细说明。
 *
 * `white-space: pre-line` 是必须的：说明是「类别 · 侧 · 默认值 / 描述 / 警告」
 * 三行纯文本，默认的 white-space 会把换行折叠成空格，挤成一大坨。
 *
 * 这几条都带 `!important`：CodeMirror 的基础样式同样用
 * `.cm-tooltip.cm-completionInfo` 选择器，权重一样，而它的样式是运行时注入到
 * head 末尾的，后来者居上——不加 !important 就压不住。
 */
.cm-tooltip.cm-completionInfo {
  max-width: 340px !important;
  padding: 8px 10px !important;
  line-height: 1.6 !important;
  white-space: pre-line !important;
  word-break: break-word;
}

.cm-diagnostic-error {
  border-bottom: 2px solid var(--level-error);
}

/*
 * 波浪线加粗、下移一点。
 *
 * 基础样式是 2px 的 SVG 贴在文字底边，压在字形下缘上，在深色背景里几乎看不见。
 * 这两条都带 !important：基础样式同样是单类选择器，而它是运行时注入到 head
 * 末尾的，同权重下后来者赢。
 */
.cm-lintRange {
  background-size: auto 3px !important;
  padding-bottom: 2px !important;
}

/* 行号栏的警告圆点，和波浪线配套 */
.cm-lint-marker-warning {
  filter: drop-shadow(0 0 2px var(--status-warning-stroke));
}

.cm-diagnostic-warning {
  border-bottom: 2px solid var(--level-warning);
}
</style>
