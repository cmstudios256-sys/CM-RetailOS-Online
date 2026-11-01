<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import {
  Building,
  Palette,
  Printer,
  ShoppingCart,
  Info,
  Search,
  CheckCircle2,
  Folder,
  ShieldCheck,
  RefreshCw,
  Plus,
  Edit2,
  Target,
  Eye,
  Sun,
  Moon,
  Monitor,
  X,
  Power,
  Trash2,
  Sparkles,
  Lock,
  Check,
  History,
  AlertTriangle,
  Package,
  Clock,
  Activity,
  Settings as SettingsIcon,
} from '@lucide/vue'
import { apiRequest, formatTime, syncOnlinePrefsFromServer } from '../api'
import { openPrintDocument, escapeHtml } from '../utils/reporting'
import CustomSelect from './CustomSelect.vue'

const props = defineProps<{
  shopName: string
  currency: string
  role: string
  userId: string
  userName?: string
  userEmail?: string
}>()

const emit = defineEmits<{
  'user-updated': [settings: { shopName: string; currency: string }]
}>()

type ShopSettingsResponse = {
  shopName: string
  currency: string
  address: string
  phone: string
  receiptHeader: string
  receiptFooter: string
  receiptShowPhone: boolean
}

type TeamMember = {
  id: string
  fullName: string
  email: string
  role: string
  active: boolean
  createdAt: string
}

type PrinterConfig = {
  id: string
  name: string
  protocol: string
  connectionType: string
  address: string
  paperWidth: string
  characterWidth: number
  isDefault: boolean
  capabilities: {
    supportsCut: boolean
    supportsQRCode: boolean
    supportsBarcode: boolean
    supportsStatus: boolean
    maxColumns?: number
  }
}

type PrintJob = {
  id: string
  receiptId: string
  printerName: string
  status: 'completed' | 'failed' | 'queued'
  errorMessage?: string
  errorCode?: string
  createdAt: string
}

const LOCAL_PREFS_KEY = 'cm_retailos_online_settings_prefs_v1'
const LOCAL_PRINTERS_KEY = 'cm_retailos_online_printers_v1'
const LOCAL_JOBS_KEY = 'cm_retailos_online_print_jobs_v1'
const LOCAL_STAFF_PERMS_KEY = 'cm_retailos_online_staff_perms_v1'

const appVersion = 'v3.0.8'
const activeTab = ref('business-info')
const settingsLoading = ref(true)
const isSavingConfig = ref(false)
const saveFeedback = ref<{ type: 'success' | 'error'; message: string } | null>(null)

function showToast(type: 'success' | 'error', message: string) {
  saveFeedback.value = { type, message }
  window.setTimeout(() => {
    if (saveFeedback.value?.message === message) {
      saveFeedback.value = null
    }
  }, 3500)
}

const appSettings = ref({
  shopDetails: {
    name: props.shopName || 'My Business',
    owner: props.userName || 'Shop Owner',
    phone: '',
    address: '',
    country: 'Uganda',
    currency: props.currency || 'UGX',
    logo: '',
  },
  receiptsSettings: {
    headerText: '',
    footerText: 'Thank you for shopping with us!',
    autoPrint: false,
    showLogo: true,
    showPhone: true,
    showCashier: true,
    printCopies: 1,
    printerName: '',
    paperWidth: '80mm',
  },
  applicationSettings: {
    themeMode: 'dark' as 'dark' | 'light' | 'system',
    accent: '#0ea5e9',
    typography: 'Outfit',
    autoClearCart: true,
    requirePinForDiscounts: false,
    barcodeBeep: true,
    discountPin: '',
    previewPosImagesOnHover: true,
    showPosImages: true,
    hideOutOfStock: false,
  },
  localizationSettings: {
    dateFormat: 'Aug 15, 2026',
    timeFormat: '12-hour',
  },
})

const selectedCountry = ref('Uganda')
const countryCurrencyMap: Record<string, string> = {
  Uganda: 'UGX',
  Kenya: 'KES',
  Tanzania: 'TZS',
  Rwanda: 'RWF',
  Burundi: 'BIF',
  'South Sudan': 'SSP',
  Nigeria: 'NGN',
  Ghana: 'GHS',
  'South Africa': 'ZAR',
  Zambia: 'ZMW',
  Egypt: 'EGP',
  Ethiopia: 'ETB',
  Morocco: 'MAD',
}

const countryOptions = [
  { label: 'Algeria', value: 'Algeria' },
  { label: 'Angola', value: 'Angola' },
  { label: 'Benin', value: 'Benin' },
  { label: 'Botswana', value: 'Botswana' },
  { label: 'Burkina Faso', value: 'Burkina Faso' },
  { label: 'Burundi', value: 'Burundi' },
  { label: 'Cabo Verde', value: 'Cabo Verde' },
  { label: 'Cameroon', value: 'Cameroon' },
  { label: 'Central African Republic', value: 'Central African Republic' },
  { label: 'Chad', value: 'Chad' },
  { label: 'Comoros', value: 'Comoros' },
  { label: 'Democratic Republic of the Congo', value: 'Democratic Republic of the Congo' },
  { label: 'Republic of the Congo', value: 'Republic of the Congo' },
  { label: 'Djibouti', value: 'Djibouti' },
  { label: 'Egypt', value: 'Egypt' },
  { label: 'Equatorial Guinea', value: 'Equatorial Guinea' },
  { label: 'Eritrea', value: 'Eritrea' },
  { label: 'Eswatini', value: 'Eswatini' },
  { label: 'Ethiopia', value: 'Ethiopia' },
  { label: 'Gabon', value: 'Gabon' },
  { label: 'Gambia', value: 'Gambia' },
  { label: 'Ghana', value: 'Ghana' },
  { label: 'Guinea', value: 'Guinea' },
  { label: 'Guinea-Bissau', value: 'Guinea-Bissau' },
  { label: 'Ivory Coast', value: 'Ivory Coast' },
  { label: 'Kenya', value: 'Kenya' },
  { label: 'Lesotho', value: 'Lesotho' },
  { label: 'Liberia', value: 'Liberia' },
  { label: 'Libya', value: 'Libya' },
  { label: 'Madagascar', value: 'Madagascar' },
  { label: 'Malawi', value: 'Malawi' },
  { label: 'Mali', value: 'Mali' },
  { label: 'Mauritania', value: 'Mauritania' },
  { label: 'Mauritius', value: 'Mauritius' },
  { label: 'Morocco', value: 'Morocco' },
  { label: 'Mozambique', value: 'Mozambique' },
  { label: 'Namibia', value: 'Namibia' },
  { label: 'Niger', value: 'Niger' },
  { label: 'Nigeria', value: 'Nigeria' },
  { label: 'Rwanda', value: 'Rwanda' },
  { label: 'Sao Tome and Principe', value: 'Sao Tome and Principe' },
  { label: 'Senegal', value: 'Senegal' },
  { label: 'Seychelles', value: 'Seychelles' },
  { label: 'Sierra Leone', value: 'Sierra Leone' },
  { label: 'Somalia', value: 'Somalia' },
  { label: 'South Africa', value: 'South Africa' },
  { label: 'South Sudan', value: 'South Sudan' },
  { label: 'Sudan', value: 'Sudan' },
  { label: 'Tanzania', value: 'Tanzania' },
  { label: 'Togo', value: 'Togo' },
  { label: 'Tunisia', value: 'Tunisia' },
  { label: 'Uganda', value: 'Uganda' },
  { label: 'Zambia', value: 'Zambia' },
  { label: 'Zimbabwe', value: 'Zimbabwe' },
]

const dateFormatOptions = [
  { label: 'Aug 15, 2026', value: 'Aug 15, 2026' },
  { label: '15 Aug 2026', value: '15 Aug 2026' },
  { label: '2026-08-15', value: '2026-08-15' },
  { label: '08/15/2026', value: '08/15/2026' },
]

const timeFormatOptions = [
  { label: '12-hour (02:30 PM)', value: '12-hour' },
  { label: '24-hour (14:30)', value: '24-hour' },
]

const availableAccentColors = [
  '#0ea5e9', // default blue
  '#8b5cf6', // purple
  '#ec4899', // pink
  '#f43f5e', // rose
  '#f97316', // orange
  '#10b981', // emerald
  '#14b8a6', // teal
  '#64748b', // slate
  '#06b6d4', // cyan
  '#6366f1', // indigo
  '#a855f7', // violet
  '#d946ef', // fuchsia
  '#eab308', // yellow
  '#84cc16', // lime
  '#22c55e', // green
  '#ef4444', // red
  '#0f766e', // deep teal
  '#be123c', // crimson
  '#7c3aed', // deep violet
  '#65a30d', // olive green
]

const settingsMenu = [
  {
    category: 'GENERAL & BRANDING',
    items: [
      { id: 'business-info', label: 'Business Information', description: 'Company identity & contact', icon: Building },
      { id: 'receipt-printing', label: 'Receipt & Printing', description: 'Receipt templates & printer setup', icon: Printer },
      { id: 'appearance-theme', label: 'Appearance & Theme', description: 'Dark mode & accent colors', icon: Palette },
    ],
  },
  {
    category: 'OPERATIONS & POS',
    items: [
      { id: 'pos-config', label: 'POS Configuration', description: 'POS behavior & defaults', icon: ShoppingCart },
    ],
  },
  {
    category: 'SYSTEM',
    items: [
      { id: 'about', label: 'About', description: 'Product information', icon: Info },
    ],
  },
]

const logoInput = ref<HTMLInputElement | null>(null)

const handleLogoUpload = (event: Event) => {
  const target = event.target as HTMLInputElement
  const file = target.files?.[0]
  if (!file) return

  if (file.size > 2 * 1024 * 1024) {
    showToast('error', 'Logo file size must be less than 2MB')
    return
  }

  const reader = new FileReader()
  reader.onload = (e) => {
    if (e.target?.result) {
      appSettings.value.shopDetails.logo = e.target.result as string
      persistLocalPrefs()
    }
  }
  reader.readAsDataURL(file)
}

function applyThemeModeDom(mode: 'dark' | 'light' | 'system') {
  const root = document.documentElement
  if (mode === 'system') {
    const prefersLight = window.matchMedia('(prefers-color-scheme: light)').matches
    if (prefersLight) {
      root.setAttribute('data-theme', 'light')
    } else {
      root.removeAttribute('data-theme')
    }
  } else if (mode === 'light') {
    root.setAttribute('data-theme', 'light')
  } else {
    root.removeAttribute('data-theme')
  }
}

function applyAccentColorDom(color: string) {
  if (!color) return
  document.documentElement.style.setProperty('--accent-color', color)
}

function applyTypographyDom(font: string) {
  if (!font) return
  const stack = font === 'System Default'
    ? "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
    : `'${font}', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif`
  document.documentElement.style.setProperty('--app-font-family', stack)
}

function loadLocalPrefs() {
  try {
    const raw = localStorage.getItem(LOCAL_PREFS_KEY)
    if (!raw) return
    const parsed = JSON.parse(raw)
    if (parsed.owner) appSettings.value.shopDetails.owner = parsed.owner
    if (parsed.country) {
      appSettings.value.shopDetails.country = parsed.country
      selectedCountry.value = parsed.country
    }
    if (parsed.logo !== undefined) appSettings.value.shopDetails.logo = parsed.logo
    if (parsed.receiptsSettings) {
      Object.assign(appSettings.value.receiptsSettings, parsed.receiptsSettings)
    }
    if (parsed.applicationSettings) {
      Object.assign(appSettings.value.applicationSettings, parsed.applicationSettings)
    }
    if (parsed.localizationSettings) {
      Object.assign(appSettings.value.localizationSettings, parsed.localizationSettings)
    }
    applyThemeModeDom(appSettings.value.applicationSettings.themeMode)
    applyAccentColorDom(appSettings.value.applicationSettings.accent)
    applyTypographyDom(appSettings.value.applicationSettings.typography)
  } catch {
    // ignore invalid localStorage state
  }
}

function persistLocalPrefs() {
  const prefsPayload = {
    owner: appSettings.value.shopDetails.owner,
    country: selectedCountry.value,
    logo: appSettings.value.shopDetails.logo,
    receiptsSettings: {
      autoPrint: appSettings.value.receiptsSettings.autoPrint,
      showLogo: appSettings.value.receiptsSettings.showLogo,
      showCashier: appSettings.value.receiptsSettings.showCashier,
      printCopies: appSettings.value.receiptsSettings.printCopies,
      printerName: appSettings.value.receiptsSettings.printerName,
      paperWidth: appSettings.value.receiptsSettings.paperWidth,
    },
    applicationSettings: appSettings.value.applicationSettings,
    localizationSettings: appSettings.value.localizationSettings,
  }
  try {
    localStorage.setItem(LOCAL_PREFS_KEY, JSON.stringify(prefsPayload))
    window.dispatchEvent(new CustomEvent('online-prefs-updated'))
  } catch {
    // ignore quota errors
  }
  if (props.role === 'owner' || props.role === 'manager') {
    void apiRequest('/settings/preferences', {
      method: 'PUT',
      body: { preferences: prefsPayload },
    }).catch(() => undefined)
  }
}

watch(selectedCountry, (country) => {
  appSettings.value.shopDetails.country = country
  const mappedCurrency = countryCurrencyMap[country]
  if (mappedCurrency) {
    appSettings.value.shopDetails.currency = mappedCurrency
  }
})

