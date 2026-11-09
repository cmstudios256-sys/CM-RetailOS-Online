<script setup lang="ts">
import { computed, ref } from 'vue'
import {
  AlertCircle,
  Check,
  Cloud,
  Download,
  Loader2,
  Lock,
  Mail,
  Store,
  User as UserIcon,
  X,
} from '@lucide/vue'
import { apiRequest, setApiMode, type User } from '../api'
import { clearGuestDataAfterClaim, exportGuestBundle } from '../guestStore'

const props = defineProps<{
  open: boolean
  shopName: string
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved', user: User): void
}>()

const activeTab = ref<'account' | 'backup'>('account')
const formShopName = ref(props.shopName || 'My Shop')
const formFullName = ref('')
const formEmail = ref('')
const formPassword = ref('')
const isSubmitting = ref(false)
const errorMessage = ref('')
const successMessage = ref('')
const backupDownloaded = ref(false)

const canSubmit = computed(() => {
  return (
    formEmail.value.trim().length > 3 &&
    formEmail.value.includes('@') &&
    formPassword.value.length >= 8 &&
    !isSubmitting.value
  )
})

async function handleCloudSave(): Promise<void> {
  if (!canSubmit.value) return
  isSubmitting.value = true
  errorMessage.value = ''
  successMessage.value = ''

  try {
    setApiMode('cloud')
    const bundle = exportGuestBundle()
    const payload = {
      email: formEmail.value.trim(),
      password: formPassword.value,
      fullName: formFullName.value.trim() || 'Shop Owner',
      shopName: formShopName.value.trim() || 'My Shop',
      bundle,
    }

    const response = await apiRequest<{ user: User; message: string }>(
      '/auth/cloud-save',
      {
        method: 'POST',
        body: payload,
      },
    )

    clearGuestDataAfterClaim()
    successMessage.value = 'Your store has been successfully saved to the cloud!'
    setTimeout(() => {
      emit('saved', response.user)
      emit('close')
    }, 1200)
  } catch (error) {
    errorMessage.value =
      error instanceof Error
        ? error.message
        : 'Could not save to the cloud. Check your connection and try again.'
  } finally {
    isSubmitting.value = false
  }
}

function handleDownloadBackup(): void {
  try {
    const bundle = exportGuestBundle()
    const jsonStr = JSON.stringify(bundle, null, 2)
    const blob = new Blob([jsonStr], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    const cleanShop = (props.shopName || 'cm-retailos')
      .toLowerCase()
      .replace(/[^a-z0-9]+/g, '-')
    const dateStr = new Date().toISOString().split('T')[0]
    link.href = url
    link.download = `${cleanShop}-backup-${dateStr}.json`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)
    backupDownloaded.value = true
    setTimeout(() => {
      backupDownloaded.value = false
    }, 4000)
  } catch (error) {
    errorMessage.value = 'Failed to generate offline backup.'
  }
}
</script>

