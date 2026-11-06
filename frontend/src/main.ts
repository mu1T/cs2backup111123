import { createApp } from 'vue'
import './style.css'
import App from './App.vue'
import { installTooltip } from './utils/tooltip'

// 全局接管原生 title，换成主题一致的浮层提示（见 utils/tooltip.ts）
installTooltip()

createApp(App).mount('#app')
