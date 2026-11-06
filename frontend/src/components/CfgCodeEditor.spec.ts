import { describe, it, expect, afterEach, beforeAll } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { EditorView } from '@codemirror/view'
import { selectedCompletion } from '@codemirror/autocomplete'
import CfgCodeEditor from './CfgCodeEditor.vue'

/**
 * 补全的**接线**测试，不是逻辑测试。
 *
 * `cfgCompletion.spec.ts` 证明「给定光标前的一行，该弹什么」算得对；但用户报的
 * 「菜单根本不弹」是接线问题——扩展没挂上、`override` 没生效、tooltip 被裁掉，
 * 这些纯函数测试一个都抓不到。上一轮就是这么翻车的：14 条逻辑用例全绿，菜单
 * 却压根没出现。
 *
 * 所以这里真的把编辑器挂到 DOM 上，真的往里面敲字，然后去 DOM 里找那个菜单。
 */

beforeAll(() => {
  // jsdom 不做布局：Range/Element 的 rect 系列方法不存在，CodeMirror 测量时会炸。
  // 给一个「有宽度但为零高度」的最小实现，够它算出位置就行。
  const zeroRect = () =>
    ({ x: 0, y: 0, top: 0, left: 0, right: 0, bottom: 0, width: 0, height: 0, toJSON: () => ({}) }) as DOMRect

  if (!Range.prototype.getClientRects) {
    Range.prototype.getClientRects = () => [] as unknown as DOMRectList
  }
  if (!Range.prototype.getBoundingClientRect) {
    Range.prototype.getBoundingClientRect = zeroRect
  }
  if (!Element.prototype.getClientRects) {
    Element.prototype.getClientRects = () => [] as unknown as DOMRectList
  }
})

let wrapper: VueWrapper | null = null

afterEach(() => {
  wrapper?.unmount()
  wrapper = null
  document.body.innerHTML = ''
})

function mountEditor(initial = '') {
  wrapper = mount(CfgCodeEditor, {
    props: { modelValue: initial },
    attachTo: document.body,
  })
  const dom = wrapper.find('.cm-editor').element as HTMLElement
  const view = EditorView.findFromDOM(dom)
  if (!view) throw new Error('编辑器没挂上')
  return view
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))

/** 逐字符敲进去，每敲一下等一会儿——补全有 100ms 的防抖。 */
async function typeChars(view: EditorView, text: string) {
  for (const ch of text) {
    const pos = view.state.selection.main.head
    view.dispatch({
      changes: { from: pos, insert: ch },
      selection: { anchor: pos + 1 },
      userEvent: 'input.type',
    })
    await sleep(130)
  }
}

/** 等菜单出现。补全激活有防抖，不能同步断言。 */
async function waitForMenu(timeout = 2000): Promise<HTMLElement | null> {
  const deadline = Date.now() + timeout
  while (Date.now() < deadline) {
    const el = document.querySelector('.cm-tooltip-autocomplete')
    if (el) return el as HTMLElement
    await sleep(30)
  }
  return null
}

const menuLabels = (menu: HTMLElement) =>
  Array.from(menu.querySelectorAll('li')).map((li) => li.textContent?.trim() || '')

