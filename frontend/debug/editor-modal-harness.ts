/**
 * 一次性探针：把「编辑器弹窗」整块挂到真实浏览器里。
 *
 * 和 editor-harness 的分工：那个只看代码编辑器本身（补全/校验/样式），
 * 这个看弹窗层面的东西——作用域切换、文件下拉、新建弹层的定位。
 *
 * 浏览器里没有 Wails 后端，`utils/wails` 会自动退回 mock 数据，
 * 所以新建文件也是「建在内存里」，足够验证交互与布局。
 */
import '../src/style.css'
import { createApp, h, ref } from 'vue'
import TextEditorModal from '../src/components/TextEditorModal.vue'
import { installTooltip } from '../src/utils/tooltip'

installTooltip()

const visible = ref(true)

createApp({
  setup() {
    return () =>
      h('div', { style: 'background:#0f1116;min-height:100vh' }, [
        h(TextEditorModal, {
          visible: visible.value,
          accountId: '123456789',
          initialScope: 'game',
          initialFile: 'autoexec.cfg',
          'onUpdate:visible': (v: boolean) => {
            visible.value = v
          },
        }),
      ])
  },
}).mount('#app')
