<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, onUnmounted, ref, watch } from 'vue'
import { useDesktopBreakpoint, useMobileOrTabletBreakpoint, usePhoneBreakpoint } from '../utils/breakpoints'
import { useBottomSheetDrag } from '../utils/useBottomSheetDrag'
import {
  BarChart3, Bell, Boxes, BrainCircuit, Calculator, CalendarDays, Cloud, Download, Eye, EyeOff, FileClock, History, LayoutDashboard,
  LockKeyhole, LogOut, Menu, Moon, Package, PackageCheck, PlusSquare, ReceiptText, Settings as SettingsIcon, Share, ShieldCheck, ShoppingCart,
  Store, Sun, Tag, LineChart, Monitor, Users, X, Zap,
} from '@lucide/vue'
import { apiRequest, applyOnlinePrefsToDom, formatDate, formatTime, readStaffPermissions, setApiMode, syncOnlinePrefsFromServer, toggleThemeMode } from '../api'
import { clearChartTooltips } from '../utils/chartTooltip'
import AsyncViewLoading from './AsyncViewLoading.vue'
import DashboardView from './DashboardView.vue'

const PointOfSaleView = defineAsyncComponent({
  loader: () => import('./PointOfSaleView.vue'),
  loadingComponent: AsyncViewLoading,
  delay: 150,
})

const ProductsView = defineAsyncComponent({
  loader: () => import('./ProductsView.vue'),
  loadingComponent: AsyncViewLoading,
  delay: 150,
})

const ReceiptsView = defineAsyncComponent({
  loader: () => import('./ReceiptsView.vue'),
  loadingComponent: AsyncViewLoading,
  delay: 150,
})

const SalesView = defineAsyncComponent({
  loader: () => import('./SalesView.vue'),
  loadingComponent: AsyncViewLoading,
  delay: 150,
})

const StockLevelsView = defineAsyncComponent({
  loader: () => import('./StockLevelsView.vue'),
  loadingComponent: AsyncViewLoading,
  delay: 150,
})

const SettingsView = defineAsyncComponent({
  loader: () => import('./SettingsView.vue'),
  loadingComponent: AsyncViewLoading,
  delay: 150,
})

const ProfitAnalyticsView = defineAsyncComponent({
  loader: () => import('./ProfitAnalyticsView.vue'),
  loadingComponent: AsyncViewLoading,
  delay: 150,
})

const BusinessIntelligenceView = defineAsyncComponent({
  loader: () => import('./BusinessIntelligenceView.vue'),
  loadingComponent: AsyncViewLoading,
  delay: 150,
})

const AuditLogsView = defineAsyncComponent({
  loader: () => import('./AuditLogsView.vue'),
  loadingComponent: AsyncViewLoading,
  delay: 150,
})

type User = {
  id: string
  shopId?: string
  fullName: string
  email: string
  role: string
  shopName: string
  currency: string
  googleConnected?: boolean
  googleEmail?: string
  googlePictureUrl?: string
}

const props = defineProps<{ user: User; guestMode?: boolean; demoMode?: boolean; claimNotice?: string }>()
const emit = defineEmits<{
  'connect-google': [page: string]
  'staff-login-success': [user: User]
  'sign-out': []
  'user-updated': [settings: Pick<User, 'fullName' | 'shopName' | 'currency'>]
}>()
const activePage = ref(props.user.role.toLowerCase() === 'cashier' ? 'pos' : 'dashboard')

watch(() => props.user.role, (newRole) => {
  if (newRole && newRole.toLowerCase() === 'cashier') {
    activePage.value = 'pos'
  }
}, { immediate: true })
const { isDesktop } = useDesktopBreakpoint()
const { isPhone } = usePhoneBreakpoint()
const { isMobileOrTablet } = useMobileOrTabletBreakpoint()
const isDesktopOnly = computed(() => isDesktop.value && !isMobileOrTablet.value && !isPhone.value)
const isGuest = computed(() => props.guestMode === true)
const sidebarCollapsed = ref(false)
const mobileMoreOpen = ref(false)
const topbarActions = ref<HTMLElement | null>(null)
const profileOpen = ref(false)
const notificationsOpen = ref(false)
const notificationLoading = ref(false)
const notificationError = ref('')
const notificationLoaded = ref(false)
const notifications = ref<{ productId: number; name: string; quantity: number; unit: string; outOfStock: boolean; read: boolean }[]>([])
const unreadNotificationCount = computed(() => notifications.value.filter(item => !item.read).length)

interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed'; platform: string }>
}

const deferredInstallPrompt = ref<BeforeInstallPromptEvent | null>(null)
const autoInstallRequested = ref(false)
const isStandalone = ref(false)
const isInstalled = ref(false)
const showIosInstallSheet = ref(false)
const isOnline = ref(typeof navigator !== 'undefined' ? navigator.onLine : true)
const reconnectRefreshKey = ref(0)
let reconnectTimer: number | null = null

function updateOnlineStatus() {
  const wasOffline = !isOnline.value
  const nowOnline = typeof navigator !== 'undefined' ? navigator.onLine : true
  isOnline.value = nowOnline
  if (wasOffline && nowOnline) {
    if (reconnectTimer !== null) window.clearTimeout(reconnectTimer)
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = null
      if (isOnline.value) {
        notificationError.value = ''
        if (notificationLoaded.value) void loadNotifications()
        reconnectRefreshKey.value += 1
      }
    }, 350)
  }
}

const isIos = computed(() => {
  if (typeof navigator === 'undefined') return false
  return /iphone|ipad|ipod/i.test(navigator.userAgent) && !(window as any).MSStream
})

const {
  sheetRef: iosSheetRef,
  sheetStyle: iosSheetStyle,
  onPointerDown: onIosSheetPointerDown,
  onPointerMove: onIosSheetPointerMove,
  onPointerUp: onIosSheetPointerUp,
  onPointerCancel: onIosSheetPointerCancel,
} = useBottomSheetDrag(() => {
  showIosInstallSheet.value = false
})

