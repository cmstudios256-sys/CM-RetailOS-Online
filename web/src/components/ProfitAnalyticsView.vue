<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import {
  ArrowDownRight,
  ArrowUpRight,
  BarChart2,
  Calendar,
  ChevronDown,
  DollarSign,
  Download,
  FileSpreadsheet,
  Percent,
  Printer,
  TrendingUp,
  X,
} from '@lucide/vue'
import { apiRequest, formatAxisValue, formatDate, formatMoney, formatTime } from '../api'
import { downloadCsv, escapeHtml, openPrintDocument } from '../utils/reporting'
import { externalChartTooltipHandler } from '../utils/chartTooltip'
import { Line, Doughnut } from 'vue-chartjs'
import {
  Chart as ChartJS,
  Title,
  Tooltip,
  Legend,
  LineElement,
  CategoryScale,
  LinearScale,
  PointElement,
  Filler,
  ArcElement,
} from 'chart.js'

ChartJS.register(
  Title,
  Tooltip,
  Legend,
  LineElement,
  CategoryScale,
  LinearScale,
  PointElement,
  Filler,
  ArcElement,
)

type Period = 'Daily' | 'Weekly' | 'Monthly'

type ChartPoint = {
  label: string
  sales: number
  profit: number
}

type ProfitCategory = {
  name: string
  value: number
}

type Dashboard = {
  currency: string
  revenue: number
  profit: number
  previousRevenue: number
  previousProfit: number
  orderCount: number
  unitsSold: number
  chart: ChartPoint[]
  profitByCategory: ProfitCategory[]
}

const props = defineProps<{ currency: string; shopName: string }>()
const periods: Period[] = ['Daily', 'Weekly', 'Monthly']
const selectedPeriod = ref<Period>('Daily')
const reportDate = ref('')
const dashboard = ref<Dashboard | null>(null)
const loading = ref(true)
const error = ref('')
const isExportDropdownOpen = ref(false)
const hoveredIndex = ref<number | null>(null)

const categoryColors = ['#0ea5e9', '#22c55e', '#f97316', '#a855f7', '#ec4899', '#eab308']

const activeCurrency = computed(() => dashboard.value?.currency || props.currency)

const margin = computed(() => {
  const revenue = dashboard.value?.revenue ?? 0
  const profit = dashboard.value?.profit ?? 0
  return revenue > 0 ? (profit / revenue) * 100 : 0
})

const previousMargin = computed(() => {
  const prevRev = dashboard.value?.previousRevenue ?? 0
  const prevProf = dashboard.value?.previousProfit ?? 0
  return prevRev > 0 ? (prevProf / prevRev) * 100 : 0
})

const periodComparisonText = computed(() => {
  if (selectedPeriod.value === 'Weekly') return 'vs last week'
  if (selectedPeriod.value === 'Monthly') return 'vs last month'
  return 'vs yesterday'
})

const profitTrend = computed(() =>
  calculateTrend(dashboard.value?.profit ?? 0, dashboard.value?.previousProfit ?? 0),
)
const revenueTrend = computed(() =>
  calculateTrend(dashboard.value?.revenue ?? 0, dashboard.value?.previousRevenue ?? 0),
)
const marginTrend = computed(() =>
  calculateTrend(margin.value, previousMargin.value),
)

const chartPoints = computed(() => dashboard.value?.chart ?? [])
const maxProfit = computed(() => Math.max(...chartPoints.value.map(point => Math.abs(point.profit || 0)), 0))
const chartScale = computed(() => {
  const max = maxProfit.value
  if (max <= 0) return 100
  const magnitude = 10 ** Math.floor(Math.log10(max))
  const normalized = max / magnitude
  if (normalized <= 1) return 1 * magnitude
  if (normalized <= 2) return 2 * magnitude
  if (normalized <= 5) return 5 * magnitude
  return 10 * magnitude
})

const yAxisTicks = computed(() => [
  chartScale.value,
  chartScale.value * 0.75,
  chartScale.value * 0.5,
  chartScale.value * 0.25,
  0,
])

