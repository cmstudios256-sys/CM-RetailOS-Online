<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import {
  ArrowDownRight,
  ArrowUpRight,
  BadgePercent,
  BarChart3,
  Calendar,
  ChevronDown,
  Download,
  FileSpreadsheet,
  PieChart,
  Printer,
  TrendingUp,
  Wallet,
  X,
} from '@lucide/vue'
import { apiRequest, formatDate, formatMoney, formatTime } from '../api'
import AppDatePicker from './AppDatePicker.vue'
import { downloadCsv, escapeHtml, openPrintDocument } from '../utils/reporting'
import { useDelayedSkeleton } from '../utils/useDelayedSkeleton'
import { externalChartTooltipHandler } from '../utils/chartTooltip'
import { getThemeColors, hexToRgba } from '../utils/chartColors'
import { Bar, Line, Doughnut } from 'vue-chartjs'
import { usePhoneBreakpoint } from '../utils/breakpoints'
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
  BarElement,
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
  BarElement,
)

type Period = 'Daily' | 'Weekly' | 'Monthly'
type TrendMode = 'profit' | 'comparison' | 'margin'

type ChartPoint = {
  label: string
  sales: number
  profit: number
}

const { isPhone } = usePhoneBreakpoint()

const themeVersion = ref(0)
function handlePrefsUpdated() {
  themeVersion.value++
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
const trendMode = ref<TrendMode>('profit')
const reportDate = ref('')
const dashboard = ref<Dashboard | null>(null)
const loading = ref(true)
const showSkeleton = useDelayedSkeleton(loading, 200)
const error = ref('')
const isExportDropdownOpen = ref(false)

const categoryPalette = ['#10b981', '#06b6d4', '#6366f1', '#f59e0b', '#ec4899', '#8b5cf6', '#3b82f6', '#14b8a6']

const activeCurrency = computed(() => props.currency || dashboard.value?.currency || 'UGX')

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
const categories = computed(() => dashboard.value?.profitByCategory ?? [])

const totalCategoryProfit = computed(() =>
  categories.value.reduce((sum, category) => sum + Math.max(category.value, 0), 0),
)

const categoryBreakdown = computed(() => {
  const total = totalCategoryProfit.value
  return categories.value
    .filter(cat => cat.value > 0)
    .map((cat, idx) => {
      const val = Math.max(0, cat.value)
      const percentage = total > 0 ? (val / total) * 100 : 0
      return {
        name: cat.name,
        value: val,
        percentage: Number(percentage.toFixed(1)),
        color: categoryPalette[idx % categoryPalette.length],
      }
    })
})

const isLightMode = computed(() => document.documentElement.getAttribute('data-theme') === 'light')

const profitTrendData = computed(() => {
  void themeVersion.value
  const { accent, complement } = getThemeColors()
  const points = chartPoints.value
  const labels = points.map(p => p.label)

  if (trendMode.value === 'margin') {
    const data = points.map(p => (p.sales > 0 ? Number(((p.profit / p.sales) * 100).toFixed(1)) : 0))
    return {
      labels,
      datasets: [{
        type: 'line' as const,
        label: 'Profit Margin (%)',
        borderColor: '#a855f7',
        backgroundColor: (context: any) => {
          const chart = context.chart
          const { ctx, chartArea } = chart
          if (!chartArea) return 'rgba(168, 85, 247, 0.2)'
          const gradient = ctx.createLinearGradient(0, chartArea.top, 0, chartArea.bottom)
          gradient.addColorStop(0, 'rgba(168, 85, 247, 0.35)')
          gradient.addColorStop(0.5, 'rgba(168, 85, 247, 0.1)')
          gradient.addColorStop(1, 'rgba(168, 85, 247, 0.0)')
          return gradient
        },
        borderWidth: 2.5,
        fill: true,
        tension: 0.35,
        pointRadius: (ctx: any) => (Number(ctx.raw) > 0 ? 3.5 : 0),
        pointHoverRadius: 6,
        pointBackgroundColor: '#a855f7',
        pointBorderColor: '#ffffff',
        pointBorderWidth: 1.5,
        data,
      }],
    }
  }

  if (trendMode.value === 'comparison') {
    return {
      labels,
      datasets: [
        {
          type: 'line' as const,
          label: 'Revenue',
          borderColor: accent,
          backgroundColor: (context: any) => {
            const chart = context.chart
            const { ctx, chartArea } = chart
            if (!chartArea) return hexToRgba(accent, 0.15)
            const gradient = ctx.createLinearGradient(0, chartArea.top, 0, chartArea.bottom)
            gradient.addColorStop(0, hexToRgba(accent, 0.28))
            gradient.addColorStop(0.5, hexToRgba(accent, 0.08))
            gradient.addColorStop(1, hexToRgba(accent, 0.0))
            return gradient
          },
          borderWidth: 2,
          fill: true,
          tension: 0.35,
          pointRadius: (ctx: any) => (Number(ctx.raw) > 0 ? 3 : 0),
          pointHoverRadius: 5,
          pointBackgroundColor: accent,
          pointBorderColor: '#ffffff',
          pointBorderWidth: 1.5,
          data: points.map(p => p.sales),
        },
        {
          type: 'line' as const,
          label: 'Net Profit',
          borderColor: complement,
          backgroundColor: (context: any) => {
            const chart = context.chart
            const { ctx, chartArea } = chart
            if (!chartArea) return hexToRgba(complement, 0.2)
            const gradient = ctx.createLinearGradient(0, chartArea.top, 0, chartArea.bottom)
            gradient.addColorStop(0, hexToRgba(complement, 0.38))
            gradient.addColorStop(0.5, hexToRgba(complement, 0.12))
            gradient.addColorStop(1, hexToRgba(complement, 0.0))
            return gradient
          },
          borderWidth: 2.5,
          fill: true,
          tension: 0.35,
          pointRadius: (ctx: any) => (Number(ctx.raw) > 0 ? 3.5 : 0),
          pointHoverRadius: 6,
          pointBackgroundColor: complement,
          pointBorderColor: '#ffffff',
          pointBorderWidth: 1.5,
          data: points.map(p => p.profit),
        },
      ],
    }
  }

  // Default: Net Profit (uses accent border with complementary subtle gradient fill)
  return {
    labels,
    datasets: [{
      type: 'line' as const,
      label: 'Net Profit',
      borderColor: accent,
      backgroundColor: (context: any) => {
        const chart = context.chart
        const { ctx, chartArea } = chart
        if (!chartArea) return hexToRgba(complement, 0.2)
        const gradient = ctx.createLinearGradient(0, chartArea.top, 0, chartArea.bottom)
        gradient.addColorStop(0, hexToRgba(complement, 0.4))
        gradient.addColorStop(0.5, hexToRgba(complement, 0.12))
        gradient.addColorStop(1, hexToRgba(complement, 0.0))
        return gradient
      },
      borderWidth: 2.5,
      fill: true,
      tension: 0.35,
      pointRadius: (ctx: any) => (Number(ctx.raw) > 0 ? 3.5 : 0),
      pointHoverRadius: 6,
      pointBackgroundColor: accent,
      pointBorderColor: '#ffffff',
      pointBorderWidth: 1.5,
      data: points.map(p => p.profit),
    }],
  }
})

const mobileProfitTrendData = computed(() => {
  void themeVersion.value
  const { accent, complement } = getThemeColors()
  const points = chartPoints.value.slice(-7)
  return {
    labels: points.map(point => point.label),
    datasets: [{
      label: 'Net Profit',
      data: points.map(point => point.profit),
      borderColor: accent,
      backgroundColor: selectedPeriod.value === 'Daily' ? accent : hexToRgba(complement, 0.25),
      borderWidth: 2,
      borderRadius: selectedPeriod.value === 'Daily' ? 4 : 0,
      fill: selectedPeriod.value !== 'Daily',
      tension: selectedPeriod.value === 'Daily' ? 0 : 0.35,
      pointRadius: selectedPeriod.value === 'Daily' ? 0 : 3,
      pointHoverRadius: selectedPeriod.value === 'Daily' ? 0 : 6,
    }],
  }
})

const hasMobileProfitData = computed(() =>
  mobileProfitTrendData.value.datasets.some(dataset =>
    dataset.data.some(value => Number(value) !== 0),
  ),
)

const mobileChartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    mode: 'nearest' as const,
    intersect: true,
  },
  plugins: {
    legend: { display: false },
    tooltip: {
      enabled: true,
      displayColors: false,
      callbacks: {
        label: (context: any) => `${activeCurrency.value} ${Number(context.parsed.y || 0).toLocaleString()}`,
      },
    },
  },
  scales: {
    x: {
      grid: { display: false },
      border: { display: false },
      ticks: {
        color: isLightMode.value ? '#0f172a' : '#94a3b8',
        autoSkip: true,
        maxTicksLimit: 4,
        maxRotation: 0,
        font: { size: 10 },
      },
    },
    y: {
      beginAtZero: true,
      grid: {
        display: true,
        color: isLightMode.value ? 'rgba(148, 163, 184, 0.08)' : 'rgba(148, 163, 184, 0.08)',
        drawTicks: false,
      },
      border: { display: false },
      ticks: {
        maxTicksLimit: 4,
        color: isLightMode.value ? '#0f172a' : '#94a3b8',
        font: { size: 10 },
        callback: (value: number | string) => {
          const amount = Number(value)
          if (amount >= 1000000) return `${amount / 1000000}M`
          if (amount >= 1000) return `${amount / 1000}k`
          return `${amount}`
        },
      },
    },
  },
}))

