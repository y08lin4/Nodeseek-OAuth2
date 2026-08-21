// vite 配置（纯 JS 形式：避免 vite 用 esbuild 打包 TS 配置，
// 规避沙箱对子进程 spawn 的限制；内容与 TS 版一致）
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// 开发服务器代理：/api 与 /oauth 前缀转发到本地 Go 后端（见 SPEC.md 3.3 契约）
export default defineConfig({
  plugins: [vue()],
  // 显式 define：正常构建环境（CI/Docker）由 vite+esbuild 替换 process.env.NODE_ENV，
  // 与 index.html 的 polyfill 双保险，确保产物不依赖 Node 全局。
  define: {
    'process.env.NODE_ENV': JSON.stringify('production'),
  },
  build: {
    // 沙箱兼容：跳过 esbuild 压缩（stub 只做转译/透传），产物有效但未压缩
    minify: false,
    cssMinify: false,
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/oauth': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
})
