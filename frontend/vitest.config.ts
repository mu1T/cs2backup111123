import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import path from 'path'

// 测试配置与 vite.config.ts 分开：构建配置不需要 jsdom，测试也不需要 emptyOutDir 之类的构建项。
// alias 必须与构建保持一致，否则 `@/` 导入在测试里解析不到。
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  test: {
    environment: 'jsdom',
    include: ['src/**/*.spec.ts'],
    globals: true,
  },
})