const profitLinePoints = computed(() =>
  chartPoints.value.map((point, index) => {
    const total = Math.max(chartPoints.value.length - 1, 1)
    const x = (index / total) * 100
    const normalized = Math.min(Math.max((point.profit || 0) / chartScale.value, 0), 1)
    const y = 100 - normalized * 84 - 8
    return { x, y, ...point }
  }),
)

const profitLinePath = computed(() =>
  profitLinePoints.value
    .map((point, index) => `${index === 0 ? 'M' : 'L'} ${point.x.toFixed(2)} ${point.y.toFixed(2)}`)
    .join(' '),
)

const profitAreaPath = computed(() => {
  if (!profitLinePoints.value.length) return ''
  const first = profitLinePoints.value[0]
  const last = profitLinePoints.value[profitLinePoints.value.length - 1]
  return `${profitLinePath.value} L ${last.x.toFixed(2)} 96 L ${first.x.toFixed(2)} 96 Z`
})

const categories = computed(() => dashboard.value?.profitByCategory ?? [])
const totalCategoryProfit = computed(() =>
  categories.value.reduce((sum, category) => sum + Math.max(category.value, 0), 0),
)

const doughnutSegments = computed(() => {
  const positiveTotal = totalCategoryProfit.value
  if (positiveTotal <= 0) return []
  const radius = 36
  const circumference = 2 * Math.PI * radius
  let cumulativeFraction = 0
  return categories.value
    .filter(item => item.value > 0)
    .map((item, idx) => {
      const fraction = item.value / positiveTotal
      const dashLength = fraction * circumference
      const dashOffset = -cumulativeFraction * circumference
      cumulativeFraction += fraction
      return {
        name: item.name,
        value: item.value,
        color: categoryColors[idx % categoryColors.length],
        dashArray: `${dashLength} ${circumference - dashLength}`,
        dashOffset,
      }
    })
})

const isLightMode = computed(() => document.documentElement.getAttribute('data-theme') === 'light')
const currentAccentColor = computed(() => getComputedStyle(document.documentElement).getPropertyValue('--accent-color').trim() || '#0ea5e9')

const profitTrendData = computed(() => {
  const points = chartPoints.value
  const labels = points.map(p => p.label)
  const data = points.map(p => p.profit)

  return {
    labels,
    datasets: [{
      type: 'line' as const,
      label: 'Net Profit',
      borderColor: currentAccentColor.value,
      backgroundColor: currentAccentColor.value + '33',
      borderWidth: 2,
      fill: true,
      tension: 0.4,
      pointRadius: 3,
      pointHoverRadius: 6,
      data,
    }],
  }
})

const profitByCategoryData = computed(() => {
  const cats = categories.value
  return {
    labels: cats.map(c => c.name),
    datasets: [{
      backgroundColor: ['#0ea5e9', '#22c55e', '#f97316', '#a855f7', '#ec4899', '#eab308', '#06b6d4'],
      data: cats.map(c => c.value),
    }],
  }
})

const chartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      enabled: false,
      external: externalChartTooltipHandler,
      callbacks: {
        label: (context: any) => {
          const label = context.dataset.label || ''
          return `${label}: ${activeCurrency.value} ${(context.parsed?.y ?? 0).toLocaleString()}`
        },
      },
    },
  },
  scales: {
    x: {
      grid: {
        color: isLightMode.value ? 'rgba(0, 0, 0, 0.08)' : 'rgba(255, 255, 255, 0.08)',
      },
      ticks: {
        color: isLightMode.value ? '#0f172a' : '#94a3b8',
        autoSkip: true,
        maxTicksLimit: 12,
        maxRotation: 0,
        minRotation: 0,
        font: { size: 11 },
      },
    },
    y: {
      grid: {
        color: isLightMode.value ? 'rgba(0, 0, 0, 0.08)' : 'rgba(255, 255, 255, 0.08)',
      },
      ticks: {
        color: isLightMode.value ? '#0f172a' : '#94a3b8',
        font: { size: 11 },
        callback: (value: number | string) => {
          if (typeof value === 'number') {
            if (value >= 1000000) return `${activeCurrency.value} ${value / 1000000}M`
            if (value >= 1000) return `${activeCurrency.value} ${value / 1000}k`
            return `${activeCurrency.value} ${value}`
          }
          return value
        },
      },
    },
  },
}))

const doughnutOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  animation: {
    animateScale: true,
    animateRotate: true,
    duration: 1000,
  },
  plugins: {
    tooltip: {
      enabled: false,
      external: externalChartTooltipHandler,
    },
    legend: {
      position: 'right' as const,
      labels: {
        color: isLightMode.value ? '#0f172a' : '#f8fafc',
        boxWidth: 14,
        padding: 12,
        font: { size: 11 },
      },
    },
  },
}))

function calculateTrend(current: number, previous: number) {
  if (previous === 0) {
    return current > 0 ? { text: '+100.0%', isPositive: true } : { text: '0.0%', isPositive: true }
  }
  const diff = ((current - previous) / Math.abs(previous)) * 100
  return {
    text: `${diff >= 0 ? '+' : ''}${diff.toFixed(1)}%`,
    isPositive: diff >= 0,
  }
}

function showXAxisLabel(index: number) {
  if (selectedPeriod.value !== 'Daily' && !reportDate.value) return true
  return index % 2 === 0
}

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
  if (!dashboard.value) return
  isExportDropdownOpen.value = false
  const rows: Array<Array<string | number>> = [
    ['Summary Metric', 'Value', 'Comparison', 'Period'],
    ['Gross Profit', dashboard.value.profit, profitTrend.value.text, selectedPeriod.value],
    ['Net Profit', dashboard.value.profit, profitTrend.value.text, selectedPeriod.value],
    ['Profit Margin (%)', Number(margin.value.toFixed(2)), marginTrend.value.text, selectedPeriod.value],
    ['Total Revenue', dashboard.value.revenue, revenueTrend.value.text, selectedPeriod.value],
    [],
    ['Profit Trend Bucket', 'Sales Revenue', 'Profit'],
    ...chartPoints.value.map(point => [point.label, point.sales, point.profit]),
    [],
    ['Category', 'Profit', ''],
    ...categories.value.map(category => [category.name, category.value, '']),
  ]
  downloadCsv(`profit_analytics_${selectedPeriod.value.toLowerCase()}_${reportDate.value || 'current'}.csv`, ['Column 1', 'Column 2', 'Column 3', 'Column 4'], rows)
}

function printReport() {
  if (!dashboard.value) return
  isExportDropdownOpen.value = false
  const trendRows = chartPoints.value.map(point => `
    <tr>
      <td>${escapeHtml(point.label)}</td>
      <td class="num">${escapeHtml(formatMoney(point.sales, activeCurrency.value))}</td>
      <td class="num">${escapeHtml(formatMoney(point.profit, activeCurrency.value))}</td>
    </tr>
  `).join('')
  const categoryRows = categories.value.map(category => `
    <tr>
      <td>${escapeHtml(category.name)}</td>
      <td class="num">${escapeHtml(formatMoney(category.value, activeCurrency.value))}</td>
    </tr>
  `).join('')

  openPrintDocument(
    `${props.shopName} Profit Analytics Report`,
    `<div class="meta">
      <div>
        <h1>Profit Analytics Report</h1>
        <p>${escapeHtml(props.shopName)} · ${escapeHtml(selectedPeriod.value)}${reportDate.value ? ` (${escapeHtml(reportDate.value)})` : ''}</p>
      </div>
      <div>
        <p>${escapeHtml(formatDate(new Date()))} ${escapeHtml(formatTime(new Date()))}</p>
        <p>Profit ${escapeHtml(formatMoney(dashboard.value.profit, activeCurrency.value))} · Margin ${escapeHtml(margin.value.toFixed(1))}% · Revenue ${escapeHtml(formatMoney(dashboard.value.revenue, activeCurrency.value))}</p>
      </div>
    </div>
    <h2>Profit Trend</h2>
    <table>
      <thead><tr><th>Period</th><th class="num">Revenue</th><th class="num">Profit</th></tr></thead>
      <tbody>${trendRows}</tbody>
    </table>
    <h2 style="margin-top:24px;">Profit by Category</h2>
    <table>
      <thead><tr><th>Category</th><th class="num">Profit</th></tr></thead>
      <tbody>${categoryRows || '<tr><td colspan="2">No category profit data for this period.</td></tr>'}</tbody>
    </table>`,
  )
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const query = new URLSearchParams({ period: selectedPeriod.value })
    if (reportDate.value) query.set('reportDate', reportDate.value)
    dashboard.value = await apiRequest<Dashboard>(`/dashboard?${query.toString()}`)
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Profit analytics could not be loaded.'
  } finally {
    loading.value = false
  }
}

