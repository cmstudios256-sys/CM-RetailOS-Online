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
  schemaVersion: 1 | 2
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
  const now = new Date()
  const productSeeds: Product[] = [
    { id: 1, sku: 'DEMO-001', name: 'Demo Coffee Beans', category: 'Demo Groceries', barcode: '', unit: 'bag', sellingPrice: 12000, costPrice: 8000, quantity: 18, minStock: 5, image: '' },
    { id: 2, sku: 'DEMO-002', name: 'Sample Notebook', category: 'Demo Stationery', barcode: '', unit: 'piece', sellingPrice: 4500, costPrice: 2500, quantity: 32, minStock: 8, image: '' },
    { id: 3, sku: 'DEMO-003', name: 'Example Snack Pack', category: 'Demo Groceries', barcode: '', unit: 'pack', sellingPrice: 3000, costPrice: 1800, quantity: 4, minStock: 6, image: '' },
  ]
  const coffee = productSeeds[0]
  const notebook = productSeeds[1]
  const demoSaleAt = (id: string, completedAt: Date, product: Product, quantity: number): Sale => {
    const lineTotal = product.sellingPrice * quantity
    return {
      id,
      cashierName: 'Shop Owner',
      customerName: 'Demo Customer',
      subtotal: lineTotal,
      discount: 0,
      total: lineTotal,
      paymentMethod: 'cash',
      completedAt: completedAt.toISOString(),
      items: [{
        productId: product.id,
        productName: product.name,
        quantity,
        unitPrice: product.sellingPrice,
        unitCost: product.costPrice,
        lineTotal,
      }],
    }
  }
  const weekStart = new Date(now)
  weekStart.setHours(0, 0, 0, 0)
  weekStart.setDate(weekStart.getDate() - ((weekStart.getDay() + 6) % 7))
  const atHourOnDay = (dayOffset: number, hour: number) => {
    const date = new Date(weekStart)
    date.setDate(date.getDate() + dayOffset)
    date.setHours(hour, 0, 0, 0)
    return date
  }
  const todayHour = now.getHours() > 1 ? now.getHours() : 1
  const sales = [
    demoSaleAt('demo-sale-001', new Date(now.getFullYear(), now.getMonth(), now.getDate(), 1, 0, 0), coffee, 1),
    demoSaleAt('demo-sale-002', new Date(now.getFullYear(), now.getMonth(), now.getDate(), todayHour, 0, 0), notebook, 2),
    demoSaleAt('demo-sale-003', atHourOnDay(0, 9), coffee, 1),
    demoSaleAt('demo-sale-004', atHourOnDay(1, 13), notebook, 1),
    demoSaleAt('demo-sale-005', atHourOnDay(2, 17), coffee, 2),
    demoSaleAt('demo-sale-006', atHourOnDay(3, 6), productSeeds[2], 2),
    demoSaleAt('demo-sale-007', atHourOnDay(4, 11), notebook, 3),
    demoSaleAt('demo-sale-008', atHourOnDay(5, 15), coffee, 1),
    demoSaleAt('demo-sale-009', atHourOnDay(6, 19), productSeeds[2], 1),
  ]
  const movements: GuestStockMovement[] = productSeeds.map(product => ({
    id: `demo-opening-${product.id}`,
    productId: product.id,
    quantityDelta: product.quantity,
    reason: 'Demo opening stock',
    createdAt: now.toISOString(),
  }))
  return {
    schemaVersion: 2,
    guestDeviceId: getGuestDeviceId(),
    idempotencyKey: uuid(),
    demoData: true,
    products: productSeeds,
    categories: ['Demo Groceries', 'Demo Stationery'],
    sales,
    stockMovements: movements,
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
  if (state.demoData && state.schemaVersion !== 2) {
    const refreshed = seedState()
    writeState(refreshed)
    return refreshed
  }
  if (state.schemaVersion !== 2) {
    state.schemaVersion = 2
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
  const periodStart = (date: Date, amount: number) => {
    const start = new Date(date)
    if (period === 'Weekly') start.setDate(start.getDate() - 7 * amount)
    else if (period === 'Monthly') start.setDate(start.getDate() - 30 * amount)
    else start.setDate(start.getDate() - amount)
    start.setHours(0, 0, 0, 0)
    return start
  }
  const start = periodStart(now, 0)
  const end = new Date(now)
  if (period === 'Weekly') start.setDate(start.getDate() - 6)
  if (period === 'Monthly') start.setDate(start.getDate() - 29)
  const selected = state.sales.filter(sale => {
    const completed = new Date(sale.completedAt)
    return completed >= start && completed <= end
  })
  const previousStart = periodStart(start, 1)
  const previousSales = state.sales.filter(sale => {
    const completed = new Date(sale.completedAt)
    return completed >= previousStart && completed < start
  })
  const revenue = (sales: Sale[]) => sales.reduce((sum, sale) => sum + sale.total, 0)
  const profit = (sales: Sale[]) => sales.reduce((sum, sale) => sum + sale.items.reduce((lineSum, item) => lineSum + item.lineTotal - item.quantity * item.unitCost, 0) - sale.discount, 0)
  const chartCount = period === 'Daily' ? 24 : period === 'Weekly' ? 7 : 30
  const chart = Array.from({ length: chartCount }, (_, index) => {
    const label = period === 'Daily'
      ? `${String(index).padStart(2, '0')}:00`
      : period === 'Weekly'
        ? new Intl.DateTimeFormat(undefined, { weekday: 'short' }).format(new Date(start.getTime() + index * 86400000))
        : String(index + 1)
    return { label, sales: 0, profit: 0 }
  })
  const productTotals = new Map<number, { name: string; quantitySold: number; revenue: number }>()
  for (const sale of selected) {
    const completed = new Date(sale.completedAt)
    const index = period === 'Daily'
      ? completed.getHours()
      : Math.max(0, Math.min(chartCount - 1, Math.floor((completed.getTime() - start.getTime()) / 86400000)))
    const saleProfit = sale.items.reduce((sum, item) => sum + item.lineTotal - item.quantity * item.unitCost, 0) - sale.discount
    chart[index].sales += sale.total
    chart[index].profit += saleProfit
    for (const item of sale.items) {
      const current = productTotals.get(item.productId) ?? { name: item.productName, quantitySold: 0, revenue: 0 }
      current.quantitySold += item.quantity
      current.revenue += item.lineTotal
      productTotals.set(item.productId, current)
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
    profitByCategory: [],
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
    const hourWindow = Math.floor(new Date(sale.completedAt).getHours() / 2) % 12
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
      shopName: state.demoData ? 'Demo Shop' : state.settings.shopName,
      currency: state.settings.currency,
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
    state.settings.preferences = (body.preferences as Record<string, unknown>) ?? {}
    record(state, 'Preferences updated', 'Local guest preferences')
    writeState(state)
    return { preferences: state.settings.preferences } as T
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
  return bundle
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