const profitByCategoryData = computed(() => {
  const cats = categoryBreakdown.value
  return {
    labels: cats.map(c => c.name),
    datasets: [{
      backgroundColor: cats.map(c => c.color),
      hoverBackgroundColor: cats.map(c => c.color),
      borderWidth: 2,
      borderColor: isLightMode.value ? '#ffffff' : '#0f172a',
      data: cats.map(c => c.value),
    }],
  }
})

const chartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    mode: 'index' as const,
    intersect: false,
  },
  plugins: {
    legend: {
      display: trendMode.value === 'comparison',
      position: 'top' as const,
      align: 'end' as const,
      labels: {
        color: isLightMode.value ? '#0f172a' : '#94a3b8',
        boxWidth: 8,
        boxHeight: 8,
        usePointStyle: true,
        pointStyle: 'circle',
        padding: 14,
        font: { size: 11, weight: 'bold' as const },
      },
    },
    tooltip: {
      enabled: false,
      external: externalChartTooltipHandler,
      callbacks: {
        label: (context: any) => {
          const label = context.dataset.label || ''
          const val = context.parsed?.y ?? 0
          if (trendMode.value === 'margin') {
            return `${label}: ${val.toFixed(1)}%`
          }
          return `${label}: ${activeCurrency.value} ${val.toLocaleString()}`
        },
      },
    },
  },
  scales: {
    x: {
      grid: {
        display: false,
        drawBorder: false,
      },
      ticks: {
        color: isLightMode.value ? '#64748b' : '#94a3b8',
        autoSkip: true,
        maxTicksLimit: 12,
        maxRotation: 0,
        font: { size: 11, weight: 'normal' as const },
      },
    },
    y: {
      beginAtZero: true,
      grid: {
        color: isLightMode.value ? 'rgba(0, 0, 0, 0.06)' : 'rgba(255, 255, 255, 0.05)',
        borderDash: [3, 4],
        drawTicks: false,
      },
      border: { display: false },
      ticks: {
        color: isLightMode.value ? '#64748b' : '#94a3b8',
        font: { size: 11 },
        padding: 8,
        callback: (value: number | string) => {
          const num = Number(value)
          if (trendMode.value === 'margin') return `${num.toFixed(0)}%`
          if (num >= 1000000) return `${num / 1000000}M`
          if (num >= 1000) return `${num / 1000}k`
          return `${num}`
        },
      },
    },
  },
}))

const doughnutOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  cutout: '74%',
  plugins: {
    legend: { display: false },
    tooltip: {
      enabled: false,
      external: externalChartTooltipHandler,
      callbacks: {
        label: (context: any) => {
          const val = Number(context.raw || 0)
          const pct = totalCategoryProfit.value > 0 ? ((val / totalCategoryProfit.value) * 100).toFixed(1) : '0'
          return `${context.label}: ${activeCurrency.value} ${val.toLocaleString()} (${pct}%)`
        },
      },
    },
  },
  animation: {
    animateScale: true,
    animateRotate: true,
    duration: 800,
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
    ['Category', 'Profit', 'Share (%)'],
    ...categoryBreakdown.value.map(cat => [cat.name, cat.value, `${cat.percentage}%`]),
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
  const categoryRows = categoryBreakdown.value.map(category => `
    <tr>
      <td>${escapeHtml(category.name)}</td>
      <td class="num">${escapeHtml(formatMoney(category.value, activeCurrency.value))}</td>
      <td class="num">${category.percentage}%</td>
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
      <thead><tr><th>Category</th><th class="num">Profit</th><th class="num">Share</th></tr></thead>
      <tbody>${categoryRows || '<tr><td colspan="3">No category profit data for this period.</td></tr>'}</tbody>
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
  void load()
  window.addEventListener('click', closeExportDropdown)
  window.addEventListener('online-prefs-updated', handlePrefsUpdated)
})
onUnmounted(() => {
  window.removeEventListener('click', closeExportDropdown)
  window.removeEventListener('online-prefs-updated', handlePrefsUpdated)
})
</script>

<template>
  <section class="analytics-section">
    <div v-if="error" class="inline-error" role="alert">{{ error }}</div>

    <template v-if="showSkeleton">
      <div class="kpi-grid">
        <div v-for="i in 4" :key="i" class="kpi-card skeleton-box skeleton-shimmer" style="height: 110px; border-radius: 14px;"></div>
      </div>
      <div class="analytics-panels-grid">
        <div class="analytics-panel skeleton-box skeleton-shimmer" style="height: 400px; border-radius: 16px;"></div>
        <div class="analytics-panel skeleton-box skeleton-shimmer" style="height: 400px; border-radius: 16px;"></div>
      </div>
    </template>

    <template v-else>
      <!-- KPI Cards Grid -->
      <div class="kpi-grid">
      <!-- Gross Profit -->
      <div class="kpi-card card-gross">
        <div class="kpi-header">
          <div>
            <div class="kpi-title-row">
              <h3 class="kpi-title">Gross Profit</h3>
              <span class="kpi-badge-hint">Before OPEX</span>
            </div>
            <p class="kpi-value">{{ formatMoney(dashboard?.profit ?? 0, activeCurrency) }}</p>
          </div>
          <div class="kpi-icon icon-gross">
            <Wallet :size="20" />
          </div>
        </div>
        <div class="kpi-footer">
          <div class="kpi-change" :class="profitTrend.isPositive ? 'positive' : 'negative'">
            <component :is="profitTrend.isPositive ? ArrowUpRight : ArrowDownRight" :size="15" class="change-icon" />
            <span>{{ profitTrend.text }}</span>
          </div>
          <span class="change-period">{{ periodComparisonText }}</span>
        </div>
      </div>

      <!-- Net Profit -->
      <div class="kpi-card card-net">
        <div class="kpi-header">
          <div>
            <div class="kpi-title-row">
              <h3 class="kpi-title">Net Profit</h3>
              <span class="kpi-badge-hint">After Discounts</span>
            </div>
            <p class="kpi-value">{{ formatMoney(dashboard?.profit ?? 0, activeCurrency) }}</p>
          </div>
          <div class="kpi-icon icon-net">
            <TrendingUp :size="20" />
          </div>
        </div>
        <div class="kpi-footer">
          <div class="kpi-change" :class="profitTrend.isPositive ? 'positive' : 'negative'">
            <component :is="profitTrend.isPositive ? ArrowUpRight : ArrowDownRight" :size="15" class="change-icon" />
            <span>{{ profitTrend.text }}</span>
          </div>
          <span class="change-period">{{ periodComparisonText }}</span>
        </div>
      </div>

      <!-- Profit Margin -->
      <div class="kpi-card card-margin">
        <div class="kpi-header">
          <div>
            <div class="kpi-title-row">
              <h3 class="kpi-title">Profit Margin</h3>
              <span class="kpi-badge-hint">Efficiency</span>
            </div>
            <p class="kpi-value">{{ margin.toFixed(1) }}%</p>
          </div>
          <div class="kpi-icon icon-margin">
            <BadgePercent :size="20" />
          </div>
        </div>
        <div class="kpi-footer">
          <div class="kpi-change" :class="marginTrend.isPositive ? 'positive' : 'negative'">
            <component :is="marginTrend.isPositive ? ArrowUpRight : ArrowDownRight" :size="15" class="change-icon" />
            <span>{{ marginTrend.text }}</span>
          </div>
          <span class="change-period">{{ periodComparisonText }}</span>
        </div>
      </div>

      <!-- Total Revenue -->
      <div class="kpi-card card-revenue">
        <div class="kpi-header">
          <div>
            <div class="kpi-title-row">
              <h3 class="kpi-title">Total Revenue</h3>
              <span class="kpi-badge-hint">Gross Volume</span>
            </div>
            <p class="kpi-value">{{ formatMoney(dashboard?.revenue ?? 0, activeCurrency) }}</p>
          </div>
          <div class="kpi-icon icon-revenue">
            <BarChart3 :size="20" />
          </div>
        </div>
        <div class="kpi-footer">
          <div class="kpi-change" :class="revenueTrend.isPositive ? 'positive' : 'negative'">
            <component :is="revenueTrend.isPositive ? ArrowUpRight : ArrowDownRight" :size="15" class="change-icon" />
            <span>{{ revenueTrend.text }}</span>
          </div>
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

        <AppDatePicker
          v-model="reportDate"
          placeholder="Filter by Date"
          title="Filter Profit Analytics by Date"
          aria-label="Filter profit analytics by date"
        />
      </div>

      <div class="export-dropdown-container">
        <button type="button" class="export-btn" @click.stop="toggleExportDropdown">
          <Download :size="15" />
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

    <!-- Mobile View -->
    <div v-if="isPhone" class="mobile-chart-card">
      <h3 class="card-title">Profit Trend</h3>
      <div class="mobile-chart-container">
        <div v-if="!hasMobileProfitData" class="empty-chart-state">
          No profit recorded in this period
        </div>
        <Bar
          v-else-if="selectedPeriod === 'Daily'"
          :data="mobileProfitTrendData"
          :options="mobileChartOptions"
        />
        <Line
          v-else
          :data="mobileProfitTrendData"
          :options="mobileChartOptions"
        />
      </div>
    </div>

    <!-- Desktop / Tablet Charts Grid -->
    <div v-else class="charts-grid">
      <!-- Profit & Dynamics Trend Card -->
      <div class="chart-card trend-card">
        <div class="card-header-row">
          <div>
            <h3 class="card-title">Profit & Revenue Dynamics</h3>
            <p class="card-subtitle">{{ selectedPeriod }} profitability trajectory</p>
          </div>

          <div class="trend-mode-toggle">
            <button
              type="button"
              class="mode-btn"
              :class="{ active: trendMode === 'profit' }"
              @click="trendMode = 'profit'"
            >
              Net Profit
            </button>
            <button
              type="button"
              class="mode-btn"
              :class="{ active: trendMode === 'comparison' }"
              @click="trendMode = 'comparison'"
            >
              Revenue vs Profit
            </button>
            <button
              type="button"
              class="mode-btn"
              :class="{ active: trendMode === 'margin' }"
              @click="trendMode = 'margin'"
            >
              Margin %
            </button>
          </div>
        </div>

        <div class="chart-container" style="position: relative; height: 100%; min-height: 280px;">
          <Line :data="profitTrendData" :options="chartOptions" />
        </div>
      </div>

      <!-- Profit by Category Card -->
      <div class="chart-card category-card">
        <div class="card-header-row">
          <div>
            <h3 class="card-title">Profit by Category</h3>
            <p class="card-subtitle">Contribution across catalog sectors</p>
          </div>
          <span v-if="categoryBreakdown.length" class="category-count-badge">
            {{ categoryBreakdown.length }} {{ categoryBreakdown.length === 1 ? 'Category' : 'Categories' }}
          </span>
        </div>

        <div v-if="!categoryBreakdown.length || totalCategoryProfit <= 0" class="empty-category-chart">
          <div class="empty-chart-icon-box">
            <PieChart :size="32" />
          </div>
          <p class="empty-title">No category profit recorded</p>
          <span class="empty-hint">Category metrics will reflect as sales are processed.</span>
        </div>

        <div v-else class="category-analytics-body">
          <div class="doughnut-chart-wrapper">
            <div class="doughnut-canvas-box">
              <Doughnut :data="profitByCategoryData" :options="doughnutOptions" />
              <div class="donut-center-stat">
                <span class="donut-center-label">TOTAL PROFIT</span>
                <span class="donut-center-val">{{ formatMoney(totalCategoryProfit, activeCurrency) }}</span>
              </div>
            </div>
          </div>

          <div class="category-list-wrapper">
            <div
              v-for="cat in categoryBreakdown"
              :key="cat.name"
              class="category-breakdown-row"
            >
              <div class="cat-row-top">
                <div class="cat-name-col">
                  <span class="cat-color-dot" :style="{ backgroundColor: cat.color }" />
                  <span class="cat-name" :title="cat.name">{{ cat.name }}</span>
                </div>
                <div class="cat-val-col">
                  <span class="cat-val">{{ formatMoney(cat.value, activeCurrency) }}</span>
                  <span class="cat-pct">{{ cat.percentage }}%</span>
                </div>
              </div>
              <div class="cat-progress-track">
                <div
                  class="cat-progress-fill"
                  :style="{ width: `${cat.percentage}%`, backgroundColor: cat.color }"
                />
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
    </template>
  </section>
</template>

<style scoped>
.analytics-section {
  display: flex;
  flex-direction: column;
  flex: 1;
  height: 100%;
  min-height: 0;
  gap: 12px;
}

/* ==========================================================================
   KPI Cards Grid
   ========================================================================== */
.kpi-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
  flex-shrink: 0;
}

.kpi-card {
  position: relative;
  background: var(--glass-bg);
  backdrop-filter: var(--glass-blur);
  border: 1px solid var(--glass-border);
  border-radius: 14px;
  padding: 1.1rem 1.25rem;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
  overflow: hidden;
}

.kpi-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 12px 28px -10px rgba(0, 0, 0, 0.4);
}

/* Colored accent borders & gradients */
.card-gross {
  border-top: 2.5px solid #10b981;
  background: linear-gradient(180deg, rgba(16, 185, 129, 0.06) 0%, rgba(15, 23, 42, 0.4) 100%);
}
.card-net {
  border-top: 2.5px solid #0ea5e9;
  background: linear-gradient(180deg, rgba(14, 165, 233, 0.06) 0%, rgba(15, 23, 42, 0.4) 100%);
}
.card-margin {
  border-top: 2.5px solid #a855f7;
  background: linear-gradient(180deg, rgba(168, 85, 247, 0.06) 0%, rgba(15, 23, 42, 0.4) 100%);
}
.card-revenue {
  border-top: 2.5px solid #f59e0b;
  background: linear-gradient(180deg, rgba(245, 158, 11, 0.06) 0%, rgba(15, 23, 42, 0.4) 100%);
}

[data-theme='light'] .kpi-card {
  background: #ffffff !important;
  border: 1px solid #e2e8f0 !important;
  box-shadow: 0 4px 18px -2px rgba(15, 23, 42, 0.06), 0 2px 6px -1px rgba(15, 23, 42, 0.03) !important;
}

[data-theme='light'] .kpi-card:hover {
  box-shadow: 0 10px 25px -4px rgba(15, 23, 42, 0.1) !important;
}

[data-theme='light'] .card-gross {
  border-top: 2.5px solid #10b981 !important;
  background: linear-gradient(180deg, rgba(16, 185, 129, 0.08) 0%, #ffffff 100%) !important;
}

[data-theme='light'] .card-net {
  border-top: 2.5px solid #0ea5e9 !important;
  background: linear-gradient(180deg, rgba(14, 165, 233, 0.08) 0%, #ffffff 100%) !important;
}

[data-theme='light'] .card-margin {
  border-top: 2.5px solid #a855f7 !important;
  background: linear-gradient(180deg, rgba(168, 85, 247, 0.08) 0%, #ffffff 100%) !important;
}

[data-theme='light'] .card-revenue {
  border-top: 2.5px solid #f59e0b !important;
  background: linear-gradient(180deg, rgba(245, 158, 11, 0.08) 0%, #ffffff 100%) !important;
}

[data-theme='light'] .kpi-value {
  color: #0f172a !important;
}

[data-theme='light'] .kpi-title {
  color: #64748b !important;
}

[data-theme='light'] .kpi-badge-hint {
  background: #f1f5f9 !important;
  color: #64748b !important;
}

[data-theme='light'] .time-range-selector {
  background-color: #f1f5f9 !important;
  border-color: #e2e8f0 !important;
}

[data-theme='light'] .time-range-selector button {
  color: #64748b !important;
}

[data-theme='light'] .time-range-selector button:hover {
  color: #0f172a !important;
}

[data-theme='light'] .time-range-selector button.active {
  background-color: #ffffff !important;
  color: var(--accent-color) !important;
  border: 1px solid #cbd5e1 !important;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.08) !important;
}

[data-theme='light'] .filter-btn {
  background-color: #ffffff !important;
  border-color: #cbd5e1 !important;
  color: #334155 !important;
}

.kpi-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 0.8rem;
}

.kpi-title-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.35rem;
}

.kpi-title {
  margin: 0;
  font-size: 0.825rem;
  font-weight: 600;
  color: var(--text-muted);
  letter-spacing: 0.02em;
}

.kpi-badge-hint {
  font-size: 0.65rem;
  font-weight: 500;
  padding: 0.15rem 0.4rem;
  border-radius: 4px;
  background: rgba(148, 163, 184, 0.1);
  color: #94a3b8;
}

.kpi-value {
  margin: 0;
  font-size: 1.55rem;
  font-weight: 800;
  color: var(--text-primary);
  letter-spacing: -0.02em;
}

.kpi-icon {
  width: 38px;
  height: 38px;
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.icon-gross { background-color: rgba(16, 185, 129, 0.12); color: #10b981; }
.icon-net { background-color: rgba(14, 165, 233, 0.12); color: #0ea5e9; }
.icon-margin { background-color: rgba(168, 85, 247, 0.12); color: #a855f7; }
.icon-revenue { background-color: rgba(245, 158, 11, 0.12); color: #f59e0b; }

.kpi-footer {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.kpi-change {
  display: inline-flex;
  align-items: center;
  gap: 0.2rem;
  font-size: 0.775rem;
  font-weight: 700;
  padding: 0.2rem 0.5rem;
  border-radius: 6px;
  border: none;
}

.kpi-change.positive {
  background-color: rgba(16, 185, 129, 0.12);
  color: #10b981;
}

.kpi-change.negative {
  background-color: rgba(239, 68, 68, 0.12);
  color: #ef4444;
}

.change-period {
  color: var(--text-muted);
  font-size: 0.75rem;
  font-weight: 400;
}

/* ==========================================================================
   Controls Bar
   ========================================================================== */
.analytics-controls {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.75rem;
  flex-shrink: 0;
}

.controls-left {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
}

.time-range-selector {
  display: inline-flex;
  background-color: var(--bg-sidebar);
  border-radius: 8px;
  padding: 0.25rem;
  border: 1px solid var(--border-subtle);
}

.time-range-selector button {
  background: transparent;
  border: none;
  color: var(--text-muted);
  padding: 0.4rem 0.95rem;
  border-radius: 6px;
  font-weight: 600;
  font-size: 0.825rem;
  cursor: pointer;
  transition: all 0.18s ease;
}

.time-range-selector button:hover {
  color: var(--text-primary);
}

.time-range-selector button.active {
  background-color: var(--bg-surface);
  color: #38bdf8;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.25);
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
  border-radius: 8px;
  border: 1px solid var(--border-subtle);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  font-weight: 500;
  font-size: 0.825rem;
  position: relative;
  overflow: hidden;
  transition: all 0.18s ease;
}

.filter-btn:hover {
  border-color: var(--border-color);
  color: var(--text-primary);
}

.date-picker-btn.active-filter {
  border-color: rgba(14, 165, 233, 0.5);
  color: #38bdf8;
  background-color: rgba(14, 165, 233, 0.08);
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
  border-radius: 8px;
  width: 32px;
  height: 32px;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.18s ease;
}

.clear-date-btn:hover {
  background-color: #ef4444;
  color: #ffffff;
}

.export-dropdown-container {
  position: relative;
}

.export-btn {
  background-color: var(--accent-color, #0ea5e9);
  color: #ffffff;
  font-weight: 600;
  font-size: 0.825rem;
  padding: 0.5rem 1rem;
  border-radius: 8px;
  border: none;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  box-shadow: 0 4px 12px rgba(14, 165, 233, 0.25);
  transition: all 0.18s ease;
}

.export-btn:hover {
  background-color: var(--accent-hover, #0284c7);
  box-shadow: 0 6px 16px rgba(14, 165, 233, 0.35);
}

.chevron-icon {
  transition: transform 0.2s ease;
}

.chevron-icon.rotate {
  transform: rotate(180deg);
}

.export-dropdown-menu {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  width: 220px;
  background-color: #141f32;
  border: none !important;
  outline: none !important;
  border-radius: 14px;
  box-shadow: 0 20px 45px -4px rgba(0, 0, 0, 0.65), 0 0 0 1px rgba(255, 255, 255, 0.08);
  padding: 6px;
  z-index: 9999;
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.dropdown-item {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.65rem 0.85rem;
  min-height: 46px;
  border-radius: 10px;
  background: transparent;
  border: none !important;
  outline: none !important;
  color: #e2e8f0;
  cursor: pointer;
  text-align: left;
  width: 100%;
  font-size: 0.92rem;
  font-weight: 550;
  transition: background-color 0.15s ease, transform 0.1s ease;
}

.dropdown-item:hover {
  background-color: rgba(255, 255, 255, 0.08);
  color: #ffffff;
}

.dropdown-item:active {
  background-color: rgba(255, 255, 255, 0.14);
  transform: scale(0.98);
}

.item-icon.csv { color: #22c55e; }
.item-icon.pdf { color: #a855f7; }

.item-text {
  display: flex;
  flex-direction: column;
}

.item-title {
  font-size: 0.825rem;
  font-weight: 600;
  color: var(--text-primary);
}

.item-desc {
  font-size: 0.7rem;
  color: var(--text-muted);
}

/* ==========================================================================
   Charts Grid & Cards
   ========================================================================== */
.charts-grid {
  display: grid;
  grid-template-columns: minmax(0, 1.8fr) minmax(360px, 1.2fr);
  gap: 12px;
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

.card-header-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
  flex-shrink: 0;
  gap: 0.5rem;
}

.card-title {
  margin: 0;
  font-size: 1rem;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: -0.01em;
}

.card-subtitle {
  margin: 0.15rem 0 0;
  font-size: 0.75rem;
  color: var(--text-muted);
}

.trend-mode-toggle {
  display: inline-flex;
  background: rgba(148, 163, 184, 0.08);
  border: 1px solid var(--border-subtle);
  border-radius: 7px;
  padding: 0.2rem;
  gap: 0.2rem;
}

.mode-btn {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 0.75rem;
  font-weight: 600;
  padding: 0.3rem 0.65rem;
  border-radius: 5px;
  cursor: pointer;
  transition: all 0.15s ease;
}

.mode-btn:hover {
  color: var(--text-primary);
}

.mode-btn.active {
  background-color: var(--bg-surface);
  color: #38bdf8;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
}

.chart-container {
  flex: 1;
  min-height: 240px;
  position: relative;
}

/* ==========================================================================
   Category Breakdown & Doughnut Layout
   ========================================================================== */
.category-count-badge {
  font-size: 0.72rem;
  font-weight: 600;
  padding: 0.25rem 0.6rem;
  border-radius: 6px;
  background: rgba(14, 165, 233, 0.1);
  color: #38bdf8;
  border: 1px solid rgba(14, 165, 233, 0.2);
}

.category-analytics-body {
  display: flex;
  flex-direction: column;
  flex: 1;
  gap: 1rem;
  min-height: 0;
}

.doughnut-chart-wrapper {
  display: flex;
  justify-content: center;
  align-items: center;
  flex-shrink: 0;
  padding: 0.5rem 0;
}

.doughnut-canvas-box {
  position: relative;
  width: 175px;
  height: 175px;
}

.donut-center-stat {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  pointer-events: none;
  text-align: center;
}

.donut-center-label {
  font-size: 0.625rem;
  font-weight: 700;
  color: var(--text-muted);
  letter-spacing: 0.06em;
}

.donut-center-val {
  font-size: 0.95rem;
  font-weight: 800;
  color: var(--text-primary);
  margin-top: 0.1rem;
}

.category-list-wrapper {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding-right: 0.25rem;
}

.category-breakdown-row {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.cat-row-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.775rem;
}

.cat-name-col {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  min-width: 0;
  overflow: hidden;
}

.cat-color-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}

.cat-name {
  color: var(--text-secondary);
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.cat-val-col {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
}

.cat-val {
  font-weight: 700;
  color: var(--text-primary);
}

.cat-pct {
  font-size: 0.7rem;
  font-weight: 600;
  color: var(--text-muted);
  background: rgba(148, 163, 184, 0.08);
  padding: 0.1rem 0.35rem;
  border-radius: 4px;
}

.cat-progress-track {
  width: 100%;
  height: 4px;
  border-radius: 2px;
  background: rgba(148, 163, 184, 0.12);
  overflow: hidden;
}

.cat-progress-fill {
  height: 100%;
  border-radius: 2px;
  transition: width 0.6s cubic-bezier(0.4, 0, 0.2, 1);
}

.empty-category-chart {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  padding: 2rem;
  color: var(--text-muted);
}

.empty-chart-icon-box {
  width: 52px;
  height: 52px;
  border-radius: 12px;
  background: rgba(148, 163, 184, 0.08);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 0.75rem;
  color: var(--text-muted);
}

.empty-title {
  margin: 0 0 0.25rem;
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--text-secondary);
}

.empty-hint {
  font-size: 0.75rem;
  color: var(--text-muted);
}

/* ==========================================================================
   Mobile Chart Card
   ========================================================================== */
.mobile-chart-card {
  min-width: 0;
  background: var(--glass-bg);
  border: 1px solid var(--glass-border);
  border-radius: 14px;
  padding: 12px;
}

.mobile-chart-container {
  position: relative;
  width: 100%;
  height: 220px;
  min-width: 0;
}

.empty-chart-state {
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  font-size: 0.85rem;
}

/* ==========================================================================
   Responsive Breakpoints
   ========================================================================== */
@media (max-width: 1200px) {
  .charts-grid {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 1024px) {
  .kpi-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}

@media (max-width: 640px) {
  .kpi-grid {
    grid-template-columns: 1fr;
  }

  .analytics-controls {
    align-items: flex-start;
  }

  .controls-left {
    width: 100%;
    display: grid;
    grid-template-columns: 1fr;
    gap: 8px;
  }

  .time-range-selector {
    width: 100%;
  }

  .time-range-selector button {
    flex: 1;
    min-height: 40px;
  }

  .date-picker-btn {
    width: 100%;
  }
}
</style>
