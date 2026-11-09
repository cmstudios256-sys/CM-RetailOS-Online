<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'
import { useMobileOrTabletBreakpoint, usePhoneBreakpoint } from '../utils/breakpoints'
import { useDelayedSkeleton } from '../utils/useDelayedSkeleton'
import {
  AlertTriangle,
  ArrowRight,
  Barcode,
  Camera,
  Check,
  ChevronDown,
  Edit,
  Folders,
  ImagePlus,
  Layers,
  Lock,
  Plus,
  PlusCircle,
  Search,
  ShieldCheck,
  SlidersHorizontal,
  Trash2,
  X,
} from '@lucide/vue'
import {
  apiRequest,
  formatMoney,
  formatQuantity,
  type Product,
  type ProductInput,
} from '../api'

const props = defineProps<{
  currency: string
  role: string
  userEmail?: string
  googleConnected?: boolean
  targetStockProductId?: number | null
}>()

const emit = defineEmits<{
  'navigate-settings': []
  'clear-target-stock-product': []
  'stock-adjusted': []
}>()

const products = ref<Product[]>([])
const categoryNames = ref<string[]>([])
const search = ref('')
const selectedCategory = ref('All Categories')
const selectedStock = ref<'All Stock' | 'In Stock' | 'Low Stock' | 'Out of Stock'>('All Stock')
const loading = ref(true)
const showSkeleton = useDelayedSkeleton(loading, 200)
const saving = ref(false)
const adjusting = ref(false)
const categoryBusy = ref(false)
const error = ref('')
const notice = ref('')
const isDropdownOpen = ref(false)
const { isPhone } = usePhoneBreakpoint()
const { isMobileOrTablet } = useMobileOrTabletBreakpoint()
const viewMode = computed<'grid' | 'list'>(() => (isMobileOrTablet.value ? 'grid' : 'list'))

const showForm = ref(false)
const editingProductId = ref<number | null>(null)
const editingOriginalQuantity = ref(0)
const form = ref<ProductInput>(blankProduct())

const formProfit = computed(() => {
  const cost = Number(form.value.costPrice) || 0
  const selling = Number(form.value.sellingPrice) || 0
  return selling - cost
})

const formStockStatus = computed(() => {
  const qty = Number(form.value.quantity) || 0
  const min = Number(form.value.minStock) || 0
  if (qty <= 0) return { label: 'Out of Stock', class: 'out' }
  if (qty <= min) return { label: 'Low Stock Alert', class: 'low' }
  return { label: 'In Stock', class: 'in-stock' }
})
const barcodeType = ref('EAN-13')
const barcodeInputRef = ref<HTMLInputElement | null>(null)
const isBarcodeScanning = ref(false)
let scanTimeout: ReturnType<typeof setTimeout> | null = null

const barcodeTypes = ['EAN-13', 'EAN-8', 'UPC-A', 'UPC-E', 'Code-128', 'Code-39', 'ITF-14', 'QR Code', 'Data Matrix']

function triggerBarcodeScan() {
  if (barcodeInputRef.value) {
    barcodeInputRef.value.focus()
    barcodeInputRef.value.select()
    isBarcodeScanning.value = true
    if (scanTimeout) clearTimeout(scanTimeout)
    scanTimeout = setTimeout(() => {
      isBarcodeScanning.value = false
    }, 10000)
  }
}

function handleBarcodeScanned() {
  if (isBarcodeScanning.value) {
    isBarcodeScanning.value = false
    if (scanTimeout) clearTimeout(scanTimeout)
  }
}

const cameraSupported = ref(false)
const cameraOpen = ref(false)
const cameraError = ref('')
const cameraVideo = ref<HTMLVideoElement | null>(null)
let cameraControls: any = null
let cameraSession = 0

function playScannerBeep() {
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
    // audio context restrictions
  }
}

async function openBarcodeCamera() {
  if (cameraOpen.value) return
  cameraError.value = ''
  cameraOpen.value = true
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
        closeBarcodeCamera()
        if (code) {
          form.value.barcode = code
          playScannerBeep()
          if (/^\d{13}$/.test(code)) barcodeType.value = 'EAN-13'
          else if (/^\d{12}$/.test(code)) barcodeType.value = 'UPC-A'
          else if (/^\d{8}$/.test(code)) barcodeType.value = 'EAN-8'
          else if (/^\d{14}$/.test(code)) barcodeType.value = 'ITF-14'
          else barcodeType.value = 'Code-128'
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
  const stream = cameraVideo.value?.srcObject
  if (stream instanceof MediaStream) stream.getTracks().forEach(track => track.stop())
  if (cameraVideo.value) cameraVideo.value.srcObject = null
}

let lastKeyTime = 0
let barcodeBuffer = ''

function handleGlobalKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && cameraOpen.value) {
    closeBarcodeCamera()
    return
  }

  const activeTag = document.activeElement?.tagName
  if (activeTag === 'TEXTAREA' || activeTag === 'SELECT') return
  // If active on a different text input, allow normal typing unless it's a fast scanner burst
  if (activeTag === 'INPUT' && document.activeElement !== barcodeInputRef.value) {
    // Only capture if fast burst
  }

  const now = Date.now()
  // 100ms inter-character tolerance for Bluetooth HID and Android handheld scanner services (Sunmi/Zebra)
  if (now - lastKeyTime > 100) {
    barcodeBuffer = ''
  }

  if (event.key === 'Enter') {
    if (barcodeBuffer.length >= 3) {
      event.preventDefault()
      const code = barcodeBuffer.trim()
      barcodeBuffer = ''

      playScannerBeep()

      if (showForm.value) {
        form.value.barcode = code
        if (/^\d{13}$/.test(code)) barcodeType.value = 'EAN-13'
        else if (/^\d{12}$/.test(code)) barcodeType.value = 'UPC-A'
        else if (/^\d{8}$/.test(code)) barcodeType.value = 'EAN-8'
        else if (/^\d{14}$/.test(code)) barcodeType.value = 'ITF-14'
        else barcodeType.value = 'Code-128'

        isBarcodeScanning.value = true
        if (scanTimeout) clearTimeout(scanTimeout)
        scanTimeout = setTimeout(() => {
          isBarcodeScanning.value = false
        }, 2000)
      } else {
        search.value = code
      }
    }
  } else if (event.key.length === 1) {
    barcodeBuffer += event.key
  }
  lastKeyTime = now
}

const imageInputRef = ref<HTMLInputElement | null>(null)
const cameraInputRef = ref<HTMLInputElement | null>(null)
const showPhotoPicker = ref(false)

function triggerGalleryUpload() {
  showPhotoPicker.value = false
  imageInputRef.value?.click()
}

function triggerCameraCapture() {
  showPhotoPicker.value = false
  cameraInputRef.value?.click()
}

function openPhotoPicker() {
  if (isMobileOrTablet.value) {
    showPhotoPicker.value = true
  } else {
    triggerGalleryUpload()
  }
}

function handleImageUpload(event: Event) {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) return

  const reader = new FileReader()
  reader.onload = (e) => {
    const dataUrl = e.target?.result as string
    if (!dataUrl) return
    const img = new Image()
    img.onload = () => {
      const canvas = document.createElement('canvas')
      const maxSize = 500
      let width = img.width
      let height = img.height
      if (width > height) {
        if (width > maxSize) {
          height *= maxSize / width
          width = maxSize
        }
      } else {
        if (height > maxSize) {
          width *= maxSize / height
          height = maxSize
        }
      }
      canvas.width = width
      canvas.height = height
      const ctx = canvas.getContext('2d')
      ctx?.drawImage(img, 0, 0, width, height)
      form.value.image = canvas.toDataURL('image/jpeg', 0.82)
      if (imageInputRef.value) imageInputRef.value.value = ''
      if (cameraInputRef.value) cameraInputRef.value.value = ''
    }
    img.src = dataUrl
  }
  reader.readAsDataURL(file)
}

function removeProductImage() {
  form.value.image = ''
  if (imageInputRef.value) imageInputRef.value.value = ''
  if (cameraInputRef.value) cameraInputRef.value.value = ''
}

const adjustingProduct = ref<Product | null>(null)
const adjustmentMode = ref<'add' | 'remove' | 'set'>('add')
const adjustmentQuantity = ref(1)
const adjustmentReason = ref('Stock count adjustment')

const showCategoryModal = ref(false)
const newCategoryName = ref('')
const editingCategory = ref<string | null>(null)
const editedCategoryName = ref('')

const productToDelete = ref<Product | null>(null)
const deleting = ref(false)

const canManage = computed(() => props.role === 'owner' || props.role === 'manager')

const categories = computed(() => {
  const names = new Set(
    [
      ...categoryNames.value,
      ...products.value.map(product => product.category.trim()),
    ].filter(Boolean),
  )
  return Array.from(names).sort((first, second) => first.localeCompare(second))
})

const filteredProducts = computed(() => {
  const query = search.value.trim().toLowerCase()
  return products.value.filter(product => {
    const matchesQuery = !query
      || product.name.toLowerCase().includes(query)
      || product.sku.toLowerCase().includes(query)
      || product.category.toLowerCase().includes(query)
      || product.barcode.toLowerCase().includes(query)
    const productCategory = product.category.trim() || 'General'
    const matchesCategory = selectedCategory.value === 'All Categories' || productCategory === selectedCategory.value
    const matchesStock = selectedStock.value === 'All Stock'
      || (selectedStock.value === 'Out of Stock' && product.quantity <= 0)
      || (selectedStock.value === 'Low Stock' && product.quantity > 0 && product.quantity <= product.minStock)
      || (selectedStock.value === 'In Stock' && product.quantity > product.minStock)
    return matchesQuery && matchesCategory && matchesStock
  })
})

const productsGridRef = ref<HTMLElement | null>(null)
const productsGridColumns = ref(3)
const productsGridRowHeight = computed(() => isPhone.value ? 240 : 264)

function updateProductsGridColumns() {
  if (isPhone.value) {
    productsGridColumns.value = 2
  } else if (isMobileOrTablet.value) {
    productsGridColumns.value = window.innerWidth >= 768 ? 3 : 2
  } else {
    productsGridColumns.value = 3
  }
}

const productsGridRowCount = computed(() =>
  Math.ceil(filteredProducts.value.length / productsGridColumns.value),
)

const productsGridVirtualizer = useVirtualizer(computed(() => ({
  count: productsGridRowCount.value,
  getScrollElement: () => productsGridRef.value,
  estimateSize: () => productsGridRowHeight.value,
  overscan: 3,
})))

const virtualProductRows = computed(() => productsGridVirtualizer.value.getVirtualItems())

function productsGridProducts(rowIndex: number) {
  const start = rowIndex * productsGridColumns.value
  return filteredProducts.value.slice(start, start + productsGridColumns.value)
}

const productsListRef = ref<HTMLElement | null>(null)
const productsListVirtualizer = useVirtualizer(computed(() => ({
  count: filteredProducts.value.length,
  getScrollElement: () => productsListRef.value,
  estimateSize: () => 38,
  overscan: 8,
})))
const virtualProductListItems = computed(() => productsListVirtualizer.value.getVirtualItems())

watch(
  () => [search.value, selectedCategory.value, selectedStock.value, viewMode.value, filteredProducts.value.length],
  () => {
    productsGridRef.value?.scrollTo({ top: 0 })
    productsListRef.value?.scrollTo({ top: 0 })
  },
)

const totalProducts = computed(() => filteredProducts.value.length)
const totalUnits = computed(() => filteredProducts.value.reduce((sum, product) => sum + product.quantity, 0))

const adjustmentDelta = computed(() => {
  const target = adjustingProduct.value
  const quantity = Number(adjustmentQuantity.value)
  if (!target || !Number.isFinite(quantity)) return 0
  if (adjustmentMode.value === 'add') return quantity
  if (adjustmentMode.value === 'remove') return -quantity
  return Math.round((quantity - target.quantity) * 1000) / 1000
})

const resultingStock = computed(() => {
  const target = adjustingProduct.value
  if (!target) return 0
  return Math.round((target.quantity + adjustmentDelta.value) * 1000) / 1000
})

function generateRandomSku() {
  const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789'
  let suffix = ''
  for (let i = 0; i < 6; i++) {
    suffix += chars.charAt(Math.floor(Math.random() * chars.length))
  }
  return `SKU-${suffix}`
}

