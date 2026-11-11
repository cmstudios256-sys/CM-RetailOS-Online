<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'
import { Calendar, Download, Eye, Printer, Search, Share2, X } from '@lucide/vue'
import { apiRequest, formatDate, formatISODate, formatMoney, formatQuantity, formatTime, readOnlinePrefs, type Sale } from '../api'
import AppDatePicker from './AppDatePicker.vue'
import MobileReceiptShare from './MobileReceiptShare.vue'
import { isTouchDevice } from '../utils/device'
import { downloadCsv, escapeHtml, openPrintDocument, receiptNumber, shortSaleId } from '../utils/reporting'
import { useDelayedSkeleton } from '../utils/useDelayedSkeleton'
import { useMobileOrTabletBreakpoint, usePhoneBreakpoint, useTabletBreakpoint } from '../utils/breakpoints'
import {
  getDefaultBluetoothPrinter,
  buildEscPosReceipt,
  sendToBluetoothPrinter,
} from '../utils/bluetoothPrinter'

type ShopReceiptSettings = {
  shopName: string
  currency: string
  address: string
  phone: string
  receiptHeader: string
  receiptFooter: string
  receiptShowPhone: boolean
}

const props = defineProps<{ currency: string; shopName: string }>()
const sales = ref<Sale[]>([])
const searchTerm = ref('')
const selectedDate = ref('')
const selectedSale = ref<Sale | null>(null)
const saleToShare = ref<Sale | null>(null)
const mobileTouch = ref(false)
const isDetailModalOpen = ref(false)
const shopSettings = ref<ShopReceiptSettings | null>(null)
const loading = ref(true)
const showSkeleton = useDelayedSkeleton(loading, 200)
const error = ref('')

const filteredSales = computed(() =>
  sales.value.filter(sale => {
    const query = searchTerm.value.trim().toLowerCase()
    const matchesSearch = !query
      || sale.id.toLowerCase().includes(query)
      || shortSaleId(sale.id).toLowerCase().includes(query)
      || receiptNumber(sale).toLowerCase().includes(query)
      || sale.cashierName.toLowerCase().includes(query)
      || sale.customerName.toLowerCase().includes(query)
      || sale.items.some(item => item.productName.toLowerCase().includes(query))
    const matchesDate = !selectedDate.value || formatISODate(sale.completedAt) === selectedDate.value
    return matchesSearch && matchesDate
  }),
)

const { isPhone } = usePhoneBreakpoint()
const { isTablet } = useTabletBreakpoint()
const { isMobileOrTablet } = useMobileOrTabletBreakpoint()
const headerRef = ref<HTMLElement | null>(null)
const salesListRef = ref<HTMLElement | null>(null)
const salesListVirtualizer = useVirtualizer(computed(() => ({
  count: filteredSales.value.length,
  getScrollElement: () => salesListRef.value,
  estimateSize: () => 44,
  overscan: 8,
})))
const virtualSalesListItems = computed(() => salesListVirtualizer.value.getVirtualItems())

const mobileSalesListRef = ref<HTMLElement | null>(null)
const mobileSalesVirtualizer = useVirtualizer(computed(() => ({
  count: filteredSales.value.length,
  getScrollElement: () => mobileSalesListRef.value,
  estimateSize: () => 240,
  overscan: 4,
})))
const virtualMobileSales = computed(() => mobileSalesVirtualizer.value.getVirtualItems())

function syncHeaderScroll(e: Event) {
  const target = e.target as HTMLElement
  if (headerRef.value) {
    headerRef.value.scrollLeft = target.scrollLeft
  }
}

watch(
  () => [searchTerm.value, selectedDate.value, filteredSales.value.length],
  () => {
    salesListRef.value?.scrollTo({ top: 0 })
    mobileSalesListRef.value?.scrollTo({ top: 0 })
  },
)

function getSaleItemsCount(sale: Sale) {
  return sale.items.reduce((sum, item) => sum + item.quantity, 0)
}

function calculateTotalCost(sale: Sale) {
  return sale.items.reduce((sum, item) => sum + (item.unitCost || 0) * item.quantity, 0)
}

function calculateProfit(sale: Sale) {
  return sale.total - calculateTotalCost(sale)
}

function viewSaleDetails(sale: Sale) {
  selectedSale.value = sale
  isDetailModalOpen.value = true
}

function closeDetailModal() {
  isDetailModalOpen.value = false
  selectedSale.value = null
}

function shareSaleOnMobile(sale: Sale) {
  if (mobileTouch.value || isMobileOrTablet.value) saleToShare.value = sale
}

function exportSalesHistory() {
  if (!filteredSales.value.length) return
  downloadCsv(
    `sales_history_${new Date().toISOString().slice(0, 10)}.csv`,
    ['Sale ID', 'Date', 'Cashier', 'Customer', 'Payment Method', 'Items', 'Subtotal', 'Discount', 'Total', 'Profit', 'Status'],
    filteredSales.value.map(sale => [
      `#${shortSaleId(sale.id)}`,
      `${formatDate(sale.completedAt)} ${formatTime(sale.completedAt)}`,
      sale.cashierName || 'Admin',
      sale.customerName || 'Walk-in customer',
      sale.paymentMethod,
      formatQuantity(getSaleItemsCount(sale)),
      sale.subtotal,
      sale.discount,
      sale.total,
      calculateProfit(sale),
      'Completed',
    ]),
  )
}