const canShowInstallRow = computed(() => {
  if (isStandalone.value || isInstalled.value) return false
  if (isIos.value) return true
  return deferredInstallPrompt.value !== null
})

function checkStandalone() {
  if (typeof window === 'undefined') return
  const isDisplayStandalone = window.matchMedia('(display-mode: standalone)').matches ||
    window.matchMedia('(display-mode: minimal-ui)').matches ||
    (navigator as any).standalone === true
  isStandalone.value = isDisplayStandalone
}

async function handleInstallClick() {
  mobileMoreOpen.value = false
  if (isIos.value) {
    showIosInstallSheet.value = true
    return
  }
  if (!deferredInstallPrompt.value) return
  const promptEvent = deferredInstallPrompt.value
  await promptEvent.prompt()
  const choice = await promptEvent.userChoice
  if (choice.outcome === 'accepted') {
    isInstalled.value = true
  }
  deferredInstallPrompt.value = null
}

const navigationGroups = [
  { category: 'MAIN', items: [{ id: 'dashboard', label: 'Dashboard', icon: LayoutDashboard }] },
  { category: 'SALES', items: [
    { id: 'pos', label: 'Point of Sale', icon: Calculator },
    { id: 'sales', label: 'Sales History', icon: History },
    { id: 'receipts', label: 'Receipts', icon: ReceiptText },
  ] },
  { category: 'ANALYTICS', items: [
    { id: 'profit-analytics', label: 'Profit Analytics', icon: LineChart },
    { id: 'business-intelligence', label: 'Business Intelligence', icon: BrainCircuit },
  ] },
  { category: 'INVENTORY', items: [
    { id: 'products', label: 'Products', icon: Tag },
    { id: 'stock-levels', label: 'Stock Levels', icon: Boxes },
  ] },
  { category: 'SYSTEM', items: [
    { id: 'audit-logs', label: 'Audit Logs', icon: FileClock },
    { id: 'settings', label: 'Settings', icon: SettingsIcon },
  ] },
]

const prefsTick = ref(0)

function hasNavPermission(itemId: string): boolean {
  void prefsTick.value
  const role = props.user.role.toLowerCase()
  if (role === 'owner' || role === 'manager' || role === 'administrator') return true
  const perms = readStaffPermissions(props.user.id, props.user.role)
  if (itemId === 'pos') return perms.includes('page_pos')
  if (itemId === 'products' || itemId === 'stock-levels') return perms.includes('page_inventory')
  if (itemId === 'profit-analytics' || itemId === 'business-intelligence') return perms.includes('page_bi')
  if (itemId === 'sales' || itemId === 'receipts') return perms.includes('page_sales_history')
  if (itemId === 'audit-logs') return perms.includes('page_audit')
  if (itemId === 'settings') return perms.includes('page_settings')
  return false
}

const visibleNavigationGroups = computed(() => navigationGroups
  .map(group => ({
    ...group,
    items: group.items.filter(item =>
      hasNavPermission(item.id) &&
      (isDesktop.value || !['profit-analytics', 'business-intelligence', 'audit-logs'].includes(item.id)) &&
      (!isMobileOrTablet.value || item.id !== 'stock-levels'),
    ),
  }))
  .filter(group => group.items.length > 0))
const navigation = computed(() => visibleNavigationGroups.value.flatMap(group => group.items))
const mobilePrimaryNavigation = computed(() => navigation.value.filter(item =>
  ['dashboard', 'pos', 'products'].includes(item.id),
))
const mobileMoreNavigation = computed(() => navigation.value.filter(item =>
  !['dashboard', 'pos', 'products'].includes(item.id),
))
const isMorePageActive = computed(() => mobileMoreNavigation.value.some(item => item.id === activePage.value))
const desktopOnlyPages: Record<string, string> = {
  'profit-analytics': 'Profit Analytics',
  'business-intelligence': 'Business Intelligence',
  'audit-logs': 'Audit Logs',
}
const desktopLockedPageName = computed(() => desktopOnlyPages[activePage.value] ?? '')
const isDesktopLockedPage = computed(() =>
  !isDesktop.value && Object.prototype.hasOwnProperty.call(desktopOnlyPages, activePage.value),
)
const isMobileLockedStockLevels = computed(() =>
  isMobileOrTablet.value && activePage.value === 'stock-levels',
)

function isPageDisabled(pageId: string): boolean {
  if (isMobileOrTablet.value && pageId === 'stock-levels') return true
  if (!isDesktop.value && Object.prototype.hasOwnProperty.call(desktopOnlyPages, pageId)) return true
  return false
}
const pageTitle = computed(() => props.user.shopName || 'My Business')
const now = ref(new Date())
const dateText = computed(() => {
  void prefsTick.value
  return formatDate(now.value).toUpperCase()
})
const timeText = computed(() => {
  void prefsTick.value
  return formatTime(now.value)
})
const userInitials = computed(() => props.user.fullName.trim().split(/\s+/).slice(0, 2).map(part => part[0] ?? '').join('').toUpperCase() || 'U')
const avatarImageFailed = ref(false)
const displayRole = computed(() => props.user.role.charAt(0).toUpperCase() + props.user.role.slice(1))
const isCurrentLight = computed(() => {
  void prefsTick.value
  return typeof document !== 'undefined' && document.documentElement.getAttribute('data-theme') === 'light'
})

function handleThemeToggle() {
  toggleThemeMode()
}

let clockTimer = 0

watch(() => props.user.googlePictureUrl, () => {
  avatarImageFailed.value = false
})