function blankProduct(defaultCategory = ''): ProductInput {
  return {
    sku: generateRandomSku(),
    name: '',
    category: defaultCategory,
    barcode: '',
    unit: 'pcs',
    sellingPrice: 0,
    costPrice: 0,
    quantity: 0,
    minStock: 5,
    image: '',
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

function profitPerUnit(product: Product) {
  return product.sellingPrice - product.costPrice
}

function toggleDropdown() {
  isDropdownOpen.value = !isDropdownOpen.value
}

function closeDropdown(event: MouseEvent) {
  const target = event.target as HTMLElement
  if (!target.closest('.action-dropdown-container')) {
    isDropdownOpen.value = false
  }
}

async function loadProducts() {
  loading.value = true
  error.value = ''
  try {
    const [productResponse, categoryResponse] = await Promise.all([
      apiRequest<{ products: Product[] }>('/products'),
      apiRequest<{ categories: string[] }>('/categories'),
    ])
    products.value = productResponse.products
    categoryNames.value = categoryResponse.categories
    checkTargetStockProduct()
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Products could not be loaded.'
  } finally {
    loading.value = false
  }
}

function openCreateForm() {
  isDropdownOpen.value = false
  error.value = ''
  notice.value = ''
  if (categories.value.length === 0) {
    showCategoryModal.value = true
    return
  }
  editingProductId.value = null
  editingOriginalQuantity.value = 0
  barcodeType.value = 'EAN-13'
  form.value = blankProduct(categories.value[0] ?? 'General')
  showForm.value = true
}

function openEditForm(product: Product) {
  editingProductId.value = product.id
  editingOriginalQuantity.value = product.quantity
  barcodeType.value = 'EAN-13'
  form.value = {
    sku: product.sku || generateRandomSku(),
    name: product.name,
    category: product.category || 'General',
    barcode: product.barcode,
    unit: product.unit,
    sellingPrice: product.sellingPrice,
    costPrice: product.costPrice,
    quantity: product.quantity,
    minStock: product.minStock,
    image: product.image || '',
  }
  error.value = ''
  notice.value = ''
  showForm.value = true
}

function closeForm() {
  showForm.value = false
  editingProductId.value = null
  isBarcodeScanning.value = false
}

async function saveProduct() {
  saving.value = true
  error.value = ''
  notice.value = ''
  try {
    const payload: ProductInput = {
      ...form.value,
      sku: form.value.sku.trim() || generateRandomSku(),
      category: form.value.category.trim() || 'General',
      unit: form.value.unit.trim() || 'pcs',
      sellingPrice: Number(form.value.sellingPrice),
      costPrice: Number(form.value.costPrice),
      quantity: Number(form.value.quantity),
      minStock: Number(form.value.minStock),
    }
    if (editingProductId.value === null) {
      await apiRequest<{ product: Product }>('/products', {
        method: 'POST',
        body: payload,
      })
      notice.value = 'Product added to the catalog.'
    } else {
      await apiRequest<{ product: Product }>(`/products/${editingProductId.value}`, {
        method: 'PUT',
        body: payload,
      })
      const stockDelta = Math.round((payload.quantity - editingOriginalQuantity.value) * 1000) / 1000
      if (stockDelta !== 0) {
        await apiRequest<{ product: Product }>(`/products/${editingProductId.value}/stock-adjustments`, {
          method: 'POST',
          body: {
            delta: stockDelta,
            reason: 'Product edit stock update',
          },
        })
      }
      notice.value = 'Product updated.'
    }
    closeForm()
    await loadProducts()
    emit('stock-adjusted')
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Product could not be saved.'
  } finally {
    saving.value = false
  }
}

function openAdjustStock(product: Product) {
  adjustingProduct.value = product
  adjustmentMode.value = 'add'
  adjustmentQuantity.value = 1
  adjustmentReason.value = 'Stock count adjustment'
  error.value = ''
  notice.value = ''
}

function closeAdjustStock() {
  adjustingProduct.value = null
}

function checkTargetStockProduct() {
  if (props.targetStockProductId == null || products.value.length === 0) return
  const target = products.value.find(p => p.id === props.targetStockProductId)
  if (target) {
    selectedCategory.value = 'All Categories'
    selectedStock.value = 'All Stock'
    search.value = target.name
    openAdjustStock(target)
  }
  emit('clear-target-stock-product')
}

watch(
  () => props.targetStockProductId,
  (newId) => {
    if (newId != null) {
      checkTargetStockProduct()
    }
  },
)

async function submitStockAdjustment() {
  if (!adjustingProduct.value) return
  if (adjustmentDelta.value === 0) {
    error.value = 'Choose a stock change greater than zero.'
    return
  }
  if (resultingStock.value < 0) {
    error.value = 'Stock adjustment cannot leave a negative quantity.'
    return
  }

  adjusting.value = true
  error.value = ''
  notice.value = ''
  try {
    await apiRequest<{ product: Product }>(`/products/${adjustingProduct.value.id}/stock-adjustments`, {
      method: 'POST',
      body: {
        delta: adjustmentDelta.value,
        reason: adjustmentReason.value.trim() || 'Stock count adjustment',
      },
    })
    const adjustedName = adjustingProduct.value.name
    closeAdjustStock()
    notice.value = `Stock updated for ${adjustedName}.`
    await loadProducts()
    emit('stock-adjusted')
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Stock could not be adjusted.'
  } finally {
    adjusting.value = false
  }
}

function openCategoryManagement() {
  isDropdownOpen.value = false
  showCategoryModal.value = true
  newCategoryName.value = ''
  editingCategory.value = null
  editedCategoryName.value = ''
  error.value = ''
  notice.value = ''
}

function closeCategoryManagement() {
  showCategoryModal.value = false
  editingCategory.value = null
  editedCategoryName.value = ''
}

async function createCategory() {
  const name = newCategoryName.value.trim()
  if (!name) return
  categoryBusy.value = true
  error.value = ''
  notice.value = ''
  try {
    await apiRequest('/categories', {
      method: 'POST',
      body: { name },
    })
    newCategoryName.value = ''
    notice.value = `Category "${name}" created.`
    await loadProducts()
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Category could not be created.'
  } finally {
    categoryBusy.value = false
  }
}

function startCategoryEdit(category: string) {
  editingCategory.value = category
  editedCategoryName.value = category
}

async function saveCategoryEdit(oldName: string) {
  const name = editedCategoryName.value.trim()
  if (!name || name === oldName) {
    editingCategory.value = null
    return
  }
  categoryBusy.value = true
  error.value = ''
  notice.value = ''
  try {
    await apiRequest('/categories', {
      method: 'PUT',
      body: { oldName, name },
    })
    if (selectedCategory.value === oldName) selectedCategory.value = name
    editingCategory.value = null
    notice.value = `Category renamed to "${name}".`
    await loadProducts()
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Category could not be renamed.'
  } finally {
    categoryBusy.value = false
  }
}

async function removeCategory(category: string) {
  categoryBusy.value = true
  error.value = ''
  notice.value = ''
  try {
    await apiRequest('/categories', {
      method: 'DELETE',
      body: { name: category },
    })
    if (selectedCategory.value === category) selectedCategory.value = 'All Categories'
    notice.value = `Category "${category}" deleted.`
    await loadProducts()
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Category could not be deleted.'
  } finally {
    categoryBusy.value = false
  }
}

function promptDeleteProduct(product: Product) {
  productToDelete.value = product
  error.value = ''
  notice.value = ''
}

async function confirmDeleteProduct() {
  if (!productToDelete.value) return
  deleting.value = true
  error.value = ''
  notice.value = ''
  try {
    const name = productToDelete.value.name
    await apiRequest<void>(`/products/${productToDelete.value.id}/archive`, { method: 'POST' })
    productToDelete.value = null
    notice.value = `"${name}" removed from active products.`
    await loadProducts()
  } catch (reason) {
    error.value = reason instanceof Error ? reason.message : 'Product could not be removed.'
  } finally {
    deleting.value = false
  }
}

onMounted(() => {
  cameraSupported.value = typeof navigator !== 'undefined' && (!!navigator.mediaDevices?.getUserMedia || isMobileOrTablet.value)
  void loadProducts()
  updateProductsGridColumns()
  window.addEventListener('click', closeDropdown)
  window.addEventListener('resize', updateProductsGridColumns)
  window.addEventListener('keydown', handleGlobalKeydown)
})
onUnmounted(() => {
  closeBarcodeCamera()
  window.removeEventListener('click', closeDropdown)
  window.removeEventListener('resize', updateProductsGridColumns)
  window.removeEventListener('keydown', handleGlobalKeydown)
})
</script>

<template>
  <section class="view-section products-view-section">
    <div v-if="error && !showForm && !adjustingProduct && !showCategoryModal" class="inline-error" role="alert">{{ error }}</div>
    <div v-if="notice && !isPhone" class="inline-success" role="status">{{ notice }}</div>

    <!-- Google Account Connection Gate (when not connected) -->
    <div v-if="false" class="google-required-gate">
      <div class="gate-hero-card">
        <div class="gate-header">
          <div class="gate-icon-badge">
            <svg viewBox="0 0 24 24" width="36" height="36">
              <path fill="#4285F4" d="M23.745 12.27c0-.7-.06-1.4-.19-2.07H12v4.51h6.6c-.29 1.52-1.14 2.82-2.4 3.68v3.05h3.88c2.27-2.09 3.665-5.17 3.665-9.17z"/>
              <path fill="#34A853" d="M12 24c3.24 0 5.95-1.08 7.93-2.91l-3.88-3.05c-1.08.72-2.45 1.16-4.05 1.16-3.12 0-5.77-2.1-6.72-4.93H1.25v3.15C3.26 21.36 7.33 24 12 24z"/>
              <path fill="#FBBC05" d="M5.28 14.27c-.25-.72-.38-1.49-.38-2.27s.13-1.55.38-2.27V6.58H1.25C.45 8.18 0 9.99 0 12s.45 3.82 1.25 5.42l4.03-3.15z"/>
              <path fill="#EA4335" d="M12 4.75c1.77 0 3.35.61 4.6 1.8l3.42-3.42C17.95 1.19 15.24 0 12 0 7.33 0 3.26 2.64 1.25 6.58l4.03 3.15c.95-2.83 3.6-4.98 6.72-4.98z"/>
            </svg>
            <div class="gate-lock-pill">
              <Lock :size="12" />
              <span>GOOGLE SIGN-IN REQUIRED</span>
            </div>
          </div>
          <div class="gate-title-block">
            <h2 class="gate-title">Google Account Required</h2>
            <p class="gate-subtitle">
              Sign in with Google to create or open your shop before adding products. Your shop data is stored by CM RetailOS Online and kept separate from other shops.
            </p>
          </div>
        </div>

        <!-- Official Verified Google OAuth Connection Card -->
        <div class="gate-options-grid single-opt">
          <div class="gate-option-card fast-card">
            <div class="gate-badge">VERIFIED OAUTH 2.0</div>
            <div class="gate-card-icon-wrap">
              <svg viewBox="0 0 24 24" width="32" height="32">
                <path fill="#4285F4" d="M23.745 12.27c0-.7-.06-1.4-.19-2.07H12v4.51h6.6c-.29 1.52-1.14 2.82-2.4 3.68v3.05h3.88c2.27-2.09 3.665-5.17 3.665-9.17z"/>
                <path fill="#34A853" d="M12 24c3.24 0 5.95-1.08 7.93-2.91l-3.88-3.05c-1.08.72-2.45 1.16-4.05 1.16-3.12 0-5.77-2.1-6.72-4.93H1.25v3.15C3.26 21.36 7.33 24 12 24z"/>
                <path fill="#FBBC05" d="M5.28 14.27c-.25-.72-.38-1.49-.38-2.27s.13-1.55.38-2.27V6.58H1.25C.45 8.18 0 9.99 0 12s.45 3.82 1.25 5.42l4.03-3.15z"/>
                <path fill="#EA4335" d="M12 4.75c1.77 0 3.35.61 4.6 1.8l3.42-3.42C17.95 1.19 15.24 0 12 0 7.33 0 3.26 2.64 1.25 6.58l4.03 3.15c.95-2.83 3.6-4.98 6.72-4.98z"/>
              </svg>
            </div>
            <h4 class="gate-opt-title">Sign In with Your Google Account</h4>
            <p class="gate-opt-desc">
              Google verifies your identity and creates your private shop workspace. Google is used for sign-in; it does not store or synchronize your product data.
            </p>
            <button
              type="button"
              class="btn gate-google-btn"
              @click="$emit('navigate-settings')"
            >
              <svg viewBox="0 0 24 24" width="20" height="20" style="margin-right: 12px;">
                <path fill="#4285F4" d="M23.745 12.27c0-.7-.06-1.4-.19-2.07H12v4.51h6.6c-.29 1.52-1.14 2.82-2.4 3.68v3.05h3.88c2.27-2.09 3.665-5.17 3.665-9.17z"/>
                <path fill="#34A853" d="M12 24c3.24 0 5.95-1.08 7.93-2.91l-3.88-3.05c-1.08.72-2.45 1.16-4.05 1.16-3.12 0-5.77-2.1-6.72-4.93H1.25v3.15C3.26 21.36 7.33 24 12 24z"/>
                <path fill="#FBBC05" d="M5.28 14.27c-.25-.72-.38-1.49-.38-2.27s.13-1.55.38-2.27V6.58H1.25C.45 8.18 0 9.99 0 12s.45 3.82 1.25 5.42l4.03-3.15z"/>
                <path fill="#EA4335" d="M12 4.75c1.77 0 3.35.61 4.6 1.8l3.42-3.42C17.95 1.19 15.24 0 12 0 7.33 0 3.26 2.64 1.25 6.58l4.03 3.15c.95-2.83 3.6-4.98 6.72-4.98z"/>
              </svg>
              <span>Sign in with Google</span>
            </button>
          </div>
        </div>

        <div class="gate-footer">
          <ShieldCheck :size="16" class="gate-footer-icon" />
          <span>Shop data is saved automatically by the online service.</span>
          <button type="button" class="gate-link-btn" @click="$emit('navigate-settings')">
            Open Settings <ArrowRight :size="14" />
          </button>
        </div>
      </div>
    </div>

    <!-- Normal Products View when Google is Connected -->
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
              aria-label="Search products by name or category"
            />
          </div>

          <select v-model="selectedCategory" class="filter-select" aria-label="Filter by category">
            <option value="All Categories">All Categories</option>
            <option v-for="category in categories" :key="category" :value="category">{{ category }}</option>
          </select>

          <select v-model="selectedStock" class="filter-select" aria-label="Filter by stock status">
            <option value="All Stock">All Stock</option>
            <option value="In Stock">In Stock</option>
            <option value="Low Stock">Low Stock</option>
            <option value="Out of Stock">Out of Stock</option>
          </select>

          <div v-if="canManage" class="dropdown-container action-dropdown-container">
            <button type="button" class="add-btn action-dropdown-btn" @click.stop="toggleDropdown">
              <SlidersHorizontal :size="16" />
              <span>Manage</span>
              <ChevronDown :size="16" style="margin-left: -4px" />
            </button>
            <div v-if="isDropdownOpen" class="dropdown-menu action-dropdown-menu">
              <button type="button" class="dropdown-item add-item" @click="openCreateForm">
                <Plus :size="16" />
                <span>Add Product</span>
              </button>
              <button type="button" class="dropdown-item" @click="openCategoryManagement">
                <Folders :size="16" />
                <span>Categories</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      <div v-if="isMobileOrTablet" ref="productsGridRef" class="products-grid">
        <div v-if="showSkeleton" class="products-skeleton-grid">
          <div v-for="i in 6" :key="i" class="product-grid-card">
            <div style="display: flex; justify-content: space-between; margin-bottom: 12px;">
              <div style="display: flex; flex-direction: column; gap: 6px; width: 65%;">
                <div class="skeleton-box skeleton-shimmer" style="height: 16px; width: 80%;"></div>
                <div class="skeleton-box skeleton-shimmer" style="height: 12px; width: 50%;"></div>
              </div>
              <div class="skeleton-box skeleton-shimmer" style="height: 20px; width: 64px; border-radius: 999px;"></div>
            </div>
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-bottom: 12px;">
              <div v-for="j in 4" :key="j" class="skeleton-box skeleton-shimmer" style="height: 28px; border-radius: 6px;"></div>
            </div>
            <div style="display: flex; gap: 8px;">
              <div class="skeleton-box skeleton-shimmer" style="flex: 1; height: 32px; border-radius: 6px;"></div>
              <div class="skeleton-box skeleton-shimmer" style="flex: 1; height: 32px; border-radius: 6px;"></div>
            </div>
          </div>
        </div>
        <p v-else-if="loading && !showSkeleton" class="products-grid-empty"></p>
        <p v-else-if="filteredProducts.length === 0" class="products-grid-empty">No products found.</p>
        <div
          v-else
          class="virtual-grid-spacer"
          :style="{ height: `${productsGridVirtualizer.getTotalSize()}px` }"
        >
          <div
            v-for="virtualRow in virtualProductRows"
            :key="String(virtualRow.key)"
            class="virtual-grid-row products-virtual-row"
            :style="{
              transform: `translateY(${virtualRow.start}px)`,
              height: isPhone ? '226px' : '248px',
              gridTemplateColumns: `repeat(${productsGridColumns}, minmax(0, 1fr))`,
            }"
          >
            <article
              v-for="product in productsGridProducts(virtualRow.index)"
              :key="product.id"
              class="product-grid-card"
            >
              <div class="product-grid-header">
                <div class="product-grid-name">
                  <h3>{{ product.name }}</h3>
                  <span>{{ product.category || 'General' }}</span>
                </div>
                <span class="stock-badge" :class="getStockStatusClass(product)">
                  {{ getStockStatusText(product) }}
                </span>
              </div>
              <dl class="product-grid-details">
                <div><dt>Selling price</dt><dd>{{ formatMoney(product.sellingPrice, currency) }}</dd></div>
                <div><dt>Cost price</dt><dd>{{ formatMoney(product.costPrice, currency) }}</dd></div>
                <div><dt>Quantity</dt><dd>{{ formatQuantity(product.quantity) }} {{ product.unit }}</dd></div>
                <div>
                  <dt>Profit per unit</dt>
                  <dd :class="profitPerUnit(product) >= 0 ? 'profit-positive' : 'profit-negative'">
                    {{ formatMoney(profitPerUnit(product), currency) }}
                  </dd>
                </div>
              </dl>
              <div v-if="canManage" class="product-grid-actions" :aria-label="`${product.name} actions`">
                <button type="button" @click="openEditForm(product)"><Edit :size="14" /> <span>Edit</span></button>
                <button type="button" @click="openAdjustStock(product)"><SlidersHorizontal :size="14" /> <span>Adjust</span></button>
                <button type="button" class="delete" @click="promptDeleteProduct(product)"><Trash2 :size="14" /> <span>Delete</span></button>
              </div>
            </article>
          </div>
        </div>
      </div>

      <div v-else ref="productsListRef" class="table-container virtual-list-container">
        <div class="virtual-list-header products-list-header" :class="{ 'with-actions': canManage }">
          <span>Product Name</span><span>Category</span><span>Cost Price</span><span>Selling Price</span>
          <span>Quantity</span><span>Unit</span><span>Profit Per Unit</span>
          <span v-if="canManage">Actions</span>
        </div>
        <div v-if="showSkeleton" class="products-skeleton-list">
          <div v-for="i in 8" :key="i" class="products-skeleton-row">
            <div class="skeleton-box skeleton-shimmer" style="width: 140px; height: 18px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 80px; height: 18px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 75px; height: 18px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 75px; height: 18px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 50px; height: 18px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 40px; height: 18px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 75px; height: 18px;"></div>
            <div class="skeleton-box skeleton-shimmer" style="width: 110px; height: 26px; border-radius: 6px;"></div>
          </div>
        </div>
        <p v-else-if="loading && !showSkeleton" class="products-grid-empty"></p>
        <p v-else-if="filteredProducts.length === 0" class="products-grid-empty">No products found.</p>
        <div
          v-else
          class="virtual-list-spacer"
          :style="{ height: `${productsListVirtualizer.getTotalSize()}px` }"
        >
          <div
            v-for="virtualRow in virtualProductListItems"
            :key="String(virtualRow.key)"
            class="virtual-list-row products-list-header"
            :class="{ 'with-actions': canManage }"
            :style="{ transform: `translateY(${virtualRow.start}px)`, height: `${virtualRow.size}px` }"
          >
            <template v-if="filteredProducts[virtualRow.index]">
              <span class="font-medium text-slate-200">{{ filteredProducts[virtualRow.index].name }}</span>
              <span>{{ filteredProducts[virtualRow.index].category || 'General' }}</span>
              <span>{{ formatMoney(filteredProducts[virtualRow.index].costPrice, currency) }}</span>
              <span>{{ formatMoney(filteredProducts[virtualRow.index].sellingPrice, currency) }}</span>
              <span>
                <span class="stock-badge" :class="getStockStatusClass(filteredProducts[virtualRow.index])">
                  {{ getStockStatusText(filteredProducts[virtualRow.index]) }}
                </span>
              </span>
              <span>{{ filteredProducts[virtualRow.index].unit }}</span>
              <span :class="profitPerUnit(filteredProducts[virtualRow.index]) >= 0 ? 'profit-positive' : 'profit-negative'">
                {{ formatMoney(profitPerUnit(filteredProducts[virtualRow.index]), currency) }}
              </span>
              <span v-if="canManage" class="actions">
                <button type="button" class="action-btn" title="Edit Product" @click="openEditForm(filteredProducts[virtualRow.index])"><Edit :size="16" /></button>
                <button type="button" class="action-btn" title="Adjust Stock" @click="openAdjustStock(filteredProducts[virtualRow.index])"><SlidersHorizontal :size="16" /></button>
                <button type="button" class="action-btn delete" title="Delete Product" @click="promptDeleteProduct(filteredProducts[virtualRow.index])"><Trash2 :size="16" /></button>
              </span>
            </template>
          </div>
        </div>
      </div>
    </div>

    <!-- Add / Edit Product Modal -->
    <Teleport to="body">
      <div v-if="showForm" class="modal-backdrop product-modal-backdrop" @click.self="closeForm">
        <div class="modal-content product-editor-modal">
        <div class="modal-header">
          <div class="modal-title-group">
            <h3>{{ editingProductId === null ? 'Add New Product' : 'Edit Product' }}</h3>
            <p class="modal-subtitle">Configure product identity, pricing margins, and stock thresholds.</p>
          </div>
          <button type="button" class="close-btn" aria-label="Close" @click="closeForm">
            <X :size="19" />
          </button>
        </div>

        <form class="product-form-container" @submit.prevent="saveProduct">
          <div v-if="error" class="inline-error" role="alert" style="margin-bottom: 1rem;">{{ error }}</div>

          <div class="product-hero-card">
            <input ref="imageInputRef" type="file" accept="image/*" style="display: none" @change="handleImageUpload" />
            <input ref="cameraInputRef" type="file" accept="image/*" capture="environment" style="display: none" @change="handleImageUpload" />
            <div class="image-upload-wrapper">
              <button
                type="button"
                class="image-upload-square image-upload-frame"
                :class="{ 'has-image': !!form.image }"
                :title="form.image ? 'Change product photo' : 'Upload or take product photo'"
                @click="openPhotoPicker"
              >
                <template v-if="form.image">
                  <img :src="form.image" alt="Product preview" class="uploaded-preview" />
                  <div class="photo-change-overlay">
                    <Camera :size="13" />
                    <span>Change</span>
                  </div>
                </template>
                <div v-else class="upload-placeholder">
                  <div class="upload-icon-circle">
                    <ImagePlus :size="22" class="upload-icon" />
                  </div>
                  <span class="upload-hint">Product Photo</span>
                  <span class="upload-subhint">Tap to take photo or choose</span>
                </div>
              </button>
              <button
                v-if="form.image"
                type="button"
                class="remove-image-btn"
                title="Remove photo"
                aria-label="Remove photo"
                @click.stop="removeProductImage"
              >
                <Trash2 :size="13" />
              </button>
            </div>

            <!-- Quick 1-tap buttons for Camera & Gallery -->
            <div class="photo-quick-actions">
              <button
                type="button"
                class="photo-action-chip camera-chip"
                title="Take photo with camera"
                @click="triggerCameraCapture"
              >
                <Camera :size="14" />
                <span>Take Photo</span>
              </button>
              <button
                type="button"
                class="photo-action-chip gallery-chip"
                title="Choose photo from gallery"
                @click="triggerGalleryUpload"
              >
                <ImagePlus :size="14" />
                <span>Gallery</span>
              </button>
            </div>

            <div class="photo-hero-tip">
              <span>{{ form.image ? 'High quality photos boost POS and sales visibility' : 'Take a photo with your camera or select from your gallery' }}</span>
            </div>
          </div>

          <!-- Photo Source Picker Bottom Sheet (Camera vs Gallery) -->
          <div v-if="showPhotoPicker" class="photo-picker-backdrop" @click.self="showPhotoPicker = false">
            <div class="photo-picker-sheet" role="dialog" aria-modal="true" aria-label="Choose photo source">
              <div class="photo-picker-header">
                <h3>Product Photo</h3>
                <button type="button" class="photo-picker-close" @click="showPhotoPicker = false"><X :size="18" /></button>
              </div>
              <div class="photo-picker-options">
                <button type="button" class="photo-picker-option" @click="triggerCameraCapture">
                  <div class="option-icon camera-icon-wrap"><Camera :size="22" /></div>
                  <div class="option-details">
                    <strong>Take Photo with Camera</strong>
                    <small>Snap a photo directly using your phone's camera</small>
                  </div>
                </button>
                <button type="button" class="photo-picker-option" @click="triggerGalleryUpload">
                  <div class="option-icon gallery-icon-wrap"><ImagePlus :size="22" /></div>
                  <div class="option-details">
                    <strong>Choose from Gallery</strong>
                    <small>Select an existing image from your photo library</small>
                  </div>
                </button>
              </div>
            </div>
          </div>

          <div class="form-grid">
            <div class="form-group span-2">
              <label>Product Name <span class="required-star">*</span></label>
              <input v-model.trim="form.name" type="text" placeholder="e.g. Air Freshener Lavender 300ml" required class="input-highlight" />
            </div>

            <div class="form-group">
              <label>SKU / Item Code <span class="required-star">*</span></label>
              <input v-model.trim="form.sku" type="text" placeholder="SKU-XXXXXX" required />
            </div>

            <div class="form-group">
              <label>Category <span class="required-star">*</span></label>
              <select v-model="form.category" required class="modal-select">
                <option v-for="category in categories" :key="category" :value="category">{{ category }}</option>
              </select>
            </div>

            <div class="form-group span-2">
              <label>Barcode / Scanner Code</label>
              <div class="barcode-row-inputs">
                <div class="input-with-scan-btn" :class="{ 'scanning-active': isBarcodeScanning }">
                  <div class="scan-input-wrapper">
                    <input
                      ref="barcodeInputRef"
                      v-model.trim="form.barcode"
                      type="text"
                      placeholder="Scan barcode or type code..."
                      @focus="isBarcodeScanning = true"
                      @blur="isBarcodeScanning = false"
                      @keyup.enter="handleBarcodeScanned"
                    />
                    <div class="barcode-action-btns">
                      <button
                        v-if="cameraSupported"
                        type="button"
                        class="scan-inside-btn camera-scan-btn"
                        title="Scan barcode with phone camera"
                        aria-label="Scan barcode with camera"
                        @click="openBarcodeCamera"
                      >
                        <Camera :size="16" />
                      </button>
                      <button
                        type="button"
                        class="scan-inside-btn hardware-scan-btn"
                        :class="{ active: isBarcodeScanning }"
                        title="Hardware & Bluetooth scanner ready • Sunmi, Zebra, Honeywell supported"
                        aria-label="Hardware barcode scanner"
                        @click="triggerBarcodeScan"
                      >
                        <Barcode :size="16" />
                      </button>
                    </div>
                  </div>
                  <div v-if="isBarcodeScanning" class="scan-laser-indicator">
                    <span class="laser-dot" />
                    <span>Hardware / Bluetooth Scanner ready • scan now...</span>
                  </div>
                </div>
                <select v-model="barcodeType" class="modal-select barcode-type-select" aria-label="Barcode type">
                  <option v-for="type in barcodeTypes" :key="type" :value="type">{{ type }}</option>
                </select>
              </div>
            </div>

            <!-- Pricing & Profit Margin Card -->
            <div class="form-section span-2 pricing-margin-section">
              <div class="section-header-row">
                <div class="section-title-wrap">
                  <h4 class="section-title">Pricing Margin</h4>
                </div>
              </div>
              <div class="section-grid-2">
                <div class="form-group">
                  <label>Cost Price</label>
                  <div class="input-with-prefix">
                    <span class="prefix">{{ currency }}</span>
                    <input v-model.number="form.costPrice" type="number" min="0" step="0.01" required />
                  </div>
                </div>
                <div class="form-group">
                  <label>Selling Price</label>
                  <div class="input-with-prefix">
                    <span class="prefix">{{ currency }}</span>
                    <input v-model.number="form.sellingPrice" type="number" min="0" step="0.01" required />
                  </div>
                </div>
              </div>
              <div v-if="form.costPrice > 0 && form.sellingPrice > 0" class="pricing-metrics-microbar">
                <span>Profit per unit: <strong :class="formProfit >= 0 ? 'text-emerald' : 'text-rose'">{{ formatMoney(formProfit, currency) }}</strong></span>
              </div>
            </div>

            <!-- Stock Levels & Unit Card -->
            <div class="form-section span-2 stock-levels-section">
              <div class="section-header-row">
                <div class="section-title-wrap">
                  <h4 class="section-title">Stock Levels &amp; Unit</h4>
                </div>
                <div class="stock-live-pill" :class="formStockStatus.class">
                  <span class="stock-dot"></span>
                  <span>{{ formStockStatus.label }}</span>
                </div>
              </div>
              <div class="section-grid-3">
                <div class="form-group">
                  <label>{{ editingProductId === null ? 'Initial Qty' : 'Current Qty' }}</label>
                  <input v-model.number="form.quantity" type="number" min="0" step="0.001" required />
                </div>
                <div class="form-group">
                  <label>Min Stock</label>
                  <input v-model.number="form.minStock" type="number" min="0" step="0.001" required title="Alert threshold for low stock" />
                </div>
                <div class="form-group">
                  <label>Unit</label>
                  <input v-model.trim="form.unit" type="text" placeholder="e.g. piece, kg, box" required />
                </div>
              </div>
            </div>
          </div>

          <div class="modal-actions sticky-modal-actions">
            <button type="button" class="btn-secondary" @click="closeForm">Cancel</button>
            <button type="submit" class="btn-primary btn-save-product" :disabled="saving">
              <Check v-if="!saving && editingProductId !== null" :size="16" />
              {{ saving ? 'Saving…' : editingProductId === null ? 'Add Product' : isPhone ? 'Save' : 'Save Changes' }}
            </button>
          </div>
          </form>
        </div>
      </div>
    </Teleport>

    <!-- Adjust Stock Modal -->
    <Teleport to="body">
      <div v-if="adjustingProduct" class="modal-backdrop adjust-stock-backdrop" @click.self="closeAdjustStock">
        <div class="modal-content adjust-stock-modal" style="max-width: 490px;">
          <div class="modal-header adjust-stock-header">
            <div class="modal-title-with-icon">
              <div class="modal-icon-badge">
                <SlidersHorizontal :size="20" />
              </div>
              <div class="modal-title-group">
                <h3>Adjust Stock</h3>
                <p class="modal-subtitle">
                  <span class="adjust-product-name">{{ adjustingProduct.name }}</span>
                  <span class="adjust-stock-badge">Current: {{ formatQuantity(adjustingProduct.quantity) }} {{ adjustingProduct.unit }}</span>
                </p>
              </div>
            </div>
            <button type="button" class="close-btn" @click="closeAdjustStock" aria-label="Close dialog">
              <X :size="20" />
            </button>
          </div>

          <form @submit.prevent="submitStockAdjustment" class="adjust-stock-form">
            <div v-if="error" class="inline-error" role="alert" style="margin-bottom: 1rem;">{{ error }}</div>

            <div class="form-grid">
              <div class="form-group span-2">
                <label>Adjustment Type</label>
                <select v-model="adjustmentMode" class="modal-select">
                  <option value="add">Add to stock</option>
                  <option value="remove">Remove from stock</option>
                  <option value="set">Set exact stock count</option>
                </select>
              </div>

              <div class="form-group">
                <label>{{ adjustmentMode === 'set' ? 'New Stock Quantity' : 'Quantity' }}</label>
                <input v-model.number="adjustmentQuantity" type="number" min="0" step="0.001" required />
              </div>

              <div class="form-group">
                <label>Resulting Stock</label>
                <div class="resulting-stock-box">
                  <span class="resulting-stock-val">{{ formatQuantity(resultingStock) }}</span>
                  <span class="resulting-stock-unit">{{ adjustingProduct.unit }}</span>
                </div>
              </div>

              <div class="form-group span-2">
                <label>Reason</label>
                <input v-model.trim="adjustmentReason" type="text" maxlength="140" required placeholder="Stock count, restock, damaged item…" />
              </div>
            </div>

            <div class="modal-actions adjust-stock-actions">
              <button type="button" class="btn-secondary" @click="closeAdjustStock">Cancel</button>
              <button type="submit" class="btn-primary adjust-submit-btn" :disabled="adjusting || resultingStock < 0 || adjustmentDelta === 0">
                {{ adjusting ? 'Updating…' : 'Apply Adjustment' }}
              </button>
            </div>
          </form>
        </div>
      </div>
    </Teleport>

    <!-- Category Management Modal -->
    <Teleport to="body">
      <div v-if="showCategoryModal" class="modal-backdrop" @click.self="closeCategoryManagement">
        <div class="modal-content category-modal">
          <div class="modal-header">
            <div class="modal-title-group">
              <h3>Manage Categories</h3>
              <p class="modal-subtitle">Add, rename, or delete product categories.</p>
            </div>
            <button type="button" class="close-btn" @click="closeCategoryManagement">
              <X :size="20" />
            </button>
          </div>

          <div v-if="error" class="inline-error" role="alert" style="margin-bottom: 1rem;">{{ error }}</div>

          <form class="add-category-form" @submit.prevent="createCategory">
            <input v-model.trim="newCategoryName" type="text" maxlength="80" placeholder="New category name..." />
            <button type="submit" class="btn-primary" :disabled="categoryBusy || !newCategoryName.trim()">
              <Plus :size="18" /> Add
            </button>
          </form>

          <div class="category-list">
            <div v-if="!categories.length" class="empty-categories">No categories available.</div>
            <div v-for="category in categories" :key="category" class="category-item">
              <div v-if="editingCategory === category" class="edit-category-mode">
                <input
                  v-model.trim="editedCategoryName"
                  type="text"
                  maxlength="80"
                  class="edit-cat-input"
                  @keyup.enter="saveCategoryEdit(category)"
                />
                <div class="cat-actions">
                  <button type="button" class="action-btn save" title="Save" :disabled="categoryBusy" @click="saveCategoryEdit(category)">
                    <Check :size="16" />
                  </button>
                  <button type="button" class="action-btn cancel" title="Cancel" @click="editingCategory = null">
                    <X :size="16" />
                  </button>
                </div>
              </div>
              <div v-else class="view-category-mode">
                <span class="cat-name">{{ category }}</span>
                <div class="cat-actions">
                  <button type="button" class="action-btn" title="Rename" :disabled="categoryBusy" @click="startCategoryEdit(category)">
                    <Edit :size="16" />
                  </button>
                  <button type="button" class="action-btn delete" title="Delete" :disabled="categoryBusy" @click="removeCategory(category)">
                    <Trash2 :size="16" />
                  </button>
                </div>
              </div>
            </div>
          </div>

          <div class="modal-actions">
            <button type="button" class="btn-secondary" @click="closeCategoryManagement">Done</button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Delete Product Confirmation Modal -->
    <Teleport to="body">
      <div v-if="productToDelete" class="modal-backdrop" @click.self="productToDelete = null">
        <div class="modal-content delete-confirm-modal">
          <div class="delete-warning-icon">
            <AlertTriangle :size="32" />
          </div>
          <h3>Delete Product?</h3>
          <p>
            Are you sure you want to delete <strong>{{ productToDelete.name }}</strong>?
            Existing sales history for this item will be preserved.
          </p>
          <div class="delete-confirm-actions">
            <button type="button" class="btn-secondary" @click="productToDelete = null">Cancel</button>
            <button type="button" class="btn-danger" :disabled="deleting" @click="confirmDeleteProduct">
              {{ deleting ? 'Deleting…' : 'Yes, Delete' }}
            </button>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Product Camera Scanner Modal -->
    <Teleport to="body">
      <div v-if="cameraOpen" class="barcode-camera-backdrop" @click.self="closeBarcodeCamera">
        <section
          class="barcode-camera-dialog"
          role="dialog"
          aria-modal="true"
          aria-labelledby="barcode-product-camera-title"
        >
          <header class="barcode-camera-heading">
            <div>
              <h2 id="barcode-product-camera-title">Scan product barcode</h2>
              <p>Hold barcode inside frame to auto-fill barcode</p>
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
          </div>
          <p v-if="cameraError" class="barcode-camera-error" role="alert">{{ cameraError }}</p>
          <p v-else class="barcode-camera-status" role="status">Scanning with rear camera…</p>
          <button type="button" class="barcode-camera-cancel" @click="closeBarcodeCamera">Cancel scan</button>
        </section>
      </div>
    </Teleport>
  </section>
</template>

<style scoped>
/* Google Account Required Gate */
.google-required-gate {
  width: 100%;
  max-width: 960px;
  margin: 1.5rem auto 3rem;
  padding: 0 1rem;
}

.gate-hero-card {
  position: relative;
  background: var(--bg-card, rgba(15, 23, 42, 0.75));
  backdrop-filter: blur(16px);
  border: none !important;
  outline: none !important;
  border-radius: 20px;
  padding: 32px 36px;
  box-shadow: 0 12px 36px rgba(0, 0, 0, 0.25);
  display: flex;
  flex-direction: column;
  gap: 28px;
}

.gate-header {
  display: flex;
  align-items: flex-start;
  gap: 20px;
}

.gate-icon-badge {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.gate-lock-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 3px 8px;
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.06em;
  color: #f59e0b;
  background: rgba(245, 158, 11, 0.15);
  border-radius: 6px;
  border: none !important;
  outline: none !important;
}

.gate-title-block {
  flex: 1;
}

.gate-title {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-primary, #f8fafc);
  margin: 0 0 6px;
}

.gate-subtitle {
  font-size: 14px;
  color: var(--text-muted, #94a3b8);
  margin: 0;
  line-height: 1.55;
  max-width: 720px;
}

.gate-inline-error {
  padding: 12px 16px;
  border-radius: 10px;
  background: rgba(239, 68, 68, 0.15);
  color: #f87171;
  font-size: 13.5px;
  border: none !important;
  outline: none !important;
}

.gate-options-grid {
  display: grid;
  grid-template-columns: 1fr;
  max-width: 580px;
  margin: 0 auto;
  width: 100%;
}

.gate-option-card {
  position: relative;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding: 32px 28px;
  background: var(--bg-app, #020617);
  border: none !important;
  outline: none !important;
  border-radius: 16px;
  transition: all 0.2s ease;
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
}

.gate-option-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 8px 28px rgba(0, 0, 0, 0.22);
}

.gate-badge {
  position: absolute;
  top: 16px;
  right: 16px;
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.08em;
  padding: 4px 10px;
  border-radius: 6px;
  background: rgba(16, 185, 129, 0.15);
  color: #10b981;
  border: none !important;
  outline: none !important;
}

.gate-card-icon-wrap {
  width: 52px;
  height: 52px;
  border-radius: 14px;
  background: #ffffff;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 16px;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.15);
  border: none !important;
  outline: none !important;
}

.gate-opt-title {
  font-size: 18px;
  font-weight: 700;
  color: var(--text-primary, #f8fafc);
  margin: 0 0 10px;
}

.gate-opt-desc {
  font-size: 13.5px;
  color: var(--text-muted, #94a3b8);
  margin: 0 0 24px;
  line-height: 1.55;
  max-width: 480px;
}

.gate-google-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  max-width: 320px;
  padding: 13px 20px;
  background: #ffffff;
  color: #1f2937;
  font-weight: 600;
  font-size: 14px;
  border: none !important;
  outline: none !important;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s ease;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.1);
}

.gate-google-btn:hover {
  background: #f9fafb;
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.15);
  transform: translateY(-1px);
}

.gate-footer {
  display: flex;
  align-items: center;
  gap: 8px;
  padding-top: 12px;
  font-size: 13px;
  color: var(--text-muted, #94a3b8);
}

.gate-footer-icon {
  color: var(--accent-color, #0ea5e9);
  flex-shrink: 0;
}

.gate-link-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  background: transparent;
  border: none !important;
  outline: none !important;
  color: var(--accent-color, #0ea5e9);
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  padding: 0;
  transition: opacity 0.2s;
}

.gate-link-btn:hover {
  text-decoration: underline;
  opacity: 0.85;
}

@media (max-width: 900px) {
  .gate-options-grid {
    grid-template-columns: 1fr;
  }
  .gate-header {
    flex-direction: column;
    align-items: flex-start;
  }
}

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
  flex-wrap: nowrap;
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
  pointer-events: none;
  user-select: none;
}

.summary-badge {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 0.5rem;
  padding: 0.3rem 0.7rem;
  display: flex;
  align-items: baseline;
  gap: 0.4rem;
  pointer-events: none;
  user-select: none;
  cursor: default;
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

.product-grid-actions button:focus-visible {
  outline: 2px solid var(--accent-color);
  outline-offset: 2px;
}

.products-grid {
  display: block;
  flex: 1;
  min-width: 0;
  min-height: 0;
  overflow-y: auto;
  padding: 8px 12px 90px 12px !important;
  box-sizing: border-box;
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
  gap: 16px;
}

.product-grid-card {
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  gap: 0.75rem;
  min-width: 0;
  padding: 12px 14px;
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

[data-theme='light'] .product-grid-card {
  background: #ffffff !important;
  border: 1px solid #e2e8f0 !important;
  box-shadow: 0 2px 10px rgba(15, 23, 42, 0.08) !important;
}

.product-grid-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.5rem;
  min-height: 40px;
}

.product-grid-name {
  flex: 1;
  min-width: 0;
}

.product-grid-name h3 {
  margin: 0;
  color: var(--text-primary);
  font-size: 0.9rem;
  font-weight: 650;
  line-height: 1.25;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  word-break: break-word;
}

.product-grid-name > span {
  display: block;
  margin-top: 0.2rem;
  color: var(--text-muted);
  font-size: 0.75rem;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.product-grid-header .stock-badge {
  flex-shrink: 0;
  white-space: nowrap;
  font-size: 11px;
  padding: 3px 8px;
  border-radius: 6px;
}

.product-grid-details {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.5rem 0.65rem;
  margin: 0;
}

.product-grid-details > div {
  min-width: 0;
}

.product-grid-details dt {
  color: var(--text-muted);
  font-size: 0.72rem;
}

.product-grid-details dd {
  margin: 0.15rem 0 0;
  color: var(--text-primary);
  font-size: 0.85rem;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.product-grid-actions {
  display: flex;
  flex-wrap: nowrap;
  gap: 6px;
  padding-top: 8px;
  border-top: 1px solid var(--border-subtle);
}

.product-grid-actions button {
  display: inline-flex;
  flex: 1 1 0;
  min-width: 0;
  align-items: center;
  justify-content: center;
  gap: 4px;
  min-height: 36px;
  padding: 0 4px;
  border: 1px solid rgba(148, 163, 184, 0.16);
  border-radius: 8px;
  color: var(--text-secondary);
  background: var(--bg-sidebar);
  font: inherit;
  font-size: 11.5px;
  font-weight: 550;
  white-space: nowrap;
  cursor: pointer;
  overflow: hidden;
  box-sizing: border-box;
}

.product-grid-actions button:hover {
  color: var(--text-primary);
  border-color: var(--accent-color);
}

.product-grid-actions button.delete {
  color: #f87171;
}

.product-grid-actions button.delete:hover {
  background: rgba(239, 68, 68, 0.12);
  border-color: rgba(239, 68, 68, 0.35);
}

.products-grid-empty {
  grid-column: 1 / -1;
  margin: 0;
  padding: 2rem 1rem;
  color: var(--text-muted);
  text-align: center;
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

.dropdown-container,
.action-dropdown-container {
  position: relative;
}

.add-btn,
.action-dropdown-btn {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  background-color: #0ea5e9;
  color: white;
  padding: 0.55rem 1rem;
  border-radius: 10px;
  border: none !important;
  outline: none !important;
  cursor: pointer;
  font-weight: 600;
  font-size: 0.875rem;
  transition: background-color 0.15s, transform 0.1s;
}

.add-btn:hover,
.action-dropdown-btn:hover {
  background-color: #0284c7;
}

.add-btn:active,
.action-dropdown-btn:active {
  transform: scale(0.97);
}

.dropdown-menu,
.action-dropdown-menu {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  margin-top: 0;
  background-color: #141f32;
  border: none !important;
  outline: none !important;
  border-radius: 14px;
  padding: 6px;
  display: flex;
  flex-direction: column;
  gap: 3px;
  z-index: 9999;
  box-shadow: 0 20px 45px -4px rgba(0, 0, 0, 0.65), 0 0 0 1px rgba(255, 255, 255, 0.08);
  min-width: 180px;
}

.dropdown-item {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  background-color: transparent;
  border: none !important;
  outline: none !important;
  color: #e2e8f0;
  padding: 0.65rem 0.85rem;
  min-height: 44px;
  border-radius: 10px;
  font-size: 0.92rem;
  font-weight: 550;
  cursor: pointer;
  width: 100%;
  justify-content: flex-start;
  transition: background-color 0.15s, transform 0.1s;
  text-align: left;
}

.dropdown-item:hover {
  background-color: rgba(255, 255, 255, 0.08);
  color: #ffffff;
}

.dropdown-item:active {
  background-color: rgba(255, 255, 255, 0.14);
  transform: scale(0.98);
}

.dropdown-item.add-item {
  color: #38bdf8;
  font-weight: 650;
}

.dropdown-item.add-item:hover {
  background-color: rgba(14, 165, 233, 0.12);
  color: #38bdf8;
}

.dropdown-item.add-item:active {
  background-color: rgba(14, 165, 233, 0.2);
}

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
  padding: 0.4rem 0.75rem;
  border-right: 1px solid var(--border-subtle);
  border-bottom: 1px solid var(--border-subtle);
  text-align: center;
  font-size: 0.825rem;
  white-space: nowrap;
  box-sizing: border-box;
  height: 100%;
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

.virtual-list-row .actions {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.35rem;
}

.products-list-header {
  grid-template-columns: repeat(7, minmax(110px, 1fr));
  min-width: 770px;
}

.products-list-header.with-actions {
  grid-template-columns: repeat(8, minmax(110px, 1fr));
  min-width: 880px;
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
.text-center { text-align: center; }

.profit-positive { color: #4ade80; }
.profit-negative { color: #f87171; }

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

.actions {
  display: flex;
  gap: 0.4rem;
  justify-content: center;
}

.action-btn {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  padding: 0.25rem;
  border-radius: 0.25rem;
  display: inline-flex;
}

.action-btn:hover {
  color: var(--text-primary);
  background-color: var(--bg-hover);
}

.action-btn.delete:hover {
  color: #f87171;
  background-color: rgba(239, 68, 68, 0.1);
}

.action-btn.save { color: #4ade80; }
.action-btn.cancel { color: #f87171; }

.empty-message {
  text-align: center;
  padding: 2rem;
  color: var(--text-muted);
}

.empty-categories-cta {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.85rem;
  padding: 1rem;
}

/* Modal Styles */
.modal-content {
  background-color: var(--bg-sidebar);
  padding: 1.5rem;
  border-radius: 16px;
  border: 1px solid var(--border-color);
  width: 100%;
  max-width: 540px;
  max-height: 90vh;
  overflow-y: auto;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 1.25rem;
  padding-bottom: 0.85rem;
  border-bottom: 1px solid var(--border-subtle);
  gap: 12px;
}

.modal-title-badge-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.modal-title-group h3 {
  margin: 0;
  font-size: 1.25rem;
  font-weight: 750;
  color: var(--text-primary);
  letter-spacing: -0.02em;
}

.product-modal-cat-chip {
  display: inline-flex;
  align-items: center;
  padding: 2px 9px;
  border-radius: 9999px;
  font-size: 11px;
  font-weight: 650;
  color: #38bdf8;
  background: rgba(56, 189, 248, 0.12);
  border: 1px solid rgba(56, 189, 248, 0.32);
}

.product-modal-sku-chip {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 9999px;
  font-size: 11px;
  font-weight: 500;
  font-family: ui-monospace, SFMono-Regular, monospace;
  color: #94a3b8;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
}

.modal-subtitle {
  margin: 0.25rem 0 0;
  font-size: 0.825rem;
  color: var(--text-muted);
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
  box-shadow: none !important;
}

.close-btn:hover {
  color: #ffffff;
  background: rgba(255, 255, 255, 0.16);
  transform: none;
}

.close-btn:active {
  transform: scale(0.92);
  background: rgba(255, 255, 255, 0.22);
}

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1rem;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.form-group.span-2,
.form-section.span-2 {
  grid-column: span 2;
}

.required-star {
  color: #38bdf8;
  font-weight: 700;
  margin-left: 2px;
}

.form-section {
  background-color: rgba(15, 23, 42, 0.6);
  border: none !important;
  border-radius: 0.75rem;
  padding: 1rem;
}

.pricing-margin-section {
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.72) 0%, rgba(13, 22, 38, 0.84) 100%);
  border: none !important;
  border-radius: 14px;
  padding: 12px 14px;
  box-shadow: none !important;
}

.stock-levels-section {
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.72) 0%, rgba(13, 22, 38, 0.84) 100%);
  border: none !important;
  border-radius: 14px;
  padding: 12px 14px;
  box-shadow: none !important;
}

.section-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 0.85rem;
  flex-wrap: wrap;
}

.section-title-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
}

.section-icon {
  flex-shrink: 0;
}

.section-icon-pricing {
  color: #38bdf8;
}

.section-icon-stock {
  color: #fbbf24;
}

.profit-live-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 4px 10px;
  border-radius: 9999px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.01em;
  transition: all 0.2s ease;
  border: none !important;
  outline: none !important;
  box-shadow: none !important;
}

.profit-live-pos {
  background: rgba(34, 197, 94, 0.16);
  border: none !important;
  color: #4ade80;
  box-shadow: none !important;
}

.profit-live-zero {
  background: rgba(148, 163, 184, 0.14);
  border: none !important;
  color: #94a3b8;
  box-shadow: none !important;
}

.profit-live-neg {
  background: rgba(239, 68, 68, 0.16);
  border: none !important;
  color: #f87171;
  box-shadow: none !important;
}

.profit-sep {
  opacity: 0.45;
}

.pricing-metrics-microbar {
  margin-top: 0.75rem;
  padding-top: 0.6rem;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  display: flex;
  justify-content: flex-end;
  align-items: center;
  font-size: 11px;
  color: #94a3b8;
}

.text-emerald { color: #4ade80; }
.text-rose { color: #f87171; }

.stock-live-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 4px 9px;
  border-radius: 9999px;
  font-size: 11px;
  font-weight: 600;
  border: none !important;
  outline: none !important;
  box-shadow: none !important;
}

.stock-live-pill.in-stock {
  background: rgba(34, 197, 94, 0.16);
  border: none !important;
  color: #86efac;
  box-shadow: none !important;
}

.stock-live-pill.low {
  background: rgba(245, 158, 11, 0.16);
  border: none !important;
  color: #fcd34d;
  box-shadow: none !important;
}

.stock-live-pill.out {
  background: rgba(239, 68, 68, 0.16);
  border: none !important;
  color: #fca5a5;
  box-shadow: none !important;
}

.stock-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
}

[data-theme='light'] .modal-content {
  background-color: #ffffff !important;
  border: 1px solid #e2e8f0 !important;
  box-shadow: 0 20px 40px -10px rgba(15, 23, 42, 0.12), 0 0 0 1px rgba(15, 23, 42, 0.05) !important;
}

[data-theme='light'] .modal-header {
  border-bottom: 1px solid #e2e8f0 !important;
}

[data-theme='light'] .modal-header h3 {
  color: #0f172a !important;
}

[data-theme='light'] .form-section {
  background-color: #f8fafc !important;
  border: 1px solid #e2e8f0 !important;
}

[data-theme='light'] .section-title {
  color: var(--accent-color) !important;
}

[data-theme='light'] .form-group label {
  color: #334155 !important;
}

[data-theme='light'] .form-group input,
[data-theme='light'] .modal-select {
  background-color: #ffffff !important;
  border-color: #cbd5e1 !important;
  color: #0f172a !important;
}

[data-theme='light'] .form-group input:focus,
[data-theme='light'] .modal-select:focus {
  border-color: var(--accent-color) !important;
  box-shadow: 0 0 0 2px var(--glow-primary) !important;
}

[data-theme='light'] .input-with-prefix .prefix {
  color: #64748b !important;
}

[data-theme='light'] .scan-inside-btn.camera-scan-btn {
  background: rgba(2, 132, 199, 0.12) !important;
  color: #0284c7 !important;
  box-shadow: 0 1px 4px rgba(2, 132, 199, 0.15);
}

[data-theme='light'] .scan-inside-btn.hardware-scan-btn {
  background: rgba(22, 163, 74, 0.12) !important;
  color: #16a34a !important;
  box-shadow: 0 1px 4px rgba(22, 163, 74, 0.15);
}


[data-theme='light'] .image-upload-square {
  background-color: #f8fafc !important;
  border-color: #cbd5e1 !important;
}

[data-theme='light'] .upload-hint {
  color: #64748b !important;
}

[data-theme='light'] .category-item {
  background-color: #f1f5f9 !important;
  border: none !important;
}

[data-theme='light'] .category-item:hover {
  background-color: #e2e8f0 !important;
}

[data-theme='light'] .cat-name {
  color: #0f172a !important;
  font-weight: 600 !important;
}

[data-theme='light'] .cat-actions .action-btn {
  background: rgba(2, 132, 199, 0.12) !important;
  color: #0284c7 !important;
  border: none !important;
}

[data-theme='light'] .cat-actions .action-btn:hover {
  background: #0ea5e9 !important;
  color: #ffffff !important;
}

[data-theme='light'] .cat-actions .action-btn.delete {
  background: rgba(239, 68, 68, 0.12) !important;
  color: #ef4444 !important;
}

[data-theme='light'] .cat-actions .action-btn.delete:hover {
  background: #ef4444 !important;
  color: #ffffff !important;
}

[data-theme='light'] .category-modal .modal-actions {
  border-top: none !important;
}

[data-theme='light'] .category-modal .modal-actions .btn-secondary {
  background: #0ea5e9 !important;
  color: #ffffff !important;
  border: none !important;
}

[data-theme='light'] .btn-secondary {
  background-color: #f1f5f9 !important;
  color: #334155 !important;
  border: 1px solid #cbd5e1 !important;
}

[data-theme='light'] .btn-secondary:hover {
  background-color: #e2e8f0 !important;
  color: #0f172a !important;
}

.section-title {
  margin: 0 0 0.75rem;
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
}

.section-grid-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 1rem;
}

.section-grid-3 {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 0.75rem;
}

.form-group label {
  font-size: 0.825rem;
  font-weight: 500;
  color: #cbd5e1;
}

.form-group input,
.modal-select {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 0.5rem;
  padding: 0.55rem 0.75rem;
  color: var(--text-primary);
  width: 100%;
  font-size: 0.9rem;
}

.modal-select {
  appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%2394a3b8' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'%3E%3C/polyline%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 10px center;
  background-size: 14px;
  padding-right: 32px;
}

.input-with-prefix {
  position: relative;
  display: flex;
  align-items: center;
}

.input-with-prefix .prefix {
  position: absolute;
  left: 0.75rem;
  color: var(--text-muted);
  font-weight: 500;
  font-size: 0.8rem;
  pointer-events: none;
}

.input-with-prefix input {
  padding-left: 2.85rem;
}

.barcode-row-inputs {
  display: grid;
  grid-template-columns: minmax(0, 1.6fr) minmax(120px, 1fr);
  gap: 0.75rem;
  align-items: start;
}

.input-with-scan-btn {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.scan-input-wrapper {
  position: relative;
  width: 100%;
  display: flex;
  align-items: center;
}

.scan-input-wrapper input {
  width: 100%;
  padding-right: 76px !important;
}

.input-with-scan-btn.scanning-active input {
  border-color: #22c55e !important;
  box-shadow: 0 0 0 2px rgba(34, 197, 94, 0.25) !important;
}

.barcode-action-btns {
  position: absolute;
  right: 6px;
  top: 50%;
  transform: translateY(-50%);
  display: flex;
  align-items: center;
  gap: 5px;
  z-index: 2;
}

.scan-inside-btn {
  width: 30px;
  height: 30px;
  min-width: 30px;
  min-height: 30px;
  padding: 0;
  border-radius: 8px;
  border: none !important;
  outline: none !important;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s ease;
  -webkit-tap-highlight-color: transparent;
}

.scan-inside-btn.camera-scan-btn {
  background: rgba(14, 165, 233, 0.16) !important;
  color: #38bdf8 !important;
  box-shadow: 0 2px 6px rgba(14, 165, 233, 0.22);
}

.scan-inside-btn.camera-scan-btn:hover {
  background: rgba(14, 165, 233, 0.28) !important;
  color: #ffffff !important;
}

.scan-inside-btn.camera-scan-btn:active {
  background: #0ea5e9 !important;
  color: #ffffff !important;
  transform: scale(0.92);
}

.scan-inside-btn.hardware-scan-btn {
  background: rgba(34, 197, 94, 0.16) !important;
  color: #4ade80 !important;
  box-shadow: 0 2px 6px rgba(34, 197, 94, 0.22);
}

.scan-inside-btn.hardware-scan-btn:hover {
  background: rgba(34, 197, 94, 0.28) !important;
  color: #ffffff !important;
}

.scan-inside-btn.hardware-scan-btn:active {
  background: #22c55e !important;
  color: #ffffff !important;
  transform: scale(0.92);
}

.scan-inside-btn.hardware-scan-btn.active {
  background: #22c55e !important;
  color: #ffffff !important;
  box-shadow: 0 0 12px rgba(34, 197, 94, 0.5) !important;
}

/* Camera Barcode Scanner Modal */
.barcode-camera-backdrop {
  position: fixed;
  z-index: 200;
  inset: 0;
  display: grid;
  place-items: center;
  padding: max(12px, env(safe-area-inset-top)) max(12px, env(safe-area-inset-right)) max(12px, env(safe-area-inset-bottom)) max(12px, env(safe-area-inset-left));
  background: rgba(2, 6, 23, .86);
}

.barcode-camera-dialog {
  width: min(100%, 500px);
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
  max-height: min(50dvh, 420px);
  aspect-ratio: 4 / 3;
  display: block;
  border-radius: 10px;
  background: #020617;
  object-fit: cover;
}

.barcode-scan-reticle {
  position: absolute;
  inset: 18% 14%;
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
  animation: productLaserSweep 2s ease-in-out infinite alternate;
}

@keyframes productLaserSweep {
  0% { transform: translateY(-35px); opacity: 0.7; }
  100% { transform: translateY(35px); opacity: 1; }
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

.scan-laser-indicator {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.72rem;
  color: #4ade80;
}

.laser-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background-color: #22c55e;
  box-shadow: 0 0 6px #22c55e;
}


.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.75rem;
  margin-top: 1.35rem;
  padding-top: 1rem;
  border-top: 1px solid var(--border-subtle);
}

.btn-primary,
.btn-secondary,
.btn-danger {
  padding: 0.55rem 1.15rem;
  border-radius: 0.5rem;
  font-weight: 600;
  font-size: 0.875rem;
  border: none;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}

.btn-primary {
  background: linear-gradient(135deg, var(--accent-color) 0%, #0284c7 100%);
  color: #ffffff;
}

.btn-secondary {
  background-color: transparent;
  border: 1px solid var(--border-color);
  color: #cbd5e1;
}

.btn-danger {
  background-color: #ef4444;
  color: #ffffff;
}

/* Category Modal */
.category-modal {
  max-width: 440px;
  border: none !important;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5) !important;
}

.category-modal .modal-header {
  border-bottom: none !important;
  padding-bottom: 0.5rem;
}

.add-category-form {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1.25rem;
}

.add-category-form input {
  flex-grow: 1;
  background-color: rgba(30, 41, 59, 0.72);
  border: none !important;
  outline: none !important;
  box-shadow: none !important;
  border-radius: 8px;
  padding: 0.65rem 0.95rem;
  color: var(--text-primary);
  font-size: 0.9rem;
}

.add-category-form .btn-primary {
  min-height: 42px;
  padding: 0 1.25rem;
  font-weight: 600;
  font-size: 0.9rem;
  border-radius: 8px;
  border: none !important;
  outline: none !important;
  background: #0ea5e9;
  color: #ffffff;
  box-shadow: none !important;
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  cursor: pointer;
  transition: all 0.18s ease;
}

.add-category-form .btn-primary:hover:not(:disabled) {
  background: #0284c7;
}

.add-category-form .btn-primary:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.category-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  max-height: 320px;
  overflow-y: auto;
}

.category-item {
  background-color: rgba(15, 23, 42, 0.65);
  border: none !important;
  outline: none !important;
  box-shadow: none !important;
  border-radius: 10px;
  padding: 0.75rem 1rem;
  transition: background-color 0.18s ease;
}

.category-item:hover {
  background-color: rgba(22, 33, 58, 0.75);
}

.view-category-mode,
.edit-category-mode {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.65rem;
}

.cat-name {
  font-weight: 550;
  color: #f1f5f9;
  font-size: 0.925rem;
  letter-spacing: -0.01em;
}

.edit-cat-input {
  flex-grow: 1;
  background-color: rgba(30, 41, 59, 0.85);
  border: none !important;
  outline: none !important;
  box-shadow: none !important;
  border-radius: 8px;
  padding: 0.55rem 0.85rem;
  color: #ffffff;
  font-size: 0.9rem;
}

.cat-actions {
  display: flex;
  align-items: center;
  gap: 0.45rem;
}

.cat-actions .action-btn {
  width: 34px;
  height: 34px;
  min-width: 34px;
  min-height: 34px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 8px;
  border: none !important;
  outline: none !important;
  background: rgba(56, 189, 248, 0.14);
  color: #38bdf8;
  cursor: pointer;
  padding: 0;
  transition: background 0.18s ease, transform 0.15s ease, color 0.18s ease;
  -webkit-tap-highlight-color: transparent;
  box-shadow: none !important;
}

.cat-actions .action-btn:hover {
  background: rgba(56, 189, 248, 0.28);
  color: #ffffff;
}

.cat-actions .action-btn:active {
  transform: scale(0.92);
}

.cat-actions .action-btn.delete {
  background: rgba(239, 68, 68, 0.14);
  color: #f87171;
}

.cat-actions .action-btn.delete:hover {
  background: #ef4444;
  color: #ffffff;
}

.cat-actions .action-btn.save {
  background: rgba(34, 197, 94, 0.18);
  color: #4ade80;
}

.cat-actions .action-btn.save:hover {
  background: #22c55e;
  color: #ffffff;
}

.cat-actions .action-btn.cancel {
  background: rgba(239, 68, 68, 0.14);
  color: #f87171;
}

.cat-actions .action-btn.cancel:hover {
  background: #ef4444;
  color: #ffffff;
}

.category-modal .modal-actions {
  border-top: none !important;
  margin-top: 1.25rem;
  padding-top: 0.5rem;
}

.category-modal .modal-actions .btn-secondary {
  min-width: 100px;
  padding: 0.65rem 1.6rem;
  font-weight: 600;
  font-size: 0.9rem;
  border-radius: 8px;
  border: none !important;
  outline: none !important;
  background: #0ea5e9;
  color: #ffffff;
  box-shadow: none !important;
  cursor: pointer;
  transition: all 0.18s ease;
}

.category-modal .modal-actions .btn-secondary:hover {
  background: #0284c7;
}

.category-modal .modal-actions .btn-secondary:active {
  transform: scale(0.96);
}

.empty-categories {
  text-align: center;
  padding: 1.5rem;
  color: var(--text-muted);
}

/* Adjust Stock Modal - Stand Out on Tablets & Desktop */
.adjust-stock-backdrop {
  position: fixed !important;
  inset: 0 !important;
  z-index: 100 !important;
  display: grid !important;
  place-items: center !important;
  padding: 24px !important;
  background: rgba(3, 7, 18, 0.78) !important;
  backdrop-filter: blur(14px) !important;
  -webkit-backdrop-filter: blur(14px) !important;
  overflow-y: auto !important;
  box-sizing: border-box !important;
}

.adjust-stock-modal {
  position: relative !important;
  width: min(92vw, 490px) !important;
  max-width: 490px !important;
  height: auto !important;
  max-height: 92dvh !important;
  margin: auto !important;
  padding: 26px !important;
  border-radius: 22px !important;
  background: linear-gradient(180deg, #131f36 0%, #0c1424 100%) !important;
  border: 1px solid rgba(56, 189, 248, 0.28) !important;
  box-shadow: 0 25px 65px -12px rgba(0, 0, 0, 0.88), 0 0 0 1px rgba(255, 255, 255, 0.08), 0 0 32px rgba(56, 189, 248, 0.14) !important;
  display: flex !important;
  flex-direction: column !important;
  overflow-y: auto !important;
  box-sizing: border-box !important;
}

[data-theme='light'] .adjust-stock-backdrop {
  background: rgba(15, 23, 42, 0.52) !important;
}

[data-theme='light'] .adjust-stock-modal {
  background: #ffffff !important;
  border: 1px solid #cbd5e1 !important;
  box-shadow: 0 24px 60px -12px rgba(15, 23, 42, 0.25), 0 0 0 1px rgba(0, 0, 0, 0.06) !important;
}

.adjust-stock-header {
  display: flex !important;
  align-items: flex-start !important;
  justify-content: space-between !important;
  gap: 12px !important;
  margin-bottom: 20px !important;
  padding-bottom: 16px !important;
  border-bottom: 1px solid rgba(148, 163, 184, 0.15) !important;
}

[data-theme='light'] .adjust-stock-header {
  border-bottom-color: #e2e8f0 !important;
}

.modal-title-with-icon {
  display: flex;
  align-items: center;
  gap: 14px;
}

.modal-icon-badge {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  border-radius: 12px;
  background: rgba(56, 189, 248, 0.14);
  color: #38bdf8;
  border: 1px solid rgba(56, 189, 248, 0.32);
  flex-shrink: 0;
}

[data-theme='light'] .modal-icon-badge {
  background: rgba(2, 132, 199, 0.1);
  color: #0284c7;
  border-color: rgba(2, 132, 199, 0.25);
}

.adjust-stock-header h3 {
  margin: 0;
  font-size: 19px;
  font-weight: 750;
  letter-spacing: -0.015em;
  color: var(--text-primary);
}

.adjust-stock-header .modal-subtitle {
  margin: 4px 0 0;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  font-size: 13px;
}

.adjust-product-name {
  font-weight: 600;
  color: var(--text-primary);
}

.adjust-stock-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 600;
  color: #38bdf8;
  background: rgba(56, 189, 248, 0.12);
  border: 1px solid rgba(56, 189, 248, 0.24);
}

[data-theme='light'] .adjust-stock-badge {
  color: #0369a1;
  background: #e0f2fe;
  border-color: #bae6fd;
}

.resulting-stock-box {
  display: flex;
  align-items: center;
  gap: 6px;
  height: 44px;
  padding: 0 14px;
  border-radius: 10px;
  background: rgba(56, 189, 248, 0.08);
  border: 1px solid rgba(56, 189, 248, 0.25);
  box-sizing: border-box;
}

.resulting-stock-val {
  font-size: 16px;
  font-weight: 700;
  color: #38bdf8;
}

.resulting-stock-unit {
  font-size: 13px;
  font-weight: 500;
  color: #94a3b8;
}

[data-theme='light'] .resulting-stock-box {
  background: #f0f9ff;
  border-color: #bae6fd;
}

[data-theme='light'] .resulting-stock-val {
  color: #0284c7;
}

.adjust-stock-form .form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}

.adjust-stock-form .form-group {
  display: flex;
  flex-direction: column;
}

.adjust-stock-form .form-group.span-2 {
  grid-column: 1 / -1;
}

.adjust-stock-form .form-group label {
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  color: #94a3b8;
  margin-bottom: 6px;
}

[data-theme='light'] .adjust-stock-form .form-group label {
  color: #64748b;
}

.adjust-stock-form .modal-select,
.adjust-stock-form .form-group input {
  min-height: 44px;
  border-radius: 10px;
  border: 1px solid rgba(148, 163, 184, 0.24);
  background: rgba(15, 23, 42, 0.65);
  font-size: 14px;
  color: var(--text-primary);
  padding: 0 12px;
  box-sizing: border-box;
  transition: border-color 0.15s ease, box-shadow 0.15s ease;
}

.adjust-stock-form .modal-select:focus,
.adjust-stock-form .form-group input:focus {
  border-color: #38bdf8;
  outline: none;
  box-shadow: 0 0 0 3px rgba(56, 189, 248, 0.2);
}

[data-theme='light'] .adjust-stock-form .modal-select,
[data-theme='light'] .adjust-stock-form .form-group input {
  background: #f8fafc;
  border-color: #cbd5e1;
}

.adjust-stock-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 24px;
  padding-top: 16px;
  border-top: 1px solid rgba(148, 163, 184, 0.14);
}

