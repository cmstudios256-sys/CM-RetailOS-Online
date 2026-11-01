<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ApiError, apiRequest, applyOnlinePrefsToDom, setApiMode, syncOnlinePrefsFromServer } from './api'
import { clearGuestDataAfterClaim, exportGuestBundle } from './guestStore'
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
const guestMode = ref(true)
const demoMode = ref(false)
const loading = ref(true)
const loadError = ref('')
const claimNotice = ref('')

function beginGoogleConnect(requestedPage: string): void {
  const params = new URLSearchParams({ next: requestedPage, claim: 'guest' })
  window.location.assign(`/api/v1/auth/google?${params.toString()}`)
}

async function openGuestWorkspace(): Promise<void> {
  setApiMode('guest')
  const workspace = await apiRequest<{ shopName: string; currency: string; demo: boolean }>('/public/workspace')
  guestMode.value = true
  demoMode.value = workspace.demo
  user.value = {
    id: 'guest',
    shopId: 'guest-device',
    fullName: 'Shop Owner',
    email: '',
    role: 'owner',
    shopName: workspace.shopName,
    currency: workspace.currency,
  }
}

async function openWorkspace(): Promise<void> {
  loading.value = true
  loadError.value = ''
  claimNotice.value = ''
  try {
    setApiMode('cloud')
    const result = await apiRequest<{ user: User | null }>('/auth/session')
    if (!result.user) {
      await openGuestWorkspace()
      return
    }
    guestMode.value = false
    demoMode.value = false
    user.value = result.user
    const params = new URLSearchParams(window.location.search)
    if (params.get('claim') === 'guest') {
      try {
        const bundle = exportGuestBundle()
        await apiRequest('/account/claim', { method: 'POST', body: bundle })
        clearGuestDataAfterClaim()
        await syncOnlinePrefsFromServer(result.user.id)
        params.delete('claim')
        const query = params.toString()
        window.history.replaceState({}, document.title, `${window.location.pathname}${query ? `?${query}` : ''}`)
      } catch (error) {
        claimNotice.value = error instanceof Error ? error.message : 'Guest data could not be saved to the cloud yet.'
      }
    }
  } catch (error) {
    try {
      await openGuestWorkspace()
    } catch (guestError) {
      user.value = null
      loadError.value = guestError instanceof Error
        ? guestError.message
        : error instanceof ApiError
          ? error.message
          : 'Guest data could not be opened in this browser.'
    }
  } finally {
    loading.value = false
  }
}

async function signOut(): Promise<void> {
  try {
    setApiMode('cloud')
    await apiRequest<void>('/auth/logout', { method: 'POST' })
    await openGuestWorkspace()
  } catch (error) {
    user.value = null
    loadError.value = error instanceof Error ? error.message : 'Sign out could not be completed.'
  }
}

onMounted(() => {
  applyOnlinePrefsToDom()
  void openWorkspace()
})
</script>

<template>
  <main class="page-shell">
    <WorkspaceShell
      v-if="user"
      :user="user"
      :guest-mode="guestMode"
      :demo-mode="demoMode"
      :claim-notice="claimNotice"
      @connect-google="beginGoogleConnect"
      @sign-out="signOut"
      @user-updated="user = { ...user, ...$event }"
    />
    <div v-else class="boot-screen">
      <div class="boot-card">
        <img src="/cmretailos-brand-logo.png" alt="CM RetailOS" class="boot-logo" />
        <h1 class="boot-brand">CM RETAILOS</h1>
        <template v-if="loading">
          <div class="boot-spinner"></div>
          <p class="boot-status">Opening your workspace...</p>
        </template>
        <template v-else>
          <p class="boot-error">{{ loadError }}</p>
          <button type="button" class="boot-retry-btn" @click="openWorkspace">Retry</button>
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
