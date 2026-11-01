<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import {
  Award,
  Calendar,
  ChevronDown,
  Clock,
  Download,
  FileSpreadsheet,
  Printer,
  ShoppingBag,
  X,
} from '@lucide/vue'
import { apiRequest, formatAxisValue, formatDate, formatMoney, formatTime } from '../api'
import { downloadCsv, escapeHtml, openPrintDocument } from '../utils/reporting'
import { externalChartTooltipHandler } from '../utils/chartTooltip'
import { Bar } from 'vue-chartjs'
import {
  Chart as ChartJS,
  Title,
  Tooltip,
  Legend,
  BarElement,
  CategoryScale,
  LinearScale,
} from 'chart.js'

ChartJS.register(Title, Tooltip, Legend, BarElement, CategoryScale, LinearScale)

type Period = 'Daily' | 'Weekly' | 'Monthly'

type ProfitableProduct = {
  name: string
  profit: number
}

type BasketPair = {
  pair: [string, string]
  frequency: number
}

type BusinessIntelligence = {
  period: Period
  peakSalesHours: number[]
  mostProfitableProducts: ProfitableProduct[]
  marketBasket: BasketPair[]
}

const props = defineProps<{ currency: string; shopName: string }>()
const periods: Period[] = ['Daily', 'Weekly', 'Monthly']
const hourLabels = [
  '6-8am',
  '8-10am',
  '10-12pm',
  '12-2pm',
  '2-4pm',
  '4-6pm',
  '6-8pm',
  '8-10pm',
  '10-12am',
  '12-2am',
  '2-4am',
  '4-6am',
]
const selectedPeriod = ref<Period>('Daily')
const selectedDate = ref('')
const insights = ref<BusinessIntelligence | null>(null)
const loading = ref(true)
const error = ref('')
const isExportDropdownOpen = ref(false)

const hourBars = computed(() => {
  const counts = insights.value?.peakSalesHours ?? Array.from({ length: 12 }, () => 0)
  return hourLabels.map((label, index) => ({
    label,
    count: counts[index] ?? 0,
  }))
})

const maxHourSales = computed(() => Math.max(...hourBars.value.map(item => item.count), 0))
const hourScale = computed(() => {
  const max = maxHourSales.value
  if (max <= 4) return 4
  return Math.ceil(max / 4) * 4
})
const hourTicks = computed(() => [
  hourScale.value,
  hourScale.value * 0.75,
  hourScale.value * 0.5,
  hourScale.value * 0.25,
  0,
])

const profitableProducts = computed(() => insights.value?.mostProfitableProducts ?? [])
const maxProductProfit = computed(() =>
  Math.max(...profitableProducts.value.map(item => Math.max(item.profit, 0)), 0),
)
const profitScale = computed(() => {
  const max = maxProductProfit.value
  if (max <= 0) return 100
  const magnitude = 10 ** Math.floor(Math.log10(max))
  const normalized = max / magnitude
  if (normalized <= 1) return 1 * magnitude
  if (normalized <= 2) return 2 * magnitude
  if (normalized <= 5) return 5 * magnitude
  return 10 * magnitude
})
const profitTicks = computed(() => [
  profitScale.value,
  profitScale.value * 0.75,
  profitScale.value * 0.5,
  profitScale.value * 0.25,
  0,
])

const basketPairs = computed(() => insights.value?.marketBasket ?? [])

const isLightMode = computed(() => document.documentElement.getAttribute('data-theme') === 'light')
const currentAccentColor = computed(() => getComputedStyle(document.documentElement).getPropertyValue('--accent-color').trim() || '#0ea5e9')

const salesByHourData = computed(() => ({
  labels: hourLabels,
  datasets: [
    {
      label: 'Sales Count',
      backgroundColor: currentAccentColor.value,
      borderColor: currentAccentColor.value,
      borderWidth: 1,
      borderRadius: 4,
      data: hourBars.value.map(item => item.count),
    },
  ],
}))

const mostProfitableProductsData = computed(() => {
  const data = profitableProducts.value.map(p => p.profit)
  const labels = profitableProducts.value.map(p => p.name)
  return {
    labels,
    datasets: [
      {
        label: 'Total Profit',
        backgroundColor: currentAccentColor.value + 'B3',
        borderColor: currentAccentColor.value,
        borderWidth: 1,
        borderRadius: 4,
        data,
      },
    ],
  }
})

const salesCountChartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      enabled: false,
      external: externalChartTooltipHandler,
      callbacks: {
        label: (context: any) => `Sales: ${context.parsed?.y ?? 0}`,
      },
    },
  },
  scales: {
    x: {
      grid: { color: isLightMode.value ? 'rgba(0, 0, 0, 0.08)' : 'rgba(255, 255, 255, 0.08)' },
      ticks: {
        color: isLightMode.value ? '#0f172a' : '#94a3b8',
        font: { size: 10 },
      },
    },
    y: {
      grid: { color: isLightMode.value ? 'rgba(0, 0, 0, 0.08)' : 'rgba(255, 255, 255, 0.08)' },
      ticks: {
        color: isLightMode.value ? '#0f172a' : '#94a3b8',
        font: { size: 11 },
        callback: (value: number | string) => {
          if (typeof value === 'number') {
            if (value % 1 !== 0) return ''
            if (value >= 1000000) return `${value / 1000000}M`
            if (value >= 1000) return `${value / 1000}k`
            return `${value}`
          }
          return value
        },
      },
    },
  },
}))

const profitBarChartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      enabled: false,
      external: externalChartTooltipHandler,
      callbacks: {
        label: (context: any) => `Profit: ${props.currency} ${(context.parsed?.y ?? 0).toLocaleString()}`,
      },
    },
  },
  scales: {
    x: {
      grid: { color: isLightMode.value ? 'rgba(0, 0, 0, 0.08)' : 'rgba(255, 255, 255, 0.08)' },
      ticks: {
        color: isLightMode.value ? '#0f172a' : '#94a3b8',
        font: { size: 10 },
      },
    },
    y: {
      grid: { color: isLightMode.value ? 'rgba(0, 0, 0, 0.08)' : 'rgba(255, 255, 255, 0.08)' },
      ticks: {
        color: isLightMode.value ? '#0f172a' : '#94a3b8',
        font: { size: 11 },
        callback: (value: number | string) => {
          if (typeof value === 'number') {
            if (value >= 1000000) return `${props.currency} ${value / 1000000}M`
            if (value >= 1000) return `${props.currency} ${value / 1000}k`
            return `${props.currency} ${value}`
          }
          return value
        },
      },
    },
  },
}))

function toggleExportDropdown() {
  isExportDropdownOpen.value = !isExportDropdownOpen.value
}

function closeExportDropdown(event: MouseEvent) {
  const target = event.target as HTMLElement
  if (!target.closest('.export-dropdown-container')) {
    isExportDropdownOpen.value = false
  }
}

function exportCsv() {
  if (!insights.value) return
  isExportDropdownOpen.value = false
  const rows: Array<Array<string | number>> = [
    ['Peak Sales Window', 'Number of Sales', 'Period'],
    ...hourBars.value.map(item => [item.label, item.count, selectedPeriod.value]),
    [],
    ['Most Profitable Product', `Total Profit (${props.currency})`, 'Period'],
    ...profitableProducts.value.map(item => [item.name, item.profit, selectedPeriod.value]),
    [],
    ['Product Pair', 'Purchase Frequency (Times)', ''],
    ...basketPairs.value.map(item => [`${item.pair[0]} + ${item.pair[1]}`, item.frequency, '']),
  ]
  downloadCsv(`business_intelligence_${selectedPeriod.value.toLowerCase()}_${selectedDate.value || 'current'}.csv`, ['Metric', 'Value', 'Period'], rows)
}

