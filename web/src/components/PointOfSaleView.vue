<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import {
  AlertCircle,
  Camera,
  Barcode,
  CheckCircle2,
  Delete,
  Image as ImageIcon,
  LayoutGrid,
  List,
  Lock,
  Minus,
  Plus,
  Printer,
  Search,
  Share2,
  ShoppingCart,
  Trash2,
  X,
} from '@lucide/vue'
import type { IScannerControls } from '@zxing/browser'
import { apiRequest, formatDate, formatMoney, formatQuantity, formatTime, readOnlinePrefs, type Product, type Sale } from '../api'
import MobileReceiptShare from './MobileReceiptShare.vue'
import { isTouchDevice } from '../utils/device'
import { usePhoneBreakpoint } from '../utils/breakpoints'
import { escapeHtml, openPrintDocument, receiptNumber, shortSaleId } from '../utils/reporting'

type CartItem = {
  product: Product
  quantity: number
}

type ShopReceiptSettings = {
  shopName: string
  currency: string
  address: string
  phone: string
  receiptHeader: string
  receiptFooter: string
  receiptShowPhone: boolean
}

const props = defineProps<{ currency: string; shopName: string; role: string }>()
const products = ref<Product[]>([])
const cart = ref<CartItem[]>([])
const mobileCartOpen = ref(false)
const mobileTouch = ref(false)
const cameraSupported = ref(false)
const cameraOpen = ref(false)
const cameraError = ref('')
const cameraVideo = ref<HTMLVideoElement | null>(null)
const shareSale = ref<Sale | null>(null)
const { isPhone } = usePhoneBreakpoint()
let cameraControls: IScannerControls | null = null
let cameraSession = 0
const search = ref('')
let savedViewMode = (typeof localStorage !== 'undefined' ? localStorage.getItem('cm_pos_view_mode') : null) as 'list' | 'grid' | null
const viewMode = ref<'list' | 'grid'>(
  window.matchMedia('(max-width: 639px)').matches ? 'grid' : savedViewMode === 'list' ? 'list' : 'grid',
)
function setViewMode(mode: 'list' | 'grid') {
  if (isPhone.value) return
  viewMode.value = mode
  savedViewMode = mode
  try {
    localStorage.setItem('cm_pos_view_mode', mode)
  } catch {}
}

watch(isPhone, phone => {
  if (phone) {
    viewMode.value = 'grid'
  } else {
    viewMode.value = savedViewMode === 'list' ? 'list' : 'grid'
  }
})

const productImagePreview = ref<{ src: string; alt: string; left: number; top: number } | null>(null)

function updateProductImagePreview(event: MouseEvent, product: Product) {
  if (!product.image) {
    hideProductImagePreview()
    return
  }
  const previewWidth = 180
  const previewHeight = 220
  const left = event.clientX + previewWidth + 24 <= window.innerWidth
    ? event.clientX + 16
    : Math.max(12, event.clientX - previewWidth - 16)
  const top = Math.min(
    Math.max(12, event.clientY - previewHeight / 2),
    Math.max(12, window.innerHeight - previewHeight - 12)
  )

  productImagePreview.value = {
    src: product.image,
    alt: product.name,
    left,
    top,
  }
}

function hideProductImagePreview() {
  productImagePreview.value = null
}

const customerName = ref('')
const paymentMethod = ref<'cash' | 'mobile_money' | 'card'>('cash')
const discountAmount = ref<number | ''>('')
const amountTendered = ref<number | ''>('')
const loading = ref(true)
const submitting = ref(false)
const error = ref('')
const completedSale = ref<Sale | null>(null)
const completedChange = ref(0)
const shopSettings = ref<ShopReceiptSettings | null>(null)

// Toast notifications
const showErrorToast = ref(false)
const errorToastMessage = ref('')
const showSuccessToast = ref(false)
const successToastMessage = ref('')
let toastTimer: ReturnType<typeof setTimeout> | null = null

// Hardware barcode scanner wedge state
let barcodeBuffer = ''
let lastKeyTime = 0

function triggerErrorToast(message: string) {
  errorToastMessage.value = message
  showErrorToast.value = true
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => {
    showErrorToast.value = false
  }, 3200)
}

function triggerSuccessToast(message: string) {
  successToastMessage.value = message
  showSuccessToast.value = true
  if (toastTimer) clearTimeout(toastTimer)
  toastTimer = setTimeout(() => {
    showSuccessToast.value = false
  }, 3500)
}

const canApplyDiscount = computed(() => props.role === 'owner' || props.role === 'manager')
const prefsTick = ref(0)
const onlinePrefs = computed(() => {
  void prefsTick.value
  return readOnlinePrefs()
})

// Supervisor PIN Pad state for discounts
const isDiscountUnlocked = ref(false)
const showPinPad = ref(false)
const pinValue = ref('')
const requiresPinUnlock = computed(() =>
  canApplyDiscount.value
  && onlinePrefs.value.applicationSettings?.requirePinForDiscounts === true
  && !isDiscountUnlocked.value,
)

function appendPin(digit: number) {
  if (pinValue.value.length < 4) {
    pinValue.value += String(digit)
  }
}

function clearPin() {
  pinValue.value = ''
}

function deletePinChar() {
  pinValue.value = pinValue.value.slice(0, -1)
}

function closePinPad() {
  showPinPad.value = false
  pinValue.value = ''
}

function verifyPin() {
  const configuredPin = onlinePrefs.value.applicationSettings?.discountPin?.trim() || '1234'
  if (pinValue.value === configuredPin) {
    isDiscountUnlocked.value = true
    showPinPad.value = false
    pinValue.value = ''
    triggerSuccessToast('Supervisor PIN verified. Discount unlocked.')
  } else {
    triggerErrorToast('Invalid Supervisor PIN.')
    pinValue.value = ''
  }
}

function handlePrefsUpdated() {
  prefsTick.value++
}

function playScannerBeep() {
  if (onlinePrefs.value.applicationSettings?.barcodeBeep === false) return
  try {
    const AudioCtx = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext
    if (!AudioCtx) return
    const ctx = new AudioCtx()
    const osc = ctx.createOscillator()
    const gain = ctx.createGain()
    osc.type = 'sine'
    osc.frequency.setValueAtTime(1046.5, ctx.currentTime)
    gain.gain.setValueAtTime(0.08, ctx.currentTime)
    osc.connect(gain)
    gain.connect(ctx.destination)
    osc.start()
    osc.stop(ctx.currentTime + 0.07)
  } catch {
    // ignore audio restrictions
  }
}