function printSalesHistory() {
  if (!filteredSales.value.length) return
  const totalRevenue = filteredSales.value.reduce((sum, sale) => sum + sale.total, 0)
  const totalProfit = filteredSales.value.reduce((sum, sale) => sum + calculateProfit(sale), 0)
  const rows = filteredSales.value.map(sale => `
    <tr>
      <td>#${escapeHtml(shortSaleId(sale.id))}</td>
      <td>${escapeHtml(formatDate(sale.completedAt))} ${escapeHtml(formatTime(sale.completedAt))}</td>
      <td class="num">${escapeHtml(formatQuantity(getSaleItemsCount(sale)))}</td>
      <td class="num">${escapeHtml(formatMoney(sale.total, props.currency))}</td>
      <td class="num">${escapeHtml(formatMoney(calculateProfit(sale), props.currency))}</td>
      <td>Completed</td>
    </tr>
  `).join('')

  openPrintDocument(
    `${props.shopName} Sales History Report`,
    `<div class="meta">
      <div>
        <h1>Sales History Report</h1>
        <p>${escapeHtml(props.shopName)}${selectedDate.value ? ` · Date: ${escapeHtml(selectedDate.value)}` : ' · Full Period'}</p>
      </div>
      <div>
        <p>${escapeHtml(formatDate(new Date()))} ${escapeHtml(formatTime(new Date()))}</p>
        <p>${filteredSales.value.length} transactions · Revenue ${escapeHtml(formatMoney(totalRevenue, props.currency))} · Profit ${escapeHtml(formatMoney(totalProfit, props.currency))}</p>
      </div>
    </div>
    <table>
      <thead>
        <tr><th>Sale ID</th><th>Date</th><th class="num">Items</th><th class="num">Total</th><th class="num">Profit</th><th>Status</th></tr>
      </thead>
      <tbody>${rows}</tbody>
    </table>`,
  )
}

function printReceipt(sale: Sale) {
  const settings = shopSettings.value
  const storeTitle = settings?.shopName || props.shopName

  if (isMobileOrTablet.value) {
    const btPrinter = getDefaultBluetoothPrinter()
    if (btPrinter) {
      const bytes = buildEscPosReceipt(sale, {
        shopName: storeTitle,
        currency: props.currency,
        paperWidth: btPrinter.paperWidth || '58mm',
      })
      sendToBluetoothPrinter(btPrinter, bytes)
      return
    } else {
      saleToShare.value = sale
      return
    }
  }

  const prefs = readOnlinePrefs()
  const showLogo = prefs.receiptsSettings?.showLogo !== false
  const showCashier = prefs.receiptsSettings?.showCashier !== false
  const receiptMaxWidth = prefs.receiptsSettings?.paperWidth === '58mm' ? '220px' : '300px'
  const logoHtml = showLogo && prefs.logo
    ? `<div style="display:flex;justify-content:center;margin-bottom:8px;"><img src="${escapeHtml(prefs.logo)}" style="width:48px;height:48px;object-fit:contain;filter:brightness(0);" /></div>`
    : ''
  const rows = sale.items.map(item => `
    <div style="margin-bottom: 6px;">
      <div style="font-weight: 700;">${escapeHtml(item.productName)}</div>
      <div style="display: flex; justify-content: space-between; padding-left: 8px;">
        <span>${escapeHtml(formatQuantity(item.quantity))} x ${escapeHtml(formatMoney(item.unitPrice, props.currency))}</span>
        <span>${escapeHtml(formatMoney(item.lineTotal, props.currency))}</span>
      </div>
    </div>
  `).join('')

  const discountRows = sale.discount > 0
    ? `<div style="display:flex;justify-content:space-between;margin:3px 0;"><span>Subtotal</span><span>${escapeHtml(formatMoney(sale.subtotal, props.currency))}</span></div>
       <div style="display:flex;justify-content:space-between;margin:3px 0;"><span>Discount</span><span>-${escapeHtml(formatMoney(sale.discount, props.currency))}</span></div>`
    : ''

  openPrintDocument(
    `Receipt ${receiptNumber(sale)}`,
    `<div style="max-width:${receiptMaxWidth};margin:0 auto;font-family:'Courier New',Courier,monospace;font-size:12px;color:#000;">
      <div style="text-align:center;margin-bottom:8px;">
        ${logoHtml}
        <div style="font-size:16px;font-weight:700;text-transform:uppercase;">${escapeHtml(storeTitle)}</div>
        ${settings?.address ? `<div>${escapeHtml(settings.address)}</div>` : ''}
        ${settings?.receiptShowPhone && settings?.phone ? `<div>Tel: ${escapeHtml(settings.phone)}</div>` : ''}
        ${settings?.receiptHeader ? `<div style="margin-top:4px;font-style:italic;">${escapeHtml(settings.receiptHeader)}</div>` : ''}
      </div>
      <div style="text-align:center;">--------------------------------</div>
      <div style="margin:6px 0;">
        <div style="display:flex;justify-content:space-between;"><span>Receipt:</span><span>#${escapeHtml(receiptNumber(sale))}</span></div>
        <div style="display:flex;justify-content:space-between;"><span>Sale ID:</span><span>#${escapeHtml(shortSaleId(sale.id))}</span></div>
        <div style="display:flex;justify-content:space-between;"><span>Date:</span><span>${escapeHtml(formatDate(sale.completedAt))} ${escapeHtml(formatTime(sale.completedAt))}</span></div>
        ${showCashier ? `<div style="display:flex;justify-content:space-between;"><span>Cashier:</span><span>${escapeHtml(sale.cashierName || 'Admin')}</span></div>` : ''}
        ${sale.customerName ? `<div style="display:flex;justify-content:space-between;"><span>Customer:</span><span>${escapeHtml(sale.customerName)}</span></div>` : ''}
      </div>
      <div style="text-align:center;">--------------------------------</div>
      <div style="display:flex;justify-content:space-between;font-weight:700;text-transform:uppercase;margin:4px 0;"><span>Item</span><span>Amount</span></div>
      <div style="text-align:center;">--------------------------------</div>
      <div style="margin:6px 0;">${rows}</div>
      <div style="text-align:center;">--------------------------------</div>
      <div style="margin:6px 0;">
        ${discountRows}
        <div style="display:flex;justify-content:space-between;font-weight:700;font-size:14px;margin-top:4px;"><span>TOTAL</span><span>${escapeHtml(formatMoney(sale.total, props.currency))}</span></div>
      </div>
      <div style="text-align:center;">--------------------------------</div>
      <div style="text-align:center;margin-top:8px;">
        <div>${escapeHtml(settings?.receiptFooter || 'Thank you for shopping with us!')}</div>
        <div style="font-size:10px;color:#555;margin-top:6px;">Powered by CM-RetailOS</div>
      </div>
    </div>`,
  )
}