function printReport() {
  if (!insights.value) return
  isExportDropdownOpen.value = false
  const hourRows = hourBars.value.map(item => `
    <tr><td>${escapeHtml(item.label)}</td><td class="num">${item.count}</td></tr>
  `).join('')
  const productRows = profitableProducts.value.map(item => `
    <tr><td>${escapeHtml(item.name)}</td><td class="num">${escapeHtml(formatMoney(item.profit, props.currency))}</td></tr>
  `).join('')
  const basketRows = basketPairs.value.map(item => `
    <tr><td>${escapeHtml(item.pair[0])} + ${escapeHtml(item.pair[1])}</td><td class="num">${item.frequency} times</td></tr>
  `).join('')

  openPrintDocument(
    `${props.shopName} Business Intelligence Report`,
    `<div class="meta">
      <div>
        <h1>Business Intelligence Report</h1>
        <p>${escapeHtml(props.shopName)} · ${escapeHtml(selectedPeriod.value)}${selectedDate.value ? ` (${escapeHtml(selectedDate.value)})` : ''}</p>
      </div>
      <div>
        <p>${escapeHtml(formatDate(new Date()))} ${escapeHtml(formatTime(new Date()))}</p>
      </div>
    </div>
    <h2>Peak Sales Hours</h2>
    <table><thead><tr><th>Time Window</th><th class="num">Sales Count</th></tr></thead><tbody>${hourRows}</tbody></table>
    <h2 style="margin-top:24px;">Most Profitable Products</h2>
    <table><thead><tr><th>Product</th><th class="num">Profit</th></tr></thead><tbody>${productRows || '<tr><td colspan="2">No profitable product sales in this period.</td></tr>'}</tbody></table>
    <h2 style="margin-top:24px;">Frequently Bought Together</h2>
    <table><thead><tr><th>Product Pair</th><th class="num">Frequency</th></tr></thead><tbody>${basketRows || '<tr><td colspan="2">No multi-item basket pairs in this period.</td></tr>'}</tbody></table>`,
  )
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const query = new URLSearchParams({ period: selectedPeriod.value })
    if (selectedDate.value) query.set('date', selectedDate.value)
    insights.value = await apiRequest<BusinessIntelligence>(`/business-intelligence?${query.toString()}`)
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Business intelligence could not be loaded.'
  } finally {
    loading.value = false
  }
}

watch([selectedPeriod, selectedDate], load)
onMounted(() => {
  load()
  window.addEventListener('click', closeExportDropdown)
})
onUnmounted(() => {
  window.removeEventListener('click', closeExportDropdown)
})
</script>

<template>
  <section class="bi-section">
    <div v-if="error" class="inline-error" role="alert">{{ error }}</div>

    <!-- Header Controls -->
    <div class="section-header">
      <div class="header-left">
        <h2 class="section-title">Business Intelligence Overview</h2>
      </div>
      <div class="export-dropdown-container">
        <button type="button" class="export-btn" @click.stop="toggleExportDropdown">
          <Download :size="16" />
          <span>Export Report</span>
          <ChevronDown :size="14" :class="{ rotate: isExportDropdownOpen }" class="chevron-icon" />
        </button>
        <div v-if="isExportDropdownOpen" class="export-dropdown-menu">
          <button type="button" class="dropdown-item" @click="exportCsv">
            <FileSpreadsheet :size="16" class="item-icon csv" />
            <div class="item-text">
              <span class="item-title">Export as CSV</span>
              <span class="item-desc">Spreadsheet compatible data</span>
            </div>
          </button>
          <button type="button" class="dropdown-item" @click="printReport">
            <Printer :size="16" class="item-icon pdf" />
            <div class="item-text">
              <span class="item-title">Print / Save PDF</span>
              <span class="item-desc">Formatted visual report</span>
            </div>
          </button>
        </div>
      </div>
    </div>

    <!-- Main Grid -->
    <div class="bi-grid">
      <!-- Peak Sales Hours -->
      <div class="card">
        <div class="card-header">
          <div class="header-left">
            <Clock :size="20" class="card-icon" style="color: #0ea5e9;" />
            <h3 class="card-title">Peak Sales Hours</h3>
          </div>
          <div class="bi-controls-right">
            <div class="time-range-selector">
              <button
                v-for="period in periods"
                :key="period"
                type="button"
                :class="{ active: selectedPeriod === period }"
                @click="selectedPeriod = period"
              >
                {{ period }}
              </button>
            </div>
            <div class="date-filter-wrapper">
              <label
                class="filter-btn date-picker-btn"
                :class="{ 'active-filter': selectedDate }"
                :title="selectedDate ? `Filtered by: ${selectedDate}` : 'Filter by Date'"
              >
                <Calendar :size="15" />
                <input v-model="selectedDate" type="date" class="hidden-date-input" aria-label="Filter peak sales hours by date" />
              </label>
              <button
                v-if="selectedDate"
                type="button"
                class="clear-date-btn"
                title="Clear Date Filter"
                @click="selectedDate = ''"
              >
                <X :size="12" />
              </button>
            </div>
          </div>
        </div>

        <div class="chart-container" style="position: relative; height: 100%; min-height: 240px;">
          <Bar :data="salesByHourData" :options="salesCountChartOptions" />
        </div>
      </div>

      <!-- Most Profitable Products -->
      <div class="card">
        <div class="card-header">
          <div class="header-left">
            <Award :size="20" class="card-icon" style="color: #22c55e;" />
            <h3 class="card-title">Most Profitable Products</h3>
          </div>
        </div>

        <div class="chart-container" style="position: relative; height: 100%; min-height: 240px;">
          <div v-if="!profitableProducts.length" class="empty-chart-state">
            No profitable product sales recorded for this period.
          </div>
          <Bar v-else :data="mostProfitableProductsData" :options="profitBarChartOptions" />
        </div>
      </div>

      <!-- Frequently Bought Together -->
      <div class="card full-width">
        <div class="card-header">
          <div class="header-left">
            <ShoppingBag :size="20" class="card-icon" style="color: #a855f7;" />
            <h3 class="card-title">Frequently Bought Together (Market Basket)</h3>
          </div>
        </div>
        <div class="table-container" style="flex: 1; min-height: 0; overflow-y: auto;">
          <table>
            <thead>
              <tr>
                <th class="text-center">Product Pair</th>
                <th class="text-center">Purchase Frequency</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!basketPairs.length">
                <td colspan="2" class="empty-message">Not enough multi-item sales data to determine pairs yet.</td>
              </tr>
              <tr v-for="(item, index) in basketPairs" :key="index" style="height: 38px;">
                <td class="text-center font-medium text-slate-200">
                  <span class="product-pair-badge">{{ item.pair[0] }}</span> + <span class="product-pair-badge">{{ item.pair[1] }}</span>
                </td>
                <td class="text-center font-semibold">{{ item.frequency }} times</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.bi-section {
  display: flex;
  flex-direction: column;
  flex: 1;
  height: 100%;
  min-height: 0;
  gap: 0.75rem;
}

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.75rem;
  flex-shrink: 0;
}