describe('CfgCodeEditor 的补全菜单', () => {
  it('敲两个字母就弹出候选（用户报的「菜单不弹」就是这个）', async () => {
    const view = mountEditor()
    await typeChars(view, 'cl')
    const menu = await waitForMenu()

    expect(menu, '敲了 cl 却没有出现补全菜单').not.toBeNull()
    const labels = menuLabels(menu!)
    expect(labels.length).toBeGreaterThan(0)
    expect(labels.some((l) => l.startsWith('cl_'))).toBe(true)
  })

  it('菜单挂在 body 上，不被编辑器容器裁掉', async () => {
    const view = mountEditor()
    await typeChars(view, 'cl')
    const menu = await waitForMenu()
    expect(menu).not.toBeNull()

    // 挂在编辑器容器里的话，圆角边框 + overflow:hidden 会把它裁掉
    const insideEditor = menu!.closest('.cm-editor')
    expect(insideEditor).toBeNull()
    expect(menu!.closest('body')).toBe(document.body)
  })

  it('菜单里有可选项，且带说明文字', async () => {
    const view = mountEditor()
    await typeChars(view, 'cl_show')
    const menu = await waitForMenu()
    expect(menu).not.toBeNull()

    const labels = menuLabels(menu!)
    expect(labels.some((l) => l.includes('cl_showfps'))).toBe(true)
    expect(menu!.querySelector('.cm-completionDetail')).not.toBeNull()
  })

  it('bind 之后弹键名', async () => {
    const view = mountEditor()
    await typeChars(view, 'bind ')
    const menu = await waitForMenu()
    expect(menu, 'bind 之后没有键名候选').not.toBeNull()
    expect(menuLabels(menu!).some((l) => l.includes('MOUSE1'))).toBe(true)
  })

  it('空格当下划线：敲 cl show 也能命中 cl_showfps', async () => {
    const view = mountEditor()
    await typeChars(view, 'cl show')
    const menu = await waitForMenu()
    expect(menu).not.toBeNull()
    expect(menuLabels(menu!).some((l) => l.includes('cl_showfps'))).toBe(true)
  })

  it('带参数的赋值行不弹（不刷无关候选）', async () => {
    const view = mountEditor()
    await typeChars(view, 'sensitivity 2.5')
    // 给它足够时间弹，然后确认它没弹
    await sleep(400)
    expect(document.querySelector('.cm-tooltip-autocomplete')).toBeNull()
  })

  it('Escape 能关掉菜单', async () => {
    const view = mountEditor()
    await typeChars(view, 'cl')
    expect(await waitForMenu()).not.toBeNull()

    view.contentDOM.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Escape', code: 'Escape', bubbles: true }),
    )
    await sleep(200)
    expect(document.querySelector('.cm-tooltip-autocomplete')).toBeNull()
  })
})

/** 往编辑器里按一个键。CodeMirror 的 keymap 挂在 contentDOM 上。 */
function pressKey(view: EditorView, key: string, code = key) {
  view.contentDOM.dispatchEvent(
    new KeyboardEvent('keydown', { key, code, keyCode: key === 'Tab' ? 9 : 13, bubbles: true }),
  )
}

describe('接受补全', () => {
  it('Tab 接受补全（用户报的「摁 tab 不会自动补全」）', async () => {
    const view = mountEditor()
    await typeChars(view, 'cl_show')
    expect(await waitForMenu()).not.toBeNull()

    pressKey(view, 'Tab')
    await sleep(150)

    expect(view.state.doc.toString()).toBe('cl_showfps')
  })

  it('Tab 接受后菜单要关掉', async () => {
    const view = mountEditor()
    await typeChars(view, 'cl_show')
    expect(await waitForMenu()).not.toBeNull()

    pressKey(view, 'Tab')
    await sleep(150)

    expect(document.querySelector('.cm-tooltip-autocomplete')).toBeNull()
  })

  it('对照组：Enter 本来就能接受', async () => {
    const view = mountEditor()
    await typeChars(view, 'cl_show')
    expect(await waitForMenu()).not.toBeNull()

    pressKey(view, 'Enter')
    await sleep(150)

    expect(view.state.doc.toString()).toBe('cl_showfps')
  })

  it('菜单没开时，Tab 仍然是缩进', async () => {
    const view = mountEditor()
    await typeChars(view, 'echo hi')
    await sleep(300)
    expect(document.querySelector('.cm-tooltip-autocomplete')).toBeNull()

    pressKey(view, 'Tab')
    await sleep(150)

    // 缩进会插空格/制表符，但绝不能把整行替换成指令
    expect(view.state.doc.toString()).toContain('echo hi')
  })

  it('空格当下划线 + Tab：cl show 整段被换成 cl_showfps', async () => {
    const view = mountEditor()
    await typeChars(view, 'cl show')
    expect(await waitForMenu()).not.toBeNull()

    pressKey(view, 'Tab')
    await sleep(150)

    // 替换范围是整行（from=0），所以空格也要一起消失
    expect(view.state.doc.toString()).toBe('cl_showfps')
  })

  it('bind 之后 Tab 补出动作', async () => {
    const view = mountEditor()
    await typeChars(view, 'bind MOUSE1 ')
    expect(await waitForMenu()).not.toBeNull()

    pressKey(view, 'Tab')
    await sleep(150)

    expect(view.state.doc.toString()).toBe('bind MOUSE1 +attack')
  })
})

