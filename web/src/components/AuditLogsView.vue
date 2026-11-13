<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { AlertTriangle, Eraser, RefreshCw, Search } from '@lucide/vue'
import { apiRequest, formatDate, formatTime } from '../api'

type AuditLog = {
  id: string
  timestamp: string
  user: string
  action: string
  details: string
  level: 'INFO' | 'WARN' | 'ERROR' | 'AUDIT'
}

const logs = ref<AuditLog[]>([])
const searchTerm = ref('')
const loading = ref(true)
const error = ref('')
const isClearModalOpen = ref(false)
const clearing = ref(false)

const filteredLogs = computed(() => {
  const query = searchTerm.value.trim().toLowerCase()
  if (!query) return logs.value
  return logs.value.filter(log =>
    log.user.toLowerCase().includes(query)
    || log.action.toLowerCase().includes(query)
    || log.details.toLowerCase().includes(query)
    || log.level.toLowerCase().includes(query),
  )
})

function getLogLevelClass(level: string) {
  switch (level) {
    case 'INFO':
      return 'log-info'
    case 'WARN':
      return 'log-warn'
    case 'ERROR':
      return 'log-error'
    case 'AUDIT':
      return 'log-audit'
    default:
      return 'log-audit'
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const result = await apiRequest<{ logs: AuditLog[] }>('/audit-logs')
    logs.value = result.logs || []
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Audit activity could not be loaded.'
  } finally {
    loading.value = false
  }
}

function openClearModal() {
  isClearModalOpen.value = true
}

async function confirmClearLogs() {
  clearing.value = true
  try {
    await apiRequest('/audit-logs', { method: 'DELETE' })
    logs.value = []
  } catch {
    logs.value = []
  } finally {
    clearing.value = false
    isClearModalOpen.value = false
  }
}

onMounted(load)
</script>

<template>
  <section class="view-section">
    <div v-if="error" class="inline-error" role="alert">{{ error }}</div>

    <div class="card">
      <div class="card-header">
        <div class="title-group">
          <h2 class="card-title">Audit Logs</h2>
          <span v-if="logs.length" class="log-count-pill">{{ logs.length }} Events</span>
        </div>
        <div class="header-controls">
          <div class="search-bar">
            <Search class="search-icon" :size="18" />
            <input
              v-model="searchTerm"
              type="text"
              placeholder="Search by user, action, details..."
              aria-label="Search audit logs"
            />
          </div>
          <button type="button" class="refresh-btn" :disabled="loading" title="Refresh logs" @click="load">
            <RefreshCw :size="15" :class="{ 'spin-icon': loading }" />
            <span>Refresh</span>
          </button>
          <button type="button" class="clear-btn" :disabled="logs.length === 0" title="Clear logs" @click="openClearModal">
            <Eraser :size="15" />
            <span>Clear Logs</span>
          </button>
        </div>
      </div>

      <div class="table-container">
        <table>
          <thead>
            <tr>
              <th class="col-timestamp">Timestamp</th>
              <th class="col-user">User</th>
              <th class="col-action">Action</th>
              <th class="col-details">Details</th>
              <th class="col-level">Level</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading">
              <td colspan="5" class="empty-message">Loading audit logs...</td>
            </tr>
            <tr v-else-if="filteredLogs.length === 0">
              <td colspan="5" class="empty-message">No audit logs found.</td>
            </tr>
            <tr v-for="log in filteredLogs" :key="log.id">
              <td class="col-timestamp font-medium text-slate-200">
                {{ formatDate(log.timestamp) }} {{ formatTime(log.timestamp) }}
              </td>
              <td class="col-user">{{ log.user }}</td>
              <td class="col-action font-medium">{{ log.action }}</td>
              <td class="col-details details-cell" :title="log.details">{{ log.details }}</td>
              <td class="col-level">
                <span class="log-level-badge" :class="getLogLevelClass(log.level)">
                  {{ log.level }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Clear Confirmation Modal -->
    <transition name="modal-fade">
      <div v-if="isClearModalOpen" class="modal-overlay">
        <div class="modal-card" v-draggable="'.modal-card'">
          <div class="warning-icon-box">
            <AlertTriangle :size="32" />
          </div>
          <h3 class="modal-title">Clear All Logs?</h3>
          <p class="modal-description">
            Are you sure you want to clear all audit logs? This action cannot be undone and will permanently delete the log history.
          </p>
          <div class="modal-actions">
            <button type="button" class="modal-cancel-btn" :disabled="clearing" @click="isClearModalOpen = false">
              Cancel
            </button>
            <button type="button" class="modal-confirm-btn" :disabled="clearing" @click="confirmClearLogs">
              {{ clearing ? 'Clearing...' : 'Yes, Clear Logs' }}
            </button>
          </div>
        </div>
      </div>
    </transition>
  </section>
</template>

<style scoped>
.view-section {
  display: flex;
  flex-direction: column;
  flex-grow: 1;
  min-height: 0;
  width: 100%;
}

.inline-error {
  background: rgba(239, 68, 68, 0.15);
  border: 1px solid rgba(239, 68, 68, 0.3);
  color: #fca5a5;
  padding: 0.75rem 1rem;
  border-radius: 8px;
  margin-bottom: 1rem;
  font-size: 0.875rem;
}

.card {
  background-color: var(--bg-sidebar);
  border: 1px solid var(--border-subtle);
  border-radius: 14px;
  padding: 1.25rem 1.5rem;
  color: var(--text-secondary);
  display: flex;
  flex-direction: column;
  flex-grow: 1;
  min-height: 0;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.25);
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.75rem;
  margin-bottom: 1rem;
  padding-bottom: 0.85rem;
  border-bottom: 1px solid var(--border-subtle);
}

