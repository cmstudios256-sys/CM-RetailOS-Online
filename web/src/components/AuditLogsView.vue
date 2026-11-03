<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'
import { RefreshCw, Search } from '@lucide/vue'
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

const auditListRef = ref<HTMLElement | null>(null)
const auditListVirtualizer = useVirtualizer(computed(() => ({
  count: filteredLogs.value.length,
  getScrollElement: () => auditListRef.value,
  estimateSize: () => 38,
  overscan: 8,
})))
const virtualAuditListItems = computed(() => auditListVirtualizer.value.getVirtualItems())

watch(
  () => [searchTerm.value, filteredLogs.value.length],
  () => {
    auditListRef.value?.scrollTo({ top: 0 })
  },
)

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

      <div ref="auditListRef" class="table-container virtual-list-container" style="flex: 1; min-height: 0; overflow-y: auto;">
        <div class="virtual-list-header audit-list-header">
          <span>Timestamp</span>
          <span>User</span>
          <span>Action</span>
          <span>Details</span>
          <span>Level</span>
        </div>
        <div v-if="loading" class="empty-message">Loading audit logs...</div>
        <div v-else-if="filteredLogs.length === 0" class="empty-message">No audit logs found.</div>
        <div
          v-else
          class="virtual-list-spacer"
          :style="{ height: `${auditListVirtualizer.getTotalSize()}px` }"
        >
          <div
            v-for="virtualRow in virtualAuditListItems"
            :key="String(virtualRow.key)"
            class="virtual-list-row audit-list-header"
            :style="{ transform: `translateY(${virtualRow.start}px)`, height: `${virtualRow.size}px` }"
          >
            <template v-if="filteredLogs[virtualRow.index]">
              <span class="font-medium text-slate-200">{{ formatDate(filteredLogs[virtualRow.index].timestamp) }} {{ formatTime(filteredLogs[virtualRow.index].timestamp) }}</span>
              <span>{{ filteredLogs[virtualRow.index].user }}</span>
              <span>{{ filteredLogs[virtualRow.index].action }}</span>
              <span class="details-cell" :title="filteredLogs[virtualRow.index].details">{{ filteredLogs[virtualRow.index].details }}</span>
              <span>
                <span class="log-level-badge" :class="getLogLevelClass(filteredLogs[virtualRow.index].level)">
                  {{ filteredLogs[virtualRow.index].level }}
                </span>
              </span>
            </template>
          </div>
        </div>
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

.virtual-list-container {
  position: relative;
  background-color: var(--bg-sidebar);
  border-top: 1px solid var(--border-subtle);
  border-left: 1px solid var(--border-subtle);
}

.virtual-list-spacer {
  position: relative;
  width: 100%;
}

.virtual-list-header,
.virtual-list-row {
  display: grid;
  align-items: center;
}

.audit-list-header {
  grid-template-columns: minmax(150px, 1.3fr) minmax(110px, 1fr) minmax(140px, 1.2fr) minmax(260px, 2.5fr) minmax(90px, 0.8fr);
  min-width: 750px;
}

.virtual-list-header {
  position: sticky;
  top: 0;
  z-index: 20;
  min-height: 38px;
  background-color: var(--bg-surface);
  color: var(--text-muted);
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  border-bottom: 1px solid var(--border-subtle);
}

.virtual-list-header > span,
.virtual-list-row > span {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0.4rem 0.75rem;
  border-right: 1px solid var(--border-subtle);
  border-bottom: 1px solid var(--border-subtle);
  text-align: center;
  font-size: 0.825rem;
  white-space: nowrap;
  box-sizing: border-box;
  height: 100%;
}

.virtual-list-row {
  position: absolute;
  left: 0;
  right: 0;
  top: 0;
}

.virtual-list-row:hover {
  background-color: var(--bg-surface);
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
