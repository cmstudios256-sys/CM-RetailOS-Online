import type { Sale } from '../api'
import { formatDate, formatMoney, formatQuantity, formatTime } from '../api'
import { receiptNumber, shortSaleId } from './reporting'

export type BluetoothPrinterInfo = {
  id: string
  name: string
  protocol: 'escpos'
  connectionType: 'bluetooth'
  address: string
  paperWidth: '58mm' | '80mm'
  characterWidth: number
  isDefault: boolean
}

const LOCAL_PRINTERS_KEY = 'cm_retailos_online_printers_v1'
const LOCAL_JOBS_KEY = 'cm_retailos_online_print_jobs_v1'

// Common thermal printer Bluetooth GATT Services
export const THERMAL_PRINTER_SERVICES = [
  '000018f0-0000-1000-8000-00805f9b34fb',
  'e7810a71-73ae-499d-8c15-faa9aef0c3f2',
  '49535343-fe7d-4ae5-8fa9-9fafd205e455',
  '0000ff00-0000-1000-8000-00805f9b34fb',
  '0000af30-0000-1000-8000-00805f9b34fb',
  '0000ffe0-0000-1000-8000-00805f9b34fb',
  '0000fee7-0000-1000-8000-00805f9b34fb',
  '0000ae30-0000-1000-8000-00805f9b34fb',
]

// In-memory active Bluetooth connection
let cachedDevice: any = null
let cachedGattServer: any = null
let cachedCharacteristic: any = null
let isConnectingGatt = false

function dispatchPrinterStatus(connected: boolean, deviceName: string) {
  if (typeof window !== 'undefined') {
    window.dispatchEvent(
      new CustomEvent('bluetooth-printer-status', {
        detail: { connected, deviceName },
      }),
    )
  }
}

function handleDisconnection() {
  cachedGattServer = null
  cachedCharacteristic = null
  dispatchPrinterStatus(false, cachedDevice?.name || 'Bluetooth Printer')
}

async function findWritableCharacteristic(server: any): Promise<any> {
  // 1. Try known thermal printer service UUIDs first
  for (const uuid of THERMAL_PRINTER_SERVICES) {
    try {
      const service = await server.getPrimaryService(uuid)
      if (service) {
        const chars = await service.getCharacteristics()
        for (const char of chars) {
          if (char.properties.write || char.properties.writeWithoutResponse) {
            return char
          }
        }
      }
    } catch {
      // Continue checking other services
    }
  }

  // 2. Generic fallback: enumerate all primary services
  try {
    const services = await server.getPrimaryServices()
    for (const service of services) {
      try {
        const chars = await service.getCharacteristics()
        for (const char of chars) {
          if (char.properties.write || char.properties.writeWithoutResponse) {
            return char
          }
        }
      } catch {
        // ignore
      }
    }
  } catch {
    // ignore
  }

  return null
}

export async function connectToGattDevice(device: any): Promise<boolean> {
  if (!device) return false
  if (isConnectingGatt) return false
  isConnectingGatt = true
  try {
    cachedDevice = device
    try {
      device.removeEventListener('gattserverdisconnected', handleDisconnection)
      device.addEventListener('gattserverdisconnected', handleDisconnection)
    } catch {}

    const gatt = device.gatt
    if (!gatt) {
      isConnectingGatt = false
      return false
    }

    cachedGattServer = gatt.connected ? gatt : await gatt.connect()
    cachedCharacteristic = await findWritableCharacteristic(cachedGattServer)

    const isConnected = !!(cachedCharacteristic && cachedGattServer?.connected)
    dispatchPrinterStatus(isConnected, device.name || 'Bluetooth Printer')
    isConnectingGatt = false
    return isConnected
  } catch (err) {
    console.warn('[Bluetooth Printer] Connection error:', err)
    cachedGattServer = null
    cachedCharacteristic = null
    dispatchPrinterStatus(false, 'Disconnected')
    isConnectingGatt = false
    return false
  }
}

export function isBluetoothPrinterConnected(): boolean {
  return !!(cachedCharacteristic && cachedGattServer?.connected)
}

export function disconnectBluetoothPrinter(): void {
  try {
    if (cachedGattServer?.connected) {
      cachedGattServer.disconnect()
    }
  } catch {}
  cachedDevice = null
  cachedGattServer = null
  cachedCharacteristic = null
  dispatchPrinterStatus(false, 'Disconnected')
}

