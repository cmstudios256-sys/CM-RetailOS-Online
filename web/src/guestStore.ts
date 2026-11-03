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
  schemaVersion: 1 | 2 | 3 | 4
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
  let randomState = 0x5eed1234
  const random = () => {
    randomState = (Math.imul(randomState, 1664525) + 1013904223) >>> 0
    return randomState / 0x100000000
  }
  const integer = (min: number, max: number) => Math.floor(random() * (max - min + 1)) + min
  const ean13 = (index: number) => {
    const base = `640000${String(index + 1).padStart(6, '0')}`
    let sum = 0
    for (let position = 0; position < 12; position++) {
      sum += Number(base[position]) * (position % 2 === 0 ? 1 : 3)
    }
    return `${base}${(10 - (sum % 10)) % 10}`
  }
  const catalog: Record<string, Array<[string, string, number, number]>> = {
    Groceries: [
      ['Sugar 1kg','pack',4500,.72], ['Posho Flour 2kg','pack',6200,.68], ['Rice 2kg','pack',8200,.70],
      ['Rice 5kg','bag',18000,.69], ['Salt 1kg','pack',2800,.75], ['Cooking Oil 500ml','bottle',6000,.66],
      ['Cooking Oil 1L','bottle',11500,.65], ['Beans 1kg','pack',5200,.62], ['Maize Flour 2kg','pack',4800,.69],
      ['Tea Leaves 250g','pack',4500,.64], ['Milk Powder 400g','tin',14500,.71], ['Tomato Sauce','bottle',3500,.58],
      ['Bread Loaf','loaf',4200,.60], ['Egg Tray','tray',13000,.65], ['Canned Tomatoes','can',3800,.62],
      ['Groundnuts 500g','pack',7500,.55], ['Biscuits','pack',3000,.48], ['Noodles','pack',2500,.52],
      ['Banana Matooke','bunch',18000,.55], ['Irish Potatoes 5kg','bag',12000,.60],
      ['Ghee 500g','tub',10500,.67], ['Breakfast Cereal','pack',17000,.64],
    ],
    Beverages: [
      ['Soda 500ml','bottle',1500,.61], ['Soda 1L','bottle',3000,.64], ['Mineral Water 500ml','bottle',1800,.58],
      ['Mineral Water 1.5L','bottle',3000,.60], ['Energy Drink 400ml','can',4000,.69], ['Fresh Juice 500ml','bottle',3500,.57],
      ['Instant Coffee 200g','jar',16000,.70], ['Black Tea 100 bags','box',13000,.68], ['Malt Drink','bottle',2500,.62],
      ['Yoghurt 500ml','bottle',3500,.60], ['Chilled Water 20L','bottle',9000,.55], ['Fruit Drink 1L','carton',6000,.62],
      ['Soya Drink','bottle',2500,.58], ['Iced Coffee','bottle',4500,.53], ['Sports Drink','bottle',3500,.66],
      ['Lemonade 500ml','bottle',2200,.63], ['Carbonated Water','bottle',2500,.59], ['Pineapple Juice 1L','carton',6500,.60],
      ['Mango Juice 500ml','bottle',3200,.56], ['Chocolate Malt','bottle',2800,.61], ['Milk 500ml','bottle',2200,.60],
      ['Cocoa Drink 200g','tin',12000,.68],
    ],
    Snacks: [
      ['Potato Crisps','pack',3500,.45], ['Popcorn Snack','pack',2500,.42], ['Roasted Groundnuts','pack',3000,.48],
      ['Samosa 4-pack','pack',6000,.50], ['Mandazi Pack','pack',4000,.52], ['Chocolate Bar','piece',3500,.55],
      ['Sweet Biscuits','pack',2800,.43], ['Cheese Crackers','pack',4500,.46], ['Candy Mix','pack',2200,.40],
      ['Chewing Gum','pack',2000,.44], ['Cookies','pack',3800,.49], ['Doughnut','piece',1800,.50],
      ['Meat Pie','piece',3500,.47], ['Banana Crisps','pack',3200,.43], ['Peanut Brittle','pack',3000,.42],
      ['Corn Nuts','pack',2800,.45], ['Fruit Bar','piece',2500,.44], ['Cupcake','piece',3000,.48],
      ['Mini Cake','pack',5500,.52], ['Pretzels','pack',4200,.46], ['Ice Cream Cone','piece',4000,.54],
      ['Chocolate Wafers','pack',3000,.45],
    ],
    Stationery: [
      ['Exercise Book A4','piece',2800,.52], ['Exercise Book A5','piece',1800,.50], ['Blue Pen','piece',1000,.38],
      ['Black Marker','piece',3500,.44], ['Pencil Set','pack',2500,.42], ['School Eraser','piece',800,.35],
      ['Geometry Set','pack',12000,.58], ['Stapler','piece',9000,.60], ['Staples Pack','pack',4500,.56],
      ['Glue Stick','piece',5000,.48], ['File Folder','piece',2200,.46], ['Ring Binder','piece',7000,.55],
      ['Plain Copy Paper','ream',22000,.70], ['Ruled Notebook','piece',4500,.49], ['Index Card Pack','pack',3000,.45],
      ['Correction Fluid','bottle',5500,.52], ['Highlighter','piece',3500,.47], ['Permanent Marker','piece',4000,.49],
      ['Packing Tape','roll',6500,.58], ['Envelope Pack','pack',5000,.54], ['Calculator','piece',35000,.65],
      ['Whiteboard Duster','piece',6000,.50],
    ],
    Household: [
      ['Laundry Detergent 1kg','pack',18000,.68], ['Dishwashing Liquid 500ml','bottle',7000,.60],
      ['Bar Soap Pack','pack',6500,.57], ['Toilet Cleaner','bottle',8000,.62], ['Floor Cleaner 1L','bottle',12000,.65],
      ['Mopping Bucket','piece',28000,.72], ['Kitchen Sponge','pack',3500,.44], ['Garbage Bags','roll',9000,.55],
      ['Matches Box','box',1500,.50], ['Torch Battery AA','pack',6000,.48], ['Light Bulb LED','piece',14000,.66],
      ['Laundry Basket','piece',22000,.68], ['Cloth Hanger 5-pack','pack',8000,.62], ['Cleaning Cloth','piece',4000,.45],
      ['Dust Pan','piece',10000,.64], ['Broom','piece',8500,.61], ['Mosquito Coil Pack','pack',5500,.52],
      ['Air Freshener','can',12500,.67], ['Glass Cleaner','bottle',9000,.63], ['Steel Wool Pack','pack',4500,.49],
      ['Table Cloth','piece',16000,.66], ['Plastic Basin','piece',18000,.67],
    ],
    'Personal Care': [
      ['Toothpaste 100ml','tube',5500,.64], ['Toothbrush','piece',4000,.45], ['Bath Soap','piece',3500,.52],
      ['Shampoo 400ml','bottle',9000,.61], ['Body Lotion 400ml','bottle',12000,.63], ['Roll-on Deodorant','piece',8500,.58],
      ['Hair Oil','bottle',7000,.56], ['Cotton Buds','pack',4500,.49], ['Hand Sanitizer 250ml','bottle',6500,.60],
      ['Face Tissue','pack',3000,.46], ['Disposable Razors','pack',7000,.57], ['Menstrual Pads','pack',7500,.60],
      ['Vaseline 250ml','tin',9000,.62], ['Hair Conditioner','bottle',10500,.64], ['Petroleum Jelly','tin',6500,.59],
      ['Nail Cutter','piece',5000,.50], ['Comb','piece',2500,.42], ['Lip Balm','piece',4500,.47],
      ['Perfume Roll-on','bottle',8000,.55], ['Skin Cream','tube',11000,.63], ['Baby Powder','tin',7500,.61],
      ['Bath Sponge','piece',3500,.48],
    ],
    Baby: [
      ['Baby Diapers Pack','pack',28000,.69], ['Baby Wipes','pack',9000,.60], ['Baby Bath Soap','piece',5500,.53],
      ['Baby Lotion','bottle',12000,.64], ['Baby Oil','bottle',8500,.59], ['Baby Cereal','pack',16000,.67],
      ['Baby Milk 400g','tin',18000,.70], ['Baby Feeding Bottle','piece',13000,.65], ['Baby Blanket','piece',25000,.72],
      ['Pacifier','piece',7000,.57], ['Teething Toy','piece',9000,.60], ['Baby Socks','pair',8000,.55],
      ['Baby Shampoo','bottle',9500,.62], ['Changing Mat','piece',22000,.69], ['Baby Bib','piece',6000,.51],
      ['Nappy Rash Cream','tube',10000,.63], ['Baby Towel','piece',18000,.68], ['Cotton Wool','pack',5500,.48],
      ['Baby Food Jar','jar',8000,.58], ['Baby Detergent','pack',14000,.66], ['Baby Hat','piece',7500,.54],
      ['Baby Plastic Bowl','piece',8500,.56],
    ],
    'Airtime & Accessories': [
      ['MTN Airtime UGX 5k','voucher',5000,.95], ['MTN Airtime UGX 10k','voucher',10000,.95],
      ['Airtel Airtime UGX 5k','voucher',5000,.95], ['Airtel Airtime UGX 10k','voucher',10000,.95],
      ['USB Charging Cable','piece',12000,.68], ['Phone Charger','piece',28000,.72], ['Power Bank 10Ah','piece',65000,.75],
      ['SIM Card','piece',3000,.50], ['Phone Pouch','piece',15000,.64], ['Screen Protector','piece',9000,.58],
      ['Earphones','piece',18000,.66], ['Memory Card 32GB','piece',35000,.70], ['OTG Adapter','piece',10000,.60],
      ['Bluetooth Speaker Mini','piece',55000,.74], ['Extension Cable','piece',22000,.69], ['Phone Stand','piece',14000,.61],
      ['Headset','piece',25000,.67], ['Car Charger','piece',18000,.65], ['Wall Adapter','piece',16000,.63],
      ['Data Cable Type-C','piece',15000,.67], ['Lightning Cable','piece',18000,.69], ['Phone Ring Holder','piece',6000,.52],
    ],
    Hardware: [
      ['Padlock Medium','piece',12000,.60], ['Hammer','piece',18000,.62], ['Screwdriver Set','set',22000,.64],
      ['Tape Measure 5m','piece',15000,.58], ['Wall Nails Pack','pack',8000,.50], ['Door Hinges Pair','pair',9000,.55],
      ['PVC Elbow','piece',3000,.52], ['PVC Pipe 1m','piece',11000,.57], ['Electrical Tape','roll',3500,.48],
      ['Bulb Holder','piece',5000,.54], ['Extension Socket','piece',16000,.66], ['Wood Glue','bottle',9000,.59],
      ['Paint Brush 2in','piece',7000,.51], ['Utility Knife','piece',6500,.53], ['Rubber Mallet','piece',14000,.61],
      ['Hose Clamp Pack','pack',8000,.56], ['Tile Spacer Pack','pack',4000,.47], ['Wall Plug Pack','pack',5000,.49],
      ['Padlock Small','piece',8000,.58], ['Screw Pack','pack',6000,.50],
    ],
    Health: [
      ['Paracetamol Tablets','pack',3000,.55], ['Vitamin C Tablets','pack',8000,.60], ['ORS Sachets','pack',4500,.58],
      ['Antiseptic 250ml','bottle',9000,.62], ['Bandage Roll','piece',5500,.52], ['First Aid Kit','kit',35000,.72],
      ['Antacid Tablets','pack',6500,.57], ['Cough Syrup','bottle',12000,.64], ['Digital Thermometer','piece',22000,.68],
      ['Face Mask Pack','pack',7000,.54], ['Hand Gloves Pack','pack',9000,.56], ['Pain Relief Gel','tube',10000,.61],
      ['Eye Drops','bottle',8000,.60], ['Medical Tape','roll',5000,.53], ['Cotton Wool Roll','pack',6500,.55],
      ['Disposable Gloves','pack',12000,.59], ['Oral Rehydration','pack',5000,.57], ['Insect Bite Cream','tube',9000,.60],
      ['Antiseptic Wipes','pack',6500,.54], ['Pregnancy Test Kit','piece',12000,.63], ['Blood Pressure Monitor','piece',85000,.78],
      ['Alcohol Swabs','pack',5500,.52],
    ],
  }
  const categoryNames = Object.keys(catalog)
  const productSeeds: Product[] = []
  let nextId = 1
  for (const category of categoryNames) {
    for (const raw of catalog[category]) {
      const [name, unit, price, costRate] = raw
      const variantOffset = integer(-5, 5)
      const sellingPrice = Math.max(500, Math.round((price * (1 + variantOffset / 100)) / 100) * 100)
      const costPrice = Math.round(sellingPrice * Number(costRate) / 100) * 100
      const stockRoll = nextId % 100
      const state = stockRoll < 77 ? 'in' : stockRoll < 92 ? 'low' : 'out'
      const minStock = integer(4, 12)
      const quantity = state === 'out' ? 0 : state === 'low' ? integer(1, Math.max(1, Math.floor(minStock * .8))) : integer(minStock + 5, 140)
      productSeeds.push({
        id: nextId,
        sku: `DEMO-${String(nextId).padStart(3, '0')}`,
        name: String(name),
        category,
        barcode: ean13(nextId - 1),
        unit: String(unit),
        sellingPrice,
        costPrice,
        quantity,
        minStock,
        image: '',
      })
      nextId++
    }
  }
  if (productSeeds.length < 200) throw new Error('Demo catalog is too small.')
  const sales: Sale[] = []
  const paymentMethods: Sale['paymentMethod'][] = ['cash', 'mobile_money', 'card']
  const peakHours = [7, 7, 8, 12, 12, 13, 14, 17, 18, 18, 19, 18, 19, 10, 15, 20, 1]
  const addSale = (index: number, date: Date) => {
    const itemCount = index % 17 === 0 ? 6 : integer(1, 6)
    const selectedItems: SaleItem[] = []
    for (let itemIndex = 0; itemIndex < itemCount; itemIndex++) {
      const product = itemIndex === 0 && index % 2 === 0
        ? productSeeds[25]
        : index % 11 === 0 && itemIndex === 0
          ? productSeeds[72]
          : productSeeds[integer(0, productSeeds.length - 1)]
      const quantity = integer(1, 4)
      const lineTotal = Math.round(product.sellingPrice * quantity * 100) / 100
      selectedItems.push({
        productId: product.id,
        productName: product.name,
        quantity,
        unitPrice: product.sellingPrice,
        unitCost: product.costPrice,
        lineTotal,
      })
    }
    const subtotal = selectedItems.reduce((sum, item) => sum + item.lineTotal, 0)
    const discount = index % 9 === 0 ? Math.round(subtotal * integer(5, 12) / 100 / 100) * 100 : 0
    const total = Math.round((subtotal - discount) * 100) / 100
    sales.push({
      id: `demo-sale-${String(index + 1).padStart(4, '0')}`,
      cashierName: 'Demo Shop Owner',
      customerName: index % 4 === 0 ? 'Demo Customer' : '',
      subtotal,
      discount,
      total,
      paymentMethod: paymentMethods[index % paymentMethods.length],
      completedAt: date.toISOString(),
      items: selectedItems,
    })
  }

  for (let index = 0; index < 2000; index++) {
    let date: Date
    if (index < 36) {
      const hour = index === 0 ? 1 : index % 2 === 0 ? 18 : peakHours[index % peakHours.length]
      date = new Date(now.getFullYear(), now.getMonth(), now.getDate(), hour, integer(0, 59), 0)
    } else {
      const daysAgo = 2 + integer(0, 87)
      date = new Date(now)
      date.setDate(date.getDate() - daysAgo)
      date.setHours(peakHours[integer(0, peakHours.length - 1)], integer(0, 59), 0, 0)
    }
    addSale(index, date)
  }

  const movements: GuestStockMovement[] = productSeeds.filter(product => product.quantity > 0).map(product => ({
    id: `demo-opening-${product.id}`,
    productId: product.id,
    quantityDelta: product.quantity,
    reason: 'Demo opening stock',
    createdAt: now.toISOString(),
  }))
  return {
    schemaVersion: 4,
    guestDeviceId: getGuestDeviceId(),
    idempotencyKey: uuid(),
    demoData: true,
    products: productSeeds,
    categories: categoryNames,
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
  if (state.demoData && state.schemaVersion !== 4) {
    const refreshed = seedState()
    writeState(refreshed)
    return refreshed
  }
  if (state.schemaVersion !== 4) {
    state.schemaVersion = 4
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
  const categoryTotals = new Map<string, number>()
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
