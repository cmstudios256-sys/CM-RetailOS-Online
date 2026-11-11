<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'
import { Download, LayoutGrid, List, Search } from '@lucide/vue'
import { apiRequest, formatMoney, formatQuantity, type Product } from '../api'
import { downloadCsv } from '../utils/reporting'
import { useMobileOrTabletBreakpoint, usePhoneBreakpoint } from '../utils/breakpoints'
import { useDelayedSkeleton } from '../utils/useDelayedSkeleton'

const props = defineProps<{ currency: string; role?: string }>()
const products = ref<Product[]>([])
const search = ref('')
const selectedCategory = ref('All Categories')
const selectedStock = ref<'All Stock' | 'In Stock' | 'Low Stock' | 'Out of Stock'>('All Stock')
const loading = ref(true)
const showSkeleton = useDelayedSkeleton(loading, 200)
const error = ref('')

const { isPhone } = usePhoneBreakpoint()
const { isMobileOrTablet } = useMobileOrTabletBreakpoint()

let savedStockViewMode = (typeof localStorage !== 'undefined' ? localStorage.getItem('cm_stock_levels_view_mode') : null) as 'grid' | 'list' | null
const viewMode = ref<'grid' | 'list'>(
  isMobileOrTablet.value ? 'grid' : savedStockViewMode === 'grid' ? 'grid' : 'list',
)

function setViewMode(mode: 'grid' | 'list') {
  viewMode.value = mode
  savedStockViewMode = mode
  try {
    localStorage.setItem('cm_stock_levels_view_mode', mode)
  } catch {}
  nextTick(() => {
    updateStockGridColumns()
    stockGridRef.value?.scrollTo({ top: 0 })
    stockListRef.value?.scrollTo({ top: 0 })
  })
}

watch(isMobileOrTablet, (mobile) => {
  if (mobile) {
    viewMode.value = 'grid'
  } else {
    viewMode.value = savedStockViewMode === 'grid' ? 'grid' : 'list'
  }
  nextTick(updateStockGridColumns)
})

const categories = computed(() => {
  const unique = new Set(products.value.map(product => product.category.trim()).filter(Boolean))
  return Array.from(unique).sort((first, second) => first.localeCompare(second))
})

const filteredProducts = computed(() => {
  const query = search.value.trim().toLowerCase()
  return products.value.filter(product => {
    const matchesSearch = !query
      || product.name.toLowerCase().includes(query)
      || product.category.toLowerCase().includes(query)
      || product.sku.toLowerCase().includes(query)
    const productCategory = product.category.trim() || 'General'
    const matchesCategory = selectedCategory.value === 'All Categories' || productCategory === selectedCategory.value
    const status = getStockStatus(product).text
    const matchesStock = selectedStock.value === 'All Stock' || status === selectedStock.value
    return matchesSearch && matchesCategory && matchesStock
  })
})

const stockGridRef = ref<HTMLElement | null>(null)
const stockGridColumns = ref(3)
const stockGridRowHeight = computed(() => isPhone.value ? 190 : 205)

function updateStockGridColumns() {
  if (isPhone.value) {
    stockGridColumns.value = 2
  } else if (isMobileOrTablet.value) {
    stockGridColumns.value = window.innerWidth >= 768 ? 3 : 2
  } else {
    const width = stockGridRef.value?.clientWidth || window.innerWidth
    if (width >= 1700) {
      stockGridColumns.value = 5
    } else if (width >= 1300) {
      stockGridColumns.value = 4
    } else if (width >= 950) {
      stockGridColumns.value = 3
    } else {
      stockGridColumns.value = 2
    }
  }
}

const stockGridRowCount = computed(() =>
  Math.ceil(filteredProducts.value.length / stockGridColumns.value),
)

const stockGridVirtualizer = useVirtualizer(computed(() => ({
  count: stockGridRowCount.value,
  getScrollElement: () => stockGridRef.value,
  estimateSize: () => stockGridRowHeight.value,
  overscan: 3,
})))

const virtualStockGridRows = computed(() => stockGridVirtualizer.value.getVirtualItems())

function stockGridProducts(rowIndex: number) {
  const start = rowIndex * stockGridColumns.value
  return filteredProducts.value.slice(start, start + stockGridColumns.value)
}

const stockListRef = ref<HTMLElement | null>(null)
const stockListVirtualizer = useVirtualizer(computed(() => ({
  count: filteredProducts.value.length,
  getScrollElement: () => stockListRef.value,
  estimateSize: () => 38,
  overscan: 8,
})))
const virtualStockListItems = computed(() => stockListVirtualizer.value.getVirtualItems())

