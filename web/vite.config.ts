import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import fs from 'fs'
import path from 'path'

function stampServiceWorkerVersion() {
  return {
    name: 'stamp-service-worker-version',
    closeBundle() {
      const swPath = path.resolve(__dirname, 'dist/service-worker.js')
      if (!fs.existsSync(swPath)) return
      const buildStamp = Date.now().toString(36)
      const content = fs.readFileSync(swPath, 'utf8')
      const updated = content.replace(
        /const CACHE_NAME = ['"][^'"]+['"]/,
        `const CACHE_NAME = 'cm-retailos-shell-${buildStamp}'`,
      )
      fs.writeFileSync(swPath, updated, 'utf8')
    },
  }
}

export default defineConfig({
  plugins: [vue(), stampServiceWorkerVersion()],
  resolve: {
    alias: {
      '@lucide/vue': path.resolve(__dirname, 'node_modules/@lucide/vue/dist/cjs/lucide-vue.js'),
    },
  },
  server: {
    port: 5180,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
})
