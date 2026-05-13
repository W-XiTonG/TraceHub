import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import tailwindcss from '@tailwindcss/vite'

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(), tailwindcss()],
  server: {
    proxy: {
      // 与 gin 挂载的 /uploads/site-brand 静态资源一致；开发时 <img src="/uploads/..."> 需走后端
      '/uploads': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/api': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/v1': { target: 'http://127.0.0.1:8080', changeOrigin: true, ws: true },
      '/health': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/ready': { target: 'http://127.0.0.1:8080', changeOrigin: true },
      '/metrics': { target: 'http://127.0.0.1:8080', changeOrigin: true },
    },
  },
})