watch(
  () => [search.value, selectedCategory.value, selectedStock.value, viewMode.value, filteredProducts.value.length],
  () => {
    stockGridRef.value?.scrollTo({ top: 0 })
    stockListRef.value?.scrollTo({ top: 0 })
  },
)

const totalProducts = computed(() => filteredProducts.value.length)
const totalUnits = computed(() => filteredProducts.value.reduce((sum, product) => sum + product.quantity, 0))

function getStockStatus(product: Product) {
  if (product.quantity <= 0) return { text: 'Out of Stock', class: 'status-out-of-stock' }
  if (product.quantity <= product.minStock) return { text: 'Low Stock', class: 'status-low-stock' }
  return { text: 'In Stock', class: 'status-in-stock' }
}

function projectedRevenue(product: Product) {
  return product.sellingPrice * product.quantity
}

function projectedProfit(product: Product) {
  return (product.sellingPrice - product.costPrice) * product.quantity
}

function profitPerUnit(product: Product) {
  return product.sellingPrice - product.costPrice
}

function exportStockLevels() {
  if (!filteredProducts.value.length) return
  downloadCsv(
    `stock_levels_${new Date().toISOString().slice(0, 10)}.csv`,
    ['Product', 'SKU', 'Category', 'Current Stock', 'Unit', 'Projected Revenue', 'Projected Profit', 'Profit per Unit', 'Status'],
    filteredProducts.value.map(product => [
      product.name,
      product.sku,
      product.category || 'General',
      product.quantity,
      product.unit,
      projectedRevenue(product),
      projectedProfit(product),
      profitPerUnit(product),
      getStockStatus(product).text,
    ]),
  )
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const result = await apiRequest<{ products: Product[] }>('/products')
    products.value = result.products
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Stock levels could not be loaded.'
  } finally {
    loading.value = false
    nextTick(updateStockGridColumns)
  }
}

onMounted(() => {
  load()
  updateStockGridColumns()
  window.addEventListener('resize', updateStockGridColumns)
})

onUnmounted(() => {
  window.removeEventListener('resize', updateStockGridColumns)
})
</script>

