<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RefreshCw, Search, ShieldCheck } from '@lucide/vue'
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
      return ''
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const result = await apiRequest<{ logs: AuditLog[] }>('/audit-logs')
    logs.value = result.logs
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Audit activity could not be loaded.'
  } finally {
    loading.value = false
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
          <span class="scope-note">
            <ShieldCheck :size="14" />
            Read-only history of completed sales &amp; stock adjustments (up to 500 records)
          </span>
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
          <button type="button" class="refresh-btn" :disabled="loading" @click="load">
            <RefreshCw :size="15" />
            Refresh
          </button>
        </div>
      </div>

      <div class="table-container" style="flex: 1; min-height: 0; overflow-y: auto;">
        <table>
          <thead>
            <tr>
              <th class="text-center">Timestamp</th>
              <th class="text-center">User</th>
              <th class="text-center">Action</th>
              <th class="text-center">Details</th>
              <th class="text-center">Level</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading">
              <td colspan="5" class="empty-message">Loading audit logs...</td>
            </tr>
            <tr v-else-if="filteredLogs.length === 0">
              <td colspan="5" class="empty-message">No audit logs found.</td>
            </tr>
            <tr v-for="log in filteredLogs" v-else :key="log.id" style="height: 38px;">
              <td class="text-center font-medium text-slate-200">{{ formatDate(log.timestamp) }} {{ formatTime(log.timestamp) }}</td>
              <td class="text-center">{{ log.user }}</td>
              <td class="text-center">{{ log.action }}</td>
              <td class="text-center details-cell" :title="log.details">{{ log.details }}</td>
              <td class="text-center">
                <span class="log-level-badge" :class="getLogLevelClass(log.level)">
                  {{ log.level }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </section>
</template>

<style scoped>
.view-section {
  display: flex;
  flex-direction: column;
  flex-grow: 1;
  min-height: 0;
}

.card {
  background: var(--glass-bg);
  backdrop-filter: var(--glass-blur);
  border: 1px solid var(--glass-border);
  border-radius: 16px;
  padding: 1.5rem;
  color: var(--text-secondary);
  display: flex;
  flex-direction: column;
  flex-grow: 1;
  min-width: 0;
  min-height: 0;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.75rem;
  margin-bottom: 0.75rem;
  padding-bottom: 0.75rem;
  border-bottom: 1px solid var(--border-subtle);
}

.title-group {
  display: flex;
  align-items: center;
  gap: 0.85rem;
  flex-wrap: wrap;
}

.card-title {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 600;
  color: var(--text-primary);
}

.scope-note {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.75rem;
  color: var(--text-muted);
  background: rgba(14, 165, 233, 0.08);
  border: 1px solid rgba(56, 189, 248, 0.2);
  padding: 0.2rem 0.6rem;
  border-radius: 9999px;
}

.header-controls {
  display: flex;
  gap: 0.75rem;
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
  border-radius: 0.375rem;
  padding: 0.4rem 0.75rem 0.4rem 2.25rem;
  color: var(--text-primary);
  width: min(250px, 35vw);
  font-size: 0.825rem;
}

.search-bar input:focus {
  outline: none;
  border-color: var(--accent-color);
}

.refresh-btn {
  background-color: var(--bg-surface);
  color: var(--text-secondary);
  padding: 0.4rem 0.85rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border-color);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-weight: 500;
  font-size: 0.825rem;
}

.refresh-btn:hover:not(:disabled) {
  background-color: var(--bg-hover);
  color: var(--text-primary);
}

.table-container {
  flex-grow: 1;
  min-height: 0;
  overflow-x: auto;
  overflow-y: auto;
}

table {
  width: 100%;
  min-width: 760px;
  border-collapse: separate;
  border-spacing: 0;
  color: var(--text-secondary);
  background-color: var(--bg-sidebar);
  border-top: 1px solid var(--border-subtle);
  border-left: 1px solid var(--border-subtle);
}

th,
td {
  padding: 0.4rem 0.75rem;
  text-align: center;
  border-bottom: 1px solid var(--border-subtle);
  border-right: 1px solid var(--border-subtle);
  white-space: nowrap;
  vertical-align: middle;
  font-size: 0.825rem;
}

th {
  position: sticky;
  top: 0;
  background-color: var(--bg-surface);
  z-index: 10;
  color: var(--text-muted);
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

tbody tr:hover {
  background-color: var(--bg-surface);
}

th:nth-child(1), td:nth-child(1) { width: 18%; }
th:nth-child(2), td:nth-child(2) { width: 14%; }
th:nth-child(3), td:nth-child(3) { width: 16%; }
th:nth-child(4), td:nth-child(4) { width: 42%; }
th:nth-child(5), td:nth-child(5) { width: 10%; }

.details-cell {
  max-width: 380px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.font-medium { font-weight: 500; }
.text-slate-200 { color: var(--text-primary); }
.text-center { text-align: center; }

.log-level-badge {
  padding: 0.2rem 0.5rem;
  border-radius: 9999px;
  font-size: 0.7rem;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
  border: 1px solid;
}

.log-info {
  border-color: var(--accent-hover);
  color: #7dd3fc;
  background-color: rgba(56, 189, 248, 0.1);
}

.log-warn {
  border-color: #facc15;
  color: #fde047;
  background-color: rgba(250, 204, 21, 0.1);
}

.log-error {
  border-color: #ef4444;
  color: #fca5a5;
  background-color: rgba(239, 68, 68, 0.1);
}

.log-audit {
  border-color: #a78bfa;
  color: #d8b4fe;
  background-color: rgba(167, 139, 250, 0.1);
}

.empty-message {
  text-align: center;
  padding: 1.5rem;
  color: var(--text-muted);
}
</style>