async function loadSettings() {
  settingsLoading.value = true
  loadLocalPrefs()
  await syncOnlinePrefsFromServer(props.userId)
  loadLocalPrefs()
  loadStaffPermsMap()
  try {
    const result = await apiRequest<{ settings: ShopSettingsResponse }>('/settings')
    appSettings.value.shopDetails.name = result.settings.shopName || props.shopName || 'My Business'
    appSettings.value.shopDetails.currency = result.settings.currency || props.currency || 'UGX'
    appSettings.value.shopDetails.address = result.settings.address || ''
    appSettings.value.shopDetails.phone = result.settings.phone || ''
    appSettings.value.receiptsSettings.headerText = result.settings.receiptHeader || ''
    appSettings.value.receiptsSettings.footerText = result.settings.receiptFooter || 'Thank you for shopping with us!'
    appSettings.value.receiptsSettings.showPhone = result.settings.receiptShowPhone
  } catch (reason) {
    showToast('error', reason instanceof Error ? reason.message : 'Failed to load settings.')
  } finally {
    settingsLoading.value = false
  }
}

async function saveConfigurations() {
  isSavingConfig.value = true
  try {
    appSettings.value.shopDetails.country = selectedCountry.value
    persistLocalPrefs()
    const payload: ShopSettingsResponse = {
      shopName: appSettings.value.shopDetails.name.trim() || 'My Business',
      currency: appSettings.value.shopDetails.currency.trim().toUpperCase() || 'UGX',
      address: appSettings.value.shopDetails.address.trim(),
      phone: appSettings.value.shopDetails.phone.trim(),
      receiptHeader: appSettings.value.receiptsSettings.headerText.trim(),
      receiptFooter: appSettings.value.receiptsSettings.footerText.trim(),
      receiptShowPhone: appSettings.value.receiptsSettings.showPhone,
    }
    const result = await apiRequest<{ settings: ShopSettingsResponse }>('/settings', {
      method: 'PUT',
      body: payload,
    })
    appSettings.value.shopDetails.name = result.settings.shopName
    appSettings.value.shopDetails.currency = result.settings.currency
    emit('user-updated', {
      shopName: result.settings.shopName,
      currency: result.settings.currency,
    })
    showToast('success', 'Configurations saved successfully.')
  } catch (reason) {
    showToast('error', reason instanceof Error ? reason.message : 'Failed to save configurations.')
  } finally {
    isSavingConfig.value = false
  }
}

const handleReceiptSettingChange = () => {
  persistLocalPrefs()
}

const handleGeneralAppSettingChange = () => {
  applyTypographyDom(appSettings.value.applicationSettings.typography)
  persistLocalPrefs()
}

const handleLocalizationAppSettingChange = () => {
  persistLocalPrefs()
}

const handleThemeChange = (mode: 'dark' | 'light' | 'system') => {
  appSettings.value.applicationSettings.themeMode = mode
  applyThemeModeDom(mode)
  persistLocalPrefs()
}

const handleAccentChange = (color: string) => {
  appSettings.value.applicationSettings.accent = color
  applyAccentColorDom(color)
  persistLocalPrefs()
}

// --- POS Receipt Printers (Browser & Network Spooler Configuration) ---
const configuredPrinters = ref<PrinterConfig[]>([])
const printerStatuses = ref<Record<string, { isOnline: boolean; stateDescription: string }>>({})
const discoveredWindowsPrinters = ref<{ name: string; port: string; connectionType: string }[]>([])
const isScanningPrinters = ref(false)
const isScanningDiscovery = ref(false)
const showAddPrinterModal = ref(false)
const isEditingPrinter = ref(false)
const showJobsModal = ref(false)
const printJobs = ref<PrintJob[]>([])
const isLoadingJobs = ref(false)
const isTestingPrinterId = ref<string | null>(null)
const isRetryingJobId = ref<string | null>(null)
const showDeletePrinterModal = ref(false)
const printerToDelete = ref<PrinterConfig | null>(null)

const selectedPaperWidth = computed(() => {
  const defaultPrinter = configuredPrinters.value.find(p => p.isDefault) || configuredPrinters.value[0]
  return defaultPrinter?.paperWidth || appSettings.value.receiptsSettings.paperWidth || '80mm'
})

const printerForm = ref<PrinterConfig>({
  id: '',
  name: '',
  protocol: 'escpos',
  connectionType: 'winraw',
  address: '',
  paperWidth: '80mm',
  characterWidth: 48,
  isDefault: false,
  capabilities: {
    supportsCut: true,
    supportsQRCode: true,
    supportsBarcode: true,
    supportsStatus: true,
  },
})

function loadPrinters() {
  isScanningPrinters.value = true
  try {
    const raw = localStorage.getItem(LOCAL_PRINTERS_KEY)
    configuredPrinters.value = raw ? JSON.parse(raw) : []
    for (const p of configuredPrinters.value) {
      printerStatuses.value[p.id] = { isOnline: true, stateDescription: 'Ready' }
    }
    const rawJobs = localStorage.getItem(LOCAL_JOBS_KEY)
    printJobs.value = rawJobs ? JSON.parse(rawJobs) : []
  } catch {
    configuredPrinters.value = []
  } finally {
    window.setTimeout(() => {
      isScanningPrinters.value = false
    }, 250)
  }
}

function savePrintersToStorage() {
  try {
    localStorage.setItem(LOCAL_PRINTERS_KEY, JSON.stringify(configuredPrinters.value))
    localStorage.setItem(LOCAL_JOBS_KEY, JSON.stringify(printJobs.value))
  } catch {
    // ignore storage errors
  }
}

function scanDiscoveredPrinters() {
  isScanningDiscovery.value = true
  window.setTimeout(() => {
    discoveredWindowsPrinters.value = [
      { name: 'System Browser Print Dialog (POS-80)', port: 'BROWSER_PRINT', connectionType: 'winraw' },
      { name: 'POS-58 Thermal Receipt Printer', port: 'USB001', connectionType: 'winraw' },
    ]
    isScanningDiscovery.value = false
  }, 350)
}

function selectDiscoveredPrinter(dp: { name: string; port: string; connectionType: string }) {
  if (!printerForm.value.name) {
    printerForm.value.name = dp.name
  }
  printerForm.value.address = dp.port || dp.name
}

function openAddPrinter() {
  isEditingPrinter.value = false
  printerForm.value = {
    id: `prn-${Date.now()}`,
    name: '',
    protocol: 'escpos',
    connectionType: 'winraw',
    address: '',
    paperWidth: '80mm',
    characterWidth: 48,
    isDefault: configuredPrinters.value.length === 0,
    capabilities: {
      supportsCut: true,
      supportsQRCode: true,
      supportsBarcode: true,
      supportsStatus: true,
    },
  }
  showAddPrinterModal.value = true
}

function openEditPrinter(printer: PrinterConfig) {
  isEditingPrinter.value = true
  printerForm.value = JSON.parse(JSON.stringify(printer))
  showAddPrinterModal.value = true
}

function handleSavePrinterForm() {
  const name = printerForm.value.name.trim()
  if (!name) {
    showToast('error', 'Please enter a printer display name.')
    return
  }
  const entry: PrinterConfig = {
    ...printerForm.value,
    name,
    address: printerForm.value.address.trim() || name,
    characterWidth: printerForm.value.paperWidth === '58mm' ? 32 : 48,
    capabilities: {
      ...printerForm.value.capabilities,
      maxColumns: printerForm.value.paperWidth === '58mm' ? 32 : 48,
    },
  }
  if (entry.isDefault) {
    configuredPrinters.value.forEach(p => { p.isDefault = false })
  }
  const idx = configuredPrinters.value.findIndex(p => p.id === entry.id)
  if (idx >= 0) {
    configuredPrinters.value[idx] = entry
  } else {
    configuredPrinters.value.push(entry)
  }
  printerStatuses.value[entry.id] = { isOnline: true, stateDescription: 'Ready' }
  appSettings.value.receiptsSettings.paperWidth = entry.paperWidth
  savePrintersToStorage()
  persistLocalPrefs()
  showAddPrinterModal.value = false
  showToast('success', isEditingPrinter.value ? 'Printer updated.' : 'Printer added.')
}

function handleSetDefault(printer: PrinterConfig) {
  configuredPrinters.value.forEach(p => {
    p.isDefault = p.id === printer.id
  })
  appSettings.value.receiptsSettings.paperWidth = printer.paperWidth
  savePrintersToStorage()
  persistLocalPrefs()
  showToast('success', `${printer.name} set as default POS printer.`)
}

function handleDeletePrinter(printer: PrinterConfig) {
  printerToDelete.value = printer
  showDeletePrinterModal.value = true
}

function confirmDeletePrinter() {
  if (!printerToDelete.value) return
  const removedName = printerToDelete.value.name
  configuredPrinters.value = configuredPrinters.value.filter(p => p.id !== printerToDelete.value?.id)
  if (configuredPrinters.value.length > 0 && !configuredPrinters.value.some(p => p.isDefault)) {
    configuredPrinters.value[0].isDefault = true
  }
  savePrintersToStorage()
  showDeletePrinterModal.value = false
  printerToDelete.value = null
  showToast('success', `Removed ${removedName}.`)
}

function handleTestPrinterCard(printer: PrinterConfig) {
  isTestingPrinterId.value = printer.id
  const widthMm = printer.paperWidth === '58mm' ? '58mm' : '80mm'
  const shopTitle = escapeHtml(appSettings.value.shopDetails.name || 'MY STORE')
  const address = escapeHtml(appSettings.value.shopDetails.address || 'Store Address')
  openPrintDocument(
    `Test Receipt - ${printer.name}`,
    `<div style="width:${widthMm};margin:0 auto;font-family:'Courier New',monospace;font-size:12px;color:#000;">
      <h2 style="text-align:center;margin:0 0 4px;">${shopTitle}</h2>
      <p style="text-align:center;margin:0 0 8px;">${address}</p>
      <p style="text-align:center;border-top:1px dashed #000;border-bottom:1px dashed #000;padding:6px 0;">PRINTER TEST PAGE (${escapeHtml(printer.name)})</p>
      <p style="margin:6px 0;">Connection: ${escapeHtml(printer.connectionType.toUpperCase())}</p>
      <p style="margin:6px 0;">Paper Width: ${escapeHtml(printer.paperWidth)}</p>
      <p style="text-align:center;margin-top:12px;">${escapeHtml(appSettings.value.receiptsSettings.footerText || 'Thank you!')}</p>
    </div>`,
  )
  printJobs.value.unshift({
    id: `job-${Date.now()}`,
    receiptId: 'TEST',
    printerName: printer.name,
    status: 'completed',
    createdAt: new Date().toISOString(),
  })
  savePrintersToStorage()
  window.setTimeout(() => {
    isTestingPrinterId.value = null
  }, 400)
}

function openJobsModal() {
  showJobsModal.value = true
}

function refreshJobs() {
  isLoadingJobs.value = true
  window.setTimeout(() => {
    isLoadingJobs.value = false
  }, 250)
}

function handleRetryJob(jobId: string) {
  isRetryingJobId.value = jobId
  window.setTimeout(() => {
    const job = printJobs.value.find(j => j.id === jobId)
    if (job) job.status = 'completed'
    savePrintersToStorage()
    isRetryingJobId.value = null
  }, 350)
}

// --- Users & Staff Management (Connected to Online Team Members API) ---
const teamMembers = ref<TeamMember[]>([])
const staffPermsMap = ref<Record<string, string[]>>({})
const userSearchTerm = ref('')
const showAddUserModal = ref(false)
const addUserLoading = ref(false)
const addUserError = ref('')
const newUser = ref({
  fullName: '',
  username: '',
  role: 'Cashier',
  password: '',
  permissions: [] as string[],
})

const addUserRoleOptions = [
  { label: 'Manager', value: 'Manager' },
  { label: 'Cashier', value: 'Cashier' },
]

const allPermissions = computed(() => [
  {
    id: 'page_pos',
    label: 'Point of Sale (POS)',
    desc: 'Access the checkout and sales terminal',
    icon: ShoppingCart,
    isAvailable: true,
    isProFeature: false,
  },
  {
    id: 'page_inventory',
    label: 'Inventory Management',
    desc: 'Access products, stock levels, and categories',
    icon: Package,
    isAvailable: true,
    isProFeature: false,
  },
  {
    id: 'page_bi',
    label: 'Business Intelligence',
    desc: 'Access charts, profit analytics, and sales intelligence',
    icon: Activity,
    isAvailable: true,
    isProFeature: false,
  },
  {
    id: 'page_sales_history',
    label: 'Sales History',
    desc: 'View past transactions, daily summaries, and receipts',
    icon: Clock,
    isAvailable: true,
    isProFeature: false,
  },
  {
    id: 'page_audit',
    label: 'Audit Logs',
    desc: 'View system events, audit trails, and user actions',
    icon: ShieldCheck,
    isAvailable: true,
    isProFeature: false,
  },
  {
    id: 'page_settings',
    label: 'Settings & Configuration',
    desc: 'Access app preferences, staff, and backups',
    icon: SettingsIcon,
    isAvailable: true,
    isProFeature: false,
  },
])

watch(() => newUser.value.role, (newRole) => {
  if (newRole === 'Manager') {
    newUser.value.permissions = allPermissions.value.map(p => p.id)
  } else {
    newUser.value.permissions = ['page_pos', 'page_sales_history']
  }
}, { immediate: true })