export async function autoConnectSavedBluetoothPrinter(): Promise<boolean> {
  if (!isWebBluetoothSupported()) return false
  const nav = navigator as any
  if (!nav.bluetooth || typeof nav.bluetooth.getDevices !== 'function') return false

  try {
    const permittedDevices = await nav.bluetooth.getDevices()
    if (!permittedDevices || permittedDevices.length === 0) return false

    const configured = getDefaultBluetoothPrinter()
    const target = configured?.address
      ? (permittedDevices.find((d: any) => d.id === configured.address || d.name === configured.name) || permittedDevices[0])
      : permittedDevices[0]

    if (target) {
      return await connectToGattDevice(target)
    }
  } catch (err) {
    console.warn('[Bluetooth Printer] Auto-connect failed:', err)
  }
  return false
}

export function isWebBluetoothSupported(): boolean {
  return typeof navigator !== 'undefined' && 'bluetooth' in navigator
}

export function getConfiguredBluetoothPrinters(): BluetoothPrinterInfo[] {
  try {
    const raw = localStorage.getItem(LOCAL_PRINTERS_KEY)
    if (!raw) return []
    const list = JSON.parse(raw) as any[]
    return list.filter(p => p.connectionType === 'bluetooth')
  } catch {
    return []
  }
}

export function getDefaultBluetoothPrinter(): BluetoothPrinterInfo | null {
  const printers = getConfiguredBluetoothPrinters()
  if (printers.length === 0) return null
  return printers.find(p => p.isDefault) || printers[0]
}

export function saveBluetoothPrinter(printer: BluetoothPrinterInfo): void {
  try {
    const raw = localStorage.getItem(LOCAL_PRINTERS_KEY)
    const list: any[] = raw ? JSON.parse(raw) : []
    if (printer.isDefault) {
      list.forEach(p => { p.isDefault = false })
    }
    const idx = list.findIndex(p => p.id === printer.id)
    if (idx >= 0) {
      list[idx] = printer
    } else {
      list.push(printer)
    }
    localStorage.setItem(LOCAL_PRINTERS_KEY, JSON.stringify(list))
    window.dispatchEvent(new CustomEvent('online-printers-updated'))
  } catch {
    // ignore
  }
}

export async function requestWebBluetoothDevice(): Promise<{ name: string; id: string } | null> {
  if (!isWebBluetoothSupported()) {
    throw new Error('Web Bluetooth is not supported in this browser. Try Chrome on Android or Bluefy on iOS.')
  }
  const nav = navigator as any
  try {
    const device = await nav.bluetooth.requestDevice({
      acceptAllDevices: true,
      optionalServices: THERMAL_PRINTER_SERVICES,
    })
    if (!device) return null

    await connectToGattDevice(device)

    return {
      name: device.name || 'Bluetooth Thermal Printer',
      id: device.id || device.name,
    }
  } catch (err: any) {
    if (err.name === 'NotFoundError') {
      return null // User cancelled pairing picker
    }
    throw err
  }
}

/**
 * Sanitizes any string into single-byte ASCII (0x20..0x7E) for thermal printers.
 * Eliminates non-breaking spaces (U+00A0, U+202F) that cause Chinese character artifacts in ESC/POS GBK mode.
 */
export function toAscii(str: string | null | undefined): string {
  if (!str) return ''
  return String(str)
    .replace(/[\u00A0\u1680\u180E\u2000-\u200B\u202F\u205F\u3000\uFEFF]/g, ' ')
    .replace(/[\u2018\u2019]/g, "'")
    .replace(/[\u201C\u201D]/g, '"')
    .replace(/[\u2013\u2014]/g, '-')
    .replace(/[^\x20-\x7E\n\r]/g, '')
}

/**
 * Formats currency amounts safely for thermal printers using standard ASCII space.
 * Avoids Intl.NumberFormat non-breaking space bytes (0xC2 0xA0) which print as Chinese glyphs.
 */
export function formatEscPosMoney(value: number, currency: string = 'UGX'): string {
  const cur = toAscii(currency || 'UGX').trim().toUpperCase()
  const num = Math.round(Number(value) || 0)
  const numStr = num.toLocaleString('en-US')
  return `${cur} ${numStr}`
}

/**
 * Builds standard ESC/POS binary buffer for thermal receipt printing
 */
