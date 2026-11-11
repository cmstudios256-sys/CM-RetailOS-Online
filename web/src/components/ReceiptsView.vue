<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
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

type ReceiptSettings = {
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
const settings = ref<ReceiptSettings | null>(null)
const selectedReceipt = ref<Sale | null>(null)
const receiptToShare = ref<Sale | null>(null)
const mobileTouch = ref(false)
const isReceiptModalOpen = ref(false)
const searchTerm = ref('')
const selectedDate = ref('')
const loading = ref(true)
const showSkeleton = useDelayedSkeleton(loading, 200)
const error = ref('')

const filteredReceipts = computed(() =>
  sales.value.filter(sale => {
    const query = searchTerm.value.trim().toLowerCase()
    const receiptId = receiptNumber(sale).toLowerCase()
    const matchesSearch = !query
      || receiptId.includes(query)
      || sale.id.toLowerCase().includes(query)
      || shortSaleId(sale.id).toLowerCase().includes(query)
      || sale.customerName.toLowerCase().includes(query)
      || sale.cashierName.toLowerCase().includes(query)
      || sale.items.some(item => item.productName.toLowerCase().includes(query))
    const matchesDate = !selectedDate.value || formatISODate(sale.completedAt) === selectedDate.value
    return matchesSearch && matchesDate
  }),
)

const { isPhone } = usePhoneBreakpoint()
const { isTablet } = useTabletBreakpoint()
const { isMobileOrTablet } = useMobileOrTabletBreakpoint()
const headerRef = ref<HTMLElement | null>(null)
const receiptsListRef = ref<HTMLElement | null>(null)
const receiptsListVirtualizer = useVirtualizer(computed(() => ({
  count: filteredReceipts.value.length,
  getScrollElement: () => receiptsListRef.value,
  estimateSize: () => 44,
  overscan: 8,
})))
const virtualReceiptsListItems = computed(() => receiptsListVirtualizer.value.getVirtualItems())

const mobileReceiptsListRef = ref<HTMLElement | null>(null)
const mobileReceiptsVirtualizer = useVirtualizer(computed(() => ({
  count: filteredReceipts.value.length,
  getScrollElement: () => mobileReceiptsListRef.value,
  estimateSize: () => 240,
  overscan: 4,
})))
const virtualMobileReceipts = computed(() => mobileReceiptsVirtualizer.value.getVirtualItems())

function syncHeaderScroll(e: Event) {
  const target = e.target as HTMLElement
  if (headerRef.value) {
    headerRef.value.scrollLeft = target.scrollLeft
  }
}

watch(
  () => [searchTerm.value, selectedDate.value, filteredReceipts.value.length],
  () => {
    receiptsListRef.value?.scrollTo({ top: 0 })
    mobileReceiptsListRef.value?.scrollTo({ top: 0 })
  },
)

function calculateTotalCost(sale: Sale) {
  return sale.items.reduce((sum, item) => sum + (item.unitCost || 0) * item.quantity, 0)
}

function calculateProfit(sale: Sale) {
  return sale.total - calculateTotalCost(sale)
}

function viewReceipt(sale: Sale) {
  selectedReceipt.value = sale
  isReceiptModalOpen.value = true
}

function closeModal() {
  isReceiptModalOpen.value = false
  selectedReceipt.value = null
}

function shareReceiptOnMobile(sale: Sale) {
  if (mobileTouch.value || isMobileOrTablet.value) receiptToShare.value = sale
}

function exportReceipts() {
  if (!filteredReceipts.value.length) return
  downloadCsv(
    `sales_receipts_${new Date().toISOString().slice(0, 10)}.csv`,
    ['Receipt ID', 'Sale ID', 'Date', 'Cashier', 'Customer', 'Subtotal', 'Discount', 'Total', 'Profit'],
    filteredReceipts.value.map(sale => [
      receiptNumber(sale),
      `#${shortSaleId(sale.id)}`,
      `${formatDate(sale.completedAt)} ${formatTime(sale.completedAt)}`,
      sale.cashierName || 'Admin',
      sale.customerName || 'Walk-in customer',
      sale.subtotal,
      sale.discount,
      sale.total,
      calculateProfit(sale),
    ]),
  )
}

function printReceiptsReport() {
  if (!filteredReceipts.value.length) return
  const totalRevenue = filteredReceipts.value.reduce((sum, sale) => sum + sale.total, 0)
  const totalProfit = filteredReceipts.value.reduce((sum, sale) => sum + calculateProfit(sale), 0)
  const rows = filteredReceipts.value.map(sale => `
    <tr>
      <td>${escapeHtml(receiptNumber(sale))}</td>
      <td>#${escapeHtml(shortSaleId(sale.id))}</td>
      <td>${escapeHtml(formatDate(sale.completedAt))} ${escapeHtml(formatTime(sale.completedAt))}</td>
      <td class="num">${escapeHtml(formatMoney(sale.total, props.currency))}</td>
      <td class="num">${escapeHtml(formatMoney(calculateProfit(sale), props.currency))}</td>
    </tr>
  `).join('')

  openPrintDocument(
    `${props.shopName} Sales Receipts Report`,
    `<div class="meta">
      <div>
        <h1>Sales Receipts Report</h1>
        <p>${escapeHtml(props.shopName)}${selectedDate.value ? ` · Date: ${escapeHtml(selectedDate.value)}` : ' · Full Period'}</p>
      </div>
      <div>
        <p>${escapeHtml(formatDate(new Date()))} ${escapeHtml(formatTime(new Date()))}</p>
        <p>${filteredReceipts.value.length} receipts · Revenue ${escapeHtml(formatMoney(totalRevenue, props.currency))} · Profit ${escapeHtml(formatMoney(totalProfit, props.currency))}</p>
      </div>
    </div>
    <table>
      <thead><tr><th>Receipt ID</th><th>Sale ID</th><th>Date</th><th class="num">Total</th><th class="num">Profit</th></tr></thead>
      <tbody>${rows}</tbody>
    </table>`,
  )
}

const prefsTick = ref(0)
const onlinePrefs = computed(() => {
  void prefsTick.value
  return readOnlinePrefs()
})

function printSingleReceipt(sale: Sale) {
  const storeTitle = settings.value?.shopName || props.shopName

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
      receiptToShare.value = sale
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
        ${settings.value?.address ? `<div>${escapeHtml(settings.value.address)}</div>` : ''}
        ${settings.value?.receiptShowPhone && settings.value?.phone ? `<div>Tel: ${escapeHtml(settings.value.phone)}</div>` : ''}
        ${settings.value?.receiptHeader ? `<div style="margin-top:4px;font-style:italic;">${escapeHtml(settings.value.receiptHeader)}</div>` : ''}
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
        <div>${escapeHtml(settings.value?.receiptFooter || 'Thank you for shopping with us!')}</div>
        <div style="font-size:10px;color:#555;margin-top:6px;">Powered by CM-RetailOS</div>
      </div>
    </div>`,
  )
}

async function load() {
  loading.value = true
  error.value = ''
  try {
    const [salesResult, settingsResult] = await Promise.all([
      apiRequest<{ sales: Sale[] }>('/sales'),
      apiRequest<{ settings: ReceiptSettings }>('/settings').catch(() => null),
    ])
    sales.value = salesResult.sales
    if (settingsResult?.settings) {
      settings.value = settingsResult.settings
    }
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Receipts could not be loaded.'
  } finally {
    loading.value = false
  }
}

function handlePrefsUpdated() {
  prefsTick.value++
}

onMounted(() => {
  mobileTouch.value = isTouchDevice()
  void load()
  window.addEventListener('online-prefs-updated', handlePrefsUpdated)
})

onUnmounted(() => {
  window.removeEventListener('online-prefs-updated', handlePrefsUpdated)
})
</script>

<template>
  <section class="view-section receipts-view">
    <div v-if="error" class="inline-error" role="alert">{{ error }}</div>

    <div class="card">
      <div class="card-header">
        <h2 class="card-title">Sales Receipts</h2>
        <div class="header-controls">
          <div class="search-bar">
            <Search class="search-icon" :size="18" />
            <input
              v-model="searchTerm"
              type="text"
              placeholder="Search by ID..."
              aria-label="Search receipts by ID"
            />
          </div>
          <AppDatePicker
            v-model="selectedDate"
            placeholder="Filter by Date"
            title="Filter Receipts by Date"
            aria-label="Filter receipts by date"
          />
          <button
            type="button"
            class="export-btn print-btn"
            :disabled="!filteredReceipts.length"
            @click="printReceiptsReport"
          >
            <Printer :size="16" />
            Print Report
          </button>
          <button
            type="button"
            class="export-btn"
            :disabled="!filteredReceipts.length"
            @click="exportReceipts"
          >
            <Download :size="16" />
            Export Receipts
          </button>
        </div>
      </div>

      <!-- Mobile / Tablet Card Layout (<1024px) -->
      <div v-if="isMobileOrTablet" ref="mobileReceiptsListRef" class="mobile-cards-scroll-body">
        <div v-if="showSkeleton" class="mobile-skeleton-cards">
          <div v-for="i in 4" :key="i" class="mobile-skeleton-card">
            <div class="mobile-skel-header">
              <div class="skeleton-box skeleton-shimmer" style="height: 18px; width: 45%;"></div>
              <div class="skeleton-box skeleton-shimmer" style="height: 14px; width: 35%;"></div>
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
        <div v-else-if="filteredReceipts.length === 0" class="empty-message">No receipts found.</div>
        <div
          v-else
          class="virtual-cards-spacer"
          :style="{ height: `${mobileReceiptsVirtualizer.getTotalSize()}px` }"
        >
          <div
            v-for="virtualRow in virtualMobileReceipts"
            :key="String(virtualRow.key)"
            :ref="el => mobileReceiptsVirtualizer.measureElement(el as HTMLElement)"
            :data-index="virtualRow.index"
            class="virtual-cards-row"
            :style="{
              transform: `translateY(${virtualRow.start}px)`,
            }"
          >
            <article
              v-if="filteredReceipts[virtualRow.index]"
              class="mobile-ui-card"
              tabindex="0"
              role="button"
              :aria-label="`Receipt ${receiptNumber(filteredReceipts[virtualRow.index])}`"
              @click="viewReceipt(filteredReceipts[virtualRow.index])"
              @keydown.enter="viewReceipt(filteredReceipts[virtualRow.index])"
            >
              <div class="mobile-card-header mobile-card-header-stacked">
                <span class="mobile-card-id" :title="receiptNumber(filteredReceipts[virtualRow.index])">
                  {{ receiptNumber(filteredReceipts[virtualRow.index]) }}
                </span>
                <span class="mobile-card-date">
                  {{ formatDate(filteredReceipts[virtualRow.index].completedAt) }} · {{ formatTime(filteredReceipts[virtualRow.index].completedAt) }}
                </span>
              </div>
              <div class="mobile-card-badges">
                <span class="mobile-card-pill">#{{ shortSaleId(filteredReceipts[virtualRow.index].id) }}</span>
                <span class="status-badge status-completed">Completed</span>
                <span class="mobile-card-pill mobile-card-payment">{{ filteredReceipts[virtualRow.index].paymentMethod }}</span>
              </div>
              <dl class="mobile-card-grid">
                <div>
                  <dt>Total</dt>
                  <dd class="mobile-card-total-val">{{ formatMoney(filteredReceipts[virtualRow.index].total, currency) }}</dd>
                </div>
                <div>
                  <dt>Profit</dt>
                  <dd :class="calculateProfit(filteredReceipts[virtualRow.index]) >= 0 ? 'profit-positive' : 'profit-negative'">
                    {{ formatMoney(calculateProfit(filteredReceipts[virtualRow.index]), currency) }}
                  </dd>
                </div>
                <div>
                  <dt>Payment</dt>
                  <dd>{{ filteredReceipts[virtualRow.index].paymentMethod }}</dd>
                </div>
                <div>
                  <dt>Cashier</dt>
                  <dd>{{ filteredReceipts[virtualRow.index].cashierName || 'Admin' }}</dd>
                </div>
              </dl>
              <div class="mobile-card-actions">
                <button
                  type="button"
                  class="mobile-card-btn primary-action"
                  @click.stop="viewReceipt(filteredReceipts[virtualRow.index])"
                >
                  <Eye :size="15" />
                  View
                </button>
                <button
                  type="button"
                  class="mobile-card-btn share-action"
                  aria-label="Share receipt to WhatsApp"
                  @click.stop="shareReceiptOnMobile(filteredReceipts[virtualRow.index])"
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
          <div class="virtual-list-header receipts-list-header">
            <span>Receipt ID</span>
            <span>Sale ID</span>
            <span>Date</span>
            <span>Total</span>
            <span>Profit</span>
            <span>Actions</span>
          </div>
        </div>
        <div v-if="showSkeleton" class="receipts-skeleton-list">
          <div v-for="i in 8" :key="i" class="receipts-skeleton-row">
            <div class="skeleton-box skeleton-shimmer" style="width: 100px; height: 18px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 70px; height: 18px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 110px; height: 18px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 85px; height: 18px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 80px; height: 18px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 110px; height: 26px; border-radius: 6px;"></div>
          </div>
        </div>
        <div v-else-if="loading && !showSkeleton" class="empty-message"></div>
        <div v-else-if="filteredReceipts.length === 0" class="empty-message">No receipts found.</div>
        <div
          v-else
          ref="receiptsListRef"
          class="table-container virtual-list-scroll-body"
          @scroll="syncHeaderScroll"
        >
          <div
            class="virtual-list-spacer"
            :style="{ height: `${receiptsListVirtualizer.getTotalSize()}px` }"
          >
            <div
              v-for="virtualRow in virtualReceiptsListItems"
              :key="String(virtualRow.key)"
              class="virtual-list-row receipts-list-header"
              :style="{ transform: `translateY(${virtualRow.start}px)`, height: `${virtualRow.size}px` }"
            >
              <template v-if="filteredReceipts[virtualRow.index]">
                <span class="font-medium text-slate-200" :title="receiptNumber(filteredReceipts[virtualRow.index])">{{ receiptNumber(filteredReceipts[virtualRow.index]) }}</span>
                <span>#{{ shortSaleId(filteredReceipts[virtualRow.index].id) }}</span>
                <span>{{ formatDate(filteredReceipts[virtualRow.index].completedAt) }} {{ formatTime(filteredReceipts[virtualRow.index].completedAt) }}</span>
                <span class="font-semibold">{{ formatMoney(filteredReceipts[virtualRow.index].total, currency) }}</span>
                <span
                  class="font-semibold"
                  :class="calculateProfit(filteredReceipts[virtualRow.index]) >= 0 ? 'profit-positive' : 'profit-negative'"
                >
                  {{ formatMoney(calculateProfit(filteredReceipts[virtualRow.index]), currency) }}
                </span>
                <span class="actions-cell">
                  <button type="button" class="action-btn" title="View Receipt" @click="viewReceipt(filteredReceipts[virtualRow.index])">
                    <Eye :size="15" />
                  </button>
                  <button
                    v-if="mobileTouch"
                    type="button"
                    class="action-btn"
                    title="Share receipt to WhatsApp"
                    aria-label="Share receipt to WhatsApp"
                    @click.stop="shareReceiptOnMobile(filteredReceipts[virtualRow.index])"
                  >
                    <Share2 :size="15" />
                  </button>
                  <button type="button" class="action-btn" title="Print Receipt" @click="printSingleReceipt(filteredReceipts[virtualRow.index])">
                    <Printer :size="15" />
                  </button>
                </span>
              </template>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Receipt Preview Modal -->
    <div v-if="isReceiptModalOpen && selectedReceipt" class="modal-backdrop receipt-preview-backdrop" @click.self="closeModal">
      <div class="receipt-modal">
        <div class="modal-top-bar">
          <h3>Receipt Preview</h3>
          <button type="button" class="close-btn" aria-label="Close receipt preview" @click="closeModal">
            <X :size="20" />
          </button>
        </div>

        <div class="receipt-paper-wrapper">
          <div class="receipt-paper" :style="{ maxWidth: onlinePrefs.receiptsSettings?.paperWidth === '58mm' ? '250px' : '320px' }">
            <div class="receipt-header">
              <div
                v-if="onlinePrefs.receiptsSettings?.showLogo !== false && onlinePrefs.logo"
                style="display: flex; justify-content: center; margin-bottom: 8px;"
              >
                <img :src="onlinePrefs.logo" alt="" style="width: 48px; height: 48px; object-fit: contain; filter: brightness(0);" />
              </div>
              <h2 class="store-name">{{ settings?.shopName || shopName }}</h2>
              <p v-if="settings?.address" class="store-info">{{ settings.address }}</p>
              <p v-if="settings?.receiptShowPhone && settings?.phone" class="store-info">Tel: {{ settings.phone }}</p>
              <p v-if="settings?.receiptHeader" class="custom-header">{{ settings.receiptHeader }}</p>
              <div class="dashed-divider">--------------------------------</div>
              <div class="receipt-meta">
                <p><span>Receipt:</span> <span>#{{ receiptNumber(selectedReceipt) }}</span></p>
                <p><span>Sale ID:</span> <span>#{{ shortSaleId(selectedReceipt.id) }}</span></p>
                <p><span>Date:</span> <span>{{ formatDate(selectedReceipt.completedAt) }} {{ formatTime(selectedReceipt.completedAt) }}</span></p>
                <p v-if="onlinePrefs.receiptsSettings?.showCashier !== false"><span>Cashier:</span> <span>{{ selectedReceipt.cashierName || 'Admin' }}</span></p>
                <p v-if="selectedReceipt.customerName"><span>Customer:</span> <span>{{ selectedReceipt.customerName }}</span></p>
              </div>
              <div class="dashed-divider">--------------------------------</div>
            </div>

            <div class="receipt-items">
              <div class="items-header">
                <span>Item</span>
                <span>Amount</span>
              </div>
              <div class="dashed-divider">--------------------------------</div>
              <div v-for="(item, idx) in selectedReceipt.items" :key="`${item.productId}-${idx}`" class="item-block">
                <div class="item-name">{{ item.productName }}</div>
                <div class="item-details">
                  <span>{{ formatQuantity(item.quantity) }} x {{ formatMoney(item.unitPrice, currency) }}</span>
                  <span>{{ formatMoney(item.lineTotal, currency) }}</span>
                </div>
              </div>
            </div>

            <div class="dashed-divider">--------------------------------</div>

            <div class="receipt-totals">
              <div v-if="selectedReceipt.discount > 0" class="total-row">
                <span>Subtotal</span>
                <span>{{ formatMoney(selectedReceipt.subtotal, currency) }}</span>
              </div>
              <div v-if="selectedReceipt.discount > 0" class="total-row">
                <span>Discount</span>
                <span>-{{ formatMoney(selectedReceipt.discount, currency) }}</span>
              </div>
              <div class="total-row grand-total">
                <span>TOTAL</span>
                <span>{{ formatMoney(selectedReceipt.total, currency) }}</span>
              </div>
            </div>

            <div class="dashed-divider">--------------------------------</div>

            <div class="receipt-footer">
              <p>{{ settings?.receiptFooter || 'Thank you for shopping with us!' }}</p>
              <p class="software-tag">Powered by CM-RetailOS</p>
            </div>

            <div class="paper-jagged-bottom"></div>
          </div>
        </div>

        <div class="receipt-modal-actions">
          <button type="button" class="btn-secondary receipt-close-btn" @click="closeModal">Close</button>
          <button
            v-if="isMobileOrTablet || mobileTouch"
            type="button"
            class="btn-secondary mobile-receipt-share-button"
            @click="shareReceiptOnMobile(selectedReceipt)"
          >
            <Share2 :size="16" />
            Share to WhatsApp
          </button>
          <button type="button" class="btn-primary receipt-print-btn" @click="printSingleReceipt(selectedReceipt)">
            <Printer :size="16" />
            Print Receipt
          </button>
        </div>
      </div>
    </div>
    <Teleport to="body">
      <MobileReceiptShare
        :sale="receiptToShare"
        :shop-name="settings?.shopName || shopName"
        :currency="currency"
        @close="receiptToShare = null"
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

.mobile-receipt-share-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
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
  border-radius: 0.375rem;
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

.receipts-list-header {
  grid-template-columns: minmax(200px, 1.8fr) minmax(110px, 1fr) minmax(180px, 1.5fr) minmax(110px, 1fr) minmax(110px, 1fr) 80px;
  min-width: 790px;
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
  background: rgba(14, 165, 233, 0.09);
  border: 1px solid rgba(14, 165, 233, 0.22);
  color: #38bdf8;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  padding: 0;
  border-radius: 6px;
  transition: all 0.15s ease;
}

.action-btn:hover {
  color: #ffffff;
  background-color: var(--accent-color, #0ea5e9);
  border-color: var(--accent-color, #0ea5e9);
}

table {
  width: 100%;
  min-width: 680px;
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

.profit-positive { color: #4ade80; }
.profit-negative { color: #f87171; }

.actions {
  display: flex;
  gap: 0.5rem;
  justify-content: center;
}

.empty-message {
  text-align: center;
  padding: 1.5rem;
  color: var(--text-muted);
}

/* Receipt Modal Styles */
.receipt-modal {
  background-color: var(--bg-sidebar);
  border: none !important;
  outline: none !important;
  border-radius: 16px;
  width: 100%;
  max-width: 420px;
  display: flex;
  flex-direction: column;
  max-height: 90vh;
  box-shadow: 0 25px 60px -12px rgba(0, 0, 0, 0.6);
  overflow: hidden;
}

.modal-top-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 1.1rem 1.5rem;
  border-bottom: none !important;
  background-color: var(--bg-surface);
}

.modal-top-bar h3 {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 700;
  color: var(--text-primary);
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

.receipt-paper-wrapper {
  padding: 1.5rem;
  overflow-y: auto;
  background-color: rgba(2, 6, 23, 0.35);
  display: flex;
  justify-content: center;
}

.receipt-paper {
  background-color: #ffffff;
  color: #0f172a;
  width: 100%;
  max-width: 320px;
  padding: 1.5rem 1.25rem 2rem;
  font-family: 'Courier New', Courier, monospace;
  font-size: 0.85rem;
  line-height: 1.45;
  box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.35);
  position: relative;
  border-radius: 8px 8px 0 0;
  border: none !important;
  outline: none !important;
}

.paper-jagged-bottom {
  position: absolute;
  bottom: -8px;
  left: 0;
  right: 0;
  height: 8px;
  background: linear-gradient(45deg, transparent 33.333%, #ffffff 33.333%, #ffffff 66.667%, transparent 66.667%),
              linear-gradient(-45deg, transparent 33.333%, #ffffff 33.333%, #ffffff 66.667%, transparent 66.667%);
  background-size: 16px 16px;
  background-position: 0 -8px;
}

.receipt-header {
  text-align: center;
  margin-bottom: 0.5rem;
}

.store-name {
  margin: 0 0 0.25rem;
  font-size: 1.25rem;
  font-weight: 700;
  color: #0f172a;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.store-info {
  margin: 0;
  font-size: 0.75rem;
  color: #475569;
}

.custom-header {
  margin: 0.35rem 0 0;
  font-size: 0.78rem;
  color: #334155;
  font-style: italic;
  white-space: pre-wrap;
}

.dashed-divider {
  text-align: center;
  color: #94a3b8;
  margin: 0.5rem 0;
  overflow: hidden;
  white-space: nowrap;
  user-select: none;
}

.receipt-meta {
  text-align: left;
  font-size: 0.8rem;
}

.receipt-meta p {
  margin: 0.15rem 0;
  display: flex;
  justify-content: space-between;
  gap: 0.5rem;
}

.items-header {
  display: flex;
  justify-content: space-between;
  font-weight: 700;
  font-size: 0.8rem;
  text-transform: uppercase;
}

.item-block {
  margin-bottom: 0.5rem;
  font-size: 0.8rem;
}

.item-name {
  font-weight: 600;
  color: #0f172a;
}

.item-details {
  display: flex;
  justify-content: space-between;
  color: #334155;
  padding-left: 0.5rem;
}

.receipt-totals {
  margin: 0.5rem 0;
}

.total-row {
  display: flex;
  justify-content: space-between;
  font-size: 0.85rem;
  margin: 0.25rem 0;
}

.grand-total {
  font-weight: 700;
  font-size: 1rem;
  color: #0f172a;
}

.receipt-footer {
  text-align: center;
  margin-top: 1rem;
  font-size: 0.75rem;
  color: #475569;
  white-space: pre-wrap;
}

.software-tag {
  margin-top: 0.5rem;
  font-size: 0.65rem;
  color: #94a3b8;
}

.receipt-modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  padding: 1rem 1.5rem;
  border-top: none !important;
  background-color: var(--bg-surface);
}

.btn-secondary {
  background-color: rgba(255, 255, 255, 0.08);
  color: #f1f5f9;
  border: none !important;
  outline: none !important;
  padding: 0.6rem 1.1rem;
  border-radius: 8px;
  font-weight: 600;
  cursor: pointer;
  transition: background-color 0.15s ease;
}

.btn-secondary:hover {
  background-color: rgba(255, 255, 255, 0.14);
}

.btn-primary {
  background-color: var(--accent-color);
  color: #ffffff;
  border: none !important;
  outline: none !important;
  padding: 0.6rem 1.2rem;
  border-radius: 8px;
  font-weight: 600;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
}

.btn-primary:hover {
  background-color: var(--accent-hover);
}

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

  [data-theme='light'] .receipt-preview-backdrop {
    background: transparent !important;
    backdrop-filter: none !important;
    -webkit-backdrop-filter: none !important;
    color: #0f172a !important;
  }

  [data-theme='light'] .receipt-modal {
    background: #ffffff !important;
    color: #0f172a !important;
  }

  [data-theme='light'] .receipt-header-mobile {
    background: #ffffff !important;
    color: #0f172a !important;
  }

  /* Mobile Receipt Preview Standout Fullscreen View */
  .receipt-preview-backdrop {
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

  .receipt-modal {
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

  .modal-top-bar {
    padding: max(16px, env(safe-area-inset-top)) 16px 12px 16px !important;
    border-bottom: none !important;
    background: transparent !important;
    flex-shrink: 0;
  }

  .receipt-paper-wrapper {
    flex: 1 1 auto;
    background: transparent !important;
    padding: 10px 16px 20px 16px !important;
    display: flex;
    justify-content: center;
    align-items: flex-start;
    overflow-y: auto;
    -webkit-overflow-scrolling: touch;
  }

  .receipt-paper {
    width: 100% !important;
    max-width: 330px;
    margin: 0 auto;
    box-shadow: 0 16px 36px rgba(0, 0, 0, 0.5) !important;
    border-radius: 10px 10px 0 0 !important;
    border: none !important;
    outline: none !important;
  }

  .receipt-modal-actions {
    padding: 12px 14px max(16px, env(safe-area-inset-bottom)) 14px !important;
    border-top: none !important;
    background: #0b1320 !important;
    display: flex !important;
    gap: 10px !important;
    flex-shrink: 0;
  }

  .receipt-modal-actions button {
    flex: 1 1 0;
    min-height: 48px;
    border-radius: 10px;
    font-size: 0.95rem;
    font-weight: 650;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
  }

  .receipt-close-btn {
    background: rgba(255, 255, 255, 0.08) !important;
    color: #f1f5f9 !important;
    border: none !important;
    outline: none !important;
  }

  .receipt-close-btn:active {
    background: rgba(255, 255, 255, 0.14) !important;
  }

  .mobile-receipt-share-button {
    background: #10b981 !important;
    color: #ffffff !important;
    border: none !important;
    outline: none !important;
    box-shadow: 0 4px 14px rgba(16, 185, 129, 0.35) !important;
    font-weight: 700 !important;
  }

  .mobile-receipt-share-button:active {
    background: #059669 !important;
    transform: scale(0.98);
  }
}

[data-theme='light'] .receipt-modal {
  background-color: #ffffff !important;
  border: none !important;
  box-shadow: 0 20px 40px -10px rgba(15, 23, 42, 0.12) !important;
}

@media (max-width: 639px) {
  [data-theme='light'] .receipt-preview-backdrop,
  [data-theme='light'] .receipt-modal,
  [data-theme='light'] .receipt-modal-actions {
    background: #f1f5f9 !important;
  }
}

[data-theme='light'] .modal-top-bar h3 {
  color: #0f172a !important;
}

[data-theme='light'] .receipt-close-btn {
  background: rgba(15, 23, 42, 0.07) !important;
  color: #1e293b !important;
}
</style>