[data-theme='light'] .adjust-stock-actions {
  border-top-color: #e2e8f0;
}

.adjust-stock-actions .btn-secondary {
  min-height: 44px;
  padding: 0 18px;
  border-radius: 10px;
  font-weight: 600;
  font-size: 14px;
}

.adjust-stock-actions .adjust-submit-btn {
  min-height: 44px;
  padding: 0 22px;
  border-radius: 10px;
  font-weight: 700;
  font-size: 14px;
  background: linear-gradient(135deg, #0284c7 0%, #0369a1 100%) !important;
  color: #ffffff !important;
  box-shadow: 0 4px 14px rgba(2, 132, 199, 0.4) !important;
  border: none !important;
  transition: transform 0.12s ease, box-shadow 0.12s ease;
}

.adjust-stock-actions .adjust-submit-btn:not(:disabled):hover {
  transform: translateY(-1px);
  box-shadow: 0 6px 18px rgba(2, 132, 199, 0.5) !important;
}

.adjust-stock-actions .adjust-submit-btn:not(:disabled):active {
  transform: translateY(0);
}

/* Delete Confirm Modal */
.delete-confirm-modal {
  max-width: 400px;
  text-align: center;
}

.delete-warning-icon {
  margin: 0 auto 1.1rem;
  width: 60px;
  height: 60px;
  border-radius: 50%;
  background: rgba(239, 68, 68, 0.12);
  color: #ef4444;
  display: flex;
  align-items: center;
  justify-content: center;
}

.delete-confirm-modal h3 {
  margin: 0 0 0.5rem;
  font-size: 1.2rem;
  color: #ffffff;
}

.delete-confirm-modal p {
  margin: 0 0 1.5rem;
  color: var(--text-muted);
  font-size: 0.9rem;
  line-height: 1.5;
}

.delete-confirm-actions {
  display: flex;
  gap: 0.75rem;
}

.delete-confirm-actions button {
  flex: 1;
  justify-content: center;
}

.product-hero-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
  padding: 14px;
  margin-bottom: 1.25rem;
  border-radius: 16px;
  background: linear-gradient(180deg, rgba(15, 23, 42, 0.7) 0%, rgba(10, 16, 30, 0.9) 100%);
  border: none !important;
  box-shadow: none !important;
}

