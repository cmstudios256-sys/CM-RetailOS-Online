<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Download, Search, SlidersHorizontal, X } from '@lucide/vue'
import { apiRequest, formatMoney, formatQuantity, type Product } from '../api'
import { downloadCsv } from '../utils/reporting'

const props = defineProps<{ currency: string; role: string }>()
const products = ref<Product[]>([])
const search = ref('')
const selectedCategory = ref('All Categories')
const selectedStock = ref<'All Stock' | 'In Stock' | 'Low Stock' | 'Out of Stock'>('All Stock')
const loading = ref(true)
const error = ref('')
const adjustmentError = ref('')
const adjustingProduct = ref<Product | null>(null)
const adjustmentMode = ref<'add' | 'remove' | 'set'>('add')
const adjustmentQuantity = ref(1)
const adjustmentReason = ref('Stock count adjustment')
const adjusting = ref(false)
const canManage = computed(() => props.role === 'owner' || props.role === 'manager')

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

const totalProducts = computed(() => filteredProducts.value.length)
const totalUnits = computed(() => filteredProducts.value.reduce((sum, product) => sum + product.quantity, 0))
const adjustmentDelta = computed(() => {
  const quantity = Number(adjustmentQuantity.value)
  if (!Number.isFinite(quantity)) return 0
  if (adjustmentMode.value === 'set') return quantity - (adjustingProduct.value?.quantity ?? 0)
  return adjustmentMode.value === 'add' ? quantity : -quantity
})
const resultingStock = computed(() =>
  Math.round(((adjustingProduct.value?.quantity ?? 0) + adjustmentDelta.value) * 1000) / 1000,
)

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

function openAdjustStock(product: Product) {
  adjustingProduct.value = product
  adjustmentMode.value = 'add'
  adjustmentQuantity.value = 1
  adjustmentReason.value = 'Stock count adjustment'
  adjustmentError.value = ''
}

function closeAdjustStock() {
  if (adjusting.value) return
  adjustingProduct.value = null
  adjustmentError.value = ''
}

async function submitStockAdjustment() {
  if (!adjustingProduct.value) return
  const quantity = Number(adjustmentQuantity.value)
  if (!Number.isFinite(quantity) || quantity <= 0) {
    adjustmentError.value = 'Enter a stock change greater than zero.'
    return
  }
  if (resultingStock.value < 0) {
    adjustmentError.value = 'Stock adjustment cannot leave a negative quantity.'
    return
  }

  adjusting.value = true
  adjustmentError.value = ''
  try {
    await apiRequest<{ product: Product }>(`/products/${adjustingProduct.value.id}/stock-adjustments`, {
      method: 'POST',
      body: {
        delta: adjustmentDelta.value,
        reason: adjustmentReason.value.trim() || 'Stock count adjustment',
      },
    })
    adjustingProduct.value = null
    await load()
  } catch (reason) {
    adjustmentError.value = reason instanceof Error ? reason.message : 'Stock could not be adjusted.'
  } finally {
    adjusting.value = false
  }
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
  }
}

onMounted(load)
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

      <div class="table-container" style="flex: 1; min-height: 0; overflow-y: auto;">
        <table>
          <thead>
            <tr>
              <th class="text-center">Product</th>
              <th class="text-center">Category</th>
              <th class="text-center">Current Stock</th>
              <th class="text-center">Unit</th>
              <th class="text-center">Projected Revenue</th>
              <th class="text-center">Projected Profit</th>
              <th class="text-center">Profit per Unit</th>
              <th class="text-center">Status</th>
              <th v-if="canManage" class="text-center">Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading">
              <td :colspan="canManage ? 9 : 8" class="empty-message">Loading stock levels...</td>
            </tr>
            <tr v-else-if="filteredProducts.length === 0">
              <td :colspan="canManage ? 9 : 8" class="empty-message">No products found.</td>
            </tr>
            <tr v-for="item in filteredProducts" v-else :key="item.id" style="height: 38px;">
              <td class="text-center font-medium text-slate-200">{{ item.name }}</td>
              <td class="text-center">{{ item.category || 'General' }}</td>
              <td class="text-center font-semibold">{{ formatQuantity(item.quantity) }}</td>
              <td class="text-center text-slate-400">{{ item.unit }}</td>
              <td class="text-center">{{ formatMoney(projectedRevenue(item), currency) }}</td>
              <td
                class="text-center font-semibold"
                :class="projectedProfit(item) >= 0 ? 'profit-positive' : 'profit-negative'"
              >
                {{ formatMoney(projectedProfit(item), currency) }}
              </td>
              <td
                class="text-center font-semibold"
                :class="profitPerUnit(item) >= 0 ? 'profit-positive' : 'profit-negative'"
              >
                {{ formatMoney(profitPerUnit(item), currency) }}
              </td>
              <td class="text-center">
                <span class="status-badge" :class="getStockStatus(item).class">
                  {{ getStockStatus(item).text }}
                </span>
              </td>
              <td v-if="canManage" class="text-center">
                <button
                  type="button"
                  class="stock-adjust-button"
                  :aria-label="`Adjust stock for ${item.name}`"
                  @click="openAdjustStock(item)"
                >
                  <SlidersHorizontal :size="15" />
                  Adjust
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-if="adjustingProduct" class="adjustment-backdrop" @click.self="closeAdjustStock">
      <section class="adjustment-dialog" role="dialog" aria-modal="true" aria-labelledby="stock-adjustment-title">
        <header class="adjustment-header">
          <div>
            <h2 id="stock-adjustment-title">Adjust Stock</h2>
            <p>{{ adjustingProduct.name }} · Current: {{ formatQuantity(adjustingProduct.quantity) }} {{ adjustingProduct.unit }}</p>
          </div>
          <button type="button" class="adjustment-close" aria-label="Close stock adjustment" :disabled="adjusting" @click="closeAdjustStock">
            <X :size="19" />
          </button>
        </header>
        <form class="adjustment-form" @submit.prevent="submitStockAdjustment">
          <p v-if="adjustmentError" class="inline-error" role="alert">{{ adjustmentError }}</p>
          <label>
            Adjustment type
            <select v-model="adjustmentMode">
              <option value="add">Add to stock</option>
              <option value="remove">Remove from stock</option>
              <option value="set">Set exact stock count</option>
            </select>
          </label>
          <label>
            Quantity
            <input v-model.number="adjustmentQuantity" type="number" min="0.001" step="any" required />
          </label>
          <label>
            Resulting stock
            <input :value="`${formatQuantity(resultingStock)} ${adjustingProduct.unit}`" type="text" disabled />
          </label>
          <label>
            Reason
            <input v-model.trim="adjustmentReason" type="text" maxlength="160" placeholder="Stock count, restock, damaged item…" />
          </label>
          <footer class="adjustment-actions">
            <button type="button" class="adjustment-cancel" :disabled="adjusting" @click="closeAdjustStock">Cancel</button>
            <button type="submit" class="adjustment-submit" :disabled="adjusting">
              {{ adjusting ? 'Updating…' : 'Apply Adjustment' }}
            </button>
          </footer>
        </form>
      </section>
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
  padding: 1.5rem;
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
  margin-bottom: 0.75rem;
  padding-bottom: 0.75rem;
  border-bottom: 1px solid var(--border-subtle);
}

