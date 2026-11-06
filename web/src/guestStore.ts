import type { Product, Sale, SaleItem } from './api'

export type GuestStockMovement = {
  id: string
  productId: number
  quantityDelta: number
  reason: string
  saleId?: string
  createdAt: string
}

export type GuestSettings = {
  shopName: string
  currency: string
  address: string
  phone: string
  receiptHeader: string
  receiptFooter: string
  receiptShowPhone: boolean
  preferences: Record<string, unknown>
}

export type GuestBundle = {
  schemaVersion: 1 | 2 | 3 | 4 | 5
  guestDeviceId: string
  idempotencyKey: string
  products: Product[]
  categories: string[]
  sales: Sale[]
  stockMovements: GuestStockMovement[]
  settings: GuestSettings
}

type GuestMember = {
  id: string
  fullName: string
  email: string
  role: string
  active: boolean
  createdAt: string
  permissions: string[]
}

type GuestState = GuestBundle & {
  demoData: boolean
  members: GuestMember[]
  auditLogs: { id: string; user: string; action: string; details: string; level: string; timestamp: string }[]
}

const DEVICE_KEY = 'cm_retailos_guest_device_v1'
const STORE_KEY = 'cm_retailos_guest_data_v1'
const LOCAL_PREFS_KEY = 'cm_retailos_online_settings_prefs_v1'

function readExistingPreferences(): Record<string, unknown> {
  const raw = localStorage.getItem(LOCAL_PREFS_KEY)
  if (!raw) return {}
  const parsed: unknown = JSON.parse(raw)
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('Saved settings could not be read. Correct or clear this browser’s CM RetailOS preferences and reload.')
  }
  return parsed as Record<string, unknown>
}

function uuid(): string {
  if (typeof crypto.randomUUID === 'function') return crypto.randomUUID()
  const bytes = crypto.getRandomValues(new Uint8Array(16))
  bytes[6] = (bytes[6] & 0x0f) | 0x40
  bytes[8] = (bytes[8] & 0x3f) | 0x80
  return [...bytes].map((byte, index) => {
    const value = byte.toString(16).padStart(2, '0')
    return [4, 6, 8, 10].includes(index) ? `-${value}` : value
  }).join('')
}

export function getGuestDeviceId(): string {
  let deviceId = localStorage.getItem(DEVICE_KEY)
  if (!deviceId) {
    deviceId = uuid()
    localStorage.setItem(DEVICE_KEY, deviceId)
  }
  return deviceId
}

function seedState(): GuestState {
  return {
    schemaVersion: 5,
    guestDeviceId: getGuestDeviceId(),
    idempotencyKey: uuid(),
    demoData: false,
    products: [],
    categories: [],
    sales: [],
    stockMovements: [],
    settings: {
      shopName: 'My Shop',
      currency: 'UGX',
      address: '',
      phone: '',
      receiptHeader: '',
      receiptFooter: 'Thank you for shopping with us!',
      receiptShowPhone: false,
      preferences: readExistingPreferences(),
    },
    members: [],
    auditLogs: [],
  }
}

function readState(): GuestState {
  const raw = localStorage.getItem(STORE_KEY)
  if (!raw) {
    const initial = seedState()
    writeState(initial)
    return initial
  }
  const state = JSON.parse(raw) as GuestState
  if (state.guestDeviceId !== getGuestDeviceId()
    || !Array.isArray(state.products)
    || !Array.isArray(state.sales)
    || !Array.isArray(state.categories)) {
    throw new Error('Guest data could not be read. Clear this browser’s CM RetailOS guest data and reload.')
  }
  // Auto-clear legacy seeded demo data to initialize a clean production workspace
  if (state.demoData) {
    const clean = seedState()
    writeState(clean)
    return clean
  }
  if (state.schemaVersion !== 5) {
    state.schemaVersion = 5
    writeState(state)
  }
  return state
}

function writeState(state: GuestState): void {
  try {
    localStorage.setItem(STORE_KEY, JSON.stringify(state))
  } catch {
    throw new Error('Your browser could not save guest data. Free up browser storage and try again.')
  }
}

