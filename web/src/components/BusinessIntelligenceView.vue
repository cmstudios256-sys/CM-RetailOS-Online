<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import {
  Award,
  Calendar,
  ChevronDown,
  Clock,
  Download,
  FileSpreadsheet,
  Plus,
  Printer,
  ShoppingBag,
  X,
} from '@lucide/vue'
import { apiRequest, formatDate, formatMoney, formatTime } from '../api'
import { downloadCsv, escapeHtml, openPrintDocument } from '../utils/reporting'
import { externalChartTooltipHandler } from '../utils/chartTooltip'
import { getThemeColors, hexToComplementaryHsla, hexToRgba } from '../utils/chartColors'
import { Bar } from 'vue-chartjs'
import { usePhoneBreakpoint } from '../utils/breakpoints'
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

const mobileBarEndLabelsPlugin = {
  id: 'mobileBarEndLabels',
  afterDatasetsDraw(chart: any) {
    const { ctx, chartArea } = chart
    if (!chartArea || chart.options.indexAxis !== 'y') return
    const custom = chart.options.plugins?.mobileBarEndLabels
    if (!custom?.enabled) return
    const meta = chart.getDatasetMeta(0)
    if (!meta?.data?.length) return

    ctx.save()
    ctx.font = '600 11px Inter, sans-serif'
    ctx.textAlign = 'left'
    ctx.textBaseline = 'middle'
    ctx.fillStyle = isLightMode.value ? '#334155' : '#e2e8f0'

    meta.data.forEach((bar: any, index: number) => {
      const value = chart.data.datasets[0]?.data?.[index]
      if (typeof value !== 'number' || value <= 0) return
      const label = custom.kind === 'hours'
        ? `${value} sales`
        : formatMoney(value, props.currency)
      ctx.fillText(label, Math.min(bar.x + 8, chartArea.right - 2), bar.y)
    })
    ctx.restore()
  },
}

ChartJS.register(mobileBarEndLabelsPlugin as any)

type Period = 'Daily' | 'Weekly' | 'Monthly'

type ProfitableProduct = {
  name: string
  profit: number
}

const { isPhone } = usePhoneBreakpoint()

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
const hourIntervals = [
  { axisLabel: '6 AM', range: '6:00 AM – 8:00 AM', shortRange: '6–8 AM' },
  { axisLabel: '8 AM', range: '8:00 AM – 10:00 AM', shortRange: '8–10 AM' },
  { axisLabel: '10 AM', range: '10:00 AM – 12:00 PM', shortRange: '10–12 PM' },
  { axisLabel: '12 PM', range: '12:00 PM – 2:00 PM', shortRange: '12–2 PM' },
  { axisLabel: '2 PM', range: '2:00 PM – 4:00 PM', shortRange: '2–4 PM' },
  { axisLabel: '4 PM', range: '4:00 PM – 6:00 PM', shortRange: '4–6 PM' },
  { axisLabel: '6 PM', range: '6:00 PM – 8:00 PM', shortRange: '6–8 PM' },
  { axisLabel: '8 PM', range: '8:00 PM – 10:00 PM', shortRange: '8–10 PM' },
  { axisLabel: '10 PM', range: '10:00 PM – 12:00 AM', shortRange: '10 PM–12 AM' },
  { axisLabel: '12 AM', range: '12:00 AM – 2:00 AM', shortRange: '12–2 AM' },
  { axisLabel: '2 AM', range: '2:00 AM – 4:00 AM', shortRange: '2–4 AM' },
  { axisLabel: '4 AM', range: '4:00 AM – 6:00 AM', shortRange: '4–6 AM' },
]

const selectedPeriod = ref<Period>('Daily')
const selectedDate = ref('')
const insights = ref<BusinessIntelligence | null>(null)
const loading = ref(true)
const error = ref('')
const isExportDropdownOpen = ref(false)

const hourBars = computed(() => {
  const counts = insights.value?.peakSalesHours ?? Array.from({ length: 12 }, () => 0)
  return hourIntervals.map((interval, index) => ({
    label: interval.axisLabel,
    range: interval.range,
    shortRange: interval.shortRange,
    count: counts[index] ?? 0,
  }))
})