<template>
  <div v-if="open" class="cloud-save-overlay" @click.self="emit('close')">
    <div class="cloud-save-modal" role="dialog" aria-modal="true" aria-labelledby="cloudModalTitle">
      <div class="modal-header">
        <div class="header-icon">
          <Cloud :size="24" />
        </div>
        <div class="header-text">
          <h2 id="cloudModalTitle">Save Store to Cloud</h2>
          <p>Protect your products, inventory, and sales with permanent cloud storage.</p>
        </div>
        <button type="button" class="close-btn" aria-label="Close modal" @click="emit('close')">
          <X :size="18" />
        </button>
      </div>

      <div class="tab-pills">
        <button
          type="button"
          class="tab-pill"
          :class="{ active: activeTab === 'account' }"
          @click="activeTab = 'account'"
        >
          <Cloud :size="15" />
          Cloud Account (Live Sync)
        </button>
        <button
          type="button"
          class="tab-pill"
          :class="{ active: activeTab === 'backup' }"
          @click="activeTab = 'backup'"
        >
          <Download :size="15" />
          Offline File Backup
        </button>
      </div>

      <!-- Tab 1: Cloud Account Save -->
      <div v-if="activeTab === 'account'" class="tab-content">
        <div v-if="successMessage" class="cloud-alert success">
          <Check :size="18" />
          <span>{{ successMessage }}</span>
        </div>

        <div v-if="errorMessage" class="cloud-alert error">
          <AlertCircle :size="18" />
          <span>{{ errorMessage }}</span>
        </div>

        <form @submit.prevent="handleCloudSave" class="cloud-form">
          <div class="form-row">
            <div class="form-group">
              <label for="cloudShopName">Store Name</label>
              <div class="input-wrap">
                <Store :size="16" class="input-icon" />
                <input
                  id="cloudShopName"
                  v-model="formShopName"
                  type="text"
                  placeholder="e.g. My Duuka"
                  required
                />
              </div>
            </div>

            <div class="form-group">
              <label for="cloudOwnerName">Owner Name</label>
              <div class="input-wrap">
                <UserIcon :size="16" class="input-icon" />
                <input
                  id="cloudOwnerName"
                  v-model="formFullName"
                  type="text"
                  placeholder="Your Name"
                />
              </div>
            </div>
          </div>

          <div class="form-group">
            <label for="cloudEmail">Email Address</label>
            <div class="input-wrap">
              <Mail :size="16" class="input-icon" />
              <input
                id="cloudEmail"
                v-model="formEmail"
                type="email"
                placeholder="owner@example.com"
                autocomplete="email"
                required
              />
            </div>
          </div>

          <div class="form-group">
            <label for="cloudPassword">Password (8+ characters)</label>
            <div class="input-wrap">
              <Lock :size="16" class="input-icon" />
              <input
                id="cloudPassword"
                v-model="formPassword"
                type="password"
                placeholder="Create or enter account password"
                autocomplete="current-password"
                minlength="8"
                required
              />
            </div>
            <span class="field-hint">
              If you already have an account with this email, enter its password to sync to it.
            </span>
          </div>

          <div class="modal-actions">
            <button
              type="button"
              class="btn-cancel"
              :disabled="isSubmitting"
              @click="emit('close')"
            >
              Cancel
            </button>
            <button
              type="submit"
              class="btn-submit"
              :disabled="!canSubmit"
            >
              <Loader2 v-if="isSubmitting" :size="16" class="spinner" />
              <Cloud v-else :size="16" />
              <span>{{ isSubmitting ? 'Saving to Cloud...' : 'Save Store to Cloud' }}</span>
            </button>
          </div>
        </form>
      </div>

      <!-- Tab 2: Offline File Backup -->
      <div v-else class="tab-content backup-pane">
        <div class="backup-info-card">
          <Download :size="28" class="backup-card-icon" />
          <h3>Instant Offline Backup</h3>
          <p>
            Download an offline backup file (<code>.json</code>) containing all your products,
            prices, stock levels, and completed sales. You can keep this file safe on your device or USB drive.
          </p>
          <button
            type="button"
            class="btn-download-backup"
            @click="handleDownloadBackup"
          >
            <Check v-if="backupDownloaded" :size="18" />
            <Download v-else :size="18" />
            <span>{{ backupDownloaded ? 'Downloaded Successfully!' : 'Download Backup File (.json)' }}</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.cloud-save-overlay {
  position: fixed;
  inset: 0;
  z-index: 9999;
  background: rgba(2, 6, 23, 0.78);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
}

.cloud-save-modal {
  width: 100%;
  max-width: 480px;
  background: var(--bg-surface, #0b1329);
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.12));
  border-radius: 16px;
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.45);
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

[data-theme='light'] .cloud-save-modal {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  box-shadow: 0 24px 64px rgba(15, 23, 42, 0.18);
}

.modal-header {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 18px 20px;
  border-bottom: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
  position: relative;
}

[data-theme='light'] .modal-header {
  border-bottom: 1px solid #f1f5f9;
}

.header-icon {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: rgba(2, 132, 199, 0.15);
  border: 1px solid rgba(56, 189, 248, 0.25);
  display: flex;
  align-items: center;
  justify-content: center;
  color: #38bdf8;
  flex-shrink: 0;
}