function record(state: GuestState, action: string, details: string): void {
  state.auditLogs.unshift({
    id: uuid(),
    user: 'Shop Owner',
    action,
    details,
    level: 'AUDIT',
    timestamp: new Date().toISOString(),
  })
  state.auditLogs = state.auditLogs.slice(0, 500)
}

function productsFor(state: GuestState, url: URL): Product[] {
  const query = (url.searchParams.get('q') ?? '').toLocaleLowerCase()
  return state.products
    .filter(product => (product as Product & { active?: boolean }).active !== false)
    .filter(product => !query || [product.name, product.sku, product.barcode, product.category].some(value => value.toLocaleLowerCase().includes(query)))
    .sort((a, b) => a.name.localeCompare(b.name))
}

function dashboardFor(state: GuestState, url: URL) {
  const now = new Date()
  const period = url.searchParams.get('period') ?? 'Daily'
  const reportDate = url.searchParams.get('reportDate')?.trim()
  const topDate = url.searchParams.get('date')?.trim()

  const day = new Date(now)
  day.setHours(0, 0, 0, 0)

  let start: Date
  let end: Date
  let previousStart: Date
  let previousEnd: Date

  if (reportDate) {
    const parts = reportDate.split('-').map(Number)
    start = new Date(parts[0], parts[1] - 1, parts[2], 0, 0, 0, 0)
    end = new Date(start)
    end.setDate(end.getDate() + 1)
    previousStart = new Date(start)
    previousStart.setDate(previousStart.getDate() - 1)
    previousEnd = new Date(start)
  } else if (period === 'Weekly') {
    const daysSinceMonday = (day.getDay() + 6) % 7
    start = new Date(day)
    start.setDate(start.getDate() - daysSinceMonday)
    end = new Date(start)
    end.setDate(end.getDate() + 7)
    previousStart = new Date(start)
    previousStart.setDate(previousStart.getDate() - 7)
    previousEnd = new Date(start)
  } else if (period === 'Monthly') {
    start = new Date(day.getFullYear(), day.getMonth(), 1, 0, 0, 0, 0)
    end = new Date(day.getFullYear(), day.getMonth() + 1, 1, 0, 0, 0, 0)
    previousStart = new Date(day.getFullYear(), day.getMonth() - 1, 1, 0, 0, 0, 0)
    previousEnd = new Date(start)
  } else {
    start = new Date(day)
    end = new Date(start)
    end.setDate(end.getDate() + 1)
    previousStart = new Date(start)
    previousStart.setDate(previousStart.getDate() - 1)
    previousEnd = new Date(start)
  }

  const selected = state.sales.filter(sale => {
    const completed = new Date(sale.completedAt)
    return completed >= start && completed < end
  })
  const previousSales = state.sales.filter(sale => {
    const completed = new Date(sale.completedAt)
    return completed >= previousStart && completed < previousEnd
  })
  const revenue = (sales: Sale[]) => sales.reduce((sum, sale) => sum + sale.total, 0)
  const profit = (sales: Sale[]) => sales.reduce((sum, sale) => sum + sale.items.reduce((lineSum, item) => lineSum + item.lineTotal - item.quantity * item.unitCost, 0) - sale.discount, 0)

  const chartPeriod = reportDate ? 'Daily' : period
  const chart: { label: string; sales: number; profit: number }[] = (() => {
    if (chartPeriod === 'Weekly') {
      const labels = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']
      return labels.map(label => ({ label, sales: 0, profit: 0 }))
    }
    if (chartPeriod === 'Monthly') {
      return Array.from({ length: 4 }, (_, i) => ({ label: `Week ${i + 1}`, sales: 0, profit: 0 }))
    }
    // Daily: 24 points starting at 6am (6am to 5am), matching mother app and retail.go
    return Array.from({ length: 24 }, (_, i) => {
      const hour = (i + 6) % 24
      const displayHour = hour % 12 === 0 ? 12 : hour % 12
      const meridiem = hour >= 12 ? 'pm' : 'am'
      return { label: `${displayHour}${meridiem}`, sales: 0, profit: 0 }
    })
  })()

  for (const sale of selected) {
    const completed = new Date(sale.completedAt)
    let index: number
    if (chartPeriod === 'Weekly') {
      index = (completed.getDay() + 6) % 7
    } else if (chartPeriod === 'Monthly') {
      index = Math.floor((completed.getDate() - 1) / 7)
      if (index > 3) index = 3
    } else {
      index = (completed.getHours() - 6 + 24) % 24
    }

    const saleProfit = sale.items.reduce((sum, item) => sum + item.lineTotal - item.quantity * item.unitCost, 0) - sale.discount
    if (index >= 0 && index < chart.length) {
      chart[index].sales += sale.total
      chart[index].profit += saleProfit
    }
  }

  const topTargetSales = (topDate && topDate !== reportDate)
    ? state.sales.filter(sale => {
        const completed = new Date(sale.completedAt)
        const parts = topDate.split('-').map(Number)
        const tStart = new Date(parts[0], parts[1] - 1, parts[2], 0, 0, 0, 0)
        const tEnd = new Date(tStart)
        tEnd.setDate(tEnd.getDate() + 1)
        return completed >= tStart && completed < tEnd
      })
    : selected

  const productTotals = new Map<number, { name: string; quantitySold: number; revenue: number }>()
  const categoryTotals = new Map<string, number>()
  for (const sale of topTargetSales) {
    for (const item of sale.items) {
      const current = productTotals.get(item.productId) ?? { name: item.productName, quantitySold: 0, revenue: 0 }
      current.quantitySold += item.quantity
      current.revenue += item.lineTotal
      productTotals.set(item.productId, current)

      const prod = productById(state, item.productId)
      const cat = prod?.category?.trim() || 'General'
      const itemProfit = item.lineTotal - item.quantity * item.unitCost - (sale.subtotal > 0 ? (sale.discount * item.lineTotal) / sale.subtotal : 0)
      categoryTotals.set(cat, (categoryTotals.get(cat) ?? 0) + itemProfit)
    }
  }
  const todayStart = new Date(now)
  todayStart.setHours(0, 0, 0, 0)
  const todaySales = state.sales.filter(sale => new Date(sale.completedAt) >= todayStart)
  const activeProducts = productsFor(state, url)
  return {
    productCount: activeProducts.length,
    lowStockCount: activeProducts.filter(product => product.quantity <= product.minStock).length,
    inventoryValue: activeProducts.reduce((sum, product) => sum + product.costPrice * product.quantity, 0),
    todaySales: todaySales.length,
    todayRevenue: revenue(todaySales),
    currency: state.settings.currency,
    revenue: revenue(selected),
    profit: profit(selected),
    previousRevenue: revenue(previousSales),
    previousProfit: profit(previousSales),
    orderCount: selected.length,
    previousOrders: previousSales.length,
    unitsSold: selected.reduce((sum, sale) => sum + sale.items.reduce((itemSum, item) => itemSum + item.quantity, 0), 0),
    chart,
    topProducts: [...productTotals.values()].sort((a, b) => b.revenue - a.revenue).slice(0, 5),
    profitByCategory: [...categoryTotals.entries()]
      .map(([name, value]) => ({ name, value: Math.max(0, Math.round(value)) }))
      .sort((a, b) => b.value - a.value)
      .slice(0, 6),
  }
}