const peakHourIndex = computed(() => {
  let bestIdx = -1
  let maxCount = 0
  hourBars.value.forEach((item, idx) => {
    if (item.count > maxCount) {
      maxCount = item.count
      bestIdx = idx
    }
  })
  return bestIdx
})

const peakHourInfo = computed(() => {
  if (peakHourIndex.value === -1) return null
  return hourBars.value[peakHourIndex.value]
})

const profitableProducts = computed(() => insights.value?.mostProfitableProducts ?? [])

const topProfitableProducts = computed(() => {
  return profitableProducts.value
    .slice()
    .sort((a, b) => b.profit - a.profit)
    .slice(0, 8)
})

const basketPairs = computed(() => insights.value?.marketBasket ?? [])

const maxBasketFrequency = computed(() => {
  if (!basketPairs.value.length) return 1
  return Math.max(...basketPairs.value.map(item => item.frequency), 1)
})

const mobilePeakHours = computed(() =>
  hourBars.value
    .map((item, index) => ({ ...item, index }))
    .filter(item => item.count > 0)
    .sort((a, b) => b.count - a.count)
    .slice(0, 6),
)

const mobileProfitableProducts = computed(() =>
  profitableProducts.value
    .slice()
    .sort((a, b) => b.profit - a.profit)
    .slice(0, 5),
)

const themeVersion = ref(0)
function handlePrefsUpdated() {
  themeVersion.value++
}

const mobileProductMax = computed(() =>
  Math.max(...mobileProfitableProducts.value.map(item => Math.max(item.profit, 0)), 0),
)

const isLightMode = computed(() => document.documentElement.getAttribute('data-theme') === 'light')

const salesByHourData = computed(() => {
  void themeVersion.value
  const { accent, complement } = getThemeColors()
  return {
    labels: hourIntervals.map(i => i.axisLabel),
    datasets: [
      {
        label: 'Sales Count',
        backgroundColor: (context: any) => {
          const chart = context.chart
          const { ctx, chartArea } = chart
          if (!chartArea) return accent
          const count = context.raw ?? 0
          const isPeak = context.dataIndex === peakHourIndex.value && count > 0
          if (isPeak) {
            // Highlight peak rush hour using complementary color
            const gradient = ctx.createLinearGradient(0, chartArea.bottom, 0, chartArea.top)
            gradient.addColorStop(0, hexToRgba(complement, 0.4))
            gradient.addColorStop(1, complement)
            return gradient
          }
          const gradient = ctx.createLinearGradient(0, chartArea.bottom, 0, chartArea.top)
          gradient.addColorStop(0, hexToRgba(accent, 0.25))
          gradient.addColorStop(1, accent)
          return gradient
        },
        hoverBackgroundColor: (context: any) => {
          const count = context.raw ?? 0
          return context.dataIndex === peakHourIndex.value && count > 0 ? complement : accent
        },
        borderColor: (context: any) => {
          const count = context.raw ?? 0
          return context.dataIndex === peakHourIndex.value && count > 0 ? complement : accent
        },
        borderWidth: 1,
        borderRadius: { topLeft: 6, topRight: 6 },
        borderSkipped: 'bottom' as const,
        barPercentage: 0.6,
        categoryPercentage: 0.75,
        data: hourBars.value.map(item => item.count),
      },
    ],
  }
})

const mostProfitableProductsData = computed(() => {
  void themeVersion.value
  const { accent } = getThemeColors()
  const list = topProfitableProducts.value
  const profits = list.map(p => p.profit)
  const maxVal = Math.max(...profits, 1)

  return {
    labels: list.map(p => p.name),
    datasets: [
      {
        label: 'Total Profit',
        backgroundColor: (context: any) => {
          const val = Number(context.raw ?? 0)
          const opacity = Math.min(1, Math.max(0.3, 0.25 + 0.75 * (val / maxVal)))
          return hexToComplementaryHsla(accent, opacity)
        },
        hoverBackgroundColor: hexToComplementaryHsla(accent, 0.95),
        borderColor: hexToComplementaryHsla(accent, 1),
        borderWidth: 1,
        borderRadius: { topRight: 6, bottomRight: 6 },
        borderSkipped: 'left' as const,
        barPercentage: 0.65,
        categoryPercentage: 0.8,
        data: profits,
      },
    ],
  }
})

const salesCountChartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  layout: {
    padding: {
      top: 10,
      bottom: 4,
      left: 4,
      right: 4,
    },
  },
  plugins: {
    legend: { display: false },
    tooltip: {
      enabled: false,
      external: externalChartTooltipHandler,
      callbacks: {
        title: (items: any) => {
          const idx = items[0]?.dataIndex ?? 0
          return hourIntervals[idx]?.range ?? items[0]?.label
        },
        label: (context: any) => {
          const count = Number(context.parsed?.y ?? 0)
          const isPeak = context.dataIndex === peakHourIndex.value && count > 0
          return `Sales: ${count.toLocaleString()} transactions${isPeak ? ' (🔥 Peak Rush)' : ''}`
        },
      },
    },
  },
  scales: {
    x: {
      grid: { display: false },
      border: { display: false },
      ticks: {
        color: isLightMode.value ? '#64748b' : '#94a3b8',
        font: { size: 10.5, weight: 'normal' as const },
        maxRotation: 0,
        minRotation: 0,
        autoSkip: true,
        maxTicksLimit: 12,
        padding: 8,
      },
    },
    y: {
      beginAtZero: true,
      grid: {
        color: isLightMode.value ? 'rgba(0, 0, 0, 0.05)' : 'rgba(255, 255, 255, 0.05)',
        borderDash: [3, 4],
        drawTicks: false,
      },
      border: { display: false },
      ticks: {
        color: isLightMode.value ? '#64748b' : '#94a3b8',
        font: { size: 11 },
        padding: 6,
        callback: (value: number | string) => {
          const num = Number(value)
          if (num % 1 !== 0) return ''
          return `${num}`
        },
      },
    },
  },
}))

const profitBarChartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  indexAxis: 'y' as const,
  plugins: {
    legend: { display: false },
    tooltip: {
      enabled: false,
      external: externalChartTooltipHandler,
      callbacks: {
        label: (context: any) => `Profit: ${props.currency} ${(context.parsed?.x ?? 0).toLocaleString()}`,
      },
    },
  },
  scales: {
    y: {
      grid: { display: false },
      border: { display: false },
      ticks: {
        color: isLightMode.value ? '#334155' : '#cbd5e1',
        font: { size: 11, weight: 500 },
        autoSkip: false,
      },
    },
    x: {
      beginAtZero: true,
      grid: {
        color: isLightMode.value ? 'rgba(0, 0, 0, 0.05)' : 'rgba(255, 255, 255, 0.05)',
        borderDash: [3, 4],
        drawTicks: false,
      },
      border: { display: false },
      ticks: {
        color: isLightMode.value ? '#64748b' : '#94a3b8',
        font: { size: 10 },
        callback: (value: number | string) => {
          const num = Number(value)
          if (num >= 1000000) return `${props.currency} ${num / 1000000}M`
          if (num >= 1000) return `${props.currency} ${num / 1000}k`
          return `${num}`
        },
      },
    },
  },
}))

const mobileSalesByHourData = computed(() => {
  void themeVersion.value
  const { accent } = getThemeColors()
  return {
    labels: mobilePeakHours.value.map(item => item.shortRange || item.label),
    datasets: [{
      label: 'Sales',
      data: mobilePeakHours.value.map(item => item.count),
      backgroundColor: accent,
      borderColor: accent,
      borderRadius: 4,
      borderSkipped: false,
      barThickness: 22,
      categoryPercentage: 0.62,
      barPercentage: 0.62,
    }],
  }
})

const mobileProfitableProductsData = computed(() => {
  void themeVersion.value
  const { accent } = getThemeColors()
  const list = mobileProfitableProducts.value
  const profits = list.map(item => item.profit)
  const maxVal = Math.max(...profits, 1)

  return {
    labels: list.map(item => item.name),
    datasets: [{
      label: 'Profit',
      data: profits,
      backgroundColor: profits.map(val => {
        const opacity = Math.min(1, Math.max(0.35, 0.25 + 0.75 * (val / maxVal)))
        return hexToComplementaryHsla(accent, opacity)
      }),
      borderColor: hexToComplementaryHsla(accent, 1),
      borderRadius: 4,
      borderSkipped: false,
      barThickness: 22,
      categoryPercentage: 0.62,
      barPercentage: 0.62,
    }],
  }
})

const mobileHorizontalChartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  indexAxis: 'y' as const,
  layout: { padding: { left: 8, right: 58, top: 2, bottom: 2 } },
  interaction: {
    mode: 'nearest' as const,
    intersect: true,
  },
  plugins: {
    legend: { display: false },
    tooltip: {
      enabled: true,
      displayColors: false,
    },
    mobileBarEndLabels: { enabled: true, kind: 'profit' },
  },
  scales: {
    x: {
      beginAtZero: true,
      grid: { display: false },
      border: { display: false },
      ticks: {
        maxTicksLimit: 4,
        color: isLightMode.value ? '#0f172a' : '#94a3b8',
        font: { size: 10 },
      },
    },
    y: {
      grid: { display: false },
      border: { display: false },
      ticks: {
        color: isLightMode.value ? '#0f172a' : '#94a3b8',
        font: { size: 10 },
        autoSkip: false,
      },
    },
  },
}))

const mobilePeakChartOptions = computed(() => {
  const options: any = mobileHorizontalChartOptions.value
  return {
    ...options,
    plugins: {
      ...options.plugins,
      mobileBarEndLabels: { enabled: true, kind: 'hours' },
    },
  }
})

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
    ...hourBars.value.map(item => [item.range || item.label, item.count, selectedPeriod.value]),
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
    <tr><td>${escapeHtml(item.range || item.label)}</td><td class="num">${item.count}</td></tr>
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
  <section class="bi-section">
    <div v-if="error" class="inline-error" role="alert">{{ error }}</div>

    <!-- Header Controls -->
    <div class="section-header">
      <div class="header-left-title">
        <h2 class="section-title">Business Intelligence Overview</h2>
        <p class="section-subtitle">Cross-sectional transaction density, margin leadership & affinity analytics</p>
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


    <!-- Main Grid -->
    <div class="bi-grid">
      <!-- Peak Sales Hours -->
      <div class="card bi-chart-card">
        <div class="card-header">
          <div class="header-left">
            <div class="card-icon-box icon-cyan">
              <Clock :size="18" />
            </div>
            <div>
              <div class="card-title-row">
                <h3 class="card-title">Peak Sales Hours</h3>
                <span v-if="peakHourInfo && peakHourInfo.count > 0" class="peak-badge">
                  <span class="peak-dot"></span> Peak: {{ peakHourInfo.shortRange }}
                </span>
              </div>
              <p class="card-subtitle">Transaction distribution across 2-hour intervals</p>
            </div>
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

        <div class="chart-container" style="position: relative; height: 100%; min-height: 250px;">
          <Bar v-if="!isPhone" :data="salesByHourData" :options="salesCountChartOptions" />
          <Bar
            v-else-if="mobilePeakHours.length"
            :data="mobileSalesByHourData"
            :options="mobilePeakChartOptions"
          />
          <div v-else class="empty-chart-state">No peak sales hours recorded for this period.</div>
        </div>
      </div>

      <!-- Most Profitable Products (Horizontal Bar Chart) -->
      <div class="card bi-chart-card">
        <div class="card-header">
          <div class="header-left">
            <div class="card-icon-box icon-green">
              <Award :size="18" />
            </div>
            <div>
              <h3 class="card-title">Most Profitable Products</h3>
              <p class="card-subtitle">Margin leadership ranked across catalog</p>
            </div>
          </div>
          <span v-if="topProfitableProducts.length" class="bi-pill-badge">
            Top {{ topProfitableProducts.length }} Leaders
          </span>
        </div>

        <div class="chart-container" style="position: relative; height: 100%; min-height: 250px;">
          <div v-if="isPhone && !mobileProfitableProducts.length" class="empty-chart-state">
            No profitable product sales recorded for this period.
          </div>
          <div v-else-if="isPhone" class="mobile-profit-bar-list">
            <div
              v-for="item in mobileProfitableProducts"
              :key="item.name"
              class="mobile-profit-bar-row"
            >
              <div class="mobile-profit-bar-label">
                <span>{{ item.name }}</span>
                <strong>{{ formatMoney(item.profit, props.currency) }}</strong>
              </div>
              <div class="mobile-profit-bar-track">
                <div
                  class="mobile-profit-bar-fill"
                  :style="{ width: `${mobileProductMax > 0 ? Math.max(3, (item.profit / mobileProductMax) * 100) : 0}%` }"
                ></div>
              </div>
            </div>
          </div>
          <div v-else-if="!topProfitableProducts.length" class="empty-chart-state">
            No profitable product sales recorded for this period.
          </div>
          <Bar v-else :data="mostProfitableProductsData" :options="profitBarChartOptions" />
        </div>
      </div>

      <!-- Frequently Bought Together (Market Basket) -->
      <div v-if="!isPhone" class="card full-width market-basket-card">
        <div class="card-header">
          <div class="header-left">
            <div class="card-icon-box icon-purple">
              <ShoppingBag :size="18" />
            </div>
            <div>
              <h3 class="card-title">Frequently Bought Together (Market Basket)</h3>
              <p class="card-subtitle">Product affinity pairings discovered through multi-item transaction co-occurrence</p>
            </div>
          </div>
          <span v-if="basketPairs.length" class="bi-pill-badge">
            {{ basketPairs.length }} High Affinity Pairs
          </span>
        </div>

        <div class="basket-table-wrapper">
          <div v-if="!basketPairs.length" class="empty-basket-state">
            <ShoppingBag :size="32" class="empty-icon" />
            <p class="empty-title">Not enough multi-item transaction data yet</p>
            <span class="empty-sub">Affinity pairings appear automatically when customers purchase 2 or more items in the same sale.</span>
          </div>

          <div v-else class="basket-pairs-list">
            <div
              v-for="(item, index) in basketPairs"
              :key="index"
              class="basket-pair-row"
            >
              <div class="basket-rank">
                <span class="rank-badge" :class="`rank-${index + 1}`">#{{ index + 1 }}</span>
              </div>

              <div class="basket-combo-col">
                <span class="product-item-badge">{{ item.pair[0] }}</span>
                <div class="combo-connector">
                  <Plus :size="13" />
                </div>
                <span class="product-item-badge">{{ item.pair[1] }}</span>
              </div>

              <div class="basket-strength-col">
                <div class="strength-track">
                  <div
                    class="strength-fill"
                    :style="{ width: `${(item.frequency / maxBasketFrequency) * 100}%` }"
                  />
                </div>
              </div>

              <div class="basket-frequency-col">
                <span class="frequency-pill">
                  <strong>{{ item.frequency }}</strong> co-purchases
                </span>
              </div>

              <div class="basket-action-col">
                <span class="affinity-tag" :class="index < 2 ? 'tag-strong' : 'tag-moderate'">
                  {{ index < 2 ? 'Strong Affinity' : 'Frequent Combo' }}
                </span>
              </div>
            </div>
          </div>
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
  gap: 12px;
}