const filteredProducts = computed(() => {
  const hideZero = onlinePrefs.value.applicationSettings?.hideOutOfStock === true
  const baseList = hideZero ? products.value.filter(product => product.quantity > 0) : products.value
  const query = search.value.trim().toLowerCase()
  if (!query) return baseList
  return baseList.filter(product =>
    product.name.toLowerCase().includes(query)
    || product.sku.toLowerCase().includes(query)
    || product.barcode.toLowerCase().includes(query)
    || product.category.toLowerCase().includes(query),
  )
})

const subtotal = computed(() =>
  Math.round(cart.value.reduce((sum, item) => sum + item.product.sellingPrice * item.quantity, 0) * 100) / 100,
)

const discountValue = computed(() => {
  if (!canApplyDiscount.value) return 0
  const parsed = Number(discountAmount.value)
  if (!Number.isFinite(parsed) || parsed <= 0) return 0
  return Math.round(parsed * 100) / 100
})

const discountValid = computed(() => {
  if (!canApplyDiscount.value) return discountAmount.value === '' || Number(discountAmount.value) === 0
  if (discountAmount.value === '') return true
  const parsed = Number(discountAmount.value)
  return Number.isFinite(parsed) && parsed >= 0 && parsed <= subtotal.value
})

const payableTotal = computed(() => Math.max(0, Math.round((subtotal.value - discountValue.value) * 100) / 100))

const tenderedValue = computed(() => {
  const parsed = Number(amountTendered.value)
  if (!Number.isFinite(parsed) || parsed <= 0) return 0
  return Math.round(parsed * 100) / 100
})

const changeDue = computed(() => Math.round((tenderedValue.value - payableTotal.value) * 100) / 100)
const tenderValid = computed(() => paymentMethod.value !== 'cash' || amountTendered.value === '' || tenderedValue.value >= payableTotal.value)
const cartUnits = computed(() => cart.value.reduce((sum, item) => sum + item.quantity, 0))

async function loadProducts() {
  loading.value = true
  error.value = ''
  try {
    const [result, settingsResult] = await Promise.all([
      apiRequest<{ products: Product[] }>('/products'),
      apiRequest<{ settings: ShopReceiptSettings }>('/settings').catch(() => null),
    ])
    products.value = result.products
    if (settingsResult?.settings) {
      shopSettings.value = settingsResult.settings
    }
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Products could not be loaded.'
  } finally {
    loading.value = false
  }
}

function getStockStatusClass(product: Product) {
  if (product.quantity <= 0) return 'status-out-of-stock'
  if (product.quantity <= product.minStock) return 'status-low-stock'
  return 'status-in-stock'
}

function getStockStatusText(product: Product) {
  const qty = formatQuantity(product.quantity)
  if (product.quantity <= 0) return 'Out of Stock'
  if (product.quantity <= product.minStock) return `Low Stock (${qty})`
  return `In Stock (${qty})`
}

function addToCart(product: Product): boolean {
  completedSale.value = null
  error.value = ''
  if (product.quantity <= 0) {
    triggerErrorToast(`Cannot add "${product.name}": Out of Stock!`)
    return false
  }
  const existing = cart.value.find(item => item.product.id === product.id)
  if (existing) {
    if (existing.quantity + 1 > product.quantity) {
      triggerErrorToast(`Only ${formatQuantity(product.quantity)} ${product.unit} of "${product.name}" available in stock.`)
      return false
    }
    existing.quantity = Math.round((existing.quantity + 1) * 1000) / 1000
    return true
  }
  cart.value.push({ product, quantity: Math.min(1, product.quantity) })
  return true
}

function addScannedProduct(code: string) {
  const lowered = code.toLowerCase()
  const match = products.value.find(product =>
    (product.barcode && product.barcode.toLowerCase() === lowered)
    || product.sku.toLowerCase() === lowered,
  )
  if (!match) {
    triggerErrorToast(`Barcode not found: "${code}"`)
    return
  }
  playScannerBeep()
  if (addToCart(match)) triggerSuccessToast(`${match.name} added to cart.`)
}

async function openBarcodeCamera() {
  if (!cameraSupported.value || cameraOpen.value) return
  cameraError.value = ''
  cameraOpen.value = true
  const session = ++cameraSession

  try {
    await nextTick()
    if (!cameraVideo.value) throw new Error('The camera preview could not be opened.')
    const { BrowserMultiFormatReader } = await import('@zxing/browser')
    const reader = new BrowserMultiFormatReader()
    const controls = await reader.decodeFromConstraints(
      { audio: false, video: { facingMode: { ideal: 'environment' } } },
      cameraVideo.value,
      result => {
        if (!result || session !== cameraSession || !cameraOpen.value) return
        const code = result.getText().trim()
        closeBarcodeCamera()
        addScannedProduct(code)
      },
    )
    if (session !== cameraSession || !cameraOpen.value) controls.stop()
    else cameraControls = controls
  } catch (reason) {
    if (session !== cameraSession || !cameraOpen.value) return
    const stream = cameraVideo.value?.srcObject
    if (stream instanceof MediaStream) stream.getTracks().forEach(track => track.stop())
    if (cameraVideo.value) cameraVideo.value.srcObject = null
    cameraError.value = reason instanceof Error
      ? reason.name === 'NotAllowedError'
        ? 'Camera permission was denied. Allow camera access in your browser settings and try again.'
        : reason.message
      : 'Camera access failed. Check permission and try again.'
  }
}

function closeBarcodeCamera() {
  cameraSession++
  cameraOpen.value = false
  cameraControls?.stop()
  cameraControls = null
  const stream = cameraVideo.value?.srcObject
  if (stream instanceof MediaStream) stream.getTracks().forEach(track => track.stop())
  if (cameraVideo.value) cameraVideo.value.srcObject = null
}

function openReceiptShare(sale: Sale) {
  if (mobileTouch.value) shareSale.value = sale
}

function handleSearchInputEnter() {
  const query = search.value.trim()
  if (!query) return
  const lowered = query.toLowerCase()
  const exactMatch = products.value.find(product =>
    (product.barcode && product.barcode.toLowerCase() === lowered)
    || product.sku.toLowerCase() === lowered,
  )
  if (exactMatch) {
    playScannerBeep()
    addToCart(exactMatch)
    search.value = ''
    return
  }
  if (filteredProducts.value.length === 1) {
    playScannerBeep()
    addToCart(filteredProducts.value[0])
    search.value = ''
    return
  }
  triggerErrorToast(`Product not found for "${query}".`)
}

function handleGlobalKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && cameraOpen.value) {
    closeBarcodeCamera()
    return
  }
  if (event.key === 'Escape' && mobileCartOpen.value) {
    mobileCartOpen.value = false
    return
  }
  if (showPinPad.value) return
  const activeTag = document.activeElement?.tagName
  if (activeTag === 'INPUT' || activeTag === 'TEXTAREA' || activeTag === 'SELECT') return

  const now = Date.now()
  if (now - lastKeyTime > 50) {
    barcodeBuffer = ''
  }

  if (event.key === 'Enter') {
    if (barcodeBuffer.length >= 3) {
      event.preventDefault()
      const code = barcodeBuffer.trim().toLowerCase()
      barcodeBuffer = ''
      const match = products.value.find(product =>
        (product.barcode && product.barcode.toLowerCase() === code)
        || product.sku.toLowerCase() === code,
      )
      if (match) {
        playScannerBeep()
        addToCart(match)
      } else {
        triggerErrorToast(`Barcode not found: "${code}"`)
      }
    }
  } else if (event.key.length === 1) {
    barcodeBuffer += event.key
  }
  lastKeyTime = now
}

function changeQuantity(item: CartItem, nextQuantity: number) {
  if (!Number.isFinite(nextQuantity) || nextQuantity <= 0) {
    removeFromCart(item.product.id)
    return
  }
  if (nextQuantity > item.product.quantity) {
    item.quantity = item.product.quantity
    triggerErrorToast(`Max available stock (${formatQuantity(item.product.quantity)}) reached for "${item.product.name}".`)
    return
  }
  item.quantity = Math.round(nextQuantity * 1000) / 1000
}

function removeFromCart(productId: number) {
  cart.value = cart.value.filter(item => item.product.id !== productId)
}

function clearCart() {
  cart.value = []
  customerName.value = ''
  discountAmount.value = ''
  amountTendered.value = ''
  isDiscountUnlocked.value = false
  error.value = ''
}