watch([selectedPeriod, reportDate], load)
onMounted(() => {
  load()
  window.addEventListener('click', closeExportDropdown)
})
onUnmounted(() => {
  window.removeEventListener('click', closeExportDropdown)
})
</script>

<template>
  <section class="analytics-section">
    <div v-if="error" class="inline-error" role="alert">{{ error }}</div>

    <!-- KPI Cards -->
    <div class="kpi-grid">
      <div class="kpi-card">
        <div class="kpi-header">
          <div>
            <h3 class="kpi-title">Gross Profit</h3>
            <p class="kpi-value">{{ formatMoney(dashboard?.profit ?? 0, activeCurrency) }}</p>
          </div>
          <div class="kpi-icon" style="background-color: rgba(34, 197, 94, 0.1); color: #22c55e;">
            <DollarSign :size="24" />
          </div>
        </div>
        <div class="kpi-change" :class="profitTrend.isPositive ? 'positive' : 'negative'">
          <component :is="profitTrend.isPositive ? ArrowUpRight : ArrowDownRight" :size="16" class="change-icon" />
          <span>{{ profitTrend.text }}</span>
          <span class="change-period">{{ periodComparisonText }}</span>
        </div>
      </div>

      <div class="kpi-card">
        <div class="kpi-header">
          <div>
            <h3 class="kpi-title">Net Profit</h3>
            <p class="kpi-value">{{ formatMoney(dashboard?.profit ?? 0, activeCurrency) }}</p>
          </div>
          <div class="kpi-icon" style="background-color: rgba(14, 165, 233, 0.1); color: #0ea5e9;">
            <TrendingUp :size="24" />
          </div>
        </div>
        <div class="kpi-change" :class="profitTrend.isPositive ? 'positive' : 'negative'">
          <component :is="profitTrend.isPositive ? ArrowUpRight : ArrowDownRight" :size="16" class="change-icon" />
          <span>{{ profitTrend.text }}</span>
          <span class="change-period">{{ periodComparisonText }}</span>
        </div>
      </div>

      <div class="kpi-card">
        <div class="kpi-header">
          <div>
            <h3 class="kpi-title">Profit Margin</h3>
            <p class="kpi-value">{{ margin.toFixed(1) }}%</p>
          </div>
          <div class="kpi-icon" style="background-color: rgba(168, 85, 247, 0.1); color: #a855f7;">
            <Percent :size="24" />
          </div>
        </div>
        <div class="kpi-change" :class="marginTrend.isPositive ? 'positive' : 'negative'">
          <component :is="marginTrend.isPositive ? ArrowUpRight : ArrowDownRight" :size="16" class="change-icon" />
          <span>{{ marginTrend.text }}</span>
          <span class="change-period">{{ periodComparisonText }}</span>
        </div>
      </div>

      <div class="kpi-card">
        <div class="kpi-header">
          <div>
            <h3 class="kpi-title">Total Revenue</h3>
            <p class="kpi-value">{{ formatMoney(dashboard?.revenue ?? 0, activeCurrency) }}</p>
          </div>
          <div class="kpi-icon" style="background-color: rgba(249, 115, 22, 0.1); color: #f97316;">
            <BarChart2 :size="24" />
          </div>
        </div>
        <div class="kpi-change" :class="revenueTrend.isPositive ? 'positive' : 'negative'">
          <component :is="revenueTrend.isPositive ? ArrowUpRight : ArrowDownRight" :size="16" class="change-icon" />
          <span>{{ revenueTrend.text }}</span>
          <span class="change-period">{{ periodComparisonText }}</span>
        </div>
      </div>
    </div>

    <!-- Controls Bar -->
    <div class="analytics-controls">
      <div class="controls-left">
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
          <label class="filter-btn date-picker-btn" :class="{ 'active-filter': reportDate }">
            <Calendar :size="16" />
            <span>{{ reportDate || 'Filter by Date' }}</span>
            <input v-model="reportDate" type="date" class="hidden-date-input" aria-label="Filter profit analytics by date" />
          </label>
          <button
            v-if="reportDate"
            type="button"
            class="clear-date-btn"
            title="Clear Date Filter"
            @click="reportDate = ''"
          >
            <X :size="14" />
          </button>
        </div>
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

    <!-- Charts Grid -->
    <div class="charts-grid">
      <div class="chart-card">
        <h3 class="card-title">Profit Trend</h3>
        <div class="chart-container" style="position: relative; height: 100%; min-height: 240px;">
          <Line :data="profitTrendData" :options="chartOptions" />
        </div>
      </div>

      <div class="chart-card">
        <h3 class="card-title">Profit by Category</h3>
        <div v-if="!categories.length || totalCategoryProfit <= 0" class="empty-category-chart">
          No category profit data for this period.
        </div>
        <div v-else class="chart-container doughnut-container" style="position: relative; height: 100%; min-height: 240px; display: flex; align-items: center; justify-content: center;">
          <Doughnut :data="profitByCategoryData" :options="doughnutOptions" />
        </div>
      </div>
    </div>
  </section>