/* ==========================================================================
   Header Row
   ========================================================================== */
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.75rem;
  flex-shrink: 0;
}

.header-left-title {
  display: flex;
  flex-direction: column;
}

.section-title {
  margin: 0;
  font-size: 1.2rem;
  font-weight: 700;
  color: var(--text-primary);
  letter-spacing: -0.01em;
}

.section-subtitle {
  margin: 2px 0 0;
  font-size: 0.775rem;
  color: var(--text-muted);
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
  top: calc(100% + 0.4rem);
  right: 0;
  width: 220px;
  background-color: var(--bg-sidebar);
  border: 1px solid var(--border-color);
  border-radius: 10px;
  box-shadow: 0 12px 30px rgba(0, 0, 0, 0.5);
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
  border-radius: 6px;
  background: transparent;
  border: none;
  color: var(--text-primary);
  cursor: pointer;
  text-align: left;
  width: 100%;
  transition: background-color 0.15s ease;
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
  font-size: 0.825rem;
  font-weight: 600;
  color: var(--text-primary);
}

.item-desc {
  font-size: 0.7rem;
  color: var(--text-muted);
}

.card-title-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.peak-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 0.15rem 0.55rem;
  font-size: 0.7rem;
  font-weight: 700;
  border-radius: 9999px;
  background: rgba(245, 158, 11, 0.12);
  color: #f59e0b;
  border: 1px solid rgba(245, 158, 11, 0.25);
  letter-spacing: 0.02em;
}