async function loadNotifications() {
  notificationLoading.value = true
  notificationError.value = ''
  try {
    const result = await apiRequest<{ products: { id: number; name: string; quantity: number; minStock: number; unit: string }[] }>('/products')
    const readByProduct = new Map(notifications.value.map(item => [item.productId, item.read]))
    notifications.value = result.products
      .filter(product => product.quantity <= product.minStock)
      .sort((a, b) => Number(a.quantity > 0) - Number(b.quantity > 0) || a.name.localeCompare(b.name))
      .slice(0, 20)
      .map(product => ({
        productId: product.id,
        name: product.name,
        quantity: product.quantity,
        unit: product.unit,
        outOfStock: product.quantity <= 0,
        read: readByProduct.get(product.id) ?? false,
      }))
    notificationLoaded.value = true
  } catch (reason) {
    notificationError.value = reason instanceof Error ? reason.message : 'Stock notifications could not be loaded.'
  } finally {
    notificationLoading.value = false
  }
}

function toggleNotifications() {
  notificationsOpen.value = !notificationsOpen.value
  profileOpen.value = false
  if (notificationsOpen.value) void loadNotifications()
}

function toggleProfile() {
  profileOpen.value = !profileOpen.value
  notificationsOpen.value = false
}

const staffLoginModalOpen = ref(false)
const staffLoginUsername = ref('')
const staffLoginPassword = ref('')
const staffLoginShowPassword = ref(false)
const terminalLoginRole = ref<'Admin' | 'Manager' | 'Cashier'>('Cashier')
const staffLoginLoading = ref(false)
const staffLoginError = ref('')

async function handleStaffLogin() {
  const uname = staffLoginUsername.value.trim()
  if (!uname) {
    staffLoginError.value = 'Please enter your username.'
    return
  }
  if (terminalLoginRole.value === 'Admin' && !staffLoginPassword.value) {
    staffLoginError.value = 'Please enter your administrator password.'
    return
  }
  staffLoginLoading.value = true
  staffLoginError.value = ''
  try {
    setApiMode('cloud')
    const result = await apiRequest<{ user: User }>('/auth/login', {
      method: 'POST',
      body: {
        email: uname,
        password: staffLoginPassword.value,
      },
    })
    staffLoginModalOpen.value = false
    staffLoginUsername.value = ''
    staffLoginPassword.value = ''
    emit('staff-login-success', result.user)
    if (result.user.role.toLowerCase() === 'cashier') {
      activePage.value = 'pos'
    } else {
      activePage.value = 'dashboard'
    }
  } catch (err) {
    staffLoginError.value = err instanceof Error ? err.message : 'Invalid username or password.'
  } finally {
    staffLoginLoading.value = false
  }
}

const targetStockAlertProductId = ref<number | null>(null)

function openStockAlert(productId: number) {
  targetStockAlertProductId.value = productId
  activePage.value = isMobileOrTablet.value ? 'products' : 'stock-levels'
  notificationsOpen.value = false
  const alert = notifications.value.find(item => item.productId === productId)
  if (alert) alert.read = true
}

watch(isMobileOrTablet, (mobileOrTablet) => {
  if (mobileOrTablet && activePage.value === 'stock-levels') {
    activePage.value = 'products'
  }
})

function navigateToPage(pageId: string) {
  if (activePage.value === pageId) {
    mobileMoreOpen.value = false
    return
  }
  if (isPageDisabled(pageId)) {
    return
  }
  activePage.value = pageId
  mobileMoreOpen.value = false
}

function markAllNotificationsRead() {
  notifications.value.forEach(item => { item.read = true })
}

function handleOutsidePointer(event: PointerEvent) {
  if (event.target instanceof Node && !topbarActions.value?.contains(event.target)) {
    profileOpen.value = false
    notificationsOpen.value = false
  }
}

function handleEscape(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    profileOpen.value = false
    notificationsOpen.value = false
    mobileMoreOpen.value = false
  }
}

function handlePrefsUpdated() {
  prefsTick.value++
  applyOnlinePrefsToDom()
  if (!navigation.value.some(item => item.id === activePage.value)
    && !Object.prototype.hasOwnProperty.call(desktopOnlyPages, activePage.value)
    && !isMobileLockedStockLevels.value
    && navigation.value.length > 0) {
    activePage.value = navigation.value[0].id
  }
}

watch(activePage, () => {
  clearChartTooltips()
  mobileMoreOpen.value = false
  void loadNotifications()
})

let displayModeQuery: MediaQueryList | null = null

function handleDisplayModeChange(e: MediaQueryListEvent) {
  isStandalone.value = e.matches
}

function handleBeforeInstallPrompt(e: Event) {
  e.preventDefault()
  deferredInstallPrompt.value = e as BeforeInstallPromptEvent
  if (autoInstallRequested.value && !isStandalone.value) {
    autoInstallRequested.value = false
    void handleInstallClick()
  }
}

function handleAppInstalled() {
  isInstalled.value = true
  deferredInstallPrompt.value = null
  showIosInstallSheet.value = false
  autoInstallRequested.value = false
}

