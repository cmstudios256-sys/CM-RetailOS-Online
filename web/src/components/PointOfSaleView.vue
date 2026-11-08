<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'
import {
  AlertCircle,
  Camera,
  Barcode,
  CheckCircle2,
  ChevronUp,
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
import { usePhoneBreakpoint, useTabletBreakpoint } from '../utils/breakpoints'
import { escapeHtml, openPrintDocument, receiptNumber, shortSaleId } from '../utils/reporting'
import { useBottomSheetDrag } from '../utils/useBottomSheetDrag'
import {
  getDefaultBluetoothPrinter,
  buildEscPosReceipt,
  sendToBluetoothPrinter,
} from '../utils/bluetoothPrinter'

type CartItem = {
  product: Product
  quantity: number
}

const {
  sheetRef: mobileCartSheetRef,
  sheetStyle: mobileCartSheetStyle,
  onPointerDown: onCartPointerDown,
  onPointerMove: onCartPointerMove,
  onPointerUp: onCartPointerUp,
  onPointerCancel: onCartPointerCancel,
} = useBottomSheetDrag(() => {
  mobileCartOpen.value = false
})

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
const continuousScanMode = ref(true)
const cameraScannedCount = ref(0)
const cameraLastScannedItem = ref('')
let lastScannedBarcode = ''
let lastScannedTime = 0
const shareSale = ref<Sale | null>(null)
const { isPhone } = usePhoneBreakpoint()
const { isTablet } = useTabletBreakpoint()
let cameraControls: IScannerControls | null = null
let cameraSession = 0
const search = ref('')
const searchInputRef = ref<HTMLInputElement | null>(null)
const searchPlaceholder = computed(() => isPhone.value || isTablet.value ? 'Search products' : 'Scan barcode or search products...')
let savedViewMode = (typeof localStorage !== 'undefined' ? localStorage.getItem('cm_pos_view_mode') : null) as 'list' | 'grid' | null
const viewMode = ref<'list' | 'grid'>(
  isPhone.value || isTablet.value ? 'grid' : savedViewMode === 'list' ? 'list' : 'grid',
)
function setViewMode(mode: 'list' | 'grid') {
  if (isPhone.value || isTablet.value) return
  viewMode.value = mode
  savedViewMode = mode
  try {
    localStorage.setItem('cm_pos_view_mode', mode)
  } catch {}
}

watch([isPhone, isTablet], ([phone, tablet]) => {
  updatePosGridColumns()
  if (phone || tablet) {
    viewMode.value = 'grid'
    mobileTouch.value = isTouchDevice() || phone || tablet
    if (typeof navigator !== 'undefined') {
      cameraSupported.value = true
    }
  } else {
    viewMode.value = savedViewMode === 'list' ? 'list' : 'grid'
    mobileTouch.value = isTouchDevice()
    cameraSupported.value = typeof navigator !== 'undefined' && !!navigator.mediaDevices?.getUserMedia
  }
})

const productImagePreview = ref<{ src: string; alt: string; left: number; top: number } | null>(null)

function updateProductImagePreview(event: MouseEvent, product: Product) {
  if (isPhone.value || isTablet.value || mobileTouch.value) {
    hideProductImagePreview()
    return
  }
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
const shouldShowPosImages = computed(() => isPhone.value || onlinePrefs.value.applicationSettings?.showPosImages !== false)

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
  if (!isPhone.value && onlinePrefs.value.applicationSettings?.barcodeBeep === false) return
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

const posGridRef = ref<HTMLElement | null>(null)
const posGridColumns = ref(3)
const posGridRowHeight = computed(() => isPhone.value ? 132 : isTablet.value ? 132 : 158)

function updatePosGridColumns() {
  if (isPhone.value) {
    posGridColumns.value = 2
  } else if (isTablet.value) {
    posGridColumns.value = window.innerWidth >= 768 ? 4 : 3
  } else {
    posGridColumns.value = 3
  }
}

const posGridRowCount = computed(() =>
  Math.ceil(filteredProducts.value.length / posGridColumns.value),
)

const posGridVirtualizer = useVirtualizer(computed(() => ({
  count: posGridRowCount.value,
  getScrollElement: () => posGridRef.value,
  estimateSize: () => posGridRowHeight.value,
  overscan: 3,
})))

const virtualPosRows = computed(() => posGridVirtualizer.value.getVirtualItems())

function posGridProducts(rowIndex: number) {
  const start = rowIndex * posGridColumns.value
  return filteredProducts.value.slice(start, start + posGridColumns.value)
}

const posListRef = ref<HTMLElement | null>(null)
const posListVirtualizer = useVirtualizer(computed(() => ({
  count: filteredProducts.value.length,
  getScrollElement: () => posListRef.value,
  estimateSize: () => 38,
  overscan: 8,
})))
const virtualPosListItems = computed(() => posListVirtualizer.value.getVirtualItems())

watch(
  () => [search.value, viewMode.value, filteredProducts.value.length],
  () => {
    posGridRef.value?.scrollTo({ top: 0 })
    posListRef.value?.scrollTo({ top: 0 })
  },
)

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
  if (cameraOpen.value) return
  cameraError.value = ''
  cameraOpen.value = true
  cameraScannedCount.value = 0
  cameraLastScannedItem.value = ''
  lastScannedBarcode = ''
  lastScannedTime = 0
  const session = ++cameraSession

  if (typeof navigator === 'undefined' || !navigator.mediaDevices?.getUserMedia) {
    cameraError.value = window.isSecureContext === false
      ? 'Camera access requires a secure connection (HTTPS or localhost).'
      : 'Camera is not supported on this browser or device.'
    return
  }

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
        if (!code) return

        const now = Date.now()
        // Prevent duplicate burst scans of the same code within 1.8 seconds
        if (code === lastScannedBarcode && now - lastScannedTime < 1800) return
        lastScannedBarcode = code
        lastScannedTime = now

        const match = products.value.find(product =>
          (product.barcode && product.barcode.toLowerCase() === code.toLowerCase())
          || product.sku.toLowerCase() === code.toLowerCase(),
        )

        if (match) {
          playScannerBeep()
          addToCart(match)
          cameraScannedCount.value++
          cameraLastScannedItem.value = `✓ ${match.name}`
          triggerSuccessToast(`${match.name} added to cart.`)
        } else {
          cameraLastScannedItem.value = `⚠ Not found: ${code}`
          triggerErrorToast(`Barcode not found: "${code}"`)
        }

        if (!continuousScanMode.value) {
          closeBarcodeCamera()
        }
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
  cameraScannedCount.value = 0
  cameraLastScannedItem.value = ''
  const stream = cameraVideo.value?.srcObject
  if (stream instanceof MediaStream) stream.getTracks().forEach(track => track.stop())
  if (cameraVideo.value) cameraVideo.value.srcObject = null
}

function openReceiptShare(sale: Sale) {
  if (mobileTouch.value || isPhone.value || isTablet.value) shareSale.value = sale
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
  if (activeTag === 'TEXTAREA' || activeTag === 'SELECT') return
  // If focused on a non-search input (e.g. discount, customer name), allow normal typing
  if (activeTag === 'INPUT' && document.activeElement !== searchInputRef.value) return

  const now = Date.now()
  // 100ms tolerance accommodates Bluetooth scanner packet delay and Android wedge services
  if (now - lastKeyTime > 100) {
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
        if (searchInputRef.value && document.activeElement === searchInputRef.value) {
          search.value = ''
        }
        triggerSuccessToast(`${match.name} added to cart.`)
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
  discountAmount.value = ''
  amountTendered.value = ''
  isDiscountUnlocked.value = false
  error.value = ''
}

function printCompletedReceipt(sale: Sale) {
  const settings = shopSettings.value
  const storeTitle = settings?.shopName || props.shopName

  if (isPhone.value || isTablet.value) {
    const btPrinter = getDefaultBluetoothPrinter()
    if (btPrinter) {
      const bytes = buildEscPosReceipt(sale, {
        shopName: storeTitle,
        currency: props.currency,
        paperWidth: btPrinter.paperWidth || '58mm',
      })
      sendToBluetoothPrinter(btPrinter, bytes)
        .then(res => {
          triggerSuccessToast(res.message || `Receipt printed to ${btPrinter.name}`)
        })
        .catch(err => {
          triggerSuccessToast(err.message || 'Bluetooth printer error')
        })
      return
    } else {
      shareSale.value = sale
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
        customerName: '',
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
    if (isPhone.value || isTablet.value || prefs.applicationSettings?.autoClearCart !== false) {
      clearCart()
      mobileCartOpen.value = false
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

let gridResizeObserver: ResizeObserver | null = null

onMounted(() => {
  mobileTouch.value = isTouchDevice() || isPhone.value || isTablet.value
  cameraSupported.value = typeof navigator !== 'undefined' && (!!navigator.mediaDevices?.getUserMedia || isPhone.value || isTablet.value)
  loadProducts()
  updatePosGridColumns()
  if (posGridRef.value && typeof ResizeObserver !== 'undefined') {
    gridResizeObserver = new ResizeObserver(() => {
      updatePosGridColumns()
    })
    gridResizeObserver.observe(posGridRef.value)
  }
  window.addEventListener('resize', updatePosGridColumns)
  window.addEventListener('keydown', handleGlobalKeydown)
  window.addEventListener('online-prefs-updated', handlePrefsUpdated)
})

onUnmounted(() => {
  closeBarcodeCamera()
  gridResizeObserver?.disconnect()
  window.removeEventListener('resize', updatePosGridColumns)
  window.removeEventListener('keydown', handleGlobalKeydown)
  window.removeEventListener('online-prefs-updated', handlePrefsUpdated)
  if (toastTimer) clearTimeout(toastTimer)
})
</script>

<template>
  <div class="pos-container" :class="{ 'has-cart-items': cartUnits > 0, 'tablet-pos': isTablet }">
    <!-- Toast Notifications -->
    <div v-if="showErrorToast" class="pos-toast pos-toast-error" role="alert">
      <AlertCircle :size="20" />
      <span>{{ errorToastMessage }}</span>
    </div>
    <div v-if="showSuccessToast && !isPhone" class="pos-toast pos-toast-success" role="status">
      <CheckCircle2 :size="20" />
      <span>{{ successToastMessage }}</span>
      <button v-if="completedSale" type="button" class="toast-print-btn" @click="printCompletedReceipt(completedSale)">
        <Printer :size="14" /> Print Receipt
      </button>
    </div>
    <Transition name="mobile-cart-slide">
      <button
        v-if="cartUnits > 0"
        type="button"
        class="mobile-cart-toggle has-items"
        aria-controls="pos-cart-panel"
        :aria-expanded="mobileCartOpen"
        @click="mobileCartOpen = true"
      >
        <div class="mobile-cart-toggle-left">
          <div class="cart-icon-bubble">
            <ShoppingCart :size="17" />
            <span class="cart-bubble-badge">{{ formatQuantity(cartUnits) }}</span>
          </div>
          <span class="cart-toggle-label">View Order · {{ formatQuantity(cartUnits) }} {{ cartUnits === 1 ? 'item' : 'items' }}</span>
        </div>
        <div class="mobile-cart-toggle-right">
          <strong class="cart-toggle-total">{{ formatMoney(payableTotal, currency) }}</strong>
          <ChevronUp :size="18" class="cart-toggle-chevron" />
        </div>
      </button>
    </Transition>

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
              ref="searchInputRef"
              v-model="search"
              type="text"
              :placeholder="searchPlaceholder"
              :aria-label="isPhone || isTablet ? 'Search products' : 'Scan barcode or search products'"
              @keydown.enter.prevent="handleSearchInputEnter"
            />
            <div class="scanner-indicator" title="Hardware & Bluetooth scanner ready • Sunmi, Zebra, Honeywell & Bluetooth scanners supported">
              <Barcode :size="16" class="scanner-icon" />
              <span class="scanner-dot"></span>
            </div>
          </div>
          <button
            v-if="cameraSupported && (isPhone || isTablet || mobileTouch)"
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
      <div v-else-if="viewMode === 'grid'" ref="posGridRef" class="product-grid">
        <div v-if="!filteredProducts.length" class="empty-grid-message">No products found.</div>
        <div
          v-else
          class="virtual-grid-spacer"
          :style="{ height: `${posGridVirtualizer.getTotalSize()}px` }"
        >
          <div
            v-for="virtualRow in virtualPosRows"
            :key="String(virtualRow.key)"
            class="virtual-grid-row"
            :style="{
              transform: `translateY(${virtualRow.start}px)`,
              gridTemplateColumns: isPhone ? 'repeat(2, minmax(0, 1fr))' : isTablet ? `repeat(${posGridColumns}, minmax(0, 1fr))` : 'repeat(3, minmax(0, 185px))',
            }"
          >
            <div
              v-for="product in posGridProducts(virtualRow.index)"
              :key="product.id"
              class="product-card"
              :class="{ disabled: product.quantity <= 0, 'out-of-stock': product.quantity <= 0, 'low-stock': product.quantity > 0 && product.quantity <= product.minStock, 'in-stock': product.quantity > product.minStock, 'no-image': !product.image, 'has-product-image': !!product.image }"
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
                  v-if="shouldShowPosImages"
                  class="product-image-small"
                  :class="{ 'has-image': !!product.image }"
                  @pointerenter="!isPhone && !isTablet && !mobileTouch && product.image && updateProductImagePreview($event, product)"
                  @pointermove="!isPhone && !isTablet && !mobileTouch && product.image && updateProductImagePreview($event, product)"
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
                    <ImageIcon :size="34" class="fallback-icon" :stroke-width="1.6" />
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div v-if="filteredProducts.length" class="pos-scroll-bottom-spacer" aria-hidden="true"></div>
      </div>

      <!-- Product List View -->
      <div v-else ref="posListRef" class="product-list-container virtual-list-container">
        <div class="virtual-list-header pos-list-header">
          <span>Name</span><span>Category</span><span>Price</span><span>Quantity</span><span>Action</span>
        </div>
        <div v-if="!filteredProducts.length" class="empty-grid-message">No products found.</div>
        <div
          v-else
          class="virtual-list-spacer"
          :style="{ height: `${posListVirtualizer.getTotalSize()}px` }"
        >
          <div
            v-for="virtualRow in virtualPosListItems"
            :key="String(virtualRow.key)"
            class="virtual-list-row pos-list-header"
            :class="{ 'out-of-stock': filteredProducts[virtualRow.index]?.quantity <= 0 }"
            :style="{ transform: `translateY(${virtualRow.start}px)`, height: `${virtualRow.size}px` }"
            @click="filteredProducts[virtualRow.index] && addToCart(filteredProducts[virtualRow.index])"
          >
            <template v-if="filteredProducts[virtualRow.index]">
              <span class="font-medium text-slate-200" :title="filteredProducts[virtualRow.index].name">{{ filteredProducts[virtualRow.index].name }}</span>
              <span :title="filteredProducts[virtualRow.index].category || 'Uncategorized'">{{ filteredProducts[virtualRow.index].category || 'Uncategorized' }}</span>
              <span>{{ formatMoney(filteredProducts[virtualRow.index].sellingPrice, currency) }}</span>
              <span>
                <span class="stock-badge" :class="getStockStatusClass(filteredProducts[virtualRow.index])">
                  {{ getStockStatusText(filteredProducts[virtualRow.index]) }}
                </span>
              </span>
              <span>
                <button
                  type="button"
                  class="add-btn-list"
                  :disabled="filteredProducts[virtualRow.index].quantity <= 0"
                  title="Add to cart"
                  @click.stop="addToCart(filteredProducts[virtualRow.index])"
                >
                  <Plus :size="16" />
                </button>
              </span>
            </template>
          </div>
        </div>
        <div v-if="filteredProducts.length" class="pos-scroll-bottom-spacer" aria-hidden="true"></div>
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
      ref="mobileCartSheetRef"
      class="cart-panel"
      :class="{ 'mobile-cart-open': mobileCartOpen }"
      :style="mobileCartOpen ? mobileCartSheetStyle : undefined"
    >
      <div
        class="mobile-sheet-drag-area"
        @pointerdown="onCartPointerDown"
        @pointermove="onCartPointerMove"
        @pointerup="onCartPointerUp"
        @pointercancel="onCartPointerCancel"
      >
        <div class="mobile-sheet-drag-handle" aria-hidden="true"></div>
      </div>
      <div
        class="panel-header cart-panel-header"
        @pointerdown="onCartPointerDown"
        @pointermove="onCartPointerMove"
        @pointerup="onCartPointerUp"
        @pointercancel="onCartPointerCancel"
      >
        <div class="cart-title-group">
          <div class="cart-title-badge">
            <ShoppingCart :size="17" />
          </div>
          <h2 class="panel-title">Cart</h2>
          <span v-if="cartUnits > 0" class="cart-count-pill">{{ formatQuantity(cartUnits) }} {{ cartUnits === 1 ? 'item' : 'items' }}</span>
        </div>
        <div class="cart-header-actions">
          <button
            v-if="cart.length > 0"
            type="button"
            class="clear-cart-btn"
            title="Clear Cart"
            @click="clearCart"
          >
            <span class="clear-btn-text">Clear</span>
          </button>
          <button
            type="button"
            class="mobile-cart-close"
            aria-label="Close cart"
            @click="mobileCartOpen = false"
          >
            <X :size="20" />
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
          v-if="mobileTouch || isPhone || isTablet"
          type="button"
          class="mobile-receipt-share"
          @click="openReceiptShare(completedSale)"
        >
          <Share2 :size="16" /> Share to WhatsApp
        </button>
      </div>

      <div v-if="cart.length === 0" class="empty-cart">
        <div class="empty-cart-icon-wrap">
          <ShoppingCart :size="40" />
        </div>
        <p class="empty-cart-title">Your cart is empty</p>
        <span class="empty-cart-subtitle">Add products from the catalog to build an order.</span>
      </div>

      <div v-else class="cart-items">
        <div v-for="item in cart" :key="item.product.id" class="cart-item">
          <div class="cart-item-top">
            <div class="item-info">
              <p class="item-name" :title="item.product.name">{{ item.product.name }}</p>
              <p class="item-price">{{ formatMoney(item.product.sellingPrice, currency) }} each</p>
            </div>
            <p class="item-total">{{ formatMoney(item.product.sellingPrice * item.quantity, currency) }}</p>
          </div>
          <div class="cart-item-bottom">
            <div class="quantity-controls">
              <button
                type="button"
                class="qty-btn"
                aria-label="Decrease quantity"
                @click="changeQuantity(item, item.quantity - 1)"
              >
                <Minus :size="12" />
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
                class="qty-btn"
                aria-label="Increase quantity"
                :disabled="item.quantity >= item.product.quantity"
                @click="changeQuantity(item, item.quantity + 1)"
              >
                <Plus :size="12" />
              </button>
            </div>
            <button
              type="button"
              class="remove-item-btn"
              title="Remove item"
              aria-label="Remove item"
              @click="removeFromCart(item.product.id)"
            >
              <Trash2 :size="13" />
            </button>
          </div>
        </div>
      </div>

      <div class="order-summary">
        <div class="summary-row discount-row">
          <span class="summary-label">Discount</span>
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

        <div class="summary-total-banner">
          <div class="total-text-group">
            <span class="total-label">Total to Pay</span>
            <span v-if="cartUnits > 0" class="total-badge">{{ formatQuantity(cartUnits) }} {{ cartUnits === 1 ? 'item' : 'items' }}</span>
          </div>
          <strong class="total-amount">{{ formatMoney(payableTotal, currency) }}</strong>
        </div>

        <!-- Change Calculator Section -->
        <div v-if="cart.length > 0 && paymentMethod === 'cash'" class="change-calculator">
          <div class="tendered-input-row">
            <label class="tendered-label">Cash Received</label>
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
            <span class="change-status-text">{{ changeDue < 0 ? 'Still Due' : 'Change to Return' }}</span>
            <span class="change-amount">{{ formatMoney(Math.abs(changeDue), currency) }}</span>
          </div>
        </div>

        <button
          type="button"
          class="checkout-btn"
          :disabled="cart.length === 0 || submitting || !discountValid || !tenderValid"
          @click="checkout"
        >
          <span v-if="submitting">Processing…</span>
          <span v-else class="checkout-btn-content">
            <span>Checkout</span>
            <span class="checkout-btn-price">{{ formatMoney(payableTotal, currency) }}</span>
          </span>
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
              <p>Hold barcode inside frame · Auto-adds to cart</p>
            </div>
            <button type="button" class="barcode-camera-close" aria-label="Close barcode scanner" @click="closeBarcodeCamera">
              <X :size="19" />
            </button>
          </header>
          <div class="barcode-camera-viewfinder-wrap">
            <video ref="cameraVideo" class="barcode-camera-preview" autoplay muted playsinline></video>
            <div class="barcode-scan-reticle">
              <div class="reticle-corner tl"></div>
              <div class="reticle-corner tr"></div>
              <div class="reticle-corner bl"></div>
              <div class="reticle-corner br"></div>
              <div class="reticle-laser-line"></div>
            </div>
            <div v-if="cameraLastScannedItem" class="barcode-scan-feedback">
              {{ cameraLastScannedItem }}
            </div>
          </div>
          <p v-if="cameraError" class="barcode-camera-error" role="alert">{{ cameraError }}</p>
          <div v-else class="barcode-camera-toolbar">
            <label class="continuous-scan-toggle">
              <input type="checkbox" v-model="continuousScanMode" />
              <span>Continuous Scan (Keep open)</span>
            </label>
            <span v-if="cameraScannedCount > 0" class="scanned-count-badge">
              {{ cameraScannedCount }} scanned
            </span>
          </div>
          <button type="button" class="barcode-camera-cancel" @click="closeBarcodeCamera">
            {{ cameraScannedCount > 0 ? `Done (${cameraScannedCount} added)` : 'Cancel scan' }}
          </button>
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
        v-if="productImagePreview && !isPhone && !isTablet && !mobileTouch"
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

.pos-scroll-bottom-spacer {
  display: none;
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

.barcode-camera-viewfinder-wrap {
  position: relative;
  width: 100%;
  overflow: hidden;
  border-radius: 12px;
  background: #020617;
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

.barcode-scan-reticle {
  position: absolute;
  inset: 15% 12%;
  border-radius: 8px;
  pointer-events: none;
}

.reticle-corner {
  position: absolute;
  width: 20px;
  height: 20px;
  border-color: #38bdf8;
  border-style: solid;
}

.reticle-corner.tl { top: 0; left: 0; border-width: 3px 0 0 3px; border-top-left-radius: 6px; }
.reticle-corner.tr { top: 0; right: 0; border-width: 3px 3px 0 0; border-top-right-radius: 6px; }
.reticle-corner.bl { bottom: 0; left: 0; border-width: 0 0 3px 3px; border-bottom-left-radius: 6px; }
.reticle-corner.br { bottom: 0; right: 0; border-width: 0 3px 3px 0; border-bottom-right-radius: 6px; }

.reticle-laser-line {
  position: absolute;
  top: 50%;
  left: 5%;
  right: 5%;
  height: 2px;
  background: #ef4444;
  box-shadow: 0 0 8px #ef4444;
  animation: posLaserSweep 2s ease-in-out infinite alternate;
}

@keyframes posLaserSweep {
  0% { transform: translateY(-40px); opacity: 0.7; }
  100% { transform: translateY(40px); opacity: 1; }
}

.barcode-scan-feedback {
  position: absolute;
  bottom: 12px;
  left: 50%;
  transform: translateX(-50%);
  padding: 6px 14px;
  border-radius: 999px;
  background: rgba(15, 23, 42, 0.92);
  border: 1px solid rgba(56, 189, 248, 0.4);
  color: #38bdf8;
  font-size: 13px;
  font-weight: 600;
  white-space: nowrap;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
}

.barcode-camera-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin: 14px 0;
  gap: 12px;
}

.continuous-scan-toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: #cbd5e1;
  cursor: pointer;
  user-select: none;
}

.continuous-scan-toggle input[type="checkbox"] {
  accent-color: #0284c7;
  width: 16px;
  height: 16px;
  cursor: pointer;
}

.scanned-count-badge {
  display: inline-flex;
  align-items: center;
  padding: 4px 10px;
  border-radius: 999px;
  background: rgba(34, 197, 94, 0.18);
  border: 1px solid rgba(34, 197, 94, 0.4);
  color: #86efac;
  font-size: 12px;
  font-weight: 600;
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
  display: block;
  grid-template-columns: repeat(auto-fill, minmax(155px, 1fr));
  grid-auto-rows: max-content;
  gap: 0.75rem;
  overflow-y: auto;
  flex-grow: 1;
  align-content: start;
}

.virtual-grid-spacer {
  position: relative;
  width: 100%;
}

.virtual-grid-row {
  position: absolute;
  left: 0;
  right: 0;
  top: 0;
  display: grid;
  gap: 0.75rem;
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
  padding: 9px 10px 10px 11px;
  cursor: pointer;
  transition: transform 0.2s cubic-bezier(0.16, 1, 0.3, 1), box-shadow 0.2s, border-color 0.2s, background-color 0.2s;
  display: flex;
  flex-direction: column;
  position: relative;
  min-height: 146px;
  height: 146px;
  max-width: 185px;
  width: 100%;
  box-sizing: border-box;
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
  align-items: flex-end;
  justify-content: space-between;
  gap: 6px;
  width: 100%;
  margin-top: auto;
  position: relative;
}

.card-meta {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  justify-content: flex-end;
  gap: 5px;
  flex: 1 1 0%;
  min-width: 0;
  overflow: hidden;
}

.product-stock {
  display: inline-flex;
  align-items: center;
  font-size: 10.5px;
  font-weight: 700;
  padding: 2px 6px;
  border-radius: 9999px;
  line-height: 1.2;
  white-space: nowrap;
  max-width: 100%;
  overflow: hidden;
  text-overflow: ellipsis;
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
  font-size: 14px;
  line-height: 1.2;
  white-space: nowrap;
  letter-spacing: -0.01em;
  margin-top: 1px;
  text-shadow: 0 0 5px rgba(15, 23, 42, 0.9), 0 0 10px rgba(15, 23, 42, 0.8);
}

[data-theme='light'] .product-price {
  color: #ea580c;
  font-weight: 850;
  text-shadow: 0 0 5px rgba(255, 255, 255, 0.95), 0 0 10px rgba(255, 255, 255, 0.9);
}

.product-image-small {
  width: 66px;
  height: 66px;
  margin-left: auto;
  flex-shrink: 0;
  border-radius: 9px;
  overflow: hidden;
  background: rgba(15, 23, 42, 0.7);
  border: 1px solid rgba(255, 255, 255, 0.12);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: transform 0.25s ease, border-color 0.2s ease, box-shadow 0.2s ease;
}

[data-theme='light'] .product-image-small {
  background: #f1f5f9;
  border: 1px solid #cbd5e1;
}

.real-product-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform 0.25s ease;
}

.image-fallback {
  background: linear-gradient(135deg, rgba(30, 41, 59, 0.5) 0%, rgba(15, 23, 42, 0.75) 100%);
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: background-color 0.2s ease;
}

[data-theme='light'] .image-fallback {
  background: linear-gradient(135deg, #f8fafc 0%, #e2e8f0 100%);
}

.product-card:hover .image-fallback {
  background: rgba(14, 165, 233, 0.12);
}

.fallback-icon {
  width: 34px;
  height: 34px;
  color: #94a3b8;
  opacity: 0.85;
  transition: color 0.2s ease, opacity 0.2s ease, transform 0.2s ease;
}

@media (hover: hover) and (pointer: fine) and (min-width: 1081px) {
  .product-card:hover .product-image-small {
    transform: scale(1.04);
    border-color: rgba(14, 165, 233, 0.35);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.25);
  }

  .product-card:hover .fallback-icon {
    color: var(--accent-color, #38bdf8);
    opacity: 1;
    transform: scale(1.06);
  }
}

@media (max-width: 1080px), (hover: none), (pointer: coarse) {
  .product-card:hover .product-image-small,
  .product-card:active .product-image-small,
  .product-image-small {
    transform: none !important;
    box-shadow: none !important;
  }

  .product-card:hover .fallback-icon,
  .product-card:active .fallback-icon,
  .fallback-icon {
    transform: none !important;
  }

  .product-image-hover-preview {
    display: none !important;
  }
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

.virtual-list-container {
  position: relative;
}

.virtual-list-spacer {
  position: relative;
  width: 100%;
}

.virtual-list-header,
.virtual-list-row {
  display: grid;
  grid-template-columns: minmax(140px, 2.2fr) minmax(110px, 1.3fr) minmax(90px, 1fr) minmax(100px, 1fr) 52px;
  min-width: 500px;
}

.virtual-list-header {
  position: sticky;
  top: 0;
  z-index: 20;
  min-height: 38px;
  align-items: center;
  background: var(--bg-surface);
  border-bottom: 1px solid var(--border-subtle);
  font-size: 0.75rem;
  text-transform: uppercase;
  color: var(--text-muted);
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
  min-width: 0;
}

.virtual-list-header > span:last-child,
.virtual-list-row > span:last-child {
  border-right: none;
  padding: 0.4rem 0.25rem;
}

.virtual-list-row {
  position: absolute;
  left: 0;
  right: 0;
  top: 0;
  cursor: pointer;
}

.virtual-list-row:hover {
  background: var(--bg-surface);
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
.mobile-sheet-drag-handle {
  display: none;
}

.cart-panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.85rem 1.15rem;
  border-bottom: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
}

.cart-title-group {
  display: flex;
  align-items: center;
  gap: 0.6rem;
}

.cart-title-badge {
  width: 32px;
  height: 32px;
  border-radius: 9px;
  background: transparent;
  color: #38bdf8;
  display: grid;
  place-items: center;
  border: none;
}

.cart-count-pill {
  padding: 2px 8px;
  border-radius: 9999px;
  font-size: 0.72rem;
  font-weight: 600;
  background: rgba(56, 189, 248, 0.15);
  color: #38bdf8;
  border: 1px solid rgba(56, 189, 248, 0.28);
}

.cart-header-actions {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.clear-cart-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  font-size: 0.7rem;
  font-weight: 650;
  color: #f87171;
  background: rgba(239, 68, 68, 0.12);
  border: none !important;
  outline: none !important;
  cursor: pointer;
  height: 24px;
  padding: 0 7px;
  border-radius: 6px;
  transition: all 0.15s;
  box-sizing: border-box;
}

.clear-cart-btn:hover {
  color: #ef4444;
  background-color: rgba(239, 68, 68, 0.24);
}

.clear-btn-text {
  display: inline;
}

.mobile-cart-close {
  display: none;
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
  padding: 2rem 1.5rem;
}

.empty-cart-icon-wrap {
  width: 64px;
  height: 64px;
  border-radius: 16px;
  background: rgba(148, 163, 184, 0.08);
  border: 1px dashed rgba(148, 163, 184, 0.2);
  display: grid;
  place-items: center;
  color: var(--text-muted);
  margin-bottom: 0.75rem;
}

.empty-cart-title {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text-secondary);
  margin: 0 0 0.25rem;
}

.empty-cart-subtitle {
  font-size: 0.8rem;
  color: var(--text-muted);
  max-width: 220px;
}

.cart-items {
  flex: 1 1 auto;
  min-height: 0;
  max-height: none;
  overflow-y: auto;
  padding: 0.75rem 1rem;
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}

.cart-item {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  padding: 0.65rem 0.75rem;
  background: rgba(30, 41, 59, 0.5);
  border: 1px solid rgba(255, 255, 255, 0.07);
  border-radius: 10px;
  transition: border-color 0.15s, background-color 0.15s;
}

.cart-item:hover {
  background: rgba(30, 41, 59, 0.75);
  border-color: rgba(56, 189, 248, 0.2);
}

.cart-item-top {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 0.6rem;
}

.cart-item-bottom {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.5rem;
  padding-top: 0.25rem;
  border-top: 1px dashed rgba(255, 255, 255, 0.06);
}

.item-info {
  flex-grow: 1;
  min-width: 0;
}

.item-name {
  margin: 0;
  font-weight: 600;
  color: var(--text-primary);
  font-size: 0.88rem;
  line-height: 1.25;
  word-break: break-word;
}

.item-price {
  margin: 0.15rem 0 0;
  font-size: 0.75rem;
  color: var(--text-muted);
}

.quantity-controls {
  display: inline-flex;
  align-items: center;
  gap: 0.15rem;
  background-color: rgba(255, 255, 255, 0.06);
  border: none !important;
  outline: none !important;
  border-radius: 6px;
  padding: 1.5px 2px;
}

.quantity-controls button {
  background: rgba(255, 255, 255, 0.06);
  border: none !important;
  outline: none !important;
  color: var(--text-secondary);
  cursor: pointer;
  width: 22px;
  height: 22px;
  border-radius: 4px;
  display: grid;
  place-items: center;
  transition: all 0.15s;
}

.quantity-controls button:hover:not(:disabled) {
  background: rgba(56, 189, 248, 0.2);
  color: #38bdf8;
}

.quantity-controls button:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}

.quantity-input {
  width: 1.9rem;
  text-align: center;
  font-weight: 700;
  font-size: 0.78rem;
  background: transparent;
  border: none !important;
  outline: none !important;
  color: #ffffff;
  padding: 0;
  -moz-appearance: textfield;
}

.quantity-input::-webkit-outer-spin-button,
.quantity-input::-webkit-inner-spin-button {
  -webkit-appearance: none;
  margin: 0;
}

.item-total {
  margin: 0;
  font-weight: 700;
  text-align: right;
  font-size: 0.88rem;
  color: #38bdf8;
  white-space: nowrap;
}

.remove-item-btn {
  background: rgba(239, 68, 68, 0.12);
  border: none !important;
  outline: none !important;
  color: #f87171;
  cursor: pointer;
  width: 24px;
  height: 24px;
  border-radius: 5px;
  display: grid;
  place-items: center;
  transition: all 0.15s;
}

.remove-item-btn:hover {
  background: rgba(239, 68, 68, 0.24);
  color: #ef4444;
}

.order-summary {
  margin-top: auto;
  padding: 0.85rem 1rem;
  border-top: 1px solid var(--border-subtle, rgba(255, 255, 255, 0.08));
  background-color: rgba(15, 23, 42, 0.65);
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
  flex-shrink: 0;
}

[data-theme='light'] .order-summary {
  background-color: #ffffff;
  border-top: 1px solid #e2e8f0;
}

.summary-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.85rem;
}

.summary-subtotal-row {
  color: var(--text-muted);
  font-size: 0.8rem;
}

.summary-total-banner {
  background: linear-gradient(135deg, rgba(30, 41, 59, 0.95) 0%, rgba(15, 23, 42, 0.95) 100%);
  border: none;
  border-radius: 12px;
  padding: 0.75rem 1rem;
  display: flex;
  justify-content: space-between;
  align-items: center;
  box-shadow: none;
}

[data-theme='light'] .summary-total-banner {
  background: linear-gradient(135deg, color-mix(in srgb, var(--accent-color) 8%, #ffffff) 0%, #f8fafc 100%);
  border: none;
  box-shadow: none;
}

[data-theme='light'] .total-label {
  color: #475569;
}

[data-theme='light'] .total-amount {
  color: #0f172a;
}

[data-theme='light'] .total-badge {
  color: var(--accent-color);
  background: color-mix(in srgb, var(--accent-color) 12%, transparent);
}

[data-theme='light'] .discount-input-wrapper {
  background-color: rgba(15, 23, 42, 0.05);
  border: none !important;
}

[data-theme='light'] .discount-input {
  color: #0f172a;
}

[data-theme='light'] .cart-item {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.04);
}

[data-theme='light'] .cart-item:hover {
  background: #f8fafc;
  border-color: #cbd5e1;
}

[data-theme='light'] .cart-item-bottom {
  border-top: 1px dashed #e2e8f0;
}

[data-theme='light'] .quantity-controls {
  background-color: rgba(15, 23, 42, 0.06);
  border: none !important;
}

[data-theme='light'] .quantity-controls button {
  background: #ffffff;
  color: #0f172a;
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.05);
}

[data-theme='light'] .quantity-controls button:hover:not(:disabled) {
  background: #f8fafc;
  color: var(--accent-color);
}

[data-theme='light'] .quantity-input {
  color: #0f172a;
}

[data-theme='light'] .item-total {
  color: var(--accent-color);
}

[data-theme='light'] .checkout-btn:disabled {
  background: #e2e8f0;
  color: #94a3b8;
}

[data-theme='light'] .change-calculator {
  background-color: #f8fafc;
  border-color: #e2e8f0;
}

[data-theme='light'] .tendered-wrapper {
  background-color: rgba(15, 23, 42, 0.05);
  border: none !important;
}

[data-theme='light'] .tendered-input {
  color: var(--accent-color);
}

[data-theme='light'] .cart-panel {
  background: #ffffff !important;
  border-color: #e2e8f0 !important;
}

[data-theme='light'] .cart-panel-header {
  background: #ffffff !important;
  border-bottom: 1px solid #e2e8f0 !important;
}

[data-theme='light'] .cart-title-badge {
  background: transparent !important;
  color: var(--accent-color) !important;
  border: none !important;
}

[data-theme='light'] .cart-count-pill {
  background: color-mix(in srgb, var(--accent-color) 12%, transparent) !important;
  color: var(--accent-color) !important;
  border-color: color-mix(in srgb, var(--accent-color) 25%, transparent) !important;
}

[data-theme='light'] .empty-cart-icon-wrap {
  background: #f1f5f9 !important;
  border-color: #cbd5e1 !important;
  color: #94a3b8 !important;
}

[data-theme='light'] .empty-cart-title {
  color: #0f172a !important;
}

[data-theme='light'] .empty-cart-subtitle {
  color: #64748b !important;
}

[data-theme='light'] .mobile-cart-close {
  background: #f1f5f9 !important;
  border: 1px solid #e2e8f0 !important;
  color: #64748b !important;
}

[data-theme='light'] .mobile-cart-close:hover {
  background: #e2e8f0 !important;
  color: #0f172a !important;
}

[data-theme='light'] .mobile-sheet-drag-handle {
  background: #cbd5e1 !important;
}

.total-text-group {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.total-label {
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: #94a3b8;
  font-weight: 700;
}

.total-badge {
  font-size: 0.7rem;
  color: #38bdf8;
  background: rgba(56, 189, 248, 0.12);
  padding: 1px 6px;
  border-radius: 9999px;
  font-weight: 600;
  width: fit-content;
}

.total-amount {
  font-size: 1.35rem;
  font-weight: 800;
  color: #34d399;
  letter-spacing: -0.01em;
}

.discount-input-wrapper {
  display: flex;
  align-items: center;
  background-color: rgba(255, 255, 255, 0.06);
  border: none !important;
  outline: none !important;
  border-radius: 6px;
  padding: 0 6px;
  width: 92px;
  height: 26px;
  box-sizing: border-box;
  transition: background-color 0.15s ease;
}

.discount-input-wrapper:focus-within {
  background-color: rgba(255, 255, 255, 0.1);
}

.currency-prefix {
  color: var(--text-muted);
  font-size: 0.7rem;
  margin-right: 0.2rem;
  user-select: none;
  font-weight: 600;
}

.discount-input {
  background: transparent;
  border: none !important;
  color: var(--text-primary);
  font-size: 0.78rem;
  font-weight: 650;
  width: 100%;
  text-align: right;
  outline: none !important;
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
  width: 148px;
  min-width: 140px;
  height: 28px;
  background-color: rgba(255, 255, 255, 0.06);
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
  background: linear-gradient(135deg, #22c55e 0%, #16a34a 100%);
  color: #ffffff;
  font-weight: 700;
  padding: 0.8rem 1rem;
  border-radius: 12px;
  border: none;
  cursor: pointer;
  margin-top: 0.35rem;
  font-size: 1rem;
  transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  box-shadow: 0 4px 16px rgba(34, 197, 94, 0.35);
  display: flex;
  align-items: center;
  justify-content: center;
}

.checkout-btn-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.checkout-btn-price {
  background: rgba(0, 0, 0, 0.22);
  padding: 0.25rem 0.65rem;
  border-radius: 8px;
  font-size: 0.92rem;
  font-weight: 800;
  letter-spacing: 0.01em;
}

.checkout-btn:hover:not(:disabled) {
  background: linear-gradient(135deg, #16a34a 0%, #15803d 100%);
  box-shadow: 0 6px 20px rgba(34, 197, 94, 0.45);
  transform: translateY(-1px);
}

.checkout-btn:active:not(:disabled) {
  transform: translateY(0);
}

.checkout-btn:disabled {
  background: var(--bg-hover);
  color: var(--text-muted);
  box-shadow: none;
  cursor: not-allowed;
  transform: none;
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

@media (min-width: 640px) and (max-width: 1080px) {
  .pos-container.tablet-pos {
    width: 100%;
    max-width: 100%;
    margin: 0;
    height: 100%;
    min-height: 0;
    flex: 1 1 0%;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    gap: 0;
  }

  .tablet-pos .products-panel {
    width: 100%;
    height: 100%;
    min-height: 0;
    flex: 1 1 0%;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    border: none !important;
    border-radius: 0;
  }

  .tablet-pos .cart-panel {
    display: none;
  }

  .tablet-pos .cart-panel.mobile-cart-open {
    display: flex !important;
    position: fixed;
    z-index: 45;
    bottom: calc(58px + env(safe-area-inset-bottom));
    left: 50%;
    transform: translateX(-50%);
    width: min(600px, calc(100% - 32px));
    height: min(82dvh, 880px);
    max-height: min(86dvh, 920px);
    min-height: min(68dvh, 600px);
    flex-direction: column;
    overflow: hidden;
    border-radius: 24px;
    background: #0d1527;
    border: 1px solid rgba(56, 189, 248, 0.28);
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.8), 0 0 24px rgba(56, 189, 248, 0.12);
    backdrop-filter: blur(16px);
    animation: mobileCartSlideUp 0.22s cubic-bezier(0.16, 1, 0.3, 1);
  }

  [data-theme='light'] .tablet-pos .cart-panel.mobile-cart-open {
    background: #ffffff;
    border: 1px solid rgba(14, 165, 233, 0.2);
    box-shadow: 0 24px 64px rgba(15, 23, 42, 0.22), 0 0 24px rgba(14, 165, 233, 0.1);
  }

  .tablet-pos .mobile-cart-toggle {
    display: flex !important;
    position: fixed;
    z-index: 39;
    bottom: calc(78px + env(safe-area-inset-bottom));
    left: 50%;
    transform: translateX(-50%);
    width: min(580px, calc(100% - 32px));
    min-height: 54px;
    padding: 0 18px 0 16px;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    border: none !important;
    outline: none !important;
    border-radius: 9999px;
    color: #f1f5f9;
    background: #111b2d;
    box-shadow: 0 16px 36px rgba(0, 0, 0, 0.55), 0 4px 12px rgba(0, 0, 0, 0.35);
    font-size: 13.5px;
    font-weight: 650;
    cursor: pointer;
    transition: transform 0.14s ease, box-shadow 0.16s ease;
  }

  [data-theme='light'] .tablet-pos .mobile-cart-toggle {
    background: #ffffff;
    color: #0f172a;
    box-shadow: 0 16px 36px -4px rgba(15, 23, 42, 0.22), 0 4px 14px rgba(15, 23, 42, 0.1);
  }

  .tablet-pos .mobile-cart-backdrop {
    display: block !important;
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.65);
    backdrop-filter: blur(4px);
    z-index: 44;
  }

  [data-theme='light'] .tablet-pos .mobile-cart-backdrop {
    background: rgba(15, 23, 42, 0.45);
  }

  .tablet-pos .mobile-cart-close {
    display: flex !important;
    align-items: center;
    justify-content: center;
    width: 32px;
    height: 32px;
    border-radius: 8px;
    background: transparent;
    border: none;
    color: var(--text-muted);
    cursor: pointer;
  }

  .tablet-pos .mobile-sheet-drag-area {
    display: block !important;
    padding: 8px 0 4px;
    cursor: grab;
    touch-action: none;
  }

  .tablet-pos .mobile-sheet-drag-handle {
    display: block !important;
    width: 44px;
    height: 4px;
    border-radius: 9999px;
    background: rgba(255, 255, 255, 0.25);
    margin: 0 auto;
  }

  [data-theme='light'] .tablet-pos .mobile-sheet-drag-handle {
    background: rgba(0, 0, 0, 0.2);
  }

  .tablet-pos.has-cart-items .pos-scroll-bottom-spacer {
    display: block !important;
    height: 90px;
    flex-shrink: 0;
  }

  .tablet-pos .mobile-scan-button {
    height: 38px;
    min-height: 38px;
    padding: 0 12px;
    font-size: 12px;
    border-radius: 8px;
    flex-shrink: 0;
    gap: 6px;
    display: inline-flex;
    align-items: center;
    border: none !important;
    outline: none !important;
    background: rgba(14, 165, 233, 0.18);
    color: #38bdf8;
    cursor: pointer;
  }

  [data-theme='light'] .tablet-pos .mobile-scan-button {
    background: var(--accent-color) !important;
    color: #ffffff !important;
  }

  .tablet-pos .panel-header {
    min-height: 56px;
    padding: 8px 10px;
    gap: 8px;
  }

  .tablet-pos .panel-title {
    font-size: 14px;
  }

  .tablet-pos .header-controls {
    flex: 1;
    min-width: 0;
    gap: 8px;
  }

  .tablet-pos .view-switcher {
    display: none;
  }

  .tablet-pos .cart-items {
    flex: 1 1 auto;
    min-height: 220px;
    max-height: none;
    gap: 8px;
    padding: 8px 12px;
  }

  .tablet-pos .cart-item {
    gap: 6px;
    padding: 8px 10px;
    border-radius: 8px;
  }

  .tablet-pos .cart-item-bottom {
    padding-top: 6px;
  }

  .tablet-pos .quantity-controls button,
  .tablet-pos .remove-item-btn,
  .tablet-pos .clear-cart-btn {
    width: 30px;
    min-width: 30px;
    min-height: 26px;
    height: 26px;
  }

  .tablet-pos .clear-cart-btn {
    width: auto;
    min-width: 48px;
    gap: 4px;
    padding: 0 6px;
    font-size: 10px;
  }

  .tablet-pos .quantity-controls .quantity-input {
    min-height: 26px !important;
    height: 26px !important;
  }

  .tablet-pos .item-name {
    font-size: 13px;
    line-height: 1.2;
  }

  .tablet-pos .item-price {
    margin-top: 2px;
    font-size: 10px;
  }

  .tablet-pos .item-total {
    font-size: 12px;
  }

  .tablet-pos .search-bar {
    flex: 1;
    min-width: 0;
  }

  .tablet-pos .search-bar input {
    width: 100%;
  }

  .tablet-pos .product-grid {
    min-height: 0;
    flex: 1 1 0%;
    padding: 8px 10px;
    overflow-y: auto;
    overflow-x: hidden;
    -webkit-overflow-scrolling: touch;
    overscroll-behavior: contain;
  }

  .tablet-pos .virtual-grid-row {
    gap: 8px;
  }

  .tablet-pos .product-card {
    min-width: 0;
    max-width: none;
    min-height: 118px;
    height: 118px;
    padding: 7px 8px;
    overflow: hidden;
    isolation: isolate;
  }

  .tablet-pos .product-card::after {
    position: absolute;
    z-index: 3;
    top: 0;
    right: 12px;
    left: 12px;
    height: 2px;
    border-radius: 0 0 2px 2px;
    background: linear-gradient(90deg, transparent, var(--card-status-accent, var(--accent-color)), transparent);
    content: '';
    pointer-events: none;
  }

  .tablet-pos .product-card.in-stock { --card-status-accent: #10b981; }
  .tablet-pos .product-card.low-stock { --card-status-accent: #f59e0b; }
  .tablet-pos .product-card.out-of-stock { --card-status-accent: #ef4444; }

  .tablet-pos .product-card .product-name {
    font-size: 12px;
  }

  .tablet-pos .product-card.has-product-image::before {
    position: absolute;
    z-index: 1;
    inset: 0;
    background: linear-gradient(90deg, rgba(8, 14, 24, 0.58) 0%, rgba(8, 14, 24, 0.34) 48%, rgba(8, 14, 24, 0.04) 100%);
    content: '';
    pointer-events: none;
  }

  .tablet-pos .product-card.has-product-image > .product-name,
  .tablet-pos .product-card.has-product-image > .product-category,
  .tablet-pos .product-card.has-product-image .card-meta {
    position: relative;
    z-index: 2;
  }

  .tablet-pos .product-card.has-product-image > .product-card-body {
    position: static;
  }

  .tablet-pos .product-card.has-product-image .product-name,
  .tablet-pos .product-card.has-product-image .product-category,
  .tablet-pos .product-card.has-product-image .product-price {
    max-width: 90%;
    padding: 0;
    background: transparent;
    border: 0;
    color: #ffffff !important;
    text-shadow: 0 1px 3px rgba(0, 0, 0, 0.95), 0 0 8px rgba(0, 0, 0, 0.75);
  }

  .tablet-pos .product-card.has-product-image .product-name {
    width: fit-content;
    font-size: 12px;
    font-weight: 700;
    line-height: 1.2;
  }

  .tablet-pos .product-card.has-product-image .product-category {
    font-size: 9px;
    font-weight: 650;
  }

  .tablet-pos .product-card.has-product-image .product-stock {
    position: relative;
    z-index: 2;
    padding: 1px 0;
    border: 0;
    background: transparent;
    color: #ffffff;
    font-size: 9.5px;
    text-shadow: 0 1px 3px rgba(0, 0, 0, 0.95), 0 0 7px rgba(0, 0, 0, 0.8);
  }

  .tablet-pos .product-card.has-product-image .product-price {
    color: #ffb14a !important;
    font-size: 13px;
    font-weight: 700;
    line-height: 1.15;
    text-shadow: 0 1px 3px rgba(0, 0, 0, 0.98), 0 0 8px rgba(0, 0, 0, 0.85);
  }

  .tablet-pos .product-card.has-product-image .product-image-small.has-image {
    position: absolute;
    z-index: 0;
    inset: 0;
    width: 100%;
    height: 100%;
    margin: 0;
    border: 0;
    border-radius: inherit;
    opacity: 1;
  }

  .tablet-pos .product-card.has-product-image.out-of-stock {
    opacity: 1;
    filter: none;
  }

  .tablet-pos .product-card.has-product-image.out-of-stock .real-product-img {
    filter: grayscale(100%) brightness(0.72);
  }

  [data-theme='light'] .tablet-pos .product-card.has-product-image::before {
    background: linear-gradient(90deg, rgba(15, 23, 42, 0.38) 0%, rgba(15, 23, 42, 0.2) 48%, transparent 82%);
  }

  [data-theme='light'] .tablet-pos .product-card.has-product-image .product-stock.in-stock {
    color: #047857;
  }

  [data-theme='light'] .tablet-pos .product-card.has-product-image .product-stock.low {
    color: #92400e;
  }

  [data-theme='light'] .tablet-pos .product-card.has-product-image .product-stock.out {
    color: #b91c1c;
  }

  @media (prefers-reduced-motion: reduce) {
    .tablet-pos .product-card {
      transition: none;
    }
  }
}

@media (max-width: 639px) {
  .pos-container {
    width: 100%;
    max-width: 100%;
    margin: 0;
    height: 100%;
    min-height: 0;
    flex: 1 1 0%;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    gap: 0;
  }

  .products-panel {
    width: 100%;
    height: 100%;
    min-height: 0;
    flex: 1 1 0%;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    border: none !important;
    border-radius: 0;
  }

  .panel-header {
    min-height: 60px;
    padding: 10px 12px;
    gap: 8px;
    border-bottom: none !important;
  }

  .panel-title {
    font-size: 14px;
  }

  .header-controls {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 6px;
    min-width: 0;
  }

  .search-bar {
    flex: 1;
    min-width: 110px;
    max-width: 220px;
    height: 38px;
  }

  .search-bar input {
    width: 100%;
    height: 38px !important;
    min-height: 38px !important;
    padding: 0.4rem 2.25rem 0.4rem 2.25rem;
    font-size: 12px;
    border: none !important;
    outline: none !important;
    box-shadow: none !important;
  }

  .mobile-scan-button {
    height: 38px;
    min-height: 38px;
    padding: 0 10px;
    font-size: 12px;
    border-radius: 8px;
    flex-shrink: 0;
    gap: 5px;
    border: none !important;
    outline: none !important;
    box-shadow: none !important;
  }

  [data-theme='light'] .mobile-scan-button {
    border: none !important;
    color: #ffffff !important;
    background: var(--accent-color) !important;
    box-shadow: 0 2px 7px color-mix(in srgb, var(--accent-color) 28%, transparent) !important;
  }

  .view-switcher {
    display: none;
  }

  .product-grid {
    width: 100%;
    height: 100%;
    min-height: 0;
    flex: 1 1 0%;
    overflow-y: auto;
    -webkit-overflow-scrolling: touch;
    overscroll-behavior: contain;
    display: block;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 8px;
    padding: 8px 8px 12px 8px;
  }

  .product-list-container {
    width: 100%;
    height: 100%;
    min-height: 0;
    flex: 1 1 0%;
    overflow-y: auto;
    -webkit-overflow-scrolling: touch;
    overscroll-behavior: contain;
  }

  .pos-scroll-bottom-spacer {
    display: block;
    width: 100%;
    height: calc(88px + env(safe-area-inset-bottom));
    flex-shrink: 0;
    pointer-events: none;
  }

  .pos-container.has-cart-items .pos-scroll-bottom-spacer {
    height: calc(148px + env(safe-area-inset-bottom));
  }

  .product-card {
    min-width: 0;
    max-width: none;
    min-height: 124px;
    height: 124px;
    padding: 8px;
    border: none !important;
    outline: none !important;
    box-shadow: 0 4px 16px -2px rgba(0, 0, 0, 0.28) !important;
    overflow: hidden;
    isolation: isolate;
    transform-style: preserve-3d;
    transition: transform 0.18s ease, box-shadow 0.18s ease;
  }

  .product-card.has-product-image {
    border: none !important;
    box-shadow: 0 14px 24px -8px rgba(0, 0, 0, 0.42), 0 5px 10px rgba(0, 0, 0, 0.24) !important;
  }

  .product-card::after {
    position: absolute;
    z-index: 3;
    top: 0;
    right: 12px;
    left: 12px;
    height: 2px;
    border-radius: 0 0 2px 2px;
    background: linear-gradient(90deg, transparent, var(--card-status-accent, var(--accent-color)), transparent);
    content: '';
    pointer-events: none;
  }

  .product-card.in-stock { --card-status-accent: #10b981; }
  .product-card.low-stock { --card-status-accent: #f59e0b; }
  .product-card.out-of-stock { --card-status-accent: #ef4444; }

  .product-card.has-product-image::before {
    position: absolute;
    z-index: 1;
    inset: 0;
    background: linear-gradient(90deg, rgba(8, 14, 24, 0.58) 0%, rgba(8, 14, 24, 0.34) 48%, rgba(8, 14, 24, 0.04) 100%);
    content: '';
    pointer-events: none;
  }

  .product-card.has-product-image > .product-name,
  .product-card.has-product-image > .product-category {
    position: relative;
    z-index: 2;
  }

  .product-card.has-product-image > .product-card-body {
    position: static;
  }

  .product-card.has-product-image .card-meta {
    position: relative;
    z-index: 2;
  }

  .product-card.has-product-image .product-image-small.has-image {
    position: absolute;
    z-index: 0;
    inset: 0;
    width: 100%;
    height: 100%;
    margin: 0;
    border: none !important;
    border-radius: inherit;
    opacity: 1;
  }

  [data-theme='light'] .product-card.has-product-image::before {
    background: linear-gradient(90deg, rgba(15, 23, 42, 0.38) 0%, rgba(15, 23, 42, 0.2) 48%, transparent 82%);
  }

  [data-theme='light'] .product-card.has-product-image .product-image-small.has-image {
    opacity: 1;
  }

  [data-theme='light'] .product-card.has-product-image .product-name,
  [data-theme='light'] .product-card.has-product-image .product-category,
  [data-theme='light'] .product-card.has-product-image .product-price {
    max-width: 76%;
    padding: 0;
    border: 0;
    border-radius: 0;
    background: transparent;
    color: #ffffff !important;
    text-shadow: 0 1px 2px rgba(0, 0, 0, 0.98), 0 0 7px rgba(0, 0, 0, 0.85);
    box-shadow: none;
  }

  [data-theme='light'] .product-card.has-product-image .product-name {
    font-weight: 800;
  }

  [data-theme='light'] .product-card.has-product-image .product-category {
    font-size: 10px;
    font-weight: 700;
  }

  [data-theme='light'] .product-card.has-product-image .product-stock {
    padding: 3px 6px;
    border-width: 1px;
    text-shadow: none;
    box-shadow: 0 1px 5px rgba(15, 23, 42, 0.12);
  }

  [data-theme='light'] .product-card.has-product-image .product-stock.in-stock {
    color: #047857;
    background: rgba(236, 253, 245, 0.96);
    border-color: #6ee7b7;
  }

  [data-theme='light'] .product-card.has-product-image .product-stock.low {
    color: #92400e;
    background: rgba(255, 251, 235, 0.97);
    border-color: #fcd34d;
  }

  [data-theme='light'] .product-card.has-product-image .product-stock.out {
    color: #b91c1c;
    background: rgba(254, 242, 242, 0.97);
    border-color: #fca5a5;
  }

  [data-theme='light'] .product-card.has-product-image .product-price {
    color: #ffd08a !important;
    font-weight: 850;
  }

  .product-card:hover,
  .product-card:active {
    border: none !important;
    outline: none !important;
    border-color: transparent !important;
  }

  [data-theme='light'] .product-card {
    border: none !important;
  }

  [data-theme='light'] .product-card.has-product-image {
    border: none !important;
    box-shadow: 0 14px 24px -8px rgba(15, 23, 42, 0.28), 0 5px 10px rgba(15, 23, 42, 0.16) !important;
  }

  .product-card:hover {
    transform: none;
  }

  .product-card:active {
    transform: perspective(700px) rotateX(2deg) rotateY(-1deg) translateY(2px) scale(0.97);
    box-shadow: 0 4px 8px rgba(15, 23, 42, 0.22) !important;
  }

  .product-card .product-name {
    font-size: 12px;
  }

  .product-card.has-product-image .product-name {
    width: fit-content;
    max-width: 76%;
    padding: 0;
    color: #ffffff !important;
    font-weight: 800;
    font-size: 14px;
    line-height: 1.25;
    text-shadow: 0 1px 3px rgba(0, 0, 0, 0.95), 0 0 8px rgba(0, 0, 0, 0.75);
  }

  .product-card .product-category {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    border: none !important;
  }

  .product-card.has-product-image .product-category {
    max-width: 76%;
    padding: 0;
    color: #ffffff !important;
    font-weight: 700;
    font-size: 10px;
    text-shadow: 0 1px 3px rgba(0, 0, 0, 0.95), 0 0 7px rgba(0, 0, 0, 0.8);
  }

  [data-theme='light'] .product-card .product-category {
    border: none !important;
  }

  .product-card-body {
    gap: 6px;
  }

  .product-card.has-product-image .product-stock {
    padding: 2px 0;
    font-size: 11px;
    text-shadow: 0 1px 3px rgba(0, 0, 0, 0.95), 0 0 7px rgba(0, 0, 0, 0.8);
  }

  .product-card.has-product-image .product-price {
    padding: 2px 0;
    color: #ffb14a !important;
    font-size: 16px;
    line-height: 1.15;
    text-shadow: 0 1px 3px rgba(0, 0, 0, 0.98), 0 0 8px rgba(0, 0, 0, 0.85);
  }

  .product-card.has-product-image.out-of-stock {
    opacity: 1;
    filter: none;
  }

  .product-card.has-product-image.out-of-stock .real-product-img {
    filter: grayscale(100%) brightness(0.72);
  }

  .product-image-small {
    width: 64px;
    height: 64px;
    flex-basis: 64px;
    border: none !important;
    outline: none !important;
    box-shadow: none !important;
  }

  .product-card:hover .product-image-small {
    transform: none;
    border: none !important;
    border-color: transparent !important;
    box-shadow: none !important;
  }

  [data-theme='light'] .product-image-small {
    border: none !important;
  }

  .product-stock,
  .product-stock.in-stock,
  .product-stock.low,
  .product-stock.out {
    border: none !important;
    outline: none !important;
  }

  .fallback-icon {
    width: 36px;
    height: 36px;
  }

  .product-card:hover .fallback-icon {
    transform: none;
  }

  @media (prefers-reduced-motion: reduce) {
    .product-card {
      transition: none;
    }

    .product-card:active {
      transform: none;
    }
  }
}
</style>