.peak-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background-color: #f59e0b;
  box-shadow: 0 0 6px rgba(245, 158, 11, 0.6);
}

.icon-cyan {
  background-color: rgba(14, 165, 233, 0.12);
  color: #38bdf8;
}

.icon-green {
  background-color: rgba(16, 185, 129, 0.12);
  color: #10b981;
}

.icon-purple {
  background-color: rgba(168, 85, 247, 0.12);
  color: #c084fc;
}

/* ==========================================================================
   BI Grid
   ========================================================================== */
.bi-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  grid-template-rows: minmax(280px, 1.2fr) minmax(220px, 1fr);
  gap: 12px;
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

.card-icon-box {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.card-title {
  margin: 0;
  font-size: 0.975rem;
  font-weight: 700;
  color: var(--text-primary);
}

.card-subtitle {
  margin: 1px 0 0;
  font-size: 0.725rem;
  color: var(--text-muted);
}

.bi-pill-badge {
  font-size: 0.725rem;
  font-weight: 600;
  padding: 0.2rem 0.55rem;
  border-radius: 6px;
  background: rgba(14, 165, 233, 0.1);
  color: #38bdf8;
  border: 1px solid rgba(14, 165, 233, 0.2);
}

.bi-controls-right {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.time-range-selector {
  display: flex;
  background-color: var(--bg-sidebar);
  border-radius: 6px;
  padding: 0.2rem;
  border: 1px solid var(--border-subtle);
}

.time-range-selector button {
  background: transparent;
  border: none;
  color: var(--text-muted);
  padding: 0.25rem 0.65rem;
  border-radius: 4px;
  font-weight: 600;
  font-size: 0.775rem;
  cursor: pointer;
  transition: all 0.15s ease;
}

.time-range-selector button:hover {
  color: var(--text-primary);
}

.time-range-selector button.active {
  background-color: var(--bg-surface);
  color: #38bdf8;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
}

.date-filter-wrapper {
  display: flex;
  align-items: center;
  gap: 0.2rem;
}

.filter-btn {
  background-color: var(--bg-sidebar);
  color: var(--text-secondary);
  padding: 0.4rem;
  border-radius: 6px;
  border: 1px solid var(--border-subtle);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
  overflow: hidden;
  transition: all 0.15s ease;
}

.filter-btn:hover {
  color: var(--text-primary);
  border-color: var(--border-color);
}

.date-picker-btn.active-filter {
  border-color: rgba(14, 165, 233, 0.5);
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
  border-radius: 6px;
  padding: 0.4rem;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
}

.clear-date-btn:hover {
  background-color: #ef4444;
  color: #ffffff;
}

.chart-container {
  flex: 1;
  min-height: 200px;
  position: relative;
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
   Market Basket Pairs List
   ========================================================================== */
.market-basket-card {
  display: flex;
  flex-direction: column;
}

.basket-table-wrapper {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.empty-basket-state {
  padding: 2.5rem;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
}

.empty-icon {
  margin-bottom: 0.75rem;
  opacity: 0.4;
}

.empty-title {
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 0.25rem;
}

.empty-sub {
  font-size: 0.75rem;
  color: var(--text-muted);
}

.basket-pairs-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.basket-pair-row {
  display: grid;
  grid-template-columns: 44px minmax(280px, 1.8fr) minmax(120px, 1fr) 140px 140px;
  align-items: center;
  gap: 1rem;
  padding: 0.55rem 0.85rem;
  border-radius: 8px;
  background: rgba(148, 163, 184, 0.03);
  border: 1px solid var(--border-subtle);
  transition: all 0.15s ease;
}

.basket-pair-row:hover {
  background: rgba(148, 163, 184, 0.07);
  border-color: rgba(148, 163, 184, 0.2);
}

.basket-rank {
  display: flex;
  align-items: center;
  justify-content: center;
}

.rank-badge {
  font-size: 0.725rem;
  font-weight: 700;
  padding: 0.15rem 0.45rem;
  border-radius: 4px;
  background: rgba(148, 163, 184, 0.1);
  color: var(--text-muted);
}

.rank-badge.rank-1 {
  background: rgba(245, 158, 11, 0.18);
  color: #f59e0b;
  border: 1px solid rgba(245, 158, 11, 0.3);
}

.rank-badge.rank-2 {
  background: rgba(168, 85, 247, 0.18);
  color: #c084fc;
  border: 1px solid rgba(168, 85, 247, 0.3);
}

.rank-badge.rank-3 {
  background: rgba(14, 165, 233, 0.18);
  color: #38bdf8;
  border: 1px solid rgba(14, 165, 233, 0.3);
}

.basket-combo-col {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  min-width: 0;
  overflow: hidden;
}

.product-item-badge {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-primary);
  background: var(--bg-surface);
  border: 1px solid var(--border-color);
  padding: 0.25rem 0.65rem;
  border-radius: 6px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.combo-connector {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: rgba(168, 85, 247, 0.15);
  color: #c084fc;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.basket-strength-col {
  display: flex;
  align-items: center;
}

.strength-track {
  width: 100%;
  height: 6px;
  border-radius: 3px;
  background: rgba(148, 163, 184, 0.12);
  overflow: hidden;
}

.strength-fill {
  height: 100%;
  border-radius: 3px;
  background: linear-gradient(90deg, #0ea5e9, #a855f7);
  transition: width 0.5s ease;
}

.basket-frequency-col {
  text-align: center;
}

.frequency-pill {
  font-size: 0.775rem;
  color: var(--text-secondary);
  background: var(--bg-surface);
  padding: 0.2rem 0.6rem;
  border-radius: 5px;
  border: 1px solid var(--border-subtle);
  display: inline-block;
}

.frequency-pill strong {
  color: var(--text-primary);
  font-weight: 700;
}

.basket-action-col {
  text-align: right;
}

.affinity-tag {
  font-size: 0.725rem;
  font-weight: 600;
  padding: 0.2rem 0.5rem;
  border-radius: 4px;
  display: inline-block;
}

.tag-strong {
  background: rgba(16, 185, 129, 0.12);
  color: #10b981;
  border: 1px solid rgba(16, 185, 129, 0.2);
}

.tag-moderate {
  background: rgba(14, 165, 233, 0.1);
  color: #38bdf8;
  border: 1px solid rgba(14, 165, 233, 0.2);
}

/* ==========================================================================
   Responsive Breakpoints
   ========================================================================== */
@media (max-width: 1024px) {
  .bi-grid {
    grid-template-columns: 1fr;
    grid-template-rows: auto;
  }

  .basket-pair-row {
    grid-template-columns: 36px 1fr auto;
    gap: 0.5rem;
  }

  .basket-strength-col,
  .basket-action-col {
    display: none;
  }
}

@media (max-width: 639px) {
  .card-header {
    align-items: flex-start;
    flex-wrap: wrap;
    gap: 8px;
  }

  .bi-controls-right {
    width: 100%;
    align-items: flex-start;
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
    width: 40px;
    height: 40px;
  }

  .mobile-profit-bar-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    width: 100%;
    min-width: 0;
  }

  .mobile-profit-bar-row {
    min-width: 0;
  }

  .mobile-profit-bar-label {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 8px;
    min-width: 0;
    margin-bottom: 5px;
    font-size: 0.78rem;
  }

  .mobile-profit-bar-label span {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .mobile-profit-bar-label strong {
    flex: 0 0 auto;
    font-size: 0.75rem;
    white-space: nowrap;
  }

  .mobile-profit-bar-track {
    height: 22px;
    width: 100%;
    overflow: hidden;
    border-radius: 5px;
    background: rgba(148, 163, 184, 0.12);
  }

  .mobile-profit-bar-fill {
    height: 100%;
    max-width: 100%;
    border-radius: 5px;
    background: var(--accent-color, #0ea5e9);
  }
}
</style>