export function buildEscPosReceipt(
  sale: Sale,
  options: {
    shopName: string
    currency: string
    phone?: string
    address?: string
    headerText?: string
    footerText?: string
    showCashier?: boolean
    paperWidth?: '58mm' | '80mm'
  },
): Uint8Array {
  // 58mm thermal printers have 384 dots (30-32 columns safe width). 30 columns ensures zero line-wrap on all models.
  const is58mm = (options.paperWidth || '58mm') === '58mm'
  const maxChars = is58mm ? 30 : 48
  const divider = '-'.repeat(maxChars)

  const encoder = new TextEncoder()
  const bytes: number[] = []

  function addBytes(...b: number[]) {
    bytes.push(...b)
  }

  function addText(text: string) {
    const clean = toAscii(text)
    const encoded = encoder.encode(clean)
    for (const byte of encoded) {
      bytes.push(byte)
    }
  }

  function addLine(text: string = '') {
    addText(text + '\n')
  }

  function formatRow(left: string, right: string, width: number = maxChars): string {
    const cleanLeft = toAscii(left)
    const cleanRight = toAscii(right)
    const spaces = width - cleanLeft.length - cleanRight.length
    if (spaces >= 1) {
      return cleanLeft + ' '.repeat(spaces) + cleanRight
    }
    // If text exceeds line width, place amount right-aligned on next line to prevent split numbers
    const indent = Math.max(0, width - cleanRight.length)
    return `${cleanLeft}\n${' '.repeat(indent)}${cleanRight}`
  }

  // 1. ESC @ : Initialize printer
  addBytes(0x1b, 0x40)

  // 2. FS . : Cancel Chinese character mode (ensures byte pairs are never treated as Chinese ideograms)
  addBytes(0x1c, 0x2e)

  // 3. ESC t 0 : Select character code page 0 (PC437: Standard ASCII)
  addBytes(0x1b, 0x74, 0x00)

  // 4. ESC a 1 : Center alignment
  addBytes(0x1b, 0x61, 0x01)

  // 5. Store Title: Double height + Bold (standard width so it never clips or wraps)
  addBytes(0x1b, 0x45, 0x01) // Bold ON
  addBytes(0x1b, 0x21, 0x10) // Double height ON
  addLine(toAscii(options.shopName.toUpperCase()))
  addBytes(0x1b, 0x21, 0x00) // Normal size
  addBytes(0x1b, 0x45, 0x00) // Bold OFF

  if (options.address) addLine(toAscii(options.address))
  if (options.phone) addLine(toAscii(`Tel: ${options.phone}`))
  if (options.headerText) addLine(toAscii(options.headerText))

  addLine(divider)

  // 6. Left align for receipt meta
  addBytes(0x1b, 0x61, 0x00)
  addLine(formatRow('Receipt:', '#' + toAscii(receiptNumber(sale)), maxChars))
  addLine(formatRow('Sale ID:', '#' + toAscii(shortSaleId(sale.id)), maxChars))
  addLine(formatRow('Date:', toAscii(`${formatDate(sale.completedAt)} ${formatTime(sale.completedAt)}`), maxChars))
  if (options.showCashier !== false && sale.cashierName) {
    addLine(formatRow('Cashier:', toAscii(sale.cashierName), maxChars))
  }
  if (sale.customerName) {
    addLine(formatRow('Customer:', toAscii(sale.customerName), maxChars))
  }

  addLine(divider)

  // 7. Items header
  addBytes(0x1b, 0x45, 0x01) // Bold
  addLine(formatRow('ITEM', 'AMOUNT', maxChars))
  addBytes(0x1b, 0x45, 0x00) // Unbold
  addLine(divider)

  // 8. Item rows
  for (const item of sale.items) {
    addLine(toAscii(item.productName))
    const qtyPrice = `  ${formatQuantity(item.quantity)} x ${formatEscPosMoney(item.unitPrice, options.currency)}`
    const lineTotal = formatEscPosMoney(item.lineTotal, options.currency)
    addLine(formatRow(qtyPrice, lineTotal, maxChars))
  }

  addLine(divider)

  // 9. Subtotal & Discount
  if (sale.discount > 0) {
    addLine(formatRow('Subtotal:', formatEscPosMoney(sale.subtotal, options.currency), maxChars))
    addLine(formatRow('Discount:', `-${formatEscPosMoney(sale.discount, options.currency)}`, maxChars))
    addLine(divider)
  }

  // 10. TOTAL in double height & bold (normal width so it fits cleanly on ONE line without wrapping!)
  addBytes(0x1b, 0x45, 0x01) // Bold ON
  addBytes(0x1b, 0x21, 0x10) // Double height ON
  addLine(formatRow('TOTAL:', formatEscPosMoney(sale.total, options.currency), maxChars))
  addBytes(0x1b, 0x21, 0x00) // Reset size
  addBytes(0x1b, 0x45, 0x00) // Bold OFF

  addLine(divider)

  // 11. Center align for footer
  addBytes(0x1b, 0x61, 0x01)
  addLine(toAscii(options.footerText || 'Thank you for shopping with us!'))
  addLine('Powered by CM-RetailOS')

  // Feed 4 lines
  addBytes(0x1b, 0x64, 0x04)

  // GS V 0 : Partial/Full paper cut
  addBytes(0x1d, 0x56, 0x00)

  return new Uint8Array(bytes)
}

