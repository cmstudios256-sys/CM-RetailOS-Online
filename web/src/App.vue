<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ApiError, apiRequest, applyOnlinePrefsToDom } from './api'
import WorkspaceShell from './components/WorkspaceShell.vue'

type User = {
  id: string
  shopId: string
  fullName: string
  email: string
  role: string
  shopName: string
  currency: string
  googleConnected?: boolean
  googleEmail?: string
  googlePictureUrl?: string
}

const user = ref<User | null>(null)
const loading = ref(true)
const loadError = ref('')

async function loadPublicWorkspace() {
  loading.value = true
  loadError.value = ''
  try {
    const workspace = await apiRequest<{ shopName: string; currency: string }>('/public/workspace')
    user.value = {
      id: 'public',
      shopId: '',
      fullName: 'Public Viewer',
      email: '',
      role: 'public',
      shopName: workspace.shopName,
      currency: workspace.currency,
    }
  } catch (error) {
    user.value = null
    loadError.value = error instanceof ApiError
      ? error.message
      : error instanceof Error
        ? error.message
        : 'Failed to connect to workspace server.'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  applyOnlinePrefsToDom()
  void loadPublicWorkspace()
})
</script>

<template>
  <main class="page-shell">
    <WorkspaceShell
      v-if="user"
      :user="user"
      public-mode
    />
    <div v-else class="boot-screen">
      <div class="boot-card">
        <img src="/cmretailos-brand-logo.png" alt="CM RetailOS" class="boot-logo" />
        <h1 class="boot-brand">CM RETAILOS</h1>
        <template v-if="loading">
          <div class="boot-spinner"></div>
          <p class="boot-status">Opening dashboard...</p>
        </template>
        <template v-else>
          <p class="boot-error">{{ loadError }}</p>
          <button type="button" class="boot-retry-btn" @click="loadPublicWorkspace">Retry connection</button>
        </template>
      </div>
    </div>
  </main>
</template>

<style scoped>
.page-shell {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--bg-app, #020617);
}

.boot-screen {
  min-height: 100dvh;
  display: grid;
  place-items: center;
  padding: 24px;
  background: radial-gradient(circle at 50% 50%, #0f172a 0%, #020617 100%);
}

.boot-card {
  width: min(380px, 100%);
  padding: 36px 28px;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  border: 1px solid rgba(255, 255, 255, .08);
  border-radius: 20px;
  background: rgba(15, 23, 42, .8);
}

.boot-logo { width: 68px; height: 68px; object-fit: contain; }
.boot-brand { margin: 14px 0 22px; color: #34d399; font-size: 16px; letter-spacing: .16em; }
.boot-spinner { width: 30px; height: 30px; margin-bottom: 12px; border: 3px solid rgba(16, 185, 129, .18); border-top-color: #10b981; border-radius: 50%; animation: spin .8s linear infinite; }
.boot-status { margin: 0; color: #94a3b8; font-size: 13px; }
.boot-error { margin: 0 0 16px; color: #fca5a5; font-size: 13px; }
.boot-retry-btn { padding: 9px 16px; border: 0; border-radius: 8px; color: #fff; background: #10b981; font-weight: 650; cursor: pointer; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>