<template>
  <section class="view-section">
    <div v-if="error" class="inline-error" role="alert">{{ error }}</div>

    <div class="card">
      <div class="card-header">
        <div class="summary-badges">
          <div class="summary-badge">
            <span class="badge-value">{{ totalProducts }}</span>
            <span class="badge-label">Products</span>
          </div>
          <div class="summary-badge">
            <span class="badge-value">{{ formatQuantity(totalUnits) }}</span>
            <span class="badge-label">Total Units</span>
          </div>
        </div>

        <div class="header-controls">
          <div class="view-switcher" role="group" aria-label="Stock levels view switcher">
            <button
              type="button"
              :class="{ active: viewMode === 'grid' }"
              title="Grid View"
              aria-label="Switch to grid view"
              @click="setViewMode('grid')"
            >
              <LayoutGrid :size="18" />
            </button>
            <button
              type="button"
              :class="{ active: viewMode === 'list' }"
              title="List View"
              aria-label="Switch to list view"
              @click="setViewMode('list')"
            >
              <List :size="18" />
            </button>
          </div>

          <div class="search-bar">
            <Search class="search-icon" :size="18" />
            <input
              v-model="search"
              type="text"
              placeholder="Search by name, category..."
              aria-label="Search stock levels by name or category"
            />
          </div>

          <select v-model="selectedCategory" class="filter-select" aria-label="Filter stock by category">
            <option value="All Categories">All Categories</option>
            <option v-for="category in categories" :key="category" :value="category">{{ category }}</option>
          </select>

          <select v-model="selectedStock" class="filter-select" aria-label="Filter stock by status">
            <option value="All Stock">All Stock</option>
            <option value="In Stock">In Stock</option>
            <option value="Low Stock">Low Stock</option>
            <option value="Out of Stock">Out of Stock</option>
          </select>

          <button
            type="button"
            class="export-btn"
            :disabled="!filteredProducts.length"
            @click="exportStockLevels"
          >
            <Download :size="16" />
            Export
          </button>
        </div>
      </div>

      <!-- Grid View -->
      <div v-if="viewMode === 'grid'" ref="stockGridRef" class="stock-grid">
        <div v-if="showSkeleton" class="stock-skeleton-grid">
          <div v-for="i in 6" :key="i" class="stock-grid-card">
            <div style="display: flex; justify-content: space-between; margin-bottom: 12px;">
              <div style="display: flex; flex-direction: column; gap: 6px; width: 65%;">
                <div class="skeleton-box skeleton-shimmer" style="height: 16px; width: 80%;"></div>
                <div class="skeleton-box skeleton-shimmer" style="height: 12px; width: 50%;"></div>
              </div>
              <div class="skeleton-box skeleton-shimmer" style="height: 20px; width: 64px; border-radius: 999px;"></div>
            </div>
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 8px;">
              <div v-for="j in 4" :key="j" class="skeleton-box skeleton-shimmer" style="height: 28px; border-radius: 6px;"></div>
            </div>
          </div>
        </div>
        <p v-else-if="loading && !showSkeleton" class="stock-grid-empty"></p>
        <p v-else-if="filteredProducts.length === 0" class="stock-grid-empty">No products found.</p>
        <div
          v-else
          class="stock-virtual-grid-spacer"
          :style="{ height: `${stockGridVirtualizer.getTotalSize()}px` }"
        >
          <div
            v-for="virtualRow in virtualStockGridRows"
            :key="String(virtualRow.key)"
            class="stock-virtual-grid-row"
            :style="{
              transform: `translateY(${virtualRow.start}px)`,
              height: isPhone ? '184px' : '198px',
              gridTemplateColumns: `repeat(${stockGridColumns}, minmax(0, 1fr))`,
            }"
          >
            <article
              v-for="product in stockGridProducts(virtualRow.index)"
              :key="product.id"
              class="stock-grid-card"
            >
              <div class="stock-grid-header">
                <div v-if="product.image" class="stock-grid-thumb">
                  <img :src="product.image" :alt="product.name" loading="lazy" />
                </div>
                <div class="stock-grid-name">
                  <h3>{{ product.name }}</h3>
                  <span>{{ product.category || 'General' }}<template v-if="product.sku"> · {{ product.sku }}</template></span>
                </div>
                <span class="status-badge" :class="getStockStatus(product).class">
                  {{ getStockStatus(product).text }}
                </span>
              </div>

              <dl class="stock-grid-details">
                <div>
                  <dt>Current Stock</dt>
                  <dd class="stock-qty-val">
                    {{ formatQuantity(product.quantity) }} <span class="stock-unit">{{ product.unit }}</span>
                  </dd>
                </div>
                <div>
                  <dt>Profit / Unit</dt>
                  <dd :class="profitPerUnit(product) >= 0 ? 'profit-positive' : 'profit-negative'">
                    {{ formatMoney(profitPerUnit(product), currency) }}
                  </dd>
                </div>
                <div>
                  <dt>Projected Revenue</dt>
                  <dd>{{ formatMoney(projectedRevenue(product), currency) }}</dd>
                </div>
                <div>
                  <dt>Projected Profit</dt>
                  <dd :class="projectedProfit(product) >= 0 ? 'profit-positive' : 'profit-negative'">
                    {{ formatMoney(projectedProfit(product), currency) }}
                  </dd>
                </div>
              </dl>
            </article>
          </div>
        </div>
      </div>

      <!-- List / Table View -->
      <div v-else ref="stockListRef" class="table-container virtual-list-container" style="flex: 1; min-height: 0; overflow-y: auto;">
        <div class="virtual-list-header stock-list-header">
          <span>Product</span>
          <span>Category</span>
          <span>Current Stock</span>
          <span>Unit</span>
          <span>Projected Revenue</span>
          <span>Projected Profit</span>
          <span>Profit per Unit</span>
          <span>Status</span>
        </div>
        <div v-if="loading" class="empty-message">Loading stock levels...</div>
        <div v-else-if="filteredProducts.length === 0" class="empty-message">No products found.</div>
        <div
          v-else
          class="virtual-list-spacer"
          :style="{ height: `${stockListVirtualizer.getTotalSize()}px` }"
        >
          <div
            v-for="virtualRow in virtualStockListItems"
            :key="String(virtualRow.key)"
            class="virtual-list-row stock-list-header"
            :style="{ transform: `translateY(${virtualRow.start}px)`, height: `${virtualRow.size}px` }"
          >
            <template v-if="filteredProducts[virtualRow.index]">
              <span class="font-medium text-slate-200">{{ filteredProducts[virtualRow.index].name }}</span>
              <span>{{ filteredProducts[virtualRow.index].category || 'General' }}</span>
              <span class="font-semibold">{{ formatQuantity(filteredProducts[virtualRow.index].quantity) }}</span>
              <span class="text-slate-400">{{ filteredProducts[virtualRow.index].unit }}</span>
              <span>{{ formatMoney(projectedRevenue(filteredProducts[virtualRow.index]), currency) }}</span>
              <span
                class="font-semibold"
                :class="projectedProfit(filteredProducts[virtualRow.index]) >= 0 ? 'profit-positive' : 'profit-negative'"
              >
                {{ formatMoney(projectedProfit(filteredProducts[virtualRow.index]), currency) }}
              </span>
              <span
                class="font-semibold"
                :class="profitPerUnit(filteredProducts[virtualRow.index]) >= 0 ? 'profit-positive' : 'profit-negative'"
              >
                {{ formatMoney(profitPerUnit(filteredProducts[virtualRow.index]), currency) }}
              </span>
              <span>
                <span class="status-badge" :class="getStockStatus(filteredProducts[virtualRow.index]).class">
                  {{ getStockStatus(filteredProducts[virtualRow.index]).text }}
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
  padding: 1rem 1.25rem;
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
  margin-bottom: 0.5rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid var(--border-subtle);
}