</template>

<style scoped>
.analytics-section {
  display: flex;
  flex-direction: column;
  flex: 1;
  height: 100%;
  min-height: 0;
  gap: 10px;
}

.kpi-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 10px;
  flex-shrink: 0;
}

.kpi-card {
  background: var(--glass-bg);
  backdrop-filter: var(--glass-blur);
  border: 1px solid var(--glass-border);
  border-radius: 14px;
  padding: 1rem 1.25rem;
}

.kpi-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 0.85rem;
}

.kpi-title {
  margin: 0 0 0.4rem;
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--text-muted);
}

.kpi-value {
  margin: 0;
  font-size: 1.65rem;
  font-weight: 700;
  color: var(--text-primary);
}

.kpi-icon {
  padding: 0.7rem;
  border-radius: 12px;
  display: flex;
}

.kpi-change {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  font-size: 0.85rem;
  font-weight: 600;
}

.kpi-change.positive { color: #22c55e; }
.kpi-change.negative { color: #ef4444; }
.change-period {
  color: var(--text-muted);
  font-weight: 400;
  margin-left: 0.2rem;
}

.analytics-controls {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.75rem;
}

.controls-left {
  display: flex;
  align-items: center;
  gap: 1rem;
  flex-wrap: wrap;
}

.time-range-selector {
  display: flex;
  background-color: var(--bg-sidebar);
  border-radius: 0.5rem;
  padding: 0.25rem;
  border: 1px solid var(--border-color);
}

.time-range-selector button {
  background: transparent;
  border: none;
  color: var(--text-muted);
  padding: 0.45rem 1rem;
  border-radius: 0.375rem;
  font-weight: 500;
  font-size: 0.85rem;
  cursor: pointer;
}

.time-range-selector button.active {
  background-color: var(--bg-surface);
  color: var(--text-primary);
}

.date-filter-wrapper {
  display: flex;
  align-items: center;
  gap: 0.25rem;
}

.filter-btn {
  background-color: var(--bg-sidebar);
  color: var(--text-secondary);
  padding: 0.45rem 0.9rem;
  border-radius: 0.5rem;
  border: 1px solid var(--border-color);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  font-weight: 500;
  font-size: 0.85rem;
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
  border-radius: 0.5rem;
  padding: 0.45rem;
  cursor: pointer;
  display: flex;
}

.clear-date-btn:hover {
  background-color: #ef4444;
  color: #ffffff;
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

.charts-grid {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(300px, 1fr);
  gap: 10px;
  flex: 1;
  min-height: 0;
}

.chart-card {
  background: var(--glass-bg);
  backdrop-filter: var(--glass-blur);
  border: 1px solid var(--glass-border);
  border-radius: 14px;
  padding: 1.25rem;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.card-title {
  margin: 0 0 0.85rem;
  font-size: 1.05rem;
  font-weight: 600;
  color: var(--text-primary);
  flex-shrink: 0;
}

.chart-container {
  flex: 1;
  min-height: 200px;
  position: relative;
}

.sales-chart {
  height: 100%;
  display: grid;
  grid-template-columns: 44px minmax(0, 1fr);
  gap: 0.65rem;
}

.chart-y-axis {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  padding-bottom: 24px;
  color: var(--text-muted);
  font-size: 0.72rem;
  text-align: right;
}

.chart-canvas {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
}

.chart-canvas svg {
  width: 100%;
  flex: 1;
  overflow: visible;
}

.chart-grid-line {
  stroke: rgba(255, 255, 255, 0.08);
  stroke-width: 0.4;
}

.profit-line {
  fill: none;
  stroke: #22c55e;
  stroke-width: 1.8;
  vector-effect: non-scaling-stroke;
}

.chart-x-axis {
  height: 24px;
  display: grid;
  grid-auto-flow: column;
  grid-auto-columns: 1fr;
  align-items: center;
  color: var(--text-muted);
  font-size: 0.7rem;
  text-align: center;
}

.chart-hover-zone {
  position: absolute;
  top: 0;
  bottom: 24px;
  width: 24px;
  transform: translateX(-50%);
  cursor: pointer;
}

.chart-dot {
  position: absolute;
  left: 50%;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #22c55e;
  border: 2px solid #0f172a;
  transform: translate(-50%, -50%) scale(0);
  transition: transform 0.15s ease;
  pointer-events: none;
}

.chart-dot.active {
  transform: translate(-50%, -50%) scale(1.25);
}

.chart-floating-tooltip {
  position: absolute;
  transform: translate(-50%, -100%);
  background: rgba(15, 23, 42, 0.95);
  border: 1px solid rgba(148, 163, 184, 0.25);
  border-radius: 8px;
  padding: 0.45rem 0.65rem;
  pointer-events: none;
  z-index: 20;
  white-space: nowrap;
  box-shadow: 0 10px 20px rgba(0, 0, 0, 0.35);
}

.tooltip-label {
  font-size: 0.72rem;
  color: #94a3b8;
  margin-bottom: 0.15rem;
}

.tooltip-value {
  font-size: 0.82rem;
  font-weight: 700;
  color: #4ade80;
}

.tooltip-sub {
  font-size: 0.72rem;
  color: #e2e8f0;
}

/* Doughnut Chart */
.empty-category-chart {
  flex: 1;
  min-height: 180px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  font-size: 0.875rem;
}

.doughnut-layout {
  flex: 1;
  min-height: 180px;
  display: flex;
  align-items: center;
  gap: 1.25rem;
}

.doughnut-svg-wrap {
  width: 170px;
  height: 170px;
  flex-shrink: 0;
}

.doughnut-svg {
  width: 100%;
  height: 100%;
}

.doughnut-legend {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  flex-grow: 1;
  min-width: 0;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.8rem;
}

.legend-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.legend-name {
  color: var(--text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex-grow: 1;
}

.legend-val {
  color: var(--text-primary);
  font-weight: 600;
}

@media (max-width: 1024px) {
  .kpi-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .charts-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 640px) {
  .kpi-grid {
    grid-template-columns: 1fr;
  }
}
</style>
