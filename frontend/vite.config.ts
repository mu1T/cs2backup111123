import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import path from 'path'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        // 指令表（284 KB）单独成块：它是**数据**不是代码，跟编辑器内核分开之后
        // 编辑器主块回到 400 KB 左右，500 KB 的告警阈值就仍然是「有意义的信号」，
        // 不会被一张静态表长期占满。
        manualChunks: {
          'cs2-commands': ['./src/data/cs2Commands.generated.ts'],
        },
      },
    },
  },
})