.summary-badges {
  display: flex;
  gap: 0.5rem;
  align-items: center;
  flex-shrink: 0;
}

.summary-badge {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 0.5rem;
  padding: 0.3rem 0.7rem;
  display: flex;
  align-items: baseline;
  gap: 0.4rem;
}

.summary-badge:nth-child(2) {
  border-left-color: #22c55e;
}

.badge-value {
  font-size: 1.05rem;
  font-weight: 700;
  color: var(--text-primary);
}

.badge-label {
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.header-controls {
  display: flex;
  gap: 0.5rem;
  align-items: center;
  flex-wrap: nowrap;
  margin-left: auto;
  justify-content: flex-end;
  flex-shrink: 1;
  min-width: 0;
}

.view-switcher {
  display: flex;
  background-color: var(--bg-surface);
  border-radius: 0.5rem;
  padding: 0.2rem;
  border: 1px solid var(--border-color);
  flex-shrink: 0;
}

.view-switcher button {
  background: transparent;
  border: none;
  color: var(--text-muted);
  padding: 0.35rem 0.5rem;
  border-radius: 0.375rem;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s ease;
}

.view-switcher button:hover {
  color: var(--text-primary);
}

.view-switcher button.active {
  background-color: var(--bg-sidebar);
  color: var(--text-primary);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
}

[data-theme='light'] .view-switcher {
  background-color: #f1f5f9;
  border-color: #cbd5e1;
}

[data-theme='light'] .view-switcher button.active {
  background-color: #ffffff;
  color: #0f172a;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
}

.search-bar {
  position: relative;
  flex-shrink: 1;
  min-width: 0;
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
  width: clamp(140px, 15vw, 200px);
  font-size: 0.825rem;
}

.search-bar input:focus {
  outline: none;
  border-color: var(--accent-color);
}

.filter-select {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 0.375rem;
  padding: 0.4rem 0.65rem;
  color: var(--text-primary);
  font-size: 0.825rem;
  cursor: pointer;
  flex-shrink: 0;
  max-width: 140px;
}

.export-btn {
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
  transition: all 0.2s;
}

.export-btn:hover:not(:disabled) {
  background-color: var(--bg-hover);
  color: var(--text-primary);
}

.export-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

/* Grid View Styles */
.stock-grid {
  display: block;
  flex: 1;
  min-width: 0;
  min-height: 0;
  overflow-y: auto;
  padding: 8px 12px 24px 12px;
  box-sizing: border-box;
}

.stock-virtual-grid-spacer {
  position: relative;
  width: 100%;
}

.stock-virtual-grid-row {
  position: absolute;
  left: 0;
  right: 0;
  top: 0;
  display: grid;
  gap: 16px;
}

.stock-grid-card {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 0.85rem;
  min-width: 0;
  padding: 14px 16px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  border-radius: 14px;
  background: #111c2e;
  box-shadow: 0 4px 18px -2px rgba(0, 0, 0, 0.45);
  height: 100%;
  box-sizing: border-box;
  overflow: hidden;
  cursor: default;
  transition: transform 0.15s ease, box-shadow 0.15s ease, border-color 0.15s ease;
}

.stock-grid-card:hover {
  border-color: rgba(148, 163, 184, 0.3);
  box-shadow: 0 6px 22px -2px rgba(0, 0, 0, 0.55);
}

[data-theme='light'] .stock-grid-card {
  background: #ffffff !important;
  border: 1px solid #e2e8f0 !important;
  box-shadow: 0 2px 10px rgba(15, 23, 42, 0.08) !important;
}

.stock-grid-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.65rem;
  min-height: 44px;
}

.stock-grid-thumb {
  width: 44px;
  height: 44px;
  min-width: 44px;
  border-radius: 8px;
  overflow: hidden;
  background: rgba(15, 23, 42, 0.6);
  border: 1px solid rgba(148, 163, 184, 0.15);
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.stock-grid-thumb img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}