function printCompletedReceipt(sale: Sale) {
  const settings = shopSettings.value
  const prefs = readOnlinePrefs()
  const showLogo = prefs.receiptsSettings?.showLogo !== false
  const showCashier = prefs.receiptsSettings?.showCashier !== false
  const receiptMaxWidth = prefs.receiptsSettings?.paperWidth === '58mm' ? '220px' : '300px'
  const logoHtml = showLogo && prefs.logo
    ? `<div style="display:flex;justify-content:center;margin-bottom:8px;"><img src="${escapeHtml(prefs.logo)}" style="width:48px;height:48px;object-fit:contain;filter:brightness(0);" /></div>`
    : ''
  const storeTitle = settings?.shopName || props.shopName
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

async function checkout() {
  if (!cart.value.length) return
  if (!discountValid.value) {
    error.value = canApplyDiscount.value
      ? 'Discount must be between 0 and the cart subtotal.'
      : 'Only shop owners and managers can apply a discount.'
    return
  }
  if (!tenderValid.value) {
    error.value = 'Cash received cannot be less than the total due.'
    return
  }

  submitting.value = true
  error.value = ''
  try {
    const result = await apiRequest<{ sale: Sale }>('/sales/checkout', {
      method: 'POST',
      body: {
        customerName: customerName.value,
        paymentMethod: paymentMethod.value,
        discount: discountValue.value,
        items: cart.value.map(item => ({
          productId: item.product.id,
          quantity: item.quantity,
        })),
      },
    })
    completedChange.value = paymentMethod.value === 'cash' && tenderedValue.value > 0
      ? Math.max(0, Math.round((tenderedValue.value - result.sale.total) * 100) / 100)
      : 0
    completedSale.value = result.sale
    triggerSuccessToast(`Sale #${shortSaleId(result.sale.id)} completed (${formatMoney(result.sale.total, props.currency)})`)
    const prefs = readOnlinePrefs()
    if (prefs.applicationSettings?.autoClearCart !== false) {
      clearCart()
    }
    if (prefs.receiptsSettings?.autoPrint) {
      printCompletedReceipt(result.sale)
    }
    await loadProducts()
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Checkout could not be completed.'
  } finally {
    submitting.value = false
  }
}

onMounted(() => {
  mobileTouch.value = isTouchDevice()
  cameraSupported.value = mobileTouch.value && window.isSecureContext && !!navigator.mediaDevices?.getUserMedia
  loadProducts()
  window.addEventListener('keydown', handleGlobalKeydown)
  window.addEventListener('online-prefs-updated', handlePrefsUpdated)
})

onUnmounted(() => {
  closeBarcodeCamera()
  window.removeEventListener('keydown', handleGlobalKeydown)
  window.removeEventListener('online-prefs-updated', handlePrefsUpdated)
  if (toastTimer) clearTimeout(toastTimer)
})
</script>

<template>
  <div class="pos-container">
    <!-- Toast Notifications -->
    <div v-if="showErrorToast" class="pos-toast pos-toast-error" role="alert">
      <AlertCircle :size="20" />
      <span>{{ errorToastMessage }}</span>
    </div>
    <div v-if="showSuccessToast" class="pos-toast pos-toast-success" role="status">
      <CheckCircle2 :size="20" />
      <span>{{ successToastMessage }}</span>
      <button v-if="completedSale" type="button" class="toast-print-btn" @click="printCompletedReceipt(completedSale)">
        <Printer :size="14" /> Print Receipt
      </button>
    </div>
    <button
      type="button"
      class="mobile-cart-toggle"
      aria-controls="pos-cart-panel"
      :aria-expanded="mobileCartOpen"
      @click="mobileCartOpen = true"
    >
      <ShoppingCart :size="18" />
      <span>Cart · {{ formatQuantity(cartUnits) }}</span>
      <strong>{{ formatMoney(payableTotal, currency) }}</strong>
    </button>

    <!-- Left Panel: Product Selection -->
    <div class="products-panel">
      <div class="panel-header">
        <h2 class="panel-title">Products</h2>
        <div class="header-controls">
          <div class="view-switcher">
            <button
              type="button"
              :class="{ active: viewMode === 'grid' }"
              title="Grid View"
              @click="setViewMode('grid')"
            >
              <LayoutGrid :size="18" />
            </button>
            <button
              type="button"
              :class="{ active: viewMode === 'list' }"
              title="List View"
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
              placeholder="Scan barcode or search products..."
              aria-label="Scan barcode or search products"
              @keydown.enter.prevent="handleSearchInputEnter"
            />
            <div class="scanner-indicator" title="Ready for barcode input">
              <Barcode :size="16" class="scanner-icon" />
              <span class="scanner-dot"></span>
            </div>
          </div>
          <button
            v-if="cameraSupported"
            type="button"
            class="mobile-scan-button"
            @click="openBarcodeCamera"
          >
            <Camera :size="17" />
            Scan
          </button>
        </div>
      </div>

      <div v-if="loading" class="empty-message">Loading products...</div>

      <!-- Product Grid View -->
      <div v-else-if="viewMode === 'grid'" class="product-grid">
        <div v-if="!filteredProducts.length" class="empty-grid-message">No products found.</div>
        <div
          v-for="product in filteredProducts"
          :key="product.id"
          class="product-card"
          :class="{ disabled: product.quantity <= 0, 'out-of-stock': product.quantity <= 0, 'no-image': !product.image }"
          @click="addToCart(product)"
        >
          <div class="product-name" :title="product.name">{{ product.name }}</div>
          <div class="product-category" :title="product.category || 'General'">
            {{ product.category || 'General' }}
          </div>

          <div class="product-card-body">
            <div class="card-meta">
              <div
                class="product-stock"
                :class="{
                  'in-stock': product.quantity > product.minStock,
                  low: product.quantity <= product.minStock && product.quantity > 0,
                  out: product.quantity <= 0
                }"
              >
                <span v-if="product.quantity > product.minStock">In Stock ({{ product.quantity }})</span>
                <span v-else-if="product.quantity > 0">Low Stock ({{ product.quantity }})</span>
                <span v-else>Out of Stock</span>
              </div>
              <div class="product-price">{{ formatMoney(product.sellingPrice, currency) }}</div>
            </div>

            <div
              class="product-image-small"
              @pointerenter="product.image && updateProductImagePreview($event, product)"
              @pointermove="product.image && updateProductImagePreview($event, product)"
              @pointerleave="hideProductImagePreview"
            >
              <img
                v-if="product.image"
                :src="product.image"
                :alt="product.name"
                class="real-product-img"
                loading="lazy"
              />
              <div v-else class="image-fallback">
                <ImageIcon :size="32" class="fallback-icon" :stroke-width="1.6" />
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Product List View -->
      <div v-else class="product-list-container">
        <table class="product-list-table">
          <thead>
            <tr>
              <th class="text-center">Name</th>
              <th class="text-center">Category</th>
              <th class="text-center">Price</th>
              <th class="text-center">Quantity</th>
              <th class="text-center">Action</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="!filteredProducts.length">
              <td colspan="5" class="empty-message">No products found.</td>
            </tr>
            <tr
              v-for="product in filteredProducts"
              :key="product.id"
              :class="{ 'out-of-stock': product.quantity <= 0 }"
              style="height: 38px; cursor: pointer;"
              @click="addToCart(product)"
            >
              <td class="text-center font-medium text-slate-200">{{ product.name }}</td>
              <td class="text-center">{{ product.category || 'Uncategorized' }}</td>
              <td class="text-center">{{ formatMoney(product.sellingPrice, currency) }}</td>
              <td class="text-center">
                <span class="stock-badge" :class="getStockStatusClass(product)">
                  {{ getStockStatusText(product) }}
                </span>
              </td>
              <td class="text-center">
                <button
                  type="button"
                  class="add-btn-list"
                  :disabled="product.quantity <= 0"
                  title="Add to cart"
                  @click.stop="addToCart(product)"
                >
                  <Plus :size="16" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Right Panel: Cart -->
    <div
      v-if="mobileCartOpen"
      class="mobile-cart-backdrop"
      @click="mobileCartOpen = false"
    ></div>
    <div
      id="pos-cart-panel"
      class="cart-panel"
      :class="{ 'mobile-cart-open': mobileCartOpen }"
    >
      <div class="panel-header">
        <h2 class="panel-title">Cart ({{ formatQuantity(cartUnits) }})</h2>
        <div class="cart-header-actions">
          <button
            v-if="cart.length > 0"
            type="button"
            class="clear-cart-btn"
            title="Clear Cart"
            @click="clearCart"
          >
            <Trash2 :size="18" />
          </button>
          <button
            type="button"
            class="mobile-cart-close"
            aria-label="Close cart"
            @click="mobileCartOpen = false"
          >
            <X :size="19" />
          </button>
        </div>
      </div>

      <div v-if="error" class="inline-error pos-inline-error" role="alert">{{ error }}</div>

      <div v-if="completedSale && !cart.length" class="completed-sale-banner" role="status">
        <div>
          <strong>Sale #{{ shortSaleId(completedSale.id) }} completed</strong>
          <span>
            Total {{ formatMoney(completedSale.total, currency) }}
            <template v-if="completedChange > 0"> · Change {{ formatMoney(completedChange, currency) }}</template>
          </span>
        </div>
        <button type="button" class="banner-print-btn" @click="printCompletedReceipt(completedSale)">
          <Printer :size="15" /> Print Receipt
        </button>
        <button
          v-if="mobileTouch"
          type="button"
          class="mobile-receipt-share"
          @click="openReceiptShare(completedSale)"
        >
          <Share2 :size="16" /> Share to WhatsApp
        </button>
      </div>

      <div v-if="cart.length === 0" class="empty-cart">
        <ShoppingCart :size="48" />
        <p>Your cart is empty</p>
        <span>Add products from the left panel to get started.</span>
      </div>

      <div v-else class="cart-items">
        <div v-for="item in cart" :key="item.product.id" class="cart-item">
          <div class="item-info">
            <p class="item-name">{{ item.product.name }}</p>
            <p class="item-price">{{ formatMoney(item.product.sellingPrice, currency) }}</p>
          </div>
          <div class="quantity-controls">
            <button type="button" @click="changeQuantity(item, item.quantity - 1)">
              <Minus :size="14" />
            </button>
            <input
              type="number"
              class="quantity-input"
              :value="item.quantity"
              min="0.001"
              :max="item.product.quantity"
              step="any"
              @change="changeQuantity(item, Number(($event.target as HTMLInputElement).value))"
            />
            <button
              type="button"
              :disabled="item.quantity >= item.product.quantity"
              @click="changeQuantity(item, item.quantity + 1)"
            >
              <Plus :size="14" />
            </button>
          </div>
          <p class="item-total">{{ formatMoney(item.product.sellingPrice * item.quantity, currency) }}</p>
          <button type="button" class="remove-item-btn" @click="removeFromCart(item.product.id)">
            <Trash2 :size="16" />
          </button>
        </div>
      </div>

      <div class="order-summary">
        <div v-if="cart.length > 0" class="pos-meta-row">
          <input
            v-model.trim="customerName"
            type="text"
            maxlength="120"
            class="pos-customer-input"
            placeholder="Walk-in customer (optional)"
          />
          <select v-model="paymentMethod" class="pos-payment-select" aria-label="Payment method">
            <option value="cash">Cash</option>
            <option value="mobile_money">Mobile Money</option>
            <option value="card">Card</option>
          </select>
        </div>

        <div class="summary-row">
          <span>Discount</span>
          <button
            v-if="requiresPinUnlock"
            type="button"
            class="locked-discount-btn"
            @click="showPinPad = true"
          >
            <span>Unlock</span>
            <Lock :size="13" />
          </button>
          <div v-else-if="canApplyDiscount" class="discount-input-wrapper">
            <span class="currency-prefix">{{ currency }}</span>
            <input
              v-model.number="discountAmount"
              type="number"
              placeholder="0"
              min="0"
              :max="subtotal"
              step="any"
              class="discount-input"
            />
          </div>
          <span v-else class="discount-locked-note" title="Discounts can only be applied by Owners and Managers online">
            <Lock :size="13" /> Owner / Manager only
          </span>
        </div>

        <div v-if="discountValue > 0" class="summary-row summary-subtotal-row">
          <span>Subtotal</span>
          <span>{{ formatMoney(subtotal, currency) }}</span>
        </div>

        <div class="summary-row total">
          <span>Total</span>
          <span>{{ formatMoney(payableTotal, currency) }}</span>
        </div>

        <!-- Change Calculator Section -->
        <div v-if="cart.length > 0 && paymentMethod === 'cash'" class="change-calculator">
          <div class="tendered-input-row">
            <label>Cash Received</label>
            <div class="discount-input-wrapper tendered-wrapper">
              <span class="currency-prefix">{{ currency }}</span>
              <input
                v-model.number="amountTendered"
                type="number"
                placeholder="Enter amount..."
                min="0"
                step="any"
                class="discount-input tendered-input"
              />
            </div>
          </div>

          <div
            v-if="Number(amountTendered) > 0"
            class="change-display-row"
            :class="changeDue >= 0 ? 'change-valid' : 'change-insufficient'"
          >
            <span>{{ changeDue < 0 ? 'Still Due' : 'Change to Return' }}</span>
            <span class="change-amount">{{ formatMoney(Math.abs(changeDue), currency) }}</span>
          </div>
        </div>

        <button
          type="button"
          class="checkout-btn"
          :disabled="cart.length === 0 || submitting || !discountValid || !tenderValid"
          @click="checkout"
        >
          {{ submitting ? 'Processing…' : 'Checkout' }}
        </button>
      </div>
    </div>

    <!-- Supervisor PIN Pad Modal -->
    <div v-if="showPinPad" class="pin-modal-overlay" @click.self="closePinPad">
      <div class="pin-modal-content">
        <div class="pin-header">
          <h3>Supervisor PIN Required</h3>
          <button type="button" class="pin-close-btn" @click="closePinPad">
            <X :size="18" />
          </button>
        </div>
        <div class="pin-display">
          <div class="pin-dots">
            <span v-for="i in 4" :key="i" class="pin-dot" :class="{ filled: i <= pinValue.length }"></span>
          </div>
        </div>
        <div class="pin-pad">
          <button v-for="n in [1, 2, 3, 4, 5, 6, 7, 8, 9]" :key="n" type="button" class="pin-btn" @click="appendPin(n)">
            {{ n }}
          </button>
          <button type="button" class="pin-btn pin-btn-clear" @click="clearPin">C</button>
          <button type="button" class="pin-btn" @click="appendPin(0)">0</button>
          <button type="button" class="pin-btn pin-btn-del" @click="deletePinChar">
            <Delete :size="18" />
          </button>
        </div>
        <div class="pin-footer">
          <button type="button" class="pin-verify-btn" :disabled="pinValue.length !== 4" @click="verifyPin">
            Verify
          </button>
        </div>
      </div>
    </div>
    <Teleport to="body">
      <div v-if="cameraOpen" class="barcode-camera-backdrop" @click.self="closeBarcodeCamera">
        <section
          class="barcode-camera-dialog"
          role="dialog"
          aria-modal="true"
          aria-labelledby="barcode-camera-title"
        >
          <header class="barcode-camera-heading">
            <div>
              <h2 id="barcode-camera-title">Scan product barcode</h2>
              <p>Hold the barcode inside the frame.</p>
            </div>
            <button type="button" class="barcode-camera-close" aria-label="Close barcode scanner" @click="closeBarcodeCamera">
              <X :size="19" />
            </button>
          </header>
          <video ref="cameraVideo" class="barcode-camera-preview" autoplay muted playsinline></video>
          <p v-if="cameraError" class="barcode-camera-error" role="alert">{{ cameraError }}</p>
          <p v-else class="barcode-camera-status" role="status">Scanning with your rear camera…</p>
          <button type="button" class="barcode-camera-cancel" @click="closeBarcodeCamera">Cancel scan</button>
        </section>
      </div>
      <MobileReceiptShare
        :sale="shareSale"
        :shop-name="shopSettings?.shopName || shopName"
        :currency="currency"
        @close="shareSale = null"
      />
    </Teleport>
    <Teleport to="body">
      <div
        v-if="productImagePreview"
        class="product-image-hover-preview"
        :style="{ left: `${productImagePreview.left}px`, top: `${productImagePreview.top}px` }"
        aria-hidden="true"
      >
        <img :src="productImagePreview.src" :alt="productImagePreview.alt" />
        <span>{{ productImagePreview.alt }}</span>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.pos-container {
  display: flex;
  gap: 0.75rem;
  height: 100%;
  min-height: 0;
  flex: 1;
  width: 100%;
  color: var(--text-secondary);
  position: relative;
}