.image-upload-wrapper {
  position: relative;
  width: 112px;
  height: 112px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.image-upload-frame {
  width: 112px;
  height: 112px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.03);
  border: 1.5px dashed rgba(56, 189, 248, 0.35);
  border-radius: 18px;
  cursor: pointer;
  transition: all 0.22s cubic-bezier(0.16, 1, 0.3, 1);
  overflow: hidden;
  padding: 0;
  position: relative;
}

.image-upload-frame.has-image {
  border: none !important;
  box-shadow: 0 10px 25px -4px rgba(0, 0, 0, 0.5) !important;
}

.image-upload-frame:hover {
  border-color: #38bdf8;
  transform: translateY(-2px);
  box-shadow: 0 12px 30px -4px rgba(0, 0, 0, 0.6), 0 0 24px rgba(56, 189, 248, 0.25);
}

.uploaded-preview {
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 16px;
}

.photo-change-overlay {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  padding: 5px 0 6px;
  background: rgba(10, 17, 28, 0.85);
  backdrop-filter: blur(10px);
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  font-size: 10.5px;
  font-weight: 600;
  color: #38bdf8;
  border-top: 1px solid rgba(255, 255, 255, 0.1);
  transition: background 0.18s ease;
}

.photo-change-overlay:hover {
  background: rgba(2, 132, 199, 0.9);
  color: #ffffff;
}