function productById(state: GuestState, id: number): Product | undefined {
  return state.products.find(product => product.id === id && (product as Product & { active?: boolean }).active !== false)
}

function updateProductStock(state: GuestState, id: number, delta: number, reason: string, saleId?: string): Product {
  const product = productById(state, id)
  if (!product) throw new Error('That product is no longer available.')
  if (!Number.isFinite(delta) || delta === 0 || product.quantity + delta < 0) throw new Error('Stock cannot be adjusted below zero.')
  product.quantity = Math.round((product.quantity + delta) * 1000) / 1000
  state.stockMovements.push({
    id: uuid(),
    productId: id,
    quantityDelta: delta,
    reason,
    saleId,
    createdAt: new Date().toISOString(),
  })
  return product
}

function businessIntelligenceFor(state: GuestState, url: URL) {
  const period = url.searchParams.get('period') ?? 'Daily'
  const peakSalesHours = Array.from({ length: 12 }, () => 0)
  const profits = new Map<string, number>()
  const pairs = new Map<string, number>()
  for (const sale of state.sales) {
    const hour = new Date(sale.completedAt).getHours()
    const hourWindow = Math.floor(((hour - 6 + 24) % 24) / 2)
    peakSalesHours[hourWindow]++
    for (const item of sale.items) {
      profits.set(item.productName, (profits.get(item.productName) ?? 0) + item.lineTotal - item.quantity * item.unitCost)
    }
    const names = sale.items.map(item => item.productName).sort()
    for (let first = 0; first < names.length; first++) {
      for (let second = first + 1; second < names.length; second++) {
        const key = `${names[first]}\u0000${names[second]}`
        pairs.set(key, (pairs.get(key) ?? 0) + 1)
      }
    }
  }
  return {
    period,
    peakSalesHours,
    mostProfitableProducts: [...profits].map(([name, profit]) => ({ name, profit })).sort((a, b) => b.profit - a.profit).slice(0, 10),
    marketBasket: [...pairs].map(([key, frequency]) => ({ pair: key.split('\u0000') as [string, string], frequency })).sort((a, b) => b.frequency - a.frequency).slice(0, 10),
  }
}