onMounted(() => {
  checkStandalone()
  // Prefetch the most-used secondary tabs during browser idle time so switching is instant
  const prefetchTabs = () => {
    void import('./PointOfSaleView.vue')
    void import('./ProductsView.vue')
  }
  if (typeof window !== 'undefined') {
    const win = window as Window & { requestIdleCallback?: (cb: () => void, opts?: { timeout: number }) => number }
    if (typeof win.requestIdleCallback === 'function') {
      win.requestIdleCallback(prefetchTabs, { timeout: 2000 })
    } else {
      window.setTimeout(prefetchTabs, 1200)
    }
  }
  if (typeof window !== 'undefined' && window.matchMedia) {
    displayModeQuery = window.matchMedia('(display-mode: standalone)')
    displayModeQuery.addEventListener('change', handleDisplayModeChange)
  }
  window.addEventListener('beforeinstallprompt', handleBeforeInstallPrompt)
  window.addEventListener('appinstalled', handleAppInstalled)
  window.addEventListener('online', updateOnlineStatus)
  window.addEventListener('offline', updateOnlineStatus)

  const params = new URLSearchParams(window.location.search)
  const shouldAutoInstall = params.get('install') === '1' || params.get('action') === 'install'
  if (shouldAutoInstall && !isStandalone.value) {
    if (isIos.value) {
      showIosInstallSheet.value = true
    } else {
      autoInstallRequested.value = true
      if (deferredInstallPrompt.value) {
        autoInstallRequested.value = false
        void handleInstallClick()
      }
    }
    params.delete('install')
    params.delete('action')
    const query = params.toString()
    window.history.replaceState({}, document.title, `${window.location.pathname}${query ? `?${query}` : ''}`)
  }

  const requestedPage = params.get('page')
  if (requestedPage && (
    navigation.value.some(item => item.id === requestedPage) ||
    Object.prototype.hasOwnProperty.call(desktopOnlyPages, requestedPage) ||
    (isMobileOrTablet.value && requestedPage === 'stock-levels')
  )) {
    activePage.value = isMobileOrTablet.value && requestedPage === 'stock-levels' ? 'products' : requestedPage
    if (params.get('claim') !== 'guest') params.delete('page')
    params.delete('google_linked')
    const query = params.toString()
    window.history.replaceState({}, document.title, `${window.location.pathname}${query ? `?${query}` : ''}`)
  }
  applyOnlinePrefsToDom()
  void syncOnlinePrefsFromServer(props.user.id)
  if (!navigation.value.some(item => item.id === activePage.value)
    && !Object.prototype.hasOwnProperty.call(desktopOnlyPages, activePage.value)
    && !isMobileLockedStockLevels.value
    && navigation.value.length > 0) {
    activePage.value = navigation.value[0].id
  }
  void loadNotifications()
  clockTimer = window.setInterval(() => { now.value = new Date() }, 1000)
  ;(window as any).__navigate = (pageId: string) => { activePage.value = pageId }
  document.addEventListener('pointerdown', handleOutsidePointer)
  window.addEventListener('keydown', handleEscape)
  window.addEventListener('online-prefs-updated', handlePrefsUpdated)
})
onUnmounted(() => {
  window.clearInterval(clockTimer)
  if (reconnectTimer !== null) window.clearTimeout(reconnectTimer)
  document.removeEventListener('pointerdown', handleOutsidePointer)
  window.removeEventListener('keydown', handleEscape)
  window.removeEventListener('online-prefs-updated', handlePrefsUpdated)
  window.removeEventListener('beforeinstallprompt', handleBeforeInstallPrompt)
  window.removeEventListener('appinstalled', handleAppInstalled)
  window.removeEventListener('online', updateOnlineStatus)
  window.removeEventListener('offline', updateOnlineStatus)
  if (displayModeQuery) {
    displayModeQuery.removeEventListener('change', handleDisplayModeChange)
  }
})
</script>