.pos-toast {
  position: fixed;
  top: 24px;
  left: 50%;
  transform: translateX(-50%);
  color: #ffffff;
  padding: 0.75rem 1.25rem;
  border-radius: 9999px;
  display: flex;
  align-items: center;
  gap: 0.65rem;
  font-weight: 600;
  font-size: 0.9rem;
  z-index: 9999;
  box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.45);
}

.pos-toast-error {
  background-color: #ef4444;
  border: 1px solid #f87171;
}

.pos-toast-success {
  background-color: #16a34a;
  border: 1px solid #4ade80;
}

.toast-print-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  margin-left: 0.4rem;
  padding: 0.25rem 0.65rem;
  border-radius: 9999px;
  border: 1px solid rgba(255, 255, 255, 0.35);
  background: rgba(15, 23, 42, 0.35);
  color: #ffffff;
  font-size: 0.78rem;
  font-weight: 600;
  cursor: pointer;
}

.products-panel,
.cart-panel {
  background: var(--glass-bg);
  backdrop-filter: var(--glass-blur);
  border: 1px solid var(--glass-border);
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.products-panel {
  flex: 3;
  min-width: 0;
}

.cart-panel {
  flex: 2;
  min-width: 340px;
}

.panel-header {
  padding: 1rem 1.25rem;
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
  flex-shrink: 0;
}

.panel-title {
  margin: 0;
  font-size: 1.15rem;
  font-weight: 600;
  color: var(--text-primary);
}

.header-controls {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  min-width: 0;
}

.view-switcher {
  display: flex;
  background-color: var(--bg-surface);
  border-radius: 0.5rem;
  padding: 0.2rem;
  border: 1px solid var(--border-color);
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
}

.view-switcher button.active {
  background-color: var(--bg-sidebar);
  color: var(--text-primary);
}

.search-bar {
  position: relative;
  display: flex;
  align-items: center;
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
  border-radius: 0.5rem;
  padding: 0.45rem 3rem 0.45rem 2.25rem;
  color: var(--text-primary);
  width: 240px;
  font-size: 0.825rem;
}

.search-bar input:focus {
  outline: none;
  border-color: var(--accent-color);
}

.scanner-indicator {
  position: absolute;
  right: 0.75rem;
  top: 50%;
  transform: translateY(-50%);
  display: flex;
  align-items: center;
  gap: 4px;
  color: var(--text-muted);
  pointer-events: none;
}

.scanner-dot {
  width: 6px;
  height: 6px;
  background-color: #22c55e;
  border-radius: 50%;
  box-shadow: 0 0 6px #22c55e;
}

.mobile-scan-button,
.mobile-receipt-share {
  min-width: 44px;
  min-height: 44px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  padding: 0 12px;
  border: 1px solid rgba(14, 165, 233, .35);
  border-radius: 9px;
  color: #bae6fd;
  background: rgba(14, 165, 233, .12);
  font-size: 13px;
  font-weight: 650;
  cursor: pointer;
}

.mobile-receipt-share {
  border-color: rgba(34, 197, 94, .35);
  color: #dcfce7;
  background: rgba(22, 163, 74, .18);
}

.barcode-camera-backdrop {
  position: fixed;
  z-index: 110;
  inset: 0;
  display: grid;
  place-items: center;
  padding: max(12px, env(safe-area-inset-top)) max(12px, env(safe-area-inset-right)) max(12px, env(safe-area-inset-bottom)) max(12px, env(safe-area-inset-left));
  background: rgba(2, 6, 23, .86);
}

.barcode-camera-dialog {
  width: min(100%, 520px);
  padding: 18px;
  border: 1px solid rgba(148, 163, 184, .2);
  border-radius: 16px;
  background: #101a2a;
  color: #e5edf7;
  box-shadow: 0 24px 70px rgba(0, 0, 0, .45);
}

.barcode-camera-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 14px;
}

