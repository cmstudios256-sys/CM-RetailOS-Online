<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import {
  Boxes,
  Calendar,
  ShoppingCart,
  TrendingUp,
  Wallet,
  X,
} from '@lucide/vue'
import { apiRequest, formatAxisValue, formatQuantity, type Product } from '../api'
import { getThemeColors, hexToRgba } from '../utils/chartColors'
import DashboardSalesChart from './DashboardSalesChart.vue'
import { usePhoneBreakpoint } from '../utils/breakpoints'

type Period = 'Daily' | 'Weekly' | 'Monthly'

type ChartPoint = {
  label: string
  sales: number
  profit: number
}

const { isPhone } = usePhoneBreakpoint()

type TopProduct = {
  name: string
  quantitySold: number
  revenue: number
}

type DashboardProduct = Pick<Product, 'id' | 'name' | 'unit' | 'quantity' | 'minStock'>
  & Partial<Pick<Product, 'costPrice'>>

type DashboardResponse = {
  productCount: number
  lowStockCount: number
  inventoryValue: number
  todaySales: number
  todayRevenue: number
  currency: string
  revenue: number
  profit: number
  previousRevenue: number
  previousProfit: number
  orderCount: number
  previousOrders: number
  unitsSold: number
  chart: ChartPoint[]
  topProducts: TopProduct[]
}

const props = defineProps<{ currency: string; role?: string }>()

const periods: Period[] = ['Daily', 'Weekly', 'Monthly']
const selectedPeriod = ref<Period>('Daily')
const reportDate = ref('')
const topSellingDate = ref('')
const rotatingDashboardCard = ref<'top-selling' | 'stock-alerts'>('top-selling')
const dashboard = ref<DashboardResponse | null>(null)
const products = ref<DashboardProduct[]>([])
const loading = ref(true)
const error = ref('')
const hoveredIndex = ref<number | null>(null)

// Restock Modal State
const isRestockModalOpen = ref(false)
const productToRestock = ref<DashboardProduct | null>(null)
const newQuantity = ref<number>(0)
const restocking = ref(false)
const restockError = ref('')
const themeVersion = ref(0)

function handlePrefsUpdated() {
  themeVersion.value++
}

const activeCurrency = computed(() => dashboard.value?.currency || props.currency || 'UGX')

const stockAlerts = computed(() =>
  products.value
    .filter(product => product.quantity <= product.minStock)
    .sort((first, second) => first.quantity - second.quantity)
    .map(product => ({
      product,
      name: product.name,
      quantity: product.quantity,
      status: product.quantity <= 0 ? ('out' as const) : ('low' as const),
    })),
)

const activeProductsCount = computed(() =>
  products.value.filter(product => product.quantity > 0).length,
)

function getTrend(current: number, previous: number) {
  if (previous === 0 && current > 0) return { trend: '+100.0%', class: 'trend-up' }
  if (previous === 0) return { trend: '+0.0%', class: 'trend-up' }
  const percent = ((current - previous) / Math.abs(previous)) * 100
  const sign = percent >= 0 ? '+' : ''
  return {
    trend: `${sign}${percent.toFixed(1)}%`,
    class: percent >= 0 ? 'trend-up' : 'trend-down',
  }
}

const statCards = computed(() => {
  const totalRevenue = dashboard.value?.revenue ?? 0
  const totalProfit = dashboard.value?.profit ?? 0
  const totalOrders = dashboard.value?.orderCount ?? 0
  const totalUnitsSold = Math.round(dashboard.value?.unitsSold ?? 0)
  const inventoryValue = dashboard.value?.inventoryValue
    ?? products.value.reduce((sum, p) => sum + (p.costPrice ?? 0) * p.quantity, 0)

  let timeLabel = 'Today'
  let vsLabel = 'yesterday'
  if (selectedPeriod.value === 'Weekly') {
    timeLabel = 'This Week'
    vsLabel = 'last week'
  } else if (selectedPeriod.value === 'Monthly') {
    timeLabel = 'This Month'
    vsLabel = 'last month'
  }

  const revTrend = getTrend(totalRevenue, dashboard.value?.previousRevenue ?? 0)
  const profTrend = getTrend(totalProfit, dashboard.value?.previousProfit ?? 0)
  const ordTrend = getTrend(totalOrders, dashboard.value?.previousOrders ?? 0)

  return [
    {
      label: `${timeLabel}'s Revenue`,
      isCurrency: true,
      numberPart: Math.round(totalRevenue).toLocaleString(),
      subValue: '',
      icon: Wallet,
      color: 'icon-success',
      trend: revTrend.trend,
      trendClass: revTrend.class,
      trendText: `vs ${vsLabel}`,
      sparkline: chartPoints.value.slice(-7).map(point => point.sales || 0),
    },
    {
      label: `${timeLabel}'s Profit`,
      isCurrency: true,
      numberPart: Math.round(totalProfit).toLocaleString(),
      subValue: '',
      icon: TrendingUp,
      color: 'icon-accent',
      trend: profTrend.trend,
      trendClass: profTrend.class,
      trendText: `vs ${vsLabel}`,
      sparkline: chartPoints.value.slice(-7).map(point => point.profit || 0),
    },
    {
      label: `Orders ${timeLabel}`,
      isCurrency: false,
      numberPart: Math.round(totalOrders).toLocaleString(),
      subValue: `(${totalUnitsSold.toLocaleString()} units)`,
      icon: ShoppingCart,
      color: 'icon-purple',
      trend: ordTrend.trend,
      trendClass: ordTrend.class,
      trendText: `vs ${vsLabel}`,
      sparkline: [] as number[],
    },
    {
      label: 'Total Inventory Value',
      isCurrency: true,
      numberPart: Math.round(inventoryValue).toLocaleString(),
      subValue: '',
      icon: Boxes,
      color: 'icon-warning',
      trend: `${activeProductsCount.value} active items`,
      trendClass: 'trend-warning',
      trendText: 'currently in stock',
      sparkline: [] as number[],
    },
  ]
})