.summary-badges {
  display: flex;
  gap: 0.75rem;
  align-items: center;
}

.summary-badge {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 0.5rem;
  padding: 0.35rem 0.85rem;
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
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
  gap: 0.65rem;
  align-items: center;
  flex-wrap: wrap;
}

.search-bar {
  position: relative;
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
  width: min(210px, 32vw);
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
  padding: 0.4rem 0.75rem;
  color: var(--text-primary);
  font-size: 0.825rem;
  cursor: pointer;
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

.table-container {
  flex-grow: 1;
  min-height: 0;
  overflow-x: auto;
  overflow-y: auto;
}

.stock-adjust-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  min-width: 44px;
  min-height: 44px;
  padding: 0.4rem 0.65rem;
  border: 1px solid var(--border-color);
  border-radius: 0.45rem;
  color: var(--text-secondary);
  background: var(--bg-surface);
  font: inherit;
  font-size: 0.78rem;
  cursor: pointer;
}

.stock-adjust-button:hover,
.stock-adjust-button:focus-visible {
  color: var(--text-primary);
  border-color: var(--accent-color);
}

.adjustment-backdrop {
  position: fixed;
  z-index: 100;
  inset: 0;
  display: grid;
  place-items: center;
  padding: 16px max(16px, env(safe-area-inset-right)) max(16px, env(safe-area-inset-bottom)) max(16px, env(safe-area-inset-left));
  background: rgba(2, 6, 23, 0.76);
}

.adjustment-dialog {
  width: min(100%, 480px);
  max-height: min(90vh, 760px);
  overflow: auto;
  padding: 1.25rem;
  border: 1px solid var(--border-color);
  border-radius: 0.9rem;
  color: var(--text-primary);
  background: var(--bg-sidebar);
  box-shadow: 0 24px 70px rgba(0, 0, 0, 0.45);
}

.adjustment-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1.25rem;
}

.adjustment-header h2 {
  margin: 0;
  font-size: 1.15rem;
}

.adjustment-header p {
  margin: 0.35rem 0 0;
  color: var(--text-muted);
  font-size: 0.82rem;
}

.adjustment-close,
.adjustment-cancel,
.adjustment-submit {
  min-width: 44px;
  min-height: 44px;
  border-radius: 0.45rem;
  cursor: pointer;
}

.adjustment-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border-color);
  color: var(--text-secondary);
  background: transparent;
}

.adjustment-form {
  display: grid;
  gap: 0.9rem;
}

.adjustment-form label {
  display: grid;
  gap: 0.4rem;
  color: var(--text-secondary);
  font-size: 0.85rem;
  font-weight: 600;
}

.adjustment-form input,
.adjustment-form select {
  width: 100%;
  min-height: 46px;
  padding: 0.55rem 0.7rem;
  border: 1px solid var(--border-color);
  border-radius: 0.45rem;
  color: var(--text-primary);
  background: var(--bg-surface);
  font: inherit;
  font-size: 16px;
}

.adjustment-actions {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 0.6rem;
  margin-top: 0.3rem;
}

.adjustment-cancel,
.adjustment-submit {
  padding: 0 0.9rem;
  border: 1px solid var(--border-color);
  font: inherit;
  font-size: 0.85rem;
  font-weight: 600;
}

.adjustment-cancel {
  color: var(--text-primary);
  background: var(--bg-surface);
}

.adjustment-submit {
  color: white;
  background: var(--accent-color);
}

.adjustment-cancel:disabled,
.adjustment-submit:disabled {
  opacity: 0.6;
  cursor: wait;
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
</style>