function loadStaffPermsMap() {
  try {
    const raw = localStorage.getItem(LOCAL_STAFF_PERMS_KEY)
    if (raw) staffPermsMap.value = JSON.parse(raw)
  } catch {
    staffPermsMap.value = {}
  }
}

function saveStaffPermsMap() {
  try {
    localStorage.setItem(LOCAL_STAFF_PERMS_KEY, JSON.stringify(staffPermsMap.value))
    window.dispatchEvent(new CustomEvent('online-prefs-updated'))
  } catch {
    // ignore
  }
}

function mapRoleLabel(apiRole: string): string {
  const r = apiRole.toLowerCase()
  if (r === 'owner' || r === 'admin' || r === 'administrator') return 'Administrator'
  if (r === 'manager' || r === 'store manager') return 'Store Manager'
  return 'Cashier'
}

const systemUsers = computed(() =>
  teamMembers.value.map(member => ({
    id: member.id,
    fullName: member.fullName,
    username: member.email,
    role: mapRoleLabel(member.role),
    rawRole: member.role,
    isActive: member.active,
    permissions: staffPermsMap.value[member.id] || (member.role === 'cashier' ? ['page_pos', 'page_sales_history'] : allPermissions.value.map(p => p.id)),
  })),
)

const filteredSystemUsers = computed(() => {
  if (!userSearchTerm.value.trim()) return systemUsers.value
  const term = userSearchTerm.value.toLowerCase()
  return systemUsers.value.filter(
    u =>
      u.fullName.toLowerCase().includes(term) ||
      u.username.toLowerCase().includes(term) ||
      u.role.toLowerCase().includes(term),
  )
})

async function loadUsers() {
  loadStaffPermsMap()
  try {
    const result = await apiRequest<{ members: TeamMember[] }>('/team-members')
    teamMembers.value = result.members || []
  } catch {
    teamMembers.value = []
  }
}

function openAddUserModal() {
  addUserError.value = ''
  newUser.value = {
    fullName: '',
    username: '',
    role: 'Cashier',
    password: '',
    permissions: ['page_pos', 'page_sales_history'],
  }
  showAddUserModal.value = true
}

async function handleAddUser() {
  addUserError.value = ''
  const rawInput = newUser.value.username.trim()
  if (!rawInput) {
    addUserError.value = 'Please enter a username or email.'
    return
  }
  if (!newUser.value.password || newUser.value.password.length < 8) {
    addUserError.value = 'Password must be at least 8 characters for online sign-in.'
    return
  }
  const email = rawInput.includes('@') ? rawInput.toLowerCase() : `${rawInput.toLowerCase().replace(/[^a-z0-9._-]/g, '')}@cmretail.local`
  const fullName = newUser.value.fullName.trim() || rawInput
  const apiRole = newUser.value.role === 'Manager' ? 'manager' : 'cashier'

  addUserLoading.value = true
  try {
    const created = await apiRequest<{ member: TeamMember }>('/team-members', {
      method: 'POST',
      body: {
        fullName,
        email,
        role: apiRole,
        password: newUser.value.password,
      },
    })
    if (created?.member?.id) {
      staffPermsMap.value[created.member.id] = [...newUser.value.permissions]
      saveStaffPermsMap()
      await apiRequest(`/team-members/${created.member.id}/permissions`, {
        method: 'PUT',
        body: {
          fullName,
          permissions: newUser.value.permissions,
        },
      }).catch(() => undefined)
    }
    showAddUserModal.value = false
    await loadUsers()
    showToast('success', `Account for ${fullName} has been created.`)
  } catch (reason) {
    addUserError.value = reason instanceof Error ? reason.message : 'Could not create staff account.'
  } finally {
    addUserLoading.value = false
  }
}

const showEditPermissionsModal = ref(false)
const editingUser = ref<{ id: string; fullName: string; username: string; role: string; permissions: string[] }>({
  id: '',
  fullName: '',
  username: '',
  role: 'Cashier',
  permissions: [],
})
const editUserLoading = ref(false)
const editUserError = ref('')

function openEditPermissionsModal(user: { id: string; fullName: string; username: string; role: string; permissions: string[] }) {
  editingUser.value = {
    id: user.id,
    fullName: user.fullName,
    username: user.fullName || user.username,
    role: user.role,
    permissions: [...user.permissions],
  }
  editUserError.value = ''
  showEditPermissionsModal.value = true
}

async function handleUpdateUser() {
  editUserError.value = ''
  if (!editingUser.value.username.trim()) {
    editUserError.value = 'Please enter a staff name.'
    return
  }
  editUserLoading.value = true
  try {
    await apiRequest(`/team-members/${editingUser.value.id}/permissions`, {
      method: 'PUT',
      body: {
        fullName: editingUser.value.username.trim(),
        permissions: editingUser.value.permissions,
      },
    })
    staffPermsMap.value[editingUser.value.id] = [...editingUser.value.permissions]
    saveStaffPermsMap()
    await loadUsers()
    showEditPermissionsModal.value = false
    showToast('success', `Permissions updated for "${editingUser.value.username.trim()}".`)
  } catch (reason) {
    editUserError.value = reason instanceof Error ? reason.message : 'Could not update staff permissions.'
  } finally {
    editUserLoading.value = false
  }
}

async function handleToggleUserStatus(id: string, name: string, currentStatus: boolean) {
  try {
    await apiRequest(`/team-members/${id}/status`, {
      method: 'PATCH',
      body: { active: !currentStatus },
    })
    await loadUsers()
    showToast('success', `${name}'s account has been ${currentStatus ? 'deactivated' : 'activated'}.`)
  } catch (reason) {
    showToast('error', reason instanceof Error ? reason.message : 'Could not update staff status.')
  }
}

onMounted(() => {
  void loadSettings()
  void loadUsers()
  loadPrinters()

  const params = new URLSearchParams(window.location.search)
  const gErr = params.get('google_error')
  if (gErr) {
    if (gErr === 'google_account_conflict') {
      showToast('error', 'This Google account is already linked to another business.')
    } else if (gErr === 'google_sign_in_cancelled') {
      showToast('success', 'Google connection was cancelled.')
    } else {
      showToast('error', 'Google authentication could not be completed. Please try again.')
    }
    window.history.replaceState({}, document.title, window.location.pathname)
  }
})
</script>