.header-text h2 {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 700;
  color: var(--text-primary, #f8fafc);
}

[data-theme='light'] .header-text h2 {
  color: #0f172a;
}

.header-text p {
  margin: 2px 0 0;
  font-size: 0.8rem;
  color: var(--text-muted, #94a3b8);
  line-height: 1.35;
}

.close-btn {
  margin-left: auto;
  background: transparent;
  border: none;
  color: var(--text-muted, #94a3b8);
  cursor: pointer;
  padding: 6px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background 0.15s;
}

.close-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary, #ffffff);
}

.tab-pills {
  display: flex;
  gap: 6px;
  padding: 12px 20px 0;
}

.tab-pill {
  flex: 1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 8px 12px;
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-muted, #94a3b8);
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid transparent;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.15s;
}

[data-theme='light'] .tab-pill {
  background: #f1f5f9;
  color: #64748b;
}

.tab-pill.active {
  color: #38bdf8;
  background: rgba(2, 132, 199, 0.12);
  border-color: rgba(56, 189, 248, 0.3);
}

[data-theme='light'] .tab-pill.active {
  color: #0284c7;
  background: #e0f2fe;
  border-color: #bae6fd;
}

.tab-content {
  padding: 18px 20px 20px;
}

.cloud-alert {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  border-radius: 8px;
  font-size: 0.825rem;
  margin-bottom: 14px;
}

.cloud-alert.error {
  background: rgba(239, 68, 68, 0.12);
  color: #f87171;
  border: 1px solid rgba(239, 68, 68, 0.3);
}

.cloud-alert.success {
  background: rgba(16, 185, 129, 0.12);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
}

.cloud-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.form-row {
  display: flex;
  gap: 12px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 5px;
  flex: 1;
}

.form-group label {
  font-size: 0.775rem;
  font-weight: 600;
  color: var(--text-muted, #cbd5e1);
  text-transform: uppercase;
  letter-spacing: 0.02em;
}

[data-theme='light'] .form-group label {
  color: #475569;
}

.input-wrap {
  position: relative;
  display: flex;
  align-items: center;
}

.input-icon {
  position: absolute;
  left: 10px;
  color: #94a3b8;
  pointer-events: none;
}

.input-wrap input {
  width: 100%;
  padding: 9px 12px 9px 34px;
  background: rgba(15, 23, 42, 0.6);
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.12));
  border-radius: 8px;
  color: var(--text-primary, #f8fafc);
  font-size: 0.875rem;
  outline: none;
  transition: border-color 0.15s;
}

[data-theme='light'] .input-wrap input {
  background: #f8fafc;
  border: 1px solid #cbd5e1;
  color: #0f172a;
}

.input-wrap input:focus {
  border-color: #38bdf8;
  box-shadow: 0 0 0 2px rgba(56, 189, 248, 0.2);
}

.field-hint {
  font-size: 0.725rem;
  color: var(--text-muted, #94a3b8);
  line-height: 1.3;
}

.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 10px;
}

.btn-cancel {
  padding: 9px 16px;
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--text-muted, #94a3b8);
  background: transparent;
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.12));
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.15s;
}

[data-theme='light'] .btn-cancel {
  border-color: #cbd5e1;
  color: #475569;
}

.btn-cancel:hover {
  background: rgba(255, 255, 255, 0.05);
  color: #ffffff;
}

.btn-submit {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 9px 18px;
  font-size: 0.85rem;
  font-weight: 700;
  color: #ffffff;
  background: linear-gradient(135deg, #0284c7 0%, #0369a1 100%);
  border: none;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.15s;
  box-shadow: 0 2px 8px rgba(2, 132, 199, 0.35);
}

.btn-submit:hover:not(:disabled) {
  background: linear-gradient(135deg, #0369a1 0%, #075985 100%);
  transform: translateY(-1px);
}

.btn-submit:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  box-shadow: none;
}

.spinner {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.backup-pane {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
}

.backup-info-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 16px 12px;
}

.backup-card-icon {
  color: #38bdf8;
}

.backup-info-card h3 {
  margin: 0;
  font-size: 1rem;
  font-weight: 700;
  color: var(--text-primary, #f8fafc);
}

[data-theme='light'] .backup-info-card h3 {
  color: #0f172a;
}

.backup-info-card p {
  margin: 0;
  font-size: 0.825rem;
  color: var(--text-muted, #94a3b8);
  line-height: 1.45;
  max-width: 380px;
}

.backup-info-card code {
  background: rgba(255, 255, 255, 0.08);
  padding: 2px 5px;
  border-radius: 4px;
  color: #38bdf8;
  font-size: 0.8rem;
}

[data-theme='light'] .backup-info-card code {
  background: #e2e8f0;
  color: #0284c7;
}

.btn-download-backup {
  margin-top: 10px;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 10px 20px;
  font-size: 0.85rem;
  font-weight: 700;
  color: #ffffff;
  background: #10b981;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.15s;
  box-shadow: 0 2px 8px rgba(16, 185, 129, 0.3);
}

.btn-download-backup:hover {
  background: #059669;
  transform: translateY(-1px);
}
</style>