export async function guestApiRequest<T>(path: string, options: { method?: string; body?: unknown } = {}): Promise<T> {
  const url = new URL(path, window.location.origin)
  const method = (options.method ?? (options.body === undefined ? 'GET' : 'POST')).toUpperCase()
  const body = (options.body ?? {}) as Record<string, unknown>
  const state = readState()
  const fail = (message: string): never => { throw new Error(message) }

  if (url.pathname.endsWith('/public/workspace')) {
    return {
      shopName: state.demoData ? 'Demo Shop' : (state.settings.shopName || 'My Shop'),
      currency: state.settings.currency || 'UGX',
      ownerName: (state.settings.preferences?.ownerName as string) || 'Shop Owner',
      demo: state.demoData,
    } as T
  }
  if (url.pathname.endsWith('/public/dashboard') || url.pathname.endsWith('/dashboard')) return dashboardFor(state, url) as T
  if (url.pathname.endsWith('/public/products') || url.pathname.endsWith('/products') && method === 'GET') {
    return { products: productsFor(state, url) } as T
  }
  if (url.pathname.endsWith('/categories') && method === 'GET') {
    const names = new Set(state.categories)
    for (const product of productsFor(state, url)) if (product.category) names.add(product.category)
    return { categories: [...names].sort((a, b) => a.localeCompare(b)) } as T
  }
  if (url.pathname.endsWith('/categories') && method === 'POST') {
    const name = String(body.name ?? '').trim()
    if (!name || name.length > 80) fail('Category names must be between 1 and 80 characters.')
    if (state.categories.some(category => category.toLocaleLowerCase() === name.toLocaleLowerCase())) fail('That category already exists in this shop.')
    state.categories.push(name)
    record(state, 'Category added', name)
    writeState(state)
    return { name } as T
  }
  if (url.pathname.endsWith('/categories') && method === 'PUT') {
    const oldName = String(body.oldName ?? '').trim()
    const name = String(body.name ?? '').trim()
    if (!oldName || !name || name.length > 80) fail('Category names must be between 1 and 80 characters.')
    state.categories = state.categories.filter(category => category.toLocaleLowerCase() !== oldName.toLocaleLowerCase())
    if (!state.categories.some(category => category.toLocaleLowerCase() === name.toLocaleLowerCase())) state.categories.push(name)
    for (const product of state.products) if (product.category.toLocaleLowerCase() === oldName.toLocaleLowerCase()) product.category = name
    record(state, 'Category renamed', `${oldName} → ${name}`)
    writeState(state)
    return { name } as T
  }
  if (url.pathname.endsWith('/categories') && method === 'DELETE') {
    const name = String(body.name ?? '').trim()
    state.categories = state.categories.filter(category => category.toLocaleLowerCase() !== name.toLocaleLowerCase())
    for (const product of state.products) if (product.category.toLocaleLowerCase() === name.toLocaleLowerCase()) product.category = ''
    record(state, 'Category deleted', name)
    writeState(state)
    return undefined as T
  }
  if (url.pathname === '/products' && method === 'POST') {
    const input = body as unknown as Omit<Product, 'id'>
    if (!String(input.name ?? '').trim() || !Number.isFinite(Number(input.sellingPrice)) || Number(input.sellingPrice) <= 0) fail('Enter a product name and a valid selling price.')
    const product: Product = {
      id: Math.max(0, ...state.products.map(item => item.id)) + 1,
      sku: String(input.sku ?? ''),
      name: String(input.name).trim(),
      category: String(input.category ?? ''),
      barcode: String(input.barcode ?? ''),
      unit: String(input.unit || 'piece'),
      sellingPrice: Number(input.sellingPrice),
      costPrice: Number(input.costPrice ?? 0),
      quantity: Number(input.quantity ?? 0),
      minStock: Number(input.minStock ?? 0),
      image: String(input.image ?? ''),
    }
    state.products.push(product)
    state.categories = [...new Set([...state.categories, product.category].filter(Boolean))]
    if (product.quantity > 0) state.stockMovements.push({ id: uuid(), productId: product.id, quantityDelta: product.quantity, reason: 'Opening stock', createdAt: new Date().toISOString() })
    record(state, 'Product added', product.name)
    writeState(state)
    return { product } as T
  }
  const productMatch = url.pathname.match(/^\/products\/(\d+)(?:\/(stock-adjustments|archive))?$/)
  if (productMatch) {
    const id = Number(productMatch[1])
    const action = productMatch[2]
    if (action === 'archive' && method === 'POST') {
      const product = productById(state, id)
      if (!product) throw new Error('That product is no longer available.')
      product.active = false
      record(state, 'Product archived', product.name)
      writeState(state)
      return undefined as T
    }
    if (action === 'stock-adjustments' && method === 'POST') {
      const product = updateProductStock(state, id, Number(body.delta), String(body.reason ?? '').trim())
      record(state, 'Stock adjusted', product.name)
      writeState(state)
      return { product } as T
    }
    if (!action && method === 'PUT') {
      const current = productById(state, id)
      if (!current) throw new Error('That product is no longer available.')
      const currentQuantity = current.quantity
      Object.assign(current, body, { id, quantity: currentQuantity })
      const updated = current as Product
      state.categories = [...new Set([...state.categories, updated.category].filter(Boolean))]
      record(state, 'Product updated', updated.name)
      writeState(state)
      return { product: updated } as T
    }
  }
  if (url.pathname.endsWith('/sales') && method === 'GET') {
    return { sales: [...state.sales].sort((a, b) => b.completedAt.localeCompare(a.completedAt)) } as T
  }
  if (url.pathname.endsWith('/sales/checkout') && method === 'POST') {
    const requestItems = Array.isArray(body.items) ? body.items as { productId: number; quantity: number }[] : []
    if (!requestItems.length) fail('A sale must contain at least one product.')
    const items: SaleItem[] = requestItems.map(line => {
      const product = productById(state, Number(line.productId))
      const quantity = Number(line.quantity)
      if (!product) throw new Error('A product is unavailable or has insufficient stock.')
      if (!Number.isFinite(quantity) || quantity <= 0 || product.quantity < quantity) {
        throw new Error('A product is unavailable or has insufficient stock.')
      }
      return {
        productId: product.id,
        productName: product.name,
        quantity,
        unitPrice: product.sellingPrice,
        unitCost: product.costPrice,
        lineTotal: Math.round(product.sellingPrice * quantity * 100) / 100,
      }
    })
    const subtotal = items.reduce((sum, item) => sum + item.lineTotal, 0)
    const discount = Number(body.discount ?? 0)
    if (!Number.isFinite(discount) || discount < 0 || discount > subtotal) fail('Discount must be between zero and the sale subtotal.')
    const sale: Sale = {
      id: uuid(),
      cashierName: 'Shop Owner',
      customerName: String(body.customerName ?? ''),
      subtotal,
      discount,
      total: Math.round((subtotal - discount) * 100) / 100,
      paymentMethod: (['cash', 'mobile_money', 'card'].includes(String(body.paymentMethod)) ? body.paymentMethod : 'cash') as Sale['paymentMethod'],
      completedAt: new Date().toISOString(),
      items,
    }
    for (const item of items) updateProductStock(state, item.productId, -item.quantity, `Sale ${sale.id}`, sale.id)
    state.sales.push(sale)
    record(state, 'Sale completed', sale.id)
    writeState(state)
    return { sale } as T
  }
  if (url.pathname.endsWith('/settings') && method === 'GET') {
    const { preferences: _preferences, ...settings } = state.settings
    return { settings } as T
  }
  if (url.pathname.endsWith('/settings') && method === 'PUT') {
    state.settings = { ...state.settings, ...body } as GuestSettings
    record(state, 'Shop settings updated', state.settings.shopName)
    writeState(state)
    return { settings: Object.fromEntries(Object.entries(state.settings).filter(([key]) => key !== 'preferences')) } as T
  }
  if (url.pathname.endsWith('/settings/preferences') && method === 'GET') {
    const raw = state.settings.preferences
    return { preferences: raw, permissions: [], staffPermissions: {} } as T
  }
  if (url.pathname.endsWith('/settings/preferences') && method === 'PUT') {
    state.settings.preferences = {
      ...(state.settings.preferences || {}),
      ...((body.preferences as Record<string, unknown>) ?? {}),
    }
    record(state, 'Preferences updated', 'Local guest preferences')
    writeState(state)
    return { preferences: state.settings.preferences } as T
  }
  if (url.pathname.endsWith('/auth/profile') && method === 'PUT') {
    const fullName = String(body.fullName ?? 'Shop Owner').trim() || 'Shop Owner'
    if (!state.settings.preferences) state.settings.preferences = {}
    state.settings.preferences.ownerName = fullName
    state.settings.preferences.owner = fullName
    record(state, 'Profile updated', fullName)
    writeState(state)
    return { fullName } as T
  }
  if (url.pathname.endsWith('/auth/profile') && method === 'GET') {
    const fullName = String(state.settings.preferences?.ownerName || 'Shop Owner')
    return { fullName } as T
  }
  if (url.pathname.endsWith('/business-intelligence')) return businessIntelligenceFor(state, url) as T
  if (url.pathname.endsWith('/audit-logs')) return { logs: state.auditLogs } as T
  if (url.pathname.endsWith('/team-members') && method === 'GET') return { members: state.members } as T
  if (url.pathname.endsWith('/team-members') && method === 'POST') {
    const member: GuestMember = {
      id: uuid(),
      fullName: String(body.fullName ?? 'Team Member'),
      email: String(body.email ?? ''),
      role: String(body.role ?? 'cashier'),
      active: true,
      createdAt: new Date().toISOString(),
      permissions: Array.isArray(body.permissions) ? body.permissions as string[] : [],
    }
    state.members.push(member)
    record(state, 'Team member added', member.fullName)
    writeState(state)
    return { member } as T
  }
  const memberMatch = url.pathname.match(/^\/team-members\/([^/]+)\/(permissions|status)$/)
  if (memberMatch) {
    const member = state.members.find(item => item.id === decodeURIComponent(memberMatch[1]))
    if (!member) throw new Error('That team member could not be found.')
    if (memberMatch[2] === 'permissions' && method === 'PUT') member.permissions = Array.isArray(body.permissions) ? body.permissions as string[] : []
    if (memberMatch[2] === 'status' && method === 'PATCH') member.active = Boolean(body.active)
    writeState(state)
    return undefined as T
  }
  throw new Error(`Guest mode does not support ${method} ${url.pathname}.`)
}

export function exportGuestBundle(): GuestBundle {
  const { members: _members, auditLogs: _auditLogs, demoData: _demoData, ...bundle } = readState()
  return {
    ...bundle,
    schemaVersion: 1,
  }
}

export function clearGuestDataAfterClaim(): void {
  const deviceId = getGuestDeviceId()
  writeState({
    schemaVersion: 1,
    guestDeviceId: deviceId,
    idempotencyKey: uuid(),
    demoData: false,
    products: [],
    categories: [],
    sales: [],
    stockMovements: [],
    settings: {
      shopName: 'My Shop',
      currency: 'UGX',
      address: '',
      phone: '',
      receiptHeader: '',
      receiptFooter: 'Thank you for shopping with us!',
      receiptShowPhone: false,
      preferences: {},
    },
    members: [],
    auditLogs: [],
  })
}

export function resetGuestData(): void {
  localStorage.setItem(STORE_KEY, JSON.stringify(seedState()))
}
