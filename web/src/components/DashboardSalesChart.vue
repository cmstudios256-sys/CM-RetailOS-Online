<script setup lang="ts">
import { computed } from 'vue'
import { externalChartTooltipHandler } from '../utils/chartTooltip'
import { Chart } from 'vue-chartjs'
import {
  Chart as ChartJS,
  Title,
  Tooltip,
  Legend,
  LineElement,
  CategoryScale,
  LinearScale,
  PointElement,
  BarElement,
  Filler,
  type Chart as ChartJsChart,
  type ChartData,
  type ChartOptions,
} from 'chart.js'

ChartJS.register(
  Title,
  Tooltip,
  Legend,
  LineElement,
  CategoryScale,
  LinearScale,
  PointElement,
  BarElement,
  Filler,
)

const props = defineProps<{
  chartData: ChartData<'line' | 'bar', number[], string>
  currencySymbol?: string
  isPhone?: boolean
}>()

const isLightMode = computed(() => document.documentElement.getAttribute('data-theme') === 'light')
const cur = computed(() => props.currencySymbol || 'UGX')

const chartOptions = computed<ChartOptions<'line' | 'bar'>>(() => ({
  responsive: true,
  maintainAspectRatio: false,
  layout: {
    padding: {
      top: 6,
      bottom: 6,
      left: 0,
      right: 0,
    },
  },
  interaction: {
    mode: 'index',
    intersect: false,
  },
  plugins: {
    legend: {
      display: false,
    },
    tooltip: {
      enabled: false,
      external: externalChartTooltipHandler,
      callbacks: {
        label: (context) => {
          const label = context.dataset.label || ''
          return `${label}: ${cur.value} ${(context.parsed?.y ?? 0).toLocaleString()}`
        },
      },
    },
  },
  scales: {
    x: {
      grid: {
        display: false,
      },
      ticks: {
        color: isLightMode.value ? '#64748b' : '#94a3b8',
        autoSkip: true,
        maxTicksLimit: 12,
        maxRotation: 0,
        minRotation: 0,
        font: {
          size: 11,
          weight: 'normal',
        },
      },
      border: {
        display: false,
      },
    },
    y: {
      beginAtZero: true,
      grid: {
        color: isLightMode.value ? 'rgba(0, 0, 0, 0.05)' : 'rgba(255, 255, 255, 0.05)',
        borderDash: [3, 4],
        drawTicks: false,
      },
      ticks: {
        color: isLightMode.value ? '#64748b' : '#94a3b8',
        font: {
          size: 11,
        },
        padding: 8,
        callback: (value) => {
          if (typeof value === 'number') {
            if (value === 0) return '0'
            if (value >= 1000000) return `${value / 1000000}M`
            if (value >= 1000) return `${value / 1000}k`
            return `${value}`
          }
          return value
        },
      },
      border: {
        display: false,
      },
    },
  },
  elements: {
    line: {
      tension: 0.3,
    },
    point: {
      radius: (ctx) => (Number(ctx.raw) > 0 ? 3 : 0),
      hoverRadius: 6,
      hoverBackgroundColor: '#ffffff',
      hoverBorderWidth: 2,
    },
  },
}))

const mobileChartData = computed<ChartData<'bar', number[], string>>(() => {
  const source = props.chartData.labels?.slice(-7) ?? []
  const salesDataset = props.chartData.datasets.find(dataset => dataset.label?.toLowerCase().includes('sales'))
  const sourceData = salesDataset?.data
  const labels = source.map(label => String(label))
  const values = Array.isArray(sourceData)
    ? sourceData.slice(-7).map(value => Number(value) || 0)
    : []
  const accent = getComputedStyle(document.documentElement).getPropertyValue('--accent-color').trim() || '#0ea5e9'

  return {
    labels,
    datasets: [{
      label: 'Sales',
      backgroundColor: accent,
      borderColor: accent,
      borderRadius: 4,
      borderSkipped: false,
      data: values,
      barPercentage: 0.72,
      categoryPercentage: 0.72,
    }],
  }
})

const hasMobileSales = computed(() =>
  mobileChartData.value.datasets.some(dataset =>
    dataset.data.some(value => Number(value) > 0),
  ),
)

const mobileChartOptions = computed<ChartOptions<'bar'>>(() => ({
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
        label: context => `${cur.value} ${Number(context.parsed.y || 0).toLocaleString()}`,
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
        color: isLightMode.value ? 'rgba(15, 23, 42, 0.06)' : 'rgba(148, 163, 184, 0.08)',
        drawTicks: false,
      },
      border: { display: false },
      ticks: {
        display: true,
        color: isLightMode.value ? '#0f172a' : '#94a3b8',
        maxTicksLimit: 4,
        font: { size: 10 },
        callback: value => {
          const amount = Number(value)
          if (amount >= 1000000) return `${amount / 1000000}M`
          if (amount >= 1000) return `${amount / 1000}k`
          return `${amount}`
        },
      },
    },
  },
}))

const gradientPlugin = {
  id: 'gradient',
  beforeRender(chart: ChartJsChart) {
    const lineDataset = chart.data.datasets.find(d => d.type === 'line' || (d as any).fill)
    if (lineDataset) {
      const ctx = chart.ctx
      const chartArea = chart.chartArea
      if (!chartArea) return
      const gradient = ctx.createLinearGradient(0, chartArea.top, 0, chartArea.bottom)
      const border = String(lineDataset.borderColor || '#0ea5e9').toLowerCase()
      const isEmerald = border.includes('10b981') || border.includes('16, 185, 129')

      if (isEmerald) {
        gradient.addColorStop(0, 'rgba(16, 185, 129, 0.38)')
        gradient.addColorStop(0.5, 'rgba(16, 185, 129, 0.1)')
        gradient.addColorStop(1, 'rgba(16, 185, 129, 0.0)')
      } else {
        gradient.addColorStop(0, 'rgba(14, 165, 233, 0.38)')
        gradient.addColorStop(0.5, 'rgba(14, 165, 233, 0.1)')
        gradient.addColorStop(1, 'rgba(14, 165, 233, 0.0)')
      }
      lineDataset.backgroundColor = gradient
    }
  },
}
</script>

<template>
  <div v-if="props.isPhone && !hasMobileSales" class="empty-chart-state">
    No sales in this period
  </div>
  <Chart
    v-else-if="props.isPhone"
    type="bar"
    :data="mobileChartData as any"
    :options="mobileChartOptions as any"
  />
  <Chart
    v-else
    type="bar"
    :data="props.chartData"
    :options="chartOptions"
    :plugins="[gradientPlugin]"
  />
</template>
