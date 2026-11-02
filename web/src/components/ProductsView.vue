<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { usePhoneBreakpoint } from '../utils/breakpoints'
import {
  AlertTriangle,
  ArrowRight,
  Barcode,
  Check,
  ChevronDown,
  Edit,
  ImagePlus,
  Layers,
  LayoutGrid,
  Lock,
  List,
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
}>()

const emit = defineEmits<{
  'navigate-settings': []
}>()

const products = ref<Product[]>([])
const categoryNames = ref<string[]>([])
const search = ref('')
const selectedCategory = ref('All Categories')
const selectedStock = ref<'All Stock' | 'In Stock' | 'Low Stock' | 'Out of Stock'>('All Stock')
const loading = ref(true)
const saving = ref(false)
const adjusting = ref(false)
const categoryBusy = ref(false)
const error = ref('')
const notice = ref('')
const isDropdownOpen = ref(false)
const { isPhone } = usePhoneBreakpoint()
const viewMode = ref<'grid' | 'list'>(
  typeof window !== 'undefined' && window.matchMedia('(max-width: 639px)').matches ? 'grid' : 'list',
)
const viewModeManuallySelected = ref(false)

const showForm = ref(false)
const editingProductId = ref<number | null>(null)
const editingOriginalQuantity = ref(0)
const form = ref<ProductInput>(blankProduct())
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

function setViewMode(mode: 'grid' | 'list') {
  if (isPhone.value) return
  viewModeManuallySelected.value = true
  viewMode.value = mode
}

function updateDefaultViewMode() {
  if (isPhone.value) {
    viewMode.value = 'grid'
  } else if (!viewModeManuallySelected.value) {
    viewMode.value = 'list'
  }
}

const imageInputRef = ref<HTMLInputElement | null>(null)

function triggerImageUpload() {
  imageInputRef.value?.click()
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
    }
    img.src = dataUrl
  }
  reader.readAsDataURL(file)
}

