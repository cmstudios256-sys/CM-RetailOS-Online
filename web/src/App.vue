<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { ArrowRight, MapPin, Phone, Store } from '@lucide/vue'
import { ApiError, apiRequest, applyOnlinePrefsToDom, clearSignedInUserLocalData, setApiMode, syncOnlinePrefsFromServer } from './api'
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

type ShopSettings = {
  shopName: string
  currency: string
  address: string
  phone: string
  receiptHeader: string
  receiptFooter: string
  receiptShowPhone: boolean
}

const user = ref<User | null>(null)
const guestMode = ref(true)
const demoMode = ref(false)
const loading = ref(true)
const loadError = ref('')
const claimNotice = ref('')
const businessSetupOpen = ref(false)
const businessSetupLoading = ref(false)
const businessSetupSaving = ref(false)
const businessSetupLoaded = ref(false)
const businessSetupError = ref('')
const loadedShopSettings = ref<ShopSettings | null>(null)
const businessNameInput = ref<HTMLInputElement | null>(null)
const businessSetup = ref<Pick<ShopSettings, 'shopName' | 'phone' | 'address'> & { ownerName: string }>({
  shopName: '',
  ownerName: '',
  phone: '',
  address: '',
})
const canSaveBusinessSetup = computed(() =>
  businessSetup.value.shopName.trim().length > 0 &&
  businessSetup.value.shopName.trim().length <= 120 &&
  businessSetup.value.ownerName.trim().length > 0 &&
  businessSetup.value.ownerName.trim().length <= 100 &&
  businessSetup.value.phone.trim().length > 0 &&
  businessSetup.value.phone.trim().length <= 32 &&
  businessSetup.value.address.trim().length > 0 &&
  businessSetup.value.address.trim().length <= 200,
)

async function markBusinessSetupComplete(existingPreferences?: Record<string, unknown>): Promise<void> {
  const preferences = existingPreferences
    ?? (await apiRequest<{ preferences?: Record<string, unknown> }>('/settings/preferences')).preferences
  await apiRequest('/settings/preferences', {
    method: 'PUT',
    body: {
      preferences: {
        ...preferences,
        __businessProfileSetupComplete: true,
      },
    },
  })
}

async function beginGoogleConnect(requestedPage: string): Promise<void> {
  claimNotice.value = ''
  setApiMode('cloud')
  try {
    const providers = await apiRequest<{ googleEnabled: boolean }>('/auth/providers')
    if (!providers.googleEnabled) {
      claimNotice.value = 'Google sign-in is not configured for this deployment yet.'
      return
    }
  } catch (error) {
    claimNotice.value = error instanceof Error
      ? error.message
      : 'Google sign-in could not be reached. Check your connection and try again.'
    return
  }

  const params = new URLSearchParams({ next: requestedPage, claim: 'guest' })
  window.location.assign(`/api/v1/auth/google?${params.toString()}`)
}

async function loadBusinessSetup(): Promise<void> {
  businessSetupOpen.value = true
  businessSetupLoading.value = true
  businessSetupLoaded.value = false
  businessSetupError.value = ''
  try {
    const result = await apiRequest<{ settings: ShopSettings }>('/settings')
    const settings = result.settings
    loadedShopSettings.value = settings
    businessSetup.value = {
      shopName: settings.shopName === 'My Shop' ? '' : settings.shopName,
      ownerName: user.value?.fullName === 'Shop Owner' ? '' : user.value?.fullName ?? '',
      phone: settings.phone,
      address: settings.address,
    }
    businessSetupLoaded.value = true
    const preferences = await apiRequest<{ preferences?: Record<string, unknown> }>('/settings/preferences')
    if (preferences.preferences?.__businessProfileSetupComplete === true) {
      businessSetupOpen.value = false
      return
    }
    if (settings.shopName.trim() && settings.phone.trim() && settings.address.trim()) {
      await markBusinessSetupComplete(preferences.preferences)
      businessSetupOpen.value = false
      return
    }
    await nextTick()
    businessNameInput.value?.focus()
  } catch (error) {
    loadedShopSettings.value = null
    businessSetupLoaded.value = false
    businessSetupError.value = error instanceof Error
      ? error.message
      : 'Your business details could not be loaded.'
  } finally {
    businessSetupLoading.value = false
  }
}

