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
import AppDatePicker from './AppDatePicker.vue'
import DashboardSalesChart from './DashboardSalesChart.vue'
import { useMobileOrTabletBreakpoint, usePhoneBreakpoint } from '../utils/breakpoints'
import { useDelayedSkeleton } from '../utils/useDelayedSkeleton'

type Period = 'Daily' | 'Weekly' | 'Monthly'

type ChartPoint = {
  label: string
  sales: number
  profit: number
}

const { isPhone } = usePhoneBreakpoint()
const { isMobileOrTablet } = useMobileOrTabletBreakpoint()

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
const showSkeleton = useDelayedSkeleton(loading, 200)
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

const activeCurrency = computed(() => props.currency || dashboard.value?.currency || 'UGX')

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
      label: isPhone.value ? 'Inventory Value' : 'Total Inventory Value',
      isCurrency: true,
      numberPart: Math.round(inventoryValue).toLocaleString(),
      subValue: '',
      icon: Boxes,
      color: 'icon-warning',
      trend: `${activeProductsCount.value} active items`,
      trendClass: 'trend-warning',
      trendText: isPhone.value ? '' : 'currently in stock',
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

const displayedTopProducts = computed(() => {
  const list = dashboard.value?.topProducts || []
  return isPhone.value ? list.slice(0, 3) : list
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

    <!-- Skeleton Loading State (Shown only when loading >200ms) -->
    <template v-if="showSkeleton">
      <div class="stats-grid">
        <div v-for="i in 4" :key="i" class="stat-card skeleton-stat-card">
          <div class="stat-card-header">
            <div class="skeleton-box skeleton-shimmer" style="width: 55%; height: 14px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 26px; height: 26px; border-radius: 8px;"></div>
          </div>
          <div class="stat-content" style="gap: 6px;">
            <div class="skeleton-box skeleton-shimmer" style="width: 75%; height: 22px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 45%; height: 12px;"></div>
          </div>
        </div>
      </div>
      <div class="dashboard-cards-wrapper">
        <div class="card sales-overview-card">
          <div class="tab-header" style="justify-content: space-between;">
            <div style="display: flex; flex-direction: column; gap: 6px;">
              <div class="skeleton-box skeleton-shimmer" style="width: 140px; height: 20px;"></div>
              <div class="skeleton-box skeleton-shimmer" style="width: 200px; height: 12px;"></div>
            </div>
            <div style="display: flex; gap: 8px;">
              <div class="skeleton-box skeleton-shimmer" style="width: 100px; height: 32px; border-radius: 999px;"></div>
              <div class="skeleton-box skeleton-shimmer" style="width: 130px; height: 32px; border-radius: 999px;"></div>
            </div>
          </div>
          <div class="skeleton-box skeleton-shimmer" style="width: 100%; height: 260px; margin: 16px 0; border-radius: 12px;"></div>
          <div class="chart-stats-grid">
            <div v-for="j in 3" :key="j" class="skeleton-box skeleton-shimmer" style="height: 64px; border-radius: 10px;"></div>
          </div>
        </div>
        <div class="rotating-card-slot">
          <div class="card rotating-dashboard-card" style="height: 100%; min-height: 240px;">
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;">
              <div class="skeleton-box skeleton-shimmer" style="width: 150px; height: 20px;"></div>
              <div class="skeleton-box skeleton-shimmer" style="width: 32px; height: 32px; border-radius: 8px;"></div>
            </div>
            <div style="display: flex; flex-direction: column; gap: 14px;">
              <div v-for="k in 5" :key="k" class="skeleton-box skeleton-shimmer" style="width: 100%; height: 48px; border-radius: 8px;"></div>
            </div>
          </div>
        </div>
      </div>
    </template>

    <template v-else>
      <!-- Stats Grid -->
      <div class="stats-grid">
        <div v-for="stat in statCards" :key="stat.label" class="stat-card">
          <div class="stat-card-header">
            <span class="stat-label">{{ stat.label }}</span>
            <div class="stat-icon">
              <component :is="stat.icon" :class="stat.color" :size="isPhone ? 15 : 20" />
            </div>
          </div>
          <div class="stat-content">
            <div class="stat-value">
              <template v-if="stat.isCurrency">
                <span class="currency-prefix">{{ activeCurrency }}</span>
                <span class="stat-amount">{{ stat.numberPart }}</span>
              </template>
              <template v-else>
                <span class="stat-amount">{{ stat.numberPart }}</span>
              </template>
              <span v-if="stat.subValue" class="stat-sub-value">{{ stat.subValue }}</span>
            </div>
            <div class="stat-trend-wrapper">
              <span class="stat-trend" :class="stat.trendClass">{{ stat.trend }}</span>
              <span v-if="stat.trendText" class="trend-text">{{ stat.trendText }}</span>
            </div>
          </div>
        </div>
      </div>

    <div class="dashboard-cards-wrapper">
      <!-- Sales Overview -->
      <div class="card sales-overview-card">
        <div class="tab-header">
          <div class="tab-header-title-group">
            <div>
              <h3 class="card-title">Sales Overview</h3>
              <p class="card-subtitle">{{ selectedPeriod }} revenue & profitability timeline</p>
            </div>
            <AppDatePicker
              v-if="isMobileOrTablet"
              v-model="reportDate"
              placeholder="Filter by date"
              title="Filter Sales Overview by Date"
              aria-label="Filter Sales Overview by date"
              :compact="true"
            />
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
            <AppDatePicker
              v-if="!isMobileOrTablet"
              v-model="reportDate"
              placeholder="Filter chart by date"
              title="Filter Sales Overview by Date"
              aria-label="Filter Sales Overview by date"
              :compact="true"
            />
          </div>
        </div>

        <div class="sales-chart-container">
          <div class="sales-chart-plot">
            <DashboardSalesChart
              :chart-data="salesChartData"
              :currency-symbol="activeCurrency"
              :is-phone="isMobileOrTablet"
            />
          </div>

          <div class="chart-stats">
            <div
              v-for="stat in salesSummaryStats"
              :key="stat.label"
              class="chart-stat"
              :class="`stat-${stat.label.toLowerCase()}`"
            >
              <div class="stat-badge-row">
                <span class="chart-stat-label">{{ stat.label }}</span>
                <span v-if="!isMobileOrTablet" class="stat-badge">{{ stat.sub }}</span>
              </div>
              <span class="chart-stat-value">{{ stat.value }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Right Column: Top Selling Products / Stock Alerts -->
      <div class="rotating-card-slot">
        <!-- Top Selling Products -->
        <div v-if="rotatingDashboardCard === 'top-selling'" class="card rotating-dashboard-card">
          <div class="rotating-card-header">
            <div class="title-with-icon">
              <h3 class="card-title">Top Selling Products</h3>
              <AppDatePicker
                v-if="!isMobileOrTablet"
                v-model="topSellingDate"
                placeholder="Filter by date"
                title="Filter Top Selling Products by Date"
                aria-label="Filter Top Selling Products by date"
                :compact="true"
              />
            </div>
            <button
              v-if="!isMobileOrTablet"
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
            <div v-if="!displayedTopProducts.length" class="empty-product-list">
              No sales recorded yet.
            </div>
            <div v-for="product in displayedTopProducts" :key="product.name" class="product-item">
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
              v-if="!isMobileOrTablet"
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
    </template>

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
  gap: 8px;
  min-width: 0;
  min-height: 0;
}

/* Stats Grid */
.stats-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 8px;
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

[data-theme='light'] .stat-label {
  color: #64748b !important;
}

[data-theme='light'] .stat-value {
  color: #0f172a !important;
}

[data-theme='light'] .currency-prefix {
  color: #64748b !important;
}

[data-theme='light'] .stat-sub-value {
  color: #64748b !important;
}

[data-theme='light'] .trend-text {
  color: #64748b !important;
}

[data-theme='light'] .chart-mode-nav button.active {
  background: color-mix(in srgb, var(--accent-color) 14%, transparent) !important;
  color: var(--accent-color) !important;
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

.stat-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 0.65rem;
  width: 100%;
}

.stat-icon {
  position: static;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--border-light);
  border-radius: 12px;
  padding: 0.45rem;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
  flex-shrink: 0;
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
  padding-right: 0;
  width: 100%;
}

.stat-label {
  font-size: 0.85rem;
  color: var(--text-muted);
  margin-bottom: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
}

.stat-value {
  font-size: 1.15rem;
  font-weight: 700;
  color: var(--text-primary);
  margin-bottom: 6px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  line-height: 1.2;
  letter-spacing: -0.02em;
  font-variant-numeric: tabular-nums;
  display: flex;
  align-items: baseline;
  width: 100%;
}

.currency-prefix {
  font-size: 0.78em;
  font-weight: 600;
  color: var(--text-muted);
  margin-right: 4px;
  flex-shrink: 0;
}

.stat-amount {
  font-weight: 800;
  letter-spacing: -0.02em;
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

@media (min-width: 1025px) {
  .stat-card {
    padding: 0.78rem 0.9rem;
    min-height: 0;
  }

  .stat-card-header {
    margin-bottom: 0.28rem;
  }

  .stat-icon {
    width: 28px;
    height: 28px;
    padding: 0.34rem;
    border-radius: 10px;
  }

  .stat-value {
    margin-bottom: 4px;
    line-height: 1.12;
  }

  .stat-sub-value {
    font-size: 0.78rem;
  }
}

/* Main Dashboard Cards Wrapper */
.dashboard-cards-wrapper {
  display: grid;
  grid-template-columns: minmax(0, 2fr) minmax(290px, 1fr);
  gap: 8px;
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

.sales-overview-card:active,
.rotating-dashboard-card:active {
  transform: none;
}

.tab-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  gap: 12px;
  flex-wrap: wrap;
}

.tab-header-title-group {
  display: flex;
  align-items: center;
  gap: 12px;
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
  padding-left: 0.8rem;
  padding-right: 0.8rem;
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
  width: 36px;
  height: 36px;
  min-width: 36px;
  min-height: 36px;
  border-radius: 50%;
  border: none !important;
  outline: none !important;
  background: rgba(255, 255, 255, 0.08);
  color: #cbd5e1;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  padding: 0;
  transition: background 0.18s ease, transform 0.15s ease, color 0.18s ease;
  -webkit-tap-highlight-color: transparent;
  flex-shrink: 0;
}

.close-btn:hover {
  color: #ffffff;
  background: rgba(255, 255, 255, 0.16);
}

.close-btn:active {
  transform: scale(0.92);
  background: rgba(255, 255, 255, 0.22);
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
    grid-template-columns: repeat(2, minmax(0, 1fr)) !important;
    gap: 6px;
    flex: 0 0 auto;
    min-height: auto;
    grid-template-rows: auto;
  }

  .view-section {
    flex: 1 1 auto;
    min-height: 100%;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .stat-card {
    min-height: 86px;
    padding: 10px 10px 8px;
    border-radius: 14px;
    border: none !important;
    background: rgba(28, 28, 30, 0.78);
    backdrop-filter: blur(20px);
    -webkit-backdrop-filter: blur(20px);
    box-shadow: 0 4px 16px -2px rgba(0, 0, 0, 0.28);
    display: flex;
    flex-direction: column;
    justify-content: space-between;
  }

  .stat-card:nth-child(1) {
    border: none !important;
    border-top: none !important;
    background: linear-gradient(180deg, rgba(34, 197, 94, 0.08) 0%, rgba(28, 28, 30, 0.78) 100%);
  }
  .stat-card:nth-child(1) .stat-icon {
    background: rgba(34, 197, 94, 0.16) !important;
    border: none !important;
  }
  .stat-card:nth-child(1) .stat-icon svg {
    color: #22c55e !important;
  }

  .stat-card:nth-child(2) {
    border: none !important;
    border-top: none !important;
    background: linear-gradient(180deg, rgba(56, 189, 248, 0.08) 0%, rgba(28, 28, 30, 0.78) 100%);
  }
  .stat-card:nth-child(2) .stat-icon {
    background: rgba(56, 189, 248, 0.16) !important;
    border: none !important;
  }
  .stat-card:nth-child(2) .stat-icon svg {
    color: #38bdf8 !important;
  }

  .stat-card:nth-child(3) {
    border: none !important;
    border-top: none !important;
    background: linear-gradient(180deg, rgba(167, 139, 250, 0.08) 0%, rgba(28, 28, 30, 0.78) 100%);
  }
  .stat-card:nth-child(3) .stat-icon {
    background: rgba(167, 139, 250, 0.16) !important;
    border: none !important;
  }
  .stat-card:nth-child(3) .stat-icon svg {
    color: #a78bfa !important;
  }

  .stat-card:nth-child(4) {
    border: none !important;
    border-top: none !important;
    background: linear-gradient(180deg, rgba(250, 204, 21, 0.08) 0%, rgba(28, 28, 30, 0.78) 100%);
  }
  .stat-card:nth-child(4) .stat-icon {
    background: rgba(250, 204, 21, 0.16) !important;
    border: none !important;
  }
  .stat-card:nth-child(4) .stat-icon svg {
    color: #facc15 !important;
  }

  .stat-card-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 6px;
    margin-bottom: 6px;
    width: 100%;
  }

  .stat-icon {
    position: static;
    top: auto;
    right: auto;
    width: 26px;
    height: 26px;
    padding: 0;
    border-radius: 8px;
    box-shadow: none;
    display: flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .stat-icon svg {
    width: 15px;
    height: 15px;
  }

  .stat-content {
    padding-right: 0 !important;
    display: flex;
    flex-direction: column;
    width: 100%;
  }

  .stat-label {
    margin-bottom: 0;
    font-size: 11.5px;
    line-height: 1.2;
    font-weight: 500;
    color: #8e8e93;
    letter-spacing: -0.05px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    flex: 1;
  }

  .stat-value {
    margin-bottom: 3px;
    font-size: clamp(14px, 4.4vw, 17px);
    line-height: 1.2;
    font-weight: 750;
    color: #ffffff;
    letter-spacing: -0.35px;
    font-variant-numeric: tabular-nums;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: flex;
    align-items: baseline;
    width: 100%;
  }

  .currency-prefix {
    font-size: 10.5px;
    font-weight: 600;
    color: #8e8e93;
    margin-right: 3px;
    flex-shrink: 0;
    letter-spacing: 0;
  }

  .stat-amount {
    font-weight: 800;
    letter-spacing: -0.35px;
  }

  .stat-sub-value {
    font-size: 10.5px;
    color: #8e8e93;
    margin-left: 3px;
  }

  .stat-trend-wrapper {
    font-size: 11px;
    line-height: 1.25;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    width: 100%;
  }

  .stat-trend {
    font-weight: 600;
  }

  .trend-positive {
    color: #34c759 !important;
  }

  .trend-negative {
    color: #ff453a !important;
  }

  .trend-text {
    color: #8e8e93;
    margin-left: 3px;
    font-weight: 400;
  }

  .stat-sparkline {
    display: none !important;
  }

  [data-theme='light'] .stat-card {
    background: #ffffff !important;
    border: 1px solid rgba(0, 0, 0, 0.08) !important;
    border-top: 1px solid rgba(0, 0, 0, 0.08) !important;
    box-shadow: 0 2px 10px rgba(0, 0, 0, 0.04) !important;
  }

  [data-theme='light'] .stat-card:nth-child(1),
  [data-theme='light'] .stat-card:nth-child(2),
  [data-theme='light'] .stat-card:nth-child(3),
  [data-theme='light'] .stat-card:nth-child(4) {
    border: none !important;
    border-top: none !important;
  }

  [data-theme='light'] .stat-label {
    color: #6e6e73 !important;
  }

  [data-theme='light'] .stat-value {
    color: #000000 !important;
  }

  [data-theme='light'] .currency-prefix {
    color: #86868b !important;
  }

  [data-theme='light'] .stat-sub-value {
    color: #86868b !important;
  }

  [data-theme='light'] .trend-text {
    color: #86868b !important;
  }

  .dashboard-cards-wrapper {
    display: flex;
    flex-direction: column;
    flex: 1 1 auto;
    min-height: 0;
    gap: 8px;
    min-width: 0;
  }

  .sales-overview-card {
    display: flex;
    flex-direction: column;
    flex: 1 1 auto;
    min-height: 250px;
    padding: 10px 10px 8px;
    border-radius: 14px;
    border: none !important;
    overflow: visible;
  }

  .tab-header {
    flex-direction: column;
    align-items: stretch;
    gap: 8px;
    margin-bottom: 8px;
  }

  .tab-header-title-group {
    display: flex;
    align-items: center;
    justify-content: space-between;
    width: 100%;
    gap: 8px;
  }

  .tab-controls-right {
    width: 100%;
    display: flex;
    flex-wrap: nowrap;
    align-items: center;
    justify-content: space-between;
    gap: 6px;
  }

  .chart-mode-nav {
    display: inline-flex;
    flex: 0 0 auto;
    padding: 2px;
    border: none !important;
  }

  .chart-mode-nav button {
    min-height: 30px;
    padding: 2px 8px;
    font-size: 11px;
    border: none !important;
  }

  .tab-nav {
    display: inline-flex;
    flex: 0 0 auto;
    padding: 2px;
    border: none !important;
  }

  .tab-nav button {
    min-height: 30px;
    padding: 2px 8px;
    font-size: 11px;
    border: none !important;
  }

  .date-picker-btn {
    width: 32px;
    height: 32px;
    min-width: 32px;
    min-height: 32px;
    padding: 0;
    justify-content: center;
    flex: 0 0 auto;
    border: none !important;
  }

  .date-picker-btn span {
    display: none;
  }

  .sales-chart-container {
    display: flex;
    flex-direction: column;
    flex: 1 1 auto;
    min-height: 0;
    gap: 6px;
    justify-content: flex-start;
  }

  .sales-chart-plot {
    position: relative;
    inset: auto;
    flex: 1 1 auto;
    min-height: 125px;
    height: 100%;
    width: 100%;
  }

  .chart-stats {
    position: static;
    display: grid;
    grid-template-columns: repeat(3, minmax(0, 1fr));
    gap: 6px;
    flex: 0 0 auto;
    margin-top: 6px;
  }

  .chart-stat {
    padding: 6px 6px 5px;
    min-width: 0;
    border-radius: 8px;
    border: none !important;
    border-top: none !important;
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
  }

  .chart-stat.stat-sales {
    background: linear-gradient(180deg, rgba(14, 165, 233, 0.12) 0%, rgba(15, 23, 42, 0.4) 100%);
    border: none !important;
    border-top: none !important;
  }

  .chart-stat.stat-profit {
    background: linear-gradient(180deg, rgba(16, 185, 129, 0.12) 0%, rgba(15, 23, 42, 0.4) 100%);
    border: none !important;
    border-top: none !important;
  }

  .chart-stat.stat-best {
    background: linear-gradient(180deg, rgba(245, 158, 11, 0.12) 0%, rgba(15, 23, 42, 0.4) 100%);
    border: none !important;
    border-top: none !important;
  }

  [data-theme='light'] .chart-stat.stat-sales {
    background: linear-gradient(180deg, rgba(14, 165, 233, 0.10) 0%, #ffffff 100%) !important;
    border: none !important;
    border-top: none !important;
    box-shadow: 0 2px 8px rgba(14, 165, 233, 0.1) !important;
  }

  [data-theme='light'] .chart-stat.stat-profit {
    background: linear-gradient(180deg, rgba(16, 185, 129, 0.10) 0%, #ffffff 100%) !important;
    border: none !important;
    border-top: none !important;
    box-shadow: 0 2px 8px rgba(16, 185, 129, 0.1) !important;
  }

  [data-theme='light'] .chart-stat.stat-best {
    background: linear-gradient(180deg, rgba(245, 158, 11, 0.10) 0%, #ffffff 100%) !important;
    border: none !important;
    border-top: none !important;
    box-shadow: 0 2px 8px rgba(245, 158, 11, 0.1) !important;
  }

  [data-theme='light'] .stat-sales .chart-stat-value {
    color: #0369a1 !important;
  }

  [data-theme='light'] .stat-profit .chart-stat-value {
    color: #047857 !important;
  }

  [data-theme='light'] .stat-best .chart-stat-value {
    color: #b45309 !important;
  }

  .chart-stat-value {
    font-size: clamp(10px, 3.1vw, 13px);
    font-weight: 800;
    letter-spacing: -0.02em;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    margin-top: 3px;
    line-height: 1.3;
  }

  .chart-stat-label {
    font-size: 9.5px;
    font-weight: 800;
  }

  .stat-badge {
    display: none !important;
  }

  .rotating-card-slot {
    display: block;
    min-width: 0;
    min-height: auto;
    width: 100%;
  }

  .rotating-dashboard-card {
    min-height: auto;
    padding: 8px 10px 6px;
    border-radius: 12px;
    margin-bottom: 4px;
    background: rgba(28, 28, 30, 0.65);
    border: none !important;
  }

  .rotating-card-header {
    margin-bottom: 4px;
    gap: 6px;
  }

  .rotating-card-header .date-picker-btn,
  .rotating-card-header .rotation-switch {
    display: none !important;
  }

  .rotating-card-header .card-title {
    font-size: 11px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--text-muted, #8e8e93);
  }

  .product-list {
    max-height: 125px;
    overflow-y: auto;
    scrollbar-width: none;
  }

  .product-list::-webkit-scrollbar {
    display: none;
  }

  .product-item,
  .stock-alert-item {
    padding: 4px 2px;
    min-height: 26px;
    border: none !important;
    border-bottom: none !important;
  }

  .product-item:nth-child(n+4) {
    display: none !important;
  }

  .product-item:last-child,
  .stock-alert-item:last-child {
    border-bottom: none;
  }

  .product-name {
    font-size: 11px;
    font-weight: 600;
    line-height: 1.2;
    max-width: 190px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .product-sold {
    font-size: 9px;
    color: var(--text-muted, #8e8e93);
    margin-top: 1px;
  }

  .product-revenue {
    font-size: 11px;
    font-weight: 700;
    color: var(--accent-color, #38bdf8);
  }
}
</style>
