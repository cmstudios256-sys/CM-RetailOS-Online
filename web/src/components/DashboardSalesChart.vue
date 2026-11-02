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
      bottom: props.isPhone ? 10 : 110,
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
        color: isLightMode.value ? '#0f172a' : '#94a3b8',
        autoSkip: true,
        maxTicksLimit: 12,
        maxRotation: 0,
        minRotation: 0,
        font: {
          size: 11,
        },
      },
      border: {
        color: isLightMode.value ? '#cbd5e1' : '#334155',
      },
    },
    y: {
      beginAtZero: true,
      grid: {
        color: isLightMode.value ? 'rgba(0, 0, 0, 0.08)' : 'rgba(255, 255, 255, 0.08)',
      },
      ticks: {
        color: isLightMode.value ? '#0f172a' : '#94a3b8',
        font: {
          size: 11,
        },
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
      tension: 0.4,
    },
    point: {
      radius: 0,
      hoverRadius: 5,
      hoverBackgroundColor: '#fff',
      hoverBorderColor: 'var(--accent-color, #0ea5e9)',
    },
  },
}))

const mobileChartData = computed<ChartData<'bar', number[], string>>(() => {
  const source = props.chartData.labels?.slice(-7) ?? []
  const salesDataset = props.chartData.datasets.find(dataset => dataset.label === 'Sales')
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
    if (chart.data.datasets[0]) {
      const dataset = chart.data.datasets[0]
      const ctx = chart.ctx
      const gradient = ctx.createLinearGradient(0, 0, 0, chart.height)
      const accent = getComputedStyle(document.documentElement).getPropertyValue('--accent-color').trim() || '#0ea5e9'

      const hexToRgba = (hex: string, alpha: number) => {
        let r = 14, g = 165, b = 233
        const clean = hex.startsWith('#') ? hex.slice(1) : hex
        if (clean.length === 3) {
          r = parseInt(clean[0] + clean[0], 16)
          g = parseInt(clean[1] + clean[1], 16)
          b = parseInt(clean[2] + clean[2], 16)
        } else if (clean.length === 6) {
          r = parseInt(clean.substring(0, 2), 16)
          g = parseInt(clean.substring(2, 4), 16)
          b = parseInt(clean.substring(4, 6), 16)
        }
        return `rgba(${r}, ${g}, ${b}, ${alpha})`
      }

      gradient.addColorStop(0, hexToRgba(accent, 0.35))
      gradient.addColorStop(1, hexToRgba(accent, 0))
      dataset.backgroundColor = gradient
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
