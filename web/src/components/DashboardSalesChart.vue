<script setup lang="ts">
import { computed } from 'vue'
import { externalChartTooltipHandler } from '../utils/chartTooltip'
import { getThemeColors, hexToRgba } from '../utils/chartColors'
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
        maxTicksLimit: props.isPhone ? 8 : 12,
        maxRotation: 0,
        minRotation: 0,
        font: {
          size: props.isPhone ? 10 : 11,
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
          size: props.isPhone ? 10 : 11,
        },
        padding: props.isPhone ? 4 : 8,
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
      radius: (ctx) => (Number(ctx.raw) > 0 ? (props.isPhone ? 2.5 : 3) : 0),
      hoverRadius: props.isPhone ? 5 : 6,
      hoverBackgroundColor: '#ffffff',
      hoverBorderWidth: 2,
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
      const border = String(lineDataset.borderColor || '').toLowerCase()
      const { complement } = getThemeColors()

      // If this dataset represents the sales revenue line/fill, use complementary color with alpha gradient
      const isProfit = border.includes('10b981') || border.includes('16, 185, 129')
      if (isProfit) {
        gradient.addColorStop(0, hexToRgba('#10b981', 0.38))
        gradient.addColorStop(0.5, hexToRgba('#10b981', 0.1))
        gradient.addColorStop(1, hexToRgba('#10b981', 0.0))
      } else {
        // Use complementary color for the gradient fill matching mother repo
        gradient.addColorStop(0, hexToRgba(complement, 0.4))
        gradient.addColorStop(0.5, hexToRgba(complement, 0.1))
        gradient.addColorStop(1, hexToRgba(complement, 0.0))
      }
      lineDataset.backgroundColor = gradient
    }
  },
}
</script>

<template>
  <Chart
    type="bar"
    :data="props.chartData"
    :options="chartOptions"
    :plugins="[gradientPlugin]"
  />
</template>