async function saveBusinessSetup(): Promise<void> {
  if (!canSaveBusinessSetup.value || businessSetupSaving.value || !loadedShopSettings.value) return
  businessSetupSaving.value = true
  businessSetupError.value = ''
  try {
    const result = await apiRequest<{ settings: ShopSettings }>('/settings', {
      method: 'PUT',
      body: {
        ...loadedShopSettings.value,
        shopName: businessSetup.value.shopName.trim(),
        phone: businessSetup.value.phone.trim(),
        address: businessSetup.value.address.trim(),
        receiptShowPhone: true,
      },
    })
    const profile = await apiRequest<{ fullName: string }>('/auth/profile', {
      method: 'PUT',
      body: { fullName: businessSetup.value.ownerName.trim() },
    })
    loadedShopSettings.value = result.settings
    await markBusinessSetupComplete()
    if (user.value) {
      user.value = {
        ...user.value,
        fullName: profile.fullName,
        shopName: result.settings.shopName,
        currency: result.settings.currency,
      }
    }
    businessSetupOpen.value = false
  } catch (error) {
    businessSetupError.value = error instanceof Error
      ? error.message
      : 'Your business details could not be saved. Please try again.'
  } finally {
    businessSetupSaving.value = false
  }
}

function trapBusinessSetupFocus(event: KeyboardEvent): void {
  if (event.key !== 'Tab' || !(event.currentTarget instanceof HTMLElement)) return
  const focusable = event.currentTarget.querySelectorAll<HTMLElement>(
    'input:not(:disabled), button:not(:disabled)',
  )
  if (focusable.length === 0) return
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

async function openGuestWorkspace(): Promise<void> {
  setApiMode('guest')
  const workspace = await apiRequest<{ shopName: string; currency: string; ownerName?: string; demo: boolean }>('/public/workspace')
  businessSetupOpen.value = false
  guestMode.value = true
  demoMode.value = workspace.demo
  user.value = {
    id: 'guest',
    shopId: 'guest-device',
    fullName: workspace.ownerName || 'Shop Owner',
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
  businessSetupOpen.value = false
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
    const shouldSetUpBusiness = params.get('claim') === 'guest'
    if (shouldSetUpBusiness) {
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
    if (shouldSetUpBusiness) {
      await loadBusinessSetup()
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
    clearSignedInUserLocalData()
    claimNotice.value = ''
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
      :key="user.id"
      :user="user"
      :guest-mode="guestMode"
      :demo-mode="demoMode"
      :claim-notice="claimNotice"
      @connect-google="beginGoogleConnect"
      @sign-out="signOut"
      @user-updated="user = { ...user, ...$event }"
    />
    <div v-if="businessSetupOpen" class="business-setup-backdrop">
      <section
        class="business-setup-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="business-setup-title"
        aria-describedby="business-setup-description"
        @keydown.esc.prevent.stop
        @keydown="trapBusinessSetupFocus"
      >
        <div class="business-setup-mark"><Store :size="21" stroke-width="1.8" /></div>
        <h2 id="business-setup-title">Set up your business</h2>
        <p id="business-setup-description" class="business-setup-intro">
          Add your owner name and the business details customers will see on receipts and use to contact you.
        </p>

        <p v-if="businessSetupLoading" class="business-setup-loading" role="status">
          Loading your business details…
        </p>
        <form v-else-if="businessSetupLoaded" class="business-setup-form" @submit.prevent="saveBusinessSetup">
          <label for="business-name">Business name</label>
          <input
            id="business-name"
            ref="businessNameInput"
            v-model="businessSetup.shopName"
            type="text"
            autocomplete="organization"
            maxlength="120"
            required
            placeholder="e.g. Acme Retail"
          />

          <label for="owner-name">Owner's full name</label>
          <input
            id="owner-name"
            v-model="businessSetup.ownerName"
            type="text"
            autocomplete="name"
            maxlength="100"
            required
            placeholder="Enter your full name"
          />

          <label for="business-phone">Phone number</label>
          <div class="business-setup-input-icon">
            <Phone :size="16" aria-hidden="true" />
            <input
              id="business-phone"
              v-model="businessSetup.phone"
              type="tel"
              autocomplete="tel"
              maxlength="32"
              required
              placeholder="+256 700 000 000"
            />
          </div>
          <p class="business-setup-hint">Printed on receipts and available as your WhatsApp business contact.</p>

          <label for="business-address">Physical address</label>
          <div class="business-setup-input-icon">
            <MapPin :size="16" aria-hidden="true" />
            <input
              id="business-address"
              v-model="businessSetup.address"
              type="text"
              autocomplete="street-address"
              maxlength="200"
              required
              placeholder="Street, town or area"
            />
          </div>

          <p v-if="businessSetupError" class="business-setup-error" role="alert">{{ businessSetupError }}</p>
          <button
            class="business-setup-submit"
            type="submit"
            :disabled="!canSaveBusinessSetup || businessSetupSaving"
          >
            {{ businessSetupSaving ? 'Saving details…' : 'Save and continue' }}
            <ArrowRight v-if="!businessSetupSaving" :size="17" />
          </button>
        </form>
        <div v-else class="business-setup-retry">
          <p class="business-setup-error" role="alert">{{ businessSetupError }}</p>
          <button type="button" class="business-setup-submit" :disabled="businessSetupLoading" @click="loadBusinessSetup">
            Try again
          </button>
        </div>
        <p v-if="businessSetupLoaded" class="business-setup-footnote">
          You can update these details anytime in Settings.
        </p>
      </section>
    </div>
    <div v-if="!user" class="boot-screen">
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
.business-setup-backdrop {
  position: fixed;
  z-index: 1000;
  inset: 0;
  display: grid;
  place-items: center;
  overflow-y: auto;
  padding: 24px;
  background: rgba(2, 6, 23, .82);
}
.business-setup-dialog {
  width: min(100%, 480px);
  padding: 32px;
  border: 1px solid #334155;
  border-radius: 16px;
  background: #0f172a;
  box-shadow: 0 24px 70px rgba(0, 0, 0, .45);
  color: #f8fafc;
}
.business-setup-mark {
  width: 44px;
  height: 44px;
  display: grid;
  place-items: center;
  margin-bottom: 20px;
  border: 1px solid rgba(45, 212, 191, .28);
  border-radius: 12px;
  color: #2dd4bf;
  background: rgba(20, 184, 166, .1);
}
.business-setup-dialog h2 { margin: 0; font-size: 24px; line-height: 1.25; letter-spacing: -.025em; }
.business-setup-intro { margin: 9px 0 24px; color: #cbd5e1; font-size: 14px; line-height: 1.55; }
.business-setup-form { display: flex; flex-direction: column; }
.business-setup-form label { margin: 0 0 7px; color: #e2e8f0; font-size: 13px; font-weight: 600; }
.business-setup-form input {
  width: 100%;
  min-width: 0;
  height: 46px;
  margin-bottom: 18px;
  padding: 0 13px;
  border: 1px solid #475569;
  border-radius: 8px;
  outline: none;
  color: #f8fafc;
  background: #1e293b;
  font: inherit;
  font-size: 14px;
  transition: border-color .16s ease, box-shadow .16s ease;
}
.business-setup-form input::placeholder { color: #94a3b8; }
.business-setup-form input:focus { border-color: #2dd4bf; box-shadow: 0 0 0 3px rgba(45, 212, 191, .16); }
.business-setup-input-icon { position: relative; }
.business-setup-input-icon > svg { position: absolute; z-index: 1; top: 15px; left: 13px; color: #94a3b8; pointer-events: none; }
.business-setup-input-icon input { padding-left: 39px; }
.business-setup-hint { margin: -12px 0 19px; color: #a5b4c7; font-size: 12px; line-height: 1.5; }
.business-setup-error { margin: 0 0 14px; color: #fca5a5; font-size: 13px; line-height: 1.5; }
.business-setup-submit {
  min-height: 46px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 9px;
  margin-top: 2px;
  padding: 0 16px;
  border: 0;
  border-radius: 8px;
  color: #042f2e;
  background: #2dd4bf;
  font: inherit;
  font-size: 14px;
  font-weight: 700;
  cursor: pointer;
  transition: background .16s ease, transform .16s ease;
}
.business-setup-submit:hover:not(:disabled) { background: #5eead4; transform: translateY(-1px); }
.business-setup-submit:focus-visible { outline: 3px solid #99f6e4; outline-offset: 3px; }
.business-setup-submit:disabled { color: #64748b; background: #334155; cursor: not-allowed; }
.business-setup-footnote { margin: 18px 0 0; color: #94a3b8; font-size: 12px; text-align: center; }
.business-setup-loading { margin: 28px 0; color: #cbd5e1; font-size: 14px; text-align: center; }
.business-setup-retry { text-align: center; }
.business-setup-retry .business-setup-submit { width: 100%; }
@media (max-width: 520px) {
  .business-setup-backdrop { align-items: start; padding: 14px; }
  .business-setup-dialog { margin: auto 0; padding: 24px 20px; border-radius: 12px; }
  .business-setup-dialog h2 { font-size: 22px; }
}
@keyframes spin { to { transform: rotate(360deg); } }
</style>