<template>
  <div class="workspace-shell" :class="{ 'sidebar-collapsed': sidebarCollapsed }">
    <aside class="workspace-sidebar">
      <a class="brand workspace-brand" href="/" aria-label="CM RetailOS Online">
        <img class="workspace-brand-logo" src="/cmretailos-brand-logo.png" alt="" />
        <span class="workspace-brand-copy"><strong>CM RetailOS</strong><small>Smart business management system</small></span>
      </a>
      <div class="workspace-header-divider"></div>
      <nav class="workspace-nav" aria-label="Main navigation">
        <div v-for="group in visibleNavigationGroups" :key="group.category" class="workspace-nav-group">
          <p v-if="group.category && group.category !== 'MAIN'" class="nav-caption">{{ group.category }}</p>
          <button
            v-for="item in group.items"
            :key="item.id"
            type="button"
            :class="['workspace-nav-item', { active: activePage === item.id, disabled: activePage === item.id || isPageDisabled(item.id) }]"
            :aria-current="activePage === item.id ? 'page' : undefined"
            :disabled="activePage === item.id || isPageDisabled(item.id)"
            :title="item.label"
            @click="navigateToPage(item.id)"
          >
            <component :is="item.icon" :size="19" />
            <span>{{ item.label }}</span>
            <span v-if="activePage === item.id" class="nav-dot"></span>
          </button>
        </div>

        <div v-if="user.role.toLowerCase() === 'cashier' && !sidebarCollapsed" class="sidebar-cashier-banner">
          <div class="info-graphic"><ShoppingCart :size="40" /></div>
          <div class="info-brand">CM RETAILOS</div>
          <h3 class="info-heading">Cashier Terminal</h3>
          <p class="info-text">Process sales, manage cart items, and assist customers securely.</p>
          <div class="info-secure"><ShieldCheck :size="12" /><span>Secure cashier session</span></div>
          <div class="info-about">
            <div class="about-divider"></div>
            <div class="about-features">
              <span class="about-feature"><Zap :size="12" /> Fast Checkout</span>
              <span class="about-feature"><BarChart3 :size="12" /> Live Reports</span>
              <span class="about-feature"><PackageCheck :size="12" /> Stock Tracking</span>
              <span class="about-feature"><LockKeyhole :size="12" /> Secure Access</span>
            </div>
            <div class="about-copyright">© 2026 CM RetailOS</div>
          </div>
        </div>

        <div v-if="user.role.toLowerCase() === 'manager' && !sidebarCollapsed" class="sidebar-manager-banner">
          <div class="info-graphic"><ShieldCheck :size="40" /></div>
          <div class="info-brand">CM RETAILOS</div>
          <h3 class="info-heading">Manager Terminal</h3>
          <p class="info-text">Oversee store operations, monitor inventory, and assist cashier staff.</p>
          <div class="info-secure"><ShieldCheck :size="12" /><span>Manager supervisor session</span></div>
          <div class="info-about">
            <div class="about-divider"></div>
            <div class="about-features">
              <span class="about-feature"><Zap :size="12" /> Staff Oversight</span>
              <span class="about-feature"><BarChart3 :size="12" /> Sales Analytics</span>
              <span class="about-feature"><PackageCheck :size="12" /> Stock Control</span>
              <span class="about-feature"><LockKeyhole :size="12" /> Secure Access</span>
            </div>
            <div class="about-copyright">© 2026 CM RetailOS</div>
          </div>
        </div>
      </nav>
      <div class="workspace-sidebar-footer">
        <span class="footer-version">v3.0.9</span>
        <span class="workspace-sidebar-status"><i></i> {{ isGuest ? 'Local' : 'Cloud' }}</span>
      </div>
    </aside>

    <div class="workspace-main">
      <header class="workspace-topbar">
        <div class="workspace-topbar-title">
          <span v-if="!isPhone" class="mobile-shop-mark"><Store :size="18" /></span>
          <button class="desktop-shop-icon" type="button" aria-label="Toggle sidebar" @click="sidebarCollapsed = !sidebarCollapsed"><Store :size="25" /></button>
          <div><h1>{{ pageTitle }}</h1></div>
          <span v-if="!isOnline" class="offline-chip" role="status" aria-label="Offline mode">
            <span></span> Offline
          </span>
        </div>
        <div ref="topbarActions" class="topbar-actions">
          <span v-if="demoMode" class="demo-badge"><span></span> Demo</span>
          <span v-if="!isOnline" class="connection-chip offline">
            <span></span>Offline
          </span>
          <span v-else class="connection-chip" :class="{ connected: !isGuest }">
            <span></span>{{ isGuest ? 'Not connected' : `Connected as ${user.googleEmail || user.email}` }}
          </span>
          <button v-if="isGuest" type="button" class="connect-google-button" @click="emit('connect-google', activePage)">
            <svg class="google-icon" viewBox="0 0 24 24" width="14" height="14" style="margin-right: 5px; flex-shrink: 0;">
              <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
              <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
              <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"/>
              <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"/>
            </svg>
            <span>Sign in with Google</span>
          </button>
          <button
            v-if="isDesktopOnly"
            type="button"
            class="staff-signin-topbar-btn"
            @click="staffLoginModalOpen = true; staffLoginError = ''"
            :title="user.role.toLowerCase() === 'cashier' || user.role.toLowerCase() === 'manager' ? 'Switch staff account' : 'Sign in with staff username'"
          >
            <Users :size="14" />
            <span>{{ user.role.toLowerCase() === 'cashier' || user.role.toLowerCase() === 'manager' ? 'Switch Staff' : 'Staff Sign In' }}</span>
          </button>
          <button
            v-if="isDesktopOnly && (user.role.toLowerCase() === 'cashier' || user.role.toLowerCase() === 'manager')"
            type="button"
            class="staff-signin-topbar-btn exit-admin-btn"
            title="Exit terminal and return to Admin workspace"
            @click="emit('sign-out')"
            style="background: rgba(239, 68, 68, 0.12); border-color: rgba(239, 68, 68, 0.35); color: #f87171;"
          >
            <LogOut :size="14" />
            <span>Exit to Admin</span>
          </button>
          <div class="workspace-clock">
            <CalendarDays :size="15" />
            <span><strong>{{ dateText }}</strong><small>{{ timeText }}</small></span>
          </div>
          <div class="topbar-menu">
            <button class="topbar-icon topbar-menu-trigger" type="button" aria-label="Notifications" :aria-expanded="notificationsOpen" @click="toggleNotifications">
              <Bell :size="19" />
              <span v-if="unreadNotificationCount" class="notification-count">{{ unreadNotificationCount > 9 ? '9+' : unreadNotificationCount }}</span>
            </button>
            <section v-if="notificationsOpen" class="topbar-popover notification-popover" aria-label="Notifications">
              <header class="popover-heading">
                <div><h2>Notifications</h2><small>{{ unreadNotificationCount }} unread stock alert{{ unreadNotificationCount === 1 ? '' : 's' }}</small></div>
                <div class="popover-heading-actions">
                  <button v-if="unreadNotificationCount" type="button" @click="markAllNotificationsRead">Mark all read</button>
                  <button type="button" aria-label="Close notifications" @click="notificationsOpen = false"><X :size="15" /></button>
                </div>
              </header>
              <p v-if="notificationError" class="popover-error" role="alert">{{ notificationError }}</p>
              <p v-else-if="notificationLoading && !notifications.length" class="popover-empty">Checking stock alerts…</p>
              <p v-else-if="!notifications.length" class="popover-empty"><Bell :size="22" /><span>You're all caught up!</span></p>
              <div v-else class="notification-list">
                <button v-for="item in notifications" :key="item.productId" type="button" class="notification-item" :class="{ unread: !item.read, 'notification-out': item.outOfStock }" @click="openStockAlert(item.productId)">
                  <span class="notification-mark"><Package :size="15" /></span>
                  <span class="notification-copy"><strong>{{ item.outOfStock ? 'Out of stock' : 'Low stock' }} · {{ item.name }}</strong><small>{{ item.outOfStock ? 'No stock remaining' : `${item.quantity} ${item.unit} remaining` }}</small></span>
                  <i v-if="!item.read"></i>
                </button>
              </div>
              <footer class="popover-footer"><button type="button" :disabled="notificationLoading" @click="loadNotifications">Refresh stock alerts</button></footer>
            </section>
          </div>
          <div class="topbar-menu">
            <button class="topbar-user" type="button" :aria-label="isGuest ? 'Open guest workspace menu' : `Open profile menu for ${user.fullName}`" :aria-expanded="profileOpen" @click="toggleProfile">
              <span class="profile-avatar">
                <img v-if="user.googlePictureUrl && !avatarImageFailed" :src="user.googlePictureUrl" alt="" referrerpolicy="no-referrer" @error="avatarImageFailed = true" />
                <template v-else>{{ userInitials }}</template>
              </span>
            </button>
            <section v-if="profileOpen" class="topbar-popover profile-popover" aria-label="Account menu">
              <div class="popover-profile">
                <span class="profile-avatar profile-avatar-large">
                  <img v-if="!isGuest && user.googlePictureUrl && !avatarImageFailed" :src="user.googlePictureUrl" alt="" referrerpolicy="no-referrer" @error="avatarImageFailed = true" />
                  <template v-else>{{ userInitials }}</template>
                </span>
                <div class="popover-profile-details">
                  <strong>{{ isGuest ? 'Guest workspace' : user.fullName }}</strong>
                  <small>{{ isGuest ? 'Data stays on this device' : (user.googleEmail || user.email) }}</small>
                </div>
                <button
                  type="button"
                  class="profile-theme-toggle"
                  :title="isCurrentLight ? 'Switch to dark mode' : 'Switch to light mode'"
                  :aria-label="isCurrentLight ? 'Switch to dark mode' : 'Switch to light mode'"
                  @click.stop="handleThemeToggle"
                >
                  <Sun v-if="isCurrentLight" :size="17" class="theme-sun-icon" />
                  <Moon v-else :size="17" class="theme-moon-icon" />
                </button>
              </div>
              <div class="popover-role"><ShieldCheck :size="14" /><span>{{ isGuest ? 'Local guest' : displayRole }}</span><i></i><small>{{ isGuest ? (demoMode ? 'Demo data' : 'Local device') : 'Connected' }}</small></div>
              <div class="popover-menu">
                <button v-if="hasNavPermission('settings')" type="button" @click="activePage = 'settings'; profileOpen = false"><SettingsIcon :size="15" />Settings</button>
                <button v-if="isDesktopOnly" type="button" @click="profileOpen = false; staffLoginModalOpen = true; staffLoginError = ''">
                  <Users :size="15" />
                  <span>{{ user.role.toLowerCase() === 'cashier' || user.role.toLowerCase() === 'manager' ? 'Switch Staff Account' : 'Staff Sign In' }}</span>
                </button>
                <button
                  v-if="isDesktopOnly && (user.role.toLowerCase() === 'cashier' || user.role.toLowerCase() === 'manager')"
                  type="button"
                  @click="profileOpen = false; emit('sign-out')"
                  style="color: #f87171;"
                >
                  <LogOut :size="15" />
                  <span>Exit to Admin Dashboard</span>
                </button>
                <button v-else-if="isGuest" type="button" @click="profileOpen = false; emit('connect-google', activePage)">
                  <svg class="google-icon" viewBox="0 0 24 24" width="15" height="15" style="margin-right: 6px; flex-shrink: 0;">
                    <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
                    <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
                    <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"/>
                    <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"/>
                  </svg>
                  <span>Sign in with Google</span>
                </button>
                <button v-else type="button" @click="profileOpen = false; emit('sign-out')"><LogOut :size="15" />Sign out</button>
              </div>
            </section>
          </div>
        </div>
      </header>

      <div v-if="claimNotice" class="claim-notice" role="status">
        <span>{{ claimNotice }}</span>
        <button type="button" @click="emit('connect-google', activePage)">Sign in with Google</button>
      </div>

      <main class="workspace-content">
        <Transition name="view-fade" mode="out-in">
          <section v-if="isMobileLockedStockLevels" key="mobile-stock-locked" class="desktop-only-state" role="status" aria-live="polite">
            <div class="desktop-only-icon"><Boxes :size="30" /></div>
            <h2>Stock Levels</h2>
            <p>Stock Levels is available on desktop. Use Products to view and adjust stock on tablet and mobile.</p>
          </section>
          <section v-else-if="isDesktopLockedPage" key="desktop-locked" class="desktop-only-state" role="status" aria-live="polite">
            <div class="desktop-only-icon"><Monitor :size="30" /></div>
            <h2>{{ desktopLockedPageName }}</h2>
            <p>{{ desktopLockedPageName }} is available on desktop — open CM RetailOS on a computer.</p>
          </section>
          <DashboardView v-else-if="activePage === 'dashboard'" :key="`dashboard-${reconnectRefreshKey}`" :currency="user.currency" :role="user.role" @navigate="activePage = $event" />
          <PointOfSaleView v-else-if="activePage === 'pos'" :key="`pos-${reconnectRefreshKey}`" :currency="user.currency" :shop-name="user.shopName" :role="user.role" />
          <ProductsView
            v-else-if="activePage === 'products'"
            :key="`products-${reconnectRefreshKey}`"
            :currency="user.currency"
            :role="user.role"
            :user-email="user.email"
            :google-connected="user.googleConnected"
            :target-stock-product-id="targetStockAlertProductId"
            @navigate-settings="activePage = 'settings'"
            @clear-target-stock-product="targetStockAlertProductId = null"
            @stock-adjusted="loadNotifications"
          />
          <StockLevelsView v-else-if="activePage === 'stock-levels' && !isMobileOrTablet" :key="`stock-levels-${reconnectRefreshKey}`" :currency="user.currency" :role="user.role" />
          <ReceiptsView v-else-if="activePage === 'receipts'" :key="`receipts-${reconnectRefreshKey}`" :shop-name="user.shopName" :currency="user.currency" />
          <SalesView v-else-if="activePage === 'sales'" :key="`sales-${reconnectRefreshKey}`" :shop-name="user.shopName" :currency="user.currency" />
          <ProfitAnalyticsView v-else-if="activePage === 'profit-analytics'" :key="`profit-analytics-${reconnectRefreshKey}`" :shop-name="user.shopName" :currency="user.currency" />
          <BusinessIntelligenceView v-else-if="activePage === 'business-intelligence'" :key="`business-intelligence-${reconnectRefreshKey}`" :shop-name="user.shopName" :currency="user.currency" />
          <AuditLogsView v-else-if="activePage === 'audit-logs'" :key="`audit-logs-${reconnectRefreshKey}`" />
          <SettingsView v-else-if="activePage === 'settings'" :key="`settings-${reconnectRefreshKey}`" :shop-name="user.shopName" :currency="user.currency" :role="user.role" :user-id="user.id" :user-name="user.fullName" :user-email="user.email" @user-updated="emit('user-updated', $event)" />
        </Transition>
      </main>

      <nav class="mobile-bottom-nav" aria-label="Mobile main navigation">
        <button
          v-for="item in mobilePrimaryNavigation"
          :key="item.id"
          type="button"
          :class="{ active: activePage === item.id, disabled: activePage === item.id || isPageDisabled(item.id) }"
          :aria-current="activePage === item.id ? 'page' : undefined"
          :disabled="activePage === item.id || isPageDisabled(item.id)"
          @click="navigateToPage(item.id)"
        >
          <span class="nav-icon-container">
            <component :is="item.icon" :size="20" />
          </span>
          <span class="nav-label">{{ item.id === 'dashboard' ? 'Dashboard' : item.id === 'products' ? 'Products' : 'POS' }}</span>
        </button>
        <button
          v-if="mobileMoreNavigation.length"
          type="button"
          :class="{ active: isMorePageActive || mobileMoreOpen }"
          aria-haspopup="menu"
          :aria-expanded="mobileMoreOpen"
          aria-controls="mobile-more-menu"
          @click="mobileMoreOpen = !mobileMoreOpen"
        >
          <span class="nav-icon-container">
            <Menu :size="20" />
          </span>
          <span class="nav-label">More</span>
        </button>
        <div
          v-if="mobileMoreOpen"
          id="mobile-more-menu"
          class="mobile-more-menu"
          role="menu"
          aria-label="More pages"
        >
          <button
            v-for="item in mobileMoreNavigation"
            :key="item.id"
            type="button"
            role="menuitem"
            :class="{ active: activePage === item.id, disabled: activePage === item.id || isPageDisabled(item.id) }"
            :aria-current="activePage === item.id ? 'page' : undefined"
            :disabled="activePage === item.id || isPageDisabled(item.id)"
            @click="navigateToPage(item.id)"
          >
            <component :is="item.icon" :size="18" />
            <span>{{ item.label }}</span>
          </button>
          <button
            v-if="isGuest"
            type="button"
            role="menuitem"
            class="google-signin-menu-item"
            @click="mobileMoreOpen = false; emit('connect-google', activePage)"
          >
            <svg class="google-icon" viewBox="0 0 24 24" width="18" height="18">
              <path fill="#4285F4" d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"/>
              <path fill="#34A853" d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"/>
              <path fill="#FBBC05" d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"/>
              <path fill="#EA4335" d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"/>
            </svg>
            <span>Sign in with Google</span>
          </button>
          <button
            v-if="canShowInstallRow"
            type="button"
            role="menuitem"
            class="install-app-menu-item"
            @click="handleInstallClick"
          >
            <Download :size="18" />
            <span>Install app</span>
          </button>
        </div>
      </nav>

      <!-- iOS Install Instructions Sheet -->
      <div
        v-if="showIosInstallSheet"
        class="ios-install-backdrop"
        @click.self="showIosInstallSheet = false"
      >
        <div
          ref="iosSheetRef"
          class="ios-install-sheet"
          :style="iosSheetStyle"
          role="dialog"
          aria-modal="true"
          aria-label="Install CM RetailOS"
        >
          <div
            class="ios-sheet-handle-area"
            @pointerdown="onIosSheetPointerDown"
            @pointermove="onIosSheetPointerMove"
            @pointerup="onIosSheetPointerUp"
            @pointercancel="onIosSheetPointerCancel"
          >
            <div class="ios-sheet-handle"></div>
          </div>
          <div
            class="ios-sheet-header"
            @pointerdown="onIosSheetPointerDown"
            @pointermove="onIosSheetPointerMove"
            @pointerup="onIosSheetPointerUp"
            @pointercancel="onIosSheetPointerCancel"
          >
            <div class="ios-sheet-app-icon">
              <img src="/icons/cm-retailos-192.png" alt="CM RetailOS" />
            </div>
            <div>
              <h3>Install CM RetailOS</h3>
              <p>Add to your home screen for fast offline access</p>
            </div>
            <button
              type="button"
              class="ios-sheet-close"
              aria-label="Close"
              @click="showIosInstallSheet = false"
            >
              <X :size="16" />
            </button>
          </div>
          <div class="ios-sheet-steps">
            <div class="ios-sheet-step">
              <span class="step-num">1</span>
              <p>Tap the <strong>Share</strong> button <Share :size="15" class="inline-sheet-icon" /> in Safari's toolbar.</p>
            </div>
            <div class="ios-sheet-step">
              <span class="step-num">2</span>
              <p>Scroll down and tap <strong>Add to Home Screen</strong> <PlusSquare :size="15" class="inline-sheet-icon" />.</p>
            </div>
          </div>
          <button
            type="button"
            class="ios-sheet-action"
            @click="showIosInstallSheet = false"
          >
            Got it
          </button>
        </div>
      </div>
    </div>

    <!-- Staff / Terminal Sign In Modal (Desktop Only) -->
    <Teleport to="body">
      <div v-if="isDesktopOnly && staffLoginModalOpen" class="modal-overlay" style="z-index: 1200;">
        <div class="modal-content" style="max-width: 420px; border-radius: 16px; overflow: hidden; padding: 24px;">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 14px;">
            <h3 style="margin: 0; font-size: 17px; font-weight: 700; display: flex; align-items: center; gap: 8px;">
              <Users :size="19" style="color: var(--accent-color);" />
              Terminal Sign In
            </h3>
            <button type="button" class="close-btn" @click="staffLoginModalOpen = false"><X :size="18" /></button>
          </div>

          <!-- Role Toggle Buttons -->
          <div style="display: grid; grid-template-columns: repeat(3, 1fr); gap: 6px; background: rgba(0,0,0,0.18); padding: 4px; border-radius: 10px; margin-bottom: 16px; border: 1px solid var(--border-color);">
            <button
              type="button"
              @click="terminalLoginRole = 'Admin'; staffLoginError = ''"
              style="padding: 8px 12px; font-size: 13px; font-weight: 600; border-radius: 8px; border: none; cursor: pointer; transition: all 0.2s;"
              :style="terminalLoginRole === 'Admin' ? 'background: var(--accent-color); color: #fff;' : 'background: transparent; color: var(--text-muted);'"
            >
              Admin
            </button>
            <button
              type="button"
              @click="terminalLoginRole = 'Manager'; staffLoginError = ''"
              style="padding: 8px 12px; font-size: 13px; font-weight: 600; border-radius: 8px; border: none; cursor: pointer; transition: all 0.2s;"
              :style="terminalLoginRole === 'Manager' ? 'background: var(--accent-color); color: #fff;' : 'background: transparent; color: var(--text-muted);'"
            >
              Manager
            </button>
            <button
              type="button"
              @click="terminalLoginRole = 'Cashier'; staffLoginError = ''"
              style="padding: 8px 12px; font-size: 13px; font-weight: 600; border-radius: 8px; border: none; cursor: pointer; transition: all 0.2s;"
              :style="terminalLoginRole === 'Cashier' ? 'background: var(--accent-color); color: #fff;' : 'background: transparent; color: var(--text-muted);'"
            >
              Cashier
            </button>
          </div>

          <p style="color: var(--text-muted); font-size: 13px; margin: 0 0 16px; line-height: 1.5;">
            <span v-if="terminalLoginRole === 'Admin'">Administrator access requires username/email and password.</span>
            <span v-else-if="terminalLoginRole === 'Manager'">Store Managers log in using their username and password.</span>
            <span v-else>Cashiers log in using their assigned username. No password required.</span>
          </p>

          <form @submit.prevent="handleStaffLogin">
            <div class="input-group full-width" style="margin-bottom: 14px;">
              <label style="font-weight: 600; font-size: 12px; margin-bottom: 6px; display: block; color: var(--text-primary);">
                {{ terminalLoginRole === 'Admin' ? 'Username or Email *' : 'Username *' }}
              </label>
              <input
                type="text"
                v-model="staffLoginUsername"
                :placeholder="terminalLoginRole === 'Admin' ? 'admin or your email' : (terminalLoginRole === 'Manager' ? 'e.g. manager' : 'e.g. cashier')"
                required
                autofocus
                style="width: 100%; padding: 10px 14px; background: var(--bg-sidebar); border: 1px solid var(--border-color); border-radius: 8px; color: var(--text-primary); font-size: 14px;"
              />
            </div>

            <!-- Password field for Admin and Manager -->
            <div v-if="terminalLoginRole !== 'Cashier'" class="input-group full-width" style="margin-bottom: 16px;">
              <label style="font-weight: 600; font-size: 12px; margin-bottom: 6px; display: block; color: var(--text-primary);">
                Password {{ terminalLoginRole === 'Admin' ? '*' : '(if assigned)' }}
              </label>
              <div style="position: relative; display: flex; align-items: center;">
                <input
                  :type="staffLoginShowPassword ? 'text' : 'password'"
                  v-model="staffLoginPassword"
                  placeholder="Enter your password"
                  :required="terminalLoginRole === 'Admin'"
                  style="width: 100%; padding: 10px 40px 10px 14px; background: var(--bg-sidebar); border: 1px solid var(--border-color); border-radius: 8px; color: var(--text-primary); font-size: 14px;"
                />
                <button
                  type="button"
                  @click="staffLoginShowPassword = !staffLoginShowPassword"
                  style="position: absolute; right: 10px; background: none; border: none; color: var(--text-muted); cursor: pointer; display: flex; align-items: center; padding: 4px;"
                >
                  <EyeOff v-if="staffLoginShowPassword" :size="16" />
                  <Eye v-else :size="16" />
                </button>
              </div>
            </div>

            <p v-if="staffLoginError" style="color: #ef4444; font-size: 13px; margin: 0 0 14px; padding: 8px 12px; background: rgba(239,68,68,0.1); border-radius: 6px; border: 1px solid rgba(239,68,68,0.25);">
              {{ staffLoginError }}
            </p>
            <div style="display: flex; justify-content: flex-end; gap: 10px; margin-top: 10px;">
              <button type="button" class="btn btn-outline" @click="staffLoginModalOpen = false">Cancel</button>
              <button type="submit" class="btn btn-primary" :disabled="!staffLoginUsername.trim() || staffLoginLoading" style="padding: 9px 20px;">
                {{ staffLoginLoading ? 'Signing in...' : 'Sign In' }}
              </button>
            </div>
            <div v-if="user.role.toLowerCase() === 'cashier' || user.role.toLowerCase() === 'manager'" style="margin-top: 16px; padding-top: 14px; border-top: 1px solid var(--border-color); text-align: center;">
              <button
                type="button"
                @click="staffLoginModalOpen = false; emit('sign-out')"
                style="background: none; border: none; color: var(--accent-color); font-size: 13px; font-weight: 600; cursor: pointer; text-decoration: underline;"
              >
                Return to Admin / Owner Dashboard
              </button>
            </div>
          </form>
        </div>
      </div>
    </Teleport>
  </div>
</template>