.section-title {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 600;
  color: var(--text-primary);
}

.export-dropdown-container {
  position: relative;
}

.export-btn {
  background-color: var(--accent-color);
  color: #ffffff;
  font-weight: 500;
  font-size: 0.85rem;
  padding: 0.55rem 1rem;
  border-radius: 0.5rem;
  border: none;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
}

.export-btn:hover {
  background-color: var(--accent-hover);
}

.chevron-icon {
  transition: transform 0.2s ease;
}

.chevron-icon.rotate {
  transform: rotate(180deg);
}

.export-dropdown-menu {
  position: absolute;
  top: calc(100% + 0.5rem);
  right: 0;
  width: 220px;
  background-color: var(--bg-sidebar);
  border: 1px solid var(--border-color);
  border-radius: 0.75rem;
  box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.5);
  padding: 0.4rem;
  z-index: 50;
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.dropdown-item {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.6rem 0.75rem;
  border-radius: 0.5rem;
  background: transparent;
  border: none;
  color: var(--text-primary);
  cursor: pointer;
  text-align: left;
  width: 100%;
}

.dropdown-item:hover {
  background-color: var(--bg-surface);
}

.item-icon.csv { color: #22c55e; }
.item-icon.pdf { color: #a855f7; }

.item-text {
  display: flex;
  flex-direction: column;
}

.item-title {
  font-size: 0.85rem;
  font-weight: 600;
  color: var(--text-primary);
}

.item-desc {
  font-size: 0.7rem;
  color: var(--text-muted);
}

.bi-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  grid-template-rows: minmax(220px, 1.15fr) minmax(170px, 0.85fr);
  gap: 0.75rem;
  flex: 1;
  min-height: 0;
}

.card {
  background: var(--glass-bg);
  backdrop-filter: var(--glass-blur);
  border: 1px solid var(--glass-border);
  border-radius: 14px;
  padding: 1.15rem 1.35rem;
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}

.full-width {
  grid-column: 1 / -1;
}

.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.85rem;
  flex-wrap: wrap;
  flex-shrink: 0;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 0.75rem;
}

.card-title {
  margin: 0;
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary);
}