/**
 * Builds test ESC/POS buffer for printer testing
 */
export function buildEscPosTestPrint(printerName: string, paperWidth: '58mm' | '80mm' = '58mm'): Uint8Array {
  const is58mm = paperWidth === '58mm'
  const maxChars = is58mm ? 30 : 48
  const divider = '-'.repeat(maxChars)

  const bytes: number[] = [
    0x1b, 0x40,        // ESC @ : Init
    0x1c, 0x2e,        // FS . : Cancel Chinese mode
    0x1b, 0x74, 0x00,  // ESC t 0 : Code Page 0 (PC437)
    0x1b, 0x61, 0x01,  // ESC a 1 : Center
  ]
  const encoder = new TextEncoder()

  function addText(text: string) {
    const clean = toAscii(text)
    for (const b of encoder.encode(clean + '\n')) bytes.push(b)
  }

  bytes.push(0x1b, 0x45, 0x01, 0x1b, 0x21, 0x10) // Double height + Bold
  addText('BLUETOOTH PRINTER')
  bytes.push(0x1b, 0x21, 0x00, 0x1b, 0x45, 0x00) // Reset
  addText('TEST RECEIPT')
  addText(divider)
  addText(`Printer: ${toAscii(printerName)}`)
  addText(`Paper: ${paperWidth}`)
  addText(`Protocol: ESC/POS`)
  addText(`Status: Online & Ready`)
  addText(divider)
  addText('CM-RetailOS Bluetooth Driver')
  addText(toAscii(new Date().toLocaleString()))
  bytes.push(0x1b, 0x64, 0x04, 0x1d, 0x56, 0x00) // Feed & cut

  return new Uint8Array(bytes)
}

/**
 * Sends ESC/POS receipt data to the Bluetooth printer
 */
export async function sendToBluetoothPrinter(
  printer: BluetoothPrinterInfo,
  rawBytes: Uint8Array,
): Promise<{ success: boolean; message: string }> {
  if (isWebBluetoothSupported()) {
    // If not connected, attempt auto-reconnect
    if (!cachedCharacteristic || !cachedGattServer?.connected) {
      if (cachedDevice) {
        await connectToGattDevice(cachedDevice)
      } else {
        await autoConnectSavedBluetoothPrinter()
      }
    }

    if (cachedCharacteristic) {
      try {
        const CHUNK_SIZE = 100 // Safe 100-byte chunks to fit within standard BLE MTU limits
        for (let i = 0; i < rawBytes.length; i += CHUNK_SIZE) {
          const chunk = rawBytes.slice(i, i + CHUNK_SIZE)
          if (cachedCharacteristic.properties.writeWithoutResponse) {
            await cachedCharacteristic.writeValueWithoutResponse(chunk)
          } else {
            await cachedCharacteristic.writeValue(chunk)
          }
          await new Promise(resolve => setTimeout(resolve, 25))
        }
      } catch (writeErr: any) {
        console.warn('[Bluetooth Printer] Write error:', writeErr)
        throw new Error(writeErr.message || `Failed to transmit receipt to "${printer.name}".`)
      }
    } else {
      throw new Error(`Bluetooth printer "${printer.name}" is offline or disconnected. Please tap "Scan & Pair Printer" to connect.`)
    }
  } else {
    throw new Error('Web Bluetooth is not supported in this browser. Try Chrome on Android or Bluefy on iOS.')
  }

  // Record print job in local history
  try {
    const rawJobs = localStorage.getItem(LOCAL_JOBS_KEY)
    const jobs: any[] = rawJobs ? JSON.parse(rawJobs) : []
    jobs.unshift({
      id: `bt-job-${Date.now()}`,
      receiptId: 'BT-THERMAL',
      printerName: printer.name,
      status: 'completed',
      createdAt: new Date().toISOString(),
    })
    localStorage.setItem(LOCAL_JOBS_KEY, JSON.stringify(jobs.slice(0, 50)))
  } catch {
    // ignore
  }

  return {
    success: true,
    message: `Receipt printed to ${printer.name} (${printer.paperWidth || '58mm'}).`,
  }
}