/** 全量指令表接进来之后，候选上要能看出「这条要不要作弊」。 */
describe('候选上的适用范围', () => {
  it('需要 sv_cheats 的候选带 cm-cfg-cheat 类', async () => {
    const view = mountEditor()
    await typeChars(view, 'nocli')
    const menu = await waitForMenu()
    expect(menu).not.toBeNull()

    const row = Array.from(menu!.querySelectorAll('li')).find((li) =>
      li.textContent?.includes('noclip'),
    )
    expect(row, '菜单里没有 noclip').toBeDefined()
    expect(row!.className).toContain('cm-cfg-cheat')
    // detail 里也要写字，光靠颜色对色盲不友好
    expect(row!.textContent).toContain('需作弊')
  })

  it('普通客户端指令不带这个类', async () => {
    const view = mountEditor()
    await typeChars(view, 'sensitiv')
    const menu = await waitForMenu()
    const row = Array.from(menu!.querySelectorAll('li')).find((li) =>
      li.textContent?.includes('sensitivity'),
    )
    expect(row!.className).not.toContain('cm-cfg-cheat')
    expect(row!.textContent).toContain('仅客户端')
  })

  it('选中时右侧浮层给出中文说明，并解释为什么 autoexec 里没用', async () => {
    const view = mountEditor()
    await typeChars(view, 'nocli')
    expect(await waitForMenu()).not.toBeNull()

    // 菜单一打开就默认选中第一项，info 应该已经渲染
    let info: HTMLElement | null = null
    const deadline = Date.now() + 1500
    while (Date.now() < deadline && !info) {
      info = document.querySelector('.cm-tooltip.cm-completionInfo')
      if (!info) await sleep(30)
    }

    expect(info, '没有出现详细说明浮层').not.toBeNull()
    expect(info!.textContent).toContain('飞过墙壁')
    expect(info!.textContent).toContain('sv_cheats 1')
    expect(info!.textContent).toContain('autoexec')
  })

  /**
   * 说明浮层是**菜单的子元素**，靠 `left: 100%` 定位在菜单框外面。
   * 这个结构决定了菜单不能 `overflow: hidden`（否则浮层被裁掉）。
   * CodeMirror 若改了这个结构，这里会红——那时上面那条样式契约要重新看。
   */
  it('说明浮层嵌在菜单里，而不是挂在 body 上', async () => {
    const view = mountEditor()
    await typeChars(view, 'nocli')
    const menu = await waitForMenu()
    expect(menu).not.toBeNull()

    let info: HTMLElement | null = null
    const deadline = Date.now() + 1500
    while (Date.now() < deadline && !info) {
      info = document.querySelector('.cm-tooltip.cm-completionInfo')
      if (!info) await sleep(30)
    }
    expect(info).not.toBeNull()
    expect(menu!.contains(info!)).toBe(true)
  })

  it('说明里分段：类别行、描述、作弊警告之间有空行', async () => {
    const view = mountEditor()
    await typeChars(view, 'nocli')
    expect(await waitForMenu()).not.toBeNull()

    let info: HTMLElement | null = null
    const deadline = Date.now() + 1500
    while (Date.now() < deadline && !info) {
      info = document.querySelector('.cm-tooltip.cm-completionInfo')
      if (!info) await sleep(30)
    }
    expect(info!.textContent).toContain('\n\n')
  })
})