<template>
  <div class="settings-page">
    <!-- Sidebar -->
    <aside class="settings-sidebar">
      <nav class="sidebar-menu">
        <div v-for="category in settingsMenu" :key="category.category" class="menu-category">
          <h3 class="category-title">{{ category.category }}</h3>
          <div class="menu-group-card">
            <button
              v-for="item in category.items"
              :key="item.id"
              class="menu-item"
              :class="{ active: activeTab === item.id }"
              @click="activeTab = item.id"
            >
              <component :is="item.icon" class="item-icon" :size="20" />
              <div class="item-text">
                <span class="item-label">{{ item.label }}</span>
                <span class="item-desc">{{ item.description }}</span>
              </div>
            </button>
          </div>
        </div>
      </nav>

      <div v-if="saveFeedback" class="sidebar-feedback" :class="saveFeedback.type">
        {{ saveFeedback.message }}
      </div>

      <div style="padding: 16px; margin-top: auto;">
        <button
          class="btn btn-primary"
          style="width: 100%; display: flex; justify-content: center; padding: 12px;"
          @click="saveConfigurations"
          :disabled="isSavingConfig || settingsLoading"
        >
          <span v-if="isSavingConfig">Saving...</span>
          <span v-else>Save Configurations</span>
        </button>
      </div>
    </aside>

    <!-- Main Content -->
    <main class="settings-content">
      <!-- Business Information -->
      <template v-if="activeTab === 'business-info'">
        <div class="form-grid business-info-grid">
          <div class="form-card full-width business-unified-card">
            <div class="card-header">
              <h3>Company Profile</h3>
              <p>Manage your business identity, contact details, and location.</p>
            </div>

            <!-- Logo Section inside Unified Card -->
            <div class="business-unified-section logo-section">
              <div class="card-section-header">
                <p>This logo will appear on receipts and reports.</p>
              </div>
              <div class="logo-upload-area">
                <input
                  type="file"
                  accept="image/png, image/jpeg"
                  style="display: none;"
                  ref="logoInput"
                  @change="handleLogoUpload"
                />
                <div
                  class="current-logo"
                  style="display: flex; align-items: center; justify-content: center; font-weight: 700; color: var(--text-muted); font-size: 1.2rem; letter-spacing: 0.1em; overflow: hidden;"
                >
                  <img
                    v-if="appSettings.shopDetails.logo"
                    :src="appSettings.shopDetails.logo"
                    style="width: 100%; height: 100%; object-fit: contain;"
                  />
                  <span v-else>LOGO</span>
                </div>
                <div class="upload-actions">
                  <button type="button" class="btn btn-outline" @click="logoInput?.click()">
                    <Folder :size="14" /> Browse Files
                  </button>
                  <button
                    v-if="appSettings.shopDetails.logo"
                    type="button"
                    class="btn btn-outline"
                    style="border-color: #ef4444; color: #ef4444;"
                    @click="appSettings.shopDetails.logo = ''; persistLocalPrefs()"
                  >
                    <X :size="14" /> Remove
                  </button>
                  <span class="upload-hint">Recommended size: 256x256px. PNG or JPG.</span>
                </div>
              </div>
            </div>

            <div class="business-details-columns">
              <section class="business-details-section">
                <div class="card-section-header">
                  <h4>General &amp; Contact Details</h4>
                </div>
                <div class="card-body">
                  <div class="input-group full-width">
                    <label>Business Name</label>
                    <input
                      type="text"
                      v-model="appSettings.shopDetails.name"
                      placeholder="Enter business name"
                    />
                  </div>
                  <div class="input-group full-width">
                    <label>Owner Names</label>
                    <input
                      type="text"
                      v-model="appSettings.shopDetails.owner"
                      placeholder="Enter owner's name"
                    />
                  </div>
                  <div class="input-group full-width">
                    <label>Phone Number</label>
                    <input
                      type="tel"
                      v-model="appSettings.shopDetails.phone"
                      placeholder="Phone number"
                    />
                  </div>
                </div>
              </section>

              <section class="business-details-section location-details-section">
                <div class="card-section-header">
                  <h4>Location</h4>
                  <p>Where your business operates.</p>
                </div>
                <div class="card-body">
                  <div class="input-group full-width">
                    <label>Country</label>
                    <CustomSelect v-model="selectedCountry" :options="countryOptions" :fullWidth="true" />
                  </div>

                  <div class="input-group full-width">
                    <label>Physical Address</label>
                    <input
                      type="text"
                      v-model="appSettings.shopDetails.address"
                      placeholder="Full street address"
                    />
                  </div>
                </div>
              </section>
            </div>
          </div>
        </div>
      </template>

      <!-- Receipt & Printing -->
      <template v-else-if="activeTab === 'receipt-printing'">
        <div class="receipt-tab-container" style="display: flex; flex-direction: column; gap: 16px; width: 100%; flex-shrink: 0;">
          <!-- POS Thermal Hardware Printers (Full Width) -->
          <div class="form-card full-width pos-printers-card" style="width: 100%; box-sizing: border-box;">
            <div
              class="card-header"
              style="display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 12px; padding: 14px 18px; border-bottom: 1px solid var(--border-subtle);"
            >
              <div style="min-width: 200px; flex: 1 1 260px;">
                <h3 style="margin: 0; font-size: 15px; font-weight: 700; display: flex; align-items: center; gap: 8px; flex-wrap: wrap;">
                  <Printer :size="18" style="color: var(--accent-color); flex-shrink: 0;" />
                  <span>POS Receipt Printers</span>
                  <span
                    style="font-size: 11px; padding: 2px 8px; border-radius: 10px; background: rgba(14, 165, 233, 0.15); color: var(--accent-color); font-weight: 600; white-space: nowrap;"
                  >
                    {{ configuredPrinters.length }} configured
                  </span>
                </h3>
                <span style="font-size: 12px; color: var(--text-muted); display: block; margin-top: 2px;">
                  ESC/POS direct printing via Windows RAW Spooler, Network TCP/IP, or USB
                </span>
              </div>
              <div style="display: flex; align-items: center; gap: 8px; flex-wrap: wrap;">
                <button
                  type="button"
                  class="btn btn-outline"
                  style="padding: 6px 12px; font-size: 12px; display: flex; align-items: center; gap: 5px; white-space: nowrap;"
                  @click="openJobsModal"
                  title="View print history and queued jobs"
                >
                  <History :size="14" />
                  <span>Queue &amp; Logs</span>
                </button>
                <button
                  type="button"
                  class="btn btn-outline"
                  style="padding: 6px 10px; font-size: 12px; display: flex; align-items: center; gap: 5px; white-space: nowrap;"
                  @click="loadPrinters"
                  :title="isScanningPrinters ? 'Scanning...' : 'Scan / Refresh printers'"
                  :disabled="isScanningPrinters"
                >
                  <RefreshCw :size="14" :class="{ 'spin-icon': isScanningPrinters }" />
                </button>
                <button
                  type="button"
                  class="btn btn-primary"
                  style="padding: 6px 14px; font-size: 12px; display: flex; align-items: center; gap: 5px; white-space: nowrap;"
                  @click="openAddPrinter"
                >
                  <Plus :size="14" />
                  <span>Add Printer</span>
                </button>
              </div>
            </div>

            <div class="card-body" style="padding: 16px; display: block;">
              <!-- Empty State -->
              <div
                v-if="configuredPrinters.length === 0"
                style="padding: 24px 16px; text-align: center; background: var(--bg-surface); border-radius: 10px; border: 1px dashed var(--border-color);"
              >
                <Printer :size="32" style="color: var(--text-muted); margin: 0 auto 10px auto; opacity: 0.7;" />
                <h4 style="margin: 0 0 6px 0; font-size: 14px; font-weight: 600; color: var(--text-primary);">No POS Printers Configured</h4>
                <p style="margin: 0 0 14px 0; font-size: 12px; color: var(--text-muted); max-width: 400px; margin-inline: auto;">
                  Add your Windows named receipt printer or Ethernet/WiFi thermal printer to enable high-speed direct printing.
                </p>
                <div style="display: flex; justify-content: center; gap: 10px;">
                  <button type="button" class="btn btn-primary" style="padding: 8px 18px; font-size: 12px;" @click="openAddPrinter">
                    <Plus :size="14" style="margin-right: 4px;" /> Add Your First Printer
                  </button>
                </div>
              </div>

              <!-- Configured Printers List -->
              <div v-else style="display: flex; flex-direction: column; gap: 10px;">
                <div
                  v-for="printer in configuredPrinters"
                  :key="printer.id"
                  style="background: rgba(255, 255, 255, 0.03); border: 1px solid var(--border-subtle); border-radius: 8px; padding: 12px 14px; display: flex; flex-direction: column; gap: 10px; transition: border-color 0.2s;"
                  :style="printer.isDefault ? 'border-color: var(--accent-color); background: rgba(14, 165, 233, 0.03);' : ''"
                >
                  <div style="display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 10px;">
                    <div style="display: flex; align-items: center; gap: 10px; min-width: 0; flex: 1 1 260px;">
                      <span
                        style="width: 10px; height: 10px; border-radius: 50%; display: inline-block; flex-shrink: 0;"
                        :style="{
                          backgroundColor: printerStatuses[printer.id]?.isOnline !== false ? '#10b981' : '#ef4444',
                          boxShadow: printerStatuses[printer.id]?.isOnline !== false ? '0 0 8px rgba(16, 185, 129, 0.5)' : 'none',
                        }"
                      ></span>

                      <div style="min-width: 0; flex: 1;">
                        <div style="display: flex; align-items: center; gap: 8px; flex-wrap: wrap;">
                          <span style="font-weight: 700; font-size: 14px; color: var(--text-primary); word-break: break-word;">{{ printer.name }}</span>
                          <span
                            v-if="printer.isDefault"
                            style="font-size: 10px; font-weight: 700; letter-spacing: 0.5px; padding: 2px 8px; border-radius: 12px; background: rgba(16, 185, 129, 0.15); color: #10b981; border: 1px solid rgba(16, 185, 129, 0.3); white-space: nowrap; flex-shrink: 0;"
                          >
                            DEFAULT POS
                          </span>
                        </div>
                        <div style="font-size: 11px; color: var(--text-muted); display: flex; align-items: center; gap: 6px; margin-top: 3px; flex-wrap: wrap;">
                          <span style="text-transform: uppercase; font-weight: 600; padding: 1px 6px; border-radius: 4px; background: rgba(255,255,255,0.06); color: var(--text-secondary); white-space: nowrap;">
                            {{ printer.connectionType }}
                          </span>
                          <span style="word-break: break-all;">{{ printer.address || printer.name }}</span>
                          <span aria-hidden="true">|</span>
                          <span style="font-weight: 600; color: var(--accent-color); white-space: nowrap;">{{ printer.paperWidth || '80mm' }}</span>
                        </div>
                      </div>
                    </div>

                    <div style="display: flex; align-items: center; gap: 6px; flex-wrap: wrap; flex-shrink: 0;">
                      <button
                        type="button"
                        class="btn btn-outline"
                        style="padding: 5px 10px; font-size: 11px; display: flex; align-items: center; gap: 4px; white-space: nowrap;"
                        @click="handleTestPrinterCard(printer)"
                        :disabled="isTestingPrinterId === printer.id"
                      >
                        <Printer :size="13" />
                        <span>{{ isTestingPrinterId === printer.id ? 'Printing...' : 'Test' }}</span>
                      </button>

                      <button
                        v-if="!printer.isDefault"
                        type="button"
                        class="btn btn-outline"
                        style="padding: 5px 10px; font-size: 11px; white-space: nowrap;"
                        @click="handleSetDefault(printer)"
                      >
                        Set Default
                      </button>

                      <button type="button" class="icon-btn" style="padding: 5px;" @click="openEditPrinter(printer)" title="Edit printer configuration">
                        <Edit2 :size="14" />
                      </button>

                      <button type="button" class="icon-btn" style="padding: 5px; color: #ef4444;" @click="handleDeletePrinter(printer)" title="Remove printer">
                        <Trash2 :size="14" />
                      </button>
                    </div>
                  </div>

                  <div style="display: flex; align-items: center; gap: 6px; flex-wrap: wrap; font-size: 10px; color: var(--text-muted); padding-top: 6px; border-top: 1px solid rgba(255,255,255,0.04);">
                    <span v-if="printer.capabilities?.supportsCut" style="padding: 1px 6px; background: rgba(255,255,255,0.04); border-radius: 4px;">Auto-Cut</span>
                    <span v-if="printer.capabilities?.supportsQRCode" style="padding: 1px 6px; background: rgba(255,255,255,0.04); border-radius: 4px;">QR Code</span>
                    <span v-if="printer.capabilities?.supportsBarcode" style="padding: 1px 6px; background: rgba(255,255,255,0.04); border-radius: 4px;">1D Barcode</span>
                    <span style="margin-left: auto; color: var(--text-muted);">Width: {{ printer.capabilities?.maxColumns || (printer.paperWidth === '58mm' ? 32 : 48) }} cols</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <!-- Bottom Section: Receipt Config (Left) + Live Preview (Right) -->
          <div class="form-grid receipt-grid">
            <div class="settings-column">
              <!-- Receipt Layout -->
              <div class="form-card full-width">
                <div class="card-header" style="padding: 14px 18px; border-bottom: 1px solid var(--border-subtle);">
                  <h3 style="margin: 0; font-size: 14px; font-weight: 700;">Receipt Header &amp; Footer Content</h3>
                </div>
                <div class="card-body">
                  <div class="input-group full-width">
                    <label>Header Text (Printed directly beneath store name)</label>
                    <textarea
                      rows="2"
                      v-model="appSettings.receiptsSettings.headerText"
                      placeholder="e.g. Welcome to our store!"
                    ></textarea>
                  </div>
                  <div class="input-group full-width">
                    <label>Footer Text (Printed at the end of the receipt)</label>
                    <textarea
                      rows="2"
                      v-model="appSettings.receiptsSettings.footerText"
                      placeholder="e.g. Goods once sold are not returnable. Thank you for shopping with us!"
                    ></textarea>
                  </div>
                </div>
              </div>

              <!-- Visible Elements & Automation -->
              <div class="form-card full-width">
                <div class="card-header" style="padding: 14px 18px; border-bottom: 1px solid var(--border-subtle);">
                  <h3 style="margin: 0; font-size: 14px; font-weight: 700;">Printing Automation &amp; Features</h3>
                </div>
                <div class="card-body toggle-list">
                  <div class="toggle-group">
                    <div class="toggle-info">
                      <span class="toggle-label">Auto-Print Receipts</span>
                      <span class="toggle-desc">Automatically print a receipt immediately after checkout.</span>
                    </div>
                    <label class="switch">
                      <input type="checkbox" v-model="appSettings.receiptsSettings.autoPrint" @change="handleReceiptSettingChange" />
                      <span class="slider round"></span>
                    </label>
                  </div>

                  <div class="toggle-group">
                    <span class="toggle-label">Show Store Logo</span>
                    <label class="switch">
                      <input type="checkbox" v-model="appSettings.receiptsSettings.showLogo" @change="handleReceiptSettingChange" />
                      <span class="slider round"></span>
                    </label>
                  </div>

                  <div class="toggle-group">
                    <span class="toggle-label">Show Business Phone Number</span>
                    <label class="switch">
                      <input type="checkbox" v-model="appSettings.receiptsSettings.showPhone" @change="handleReceiptSettingChange" />
                      <span class="slider round"></span>
                    </label>
                  </div>

                  <div class="toggle-group">
                    <span class="toggle-label">Show Cashier Name</span>
                    <label class="switch">
                      <input type="checkbox" v-model="appSettings.receiptsSettings.showCashier" @change="handleReceiptSettingChange" />
                      <span class="slider round"></span>
                    </label>
                  </div>

                  <div class="input-group" style="margin-top: 8px;">
                    <label>Number of Copies</label>
                    <input
                      type="number"
                      v-model.number="appSettings.receiptsSettings.printCopies"
                      min="1"
                      max="5"
                      @change="handleReceiptSettingChange"
                    />
                  </div>
                </div>
              </div>
            </div>

            <!-- Live Preview Side -->
            <div class="preview-column">
              <div class="preview-container">
                <div class="receipt-preview-wrapper">
                  <div :class="['receipt-paper', selectedPaperWidth === '58mm' ? 'paper-58mm' : 'paper-80mm']">
                    <div class="receipt-content">
                      <div
                        class="r-logo"
                        v-if="appSettings.receiptsSettings.showLogo"
                        :style="`display: flex; align-items: center; justify-content: center; font-weight: 700; color: #1e293b; font-size: 0.9rem; letter-spacing: 0.05em; background-color: ${appSettings.shopDetails.logo ? 'transparent' : '#e2e8f0'}; width: 48px; height: 48px; border-radius: 8px; margin: 0 auto 12px auto; overflow: hidden;`"
                      >
                        <img
                          v-if="appSettings.shopDetails.logo"
                          :src="appSettings.shopDetails.logo"
                          style="width: 100%; height: 100%; object-fit: contain; filter: brightness(0);"
                        />
                        <span v-else>LOGO</span>
                      </div>
                      <div class="r-header">
                        <h2>{{ appSettings.shopDetails.name || 'MY STORE' }}</h2>
                        <p>{{ appSettings.shopDetails.address || 'Store Address' }}</p>
                        <p v-if="appSettings.receiptsSettings.showPhone && appSettings.shopDetails.phone">
                          Tel: {{ appSettings.shopDetails.phone }}
                        </p>
                        <p class="r-custom-text" style="white-space: pre-wrap;">{{ appSettings.receiptsSettings.headerText }}</p>
                      </div>
                      <div class="r-divider">--------------------------------</div>
                      <div class="r-meta">
                        <div class="r-meta-row"><span>Receipt:</span> <span>#10042</span></div>
                        <div class="r-meta-row"><span>Date:</span> <span>Aug 15, 2026 14:30</span></div>
                        <div class="r-meta-row" v-if="appSettings.receiptsSettings.showCashier">
                          <span>Cashier:</span> <span>{{ props.userName || 'Admin' }}</span>
                        </div>
                      </div>
                      <div class="r-divider">--------------------------------</div>
                      <div class="r-items">
                        <div class="r-item-head-row">
                          <span>Item</span>
                          <span>Amount</span>
                        </div>
                        <div class="r-item-block">
                          <div class="r-item-title">Matooke 1kg</div>
                          <div class="r-item-calc"><span>&nbsp;&nbsp;1 x 10,000</span><span class="r-item-price">10,000</span></div>
                        </div>
                        <div class="r-item-block">
                          <div class="r-item-title">Fresh Avocado</div>
                          <div class="r-item-calc"><span>&nbsp;&nbsp;2 x 1,000</span><span class="r-item-price">2,000</span></div>
                        </div>
                        <div class="r-item-block">
                          <div class="r-item-title">Tomatoes 500g</div>
                          <div class="r-item-calc"><span>&nbsp;&nbsp;1 x 3,000</span><span class="r-item-price">3,000</span></div>
                        </div>
                      </div>
                      <div class="r-divider">--------------------------------</div>
                      <div class="r-totals">
                        <h3><span>TOTAL</span> <span class="r-amt">{{ appSettings.shopDetails.currency }} 15,000</span></h3>
                      </div>
                      <div class="r-divider">--------------------------------</div>
                      <div class="r-footer">
                        <p class="r-custom-text" style="white-space: pre-wrap;">{{ appSettings.receiptsSettings.footerText }}</p>
                      </div>
                    </div>
                    <div class="paper-jagged-bottom"></div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Add / Edit Printer Modal -->
        <div v-if="showAddPrinterModal" class="modal-overlay" @click.self="showAddPrinterModal = false">
          <div class="modal-content" style="max-width: 580px;">
            <div class="modal-header">
              <h3>{{ isEditingPrinter ? 'Edit POS Printer' : 'Add ESC/POS Thermal Printer' }}</h3>
              <button @click="showAddPrinterModal = false" class="close-btn"><X :size="20" /></button>
            </div>
            <div class="modal-body" style="padding: 20px; display: flex; flex-direction: column; gap: 16px; overflow-y: auto; flex: 1;">
              <div class="input-group full-width">
                <label>Printer Display Name *</label>
                <input type="text" v-model="printerForm.name" placeholder="e.g. Main Counter POS-80" />
              </div>

              <div class="input-group full-width">
                <label>Connection Protocol / Transport</label>
                <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin-top: 4px;">
                  <button
                    type="button"
                    class="btn"
                    :class="printerForm.connectionType === 'winraw' ? 'btn-primary' : 'btn-outline'"
                    style="padding: 8px 10px; font-size: 12px;"
                    @click="printerForm.connectionType = 'winraw'"
                  >
                    Windows Spooler
                  </button>
                  <button
                    type="button"
                    class="btn"
                    :class="printerForm.connectionType === 'serial' ? 'btn-primary' : 'btn-outline'"
                    style="padding: 8px 10px; font-size: 12px;"
                    @click="printerForm.connectionType = 'serial'"
                  >
                    Bluetooth / COM Port
                  </button>
                  <button
                    type="button"
                    class="btn"
                    :class="printerForm.connectionType === 'tcp' ? 'btn-primary' : 'btn-outline'"
                    style="padding: 8px 10px; font-size: 12px;"
                    @click="printerForm.connectionType = 'tcp'"
                  >
                    Network TCP/IP
                  </button>
                  <button
                    type="button"
                    class="btn"
                    :class="printerForm.connectionType === 'usb' ? 'btn-primary' : 'btn-outline'"
                    style="padding: 8px 10px; font-size: 12px;"
                    @click="printerForm.connectionType = 'usb'"
                  >
                    Direct USB
                  </button>
                </div>
              </div>

              <div
                v-if="printerForm.connectionType === 'winraw'"
                style="background: rgba(255,255,255,0.03); border: 1px solid var(--border-subtle); border-radius: 8px; padding: 12px; display: flex; flex-direction: column; gap: 10px;"
              >
                <div style="display: flex; align-items: center; justify-content: space-between;">
                  <label style="font-size: 12px; font-weight: 600; color: var(--text-secondary); margin: 0;">Discovered Windows Printers</label>
                  <button
                    type="button"
                    class="btn btn-outline"
                    style="padding: 3px 8px; font-size: 11px; display: flex; align-items: center; gap: 4px;"
                    @click="scanDiscoveredPrinters"
                    :disabled="isScanningDiscovery"
                  >
                    <RefreshCw :size="11" :class="{ 'spin-icon': isScanningDiscovery }" />
                    <span>Scan</span>
                  </button>
                </div>

                <div
                  v-if="discoveredWindowsPrinters.length > 0"
                  style="display: flex; flex-direction: column; gap: 6px; max-height: 140px; overflow-y: auto;"
                >
                  <div
                    v-for="dp in discoveredWindowsPrinters"
                    :key="dp.name"
                    @click="selectDiscoveredPrinter(dp)"
                    style="padding: 6px 10px; border-radius: 6px; background: rgba(0,0,0,0.2); border: 1px solid var(--border-subtle); cursor: pointer; display: flex; align-items: center; justify-content: space-between; font-size: 12px;"
                    :style="printerForm.address === (dp.port || dp.name) ? 'border-color: var(--accent-color); background: rgba(14, 165, 233, 0.1);' : ''"
                  >
                    <span style="font-weight: 600;">{{ dp.name }}</span>
                    <span style="font-size: 11px; color: var(--text-muted);">{{ dp.port || 'RAW' }}</span>
                  </div>
                </div>
                <div v-else style="font-size: 12px; color: var(--text-muted); text-align: center; padding: 8px;">
                  {{ isScanningDiscovery ? 'Scanning for installed printers...' : 'Click Scan to detect Windows printers' }}
                </div>

                <div class="input-group full-width" style="margin-top: 4px;">
                  <label style="font-size: 12px;">Printer Spooler Name in Windows *</label>
                  <input type="text" v-model="printerForm.address" placeholder="e.g. POS-80 or EPSON TM-T88VI" />
                </div>
              </div>

              <div
                v-else-if="printerForm.connectionType === 'serial'"
                style="background: rgba(255,255,255,0.03); border: 1px solid var(--border-subtle); border-radius: 8px; padding: 12px; display: flex; flex-direction: column; gap: 10px;"
              >
                <div class="input-group full-width">
                  <label style="font-size: 12px;">Serial / COM Port Name *</label>
                  <input type="text" v-model="printerForm.address" placeholder="e.g. COM3 or COM5" />
                </div>
              </div>

              <div
                v-else-if="printerForm.connectionType === 'tcp'"
                style="background: rgba(255,255,255,0.03); border: 1px solid var(--border-subtle); border-radius: 8px; padding: 12px; display: flex; flex-direction: column; gap: 10px;"
              >
                <div class="input-group full-width">
                  <label style="font-size: 12px;">Printer IP Address and Port *</label>
                  <input type="text" v-model="printerForm.address" placeholder="192.168.1.100:9100" />
                </div>
              </div>

              <div
                v-else
                style="background: rgba(255,255,255,0.03); border: 1px solid var(--border-subtle); border-radius: 8px; padding: 12px; display: flex; flex-direction: column; gap: 10px;"
              >
                <div class="input-group full-width">
                  <label style="font-size: 12px;">USB Device Port / Path</label>
                  <input type="text" v-model="printerForm.address" placeholder="USB001 or printer name" />
                </div>
              </div>

              <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px;">
                <div class="input-group">
                  <label>Paper Roll Width</label>
                  <select
                    v-model="printerForm.paperWidth"
                    style="width: 100%; padding: 8px; border-radius: 6px; background: var(--bg-surface); border: 1px solid var(--border-color); color: var(--text-primary);"
                  >
                    <option value="80mm">80mm (Standard POS)</option>
                    <option value="58mm">58mm (Mobile / POS-58)</option>
                  </select>
                </div>
                <div class="input-group">
                  <label>Characters Per Line</label>
                  <input
                    type="number"
                    v-model.number="printerForm.characterWidth"
                    :placeholder="printerForm.paperWidth === '58mm' ? '32' : '48'"
                  />
                </div>
              </div>

              <div>
                <label style="font-size: 12px; font-weight: 600; color: var(--text-secondary); margin-bottom: 8px; display: block;">Hardware Capabilities</label>
                <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 8px;">
                  <label style="display: flex; align-items: center; gap: 8px; font-size: 12px; cursor: pointer;">
                    <input type="checkbox" v-model="printerForm.capabilities.supportsCut" />
                    <span>Auto-Cutter</span>
                  </label>
                  <label style="display: flex; align-items: center; gap: 8px; font-size: 12px; cursor: pointer;">
                    <input type="checkbox" v-model="printerForm.capabilities.supportsQRCode" />
                    <span>QR Code (2D)</span>
                  </label>
                  <label style="display: flex; align-items: center; gap: 8px; font-size: 12px; cursor: pointer;">
                    <input type="checkbox" v-model="printerForm.capabilities.supportsBarcode" />
                    <span>Barcode (1D)</span>
                  </label>
                  <label style="display: flex; align-items: center; gap: 8px; font-size: 12px; cursor: pointer;">
                    <input type="checkbox" v-model="printerForm.capabilities.supportsStatus" />
                    <span>Status Polling (DLE EOT)</span>
                  </label>
                </div>
              </div>

              <div
                class="toggle-group"
                style="margin-top: 4px; padding: 8px 12px; background: rgba(255,255,255,0.02); border-radius: 6px; border: 1px solid var(--border-subtle);"
              >
                <div class="toggle-info">
                  <span class="toggle-label" style="font-size: 13px;">Default Receipt Printer</span>
                  <span class="toggle-desc" style="font-size: 11px;">Automatically route completed POS sales to this printer.</span>
                </div>
                <label class="switch">
                  <input type="checkbox" v-model="printerForm.isDefault" />
                  <span class="slider round"></span>
                </label>
              </div>

              <div style="display: flex; justify-content: flex-end; gap: 10px; margin-top: 10px; padding-top: 12px; border-top: 1px solid var(--border-subtle);">
                <button type="button" class="btn btn-outline" @click="showAddPrinterModal = false">Cancel</button>
                <button type="button" class="btn btn-primary" @click="handleSavePrinterForm">
                  {{ isEditingPrinter ? 'Update Printer' : 'Save Printer' }}
                </button>
              </div>
            </div>
          </div>
        </div>

        <!-- Spooler Queue / Print History Modal -->
        <div v-if="showJobsModal" class="modal-overlay" @click.self="showJobsModal = false">
          <div class="modal-content" style="max-width: 720px;">
            <div class="modal-header">
              <div style="display: flex; align-items: center; gap: 8px;">
                <History :size="18" style="color: var(--accent-color);" />
                <h3>Print Spooler Queue &amp; Logs</h3>
              </div>
              <div style="display: flex; align-items: center; gap: 8px;">
                <button type="button" class="btn btn-outline" style="padding: 4px 8px; font-size: 11px;" @click="refreshJobs" :disabled="isLoadingJobs">
                  <RefreshCw :size="12" :class="{ 'spin-icon': isLoadingJobs }" />
                </button>
                <button @click="showJobsModal = false" class="close-btn"><X :size="20" /></button>
              </div>
            </div>
            <div class="modal-body" style="padding: 16px;">
              <div v-if="printJobs.length === 0" style="padding: 32px; text-align: center; color: var(--text-muted); font-size: 13px;">
                No recent print jobs found in spooler history.
              </div>
              <div v-else style="max-height: 420px; overflow-y: auto;">
                <table class="print-jobs-table">
                  <colgroup>
                    <col class="print-jobs-time-column" />
                    <col class="print-jobs-receipt-column" />
                    <col class="print-jobs-printer-column" />
                    <col class="print-jobs-status-column" />
                    <col class="print-jobs-detail-column" />
                    <col class="print-jobs-action-column" />
                  </colgroup>
                  <thead>
                    <tr style="border-bottom: 1px solid var(--border-subtle); text-align: left; color: var(--text-muted); font-size: 11px; text-transform: uppercase;">
                      <th>Time</th>
                      <th>Receipt #</th>
                      <th>Printer</th>
                      <th>Status</th>
                      <th>Error / Detail</th>
                      <th class="print-jobs-action-cell">Action</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="job in printJobs" :key="job.id" style="border-bottom: 1px solid rgba(255,255,255,0.04);">
                      <td class="print-jobs-time-cell">{{ formatTime(job.createdAt) }}</td>
                      <td class="print-jobs-receipt-cell">#{{ job.receiptId ? job.receiptId.slice(-6) : 'TEST' }}</td>
                      <td class="print-jobs-printer-cell">{{ job.printerName || 'Default' }}</td>
                      <td>
                        <span
                          style="font-size: 10px; font-weight: 700; padding: 2px 7px; border-radius: 10px; text-transform: uppercase;"
                          :style="{
                            backgroundColor: job.status === 'completed' ? 'rgba(16, 185, 129, 0.15)' : 'rgba(239, 68, 68, 0.15)',
                            color: job.status === 'completed' ? '#10b981' : '#ef4444',
                          }"
                        >
                          {{ job.status }}
                        </span>
                      </td>
                      <td class="print-jobs-detail-cell">
                        {{ job.errorMessage || (job.status === 'completed' ? 'Delivered to spooler' : 'In progress') }}
                      </td>
                      <td class="print-jobs-action-cell">
                        <button
                          type="button"
                          class="btn btn-outline"
                          style="padding: 3px 8px; font-size: 11px;"
                          @click="handleRetryJob(job.id)"
                          :disabled="isRetryingJobId === job.id"
                        >
                          {{ isRetryingJobId === job.id ? 'Retrying...' : 'Retry' }}
                        </button>
                      </td>
                    </tr>
                  </tbody>
                </table>
              </div>
              <div style="display: flex; justify-content: flex-end; margin-top: 14px; padding-top: 10px; border-top: 1px solid var(--border-subtle);">
                <button type="button" class="btn btn-outline" @click="showJobsModal = false">Close</button>
              </div>
            </div>
          </div>
        </div>
      </template>

      <!-- POS Configuration -->
      <template v-else-if="activeTab === 'pos-config'">
        <div class="form-grid">
          <!-- Checkout Workflow -->
          <div class="form-card">
            <div class="card-header">
              <h3>Checkout Workflow</h3>
            </div>
            <div class="card-body toggle-list">
              <div class="toggle-group">
                <div class="toggle-info">
                  <span class="toggle-label">Auto-Clear Cart</span>
                  <span class="toggle-desc">Clear cart automatically after a successful sale.</span>
                </div>
                <label class="switch">
                  <input type="checkbox" v-model="appSettings.applicationSettings.autoClearCart" @change="handleGeneralAppSettingChange" />
                  <span class="slider round"></span>
                </label>
              </div>
              <div class="toggle-group">
                <div class="toggle-info">
                  <span class="toggle-label">Require PIN for Discounts</span>
                  <span class="toggle-desc">Ask for supervisor PIN when applying manual discounts.</span>
                </div>
                <label class="switch">
                  <input type="checkbox" v-model="appSettings.applicationSettings.requirePinForDiscounts" @change="handleGeneralAppSettingChange" />
                  <span class="slider round"></span>
                </label>
              </div>
              <div class="toggle-group">
                <div class="toggle-info">
                  <span class="toggle-label">Scanner Beep Sound</span>
                  <span class="toggle-desc">Play a beep when a barcode is scanned.</span>
                </div>
                <label class="switch">
                  <input type="checkbox" v-model="appSettings.applicationSettings.barcodeBeep" @change="handleGeneralAppSettingChange" />
                  <span class="slider round"></span>
                </label>
              </div>
              <div class="toggle-group" v-if="appSettings.applicationSettings.requirePinForDiscounts">
                <div class="toggle-info">
                  <span class="toggle-label">Set Supervisor PIN</span>
                  <span class="toggle-desc">4-digit PIN to approve discounts.</span>
                </div>
                <div style="display: flex; gap: 0.5rem;">
                  <input
                    type="password"
                    v-model="appSettings.applicationSettings.discountPin"
                    @change="handleGeneralAppSettingChange"
                    placeholder="****"
                    maxlength="4"
                    style="width: 80px; text-align: center; background: rgba(0,0,0,0.2); border: 1px solid var(--border-light); color: var(--text-primary); padding: 0.5rem; border-radius: 6px; font-size: 1rem; letter-spacing: 2px;"
                  />
                </div>
              </div>
            </div>
          </div>

          <!-- Display -->
          <div class="form-card">
            <div class="card-header">
              <h3>Display</h3>
            </div>
            <div class="card-body toggle-list">
              <div class="toggle-group">
                <div class="toggle-info">
                  <span class="toggle-label">Enlarge Product Image on Hover</span>
                  <span class="toggle-desc">Show a larger image when hovering over a product in the POS grid.</span>
                </div>
                <label class="switch">
                  <input type="checkbox" v-model="appSettings.applicationSettings.previewPosImagesOnHover" @change="handleGeneralAppSettingChange" />
                  <span class="slider round"></span>
                </label>
              </div>
              <div class="toggle-group">
                <div class="toggle-info">
                  <span class="toggle-label">Show Product Images</span>
                  <span class="toggle-desc">Display thumbnails in the POS product grid.</span>
                </div>
                <label class="switch">
                  <input type="checkbox" v-model="appSettings.applicationSettings.showPosImages" @change="handleGeneralAppSettingChange" />
                  <span class="slider round"></span>
                </label>
              </div>
              <div class="toggle-group">
                <div class="toggle-info">
                  <span class="toggle-label">Hide Out of Stock</span>
                  <span class="toggle-desc">Do not show items with 0 inventory.</span>
                </div>
                <label class="switch">
                  <input type="checkbox" v-model="appSettings.applicationSettings.hideOutOfStock" @change="handleGeneralAppSettingChange" />
                  <span class="slider round"></span>
                </label>
              </div>
            </div>
          </div>
        </div>
      </template>

      <!-- Appearance & Theme -->
      <template v-else-if="activeTab === 'appearance-theme'">
        <div class="form-grid">
          <div class="form-card full-width theme-selection-card">
            <div class="card-header">
              <h3>Color Scheme</h3>
            </div>
            <div class="theme-split-container">
              <div class="theme-section-left">
                <div class="card-body">
                  <div class="theme-options">
                    <button
                      type="button"
                      class="theme-btn"
                      :class="{ active: appSettings.applicationSettings.themeMode === 'dark' }"
                      @click="handleThemeChange('dark')"
                    >
                      <div class="theme-preview-container">
                        <div class="theme-preview dark">
                          <div class="tp-header"></div>
                          <div class="tp-body">
                            <div class="tp-sidebar"></div>
                            <div class="tp-content">
                              <div class="tp-card"></div>
                              <div class="tp-card tp-card-small"></div>
                            </div>
                          </div>
                        </div>
                        <div v-if="appSettings.applicationSettings.themeMode === 'dark'" class="active-indicator">
                          <CheckCircle2 :size="14" />
                        </div>
                      </div>
                      <div class="theme-label">
                        <Moon :size="16" class="theme-icon" />
                        <span>Dark Mode</span>
                      </div>
                    </button>

                    <button
                      type="button"
                      class="theme-btn"
                      :class="{ active: appSettings.applicationSettings.themeMode === 'light' }"
                      @click="handleThemeChange('light')"
                    >
                      <div class="theme-preview-container">
                        <div class="theme-preview light">
                          <div class="tp-header"></div>
                          <div class="tp-body">
                            <div class="tp-sidebar"></div>
                            <div class="tp-content">
                              <div class="tp-card"></div>
                              <div class="tp-card tp-card-small"></div>
                            </div>
                          </div>
                        </div>
                        <div v-if="appSettings.applicationSettings.themeMode === 'light'" class="active-indicator">
                          <CheckCircle2 :size="14" />
                        </div>
                      </div>
                      <div class="theme-label">
                        <Sun :size="16" class="theme-icon" />
                        <span>Light Mode</span>
                      </div>
                    </button>

                    <button
                      type="button"
                      class="theme-btn"
                      :class="{ active: appSettings.applicationSettings.themeMode === 'system' }"
                      @click="handleThemeChange('system')"
                    >
                      <div class="theme-preview-container">
                        <div class="theme-preview system">
                          <div class="tp-split-left">
                            <div class="tp-header"></div>
                            <div class="tp-sidebar"></div>
                          </div>
                          <div class="tp-split-right">
                            <div class="tp-header"></div>
                            <div class="tp-content">
                              <div class="tp-card"></div>
                              <div class="tp-card tp-card-small"></div>
                            </div>
                          </div>
                        </div>
                        <div v-if="appSettings.applicationSettings.themeMode === 'system'" class="active-indicator">
                          <CheckCircle2 :size="14" />
                        </div>
                      </div>
                      <div class="theme-label">
                        <Monitor :size="16" class="theme-icon" />
                        <span>System Match</span>
                      </div>
                    </button>
                  </div>
                </div>
              </div>

              <div class="theme-section-divider"></div>

              <div class="theme-section-right">
                <div class="card-section-header">
                  <h4>Accent Color</h4>
                </div>
                <div class="card-body">
                  <div class="color-options">
                    <button
                      v-for="color in availableAccentColors"
                      :key="color"
                      type="button"
                      class="color-btn"
                      :class="{ active: (appSettings.applicationSettings.accent || '#0ea5e9') === color }"
                      :style="`--accent: ${color}; --accent-glow: color-mix(in srgb, ${color} 50%, transparent);`"
                      @click="handleAccentChange(color)"
                    >
                      <CheckCircle2 :size="20" class="color-check" />
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div class="appearance-preferences-row">
            <!-- Typography -->
            <div class="form-card">
              <div class="card-header">
                <h3>Typography</h3>
              </div>
              <div class="card-body toggle-list">
                <div class="input-group full-width">
                  <label>App Font</label>
                  <CustomSelect
                    v-model="appSettings.applicationSettings.typography"
                    :options="[
                      { label: 'Outfit', value: 'Outfit' },
                      { label: 'Nunito', value: 'Nunito' },
                      { label: 'Inter', value: 'Inter' },
                      { label: 'Roboto', value: 'Roboto' },
                      { label: 'System Default', value: 'System Default' },
                    ]"
                    :fullWidth="true"
                    @update:modelValue="handleGeneralAppSettingChange"
                  />
                </div>
              </div>
            </div>

            <!-- Format & Localization -->
            <div class="form-card">
              <div class="card-header">
                <h3>Format &amp; Localization</h3>
              </div>
              <div class="card-body">
                <div class="input-group full-width">
                  <label>Date Format</label>
                  <CustomSelect
                    v-model="appSettings.localizationSettings.dateFormat"
                    :options="dateFormatOptions"
                    :fullWidth="true"
                    :dropUp="true"
                    @update:modelValue="handleLocalizationAppSettingChange"
                  />
                </div>
                <div class="input-group full-width">
                  <label>Time Format</label>
                  <CustomSelect
                    v-model="appSettings.localizationSettings.timeFormat"
                    :options="timeFormatOptions"
                    :fullWidth="true"
                    :dropUp="true"
                    @update:modelValue="handleLocalizationAppSettingChange"
                  />
                </div>
              </div>
            </div>
          </div>
        </div>
      </template>

      <!-- About -->
      <template v-else-if="activeTab === 'about'">
        <div class="about-grid">
          <div class="about-hero-card full-width">
            <div class="app-logo-block">
              <div class="app-logo-icon">
                <img src="/cmretailos-brand-logo.png" alt="CM RetailOS Logo" style="width: 56px; height: 56px; object-fit: contain;" />
              </div>
              <div class="app-identity">
                <h2>CM RetailOS</h2>
                <p class="app-tagline">Developed &amp; Powered by CM Studios UG</p>
              </div>
            </div>
            <div class="version-badges">
              <div class="version-badge">
                <span class="badge-label">VERSION</span>
                <span class="badge-value">{{ appVersion }}</span>
              </div>
              <div class="version-badge">
                <span class="badge-label">MARKET</span>
                <span class="badge-value">African Businesses</span>
              </div>
            </div>
          </div>

          <div class="form-card" style="padding: 24px;">
            <h3 style="font-size: 1.1rem; color: var(--text-primary); margin-bottom: 12px; font-weight: 600;">About CM RetailOS</h3>
            <p style="color: var(--text-muted); font-size: 14px; line-height: 1.6; margin-bottom: 16px;">
              CM RetailOS is a professional Point-of-Sale, inventory, sales and business management system developed for modern retail businesses across Africa.
            </p>
            <p style="color: var(--text-muted); font-size: 14px; line-height: 1.6; margin-bottom: 16px;">
              It brings together Point of Sale, Product Management, Inventory Management, Categories, Sales &amp; Receipts, Profit Analytics, Business Intelligence, User Management, Audit Logs, and System Settings.
            </p>
            <p style="color: var(--text-muted); font-size: 14px; line-height: 1.6; margin: 0;">
              The system is designed around reliability, performance, security, maintainability and scalability, with particular consideration for businesses operating across Uganda and the wider African market. Supports African currencies including UGX and others.
            </p>
          </div>

          <div class="form-card" style="padding: 32px 24px; display: flex; flex-direction: column; align-items: center; text-align: center;">
            <p style="color: var(--text-muted); font-size: 0.75rem; letter-spacing: 0.12em; text-transform: uppercase; margin: 0 0 16px;">Built by</p>
            <img
              src="/developer-portrait.jpg"
              alt="Christopher Muheki"
              style="width: 120px; height: 120px; object-fit: cover; object-position: center 15%; border-radius: 50%; border: 4px solid var(--border-subtle); margin-bottom: 16px; box-shadow: 0 4px 12px rgba(0,0,0,0.3);"
            />
            <h4 style="font-size: 1.2rem; color: var(--text-primary); margin-bottom: 4px;">Christopher Muheki</h4>
            <p style="color: var(--text-muted); font-size: 0.95rem; margin: 0 0 12px 0;">Self-Taught Software Engineer and Founder</p>
            <p style="color: var(--text-muted); font-size: 0.9rem; line-height: 1.6; margin: 0;">
              Christopher Muheki is a self-taught software engineer, designer and technology-focused creative professional. His work combines software engineering, system architecture, UI/UX design, graphics, photography, cinematography, audio engineering and digital technology to create practical, high-performance products and solutions.
            </p>
          </div>

          <div class="mvd-card form-card full-width">
            <div class="mvd-item">
              <div class="mvd-icon" style="background: rgba(14,165,233,0.1); border-color: rgba(14,165,233,0.2);">
                <Target :size="24" color="var(--accent-color)" />
              </div>
              <div class="mvd-text">
                <h3>Mission</h3>
                <p>To build reliable, professional and accessible retail technology that helps African businesses manage their operations with greater accuracy, efficiency and confidence.</p>
              </div>
            </div>
            <div class="mvd-divider"></div>
            <div class="mvd-item">
              <div class="mvd-icon" style="background: rgba(168,85,247,0.1); border-color: rgba(168,85,247,0.2);">
                <Eye :size="24" color="#a855f7" />
              </div>
              <div class="mvd-text">
                <h3>Vision</h3>
                <p>To create a modern retail operating system designed around the realities of African businesses, combining professional business management, offline reliability and intelligent analytics in one dependable platform.</p>
              </div>
            </div>
          </div>

          <div class="full-width" style="padding: 24px 0; text-align: center;">
            <p style="color: var(--text-secondary); font-weight: 500; font-size: 0.95rem; margin-bottom: 8px;">Built in Africa. Designed for modern business.</p>
            <p style="color: var(--text-muted); font-size: 0.85rem; margin: 0;">(c) 2026 CM Studios UG. All rights reserved.</p>
          </div>
        </div>
      </template>
    </main>

    <!-- Delete Printer Confirmation Modal -->
    <div v-if="showDeletePrinterModal" class="modal-overlay" @click.self="showDeletePrinterModal = false" style="z-index: 1100;">
      <div class="modal-content" style="max-width: 420px; border-radius: 16px; overflow: hidden;">
        <div style="background: linear-gradient(135deg, #7f1d1d 0%, #991b1b 100%); padding: 20px 24px; display: flex; align-items: center; gap: 12px; position: relative;">
          <div style="width: 42px; height: 42px; border-radius: 50%; background: rgba(255,255,255,0.12); display: flex; align-items: center; justify-content: center; flex-shrink: 0;">
            <Trash2 :size="20" color="white" />
          </div>
          <div>
            <h3 style="margin: 0; color: #fff; font-size: 16px; font-weight: 700;">Remove Printer</h3>
            <p style="margin: 2px 0 0; color: rgba(255,255,255,0.65); font-size: 12px;">This action cannot be undone</p>
          </div>
          <button @click="showDeletePrinterModal = false" class="close-btn" style="position: absolute; right: 16px; top: 16px; opacity: 0.7; color: white;">
            <X :size="18" />
          </button>
        </div>
        <div class="modal-body" style="padding: 24px;">
          <div style="background: var(--bg-surface); border: 1px solid var(--border-color); border-radius: 12px; padding: 16px; display: flex; align-items: center; gap: 14px; margin-bottom: 20px;">
            <div style="width: 46px; height: 46px; border-radius: 10px; background: rgba(239,68,68,0.1); border: 1px solid rgba(239,68,68,0.25); display: flex; align-items: center; justify-content: center; flex-shrink: 0;">
              <Printer :size="22" color="#ef4444" />
            </div>
            <div style="min-width: 0;">
              <p style="margin: 0; font-weight: 700; font-size: 15px; color: var(--text-primary); white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">{{ printerToDelete?.name }}</p>
              <p style="margin: 4px 0 0; font-size: 12px; color: var(--text-muted);">
                {{ printerToDelete?.connectionType?.toUpperCase() }} &bull; {{ printerToDelete?.address }}
              </p>
            </div>
          </div>
          <div v-if="printerToDelete?.isDefault" style="display: flex; gap: 10px; align-items: flex-start; background: rgba(234,179,8,0.08); border: 1px solid rgba(234,179,8,0.2); border-radius: 10px; padding: 12px 14px; margin-bottom: 20px;">
            <AlertTriangle :size="16" color="#eab308" style="flex-shrink: 0; margin-top: 1px;" />
            <p style="margin: 0; font-size: 13px; color: #ca8a04; line-height: 1.5;">
              This is your <strong>default printer</strong>. Removing it means no receipts will print until you set another printer as default.
            </p>
          </div>
          <div style="display: flex; gap: 10px;">
            <button class="btn btn-outline" style="flex: 1; height: 42px;" @click="showDeletePrinterModal = false">Cancel</button>
            <button
              style="flex: 1; height: 42px; background: #dc2626; border: none; color: white; border-radius: 8px; font-weight: 600; font-size: 14px; cursor: pointer; display: flex; align-items: center; justify-content: center; gap: 8px;"
              @click="confirmDeletePrinter"
            >
              <Trash2 :size="15" />
              Remove Printer
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.print-jobs-table {
  width: 100%;
  table-layout: fixed;
  border-collapse: collapse;
  font-size: 12px;
}