.upload-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 5px;
  padding: 10px;
}

.upload-icon-circle {
  width: 38px;
  height: 38px;
  border-radius: 50%;
  display: grid;
  place-items: center;
  background: rgba(56, 189, 248, 0.12);
  color: #38bdf8;
}

.upload-hint {
  font-size: 11px;
  font-weight: 650;
  color: #cbd5e1;
}

.upload-subhint {
  font-size: 9.5px;
  color: #64748b;
}

.remove-image-btn {
  position: absolute;
  top: -6px;
  right: -6px;
  width: 26px;
  height: 26px;
  border-radius: 50%;
  background: rgba(15, 23, 42, 0.95);
  color: #f87171;
  border: none !important;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  box-shadow: 0 4px 10px rgba(0, 0, 0, 0.5);
  transition: all 0.18s ease;
  z-index: 2;
}

.remove-image-btn:hover {
  background: #ef4444;
  color: #ffffff;
  transform: scale(1.1);
  box-shadow: 0 4px 14px rgba(239, 68, 68, 0.5);
}

.photo-hero-tip {
  font-size: 11px;
  color: #64748b;
  text-align: center;
}

.photo-quick-actions {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  margin-top: 4px;
}

.photo-action-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 14px;
  font-size: 12px;
  font-weight: 600;
  border-radius: 20px;
  cursor: pointer;
  border: 1px solid rgba(255, 255, 255, 0.12);
  background: rgba(255, 255, 255, 0.06);
  color: #f1f5f9;
  transition: all 0.2s ease;
}

