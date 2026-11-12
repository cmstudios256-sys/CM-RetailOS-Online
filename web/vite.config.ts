import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import fs from 'fs'
import path from 'path'

function stampServiceWorkerVersion() {
  return {
    name: 'stamp-service-worker-version',
    closeBundle() {
      const distDir = path.resolve(__dirname, 'dist')
      const swPath = path.join(distDir, 'service-worker.js')
      if (!fs.existsSync(swPath)) return
      const assetsDir = path.join(distDir, 'assets')
      const precacheAssets = fs.existsSync(assetsDir)
        ? fs.readdirSync(assetsDir)
          .filter(name => name.endsWith('.js') || name.endsWith('.css'))
          .sort()
          .map(name => `/assets/${name}`)
        : []
      const buildStamp = Date.now().toString(36)
      const content = fs.readFileSync(swPath, 'utf8')
      const updated = content
        .replace(
          /const CACHE_NAME = ['"][^'"]+['"]/,
          `const CACHE_NAME = 'cm-retailos-shell-${buildStamp}'`,
        )
        .replace(
          /const PRECACHE_ASSETS = \[\]/,
          `const PRECACHE_ASSETS = ${JSON.stringify(precacheAssets)}`,
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
  build: {
    rollupOptions: {
      output: {
        manualChunks: {
          'vendor-vue': ['vue'],
          'vendor-charts': ['chart.js', 'vue-chartjs'],
          'vendor-icons': ['@lucide/vue'],
        },
      },
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