.print-jobs-time-column { width: 17%; }
.print-jobs-receipt-column { width: 16%; }
.print-jobs-printer-column { width: 20%; }
.print-jobs-status-column { width: 14%; }
.print-jobs-detail-column { width: 22%; }
.print-jobs-action-column { width: 11%; }

.print-jobs-table th,
.print-jobs-table td {
  box-sizing: border-box;
  padding: 8px;
  text-align: left;
  vertical-align: middle;
}

.print-jobs-time-cell {
  color: var(--text-secondary);
  white-space: nowrap;
}

.print-jobs-receipt-cell {
  color: var(--text-primary);
  font-weight: 600;
  white-space: nowrap;
}

.print-jobs-printer-cell,
.print-jobs-detail-cell {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.print-jobs-printer-cell {
  color: var(--text-secondary);
}

.print-jobs-detail-cell {
  color: var(--text-muted);
}

.print-jobs-action-cell {
  text-align: right !important;
  white-space: nowrap;
}

@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
.spin-icon {
  animation: spin 1s linear infinite;
}

/* Page Layout */
.settings-page {
  display: flex;
  height: 100%;
  min-height: 0;
  box-sizing: border-box;
  background-color: transparent;
  color: var(--text-secondary);
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
  overflow: hidden;
  min-width: 0;
  gap: var(--space-page-tight, 8px);
}

/* Sidebar */
.settings-sidebar {
  width: 280px;
  background-color: var(--bg-sidebar);
  border: 1px solid var(--border-subtle);
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  flex-shrink: 0;
  min-height: 0;
}

.sidebar-menu {
  flex: 1;
  overflow-y: auto;
  padding: 16px 12px;
  display: flex;
  flex-direction: column;
  gap: 24px;
  -ms-overflow-style: none;
  scrollbar-width: none;
}

.sidebar-menu::-webkit-scrollbar {
  display: none;
}

.category-title {
  font-size: 11px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin: 0 0 8px 12px;
}

.menu-group-card {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.menu-item {
  width: 100%;
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 12px;
  background: transparent;
  border: none;
  border-bottom: 1px solid var(--border-color);
  border-radius: 0;
  color: var(--text-secondary);
  cursor: pointer;
  text-align: left;
  transition: all 0.2s;
}

.menu-item:last-child {
  border-bottom: none;
}

.menu-item:hover {
  background-color: var(--bg-surface-hover);
  color: var(--text-primary);
}

.menu-item.active {
  background-color: rgba(14, 165, 233, 0.1);
  color: var(--accent-color);
}

.menu-item.active .item-icon {
  color: var(--accent-color);
}

.item-icon {
  margin-top: 2px;
  flex-shrink: 0;
}

.item-text {
  display: flex;
  flex-direction: column;
}

.item-label {
  font-size: 14px;
  font-weight: 500;
  margin-bottom: 2px;
}

.item-desc {
  font-size: 11px;
  color: var(--text-muted);
}

.sidebar-feedback {
  margin: 0 16px;
  padding: 8px 12px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 500;
}

.sidebar-feedback.success {
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.35);
  color: #10b981;
}

.sidebar-feedback.error {
  background: rgba(239, 68, 68, 0.15);
  border: 1px solid rgba(239, 68, 68, 0.35);
  color: #ef4444;
}

/* Main Content */
.settings-content {
  flex: 1;
  min-height: 0;
  padding: 12px;
  overflow-y: auto;
  overflow-x: hidden;
  min-width: 0;
  display: flex;
  flex-direction: column;
  background-color: var(--bg-sidebar);
  border-radius: 12px;
  border: 1px solid var(--border-subtle);
  container-type: inline-size;
  container-name: settings-panel;
}

.settings-content::-webkit-scrollbar {
  width: 8px;
}

.settings-content::-webkit-scrollbar-thumb {
  background: var(--border-color);
  border-radius: 4px;
}

/* Buttons */
.btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  border-radius: 6px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
  border: 1px solid transparent;
}