function removeProductImage() {
  form.value.image = ''
  if (imageInputRef.value) imageInputRef.value.value = ''
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

const defaultUnits = ['pcs', 'kg', 'L', 'g', 'ml', 'pack', 'box']

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

const availableUnits = computed(() => {
  const units = new Set(defaultUnits)
  for (const product of products.value) {
    if (product.unit && product.unit.trim()) {
      units.add(product.unit.trim())
    }
  }
  return Array.from(units)
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
  void loadProducts()
  window.addEventListener('click', closeDropdown)
  window.addEventListener('resize', updateDefaultViewMode)
})
onUnmounted(() => {
  window.removeEventListener('click', closeDropdown)
  window.removeEventListener('resize', updateDefaultViewMode)
})
</script>

<template>
  <section class="view-section">
    <div v-if="error && !showForm && !adjustingProduct && !showCategoryModal" class="inline-error" role="alert">{{ error }}</div>
    <div v-if="notice" class="inline-success" role="status">{{ notice }}</div>

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
          <div class="products-view-switcher" role="group" aria-label="Product layout">
            <button
              type="button"
              title="Grid View"
              aria-label="Grid View"
              :aria-pressed="viewMode === 'grid'"
              :class="{ active: viewMode === 'grid' }"
              @click="setViewMode('grid')"
            >
              <LayoutGrid :size="17" />
            </button>
            <button
              type="button"
              title="List View"
              aria-label="List View"
              :aria-pressed="viewMode === 'list'"
              :class="{ active: viewMode === 'list' }"
              @click="setViewMode('list')"
            >
              <List :size="17" />
            </button>
          </div>
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

          <div v-if="canManage" class="action-dropdown-container">
            <button type="button" class="action-dropdown-btn" @click.stop="toggleDropdown">
              <Layers :size="18" />
              <span>Manage</span>
              <ChevronDown :size="16" :class="{ rotate: isDropdownOpen }" class="chevron-icon" />
            </button>
            <div v-if="isDropdownOpen" class="action-dropdown-menu">
              <button type="button" class="dropdown-item" @click="openCreateForm">
                <PlusCircle :size="16" class="item-icon add-icon" />
                <span>Add Product</span>
              </button>
              <button type="button" class="dropdown-item" @click="openCategoryManagement">
                <Layers :size="16" class="item-icon cat-icon" />
                <span>Categories</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      <div v-if="viewMode === 'grid'" class="products-grid">
        <p v-if="loading" class="products-grid-empty">Loading products...</p>
        <p v-else-if="filteredProducts.length === 0" class="products-grid-empty">No products found.</p>
        <article v-for="product in filteredProducts" v-else :key="product.id" class="product-grid-card">
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
            <div>
              <dt>Selling price</dt>
              <dd>{{ formatMoney(product.sellingPrice, currency) }}</dd>
            </div>
            <div>
              <dt>Cost price</dt>
              <dd>{{ formatMoney(product.costPrice, currency) }}</dd>
            </div>
            <div>
              <dt>Quantity</dt>
              <dd>{{ formatQuantity(product.quantity) }} {{ product.unit }}</dd>
            </div>
            <div>
              <dt>Profit per unit</dt>
              <dd :class="profitPerUnit(product) >= 0 ? 'profit-positive' : 'profit-negative'">
                {{ formatMoney(profitPerUnit(product), currency) }}
              </dd>
            </div>
          </dl>
          <div v-if="canManage" class="product-grid-actions" :aria-label="`${product.name} actions`">
            <button type="button" @click="openEditForm(product)">
              <Edit :size="15" /> Edit
            </button>
            <button type="button" @click="openAdjustStock(product)">
              <SlidersHorizontal :size="15" /> Adjust Stock
            </button>
            <button type="button" class="delete" @click="promptDeleteProduct(product)">
              <Trash2 :size="15" /> Delete
            </button>
          </div>
        </article>
      </div>

      <div v-else class="table-container" style="flex: 1; min-height: 0; overflow-y: auto;">
        <table>
          <thead>
            <tr>
              <th class="text-center">Product Name</th>
              <th class="text-center">Category</th>
              <th class="text-center">Cost Price</th>
              <th class="text-center">Selling Price</th>
              <th class="text-center">Quantity</th>
              <th class="text-center">Unit</th>
              <th class="text-center">Profit Per Unit</th>
              <th v-if="canManage" class="text-center">Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-if="loading">
              <td :colspan="canManage ? 8 : 7" class="empty-message">Loading products...</td>
            </tr>
            <tr v-else-if="filteredProducts.length === 0">
              <td :colspan="canManage ? 8 : 7" class="empty-message">
                <div v-if="categories.length === 0 && canManage" class="empty-categories-cta">
                  <p>No categories found. You must create a category before adding products.</p>
                  <button type="button" class="btn-primary" @click="openCategoryManagement">
                    <Layers :size="18" />
                    Create Your First Category
                  </button>
                </div>
                <span v-else>No products found.</span>
              </td>
            </tr>
            <tr v-for="product in filteredProducts" v-else :key="product.id" style="height: 38px;">
              <td class="text-center font-medium text-slate-200">{{ product.name }}</td>
              <td class="text-center">{{ product.category || 'General' }}</td>
              <td class="text-center">{{ formatMoney(product.costPrice, currency) }}</td>
              <td class="text-center">{{ formatMoney(product.sellingPrice, currency) }}</td>
              <td class="text-center">
                <span class="stock-badge" :class="getStockStatusClass(product)">
                  {{ getStockStatusText(product) }}
                </span>
              </td>
              <td class="text-center">{{ product.unit }}</td>
              <td
                class="text-center font-semibold"
                :class="profitPerUnit(product) >= 0 ? 'profit-positive' : 'profit-negative'"
              >
                {{ formatMoney(profitPerUnit(product), currency) }}
              </td>
              <td v-if="canManage" class="text-center actions">
                <button type="button" class="action-btn" title="Edit Product" @click="openEditForm(product)">
                  <Edit :size="16" />
                </button>
                <button type="button" class="action-btn" title="Adjust Stock" @click="openAdjustStock(product)">
                  <SlidersHorizontal :size="16" />
                </button>
                <button type="button" class="action-btn delete" title="Delete Product" @click="promptDeleteProduct(product)">
                  <Trash2 :size="16" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Add / Edit Product Modal -->
    <div v-if="showForm" class="modal-backdrop" @click.self="closeForm">
      <div class="modal-content">
        <div class="modal-header">
          <div class="modal-title-group">
            <h3>{{ editingProductId === null ? 'Add New Product' : 'Edit Product' }}</h3>
            <p class="modal-subtitle">Fill in the product details below to update your inventory.</p>
          </div>
          <button type="button" class="close-btn" @click="closeForm">
            <X :size="20" />
          </button>
        </div>

        <form @submit.prevent="saveProduct">
          <div v-if="error" class="inline-error" role="alert" style="margin-bottom: 1rem;">{{ error }}</div>

          <div class="product-top-section">
            <div class="product-top-fields">
              <div class="form-group">
                <label>Product Name *</label>
                <input v-model.trim="form.name" type="text" placeholder="e.g. Fresh Orange Juice 1L" required />
              </div>

              <div class="product-sku-cat-row">
                <div class="form-group">
                  <label>SKU *</label>
                  <input v-model.trim="form.sku" type="text" placeholder="SKU-XXXXXX" required />
                </div>

                <div class="form-group">
                  <label>Category *</label>
                  <select v-model="form.category" required class="modal-select">
                    <option v-for="category in categories" :key="category" :value="category">{{ category }}</option>
                  </select>
                </div>
              </div>
            </div>

            <input ref="imageInputRef" type="file" accept="image/*" style="display: none" @change="handleImageUpload" />
            <div class="image-upload-wrapper">
              <button
                type="button"
                class="image-upload-square"
                :title="form.image ? 'Change product photo' : 'Upload product photo'"
                @click="triggerImageUpload"
              >
                <img v-if="form.image" :src="form.image" alt="Product preview" class="uploaded-preview" />
                <div v-else class="upload-placeholder">
                  <ImagePlus :size="24" class="upload-icon" />
                  <span class="upload-hint">Add Photo</span>
                </div>
              </button>
              <button
                v-if="form.image"
                type="button"
                class="remove-image-btn"
                title="Remove photo"
                @click.stop="removeProductImage"
              >
                <X :size="12" />
              </button>
            </div>
          </div>

          <div class="form-grid">

            <div class="form-group span-2">
              <label>Barcode</label>
              <div class="barcode-row-inputs">
                <div class="input-with-scan-btn" :class="{ 'scanning-active': isBarcodeScanning }">
                  <input
                    ref="barcodeInputRef"
                    v-model.trim="form.barcode"
                    type="text"
                    placeholder="Scan or enter..."
                    @focus="isBarcodeScanning = true"
                    @blur="isBarcodeScanning = false"
                    @keyup.enter="handleBarcodeScanned"
                  />
                  <button
                    type="button"
                    class="scan-inside-btn"
                    :class="{ active: isBarcodeScanning }"
                    title="Click to activate scanner &amp; focus input"
                    @click="triggerBarcodeScan"
                  >
                    <Barcode :size="16" />
                  </button>
                  <div v-if="isBarcodeScanning" class="scan-laser-indicator">
                    <span class="laser-dot" />
                    <span>Scanner ready &bull; scan now...</span>
                  </div>
                </div>
                <select v-model="barcodeType" class="modal-select barcode-type-select" aria-label="Barcode type">
                  <option v-for="type in barcodeTypes" :key="type" :value="type">{{ type }}</option>
                </select>
              </div>
            </div>

            <div class="form-section span-2">
              <h4 class="section-title">Pricing Margin</h4>
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
            </div>

            <div class="form-section span-2">
              <h4 class="section-title">Stock Levels &amp; Unit</h4>
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
                  <input v-model.trim="form.unit" type="text" placeholder="pcs, kg..." required />
                </div>
              </div>
              <div class="unit-selector">
                <span class="unit-selector-label">Select Unit:</span>
                <div class="unit-pills">
                  <button
                    v-for="unit in availableUnits"
                    :key="unit"
                    type="button"
                    class="unit-pill"
                    :class="{ active: form.unit === unit }"
                    @click="form.unit = unit"
                  >
                    {{ unit }}
                  </button>
                </div>
              </div>
            </div>
          </div>

          <div class="modal-actions">
            <button type="button" class="btn-secondary" @click="closeForm">Cancel</button>
            <button type="submit" class="btn-primary" :disabled="saving">
              {{ saving ? 'Saving…' : editingProductId === null ? 'Add Product' : 'Save Changes' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Adjust Stock Modal -->
    <div v-if="adjustingProduct" class="modal-backdrop" @click.self="closeAdjustStock">
      <div class="modal-content" style="max-width: 460px;">
        <div class="modal-header">
          <div class="modal-title-group">
            <h3>Adjust Stock</h3>
            <p class="modal-subtitle">{{ adjustingProduct.name }} · Current: {{ formatQuantity(adjustingProduct.quantity) }} {{ adjustingProduct.unit }}</p>
          </div>
          <button type="button" class="close-btn" @click="closeAdjustStock">
            <X :size="20" />
          </button>
        </div>

        <form @submit.prevent="submitStockAdjustment">
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
              <input :value="`${formatQuantity(resultingStock)} ${adjustingProduct.unit}`" disabled />
            </div>

            <div class="form-group span-2">
              <label>Reason</label>
              <input v-model.trim="adjustmentReason" type="text" maxlength="140" required placeholder="Stock count, restock, damaged item…" />
            </div>
          </div>

          <div class="modal-actions">
            <button type="button" class="btn-secondary" @click="closeAdjustStock">Cancel</button>
            <button type="submit" class="btn-primary" :disabled="adjusting || resultingStock < 0 || adjustmentDelta === 0">
              {{ adjusting ? 'Updating…' : 'Apply Adjustment' }}
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Category Management Modal -->
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

    <!-- Delete Product Confirmation Modal -->
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

.products-view-switcher {
  display: inline-flex;
  padding: 2px;
  border: 1px solid var(--border-color);
  border-radius: 0.5rem;
  background-color: var(--bg-surface);
}

.products-view-switcher button {
  width: 44px;
  height: 44px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 0.375rem;
  color: var(--text-muted);
  background: transparent;
  cursor: pointer;
}

.products-view-switcher button.active {
  color: var(--text-primary);
  background: var(--bg-hover);
}

.products-view-switcher button:focus-visible,
.product-grid-actions button:focus-visible {
  outline: 2px solid var(--accent-color);
  outline-offset: 2px;
}

.products-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  align-content: start;
  gap: 0.75rem;
  flex: 1;
  min-width: 0;
  min-height: 0;
  overflow: auto;
}

.product-grid-card {
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  min-width: 0;
  padding: 1rem;
  border: 1px solid var(--border-subtle);
  border-radius: 0.75rem;
  background: var(--bg-surface);
}

.product-grid-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.65rem;
}

.product-grid-name {
  min-width: 0;
}

.product-grid-name h3 {
  margin: 0;
  color: var(--text-primary);
  font-size: 0.95rem;
  line-height: 1.35;
  overflow-wrap: anywhere;
}

.product-grid-name > span {
  display: block;
  margin-top: 0.25rem;
  color: var(--text-muted);
  font-size: 0.78rem;
}

.product-grid-header .stock-badge {
  flex: 0 0 auto;
  white-space: normal;
  text-align: center;
}

.product-grid-details {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.75rem;
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
  margin: 0.2rem 0 0;
  color: var(--text-primary);
  font-size: 0.85rem;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.product-grid-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
  padding-top: 0.75rem;
  border-top: 1px solid var(--border-subtle);
}

.product-grid-actions button {
  display: inline-flex;
  flex: 1 1 auto;
  align-items: center;
  justify-content: center;
  gap: 0.35rem;
  min-width: 44px;
  min-height: 44px;
  padding: 0.4rem 0.55rem;
  border: 1px solid var(--border-color);
  border-radius: 0.45rem;
  color: var(--text-secondary);
  background: var(--bg-sidebar);
  font: inherit;
  font-size: 0.78rem;
  cursor: pointer;
}

.product-grid-actions button:hover {
  color: var(--text-primary);
  border-color: var(--accent-color);
}

.product-grid-actions button.delete {
  color: #f87171;
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

.action-dropdown-container {
  position: relative;
}

.action-dropdown-btn {
  background: linear-gradient(135deg, var(--accent-color) 0%, #0284c7 100%);
  color: #ffffff;
  font-weight: 500;
  padding: 0.45rem 0.9rem;
  border-radius: 0.5rem;
  border: none;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  font-size: 0.85rem;
}

.chevron-icon {
  transition: transform 0.2s ease;
}

.chevron-icon.rotate {
  transform: rotate(180deg);
}

.action-dropdown-menu {
  position: absolute;
  top: calc(100% + 0.5rem);
  right: 0;
  background-color: var(--bg-sidebar);
  border: 1px solid var(--border-color);
  border-radius: 0.75rem;
  padding: 0.4rem;
  min-width: 185px;
  box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.5);
  z-index: 50;
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}

.dropdown-item {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.6rem 0.85rem;
  border-radius: 0.5rem;
  border: none;
  background: transparent;
  color: var(--text-secondary);
  font-size: 0.875rem;
  font-weight: 500;
  cursor: pointer;
  text-align: left;
  width: 100%;
}

.dropdown-item:hover {
  background-color: var(--bg-surface);
  color: var(--text-primary);
}

.item-icon.add-icon { color: #38bdf8; }
.item-icon.cat-icon { color: #a855f7; }

.table-container {
  flex-grow: 1;
  min-height: 0;
  overflow-x: auto;
  overflow-y: auto;
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
}

.modal-title-group h3 {
  margin: 0;
  font-size: 1.2rem;
  font-weight: 700;
  color: var(--text-primary);
}

.modal-subtitle {
  margin: 0.2rem 0 0;
  font-size: 0.825rem;
  color: var(--text-muted);
}

.close-btn {
  background: rgba(255, 255, 255, 0.05);
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  padding: 0.4rem;
  border-radius: 0.5rem;
  display: flex;
}

.close-btn:hover {
  color: var(--text-primary);
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

.form-section {
  background-color: rgba(15, 23, 42, 0.6);
  border: 1px solid var(--border-subtle);
  border-radius: 0.75rem;
  padding: 1rem;
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

.input-with-scan-btn input {
  padding-right: 2.5rem;
}

.input-with-scan-btn.scanning-active input {
  border-color: #22c55e;
  box-shadow: 0 0 0 2px rgba(34, 197, 94, 0.2);
}

.scan-inside-btn {
  position: absolute;
  right: 0.5rem;
  top: 0.42rem;
  background: transparent;
  border: none;
  color: var(--text-muted);
  padding: 0.25rem;
  border-radius: 0.35rem;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
}

.scan-inside-btn:hover,
.scan-inside-btn.active {
  color: #22c55e;
  background: rgba(34, 197, 94, 0.12);
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

.unit-selector {
  margin-top: 0.75rem;
  padding-top: 0.75rem;
  border-top: 1px dashed var(--border-color);
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}

.unit-selector-label {
  font-size: 0.75rem;
  color: var(--text-muted);
  font-weight: 500;
}

.unit-pills {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}

.unit-pill {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  color: #cbd5e1;
  padding: 0.25rem 0.65rem;
  border-radius: 9999px;
  font-size: 0.75rem;
  font-weight: 500;
  cursor: pointer;
}

.unit-pill.active {
  background-color: rgba(14, 165, 233, 0.2);
  border-color: var(--accent-color);
  color: #38bdf8;
  font-weight: 600;
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
}

.add-category-form {
  display: flex;
  gap: 0.5rem;
  margin-bottom: 1.25rem;
}

.add-category-form input {
  flex-grow: 1;
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 0.5rem;
  padding: 0.55rem 0.85rem;
  color: var(--text-primary);
  font-size: 0.9rem;
}

.category-list {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  max-height: 280px;
  overflow-y: auto;
}

.category-item {
  background-color: rgba(15, 23, 42, 0.6);
  border: 1px solid var(--border-subtle);
  border-radius: 0.5rem;
  padding: 0.65rem 0.9rem;
}

.view-category-mode,
.edit-category-mode {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 0.5rem;
}

.cat-name {
  font-weight: 500;
  color: #e2e8f0;
  font-size: 0.9rem;
}

.edit-cat-input {
  flex-grow: 1;
  background-color: var(--bg-surface);
  border: 1px solid var(--accent-color);
  border-radius: 0.375rem;
  padding: 0.35rem 0.65rem;
  color: var(--text-primary);
  font-size: 0.875rem;
}

.cat-actions {
  display: flex;
  gap: 0.25rem;
}

.empty-categories {
  text-align: center;
  padding: 1.5rem;
  color: var(--text-muted);
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

.product-top-section {
  display: flex;
  gap: 1rem;
  align-items: stretch;
  margin-bottom: 1rem;
}

.product-top-fields {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  flex: 1;
}

.product-sku-cat-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.75rem;
}

.image-upload-wrapper {
  position: relative;
  width: 96px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.image-upload-square {
  width: 96px;
  height: 96px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background-color: var(--bg-surface-hover, rgba(255, 255, 255, 0.04));
  border: 1px dashed var(--border-color);
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.2s ease;
  overflow: hidden;
  padding: 0;
}

.image-upload-square:hover {
  background-color: rgba(14, 165, 233, 0.08);
  border-color: var(--accent-color);
  box-shadow: 0 0 12px rgba(14, 165, 233, 0.2);
}

.upload-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
}

.upload-icon {
  color: var(--text-muted);
}

.upload-hint {
  font-size: 10px;
  font-weight: 600;
  color: var(--text-muted);
}

.uploaded-preview {
  width: 100%;
  height: 100%;
  object-fit: cover;
}

.remove-image-btn {
  position: absolute;
  top: -6px;
  right: -6px;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background-color: #ef4444;
  color: #ffffff;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 1px solid rgba(0, 0, 0, 0.3);
  cursor: pointer;
  box-shadow: 0 2px 6px rgba(0, 0, 0, 0.4);
  transition: transform 0.15s ease;
}

.remove-image-btn:hover {
  transform: scale(1.1);
}

@media (max-width: 639px) {
  .products-view-switcher {
    display: none;
  }
  .products-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .product-top-section {
    flex-direction: column-reverse;
    align-items: center;
  }
  .product-top-fields {
    width: 100%;
  }
}

</style>
