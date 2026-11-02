<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useDesktopBreakpoint } from '../utils/breakpoints'
import {
  BarChart3, Bell, Boxes, BrainCircuit, Calculator, CalendarDays, FileClock, History, LayoutDashboard,
  LockKeyhole, LogOut, Menu, Package, PackageCheck, ReceiptText, Settings as SettingsIcon, ShieldCheck, ShoppingCart,
  Store, Tag, LineChart, Monitor, X, Zap,
} from '@lucide/vue'
import { apiRequest, applyOnlinePrefsToDom, formatDate, formatTime, readStaffPermissions, syncOnlinePrefsFromServer } from '../api'
import AuditLogsView from './AuditLogsView.vue'
import DashboardView from './DashboardView.vue'
import BusinessIntelligenceView from './BusinessIntelligenceView.vue'
import PointOfSaleView from './PointOfSaleView.vue'
import ProfitAnalyticsView from './ProfitAnalyticsView.vue'
import ProductsView from './ProductsView.vue'
import ReceiptsView from './ReceiptsView.vue'
import SalesView from './SalesView.vue'
import StockLevelsView from './StockLevelsView.vue'
import SettingsView from './SettingsView.vue'

type User = {
  id: string
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
  'sign-out': []
  'user-updated': [settings: Pick<User, 'fullName' | 'shopName' | 'currency'>]
}>()
const activePage = ref('dashboard')
const { isDesktop } = useDesktopBreakpoint()
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
      (isDesktop.value || !['profit-analytics', 'business-intelligence', 'audit-logs'].includes(item.id)),
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

function openStockAlert(productId: number) {
  activePage.value = 'stock-levels'
  notificationsOpen.value = false
  const alert = notifications.value.find(item => item.productId === productId)
  if (alert) alert.read = true
}

function navigateToPage(pageId: string) {
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
    && navigation.value.length > 0) {
    activePage.value = navigation.value[0].id
  }
}

watch(activePage, () => {
  mobileMoreOpen.value = false
  void loadNotifications()
})