.photo-action-chip:hover {
  transform: translateY(-1px);
}

.photo-action-chip.camera-chip {
  background: rgba(14, 165, 233, 0.15);
  border-color: rgba(14, 165, 233, 0.4);
  color: #38bdf8;
}

.photo-action-chip.camera-chip:hover {
  background: rgba(14, 165, 233, 0.25);
  border-color: #38bdf8;
}

.photo-action-chip.gallery-chip {
  background: rgba(148, 163, 184, 0.12);
  border-color: rgba(148, 163, 184, 0.25);
  color: #cbd5e1;
}

.photo-action-chip.gallery-chip:hover {
  background: rgba(148, 163, 184, 0.2);
  color: #ffffff;
}

[data-theme='light'] .photo-action-chip {
  background: #f1f5f9;
  border-color: #cbd5e1;
  color: #334155;
}

[data-theme='light'] .photo-action-chip.camera-chip {
  background: #e0f2fe;
  border-color: #7dd3fc;
  color: #0284c7;
}

[data-theme='light'] .photo-action-chip.gallery-chip {
  background: #f1f5f9;
  border-color: #cbd5e1;
  color: #475569;
}

/* Photo Picker Sheet / Modal */
.photo-picker-backdrop {
  position: fixed;
  inset: 0;
  z-index: 1200;
  background: rgba(0, 0, 0, 0.65);
  backdrop-filter: blur(4px);
  -webkit-backdrop-filter: blur(4px);
  display: flex;
  align-items: flex-end;
  justify-content: center;
}