const chartPoints = computed(() => dashboard.value?.chart ?? [])

type SalesChartMode = 'all' | 'sales' | 'profit'
const salesChartMode = ref<SalesChartMode>('all')

const salesSummaryStats = computed(() => {
  const data = chartPoints.value
  const totalSales = data.reduce((sum, item) => sum + (item.sales || 0), 0)
  const totalProfit = data.reduce((sum, item) => sum + (item.profit || 0), 0)
  const bestPoint = data.reduce<ChartPoint | null>(
    (best, item) => ((item.sales || 0) > (best?.sales || 0) ? item : best),
    null,
  )
  return [
    { label: 'Sales', value: `${activeCurrency.value} ${Math.round(totalSales).toLocaleString()}`, sub: 'Gross Revenue' },
    { label: 'Profit', value: `${activeCurrency.value} ${Math.round(totalProfit).toLocaleString()}`, sub: 'Direct Margin' },
    { label: 'Best', value: bestPoint && bestPoint.sales > 0 ? bestPoint.label : 'N/A', sub: bestPoint && bestPoint.sales > 0 ? `${activeCurrency.value} ${Math.round(bestPoint.sales).toLocaleString()}` : 'Peak Volume' },
  ]
})

const salesChartData = computed(() => {
  void themeVersion.value
  const { accent, complement } = getThemeColors()
  const data = chartPoints.value
  const labels = data.map(s => s.label)

  if (salesChartMode.value === 'sales') {
    return {
      labels,
      datasets: [
        {
          type: 'line' as const,
          label: 'Sales Revenue',
          borderColor: accent,
          borderWidth: 2.5,
          fill: true,
          data: data.map(s => s.sales),
          order: 1,
        },
      ],
    }
  }

  if (salesChartMode.value === 'profit') {
    return {
      labels,
      datasets: [
        {
          type: 'line' as const,
          label: 'Net Profit',
          borderColor: '#10b981',
          borderWidth: 2.5,
          fill: true,
          data: data.map(s => s.profit),
          order: 1,
        },
      ],
    }
  }

  // Combined: Sales line (accent) + Profit bars (complementary/accent tint matching mother repo)
  return {
    labels,
    datasets: [
      {
        type: 'line' as const,
        label: 'Sales Revenue',
        borderColor: accent,
        borderWidth: 2.5,
        fill: true,
        data: data.map(s => s.sales),
        order: 1,
      },
      {
        type: 'bar' as const,
        label: 'Net Profit',
        backgroundColor: hexToRgba(accent, 0.3),
        hoverBackgroundColor: accent,
        borderColor: hexToRgba(accent, 0.7),
        borderWidth: 1,
        borderRadius: { topLeft: 4, topRight: 4 },
        borderSkipped: 'bottom' as const,
        data: data.map(s => s.profit),
        order: 2,
        barPercentage: 0.55,
      },
    ],
  }
})

function formatCurrency(val: number) {
  return `${activeCurrency.value} ${Math.round(val || 0).toLocaleString()}`
}

function sparklinePoints(values: number[] = []) {
  if (!values.length) return ''
  const max = Math.max(...values, 1)
  const min = Math.min(...values, 0)
  const range = Math.max(max - min, 1)
  return values.map((value, index) => {
    const x = values.length === 1 ? 50 : (index / (values.length - 1)) * 100
    const y = 36 - ((value - min) / range) * 30
    return `${x.toFixed(1)},${y.toFixed(1)}`
  }).join(' ')
}