.bi-controls-right {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.time-range-selector {
  display: flex;
  background-color: var(--bg-sidebar);
  border-radius: 0.4rem;
  padding: 0.15rem;
  border: 1px solid var(--border-color);
}

.time-range-selector button {
  background: transparent;
  border: none;
  color: var(--text-muted);
  padding: 0.25rem 0.65rem;
  border-radius: 0.25rem;
  font-weight: 500;
  font-size: 0.75rem;
  cursor: pointer;
}

.time-range-selector button.active {
  background-color: var(--bg-surface);
  color: var(--text-primary);
}

.date-filter-wrapper {
  display: flex;
  align-items: center;
  gap: 0.2rem;
}

.filter-btn {
  background-color: var(--bg-sidebar);
  color: var(--text-secondary);
  padding: 0.35rem;
  border-radius: 0.4rem;
  border: 1px solid var(--border-color);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  overflow: hidden;
}

.date-picker-btn.active-filter {
  border-color: var(--accent-color);
  color: #38bdf8;
  background-color: rgba(14, 165, 233, 0.1);
}

.hidden-date-input {
  position: absolute;
  inset: 0;
  opacity: 0;
  cursor: pointer;
  width: 100%;
  height: 100%;
}

.clear-date-btn {
  background-color: rgba(239, 68, 68, 0.1);
  color: #ef4444;
  border: 1px solid rgba(239, 68, 68, 0.2);
  border-radius: 0.4rem;
  padding: 0.35rem;
  cursor: pointer;
  display: flex;
}

.clear-date-btn:hover {
  background-color: #ef4444;
  color: #ffffff;
}

/* Vertical Bar Chart */
.chart-container {
  flex: 1;
  min-height: 160px;
  position: relative;
}

.empty-chart-state {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  font-size: 0.875rem;
}

.vbar-chart {
  height: 100%;
  display: grid;
  grid-template-columns: 42px minmax(0, 1fr);
  gap: 0.65rem;
}

.vbar-y-axis {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding-bottom: 26px;
  color: var(--text-muted);
  font-size: 0.72rem;
  text-align: right;
}

.vbar-canvas {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
}

.vbar-grid-lines {
  position: absolute;
  inset: 0 0 26px 0;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  pointer-events: none;
}

.vbar-grid-line {
  width: 100%;
  border-bottom: 1px solid rgba(148, 163, 184, 0.12);
}

.vbar-columns {
  position: relative;
  z-index: 1;
  flex-grow: 1;
  display: grid;
  grid-auto-flow: column;
  grid-auto-columns: minmax(0, 1fr);
  gap: 0.45rem;
  align-items: stretch;
}

.vbar-col {
  display: flex;
  flex-direction: column;
  align-items: center;
  height: 100%;
  min-width: 0;
}

.vbar-track {
  flex-grow: 1;
  width: 100%;
  max-width: 34px;
  display: flex;
  align-items: flex-end;
  justify-content: center;
}

.vbar-fill {
  width: 100%;
  border-radius: 4px 4px 0 0;
  transition: height 0.25s ease;
}

.vbar-fill.sky {
  background-color: #0ea5e9;
}

.vbar-fill.green {
  background-color: #22c55e;
}

.vbar-label {
  height: 26px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 0.68rem;
  color: var(--text-muted);
  text-align: center;
  width: 100%;
}

.truncate-label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: block;
  line-height: 26px;
}

/* Table */
.table-container {
  overflow-x: auto;
}

table {
  width: 100%;
  min-width: 520px;
  border-collapse: separate;
  border-spacing: 0;
  color: var(--text-secondary);
  background-color: var(--bg-sidebar);
  border-top: 1px solid var(--border-subtle);
  border-left: 1px solid var(--border-subtle);
}

th,
td {
  padding: 0.45rem 0.75rem;
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

.font-medium { font-weight: 500; }
.font-semibold { font-weight: 600; }
.text-slate-200 { color: var(--text-primary); }
.text-center { text-align: center; }

.product-pair-badge {
  background-color: var(--bg-surface);
  padding: 0.25rem 0.6rem;
  border-radius: 0.375rem;
  font-size: 0.825rem;
  border: 1px solid var(--border-color);
  display: inline-block;
}

.empty-message {
  text-align: center;
  padding: 1.5rem;
  color: var(--text-muted);
}

@media (max-width: 1024px) {
  .bi-grid {
    grid-template-columns: 1fr;
  }
}
</style>