.title-group {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.card-title {
  margin: 0;
  font-size: 1.2rem;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: -0.01em;
}

.log-count-pill {
  font-size: 0.72rem;
  font-weight: 600;
  padding: 0.15rem 0.5rem;
  border-radius: 9999px;
  background: rgba(148, 163, 184, 0.12);
  color: var(--text-muted);
}

.header-controls {
  display: flex;
  gap: 0.65rem;
  align-items: center;
  flex-wrap: wrap;
}

.search-bar {
  position: relative;
}

.search-icon {
  position: absolute;
  left: 0.75rem;
  top: 50%;
  transform: translateY(-50%);
  color: var(--text-muted);
  pointer-events: none;
}

.search-bar input {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 0.45rem 0.75rem 0.45rem 2.25rem;
  color: var(--text-primary);
  width: min(280px, 35vw);
  font-size: 0.825rem;
  transition: border-color 0.2s, background-color 0.2s;
}

.search-bar input:focus {
  outline: none;
  border-color: var(--accent-color);
}

.refresh-btn {
  background-color: var(--bg-surface);
  color: var(--text-secondary);
  padding: 0.45rem 0.85rem;
  border-radius: 8px;
  border: 1px solid var(--border-color);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-weight: 500;
  font-size: 0.825rem;
  transition: all 0.2s;
}

.refresh-btn:hover:not(:disabled) {
  background-color: var(--bg-hover);
  color: var(--text-primary);
}

.clear-btn {
  background-color: rgba(239, 68, 68, 0.1);
  color: #ef4444;
  padding: 0.45rem 0.85rem;
  border-radius: 8px;
  border: 1px solid rgba(239, 68, 68, 0.25);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-weight: 500;
  font-size: 0.825rem;
  transition: all 0.2s;
}

.clear-btn:hover:not(:disabled) {
  background-color: #ef4444;
  color: #ffffff;
  border-color: #ef4444;
}

.clear-btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.table-container {
  flex-grow: 1;
  min-height: 0;
  overflow-x: auto;
  overflow-y: auto;
  min-width: 0;
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  background-color: var(--bg-sidebar);
}

table {
  width: 100%;
  min-width: 760px;
  border-collapse: collapse;
  color: var(--text-secondary);
}

th,
td {
  padding: 0.6rem 0.85rem;
  border-bottom: 1px solid var(--border-subtle);
  border-right: 1px solid var(--border-subtle);
  white-space: nowrap;
  vertical-align: middle;
  font-size: 0.825rem;
  text-align: center;
}

th:last-child,
td:last-child {
  border-right: none;
}

tbody tr:last-child td {
  border-bottom: none;
}

th {
  position: sticky;
  top: 0;
  background-color: var(--bg-surface);
  z-index: 10;
  color: var(--text-muted);
  font-size: 0.75rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

tbody tr:hover {
  background-color: var(--bg-surface);
}

.col-timestamp { width: 18%; }
.col-user { width: 14%; }
.col-action { width: 18%; }
.col-details { width: 40%; }
.col-level { width: 10%; }

.details-cell {
  max-width: 440px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  text-align: left !important;
  padding-left: 1rem !important;
}

.font-medium { font-weight: 500; }
.text-slate-200 { color: var(--text-primary); }

.log-level-badge {
  padding: 0.2rem 0.55rem;
  border-radius: 9999px;
  font-size: 0.7rem;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
  border: 1px solid;
  white-space: nowrap;
}

.log-info {
  border-color: #38bdf8;
  color: #7dd3fc;
  background-color: rgba(56, 189, 248, 0.12);
}

.log-warn {
  border-color: #facc15;
  color: #fde047;
  background-color: rgba(250, 204, 21, 0.12);
}

.log-error {
  border-color: #ef4444;
  color: #fca5a5;
  background-color: rgba(239, 68, 68, 0.12);
}

.log-audit {
  border-color: #a78bfa;
  color: #d8b4fe;
  background-color: rgba(167, 139, 250, 0.12);
}

.empty-message {
  text-align: center;
  padding: 2.5rem 1rem;
  color: var(--text-muted);
  font-size: 0.875rem;
}

/* Clear Confirmation Modal */
.modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
  background: transparent !important;
  backdrop-filter: none !important;
  -webkit-backdrop-filter: none !important;
}

.modal-card {
  width: 100%;
  max-width: 420px;
  background: var(--bg-sidebar, #0f172a);
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.1));
  border-radius: 16px;
  padding: 2rem 1.75rem;
  text-align: center;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.6);
  box-sizing: border-box;
}