.photo-picker-sheet {
  width: 100%;
  max-width: 480px;
  background: #0f172a;
  border-radius: 20px 20px 0 0;
  padding: 18px 20px max(24px, env(safe-area-inset-bottom));
  box-shadow: 0 -10px 40px rgba(0, 0, 0, 0.5);
  animation: slideUpPhotoSheet 0.25s cubic-bezier(0.16, 1, 0.3, 1);
}

@keyframes slideUpPhotoSheet {
  from { transform: translateY(100%); }
  to { transform: translateY(0); }
}

.photo-picker-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 16px;
  padding-bottom: 12px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.photo-picker-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  color: #f8fafc;
}

.photo-picker-close {
  background: transparent;
  border: none;
  color: #94a3b8;
  cursor: pointer;
  padding: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
}

.photo-picker-options {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.photo-picker-option {
  display: flex;
  align-items: center;
  gap: 14px;
  width: 100%;
  padding: 14px 16px;
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 14px;
  color: #f8fafc;
  cursor: pointer;
  text-align: left;
  transition: all 0.2s ease;
}

.photo-picker-option:active {
  background: rgba(255, 255, 255, 0.12);
  transform: scale(0.99);
}

.option-icon {
  width: 44px;
  height: 44px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.option-icon.camera-icon-wrap {
  background: rgba(14, 165, 233, 0.2);
  color: #38bdf8;
}

.option-icon.gallery-icon-wrap {
  background: rgba(168, 85, 247, 0.2);
  color: #c084fc;
}

.option-details {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.option-details strong {
  font-size: 14px;
  font-weight: 600;
  color: #f8fafc;
}

.option-details small {
  font-size: 12px;
  color: #94a3b8;
}

[data-theme='light'] .photo-picker-sheet {
  background: #ffffff;
  box-shadow: 0 -10px 40px rgba(0, 0, 0, 0.15);
}

[data-theme='light'] .photo-picker-header {
  border-bottom-color: #e2e8f0;
}

[data-theme='light'] .photo-picker-header h3 {
  color: #0f172a;
}

[data-theme='light'] .photo-picker-option {
  background: #f8fafc;
  border-color: #e2e8f0;
  color: #0f172a;
}

[data-theme='light'] .photo-picker-option:active {
  background: #f1f5f9;
}

[data-theme='light'] .option-details strong {
  color: #0f172a;
}

[data-theme='light'] .option-details small {
  color: #64748b;
}

@media (max-width: 639px) {
  .view-section {
    width: 100%;
    max-width: 100%;
    margin: 0;
  }

  .products-view-section .card,
  .card {
    width: 100%;
    max-width: 100%;
    padding: 0 !important;
    margin: 0 !important;
    border: none !important;
    border-radius: 0 !important;
    background: transparent !important;
    box-shadow: none !important;
  }

  .products-view-section .card-header,
  .card-header {
    display: flex;
    flex-direction: column;
    align-items: stretch;
    gap: 6px !important;
    margin: 0 !important;
    padding: 2px 8px 4px 8px !important;
    border-bottom: none !important;
  }

  .summary-badges {
    width: 100%;
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 6px !important;
    margin: 0 !important;
    padding: 0 !important;
    pointer-events: none !important;
    user-select: none !important;
  }

  .summary-badge {
    min-width: 0;
    justify-content: center;
    padding: 6px 8px !important;
    border: none !important;
    border-left: none !important;
    pointer-events: none !important;
    user-select: none !important;
    cursor: default !important;
  }

  .summary-badge:nth-child(2) {
    border-left: none !important;
    border-left-color: transparent !important;
  }

  .header-controls {
    width: 100%;
    display: grid;
    grid-template-columns: 1.15fr 0.92fr 0.93fr;
    gap: 6px;
    align-items: stretch;
  }

  .search-bar,
  .search-bar input {
    width: 100%;
  }

  .search-bar {
    grid-column: 1 / -1;
  }

  .search-bar input {
    height: 42px;
    font-size: 12px;
    border: none !important;
    outline: none !important;
    box-shadow: none !important;
  }

  .filter-select {
    width: 100%;
    min-width: 0;
    height: 42px;
    font-size: 13px;
    font-weight: 600;
    color: #f1f5f9;
    padding: 0 28px 0 10px !important;
    text-overflow: ellipsis;
    border: none !important;
    outline: none !important;
    box-shadow: none !important;
    background-color: rgba(30, 41, 59, 0.72) !important;
    border-radius: 10px !important;
    appearance: none !important;
    background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%2394a3b8' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'%3E%3C/polyline%3E%3C/svg%3E") !important;
    background-repeat: no-repeat !important;
    background-position: right 8px center !important;
    background-size: 14px !important;
  }

  .filter-select[aria-label="Filter by category"] {
    grid-column: auto;
  }

  .action-dropdown-container {
    position: relative;
    width: 100%;
  }

  .action-dropdown-btn,
  .add-btn {
    width: 100%;
    min-width: 0;
    height: 42px;
    font-size: 13px;
    font-weight: 650;
    padding: 0 8px;
    justify-content: center;
    gap: 5px;
    border: none !important;
    outline: none !important;
    box-shadow: none !important;
    border-radius: 10px !important;
    background-color: #0ea5e9 !important;
    color: #ffffff !important;
  }

  .action-dropdown-menu {
    position: absolute !important;
    top: calc(100% + 6px) !important;
    right: 0 !important;
    min-width: 190px !important;
    background-color: #141f32 !important;
    border: none !important;
    outline: none !important;
    border-radius: 14px !important;
    box-shadow: 0 20px 45px -4px rgba(0, 0, 0, 0.75), 0 0 0 1px rgba(255, 255, 255, 0.08) !important;
    padding: 6px !important;
    z-index: 9999 !important;
  }

  .action-dropdown-menu .dropdown-item {
    min-height: 48px;
    font-size: 14px;
    font-weight: 600;
    padding: 10px 14px;
    border-radius: 10px;
  }


  .products-grid {
    display: block;
    padding: 2px 8px 8px 8px !important;
  }

  .virtual-grid-row {
    gap: 8px;
  }

  .product-grid-card {
    min-width: 0;
    padding: 8px;
    gap: 6px;
    height: 100%;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    justify-content: space-between;
    border: none !important;
    outline: none !important;
    box-shadow: 0 4px 16px -2px rgba(0, 0, 0, 0.28) !important;
  }

  .stock-badge,
  .status-in-stock,
  .status-low-stock,
  .status-out-of-stock {
    border: none !important;
    outline: none !important;
  }

  .product-grid-header {
    gap: 4px;
    min-height: 48px;
    align-items: flex-start;
  }

  .product-grid-name {
    flex: 1;
    min-width: 0;
  }

  .product-grid-name h3 {
    font-size: 13px;
    line-height: 1.25;
    height: 33px;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }

  .product-grid-name > span {
    font-size: 11px;
    line-height: 1.2;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    display: block;
    margin-top: 2px;
  }

  .product-grid-details {
    gap: 4px 6px;
  }

  .product-grid-details dt {
    font-size: 10.5px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .product-grid-details dd {
    font-size: 12px;
    margin-top: 1px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .product-grid-actions {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 4px;
    padding-top: 6px;
    border-top: none !important;
  }

  .product-grid-actions button {
    min-height: 34px;
    padding: 0 4px;
    font-size: 11px;
    gap: 2px;
    border: none !important;
    outline: none !important;
  }

  .product-grid-actions button.delete {
    grid-column: 1 / -1;
  }

  .product-modal-backdrop {
    z-index: 60;
    padding: 0;
    place-items: stretch;
    overflow: hidden;
  }

  .product-editor-modal {
    display: flex !important;
    flex-direction: column !important;
    height: 100dvh !important;
    max-height: 100dvh !important;
    padding: 0 !important;
    border-radius: 0 !important;
    border: none !important;
    outline: none !important;
    box-shadow: none !important;
    background: #0a0f1d !important;
    overflow: hidden !important;
  }

  .product-editor-modal .modal-header {
    padding: max(16px, env(safe-area-inset-top)) 16px 14px 16px;
    margin: 0;
    flex-shrink: 0;
    background: rgba(13, 20, 36, 0.98);
    border-bottom: none !important;
  }

  .product-editor-modal .form-group input,
  .product-editor-modal .modal-select,
  .product-editor-modal .input-with-prefix input,
  .product-editor-modal .input-with-scan-btn input {
    border: none !important;
    outline: none !important;
    box-shadow: none !important;
    background-color: rgba(30, 41, 59, 0.72) !important;
    min-height: 42px;
    font-size: 14px !important;
    border-radius: 10px !important;
  }

  .product-editor-modal .input-with-scan-btn input {
    padding-right: 76px !important;
  }

  .product-editor-modal .modal-select {
    padding-right: 32px !important;
    font-weight: 600;
    color: #f1f5f9 !important;
  }

  .product-editor-modal .pricing-margin-section,
  .product-editor-modal .stock-levels-section,
  .product-editor-modal .product-hero-card,
  .product-editor-modal .form-section {
    border: none !important;
    box-shadow: none !important;
  }

  .product-editor-modal .pricing-metrics-microbar {
    border-top: none !important;
  }

  .product-editor-modal .photo-change-overlay {
    border-top: none !important;
  }

  .category-modal {
    border: none !important;
    outline: none !important;
    box-shadow: none !important;
  }

  .category-modal .modal-header,
  .category-modal .modal-actions {
    border: none !important;
    border-top: none !important;
    border-bottom: none !important;
  }

  .category-modal .add-category-form input,
  .category-modal .edit-cat-input {
    border: none !important;
    outline: none !important;
    box-shadow: none !important;
    background-color: rgba(30, 41, 59, 0.72) !important;
  }

  .category-modal .category-item {
    border: none !important;
    box-shadow: none !important;
    background-color: rgba(15, 23, 42, 0.65) !important;
  }

  .product-form-container {
    flex: 1 1 auto;
    min-height: 0;
    overflow-y: auto;
    -webkit-overflow-scrolling: touch;
    padding: 14px 14px 0 14px;
    display: flex;
    flex-direction: column;
  }

  .product-hero-card {
    padding: 12px;
    margin-bottom: 1rem;
    gap: 8px;
  }

  .sticky-modal-actions {
    position: sticky;
    bottom: 0;
    margin: auto -14px 0 -14px !important;
    padding: 12px 14px max(16px, env(safe-area-inset-bottom)) 14px !important;
    background: rgba(10, 16, 29, 0.96) !important;
    backdrop-filter: blur(20px) !important;
    -webkit-backdrop-filter: blur(20px) !important;
    border-top: none !important;
    box-shadow: none !important;
    display: grid !important;
    grid-template-columns: repeat(2, minmax(0, 1fr)) !important;
    gap: 10px !important;
    z-index: 20 !important;
  }

  .sticky-modal-actions .btn-primary,
  .sticky-modal-actions .btn-secondary {
    min-height: 48px;
    font-size: 13.5px;
    font-weight: 700;
    justify-content: center;
    border-radius: 10px;
    transition: transform 0.12s ease, filter 0.12s ease;
  }

  .sticky-modal-actions .btn-secondary {
    background: rgba(148, 163, 184, 0.14) !important;
    color: #e2e8f0 !important;
  }

  .sticky-modal-actions .btn-primary:active,
  .sticky-modal-actions .btn-secondary:active {
    filter: brightness(0.9);
  }

  /* Light Theme overrides on mobile */
  [data-theme='light'] .product-editor-modal {
    background: #ffffff !important;
    color: #0f172a !important;
  }

  [data-theme='light'] .product-editor-modal .modal-header {
    background: #ffffff !important;
    border-bottom: 1px solid #f1f5f9 !important;
  }

  [data-theme='light'] .product-editor-modal .modal-header h3 {
    color: #0f172a !important;
  }

  [data-theme='light'] .product-editor-modal .modal-subtitle {
    color: #64748b !important;
  }

  [data-theme='light'] .product-editor-modal .form-group input,
  [data-theme='light'] .product-editor-modal .modal-select,
  [data-theme='light'] .product-editor-modal .input-with-prefix input,
  [data-theme='light'] .product-editor-modal .input-with-scan-btn input {
    background-color: #f1f5f9 !important;
    color: #0f172a !important;
  }

  [data-theme='light'] .product-editor-modal .modal-select {
    color: #0f172a !important;
  }

  [data-theme='light'] .product-editor-modal .pricing-margin-section,
  [data-theme='light'] .product-editor-modal .stock-levels-section,
  [data-theme='light'] .product-editor-modal .product-hero-card,
  [data-theme='light'] .product-editor-modal .form-section {
    background: #f8fafc !important;
    color: #0f172a !important;
  }

  [data-theme='light'] .product-editor-modal .form-group label {
    color: #475569 !important;
  }

  [data-theme='light'] .category-modal {
    background-color: #ffffff !important;
    color: #0f172a !important;
  }

  [data-theme='light'] .category-modal .modal-header h3 {
    color: #0f172a !important;
  }

  [data-theme='light'] .category-modal .modal-subtitle {
    color: #64748b !important;
  }

  [data-theme='light'] .category-modal .add-category-form input,
  [data-theme='light'] .category-modal .edit-cat-input {
    background-color: #f1f5f9 !important;
    color: #0f172a !important;
  }

  [data-theme='light'] .category-modal .category-item {
    background-color: #f8fafc !important;
    color: #0f172a !important;
  }

  [data-theme='light'] .category-modal .cat-name {
    color: #0f172a !important;
  }

  [data-theme='light'] .sticky-modal-actions {
    background: #ffffff !important;
    border-top: none !important;
    box-shadow: none !important;
  }

  [data-theme='light'] .sticky-modal-actions .btn-secondary {
    background: #f1f5f9 !important;
    color: #0f172a !important;
  }

  .product-editor-modal .close-btn {
    background: transparent !important;
    border: none !important;
    outline: none !important;
    box-shadow: none !important;
  }

  .product-editor-modal .product-hero-card {
    background: transparent !important;
    border: none !important;
    box-shadow: none !important;
  }

  .product-editor-modal .image-upload-frame.has-image {
    border: none !important;
    outline: none !important;
    box-shadow: none !important;
  }

  .product-editor-modal .sticky-modal-actions .btn-secondary {
    border: none !important;
    outline: none !important;
    box-shadow: none !important;
  }

  [data-theme='light'] .product-editor-modal .product-hero-card {
    background: transparent !important;
  }
}

</style>
