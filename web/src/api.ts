import { guestApiRequest } from './guestStore'

export class ApiError extends Error {
  constructor(message: string, readonly status: number, readonly code: string) {
    super(message)
  }
}

let apiMode: 'cloud' | 'guest' = 'cloud'

export function setApiMode(mode: 'cloud' | 'guest'): void {
  apiMode = mode
}

export async function apiRequest<T>(
  path: string,
  options: { method?: string; body?: unknown } = {},
): Promise<T> {
  if (apiMode === 'guest') {
    try {
      return await guestApiRequest<T>(path, options)
    } catch (error) {
      throw error instanceof ApiError ? error : new ApiError(
        error instanceof Error ? error.message : 'The local guest request could not be completed.',
        400,
        'guest_request_failed',
      )
    }
  }

  let response: Response
  try {
    response = await fetch(`/api/v1${path}`, {
      method: options.method ?? (options.body === undefined ? 'GET' : 'POST'),
      credentials: 'same-origin',
      headers: options.body === undefined ? undefined : { 'Content-Type': 'application/json' },
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
    })
  } catch {
    throw new Error('Could not connect to CM RetailOS Online. Check your connection and try again.')
  }

  const responseBody = response.status === 204 ? '' : await response.text()
  let payload: { error?: { code?: string; message?: string } } | null = null
  if (responseBody) {
    try {
      payload = JSON.parse(responseBody)
    } catch {
      payload = null
    }
  }
  if (!response.ok) {
    const message = payload?.error?.message
      ?? (response.status >= 500
        ? 'CM RetailOS Online is temporarily unavailable. Please try again shortly.'
        : 'The request could not be completed.')
    throw new ApiError(message, response.status, payload?.error?.code ?? 'request_failed')
  }
  return payload as T
}

export type Product = {
  id: number
  sku: string
  name: string
  category: string
  barcode: string
  unit: string
  sellingPrice: number
  costPrice: number
  quantity: number
  minStock: number
  image?: string
  active?: boolean
}

export type ProductInput = Omit<Product, 'id'>

export type SaleItem = {
  productId: number
  productName: string
  quantity: number
  unitPrice: number
  unitCost: number
  lineTotal: number
}

export type Sale = {
  id: string
  cashierName: string
  customerName: string
  subtotal: number
  discount: number
  total: number
  paymentMethod: 'cash' | 'mobile_money' | 'card'
  completedAt: string
  items: SaleItem[]
}

export function formatMoney(value: number, currency: string): string {
  try {
    return new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency: currency || 'UGX',
      maximumFractionDigits: currency === 'UGX' ? 0 : 2,
    }).format(value)
  } catch {
    return `${currency || 'UGX'} ${value.toLocaleString()}`
  }
}

export function formatQuantity(value: number): string {
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 3 }).format(value)
}

export function formatDateTime(value: string): string {
  const date = new Date(value)
  return Number.isNaN(date.getTime())
    ? 'Date unavailable'
    : new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(date)
}

const APP_TIME_ZONE = 'Africa/Kampala'
const LOCAL_PREFS_KEY = 'cm_retailos_online_settings_prefs_v1'
const LOCAL_STAFF_PERMS_KEY = 'cm_retailos_online_staff_perms_v1'

export function readStaffPermissions(userId: string, role: string): string[] {
  const normalizedRole = role.toLowerCase()
  if (normalizedRole === 'owner' || normalizedRole === 'manager' || normalizedRole === 'administrator') {
    return ['page_pos', 'page_inventory', 'page_bi', 'page_sales_history', 'page_audit', 'page_settings']
  }
  try {
    const raw = localStorage.getItem(LOCAL_STAFF_PERMS_KEY)
    if (raw) {
      const map = JSON.parse(raw) as Record<string, string[]>
      if (Array.isArray(map[userId]) && map[userId].length > 0) {
        return map[userId]
      }
    }
  } catch {
    // ignore invalid localStorage state
  }
  return ['page_pos', 'page_sales_history']
}

export type OnlineSettingsPrefs = {
  owner?: string
  country?: string
  logo?: string
  receiptsSettings?: {
    autoPrint?: boolean
    showLogo?: boolean
    showCashier?: boolean
    printCopies?: number
    printerName?: string
    paperWidth?: string
  }
  applicationSettings?: {
    themeMode?: 'dark' | 'light' | 'system'
    accent?: string
    typography?: string
    autoClearCart?: boolean
    requirePinForDiscounts?: boolean
    barcodeBeep?: boolean
    discountPin?: string
    previewPosImagesOnHover?: boolean
    showPosImages?: boolean
    hideOutOfStock?: boolean
  }
  localizationSettings?: {
    dateFormat?: string
    timeFormat?: string
  }
}

export function readOnlinePrefs(): OnlineSettingsPrefs {
  try {
    const raw = localStorage.getItem(LOCAL_PREFS_KEY)
    return raw ? (JSON.parse(raw) as OnlineSettingsPrefs) : {}
  } catch {
    return {}
  }
}

