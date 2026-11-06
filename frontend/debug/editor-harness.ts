/**
 * 一次性探针：把 CFG 编辑器单独挂到一个真实浏览器里。
 *
 * 目的：jsdom 里「元素存在」不等于「肉眼看得见」——裁剪、层级、位置这些只有真
 * 浏览器算得出来。这个文件是**长期保留**的调试入口：`npm run dev` 之后打开
 * `/debug/editor-harness.html` 就能看到一个独立的编辑器，改样式时先在这儿看，
 * 别在应用里靠猜。它不参与构建（不是 index.html 的入口）。
 */
import '../src/style.css'
import { createApp, h, ref } from 'vue'
import CfgCodeEditor from '../src/components/CfgCodeEditor.vue'
import { installTooltip } from '../src/utils/tooltip'

// 悬停提示依赖全局 tooltip（main.ts 里装的），探针要自己装一份
installTooltip()

const doc = ref(
  [
    '// 练习 cfg：这些在官方服务器上都不会生效',
    'bind F noclip',
    'bind G give weapon_ak47',
    'cl_radar_scale 0.33',
    'cl_radar_scale_alternate 1.00',
    'noclip',
    'cl_crosshairstyle 7',
    'sensitivity 2.5',
  ].join('\n'),
)

createApp({
  setup() {
    return () =>
      h(
        // 复刻 TextEditorModal 的遮罩层：真应用里唯一的上下文差异就在这里
        // （fixed 全屏 + 85% 黑 + blur + z-index 1600）
        'div',
        {
          style:
            'position: fixed; inset: 0; background: rgba(10,12,16,0.85); backdrop-filter: blur(6px); z-index: 1600; display: flex; align-items: center; justify-content: center; padding: 24px;',
        },
        [
          h(
            'div',
            {
              style:
                'width: 820px; height: 420px; display: flex; padding: 16px; box-sizing: border-box; background: #14161c; border-radius: 10px;',
            },
            [
              h(CfgCodeEditor, {
                modelValue: doc.value,
                'onUpdate:modelValue': (v: string) => {
                  doc.value = v
                },
              }),
            ],
          ),
        ],
      )
  },
}).mount('#app')