.btn-primary {
  background-color: var(--accent-color);
  color: white;
}

.btn-primary:hover {
  background-color: var(--accent-hover);
}

.btn-outline {
  background-color: var(--bg-surface);
  border-color: var(--border-color);
  color: var(--text-primary);
}

.btn-outline:hover {
  background-color: var(--border-color);
}

.icon-btn {
  background: transparent;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  padding: 4px;
  border-radius: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
}

.icon-btn:hover {
  background-color: var(--bg-surface);
  color: var(--text-primary);
}

/* Forms & Cards */
.form-grid {
  width: 100%;
  flex-shrink: 0;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  padding-bottom: 8px;
}

.form-card {
  width: 100%;
  min-width: 0;
  flex-shrink: 0;
  box-sizing: border-box;
  background-color: var(--bg-sidebar);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  overflow: visible;
  transition: background-color 0.2s;
}

.table-container {
  width: 100%;
  min-width: 0;
  flex-shrink: 0;
}

.business-info-grid {
  display: flex;
  flex-direction: column;
  flex: 1;
}

.business-unified-card {
  flex: 1;
  display: flex;
  flex-direction: column;
}

.full-width {
  grid-column: 1 / -1;
}

.card-header {
  padding: 1rem 1.5rem;
  border-bottom: 1px solid var(--border-subtle);
  background: transparent;
}