function toggleDashboardCard() {
  rotatingDashboardCard.value =
    rotatingDashboardCard.value === 'top-selling' ? 'stock-alerts' : 'top-selling'
}

function openRestockModal(product: DashboardProduct) {
  productToRestock.value = product
  newQuantity.value = product.quantity
  restockError.value = ''
  isRestockModalOpen.value = true
}

function closeRestockModal() {
  isRestockModalOpen.value = false
  productToRestock.value = null
  restockError.value = ''
}

async function handleRestock() {
  if (!productToRestock.value) return
  const targetQty = Number(newQuantity.value)
  if (!Number.isFinite(targetQty) || targetQty < 0) {
    restockError.value = 'Enter a valid non-negative quantity.'
    return
  }
  const delta = Math.round((targetQty - productToRestock.value.quantity) * 1000) / 1000
  if (delta === 0) {
    closeRestockModal()
    return
  }

  restocking.value = true
  restockError.value = ''
  try {
    await apiRequest(`/products/${productToRestock.value.id}/stock-adjustments`, {
      method: 'POST',
      body: {
        delta,
        reason: 'Dashboard stock adjustment',
      },
    })
    closeRestockModal()
    await load()
  } catch (reason) {
    restockError.value = reason instanceof Error ? reason.message : 'Failed to update stock quantity.'
  } finally {
    restocking.value = false
  }
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const query = new URLSearchParams({ period: selectedPeriod.value })
    if (reportDate.value) query.set('reportDate', reportDate.value)
    if (topSellingDate.value) query.set('date', topSellingDate.value)
    const [dashboardData, productResponse] = await Promise.all([
      apiRequest<DashboardResponse>(`/dashboard?${query.toString()}`),
      apiRequest<{ products: DashboardProduct[] }>('/products'),
    ])
    dashboard.value = dashboardData
    products.value = productResponse.products
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Dashboard could not be loaded.'
  } finally {
    loading.value = false
  }
}

watch([selectedPeriod, reportDate, topSellingDate], load)
onMounted(() => {
  void load()
  window.addEventListener('online-prefs-updated', handlePrefsUpdated)
})
onUnmounted(() => {
  window.removeEventListener('online-prefs-updated', handlePrefsUpdated)
})
</script>

