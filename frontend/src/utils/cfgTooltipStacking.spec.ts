import { describe, it, expect } from 'vitest'
// 用 `?raw` 把组件源码当字符串读进来：比 node:fs 少一个 @types/node 依赖，也不用拼路径。
// （`.css?raw` 在测试环境里会被 CSS 处理链吃掉、返回空串，所以只读 .vue。）
import editorSource from '../components/CfgCodeEditor.vue?raw'
import modalSource from '../components/TextEditorModal.vue?raw'

/**
 * 补全菜单「看得见」的前提，用静态检查钉住。
 *
 * 这两条都是真实翻过车的：
 * 1. 菜单挂到 body 之后，层级默认是 CodeMirror 的 500，被 1600 的弹窗遮罩压在
 *    下面 —— 表现就是「菜单根本不弹」，其实弹了，只是看不见。
 * 2. 背景用 `--bg-card`（rgba(255,255,255,0.05)）时，铺在 85% 黑的遮罩上等于
 *    透明，文字糊成一片。
 *
 * jsdom 不做布局也不算层叠，所以这里直接读样式源码做契约检查：以后谁改动层级，
 * 必须同时把补全菜单抬上去，否则测试红。
 */

/** 取出某个选择器对应的声明块（够用就好：只看第一个匹配）。 */
function ruleBody(css: string, selector: string): string {
  const at = css.indexOf(selector)
  if (at < 0) throw new Error(`样式里找不到 ${selector}`)
  const open = css.indexOf('{', at)
  const close = css.indexOf('}', open)
  return css.slice(open + 1, close)
}

/** 去掉注释再断言：注释里会提到被禁止的写法，不能算违规。 */
function withoutComments(block: string): string {
  return block.replace(/\/\*[\s\S]*?\*\//g, '')
}

function zIndexOf(block: string): number {
  const m = /z-index:\s*(\d+)/.exec(block)
  if (!m) throw new Error('这段样式没有 z-index')
  return Number(m[1])
}

describe('CFG 补全菜单的可见性契约', () => {
  const tooltip = ruleBody(editorSource, '.cm-tooltip {')
  const backdrop = ruleBody(modalSource, '.modal-backdrop {')

  it('菜单必须压在编辑弹窗遮罩之上', () => {
    expect(zIndexOf(tooltip)).toBeGreaterThan(zIndexOf(backdrop))
  })

  it('层级要带 !important：CodeMirror 基础样式的选择器权重更高', () => {
    expect(tooltip).toMatch(/z-index:\s*\d+\s*!important/)
  })

  it('背景必须是实色，不能是半透明卡片色', () => {
    expect(tooltip).toMatch(/background:\s*var\(--bg-card-solid\)/)
    expect(tooltip).not.toMatch(/var\(--bg-card\)/)
  })

  it('菜单样式在组件里是非 scoped 的（挂在 body 上，scoped 选不中）', () => {
    // 组件里两个 style 块：scoped 管编辑器容器，非 scoped 管挂到 body 的浮层
    expect(editorSource.match(/<style(\s+scoped)?>/g)).toEqual(['<style scoped>', '<style>'])
  })

  /**
   * 补全菜单**不能**裁剪。
   *
   * 选中候选时右侧那块说明浮层（`.cm-completionInfo`）是菜单的**子元素**，
   * 用 `left: 100%` 定位在菜单框外面。给菜单加 overflow: hidden，说明浮层就
   * 整个被裁掉——而它的 getBoundingClientRect 仍然报 340×118，看起来「存在」，
   * 一个像素都画不出来。jsdom 不算裁剪，rect 检查也看不出，只有真浏览器截图能发现。
   */
  it('菜单不能 overflow: hidden（说明浮层定位在菜单框外，会被裁掉）', () => {
    expect(withoutComments(tooltip)).not.toMatch(/overflow:\s*hidden/)
  })
})