export async function syncOnlinePrefsFromServer(currentUserId?: string): Promise<void> {
  try {
    const result = await apiRequest<{
      preferences?: OnlineSettingsPrefs
      permissions?: string[]
      staffPermissions?: Record<string, string[]>
    }>('/settings/preferences')

    if (result.preferences && Object.keys(result.preferences).length > 0) {
      const merged: OnlineSettingsPrefs = {
        ...readOnlinePrefs(),
        ...result.preferences,
        receiptsSettings: {
          ...readOnlinePrefs().receiptsSettings,
          ...result.preferences.receiptsSettings,
        },
        applicationSettings: {
          ...readOnlinePrefs().applicationSettings,
          ...result.preferences.applicationSettings,
        },
        localizationSettings: {
          ...readOnlinePrefs().localizationSettings,
          ...result.preferences.localizationSettings,
        },
      }
      localStorage.setItem(LOCAL_PREFS_KEY, JSON.stringify(merged))
    }

    const rawMap = localStorage.getItem(LOCAL_STAFF_PERMS_KEY)
    const permsMap: Record<string, string[]> = rawMap ? JSON.parse(rawMap) : {}
    if (result.staffPermissions) {
      for (const [memberId, perms] of Object.entries(result.staffPermissions)) {
        if (Array.isArray(perms) && perms.length > 0) {
          permsMap[memberId] = perms
        }
      }
    }
    if (currentUserId && Array.isArray(result.permissions) && result.permissions.length > 0) {
      permsMap[currentUserId] = result.permissions
    }
    localStorage.setItem(LOCAL_STAFF_PERMS_KEY, JSON.stringify(permsMap))

    applyOnlinePrefsToDom()
    window.dispatchEvent(new CustomEvent('online-prefs-updated'))
  } catch {
    // ignore if offline or endpoint not yet migrated
  }
}

export function applyOnlinePrefsToDom(): void {
  const prefs = readOnlinePrefs()
  const root = document.documentElement
  const mode = prefs.applicationSettings?.themeMode ?? 'dark'
  if (mode === 'system') {
    const prefersLight = window.matchMedia('(prefers-color-scheme: light)').matches
    if (prefersLight) root.setAttribute('data-theme', 'light')
    else root.removeAttribute('data-theme')
  } else if (mode === 'light') {
    root.setAttribute('data-theme', 'light')
  } else {
    root.removeAttribute('data-theme')
  }

  const accent = prefs.applicationSettings?.accent
  if (accent) {
    root.style.setProperty('--accent-color', accent)
  }

  const font = prefs.applicationSettings?.typography
  if (font) {
    const stack = font === 'System Default'
      ? "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif"
      : `'${font}', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif`
    root.style.setProperty('--app-font-family', stack)
  }
}

export function formatDate(value: Date | string): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return String(value)
  const parts = new Intl.DateTimeFormat('en-GB', {
    timeZone: APP_TIME_ZONE,
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
  }).formatToParts(date)
  const day = parts.find(part => part.type === 'day')?.value ?? ''
  const monthNum = parts.find(part => part.type === 'month')?.value ?? ''
  const year = parts.find(part => part.type === 'year')?.value ?? ''
  const monthNames = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
  const month = monthNames[Number(monthNum) - 1] ?? monthNum
  const fmt = readOnlinePrefs().localizationSettings?.dateFormat
  if (fmt === '15 Aug 2026' || fmt === 'DD MMM YYYY') return `${day} ${month} ${year}`
  if (fmt === '2026-08-15' || fmt === 'YYYY-MM-DD') return `${year}-${monthNum}-${day}`
  if (fmt === '08/15/2026' || fmt === 'MM/DD/YYYY') return `${monthNum}/${day}/${year}`
  return `${month} ${day}, ${year}`
}

export function formatTime(value: Date | string): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const use24Hour = readOnlinePrefs().localizationSettings?.timeFormat === '24-hour'
  const parts = new Intl.DateTimeFormat('en-US', {
    timeZone: APP_TIME_ZONE,
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: use24Hour ? 'h23' : 'h12',
  }).formatToParts(date)
  const hours = parts.find(part => part.type === 'hour')?.value ?? '00'
  const minutes = parts.find(part => part.type === 'minute')?.value ?? '00'
  const seconds = parts.find(part => part.type === 'second')?.value ?? '00'
  const ampm = parts.find(part => part.type === 'dayPeriod')?.value ?? ''
  return use24Hour
    ? `${hours.padStart(2, '0')}:${minutes}:${seconds}`
    : `${hours.padStart(2, '0')}:${minutes}:${seconds} ${ampm}`.trim()
}

export function formatISODate(value: Date | string): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return ''
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: APP_TIME_ZONE,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).formatToParts(date)
  const year = parts.find(part => part.type === 'year')?.value ?? ''
  const month = parts.find(part => part.type === 'month')?.value ?? ''
  const day = parts.find(part => part.type === 'day')?.value ?? ''
  return `${year}-${month}-${day}`
}

export function formatAxisValue(value: number): string {
  const abs = Math.abs(value)
  if (abs >= 1_000_000) {
    const scaled = value / 1_000_000
    return `${Number.isInteger(scaled) ? scaled : scaled.toFixed(1)}M`
  }
  if (abs >= 1_000) {
    const scaled = value / 1_000
    return `${Number.isInteger(scaled) ? scaled : scaled.toFixed(1)}k`
  }
  return Number.isInteger(value) ? String(value) : value.toFixed(0)
}

export function isGoogleAccountLinked(user?: { email?: string; googleConnected?: boolean } | null): boolean {
  return user?.googleConnected === true
}