onMounted(() => {
  const params = new URLSearchParams(window.location.search)
  const requestedPage = params.get('page')
  if (requestedPage && (
    navigation.value.some(item => item.id === requestedPage) ||
    Object.prototype.hasOwnProperty.call(desktopOnlyPages, requestedPage)
  )) {
    activePage.value = requestedPage
    if (params.get('claim') !== 'guest') params.delete('page')
    params.delete('google_linked')
    const query = params.toString()
    window.history.replaceState({}, document.title, `${window.location.pathname}${query ? `?${query}` : ''}`)
  }
  applyOnlinePrefsToDom()
  void syncOnlinePrefsFromServer(props.user.id)
  if (!navigation.value.some(item => item.id === activePage.value)
    && !Object.prototype.hasOwnProperty.call(desktopOnlyPages, activePage.value)
    && navigation.value.length > 0) {
    activePage.value = navigation.value[0].id
  }
  void loadNotifications()
  clockTimer = window.setInterval(() => { now.value = new Date() }, 1000)
  document.addEventListener('pointerdown', handleOutsidePointer)
  window.addEventListener('keydown', handleEscape)
  window.addEventListener('online-prefs-updated', handlePrefsUpdated)
})
onUnmounted(() => {
  window.clearInterval(clockTimer)
  document.removeEventListener('pointerdown', handleOutsidePointer)
  window.removeEventListener('keydown', handleEscape)
  window.removeEventListener('online-prefs-updated', handlePrefsUpdated)
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
            :class="['workspace-nav-item', { active: activePage === item.id }]"
            :aria-current="activePage === item.id ? 'page' : undefined"
            :title="item.label"
            @click="activePage = item.id"
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
      </nav>
      <div class="workspace-sidebar-footer">
        <span class="footer-version">v3.0.8</span>
        <span class="workspace-sidebar-status"><i></i> {{ isGuest ? 'Local' : 'Cloud' }}</span>
      </div>
    </aside>

    <div class="workspace-main">
      <header class="workspace-topbar">
        <div class="workspace-topbar-title">
          <span class="mobile-shop-mark"><Store :size="18" /></span>
          <button class="desktop-shop-icon" type="button" aria-label="Toggle sidebar" @click="sidebarCollapsed = !sidebarCollapsed"><Store :size="25" /></button>
          <div><h1>{{ pageTitle }}</h1></div>
        </div>
        <div ref="topbarActions" class="topbar-actions">
          <span v-if="demoMode" class="demo-badge"><span></span> Demo</span>
          <span class="connection-chip" :class="{ connected: !isGuest }">
            <span></span>{{ isGuest ? 'Not connected' : `Connected as ${user.googleEmail || user.email}` }}
          </span>
          <button v-if="isGuest" type="button" class="connect-google-button" @click="emit('connect-google', activePage)">Connect Google</button>
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
                  <template v-else>{{ isGuest ? 'D' : userInitials }}</template>
                </span>
                <div><strong>{{ isGuest ? 'Guest workspace' : user.fullName }}</strong><small>{{ isGuest ? 'Data stays on this device' : (user.googleEmail || user.email) }}</small></div>
              </div>
              <div class="popover-role"><ShieldCheck :size="14" /><span>{{ isGuest ? 'Local guest' : displayRole }}</span><i></i><small>{{ isGuest ? 'Demo data' : 'Connected' }}</small></div>
              <div class="popover-menu">
                <button v-if="hasNavPermission('settings')" type="button" @click="activePage = 'settings'; profileOpen = false"><SettingsIcon :size="15" />Settings</button>
                <button v-if="isGuest" type="button" @click="profileOpen = false; emit('connect-google', activePage)"><Store :size="15" />Save to cloud</button>
                <button v-else type="button" @click="profileOpen = false; emit('sign-out')"><LogOut :size="15" />Sign out</button>
              </div>
            </section>
          </div>
        </div>
      </header>

      <div v-if="claimNotice" class="claim-notice" role="status">
        <span>{{ claimNotice }}</span>
        <button type="button" @click="emit('connect-google', activePage)">Retry save to cloud</button>
      </div>

      <main class="workspace-content">
        <section v-if="isDesktopLockedPage" class="desktop-only-state" role="status" aria-live="polite">
          <div class="desktop-only-icon"><Monitor :size="30" /></div>
          <h2>{{ desktopLockedPageName }}</h2>
          <p>{{ desktopLockedPageName }} is available on desktop — open CM RetailOS on a computer.</p>
        </section>
        <DashboardView v-else-if="activePage === 'dashboard'" :currency="user.currency" :role="user.role" @navigate="activePage = $event" />
        <PointOfSaleView v-else-if="activePage === 'pos'" :currency="user.currency" :shop-name="user.shopName" :role="user.role" />
        <ProductsView v-else-if="activePage === 'products'" :currency="user.currency" :role="user.role" :user-email="user.email" :google-connected="user.googleConnected" @navigate-settings="activePage = 'settings'" />
        <StockLevelsView v-else-if="activePage === 'stock-levels'" :currency="user.currency" :role="user.role" />
        <ReceiptsView v-else-if="activePage === 'receipts'" :shop-name="user.shopName" :currency="user.currency" />
        <SalesView v-else-if="activePage === 'sales'" :shop-name="user.shopName" :currency="user.currency" />
        <ProfitAnalyticsView v-else-if="activePage === 'profit-analytics'" :shop-name="user.shopName" :currency="user.currency" />
        <BusinessIntelligenceView v-else-if="activePage === 'business-intelligence'" :shop-name="user.shopName" :currency="user.currency" />
        <AuditLogsView v-else-if="activePage === 'audit-logs'" />
        <SettingsView v-else-if="activePage === 'settings'" :shop-name="user.shopName" :currency="user.currency" :role="user.role" :user-id="user.id" :user-name="user.fullName" :user-email="user.email" @user-updated="emit('user-updated', $event)" />
      </main>

      <nav class="mobile-bottom-nav" aria-label="Mobile main navigation">
        <button
          v-for="item in mobilePrimaryNavigation"
          :key="item.id"
          type="button"
          :class="{ active: activePage === item.id }"
          :aria-current="activePage === item.id ? 'page' : undefined"
          @click="navigateToPage(item.id)"
        >
          <component :is="item.icon" :size="20" />
          <span>{{ item.id === 'dashboard' ? 'Dashboard' : item.id === 'products' ? 'Products' : 'POS' }}</span>
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
          <Menu :size="20" />
          <span>More</span>
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
            :class="{ active: activePage === item.id }"
            :aria-current="activePage === item.id ? 'page' : undefined"
            @click="navigateToPage(item.id)"
          >
            <component :is="item.icon" :size="18" />
            <span>{{ item.label }}</span>
          </button>
        </div>
      </nav>
    </div>
  </div>
</template>