async function loadSales() {
  loading.value = true
  error.value = ''
  try {
    const [result, settingsResult] = await Promise.all([
      apiRequest<{ sales: Sale[] }>('/sales'),
      apiRequest<{ settings: ShopReceiptSettings }>('/settings').catch(() => null),
    ])
    sales.value = result.sales
    if (settingsResult?.settings) {
      shopSettings.value = settingsResult.settings
    }
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Sales history could not be loaded.'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  mobileTouch.value = isTouchDevice()
  void loadSales()
})
</script>

<template>
  <section class="view-section sales-view">
    <div v-if="error" class="inline-error" role="alert">{{ error }}</div>

    <div class="card">
      <div class="card-header">
        <h2 class="card-title">Sales History</h2>
        <div class="header-controls">
          <div class="search-bar">
            <Search class="search-icon" :size="18" />
            <input v-model="searchTerm" type="text" placeholder="Search by ID..." aria-label="Search sales by ID" />
          </div>
          <AppDatePicker
            v-model="selectedDate"
            placeholder="Filter by Date"
            title="Filter Sales by Date"
            aria-label="Filter sales by date"
          />
          <button
            type="button"
            class="export-btn print-btn"
            :disabled="!filteredSales.length"
            @click="printSalesHistory"
          >
            <Printer :size="16" />
            Print Report
          </button>
          <button
            type="button"
            class="export-btn"
            :disabled="!filteredSales.length"
            @click="exportSalesHistory"
          >
            <Download :size="16" />
            Export
          </button>
        </div>
      </div>

      <!-- Mobile / Tablet Card Layout (<1024px) -->
      <div v-if="isMobileOrTablet" ref="mobileSalesListRef" class="mobile-cards-scroll-body">
        <div v-if="showSkeleton" class="mobile-skeleton-cards">
          <div v-for="i in 4" :key="i" class="mobile-skeleton-card">
            <div class="mobile-skel-header">
              <div class="skeleton-box skeleton-shimmer" style="height: 18px; width: 35%;"></div>
              <div class="skeleton-box skeleton-shimmer" style="height: 14px; width: 40%;"></div>
            </div>
            <div class="mobile-skel-badges">
              <div class="skeleton-box skeleton-shimmer" style="height: 20px; width: 60px; border-radius: 999px;"></div>
              <div class="skeleton-box skeleton-shimmer" style="height: 20px; width: 70px; border-radius: 999px;"></div>
              <div class="skeleton-box skeleton-shimmer" style="height: 20px; width: 50px; border-radius: 999px;"></div>
            </div>
            <div class="mobile-skel-grid">
              <div v-for="j in 4" :key="j" class="skeleton-box skeleton-shimmer" style="height: 38px; border-radius: 6px;"></div>
            </div>
            <div class="mobile-skel-actions">
              <div class="skeleton-box skeleton-shimmer" style="height: 44px; width: 100%; border-radius: 8px;"></div>
            </div>
          </div>
        </div>
        <div v-else-if="loading && !showSkeleton" class="empty-message"></div>
        <div v-else-if="filteredSales.length === 0" class="empty-message">No sales records found.</div>
        <div
          v-else
          class="virtual-cards-spacer"
          :style="{ height: `${mobileSalesVirtualizer.getTotalSize()}px` }"
        >
          <div
            v-for="virtualRow in virtualMobileSales"
            :key="String(virtualRow.key)"
            :ref="el => mobileSalesVirtualizer.measureElement(el as HTMLElement)"
            :data-index="virtualRow.index"
            class="virtual-cards-row"
            :style="{
              transform: `translateY(${virtualRow.start}px)`,
            }"
          >
            <article
              v-if="filteredSales[virtualRow.index]"
              class="mobile-ui-card"
              tabindex="0"
              role="button"
              :aria-label="`Sale #${shortSaleId(filteredSales[virtualRow.index].id)}`"
              @click="viewSaleDetails(filteredSales[virtualRow.index])"
              @keydown.enter="viewSaleDetails(filteredSales[virtualRow.index])"
            >
              <div class="mobile-card-header">
                <span class="mobile-card-id">#{{ shortSaleId(filteredSales[virtualRow.index].id) }}</span>
                <span class="mobile-card-date">
                  {{ formatDate(filteredSales[virtualRow.index].completedAt) }} · {{ formatTime(filteredSales[virtualRow.index].completedAt) }}
                </span>
              </div>
              <div class="mobile-card-badges">
                <span class="mobile-card-pill">{{ formatQuantity(getSaleItemsCount(filteredSales[virtualRow.index])) }} items</span>
                <span class="status-badge status-completed">Completed</span>
                <span class="mobile-card-pill mobile-card-payment">{{ filteredSales[virtualRow.index].paymentMethod }}</span>
              </div>
              <dl class="mobile-card-grid">
                <div>
                  <dt>Total</dt>
                  <dd class="mobile-card-total-val">{{ formatMoney(filteredSales[virtualRow.index].total, currency) }}</dd>
                </div>
                <div>
                  <dt>Profit</dt>
                  <dd :class="calculateProfit(filteredSales[virtualRow.index]) >= 0 ? 'profit-positive' : 'profit-negative'">
                    {{ formatMoney(calculateProfit(filteredSales[virtualRow.index]), currency) }}
                  </dd>
                </div>
                <div>
                  <dt>Customer</dt>
                  <dd>{{ filteredSales[virtualRow.index].customerName || 'Walk-in' }}</dd>
                </div>
                <div>
                  <dt>Cashier</dt>
                  <dd>{{ filteredSales[virtualRow.index].cashierName || 'Admin' }}</dd>
                </div>
              </dl>
              <div class="mobile-card-actions">
                <button
                  type="button"
                  class="mobile-card-btn primary-action"
                  @click.stop="viewSaleDetails(filteredSales[virtualRow.index])"
                >
                  <Eye :size="15" />
                  View Receipt
                </button>
                <button
                  type="button"
                  class="mobile-card-btn share-action"
                  aria-label="Share receipt to WhatsApp"
                  @click.stop="shareSaleOnMobile(filteredSales[virtualRow.index])"
                >
                  <Share2 :size="15" />
                  Share
                </button>
              </div>
            </article>
          </div>
        </div>
      </div>

      <!-- Desktop / Tablet Table Layout (>=640px) -->
      <div v-else class="virtual-list-container">
        <div ref="headerRef" class="virtual-list-header-wrapper">
          <div class="virtual-list-header sales-list-header">
            <span>Sale ID</span>
            <span>Date</span>
            <span>Items</span>
            <span>Total</span>
            <span>Profit</span>
            <span>Status</span>
            <span>Actions</span>
          </div>
        </div>
        <div v-if="showSkeleton" class="sales-skeleton-list">
          <div v-for="i in 8" :key="i" class="sales-skeleton-row">
            <div class="skeleton-box skeleton-shimmer" style="width: 70px; height: 18px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 110px; height: 18px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 50px; height: 18px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 85px; height: 18px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 80px; height: 18px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 60px; height: 22px; border-radius: 999px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 65px; height: 26px; border-radius: 6px;"></div>
          </div>
        </div>
        <div v-else-if="loading && !showSkeleton" class="empty-message"></div>
        <div v-else-if="filteredSales.length === 0" class="empty-message">No sales records found.</div>
        <div
          v-else
          ref="salesListRef"
          class="virtual-list-scroll-body"
          @scroll="syncHeaderScroll"
        >
          <div
            class="virtual-list-spacer"
            :style="{ height: `${salesListVirtualizer.getTotalSize()}px` }"
          >
            <div
              v-for="virtualRow in virtualSalesListItems"
              :key="String(virtualRow.key)"
              class="virtual-list-row sales-list-header"
              :style="{ transform: `translateY(${virtualRow.start}px)`, height: `${virtualRow.size}px` }"
            >
              <template v-if="filteredSales[virtualRow.index]">
                <span class="font-medium text-slate-200">#{{ shortSaleId(filteredSales[virtualRow.index].id) }}</span>
                <span>{{ formatDate(filteredSales[virtualRow.index].completedAt) }} {{ formatTime(filteredSales[virtualRow.index].completedAt) }}</span>
                <span>{{ formatQuantity(getSaleItemsCount(filteredSales[virtualRow.index])) }}</span>
                <span class="font-semibold">{{ formatMoney(filteredSales[virtualRow.index].total, currency) }}</span>
                <span
                  class="font-semibold"
                  :class="calculateProfit(filteredSales[virtualRow.index]) >= 0 ? 'profit-positive' : 'profit-negative'"
                >
                  {{ formatMoney(calculateProfit(filteredSales[virtualRow.index]), currency) }}
                </span>
                <span>
                  <span class="status-badge status-completed">Completed</span>
                </span>
                <span class="actions-cell">
                  <button
                    v-if="mobileTouch"
                    type="button"
                    class="action-btn mobile-sales-share"
                    aria-label="Share receipt to WhatsApp"
                    title="Share receipt to WhatsApp"
                    @click.stop="shareSaleOnMobile(filteredSales[virtualRow.index])"
                  >
                    <Share2 :size="16" />
                  </button>
                  <button type="button" class="action-btn" @click="viewSaleDetails(filteredSales[virtualRow.index])">
                    <Eye :size="16" />
                    View
                  </button>
                </span>
              </template>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Sale Details Modal -->
    <div v-if="isDetailModalOpen && selectedSale" class="modal-backdrop sale-details-backdrop" @click.self="closeDetailModal">
      <div class="modal-content sale-details-modal" v-draggable="'.modal-header'">
        <div class="modal-header">
          <h3>Sale Details</h3>
          <button type="button" class="close-btn" aria-label="Close sale details" @click="closeDetailModal">
            <X :size="20" />
          </button>
        </div>

        <div class="modal-body">
          <div class="sale-info-grid">
            <div class="info-item">
              <span class="info-label">Sale ID</span>
              <span class="info-value">#{{ shortSaleId(selectedSale.id) }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">Date</span>
              <span class="info-value">{{ formatDate(selectedSale.completedAt) }} {{ formatTime(selectedSale.completedAt) }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">Cashier</span>
              <span class="info-value">{{ selectedSale.cashierName || 'Admin' }}</span>
            </div>
            <div class="info-item">
              <span class="info-label">Status</span>
              <span class="status-badge status-completed" style="width: fit-content;">Completed</span>
            </div>
          </div>

          <h4 class="items-heading">Items Purchased</h4>
          <div class="modal-table-container">
            <table class="items-table">
              <thead>
                <tr>
                  <th>Item Name</th>
                  <th class="text-center">Qty</th>
                  <th class="text-right">Price</th>
                  <th class="text-right">Total</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="(item, idx) in selectedSale.items" :key="`${item.productId}-${idx}`">
                  <td class="font-medium item-name-cell">{{ item.productName }}</td>
                  <td class="text-center">{{ formatQuantity(item.quantity) }}</td>
                  <td class="text-right">{{ formatMoney(item.unitPrice, currency) }}</td>
                  <td class="text-right font-semibold">{{ formatMoney(item.lineTotal, currency) }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div class="sale-summary">
            <div class="summary-row">
              <span>Cost of Goods</span>
              <span>{{ formatMoney(calculateTotalCost(selectedSale), currency) }}</span>
            </div>
            <div v-if="selectedSale.discount > 0" class="summary-row" style="color: #f59e0b;">
              <span>Discount Applied</span>
              <span>-{{ formatMoney(selectedSale.discount, currency) }}</span>
            </div>
            <div class="summary-row" :class="calculateProfit(selectedSale) >= 0 ? 'profit-positive' : 'profit-negative'">
              <span>Gross Profit</span>
              <span>{{ formatMoney(calculateProfit(selectedSale), currency) }}</span>
            </div>
            <div class="summary-row total-row">
              <span>Total Revenue</span>
              <span>{{ formatMoney(selectedSale.total, currency) }}</span>
            </div>
          </div>
        </div>

        <div class="modal-footer">
          <button type="button" class="btn-secondary" @click="closeDetailModal">Close</button>
          <button
            v-if="isMobileOrTablet || mobileTouch"
            type="button"
            class="btn-secondary mobile-sales-share-button"
            @click="shareSaleOnMobile(selectedSale)"
          >
            <Share2 :size="16" />
            Share to WhatsApp
          </button>
          <button type="button" class="btn-primary" @click="printReceipt(selectedSale)">
            <Printer :size="16" />
            Print Receipt
          </button>
        </div>
      </div>
    </div>
    <Teleport to="body">
      <MobileReceiptShare
        :sale="saleToShare"
        :shop-name="shopSettings?.shopName || shopName"
        :currency="currency"
        @close="saleToShare = null"
      />
    </Teleport>
  </section>
</template>

<style scoped>
.view-section {
  display: flex;
  flex-direction: column;
  flex-grow: 1;
  min-height: 0;
}

.mobile-sales-share-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
}

.mobile-sales-share {
  margin-right: 6px;
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

.card-title {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 600;
  color: var(--text-primary);
}

.header-controls {
  display: flex;
  gap: 0.75rem;
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
  border-radius: 8px;
  padding: 0.45rem 0.75rem 0.45rem 2.25rem;
  color: var(--text-primary);
  width: 230px;
  height: 38px;
  box-sizing: border-box;
  font-size: 0.825rem;
}

.search-bar input:focus {
  outline: none;
  border-color: var(--accent-color);
}

.date-filter-wrapper {
  display: flex;
  align-items: center;
  gap: 0.35rem;
}

.filter-btn,
.export-btn {
  background-color: var(--bg-surface);
  color: var(--text-secondary);
  padding: 0.45rem 0.9rem;
  height: 38px;
  box-sizing: border-box;
  border-radius: 8px;
  border: 1px solid var(--border-color);
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  font-weight: 500;
  font-size: 0.825rem;
  white-space: nowrap;
  transition: all 0.2s;
}

.filter-btn:hover,
.export-btn:hover:not(:disabled) {
  background-color: var(--bg-hover);
  color: var(--text-primary);
}

.export-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.date-picker-btn {
  position: relative;
  overflow: hidden;
  white-space: nowrap;
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
  border-radius: 8px;
  width: 38px;
  height: 38px;
  padding: 0;
  box-sizing: border-box;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
}

.clear-date-btn:hover {
  background-color: #ef4444;
  color: #ffffff;
}

.print-btn {
  background-color: rgba(14, 165, 233, 0.1);
  color: #38bdf8;
  border-color: rgba(14, 165, 233, 0.3);
}

.print-btn:hover:not(:disabled) {
  background-color: var(--accent-color);
  color: #ffffff;
}

.table-container {
  flex-grow: 1;
  min-height: 0;
}

.virtual-list-container {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  position: relative;
  background-color: var(--bg-sidebar);
  border: 1px solid var(--border-subtle);
  border-radius: 12px;
  overflow: hidden;
}

.virtual-list-header-wrapper {
  overflow-x: hidden;
  flex-shrink: 0;
  background-color: var(--bg-surface);
  border-bottom: 1px solid var(--border-subtle);
}

.virtual-list-scroll-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  overflow-x: auto;
  position: relative;
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

.sales-list-header {
  grid-template-columns: minmax(110px, 1.1fr) minmax(175px, 1.6fr) minmax(65px, 0.6fr) minmax(110px, 1.1fr) minmax(110px, 1.1fr) minmax(105px, 1fr) minmax(95px, 0.9fr);
  min-width: 770px;
}

.virtual-list-header {
  min-height: 40px;
  background-color: var(--bg-surface);
  color: var(--text-muted);
  font-size: 0.75rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.virtual-list-header > span,
.virtual-list-row > span {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0.45rem 0.65rem;
  border-right: 1px solid var(--border-subtle);
  border-bottom: 1px solid var(--border-subtle);
  text-align: center;
  font-size: 0.825rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  height: 100%;
  box-sizing: border-box;
  min-width: 0;
}

.virtual-list-header > span:last-child,
.virtual-list-row > span:last-child {
  border-right: none;
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

.actions-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
}

.action-btn {
  background: rgba(14, 165, 233, 0.1);
  border: 1px solid rgba(14, 165, 233, 0.25);
  color: #38bdf8;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 0.35rem;
  padding: 0.3rem 0.65rem;
  border-radius: 6px;
  font-size: 0.8rem;
  font-weight: 600;
  transition: all 0.15s ease;
}

.action-btn:hover {
  color: #ffffff;
  background-color: var(--accent-color, #0ea5e9);
  border-color: var(--accent-color, #0ea5e9);
}

table {
  width: 100%;
  min-width: 720px;
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
.text-center { text-align: center; }
.text-right { text-align: right; }

.profit-positive { color: #4ade80; }
.profit-negative { color: #f87171; }

.status-badge {
  padding: 0.25rem 0.6rem;
  border-radius: 9999px;
  font-size: 0.7rem;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
  border: none !important;
  outline: none !important;
  box-shadow: none !important;
}

.status-completed {
  color: #4ade80;
  background-color: rgba(34, 197, 94, 0.16);
  border: none !important;
}

.empty-message {
  text-align: center;
  padding: 1.5rem;
  color: var(--text-muted);
}

/* Sale Details Modal Base Styles */
.sale-details-backdrop {
  position: fixed;
  inset: 0;
  z-index: 100;
  display: flex;
  align-items: center;
  justify-content: center;
  background: transparent;
  backdrop-filter: none;
  -webkit-backdrop-filter: none;
}

.modal-content.sale-details-modal {
  background-color: var(--bg-sidebar, #0f172a);
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
  border-radius: 16px;
  width: 100%;
  max-width: 600px;
  max-height: 90vh;
  box-shadow: 0 25px 60px -12px rgba(0, 0, 0, 0.6);
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  overflow: hidden;
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1.25rem 1.5rem;
  border-bottom: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
  flex-shrink: 0;
}

.modal-header h3 {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 700;
  color: var(--text-primary, #f8fafc);
  letter-spacing: -0.01em;
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
  padding: 1.25rem 1.5rem;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 1rem;
  min-width: 0;
  box-sizing: border-box;
  -webkit-overflow-scrolling: touch;
}

.sale-info-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 12px 16px;
  background-color: rgba(15, 23, 42, 0.65);
  padding: 14px 16px;
  border-radius: 14px;
  border: none !important;
  outline: none !important;
  box-shadow: none !important;
}

.info-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.info-label {
  font-size: 0.72rem;
  color: #94a3b8;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  font-weight: 600;
}

.info-value {
  font-size: 0.95rem;
  font-weight: 600;
  color: #f1f5f9;
  word-break: break-word;
}

.sale-info-grid .info-item:first-child .info-value {
  color: #38bdf8;
  font-weight: 700;
}

.items-heading {
  margin: 4px 0 0;
  font-size: 0.95rem;
  font-weight: 700;
  color: #f1f5f9;
  letter-spacing: -0.01em;
}

.modal-table-container {
  border: none !important;
  outline: none !important;
  border-radius: 0;
  background: transparent !important;
  overflow-x: auto;
  -webkit-overflow-scrolling: touch;
  max-width: 100%;
  min-width: 0;
}

.items-table {
  width: 100%;
  min-width: 0;
  border-collapse: collapse;
  background: transparent !important;
}

.items-table th {
  background: transparent !important;
  background-color: transparent !important;
  color: #94a3b8;
  font-size: 0.72rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  padding: 10px 8px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  border-right: none;
  white-space: nowrap;
}

.items-table th:first-child {
  padding-left: 0;
}

.items-table th:last-child {
  padding-right: 0;
}

.items-table td {
  padding: 10px 8px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  border-right: none;
  font-size: 0.875rem;
  color: #e2e8f0;
}

.items-table td:first-child {
  padding-left: 0;
}

.items-table td:last-child {
  padding-right: 0;
}

.items-table tbody tr:last-child td {
  border-bottom: none;
}

.item-name-cell {
  color: #f8fafc;
  font-weight: 600;
}

.sale-summary {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  background-color: rgba(15, 23, 42, 0.65);
  padding: 14px 16px;
  border-radius: 14px;
  border: none !important;
  outline: none !important;
  box-shadow: none !important;
}

.summary-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.88rem;
  color: #94a3b8;
}

.summary-row span:last-child {
  font-weight: 600;
  color: #f1f5f9;
}

.profit-positive span:last-child,
.profit-positive {
  color: #34d399 !important;
  font-weight: 700 !important;
}

.profit-negative span:last-child,
.profit-negative {
  color: #f87171 !important;
  font-weight: 700 !important;
}

.total-row {
  border-top: 1px solid rgba(255, 255, 255, 0.08);
  padding-top: 0.75rem;
  margin-top: 0.25rem;
}

.total-row span:first-child {
  font-size: 1.05rem;
  font-weight: 700;
  color: #f8fafc;
}

.total-row span:last-child {
  font-size: 1.25rem !important;
  font-weight: 800 !important;
  color: #38bdf8 !important;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 14px 16px;
  border-top: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
  background-color: rgba(15, 23, 42, 0.5);
  flex-shrink: 0;
}

.modal-footer button {
  min-height: 44px;
  border-radius: 10px;
  font-size: 0.9rem;
  font-weight: 650;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 0 16px;
  border: none !important;
  outline: none !important;
  box-shadow: none !important;
  transition: all 0.15s ease-out;
}

.modal-footer button:active {
  transform: scale(0.98);
}

.modal-footer .btn-secondary {
  background: rgba(255, 255, 255, 0.08);
  color: #f1f5f9;
}

.modal-footer .btn-secondary:hover {
  background: rgba(255, 255, 255, 0.15);
  color: #ffffff;
}

.modal-footer .mobile-sales-share-button {
  background: #10b981 !important;
  color: #ffffff !important;
  font-weight: 700 !important;
}

.modal-footer .mobile-sales-share-button:hover {
  background: #059669 !important;
}

@media (max-width: 639px) {
  .card {
    border: none !important;
    outline: none !important;
    box-shadow: none !important;
    background: transparent !important;
    padding: 8px 8px 16px 8px !important;
  }

  .card-header {
    border-bottom: none !important;
    padding-bottom: 0.25rem !important;
    margin-bottom: 0.5rem !important;
  }

  .header-controls {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .search-bar {
    flex: 1 1 auto;
    min-width: 0;
  }

  .search-bar input {
    width: 100% !important;
    border: none !important;
    outline: none !important;
    box-shadow: none !important;
    background-color: rgba(30, 41, 59, 0.72) !important;
  }

  .filter-btn,
  .date-picker-btn,
  .export-btn {
    border: none !important;
    outline: none !important;
    box-shadow: none !important;
    background-color: rgba(30, 41, 59, 0.72) !important;
  }

  /* Mobile Light Theme Overrides */
  [data-theme='light'] .search-bar input {
    background-color: #ffffff !important;
    color: #0f172a !important;
    border: 1px solid #e2e8f0 !important;
  }

  [data-theme='light'] .filter-btn,
  [data-theme='light'] .date-picker-btn,
  [data-theme='light'] .export-btn {
    background-color: #ffffff !important;
    color: #334155 !important;
    border: 1px solid #e2e8f0 !important;
  }

  [data-theme='light'] .sale-details-backdrop {
    background: transparent !important;
    backdrop-filter: none !important;
    -webkit-backdrop-filter: none !important;
    color: #0f172a !important;
  }

  [data-theme='light'] .modal-content.sale-details-modal {
    background: #ffffff !important;
    color: #0f172a !important;
  }

  [data-theme='light'] .sale-details-header {
    background: #ffffff !important;
    color: #0f172a !important;
  }

  [data-theme='light'] .sale-meta-grid,
  [data-theme='light'] .sale-items-table-wrap {
    background: #f8fafc !important;
    color: #0f172a !important;
  }

  /* Mobile Sale Details Page View */
  .sale-details-backdrop {
    position: fixed !important;
    inset: 0 !important;
    z-index: 100 !important;
    padding: 0 !important;
    margin: 0 !important;
    width: 100% !important;
    height: 100% !important;
    max-width: 100vw !important;
    display: flex !important;
    flex-direction: column !important;
    background: #0b1320 !important;
    backdrop-filter: none !important;
    -webkit-backdrop-filter: none !important;
    overflow: hidden !important;
  }

  .modal-content.sale-details-modal {
    width: 100% !important;
    max-width: 100% !important;
    min-width: 0 !important;
    height: 100% !important;
    height: 100dvh !important;
    max-height: 100dvh !important;
    border-radius: 0 !important;
    border: none !important;
    outline: none !important;
    box-shadow: none !important;
    margin: 0 !important;
    padding: 0 !important;
    display: flex !important;
    flex-direction: column !important;
    background: #0b1320 !important;
    overflow: hidden !important;
  }

  .modal-header {
    padding: max(16px, env(safe-area-inset-top)) 16px 14px 16px !important;
    border-bottom: none !important;
    flex-shrink: 0;
  }

  .modal-body {
    padding: 0 14px 16px 14px !important;
    min-width: 0 !important;
    max-width: 100% !important;
    box-sizing: border-box !important;
    overflow-x: hidden !important;
    overflow-y: auto !important;
    gap: 12px !important;
    flex: 1 1 auto;
  }

  .sale-info-grid {
    padding: 12px 14px !important;
    gap: 10px 14px !important;
    border-radius: 12px !important;
  }

  .items-table {
    width: 100% !important;
    min-width: 0 !important;
  }

  .items-table th,
  .items-table td {
    padding: 8px 6px !important;
    font-size: 0.8rem !important;
  }

  .items-table th:first-child,
  .items-table td:first-child {
    padding-left: 0 !important;
  }

  .items-table th:last-child,
  .items-table td:last-child {
    padding-right: 0 !important;
  }

  .items-table td:first-child {
    max-width: 120px;
    white-space: normal !important;
    word-break: break-word !important;
  }

  .sale-summary {
    padding: 12px 14px !important;
    border-radius: 12px !important;
  }

  .modal-footer {
    padding: 12px 14px max(16px, env(safe-area-inset-bottom)) 14px !important;
    border-top: none !important;
    background: #0b1320 !important;
    gap: 10px !important;
    flex-shrink: 0;
  }

  .modal-footer button {
    flex: 1 1 0;
    min-height: 48px;
    border-radius: 10px;
    font-size: 0.92rem !important;
  }
}

[data-theme='light'] .modal-content,
[data-theme='light'] .sale-details-modal {
  background-color: #ffffff !important;
  border: 1px solid #e2e8f0 !important;
  box-shadow: 0 20px 40px -10px rgba(15, 23, 42, 0.12) !important;
}

@media (max-width: 639px) {
  [data-theme='light'] .sale-details-backdrop,
  [data-theme='light'] .modal-content.sale-details-modal,
  [data-theme='light'] .modal-footer {
    background: #ffffff !important;
  }
}

[data-theme='light'] .modal-header {
  border-bottom: 1px solid #e2e8f0 !important;
}

[data-theme='light'] .modal-header h3 {
  color: #0f172a !important;
}

[data-theme='light'] .close-btn {
  color: #64748b !important;
}

[data-theme='light'] .close-btn:hover {
  color: #0f172a !important;
}

[data-theme='light'] .sale-info-grid {
  background-color: #f8fafc !important;
  border: 1px solid #e2e8f0 !important;
}

[data-theme='light'] .info-label {
  color: #64748b !important;
}

[data-theme='light'] .info-value {
  color: #0f172a !important;
}

[data-theme='light'] .items-heading {
  color: #0f172a !important;
}

[data-theme='light'] .modal-table-container {
  border: none !important;
  background: transparent !important;
}

[data-theme='light'] .items-table th {
  background: transparent !important;
  background-color: transparent !important;
  color: #475569 !important;
  border-bottom: 1px solid #e2e8f0 !important;
}

[data-theme='light'] .items-table td {
  border-bottom: 1px solid #f1f5f9 !important;
  color: #0f172a !important;
}

[data-theme='light'] .item-name-cell {
  color: #0f172a !important;
}

[data-theme='light'] .sale-summary {
  background-color: #f8fafc !important;
  border: 1px solid #e2e8f0 !important;
}

[data-theme='light'] .summary-row {
  color: #475569 !important;
}

[data-theme='light'] .total-row {
  border-top: 1px solid #e2e8f0 !important;
  color: #0f172a !important;
}

[data-theme='light'] .modal-footer {
  background-color: #f8fafc !important;
  border-top: 1px solid #e2e8f0 !important;
}

[data-theme='light'] .btn-secondary {
  background-color: #ffffff !important;
  border-color: #cbd5e1 !important;
  color: #334155 !important;
}

[data-theme='light'] .btn-secondary:hover {
  background-color: #f1f5f9 !important;
  color: #0f172a !important;
}

.btn-secondary {
  background-color: var(--bg-surface);
  color: var(--text-secondary);
  border: 1px solid var(--border-color);
  padding: 0.5rem 1rem;
  border-radius: 6px;
  font-weight: 500;
  cursor: pointer;
}

.btn-secondary:hover {
  background-color: var(--bg-hover);
  color: var(--text-primary);
}

.btn-primary {
  background-color: var(--accent-color);
  color: #ffffff;
  border: none;
  padding: 0.5rem 1rem;
  border-radius: 6px;
  font-weight: 500;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}

.btn-primary:hover {
  background-color: var(--accent-hover);
}
</style>