.warning-icon-box {
  margin: 0 auto 1.25rem auto;
  width: 58px;
  height: 58px;
  background: rgba(239, 68, 68, 0.12);
  color: #ef4444;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.modal-title {
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--text-primary, #ffffff);
  margin: 0 0 0.5rem 0;
}

.modal-description {
  color: var(--text-muted, #94a3b8);
  font-size: 0.9rem;
  margin: 0 0 1.75rem 0;
  line-height: 1.5;
}

.modal-actions {
  display: flex;
  gap: 0.75rem;
  width: 100%;
}

.modal-cancel-btn {
  flex: 1;
  padding: 0.65rem 1rem;
  background: var(--bg-surface, #1e293b);
  color: var(--text-secondary, #cbd5e1);
  border: 1px solid var(--border-color, rgba(255, 255, 255, 0.1));
  border-radius: 8px;
  cursor: pointer;
  font-weight: 600;
  font-size: 0.875rem;
  transition: all 0.2s;
}

.modal-cancel-btn:hover:not(:disabled) {
  background: var(--bg-hover, #334155);
  color: var(--text-primary, #ffffff);
}

.modal-confirm-btn {
  flex: 1;
  padding: 0.65rem 1rem;
  background: #ef4444;
  color: #ffffff;
  border: none;
  border-radius: 8px;
  cursor: pointer;
  font-weight: 600;
  font-size: 0.875rem;
  box-shadow: 0 4px 12px rgba(239, 68, 68, 0.35);
  transition: all 0.2s;
}

.modal-confirm-btn:hover:not(:disabled) {
  background: #dc2626;
  box-shadow: 0 6px 16px rgba(239, 68, 68, 0.45);
}

.spin-icon {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

.modal-fade-enter-active,
.modal-fade-leave-active {
  transition: opacity 0.2s ease;
}

.modal-fade-enter-from,
.modal-fade-leave-to {
  opacity: 0;
}

/* Light Theme Overrides */
[data-theme='light'] .card {
  background-color: #ffffff !important;
  border: 1px solid #e2e8f0 !important;
  box-shadow: 0 4px 20px rgba(15, 23, 42, 0.06) !important;
}

[data-theme='light'] .card-title {
  color: #0f172a !important;
}

[data-theme='light'] .table-container {
  background-color: #ffffff !important;
  border: 1px solid #e2e8f0 !important;
}

[data-theme='light'] table {
  background-color: #ffffff !important;
}

[data-theme='light'] th {
  background-color: #f8fafc !important;
  color: #64748b !important;
  border-bottom: 1px solid #e2e8f0 !important;
  border-right: 1px solid #e2e8f0 !important;
}

[data-theme='light'] td {
  border-bottom: 1px solid #f1f5f9 !important;
  border-right: 1px solid #f1f5f9 !important;
  color: #334155 !important;
}

[data-theme='light'] tbody tr:hover {
  background-color: #f8fafc !important;
}

[data-theme='light'] .search-bar input {
  background-color: #f8fafc !important;
  border: 1px solid #cbd5e1 !important;
  color: #0f172a !important;
}

[data-theme='light'] .refresh-btn {
  background-color: #f8fafc !important;
  border: 1px solid #cbd5e1 !important;
  color: #475569 !important;
}

[data-theme='light'] .modal-card {
  background-color: #ffffff !important;
  border: 1px solid #e2e8f0 !important;
}

[data-theme='light'] .modal-title {
  color: #0f172a !important;
}

[data-theme='light'] .modal-cancel-btn {
  background-color: #f8fafc !important;
  border: 1px solid #cbd5e1 !important;
  color: #475569 !important;
}

@media (min-width: 1081px) {
  .card {
    padding: 0;
    overflow: hidden;
  }

  .card-header {
    padding: 14px 18px 12px;
    margin-bottom: 0;
  }

  .table-container {
    border: none;
    border-radius: 0;
  }
}
</style>
