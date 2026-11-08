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
  schemaVersion: 1 | 2 | 3 | 4 | 5 | 6
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

function generateEan13(index: number): string {
  const base = `640000${String(index).padStart(6, '0')}`
  let sum = 0
  for (let i = 0; i < 12; i++) {
    sum += Number(base[i]) * (i % 2 === 0 ? 1 : 3)
  }
  const checksum = (10 - (sum % 10)) % 10
  return `${base}${checksum}`
}

export function createDemoProducts(): Product[] {
  const items: Array<{
    name: string
    category: string
    unit: string
    sellingPrice: number
    costPrice: number
    quantity: number
    minStock: number
    sku: string
    image?: string
  }> = [
    // 1-3: Footwear
    {
      name: 'Classic Red Sneakers',
      category: 'Footwear',
      unit: 'pair',
      sellingPrice: 125000,
      costPrice: 85000,
      quantity: 18,
      minStock: 5,
      sku: 'FTW-SNK-001',
      image: 'https://images.unsplash.com/photo-1542291026-7eec264c27ff?w=400&auto=format&fit=crop&q=80',
    },
    {
      name: 'White Canvas Low-Tops',
      category: 'Footwear',
      unit: 'pair',
      sellingPrice: 85000,
      costPrice: 55000,
      quantity: 12,
      minStock: 4,
      sku: 'FTW-CNV-002',
      image: 'https://images.unsplash.com/photo-1525966222134-fcfa99b8ae77?w=400&auto=format&fit=crop&q=80',
    },
    {
      name: 'Leather Oxford Dress Shoes',
      category: 'Footwear',
      unit: 'pair',
      sellingPrice: 180000,
      costPrice: 120000,
      quantity: 8,
      minStock: 3,
      sku: 'FTW-OXF-003',
      image: 'https://images.unsplash.com/photo-1614252235316-8c857d38b5f4?w=400&auto=format&fit=crop&q=80',
    },

    // 4-10: Electronics
    {
      name: 'Studio Wireless Headphones',
      category: 'Electronics',
      unit: 'pcs',
      sellingPrice: 185000,
      costPrice: 120000,
      quantity: 7,
      minStock: 3,
      sku: 'ELC-HDP-004',
      image: 'https://images.unsplash.com/photo-1505740420928-5e560c06d30e?w=400&auto=format&fit=crop&q=80',
    },
    {
      name: 'Instant Film Camera',
      category: 'Electronics',
      unit: 'pcs',
      sellingPrice: 320000,
      costPrice: 230000,
      quantity: 5,
      minStock: 2,
      sku: 'ELC-CAM-005',
      image: 'https://images.unsplash.com/photo-1526170375885-4d8ecf77b99f?w=400&auto=format&fit=crop&q=80',
    },
    {
      name: 'Bluetooth Speaker Mini',
      category: 'Electronics',
      unit: 'pcs',
      sellingPrice: 75000,
      costPrice: 48000,
      quantity: 14,
      minStock: 4,
      sku: 'ELC-SPK-006',
      image: 'https://images.unsplash.com/photo-1608043152269-423dbba4e7e1?w=400&auto=format&fit=crop&q=80',
    },
    {
      name: 'True Wireless Earbuds',
      category: 'Electronics',
      unit: 'pcs',
      sellingPrice: 95000,
      costPrice: 60000,
      quantity: 16,
      minStock: 4,
      sku: 'ELC-EAR-007',
      image: 'https://images.unsplash.com/photo-1590658268037-6bf12165a8df?w=400&auto=format&fit=crop&q=80',
    },
    {
      name: 'USB-C Fast Charging Cable',
      category: 'Electronics',
      unit: 'pcs',
      sellingPrice: 15000,
      costPrice: 7500,
      quantity: 45,
      minStock: 10,
      sku: 'ELC-CBL-008',
    },
    {
      name: '20W USB-C Power Adapter',
      category: 'Electronics',
      unit: 'pcs',
      sellingPrice: 35000,
      costPrice: 20000,
      quantity: 22,
      minStock: 5,
      sku: 'ELC-ADP-009',
    },
    {
      name: '20000mAh Power Bank',
      category: 'Electronics',
      unit: 'pcs',
      sellingPrice: 85000,
      costPrice: 55000,
      quantity: 11,
      minStock: 3,
      sku: 'ELC-PWR-010',
    },

    // 11-15: Accessories
    {
      name: 'Everyday Water Bottle',
      category: 'Accessories',
      unit: 'pcs',
      sellingPrice: 28000,
      costPrice: 16000,
      quantity: 24,
      minStock: 8,
      sku: 'ACC-BTL-011',
      image: 'https://images.unsplash.com/photo-1602143407151-7111542de6e8?w=400&auto=format&fit=crop&q=80',
    },
    {
      name: 'Minimalist Wristwatch',
      category: 'Accessories',
      unit: 'pcs',
      sellingPrice: 95000,
      costPrice: 62000,
      quantity: 2,
      minStock: 5,
      sku: 'ACC-WCH-012',
      image: 'https://images.unsplash.com/photo-1523275335684-37898b6baf30?w=400&auto=format&fit=crop&q=80',
    },
    {
      name: 'Polarized Sunglasses',
      category: 'Accessories',
      unit: 'pcs',
      sellingPrice: 45000,
      costPrice: 26000,
      quantity: 15,
      minStock: 5,
      sku: 'ACC-SUN-013',
      image: 'https://images.unsplash.com/photo-1572635196237-14b3f281503f?w=400&auto=format&fit=crop&q=80',
    },
    {
      name: 'Classic Canvas Backpack',
      category: 'Accessories',
      unit: 'pcs',
      sellingPrice: 75000,
      costPrice: 48000,
      quantity: 10,
      minStock: 3,
      sku: 'ACC-BPK-014',
      image: 'https://images.unsplash.com/photo-1553062407-98eeb64c6a62?w=400&auto=format&fit=crop&q=80',
    },
    {
      name: 'Genuine Leather Bi-fold Wallet',
      category: 'Accessories',
      unit: 'pcs',
      sellingPrice: 55000,
      costPrice: 32000,
      quantity: 14,
      minStock: 4,
      sku: 'ACC-WLT-015',
    },

    // 16-30: Groceries
    {
      name: 'Medium Roast Coffee Beans 500g',
      category: 'Groceries',
      unit: 'pack',
      sellingPrice: 42000,
      costPrice: 26000,
      quantity: 9,
      minStock: 3,
      sku: 'GRO-COF-016',
      image: 'https://images.unsplash.com/photo-1559056199-641a0ac8b55e?w=400&auto=format&fit=crop&q=80',
    },
    {
      name: 'Pure Granulated Sugar 1kg',
      category: 'Groceries',
      unit: 'pack',
      sellingPrice: 4500,
      costPrice: 3200,
      quantity: 65,
      minStock: 15,
      sku: 'GRO-SUG-017',
    },
    {
      name: 'Premium Long Grain Rice 2kg',
      category: 'Groceries',
      unit: 'pack',
      sellingPrice: 8500,
      costPrice: 6000,
      quantity: 42,
      minStock: 10,
      sku: 'GRO-RIC-018',
    },
    {
      name: 'Pure Vegetable Cooking Oil 1L',
      category: 'Groceries',
      unit: 'bottle',
      sellingPrice: 11500,
      costPrice: 7800,
      quantity: 30,
      minStock: 8,
      sku: 'GRO-OIL-019',
    },
    {
      name: 'Fortified Maize Flour 2kg',
      category: 'Groceries',
      unit: 'pack',
      sellingPrice: 4800,
      costPrice: 3400,
      quantity: 50,
      minStock: 12,
      sku: 'GRO-MAZ-020',
    },
    {
      name: 'Sliced Whole Wheat Bread 800g',
      category: 'Groceries',
      unit: 'loaf',
      sellingPrice: 4200,
      costPrice: 2700,
      quantity: 25,
      minStock: 6,
      sku: 'GRO-BRD-021',
    },
    {
      name: 'Fresh Farm Egg Tray (30 pcs)',
      category: 'Groceries',
      unit: 'tray',
      sellingPrice: 13500,
      costPrice: 9500,
      quantity: 18,
      minStock: 5,
      sku: 'GRO-EGG-022',
    },
    {
      name: 'Full Cream Milk Powder 400g',
      category: 'Groceries',
      unit: 'tin',
      sellingPrice: 14500,
      costPrice: 10500,
      quantity: 20,
      minStock: 5,
      sku: 'GRO-MLK-023',
    },
    {
      name: 'Iodized Table Salt 1kg',
      category: 'Groceries',
      unit: 'pack',
      sellingPrice: 2800,
      costPrice: 2000,
      quantity: 80,
      minStock: 20,
      sku: 'GRO-SLT-024',
    },
    {
      name: 'Organic Black Tea Leaves 250g',
      category: 'Groceries',
      unit: 'pack',
      sellingPrice: 4500,
      costPrice: 2900,
      quantity: 34,
      minStock: 8,
      sku: 'GRO-TEA-025',
    },
    {
      name: 'Tomato Puree & Paste 400g',
      category: 'Groceries',
      unit: 'can',
      sellingPrice: 3500,
      costPrice: 2100,
      quantity: 38,
      minStock: 10,
      sku: 'GRO-TOM-026',
    },
    {
      name: 'Dry Red Haricot Beans 1kg',
      category: 'Groceries',
      unit: 'pack',
      sellingPrice: 5200,
      costPrice: 3400,
      quantity: 40,
      minStock: 10,
      sku: 'GRO-BEN-027',
    },
    {
      name: 'Roasted Groundnuts 500g',
      category: 'Groceries',
      unit: 'pack',
      sellingPrice: 7500,
      costPrice: 4500,
      quantity: 28,
      minStock: 6,
      sku: 'GRO-GNT-028',
    },
    {
      name: 'Pure Blossom Honey 500g',
      category: 'Groceries',
      unit: 'jar',
      sellingPrice: 16000,
      costPrice: 10000,
      quantity: 15,
      minStock: 4,
      sku: 'GRO-HNY-029',
    },
    {
      name: 'Rolled Oats Cereal 1kg',
      category: 'Groceries',
      unit: 'pack',
      sellingPrice: 14000,
      costPrice: 9000,
      quantity: 19,
      minStock: 5,
      sku: 'GRO-OAT-030',
    },

    // 31-38: Beverages
    {
      name: 'Sparkling Mineral Water 500ml',
      category: 'Beverages',
      unit: 'bottle',
      sellingPrice: 1500,
      costPrice: 900,
      quantity: 96,
      minStock: 24,
      sku: 'BEV-WTR-031',
    },
    {
      name: 'Cola Carbonated Soft Drink 500ml',
      category: 'Beverages',
      unit: 'bottle',
      sellingPrice: 1500,
      costPrice: 950,
      quantity: 72,
      minStock: 20,
      sku: 'BEV-COL-032',
    },
    {
      name: 'Orange Citrus Soda 500ml',
      category: 'Beverages',
      unit: 'bottle',
      sellingPrice: 1500,
      costPrice: 950,
      quantity: 60,
      minStock: 20,
      sku: 'BEV-ORN-033',
    },
    {
      name: 'Fresh Passion Fruit Juice 1L',
      category: 'Beverages',
      unit: 'carton',
      sellingPrice: 6500,
      costPrice: 4000,
      quantity: 22,
      minStock: 6,
      sku: 'BEV-PSN-034',
    },
    {
      name: 'Mango Nectar Juice 500ml',
      category: 'Beverages',
      unit: 'bottle',
      sellingPrice: 3500,
      costPrice: 2100,
      quantity: 35,
      minStock: 10,
      sku: 'BEV-MNG-035',
    },
    {
      name: 'Premium Energy Drink 400ml',
      category: 'Beverages',
      unit: 'can',
      sellingPrice: 4000,
      costPrice: 2600,
      quantity: 45,
      minStock: 12,
      sku: 'BEV-ENG-036',
    },
    {
      name: 'Fresh Pasteurized Milk 500ml',
      category: 'Beverages',
      unit: 'pack',
      sellingPrice: 2200,
      costPrice: 1400,
      quantity: 30,
      minStock: 8,
      sku: 'BEV-MLK-037',
    },
    {
      name: 'Strawberry Drinking Yoghurt 500ml',
      category: 'Beverages',
      unit: 'bottle',
      sellingPrice: 3800,
      costPrice: 2400,
      quantity: 24,
      minStock: 6,
      sku: 'BEV-YGH-038',
    },

    // 39-43: Snacks
    {
      name: 'Salted Potato Crisps 120g',
      category: 'Snacks',
      unit: 'pack',
      sellingPrice: 3500,
      costPrice: 1800,
      quantity: 40,
      minStock: 10,
      sku: 'SNK-POT-039',
    },
    {
      name: 'Cheddar Cheese Crackers 150g',
      category: 'Snacks',
      unit: 'pack',
      sellingPrice: 4500,
      costPrice: 2300,
      quantity: 32,
      minStock: 8,
      sku: 'SNK-CHK-040',
    },
    {
      name: 'Dark Chocolate Bar 100g',
      category: 'Snacks',
      unit: 'piece',
      sellingPrice: 5500,
      costPrice: 3000,
      quantity: 28,
      minStock: 6,
      sku: 'SNK-CHB-041',
    },
    {
      name: 'Butter Shortbread Cookies 200g',
      category: 'Snacks',
      unit: 'pack',
      sellingPrice: 6000,
      costPrice: 3200,
      quantity: 25,
      minStock: 6,
      sku: 'SNK-CKI-042',
    },
    {
      name: 'Sweet Caramel Popcorn 100g',
      category: 'Snacks',
      unit: 'pack',
      sellingPrice: 2500,
      costPrice: 1200,
      quantity: 45,
      minStock: 12,
      sku: 'SNK-POP-043',
    },

    // 44-47: Personal Care
    {
      name: 'Herbal Toothpaste 100ml',
      category: 'Personal Care',
      unit: 'tube',
      sellingPrice: 5500,
      costPrice: 3400,
      quantity: 35,
      minStock: 8,
      sku: 'PER-TPS-044',
    },
    {
      name: 'Soft Bristle Toothbrush 2-Pack',
      category: 'Personal Care',
      unit: 'pack',
      sellingPrice: 4500,
      costPrice: 2200,
      quantity: 40,
      minStock: 10,
      sku: 'PER-TBR-045',
    },
    {
      name: 'Antibacterial Body Soap 150g',
      category: 'Personal Care',
      unit: 'piece',
      sellingPrice: 3500,
      costPrice: 1900,
      quantity: 55,
      minStock: 15,
      sku: 'PER-SOP-046',
    },
    {
      name: 'Nourishing Cocoa Body Lotion 400ml',
      category: 'Personal Care',
      unit: 'bottle',
      sellingPrice: 13500,
      costPrice: 8500,
      quantity: 18,
      minStock: 5,
      sku: 'PER-LTN-047',
    },

    // 48-50: Household
    {
      name: 'Bio Laundry Detergent Powder 1kg',
      category: 'Household',
      unit: 'pack',
      sellingPrice: 9500,
      costPrice: 6200,
      quantity: 26,
      minStock: 6,
      sku: 'HOU-DET-048',
    },
    {
      name: 'Lemon Antibacterial Dish Soap 500ml',
      category: 'Household',
      unit: 'bottle',
      sellingPrice: 4500,
      costPrice: 2600,
      quantity: 34,
      minStock: 8,
      sku: 'HOU-DSH-049',
    },
    {
      name: 'Multi-Surface Floor Cleaner 1L',
      category: 'Household',
      unit: 'bottle',
      sellingPrice: 12000,
      costPrice: 7500,
      quantity: 20,
      minStock: 5,
      sku: 'HOU-FLR-050',
    },
  ]

  return items.map((item, index) => ({
    id: index + 1,
    sku: item.sku,
    name: item.name,
    category: item.category,
    barcode: generateEan13(index + 1),
    unit: item.unit,
    sellingPrice: item.sellingPrice,
    costPrice: item.costPrice,
    quantity: item.quantity,
    minStock: item.minStock,
    image: item.image,
    active: true,
  }))
}

function seedState(): GuestState {
  const demo = createDemoProducts()
  return {
    schemaVersion: 6,
    guestDeviceId: getGuestDeviceId(),
    idempotencyKey: uuid(),
    demoData: false,
    products: demo,
    categories: Array.from(new Set(demo.map(p => p.category))).sort(),
    sales: [],
    stockMovements: demo.map(p => ({
      id: uuid(),
      productId: p.id,
      quantityDelta: p.quantity,
      reason: 'Opening stock',
      createdAt: new Date().toISOString(),
    })),
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
  if (state.products.length === 0 || (state.schemaVersion && state.schemaVersion < 6)) {
    const demo = createDemoProducts()
    state.products = demo
    state.categories = Array.from(new Set(demo.map(p => p.category))).sort()
    state.schemaVersion = 6
    writeState(state)
  }
  if (state.schemaVersion !== 6) {
    state.schemaVersion = 6
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