.card-header h3 {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}

.card-header p {
  margin: 4px 0 0 0;
  font-size: 13px;
  color: var(--text-muted);
}

.card-section-header {
  padding: 1rem 1.5rem;
  border-top: 1px solid var(--border-subtle);
  border-bottom: 1px solid var(--border-subtle);
  background: var(--bg-card-subtle, rgba(30, 41, 59, 0.25));
}

.card-section-header h4 {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}

.card-section-header p {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--text-muted);
}

.business-details-columns {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
}

.business-details-section {
  min-width: 0;
}

.business-unified-section.logo-section {
  border-bottom: 1px solid var(--glass-border);
  padding-bottom: 16px;
}

.business-unified-section.logo-section .card-section-header {
  padding: 0.875rem 1.25rem 0;
  text-align: center;
  min-height: auto;
  border-bottom: none;
}

.business-unified-section.logo-section .card-section-header p {
  margin: 0 auto;
}

.business-unified-section.logo-section .logo-upload-area {
  padding: 8px 1.25rem 0;
  justify-content: center;
}

.location-details-section {
  border-left: 1px solid var(--border-subtle);
}

.business-unified-card .card-section-header {
  border-top: 0;
  padding: 0.875rem 1.25rem;
  min-height: 72px;
  box-sizing: border-box;
}

.business-unified-card .card-body {
  padding: 1.25rem;
  gap: 16px;
}

.card-body {
  padding: 24px;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 20px;
}

.input-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.input-group.full-width {
  grid-column: 1 / -1;
}

.input-group label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-secondary);
}

.input-group input,
.input-group select,
.input-group textarea {
  width: 100%;
  box-sizing: border-box;
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 0.375rem;
  padding: 0.6rem 0.75rem;
  color: var(--text-primary);
  font-size: 0.9rem;
  transition: border-color 0.2s;
  font-family: inherit;
}

.input-group input:focus,
.input-group select:focus,
.input-group textarea:focus {
  outline: none;
  border-color: var(--accent-color);
}

.input-group input::placeholder,
.input-group textarea::placeholder {
  color: var(--text-muted);
}

.input-group textarea {
  resize: vertical;
}

/* Logo Upload */
.logo-upload-area {
  padding: 24px;
  display: flex;
  align-items: center;
  gap: 24px;
}

.current-logo {
  width: 80px;
  height: 80px;
  border-radius: 16px;
  background: linear-gradient(135deg, var(--bg-surface) 0%, var(--bg-sidebar) 100%);
  border: 2px dashed var(--text-muted);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--text-muted);
  transition: all 0.2s;
}

.upload-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.upload-hint {
  font-size: 12px;
  color: var(--text-muted);
}

/* Toggles */
.toggle-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.toggle-group {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  gap: 16px;
}

.toggle-info {
  display: flex;
  flex-direction: column;
}

.toggle-label {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
}

.toggle-desc {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 2px;
}

.switch {
  position: relative;
  display: inline-block;
  width: 44px;
  height: 24px;
  flex-shrink: 0;
}

.switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: var(--border-color);
  transition: 0.3s;
}

.slider:before {
  position: absolute;
  content: '';
  height: 18px;
  width: 18px;
  left: 3px;
  bottom: 3px;
  background-color: white;
  transition: 0.3s;
}

input:checked + .slider {
  background-color: var(--accent-color);
}

input:checked + .slider:before {
  transform: translateX(20px);
}

.slider.round {
  border-radius: 24px;
}

.slider.round:before {
  border-radius: 50%;
}

/* Receipt Grid Layout */
.receipt-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(260px, 300px);
  gap: 20px;
  align-items: start;
  width: 100%;
}

.receipt-grid .card-body {
  padding: 16px;
  gap: 12px;
}

.settings-column {
  display: flex;
  flex-direction: column;
  gap: 16px;
  min-width: 0;
}

.preview-column {
  position: relative;
  width: 100%;
  max-width: 300px;
  min-width: 0;
  display: flex;
  justify-content: center;
  box-sizing: border-box;
}

.preview-container {
  width: 100%;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 16px 12px;
  background: var(--bg-surface);
  border-radius: 12px;
  border: 1px solid var(--border-subtle);
}