<template>
  <section class="view-section">
    <div v-if="error" class="inline-error" role="alert">{{ error }}</div>

    <!-- Stats Grid -->
    <div class="stats-grid">
      <div v-for="stat in statCards" :key="stat.label" class="stat-card">
        <div class="stat-icon">
          <component :is="stat.icon" :class="stat.color" :size="24" />
        </div>
        <div class="stat-content">
          <div class="stat-label">{{ stat.label }}</div>
          <div class="stat-value">
            <template v-if="stat.isCurrency">
              <span class="currency-prefix">{{ activeCurrency }}</span>
              <span>{{ stat.numberPart }}</span>
            </template>
            <template v-else>
              {{ stat.numberPart }}
            </template>
            <span v-if="stat.subValue" class="stat-sub-value">{{ stat.subValue }}</span>
          </div>
          <div class="stat-trend-wrapper">
            <span class="stat-trend" :class="stat.trendClass">{{ stat.trend }}</span>
            <span v-if="stat.trendText" class="trend-text">{{ stat.trendText }}</span>
          </div>
          <svg
            v-if="isPhone && stat.sparkline?.length"
            class="stat-sparkline"
            viewBox="0 0 100 40"
            role="img"
            aria-label="Last seven value points"
            preserveAspectRatio="none"
          >
            <polyline
              :points="sparklinePoints(stat.sparkline)"
              fill="none"
              stroke="var(--accent-color, #0ea5e9)"
              stroke-width="2"
              vector-effect="non-scaling-stroke"
            />
          </svg>
        </div>
      </div>
    </div>

    <div v-if="!isPhone" class="dashboard-cards-wrapper">
      <!-- Sales Overview -->
      <div v-if="!isPhone" class="card sales-overview-card">
        <div class="tab-header">
          <div>
            <h3 class="card-title">Sales Overview</h3>
            <p class="card-subtitle">{{ selectedPeriod }} revenue & profitability timeline</p>
          </div>
          <div class="tab-controls-right">
            <div class="chart-mode-nav">
              <button
                type="button"
                :class="{ active: salesChartMode === 'all' }"
                @click="salesChartMode = 'all'"
              >
                All
              </button>
              <button
                type="button"
                :class="{ active: salesChartMode === 'sales' }"
                @click="salesChartMode = 'sales'"
              >
                Sales
              </button>
              <button
                type="button"
                :class="{ active: salesChartMode === 'profit' }"
                @click="salesChartMode = 'profit'"
              >
                Profit
              </button>
            </div>

            <div class="tab-nav">
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
            <div class="date-picker-btn" :class="{ 'active-date': reportDate !== '' }" title="Filter chart by date">
              <Calendar :size="15" />
              <input v-model="reportDate" type="date" class="invisible-date-picker" aria-label="Filter Sales Overview by date" />
              <button v-if="reportDate" type="button" class="clear-date-btn" title="Clear Date" @click.stop="reportDate = ''">
                <X :size="12" />
              </button>
            </div>
          </div>
        </div>

        <div class="sales-chart-container">
          <div class="sales-chart-plot">
            <DashboardSalesChart
              :chart-data="salesChartData"
              :currency-symbol="activeCurrency"
              :is-phone="isPhone"
            />
          </div>

          <div v-if="!isPhone" class="chart-stats">
            <div
              v-for="stat in salesSummaryStats"
              :key="stat.label"
              class="chart-stat"
              :class="`stat-${stat.label.toLowerCase()}`"
            >
              <div class="stat-badge-row">
                <span class="chart-stat-label">{{ stat.label }}</span>
                <span class="stat-badge">{{ stat.sub }}</span>
              </div>
              <span class="chart-stat-value">{{ stat.value }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Right Column: Top Selling Products / Stock Alerts -->
      <div v-if="!isPhone" class="rotating-card-slot">
        <!-- Top Selling Products -->
        <div v-if="rotatingDashboardCard === 'top-selling'" class="card rotating-dashboard-card">
          <div class="rotating-card-header">
            <div class="title-with-icon">
              <h3 class="card-title">Top Selling Products</h3>
              <div class="date-picker-btn" :class="{ 'active-date': topSellingDate !== '' }" title="Filter by specific date">
                <Calendar :size="16" />
                <input v-model="topSellingDate" type="date" class="invisible-date-picker" aria-label="Filter Top Selling Products by date" />
                <button v-if="topSellingDate" type="button" class="clear-date-btn" title="Clear Date" @click.stop="topSellingDate = ''">
                  <X :size="12" />
                </button>
              </div>
            </div>
            <button
              type="button"
              class="rotation-switch is-off"
              role="switch"
              aria-label="Show stock alerts"
              aria-checked="false"
              title="Switch to Stock Alerts"
              @click="toggleDashboardCard"
            >
              <span class="switch-knob"></span>
            </button>
          </div>

          <div class="product-list">
            <div v-if="!dashboard?.topProducts?.length" class="empty-product-list">
              No sales recorded yet.
            </div>
            <div v-for="product in dashboard?.topProducts" :key="product.name" class="product-item">
              <div class="product-info">
                <div class="product-name" :title="product.name">{{ product.name }}</div>
                <div class="product-sold">
                  {{ formatQuantity(product.quantitySold) }} {{ product.quantitySold === 1 ? 'unit' : 'units' }} sold
                </div>
              </div>
              <div class="product-revenue">{{ formatCurrency(product.revenue) }}</div>
            </div>
          </div>
        </div>

        <!-- Stock Alerts -->
        <div v-else class="card rotating-dashboard-card">
          <div class="rotating-card-header">
            <h3 class="card-title">Stock Alerts</h3>
            <button
              type="button"
              class="rotation-switch is-on"
              role="switch"
              aria-label="Show top selling products"
              aria-checked="true"
              title="Switch to Top Selling Products"
              @click="toggleDashboardCard"
            >
              <span class="switch-knob"></span>
            </button>
          </div>

          <div class="stock-alert-list">
            <div v-if="!stockAlerts.length" class="empty-product-list">
              All products are sufficiently stocked.
            </div>
            <div v-for="item in stockAlerts" :key="item.product.id" class="stock-alert-item">
              <div class="stock-alert-info">
                <div class="product-name">{{ item.name }}</div>
                <div class="product-sold">
                  <span :class="['status-badge', item.status]">
                    {{ item.status === 'low' ? 'Low Stock' : 'Out of Stock' }}
                  </span>
                  <span>{{ formatQuantity(item.quantity) }} units left</span>
                </div>
              </div>
              <div v-if="role !== 'cashier'" class="stock-alert-actions">
                <button type="button" class="adjust-btn" @click="openRestockModal(item.product)">Adjust</button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Restock Modal -->
    <div v-if="isRestockModalOpen && productToRestock" class="modal-backdrop" @click.self="closeRestockModal">
      <div class="modal-content">
        <div class="modal-header">
          <h3>Restock Product</h3>
          <button type="button" class="close-btn" @click="closeRestockModal">
            <X :size="20" />
          </button>
        </div>
        <form class="modal-body" @submit.prevent="handleRestock">
          <div v-if="restockError" class="inline-error" role="alert">{{ restockError }}</div>
          <div class="form-group">
            <label for="restockProductName">Product Name</label>
            <input id="restockProductName" type="text" :value="productToRestock.name" disabled />
          </div>
          <div class="form-group">
            <label for="restockNewQuantity">New Quantity</label>
            <input id="restockNewQuantity" v-model.number="newQuantity" type="number" min="0" step="any" required />
          </div>
          <div class="modal-footer">
            <button type="button" class="btn-secondary" @click="closeRestockModal">Cancel</button>
            <button type="submit" class="btn-primary" :disabled="restocking">
              {{ restocking ? 'Saving…' : 'Save Quantity' }}
            </button>
          </div>
        </form>
      </div>
    </div>
  </section>
</template>

<style scoped>
.view-section {
  display: flex;
  flex-direction: column;
  flex: 1;
  gap: 14px;
  min-width: 0;
  min-height: 0;
}

/* Stats Grid */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}