.barcode-camera-heading h2 {
  margin: 0 0 4px;
  font-size: 18px;
}

.barcode-camera-heading p,
.barcode-camera-status,
.barcode-camera-error {
  margin: 0;
  color: #aab8ca;
  font-size: 13px;
  line-height: 1.5;
}

.barcode-camera-close,
.barcode-camera-cancel {
  min-width: 44px;
  min-height: 44px;
  border: 1px solid rgba(148, 163, 184, .2);
  border-radius: 9px;
  color: #dbe5f0;
  background: rgba(148, 163, 184, .06);
  cursor: pointer;
}

.barcode-camera-preview {
  width: 100%;
  max-height: min(55dvh, 480px);
  aspect-ratio: 4 / 3;
  display: block;
  border-radius: 10px;
  background: #020617;
  object-fit: cover;
}

.barcode-camera-status,
.barcode-camera-error {
  margin: 12px 0;
}

.barcode-camera-error {
  color: #fca5a5;
}

.barcode-camera-cancel {
  width: 100%;
}

/* Grid View */
.product-grid {
  padding: 1rem;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(155px, 1fr));
  grid-auto-rows: max-content;
  gap: 0.75rem;
  overflow-y: auto;
  flex-grow: 1;
  align-content: start;
}

.empty-grid-message {
  grid-column: 1 / -1;
  text-align: center;
  padding: 2rem;
  color: var(--text-muted);
}

.product-card {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-subtle);
  border-radius: 12px;
  padding: 9px 10px;
  cursor: pointer;
  transition: transform 0.2s cubic-bezier(0.16, 1, 0.3, 1), box-shadow 0.2s, border-color 0.2s, background-color 0.2s;
  display: flex;
  flex-direction: column;
  position: relative;
  min-height: auto;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
  user-select: none;
}

[data-theme='light'] .product-card {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  box-shadow: 0 2px 6px rgba(15, 23, 42, 0.06);
}

.product-card:hover {
  transform: translateY(-3px) translateZ(0);
  border-color: var(--accent-color);
  box-shadow: 0 8px 24px -4px rgba(0, 0, 0, 0.35);
  background: linear-gradient(135deg, var(--bg-surface-hover) 0%, var(--glass-bg, var(--bg-surface)) 100%);
  z-index: 5;
}

[data-theme='light'] .product-card:hover {
  background: #ffffff;
  border-color: var(--accent-color);
  box-shadow: 0 8px 20px -4px rgba(14, 165, 233, 0.18), 0 2px 6px rgba(0, 0, 0, 0.06);
}

.product-card.disabled,
.product-card.out-of-stock {
  opacity: 0.5;
  cursor: not-allowed;
  filter: grayscale(100%);
}

.product-card.disabled:hover,
.product-card.out-of-stock:hover {
  transform: none;
  border-color: var(--border-subtle);
  box-shadow: none;
}

.product-card .product-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 3px;
  line-height: 1.3;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  text-overflow: ellipsis;
  word-break: break-word;
  letter-spacing: -0.01em;
}

[data-theme='light'] .product-card .product-name {
  color: #0f172a;
  font-weight: 700;
}

.product-card .product-category {
  font-size: 10.5px;
  font-weight: 600;
  color: #94a3b8;
  background: rgba(148, 163, 184, 0.14);
  padding: 2px 6px;
  border-radius: 4px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  line-height: 1.2;
  display: inline-block;
  align-self: flex-start;
  margin-bottom: 4px;
  width: fit-content;
  max-width: 100%;
}

[data-theme='light'] .product-card .product-category {
  color: #334155;
  background: #f1f5f9;
  border: 1px solid #cbd5e1;
}

.product-card-body {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  width: 100%;
  margin-top: 2px;
  position: relative;
}

.card-meta {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: center;
  gap: 4px;
  flex: 1;
  min-width: 0;
  position: relative;
  z-index: 2;
}

.product-stock {
  display: inline-flex;
  align-items: center;
  font-size: 11px;
  font-weight: 700;
  padding: 2px 6px;
  border-radius: 9999px;
  line-height: 1.2;
  white-space: nowrap;
}

.product-stock.in-stock {
  background: rgba(16, 185, 129, 0.2);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
}

.product-stock.low {
  background: rgba(245, 158, 11, 0.2);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.3);
}

.product-stock.out {
  background: rgba(239, 68, 68, 0.2);
  color: #f87171;
  border: 1px solid rgba(239, 68, 68, 0.3);
}