.receipt-preview-wrapper {
  padding: 0;
  width: 100%;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.receipt-paper {
  background-color: #fdfdfd;
  margin: 0 auto;
  padding: 0 !important;
  width: 100%;
  max-width: 260px;
  box-sizing: border-box;
  box-shadow: 0 4px 18px rgba(0, 0, 0, 0.2);
  position: relative;
  color: #1e293b;
  font-family: 'Courier New', Courier, monospace;
}

.receipt-paper.paper-58mm {
  width: 100%;
  max-width: 216px;
  font-size: 11px;
}

.receipt-paper.paper-58mm .receipt-content {
  padding: 12px 8px;
}

.receipt-paper.paper-58mm .r-header h2 {
  font-size: 14px;
}

.receipt-paper.paper-58mm .r-header p {
  font-size: 10px;
}

.receipt-paper.paper-58mm .r-divider {
  font-size: 10px;
  letter-spacing: -1.2px;
}

.receipt-paper.paper-58mm .r-items {
  font-size: 10.5px;
}

.receipt-paper.paper-58mm .r-totals h3 {
  font-size: 12.5px;
}

.receipt-paper.paper-58mm .r-footer {
  font-size: 10px;
}

.receipt-paper.paper-80mm {
  width: 100%;
  max-width: 266px;
  font-size: 12.5px;
}

.receipt-content {
  padding: 16px;
}

.r-header {
  text-align: center;
}

.r-header h2 {
  font-size: 18px;
  margin: 0 0 4px 0;
  font-weight: bold;
}

.r-header p {
  font-size: 12px;
  margin: 2px 0;
  color: #64748b;
}

.r-custom-text {
  font-style: italic;
  margin-top: 8px !important;
  color: #64748b !important;
}

.r-divider {
  text-align: center;
  font-size: 12px;
  letter-spacing: -0.5px;
  color: #94a3b8;
  margin: 8px 0;
  overflow: hidden;
  white-space: nowrap;
}

.r-meta-row {
  display: flex;
  justify-content: space-between;
  font-size: 11.5px;
  margin: 2px 0;
}

.r-items {
  display: flex;
  flex-direction: column;
  gap: 6px;
  width: 100%;
  font-size: 12px;
}

.r-item-head-row {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  font-weight: 700;
  border-bottom: 1px dashed #cbd5e1;
  padding-bottom: 2px;
  color: #475569;
}

.r-item-block {
  display: flex;
  flex-direction: column;
  gap: 1px;
  width: 100%;
}

.r-item-title {
  font-weight: 700;
  font-size: 11.5px;
  color: #1e293b;
  text-align: left;
}

.r-item-calc {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: #334155;
}

.r-item-price {
  text-align: right;
  font-weight: 700;
  color: #1e293b;
}

.r-totals h3 {
  display: flex;
  justify-content: space-between;
  font-size: 16px;
  font-weight: bold;
  margin: 8px 0 0 0;
}

.r-footer {
  text-align: center;
}

.paper-jagged-bottom {
  position: absolute;
  bottom: -6px;
  left: 0;
  width: 100%;
  height: 6px;
  background-size: 12px 12px;
  background-repeat: repeat-x;
  background-image: linear-gradient(-45deg, transparent 33.33%, #fdfdfd 33.33%, #fdfdfd 66.66%, transparent 66.66%),
    linear-gradient(45deg, transparent 33.33%, #fdfdfd 33.33%, #fdfdfd 66.66%, transparent 66.66%);
  filter: drop-shadow(0 2px 1px rgba(0, 0, 0, 0.1));
}

/* Theme & Appearance */
.theme-options {
  display: grid;
  grid-template-columns: repeat(3, minmax(110px, 1fr));
  gap: 14px;
  width: 100%;
}

.theme-btn {
  min-width: 0;
  background: var(--glass-bg);
  border: 1px solid var(--border-subtle);
  border-radius: 12px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  color: var(--text-muted);
  outline: none;
  position: relative;
  overflow: hidden;
}

.theme-btn:hover {
  border-color: var(--border-color);
  background: var(--bg-surface-hover);
  transform: translateY(-2px);
}

.theme-btn.active {
  border-color: var(--accent-color);
  background: color-mix(in srgb, var(--accent-color) 8%, transparent);
  box-shadow: 0 0 0 1px var(--accent-color);
}

.theme-preview-container {
  width: 100%;
  position: relative;
  border-radius: 8px;
  overflow: hidden;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

.theme-preview {
  width: 100%;
  aspect-ratio: 16 / 10;
  min-height: 52px;
  max-height: 84px;
  display: flex;
  flex-direction: column;
  background: #020617;
  border: 1px solid #334155;
  border-radius: 8px;
  overflow: hidden;
}

.theme-preview.light {
  background: #ffffff;
  border-color: #cbd5e1;
}

.theme-preview.system {
  flex-direction: row;
  border-color: var(--text-muted);
}

.tp-header {
  height: 12px;
  background: #0f172a;
  border-bottom: 1px solid #1e293b;
  flex-shrink: 0;
}
.light .tp-header { background: #f1f5f9; border-color: #e2e8f0; }

.tp-body {
  display: flex;
  flex: 1;
  min-height: 0;
}

.tp-sidebar {
  width: 25%;
  background: #020617;
  border-right: 1px solid #1e293b;
}
.light .tp-sidebar { background: #f8fafc; border-color: #e2e8f0; }

.tp-content {
  flex: 1;
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  background: #020617;
}
.light .tp-content { background: #ffffff; }

.tp-card {
  height: 24px;
  background: #0f172a;
  border-radius: 4px;
  border: 1px solid #1e293b;
}
.light .tp-card { background: #f1f5f9; border-color: #e2e8f0; }

.tp-card-small {
  height: 12px;
  width: 60%;
}

.tp-split-left {
  flex: 1;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--border-color);
}

.tp-split-right {
  flex: 1;
  display: flex;
  flex-direction: column;
  background: var(--bg-app);
}

.active-indicator {
  position: absolute;
  top: 6px;
  right: 6px;
  width: 22px;
  height: 22px;
  background: var(--accent-color);
  color: white;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.theme-label {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
  white-space: nowrap;
}

.theme-btn.active .theme-label,
.theme-btn.active .theme-icon {
  color: var(--accent-color);
}

.color-options {
  display: grid;
  grid-template-columns: repeat(5, minmax(0, 1fr));
  max-width: 340px;
  width: 100%;
  gap: 10px;
  padding: 6px 0;
}

.color-btn {
  justify-self: center;
  width: 38px;
  height: 38px;
  border-radius: 12px;
  border: 1px solid var(--border-light);
  background-color: var(--accent);
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: inset 0 2px 6px rgba(255, 255, 255, 0.2), 0 2px 8px rgba(0, 0, 0, 0.3);
  position: relative;
}

.color-btn:hover {
  transform: translateY(-3px) scale(1.05);
}

.color-btn.active {
  border: 2px solid #ffffff;
  transform: translateY(-2px) scale(1.1);
}

.color-check {
  color: white;
  opacity: 0;
  transform: scale(0.3);
  transition: all 0.2s;
}

.color-btn.active .color-check {
  opacity: 1;
  transform: scale(1);
}

.theme-split-container {
  display: grid;
  grid-template-columns: minmax(0, 1.45fr) minmax(0, 1fr);
  gap: 20px;
  width: 100%;
}

.theme-selection-card .card-body {
  display: block;
  padding: 14px 16px;
}

.theme-section-left {
  border-right: 1px solid var(--border-subtle);
  padding-right: 20px;
  min-width: 0;
}

.theme-section-right {
  padding-left: 0;
  min-width: 0;
}

.theme-section-right .card-section-header {
  margin-left: -20px;
  padding-left: calc(1.25rem + 20px);
}

.theme-section-right .card-body {
  padding-top: 8px;
}

.theme-section-right .color-options {
  padding-top: 0;
}

.theme-section-divider {
  display: none;
}

.appearance-preferences-row {
  grid-column: 1 / -1;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
  flex: 1;
  width: 100%;
}

/* About Page */
.about-grid {
  width: 100%;
  flex-shrink: 0;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  padding-bottom: 24px;
}

.about-hero-card {
  background-color: var(--bg-sidebar);
  border: 1px solid var(--border-subtle);
  border-radius: 8px;
  padding: 22px 26px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 20px;
}

.app-logo-block {
  display: flex;
  align-items: center;
  gap: 20px;
}

.app-logo-icon {
  width: 72px;
  height: 72px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.app-identity h2 {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-primary);
  margin: 0 0 6px 0;
}

.app-tagline {
  color: var(--text-muted);
  font-size: 13px;
  margin: 0;
}

.version-badges {
  display: flex;
  gap: 12px;
}

.version-badge {
  display: flex;
  flex-direction: column;
  align-items: center;
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 12px 20px;
  min-width: 90px;
}

.badge-label {
  font-size: 10px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 6px;
}

.badge-value {
  font-size: 13px;
  font-weight: 600;
  color: var(--accent-color);
}

.mvd-card {
  display: flex;
  flex-direction: column;
  padding: 0;
}

.mvd-item {
  display: flex;
  align-items: flex-start;
  gap: 20px;
  padding: 24px;
}

.mvd-divider {
  height: 1px;
  background-color: var(--bg-surface);
  margin: 0 24px;
}

.mvd-icon {
  width: 52px;
  height: 52px;
  border-radius: 12px;
  border: 1px solid transparent;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.mvd-text h3 {
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0 0 8px 0;
}

.mvd-text p {
  font-size: 13px;
  color: var(--text-muted);
  line-height: 1.7;
  margin: 0;
}

/* Staff Directory Grid */
.staff-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 20px;
  margin-top: 16px;
}

.staff-card {
  background: var(--glass-bg);
  border: 1px solid var(--border-subtle);
  border-radius: 16px;
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
  position: relative;
  overflow: hidden;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.staff-card:hover {
  border-color: var(--border-color);
  transform: translateY(-3px);
}

.staff-card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.staff-avatar {
  width: 56px;
  height: 56px;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  font-weight: 700;
  color: white;
}

.staff-actions {
  display: flex;
  gap: 4px;
}

.staff-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.staff-info h3 {
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.status-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.status-dot.online {
  background-color: #10b981;
  box-shadow: 0 0 8px #10b981;
}

.status-dot.offline {
  background-color: #ef4444;
}

.staff-email {
  font-size: 13px;
  color: var(--text-muted);
}

.role-badge {
  display: inline-flex;
  align-items: center;
  padding: 4px 12px;
  border-radius: 20px;
  font-size: 12px;
  font-weight: 600;
  align-self: flex-start;
  margin-top: 4px;
}

.role-badge.admin {
  background: rgba(99, 102, 241, 0.1);
  color: #818cf8;
  border: 1px solid rgba(99, 102, 241, 0.2);
}

.role-badge.manager {
  background: rgba(249, 115, 22, 0.1);
  color: #fb923c;
  border: 1px solid rgba(249, 115, 22, 0.2);
}

.role-badge.cashier {
  background: rgba(16, 185, 129, 0.1);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.2);
}

.staff-footer {
  margin-top: auto;
  padding-top: 16px;
  border-top: 1px solid var(--border-subtle);
}

.last-login {
  font-size: 12px;
  color: var(--text-muted);
}

.staff-quota-badge {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 3px 9px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 700;
  color: #38bdf8;
  background: rgba(56, 189, 248, 0.08);
  border: 1px solid rgba(56, 189, 248, 0.35);
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

/* Granular Permissions */
.perm-container-card {
  margin-bottom: 16px;
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  padding: 14px;
  background: var(--bg-card-subtle);
}

.perm-section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-bottom: 1px solid var(--border-subtle);
  padding-bottom: 10px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}

.perm-section-title {
  margin: 0;
  font-size: 13.5px;
  font-weight: 700;
  color: var(--text-primary);
}

.perm-section-subtitle {
  font-size: 11.5px;
  color: var(--text-muted);
}

.perm-plan-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2.5px 8px;
  border-radius: 6px;
  font-size: 10px;
  font-weight: 800;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: #ffffff;
  background: linear-gradient(135deg, #0284c7 0%, #6366f1 100%);
}

.perm-items-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}

.perm-item-card {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  cursor: pointer;
  padding: 12px;
  border-radius: 8px;
  border: 1px solid var(--border-subtle);
  background: var(--bg-surface);
  user-select: none;
}

.perm-item-card.perm-active {
  border-color: var(--accent-color);
  background: color-mix(in srgb, var(--accent-color) 6%, var(--bg-surface));
}

.perm-checkbox {
  margin-top: 4px;
  width: 16px;
  height: 16px;
  accent-color: var(--accent-color);
}

.perm-icon-wrap {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  background: var(--bg-card-subtle);
  border-radius: 8px;
  color: var(--text-muted);
  flex-shrink: 0;
}

.perm-info-wrap {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  flex: 1;
}

.perm-name {
  font-size: 13.5px;
  font-weight: 600;
  color: var(--text-primary);
}

.perm-desc {
  font-size: 11.5px;
  color: var(--text-muted);
}

/* Modal Styles */
.modal-overlay {
  position: fixed;
  inset: 0;
  background-color: rgba(2, 6, 23, 0.72);
  backdrop-filter: blur(4px);
  display: flex;
  justify-content: center;
  align-items: center;
  z-index: 1000;
  padding: 16px;
}

.modal-content {
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 12px;
  width: 100%;
  max-width: 500px;
  max-height: calc(100vh - 32px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
}

.modal-header {
  padding: 16px 24px;
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.modal-header h3 {
  margin: 0;
  font-size: 18px;
  font-weight: 600;
  color: var(--text-primary);
}

.close-btn {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  padding: 4px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.close-btn:hover {
  color: var(--text-primary);
}

@container settings-panel (max-width: 860px) {
  .receipt-grid,
  .about-grid,
  .business-details-columns,
  .theme-split-container {
    grid-template-columns: 1fr !important;
  }
  .preview-column {
    max-width: 320px;
    margin: 0 auto;
  }
  .theme-section-left {
    border-right: none;
    border-bottom: 1px solid var(--border-subtle);
    padding-right: 0;
    padding-bottom: 16px;
  }
  .theme-section-right .card-section-header {
    margin-left: 0;
    padding-left: 0;
  }
  .location-details-section {
    border-left: none;
    border-top: 1px solid var(--border-subtle);
  }
}

@container settings-panel (max-width: 680px) {
  .appearance-preferences-row {
    grid-template-columns: 1fr !important;
  }
}

@container settings-panel (max-width: 520px) {
  .theme-preview-container {
    display: none !important;
  }
  .theme-options {
    display: flex;
    flex-direction: row;
    gap: 8px;
  }
  .theme-btn {
    flex: 1;
    min-width: 0;
    padding: 10px 8px;
    flex-direction: row;
    align-items: center;
    justify-content: center;
    gap: 6px;
  }
  .theme-label {
    font-size: 12px;
  }
  .color-options {
    gap: 8px;
  }
  .color-btn {
    width: 34px;
    height: 34px;
  }
}

@media (max-width: 1024px) {
  .settings-page {
    flex-direction: column;
    overflow: visible;
    height: auto;
  }
  .settings-sidebar {
    width: 100%;
    min-width: 0;
  }
  .receipt-grid,
  .about-grid,
  .business-details-columns,
  .theme-split-container {
    grid-template-columns: 1fr !important;
  }
  .preview-column {
    max-width: 320px;
    margin: 0 auto;
    width: 100%;
  }
  .theme-section-left {
    border-right: none;
    border-bottom: 1px solid var(--border-subtle);
    padding-right: 0;
    padding-bottom: 16px;
  }
  .theme-section-right .card-section-header {
    margin-left: 0;
    padding-left: 0;
  }
  .location-details-section {
    border-left: none;
    border-top: 1px solid var(--border-subtle);
  }
}

@media (max-width: 768px) {
  .appearance-preferences-row {
    grid-template-columns: 1fr !important;
  }
}


</style>