.stat-card {
  position: relative;
  border-radius: 12px;
  padding: 1.35rem 1.5rem;
  display: flex;
  flex-direction: column;
  border: 1px solid var(--border-light);
  border-top: 2px solid transparent;
  background: linear-gradient(135deg, rgba(30, 41, 59, 0.72) 0%, rgba(15, 23, 42, 0.85) 100%);
  backdrop-filter: blur(24px);
  -webkit-backdrop-filter: blur(24px);
  box-shadow: 0 10px 30px -10px rgba(0, 0, 0, 0.5), inset 0 1px 1px var(--border-light);
  transition: transform 0.25s ease, box-shadow 0.25s ease, border-color 0.25s ease;
  min-width: 0;
}

[data-theme='light'] .stat-card {
  background: #ffffff !important;
  border: 1px solid #e2e8f0 !important;
  box-shadow: 0 4px 18px -2px rgba(15, 23, 42, 0.06), 0 2px 6px -1px rgba(15, 23, 42, 0.03) !important;
}

[data-theme='light'] .stat-card:hover {
  box-shadow: 0 10px 25px -4px rgba(15, 23, 42, 0.1) !important;
}

.stat-card:nth-child(1) { border-top-color: rgba(34, 197, 94, 0.75); }
.stat-card:nth-child(1):hover {
  box-shadow: 0 0 25px 0 rgba(34, 197, 94, 0.2), inset 0 1px 1px rgba(34, 197, 94, 0.35);
  border-top-color: #22c55e;
}

.stat-card:nth-child(2) { border-top-color: rgba(56, 189, 248, 0.75); }
.stat-card:nth-child(2):hover {
  box-shadow: 0 0 25px 0 rgba(56, 189, 248, 0.2), inset 0 1px 1px rgba(56, 189, 248, 0.35);
  border-top-color: #38bdf8;
}

.stat-card:nth-child(3) { border-top-color: rgba(167, 139, 250, 0.75); }
.stat-card:nth-child(3):hover {
  box-shadow: 0 0 25px 0 rgba(167, 139, 250, 0.2), inset 0 1px 1px rgba(167, 139, 250, 0.35);
  border-top-color: #a78bfa;
}

.stat-card:nth-child(4) { border-top-color: rgba(250, 204, 21, 0.75); }
.stat-card:nth-child(4):hover {
  box-shadow: 0 0 25px 0 rgba(250, 204, 21, 0.2), inset 0 1px 1px rgba(250, 204, 21, 0.35);
  border-top-color: #facc15;
}

.stat-icon {
  position: absolute;
  top: 0.75rem;
  right: 1rem;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--border-light);
  border-radius: 12px;
  padding: 0.55rem;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

[data-theme='light'] .stat-icon {
  background: #f8fafc !important;
  border: 1px solid #e2e8f0 !important;
  box-shadow: 0 2px 6px rgba(15, 23, 42, 0.04) !important;
}