.product-price {
  margin: 0;
  font-weight: 800;
  color: #f97316;
  font-size: 15.5px;
  line-height: 1.2;
  white-space: nowrap;
  letter-spacing: -0.01em;
  margin-top: 1px;
  position: relative;
  z-index: 2;
  text-shadow: 0 0 5px rgba(15, 23, 42, 0.9), 0 0 10px rgba(15, 23, 42, 0.8);
}

[data-theme='light'] .product-price {
  color: #ea580c;
  font-weight: 850;
  text-shadow: 0 0 5px rgba(255, 255, 255, 0.95), 0 0 10px rgba(255, 255, 255, 0.9);
}

.product-image-small {
  width: 72px;
  height: 72px;
  flex-shrink: 0;
  border-radius: 9px;
  overflow: hidden;
  background: rgba(15, 23, 42, 0.6);
  border: 1px solid rgba(255, 255, 255, 0.1);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: transform 0.25s ease, border-color 0.2s ease, box-shadow 0.2s ease;
  position: relative;
  z-index: 1;
}

[data-theme='light'] .product-image-small {
  background: #f1f5f9;
  border: 1px solid #cbd5e1;
}

.product-card:hover .product-image-small {
  transform: scale(1.04);
  border-color: rgba(14, 165, 233, 0.35);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.25);
}

.real-product-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.25s ease;
}

.image-fallback {
  background: rgba(148, 163, 184, 0.09);
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.2s ease;
}

.product-card:hover .image-fallback {
  background: rgba(14, 165, 233, 0.08);
}

.fallback-icon {
  color: #94a3b8;
  opacity: 0.75;
  transition: color 0.2s ease, opacity 0.2s ease, transform 0.2s ease;
}

.product-card:hover .fallback-icon {
  color: var(--accent-color, #38bdf8);
  opacity: 1;
  transform: scale(1.06);
}

/* Floating Hover Preview */
.product-image-hover-preview {
  position: fixed;
  z-index: 10000;
  width: 180px;
  height: 220px;
  box-sizing: border-box;
  padding: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  border: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.18));
  border-radius: 14px;
  background: var(--bg-sidebar, #0f172a);
  box-shadow: 0 18px 48px rgba(0, 0, 0, 0.65);
  pointer-events: none;
  animation: previewFadeIn 0.15s ease-out;
}

@keyframes previewFadeIn {
  from {
    opacity: 0;
    transform: scale(0.95);
  }
  to {
    opacity: 1;
    transform: scale(1);
  }
}

.product-image-hover-preview img {
  width: 100%;
  height: 168px;
  min-height: 0;
  border-radius: 10px;
  object-fit: contain;
  background: rgba(15, 23, 42, 0.4);
}

.product-image-hover-preview span {
  overflow: hidden;
  color: var(--text-primary);
  font-size: 12px;
  font-weight: 600;
  line-height: 20px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* List View */
.product-list-container {
  overflow-y: auto;
  overflow-x: auto;
  flex-grow: 1;
  min-height: 0;
}

.product-list-table {
  width: 100%;
  border-collapse: separate;
  border-spacing: 0;
  background-color: var(--bg-sidebar);
  border-top: 1px solid var(--border-subtle);
  border-left: 1px solid var(--border-subtle);
}

.product-list-table th,
.product-list-table td {
  padding: 0.4rem 0.75rem;
  text-align: center;
  border-bottom: 1px solid var(--border-subtle);
  border-right: 1px solid var(--border-subtle);
  white-space: nowrap;
  vertical-align: middle;
  font-size: 0.825rem;
}

.product-list-table th {
  position: sticky;
  top: 0;
  background-color: var(--bg-surface);
  z-index: 10;
  color: var(--text-muted);
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.product-list-table tbody tr:hover {
  background-color: var(--bg-surface);
}

.product-list-table tr.out-of-stock {
  opacity: 0.5;
  cursor: not-allowed;
}

.stock-badge {
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

.stock-badge.status-in-stock {
  border-color: #22c55e;
  color: #4ade80;
  background-color: rgba(34, 197, 94, 0.1);
}

.stock-badge.status-low-stock {
  border-color: #f59e0b;
  color: #fbbf24;
  background-color: rgba(245, 158, 11, 0.1);
}

.stock-badge.status-out-of-stock {
  border-color: #ef4444;
  color: #f87171;
  background-color: rgba(239, 68, 68, 0.1);
}

.add-btn-list {
  background-color: var(--bg-hover);
  color: var(--text-primary);
  border: 1px solid var(--border-color);
  border-radius: 0.375rem;
  padding: 0.25rem;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.add-btn-list:hover:not(:disabled) {
  background-color: var(--accent-color);
  border-color: var(--accent-color);
  color: #ffffff;
}

.add-btn-list:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

/* Cart Panel */
.clear-cart-btn {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  padding: 0.25rem;
  border-radius: 0.25rem;
  display: flex;
}

.clear-cart-btn:hover {
  color: #f87171;
  background-color: rgba(239, 68, 68, 0.1);
}

.pos-inline-error {
  margin: 0.75rem 1rem 0;
}

.completed-sale-banner {
  margin: 0.75rem 1rem 0;
  padding: 0.65rem 0.85rem;
  border-radius: 0.5rem;
  background: rgba(34, 197, 94, 0.12);
  border: 1px solid rgba(34, 197, 94, 0.3);
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.8rem;
}

.completed-sale-banner strong {
  display: block;
  color: #4ade80;
}

.completed-sale-banner span {
  color: var(--text-secondary);
}

.banner-print-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.35rem 0.65rem;
  border-radius: 0.375rem;
  border: 1px solid rgba(56, 189, 248, 0.35);
  background: rgba(14, 165, 233, 0.15);
  color: #38bdf8;
  font-size: 0.75rem;
  font-weight: 600;
  cursor: pointer;
  white-space: nowrap;
}

.empty-cart {
  flex-grow: 1;
  display: flex;
  flex-direction: column;
  justify-content: center;
  align-items: center;
  color: var(--text-muted);
  text-align: center;
  padding: 1.5rem;
}

.empty-cart p {
  font-size: 1.05rem;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0.75rem 0 0.25rem;
}

.empty-cart span {
  font-size: 0.85rem;
}

.cart-items {
  flex-grow: 1;
  overflow-y: auto;
  padding: 0.75rem 1.25rem;
}

.cart-item {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.6rem 0;
  border-bottom: 1px solid var(--border-subtle);
}

.item-info {
  flex-grow: 1;
  min-width: 0;
}

.item-name {
  margin: 0;
  font-weight: 500;
  color: var(--text-primary);
  font-size: 0.85rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.item-price {
  margin: 0.1rem 0 0;
  font-size: 0.75rem;
  color: var(--text-muted);
}

.quantity-controls {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  background-color: var(--bg-surface);
  border-radius: 0.375rem;
  padding: 0.15rem;
}

.quantity-controls button {
  background: none;
  border: none;
  color: var(--text-secondary);
  cursor: pointer;
  padding: 0.2rem;
  display: flex;
}

.quantity-controls button:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}

.quantity-input {
  width: 2.5rem;
  text-align: center;
  font-weight: 600;
  font-size: 0.85rem;
  background: transparent;
  border: 1px solid transparent;
  color: var(--text-primary);
  border-radius: 0.25rem;
  padding: 0.1rem 0;
  -moz-appearance: textfield;
}

.quantity-input::-webkit-outer-spin-button,
.quantity-input::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}

.item-total {
  margin: 0;
  font-weight: 600;
  width: 78px;
  text-align: right;
  font-size: 0.85rem;
  color: var(--text-primary);
}

.remove-item-btn {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  display: flex;
}

.remove-item-btn:hover {
  color: #f87171;
}

.order-summary {
  padding: 1rem 1.25rem;
  border-top: 1px solid var(--border-subtle);
  background-color: rgba(15, 23, 42, 0.5);
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}

.pos-meta-row {
  display: grid;
  grid-template-columns: 1fr auto;
  gap: 0.5rem;
  margin-bottom: 0.15rem;
}

.pos-customer-input,
.pos-payment-select {
  background: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 0.375rem;
  padding: 0.35rem 0.6rem;
  color: var(--text-primary);
  font-size: 0.78rem;
}

.summary-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.875rem;
}

.summary-subtotal-row {
  color: var(--text-muted);
  font-size: 0.8rem;
}

.summary-row.total {
  font-size: 1.15rem;
  font-weight: 700;
  color: var(--text-primary);
  margin-top: 0.2rem;
  padding-top: 0.55rem;
  border-top: 1px solid var(--border-color);
}

.discount-input-wrapper {
  display: flex;
  align-items: center;
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 0.375rem;
  padding: 0.25rem 0.5rem;
  width: 130px;
}

.discount-input-wrapper:focus-within {
  border-color: var(--accent-color);
}

.currency-prefix {
  color: var(--text-muted);
  font-size: 0.78rem;
  margin-right: 0.25rem;
  user-select: none;
}

.discount-input {
  background: transparent;
  border: none;
  color: var(--text-primary);
  font-size: 0.875rem;
  width: 100%;
  text-align: right;
  outline: none;
  -moz-appearance: textfield;
}

.discount-input::-webkit-outer-spin-button,
.discount-input::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}