[data-theme='light'] .stock-grid-thumb {
  background: #f1f5f9;
  border: 1px solid #cbd5e1;
}

.stock-grid-name {
  flex: 1;
  min-width: 0;
}

.stock-grid-name h3 {
  margin: 0;
  color: var(--text-primary);
  font-size: 0.92rem;
  font-weight: 650;
  line-height: 1.25;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  word-break: break-word;
}

.stock-grid-name > span {
  display: block;
  margin-top: 0.2rem;
  color: var(--text-muted);
  font-size: 0.75rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.stock-grid-header .status-badge {
  flex-shrink: 0;
}

.stock-grid-details {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.65rem 0.75rem;
  margin: 0;
  padding-top: 0.5rem;
  border-top: 1px solid var(--border-subtle);
}

.stock-grid-details > div {
  min-width: 0;
}

.stock-grid-details dt {
  color: var(--text-muted);
  font-size: 0.72rem;
}

.stock-grid-details dd {
  margin: 0.15rem 0 0;
  color: var(--text-primary);
  font-size: 0.88rem;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.stock-qty-val {
  display: flex;
  align-items: baseline;
  gap: 4px;
}

.stock-unit {
  font-size: 0.75rem;
  color: var(--text-muted);
  font-weight: 400;
}

.stock-grid-empty {
  grid-column: 1 / -1;
  margin: 0;
  padding: 2rem 1rem;
  color: var(--text-muted);
  text-align: center;
}

.stock-skeleton-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
  padding: 8px 0;
}

.skeleton-box {
  background: rgba(148, 163, 184, 0.1);
  border-radius: 4px;
}

.skeleton-shimmer {
  background: linear-gradient(
    90deg,
    rgba(148, 163, 184, 0.08) 25%,
    rgba(148, 163, 184, 0.18) 50%,
    rgba(148, 163, 184, 0.08) 75%
  );
  background-size: 200% 100%;
  animation: shimmer 1.5s infinite;
}

@keyframes shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

/* List View Styles */
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

.stock-list-header {
  grid-template-columns: minmax(160px, 1.5fr) minmax(120px, 1.1fr) minmax(95px, 0.9fr) minmax(75px, 0.7fr) minmax(165px, 1.4fr) minmax(155px, 1.3fr) minmax(130px, 1.1fr) minmax(105px, 1fr);
  min-width: 1005px;
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
  padding: 0.4rem 0.65rem;
  border-right: 1px solid var(--border-subtle);
  border-bottom: 1px solid var(--border-subtle);
  text-align: center;
  font-size: 0.825rem;
  white-space: nowrap;
  box-sizing: border-box;
  height: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
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
  min-width: 780px;
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

.font-medium { font-weight: 500; }
.font-semibold { font-weight: 600; }
.text-slate-200 { color: var(--text-primary); }
.text-slate-400 { color: var(--text-muted); }
.text-center { text-align: center; }

.profit-positive { color: #4ade80; }
.profit-negative { color: #f87171; }

.status-badge {
  padding: 0.2rem 0.55rem;
  border-radius: 9999px;
  font-size: 0.7rem;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
  border: 1px solid;
}

.status-in-stock {
  border-color: #22c55e;
  color: #4ade80;
  background-color: rgba(34, 197, 94, 0.1);
}

.status-low-stock {
  border-color: #f59e0b;
  color: #fbbf24;
  background-color: rgba(245, 158, 11, 0.1);
}

.status-out-of-stock {
  border-color: #ef4444;
  color: #f87171;
  background-color: rgba(239, 68, 68, 0.1);
}

.empty-message {
  text-align: center;
  padding: 1.5rem;
  color: var(--text-muted);
}

@media (max-width: 900px) {
  .card-header {
    flex-direction: column;
    align-items: stretch;
    gap: 0.75rem;
  }

  .summary-badges {
    width: 100%;
  }

  .summary-badge {
    flex: 1;
  }

  .header-controls {
    width: 100%;
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    margin-left: 0;
    justify-content: stretch;
  }

  .view-switcher {
    display: none !important;
  }

  .search-bar {
    flex: 1 1 100%;
    width: 100%;
  }

  .search-bar input {
    width: 100%;
  }

  .filter-select {
    flex: 1 1 calc(50% - 6px);
    max-width: none;
  }

  .export-btn {
    flex: 1 1 100%;
    justify-content: center;
  }

  .stock-grid {
    padding: 2px 4px 80px 4px !important;
  }

  .stock-virtual-grid-row {
    gap: 8px;
  }

  .stock-grid-card {
    padding: 10px 12px;
  }
}
</style>