.icon-success { color: #22c55e; }
.icon-accent { color: #0ea5e9; }
.icon-purple { color: #8b5cf6; }
.icon-warning { color: #f59e0b; }

.stat-content {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  padding-right: 2.75rem;
}

.stat-label {
  font-size: 0.85rem;
  color: var(--text-muted);
  margin-bottom: 6px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.stat-value {
  font-size: 1rem;
  font-weight: 700;
  color: var(--text-primary);
  margin-bottom: 6px;
  white-space: nowrap;
  overflow: visible;
  line-height: 1.2;
  letter-spacing: -0.02em;
  font-variant-numeric: tabular-nums;
}

.currency-prefix {
  font-size: 0.8em;
  font-weight: 600;
  color: var(--text-muted);
  margin-right: 4px;
}

.stat-sub-value {
  font-size: 0.85rem;
  font-weight: 500;
  color: var(--text-muted);
  margin-left: 6px;
}

.stat-trend-wrapper {
  display: flex;
  align-items: center;
  font-size: 0.75rem;
  white-space: nowrap;
}

.stat-trend {
  font-weight: 600;
}

.trend-up { color: #22c55e; }
.trend-down { color: #ef4444; }
.trend-warning { color: #f59e0b; }

.trend-text {
  color: var(--text-muted);
  font-weight: 400;
  margin-left: 4px;
}

.stat-sparkline {
  width: 100%;
  height: 40px;
  margin-top: 8px;
  opacity: 0.95;
}

/* Main Dashboard Cards Wrapper */
.dashboard-cards-wrapper {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(290px, 1fr);
  gap: 12px;
  align-items: stretch;
  flex: 1;
  min-height: 0;
  min-width: 0;
}

.card {
  background-color: var(--bg-sidebar);
  border: 1px solid var(--border-subtle);
  border-radius: 12px;
  padding: 1.35rem 1.5rem;
  min-height: 0;
}

.card-title {
  margin: 0;
  font-size: 1.1rem;
  font-weight: 600;
  color: var(--text-primary);
}

.sales-overview-card {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.tab-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  gap: 12px;
  flex-wrap: wrap;
}

.card-subtitle {
  margin: 2px 0 0;
  font-size: 12px;
  color: var(--text-muted);
}

.tab-controls-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.chart-mode-nav {
  display: inline-flex;
  background: var(--glass-bg);
  border-radius: 9999px;
  padding: 3px;
  border: 1px solid var(--border-light);
  gap: 2px;
}

.chart-mode-nav button {
  background: transparent;
  border: none;
  color: var(--text-muted);
  padding: 4px 10px;
  border-radius: 9999px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  transition: all 0.18s ease;
}

.chart-mode-nav button:hover {
  color: var(--text-primary);
}

.chart-mode-nav button.active {
  background: rgba(14, 165, 233, 0.22);
  color: #38bdf8;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
}

.tab-nav {
  display: flex;
  background: var(--glass-bg);
  border-radius: 9999px;
  padding: 3px;
  border: 1px solid var(--border-light);
  box-shadow: inset 0 2px 4px rgba(0, 0, 0, 0.2);
}

.tab-nav button {
  background: transparent;
  border: 1px solid transparent;
  color: var(--text-muted);
  padding: 4px 12px;
  border-radius: 9999px;
  cursor: pointer;
  font-size: 12px;
  font-weight: 600;
  transition: all 0.2s ease;
}

.tab-nav button:hover {
  color: var(--text-primary);
}

.tab-nav button.active {
  background: rgba(14, 165, 233, 0.2);
  color: #38bdf8;
  border-color: rgba(14, 165, 233, 0.4);
}

/* Sales Overview Chart */
.chart-phone-hint {
  margin: 0;
  min-height: 240px;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  font-size: 0.875rem;
  text-align: center;
}

.sales-chart-container {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 310px;
  gap: 12px;
  min-width: 0;
}

.sales-chart-plot {
  flex: 1;
  min-height: 215px;
  position: relative;
  width: 100%;
}

.chart-stats {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
  flex-shrink: 0;
}

.chart-stat {
  position: relative;
  min-width: 0;
  padding: 11px 15px;
  overflow: hidden;
  border-radius: 10px;
  border: 1px solid var(--border-subtle);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.02) 0%, rgba(15, 23, 42, 0.35) 100%);
  transition: transform 0.2s ease, box-shadow 0.2s ease;
}

[data-theme='light'] .chart-stat {
  background: #ffffff !important;
  border: 1px solid #e2e8f0 !important;
  box-shadow: 0 2px 8px -2px rgba(15, 23, 42, 0.05) !important;
}

[data-theme='light'] .chart-stat-value {
  color: #0f172a !important;
}

[data-theme='light'] .chart-stat-label {
  color: #64748b !important;
}

[data-theme='light'] .stat-badge {
  background: #f1f5f9 !important;
  color: #475569 !important;
}

.chart-stat:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 20px -6px rgba(0, 0, 0, 0.4);
}

.chart-stat::before {
  content: '';
  position: absolute;
  top: 0;
  right: 0;
  left: 0;
  height: 2.5px;
}

.stat-sales::before {
  background-color: #0ea5e9;
  box-shadow: 0 0 10px rgba(14, 165, 233, 0.5);
}
.stat-sales {
  border-top-color: rgba(14, 165, 233, 0.3);
}

.stat-profit::before {
  background-color: #10b981;
  box-shadow: 0 0 10px rgba(16, 185, 129, 0.5);
}
.stat-profit {
  border-top-color: rgba(16, 185, 129, 0.3);
}

.stat-best::before {
  background-color: #f59e0b;
  box-shadow: 0 0 10px rgba(245, 158, 11, 0.5);
}
.stat-best {
  border-top-color: rgba(245, 158, 11, 0.3);
}

.stat-badge-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 5px;
}

.chart-stat-label {
  color: var(--text-muted);
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.stat-badge {
  font-size: 10px;
  font-weight: 600;
  color: var(--text-muted);
  padding: 1px 6px;
  border-radius: 4px;
  background: rgba(148, 163, 184, 0.08);
}

.chart-stat-value {
  display: block;
  overflow: hidden;
  color: var(--text-primary);
  font-size: 1.15rem;
  font-weight: 800;
  text-overflow: ellipsis;
  white-space: nowrap;
  letter-spacing: -0.01em;
}

/* Right Rotating Card (Top Selling Products / Stock Alerts) */
.rotating-card-slot {
  display: grid;
  min-width: 0;
  min-height: 100%;
}

.rotating-dashboard-card {
  display: flex;
  flex-direction: column;
  min-height: 100%;
  overflow: hidden;
}

.rotating-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 16px;
}

.title-with-icon {
  display: flex;
  align-items: center;
  gap: 8px;
}

.date-picker-btn {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 0.375rem;
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  color: var(--text-primary);
  cursor: pointer;
  height: 34px;
  width: 34px;
}

.date-picker-btn.active-date {
  border-color: var(--accent-color);
  color: var(--accent-color);
  background-color: rgba(14, 165, 233, 0.12);
}

.invisible-date-picker {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  opacity: 0;
  cursor: pointer;
}

.clear-date-btn {
  position: absolute;
  top: -6px;
  right: -6px;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  border: 1px solid var(--border-color);
  background: var(--bg-sidebar);
  color: var(--text-muted);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  padding: 0;
  z-index: 2;
}

.clear-date-btn:hover {
  color: #ef4444;
  border-color: #ef4444;
}

.rotation-switch {
  position: relative;
  width: 42px;
  height: 22px;
  padding: 0;
  flex-shrink: 0;
  border: 1px solid var(--border-color);
  border-radius: 999px;
  background-color: rgba(51, 65, 85, 0.5);
  box-shadow: inset 0 1px 3px rgba(2, 6, 23, 0.45);
  cursor: pointer;
  transition: background-color 0.25s ease, border-color 0.25s ease;
}

.switch-knob {
  position: absolute;
  top: 3px;
  left: 3px;
  width: 14px;
  height: 14px;
  border-radius: 999px;
  background-color: var(--accent-color);
  box-shadow: 0 2px 6px rgba(2, 6, 23, 0.45);
  transition: transform 0.25s ease;
}

.rotation-switch.is-on {
  border-color: rgba(14, 165, 233, 0.72);
  background-color: rgba(14, 165, 233, 0.18);
}

.rotation-switch.is-on .switch-knob {
  transform: translateX(18px);
}

.product-list,
.stock-alert-list {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  background-color: var(--bg-sidebar);
  border-radius: 8px;
  overflow-y: auto;
}

.product-item,
.stock-alert-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 0.85rem 1rem;
  border-bottom: 1px solid var(--border-subtle);
  transition: background-color 0.2s ease;
}

.product-item:last-child,
.stock-alert-item:last-child {
  border-bottom: none;
}

.product-item:hover,
.stock-alert-item:hover {
  background-color: var(--bg-surface);
}

.product-info {
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.product-name {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-secondary);
  margin-bottom: 2px;
  overflow-wrap: break-word;
}

.product-sold {
  font-size: 12px;
  color: var(--text-muted);
}

.product-revenue {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  white-space: nowrap;
}

.empty-product-list {
  padding: 2rem 1rem;
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
}

.stock-alert-info .product-sold {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  margin-top: 4px;
}

.status-badge {
  display: inline-flex;
  align-items: center;
  font-size: 11px;
  font-weight: 600;
  padding: 3px 8px;
  border-radius: 12px;
  border: 1px solid transparent;
  white-space: nowrap;
}

.status-badge.low {
  color: #fbbf24;
  background-color: rgba(251, 191, 36, 0.18);
  border-color: rgba(251, 191, 36, 0.3);
}

.status-badge.out {
  color: #ef4444;
  background-color: rgba(239, 68, 104, 0.18);
  border-color: rgba(239, 68, 104, 0.3);
}

.adjust-btn {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  color: var(--text-secondary);
  padding: 6px 12px;
  font-size: 12px;
  font-weight: 500;
  border-radius: 6px;
  cursor: pointer;
}

.adjust-btn:hover {
  background-color: var(--bg-hover);
  color: var(--text-primary);
}

/* Modal Styles */
.modal-content {
  background-color: var(--bg-sidebar);
  border: 1px solid var(--border-subtle);
  border-radius: 0.75rem;
  width: 100%;
  max-width: 400px;
  box-shadow: 0 15px 35px rgba(0, 0, 0, 0.45);
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1rem 1.5rem;
  border-bottom: 1px solid var(--border-subtle);
}

.modal-header h3 {
  margin: 0;
  font-size: 1.15rem;
  color: var(--accent-color);
}

.close-btn {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  padding: 0.25rem;
  display: flex;
}

.modal-body {
  padding: 1.5rem;
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.form-group label {
  font-size: 0.875rem;
  font-weight: 500;
  color: var(--text-muted);
}

.form-group input {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 0.375rem;
  padding: 0.6rem 0.75rem;
  color: var(--text-primary);
  font-size: 1rem;
}

.form-group input:disabled {
  background-color: var(--border-color);
  opacity: 0.7;
  cursor: not-allowed;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  padding-top: 0.5rem;
}

.btn-secondary {
  background-color: var(--border-color);
  color: var(--text-primary);
  padding: 0.55rem 1.1rem;
  border-radius: 0.375rem;
  border: none;
  cursor: pointer;
  font-weight: 500;
}

.btn-primary {
  background-color: var(--accent-color);
  color: #ffffff;
  padding: 0.55rem 1.1rem;
  border-radius: 0.375rem;
  border: none;
  cursor: pointer;
  font-weight: 500;
}

@media (max-width: 1024px) {
  .stats-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .dashboard-cards-wrapper {
    grid-template-columns: minmax(0, 1fr);
  }

  .sales-chart-container {
    min-height: 240px;
  }

  .sales-chart-plot {
    inset: 0;
  }
}

@media (max-width: 639px) {
  .stats-grid {
    grid-template-columns: 1fr;
    gap: 10px;
    flex: 1 1 auto;
    min-height: 570px;
    grid-template-rows:
      minmax(158px, 1fr)
      minmax(158px, 1fr)
      minmax(112px, 1fr)
      minmax(112px, 1fr);
  }

  .view-section {
    flex: 1 1 auto;
    height: auto;
    min-height: 0;
    gap: 0;
  }

  .stat-card {
    min-height: 112px;
    padding: 16px 16px 14px;
    border-radius: 14px;
    border-color: rgba(148, 163, 184, 0.18);
    border-top-width: 2px;
    background: linear-gradient(
      145deg,
      rgba(28, 42, 64, 0.96),
      rgba(13, 23, 38, 0.98)
    );
    box-shadow:
      0 10px 24px rgba(0, 0, 0, 0.28),
      inset 0 1px 0 rgba(255, 255, 255, 0.055);
  }

  .stat-card:nth-child(1),
  .stat-card:nth-child(2) {
    min-height: 158px;
  }

  .stat-icon {
    top: 14px;
    right: 14px;
    width: 44px;
    height: 44px;
    padding: 0;
    border-radius: 11px;
    background: rgba(255, 255, 255, 0.075);
  }

  .stat-content {
    padding-right: 54px;
  }

  .stat-label {
    margin-bottom: 8px;
    font-size: 12px;
    line-height: 1.25;
    font-weight: 700;
    color: #b8c5d8;
    letter-spacing: 0.01em;
  }

  .stat-value {
    margin-bottom: 7px;
    font-size: 22px;
    line-height: 1.15;
    font-weight: 800;
    color: #f8fafc;
    letter-spacing: -0.025em;
  }

  .currency-prefix {
    font-size: 12px;
    font-weight: 800;
    color: #9fb1c8;
    margin-right: 5px;
  }

  .stat-sub-value {
    font-size: 13px;
    color: #cbd5e1;
  }

  .stat-trend-wrapper {
    font-size: 12px;
    line-height: 1.3;
  }

  .stat-trend {
    font-weight: 800;
  }

  .trend-text {
    color: #9aabc0;
    margin-left: 5px;
  }

  .stat-sparkline {
    height: 40px;
    margin-top: 8px;
  }

  .tab-header {
    align-items: flex-start;
  }

  .tab-controls-right {
    width: 100%;
    display: grid;
    grid-template-columns: 1fr;
    gap: 8px;
  }

  .tab-nav {
    width: 100%;
  }

  .tab-nav button {
    flex: 1;
    min-height: 44px;
  }

  .tab-controls-right .date-picker-btn {
    justify-self: start;
    width: 46px;
    height: 46px;
    min-width: 46px;
    min-height: 46px;
  }

  .tab-controls-right .date-picker-btn span {
    display: none;
  }

  .tab-nav button {
    min-height: 44px;
  }

  .sales-chart-container {
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  .sales-chart-plot {
    position: relative;
    inset: auto;
    flex: 0 0 240px;
    height: 240px;
  }

  .chart-stats {
    position: static;
    grid-template-columns: 1fr;
    gap: 8px;
  }
}
</style>