.discount-locked-note {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  font-size: 0.75rem;
  color: var(--text-muted);
  background: rgba(148, 163, 184, 0.08);
  padding: 0.2rem 0.5rem;
  border-radius: 0.35rem;
}

.change-calculator {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 0.5rem;
  padding: 0.65rem 0.75rem;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.tendered-input-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.85rem;
  color: var(--text-secondary);
  font-weight: 500;
}

.tendered-wrapper {
  width: 145px;
  background-color: var(--bg-sidebar);
}

.tendered-input {
  font-weight: 600;
  color: #38bdf8;
}

.change-display-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding-top: 0.45rem;
  border-top: 1px dashed var(--border-color);
  font-size: 0.875rem;
  font-weight: 600;
}

.change-valid {
  color: #4ade80;
}

.change-insufficient {
  color: #f87171;
}

.change-amount {
  font-size: 1rem;
  font-weight: 700;
}

.checkout-btn {
  width: 100%;
  background-color: #22c55e;
  color: #ffffff;
  font-weight: 600;
  padding: 0.75rem;
  border-radius: 0.5rem;
  border: none;
  cursor: pointer;
  margin-top: 0.25rem;
  font-size: 0.95rem;
  transition: background-color 0.2s;
}

.checkout-btn:hover:not(:disabled) {
  background-color: #16a34a;
}

.checkout-btn:disabled {
  background-color: var(--bg-hover);
  color: var(--text-muted);
  cursor: not-allowed;
}

.empty-message {
  text-align: center;
  padding: 2rem;
  color: var(--text-muted);
}

.text-center {
  text-align: center;
}

.font-medium {
  font-weight: 500;
}

.text-slate-200 {
  color: var(--text-primary);
}

.locked-discount-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.35rem 0.75rem;
  border-radius: 0.375rem;
  border: 1px solid var(--border-color);
  background-color: var(--bg-surface);
  color: var(--accent-color);
  font-size: 0.8rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}

.locked-discount-btn:hover {
  background-color: var(--bg-hover);
  border-color: var(--accent-color);
}

.pin-modal-overlay {
  position: fixed;
  inset: 0;
  background: rgba(0, 0, 0, 0.65);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
}

.pin-modal-content {
  width: 310px;
  background-color: var(--bg-sidebar);
  border: 1px solid var(--border-color);
  border-radius: 16px;
  padding: 1.25rem;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
}

.pin-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.pin-header h3 {
  margin: 0;
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-primary);
}

.pin-close-btn {
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  padding: 0.25rem;
  display: flex;
}

.pin-display {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 10px;
  padding: 0.9rem;
  margin-bottom: 1rem;
  display: flex;
  justify-content: center;
}

.pin-dots {
  display: flex;
  gap: 0.85rem;
}

.pin-dot {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: 2px solid var(--text-muted);
  transition: all 0.15s;
}

.pin-dot.filled {
  background-color: var(--accent-color);
  border-color: var(--accent-color);
}

.pin-pad {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 0.6rem;
  margin-bottom: 1rem;
}

.pin-btn {
  height: 48px;
  border-radius: 10px;
  border: 1px solid var(--border-color);
  background-color: var(--bg-surface);
  color: var(--text-primary);
  font-size: 1.1rem;
  font-weight: 600;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s;
}

.pin-btn:hover {
  background-color: var(--bg-hover);
  border-color: var(--accent-color);
}

.pin-btn-clear {
  color: #ef4444;
}

.pin-btn-del {
  color: var(--text-secondary);
}

.pin-verify-btn {
  width: 100%;
  padding: 0.7rem;
  border-radius: 10px;
  border: none;
  background-color: var(--accent-color);
  color: #ffffff;
  font-weight: 600;
  font-size: 0.95rem;
  cursor: pointer;
}

.pin-verify-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

@media (max-width: 960px) {
  .pos-container {
    flex-direction: column;
    height: auto;
  }

  .cart-panel {
    min-width: 0;
  }
}

@media (max-width: 639px) {
  .view-switcher {
    display: none;
  }
}
</style>