describe('诊断波浪线', () => {
  it('写了需要 sv_cheats 的指令，编辑器里出现警告标记', async () => {
    const view = mountEditor()
    await typeChars(view, 'noclip')
    // linter 有 400ms 防抖
    await sleep(900)

    const marks = document.querySelectorAll('.cm-lintRange-warning, .cm-lintRange-error')
    expect(marks.length, '没有出现任何波浪线标记').toBeGreaterThan(0)
  })

  it('普通指令不画线', async () => {
    const view = mountEditor()
    await typeChars(view, 'sensitivity 2.5')
    await sleep(900)

    expect(document.querySelectorAll('.cm-lintRange-warning, .cm-lintRange-error').length).toBe(0)
  })
})

describe('指令搜索（知道效果但不知道指令名）', () => {
  const ctrlK = (view: EditorView) =>
    view.contentDOM.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'k', code: 'KeyK', keyCode: 75, ctrlKey: true, bubbles: true }),
    )

  it('右上角有搜索按钮', () => {
    mountEditor()
    const btn = wrapper!.find('.cfg-search-btn')
    expect(btn.exists()).toBe(true)
    expect(btn.attributes('title')).toContain('Ctrl+K')
  })

  it('Ctrl+K 打开搜索面板', async () => {
    const view = mountEditor()
    ctrlK(view)
    await sleep(50)

    expect(wrapper!.find('.cfg-search-panel').exists()).toBe(true)
    expect(wrapper!.find('.cfg-search-input').exists()).toBe(true)
  })

  it('输入中文说明能搜到对应指令', async () => {
    const view = mountEditor()
    ctrlK(view)
    await sleep(50)

    await wrapper!.find('.cfg-search-input').setValue('冻结')
    const names = wrapper!.findAll('.cfg-hit-name').map((n) => n.text())
    expect(names).toContain('mp_freezetime')
  })

  it('输入名字前缀能搜到', async () => {
    const view = mountEditor()
    ctrlK(view)
    await sleep(50)

    await wrapper!.find('.cfg-search-input').setValue('cl_crosshair')
    const names = wrapper!.findAll('.cfg-hit-name').map((n) => n.text())
    expect(names.length).toBeGreaterThan(1)
    expect(names.every((n) => n.startsWith('cl_crosshair'))).toBe(true)
  })

  it('Enter 把选中的指令插进文档并关掉面板', async () => {
    const view = mountEditor()
    ctrlK(view)
    await sleep(50)

    await wrapper!.find('.cfg-search-input').setValue('noclip')
    await wrapper!.find('.cfg-search-input').trigger('keydown', { key: 'Enter' })
    await sleep(80)

    expect(view.state.doc.toString()).toBe('noclip')
    expect(wrapper!.find('.cfg-search-panel').exists()).toBe(false)
  })

  it('插入时替换光标前残缺的词，不会拼成两截', async () => {
    const view = mountEditor('nocl')
    view.dispatch({ selection: { anchor: 4 } })
    ctrlK(view)
    await sleep(50)

    await wrapper!.find('.cfg-search-input').setValue('noclip')
    await wrapper!.find('.cfg-search-input').trigger('keydown', { key: 'Enter' })
    await sleep(80)

    expect(view.state.doc.toString()).toBe('noclip')
  })

  it('Esc 关掉面板', async () => {
    const view = mountEditor()
    ctrlK(view)
    await sleep(50)

    await wrapper!.find('.cfg-search-input').trigger('keydown', { key: 'Escape' })
    await sleep(50)

    expect(wrapper!.find('.cfg-search-panel').exists()).toBe(false)
  })
})

describe('悬停看说明', () => {
  /** 把鼠标移到某一项候选上。列表挂在 body 上，所以事件从 document 冒泡。 */
  const hoverRow = async (menu: HTMLElement, label: string) => {
    const li = Array.from(menu.querySelectorAll('li')).find((el) =>
      el.textContent?.includes(label),
    )
    if (!li) throw new Error(`菜单里没有 ${label}`)
    li.dispatchEvent(new MouseEvent('mousemove', { bubbles: true }))
    await sleep(120)
  }

  it('鼠标停在候选上会把它选中（右侧说明跟着走）', async () => {
    const view = mountEditor()
    await typeChars(view, 'nocli')
    const menu = await waitForMenu()
    expect(menu).not.toBeNull()

    // 先确认默认选中的是第一条，再悬停到另一条上
    await hoverRow(menu!, 'noclip_fixup')
    expect(selectedCompletion(view.state)?.label).toBe('noclip_fixup')
  })

  it('悬停后说明浮层换成那一项的说明', async () => {
    const view = mountEditor()
    await typeChars(view, 'nocli')
    const menu = await waitForMenu()

    await hoverRow(menu!, 'noclip_fixup')

    let info: HTMLElement | null = null
    const deadline = Date.now() + 1500
    while (Date.now() < deadline && !info) {
      info = document.querySelector('.cm-tooltip.cm-completionInfo')
      if (!info) await sleep(30)
    }
    expect(info).not.toBeNull()
    // noclip 的说明是「飞过墙壁」，noclip_fixup 是另一句
    expect(info!.textContent).not.toContain('飞过墙壁')
  })

  it('悬停到已经被选中的那一项上不会重复派发事务', async () => {
    const view = mountEditor()
    await typeChars(view, 'nocli')
    const menu = await waitForMenu()

    await hoverRow(menu!, 'noclip')
    const before = view.state.doc.toString()
    const sel = selectedCompletion(view.state)?.label

    await hoverRow(menu!, 'noclip')
    expect(selectedCompletion(view.state)?.label).toBe(sel)
    expect(view.state.doc.toString()).toBe(before)
  })
})

/**
 * 用户的原话：「玩家编辑一个已经有指令的 cfg 文件，存在现成看不懂的指令，
 * 可以把鼠标常驻或者选择起来可以看注释」。
 *
 * 这是「读」的场景，不是「写」的场景——补全帮不上忙，那些字已经在文件里了。
 */
describe('底部说明栏：光标停在已有指令上看注释', () => {
  const bar = () => wrapper!.find('.cfg-doc-bar')

  it('光标落在已知指令上就显示它的说明', async () => {
    const view = mountEditor('noclip\nsensitivity 2.5')
    view.dispatch({ selection: { anchor: 2 } })
    await sleep(50)

    expect(bar().text()).toContain('noclip')
    expect(bar().text()).toContain('飞过墙壁')
  })

  it('需要作弊的指令额外标出来', async () => {
    const view = mountEditor('noclip')
    view.dispatch({ selection: { anchor: 1 } })
    await sleep(50)

    expect(bar().find('.cfg-doc-cheat').text()).toContain('sv_cheats')
  })

  it('光标移到普通参数上就退回提示语', async () => {
    const view = mountEditor('sensitivity 2.5')
    view.dispatch({ selection: { anchor: 14 } })
    await sleep(50)

    expect(bar().text()).toContain('把光标放在指令上')
  })

  it('选中一整条指令也能看（选区优先于光标所在的词）', async () => {
    const view = mountEditor('mp_roundtime 3')
    view.dispatch({ selection: { anchor: 0, head: 12 } })
    await sleep(50)

    expect(bar().text()).toContain('mp_roundtime')
    expect(bar().text()).toContain('分钟')
  })

  it('切换文件后说明栏跟着更新', async () => {
    const view = mountEditor('noclip')
    view.dispatch({ selection: { anchor: 0 } })
    await sleep(50)
    expect(bar().text()).toContain('飞过墙壁')

    await wrapper!.setProps({ modelValue: 'mp_freezetime 5' })
    await sleep(80)
    view.dispatch({ selection: { anchor: 0 } })
    await sleep(50)

    expect(bar().text()).toContain('mp_freezetime')
    expect(bar().text()).toContain('冻结')
  })
})
