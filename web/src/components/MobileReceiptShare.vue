<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { MessageCircle, Share2, X, Printer, Bluetooth } from '@lucide/vue'
import { formatDate, formatMoney, formatQuantity, formatTime, readOnlinePrefs, type Sale } from '../api'
import { escapeHtml, openPrintDocument, receiptNumber } from '../utils/reporting'
import {
  getDefaultBluetoothPrinter,
  buildEscPosReceipt,
  isWebBluetoothSupported,
  sendToBluetoothPrinter,
} from '../utils/bluetoothPrinter'

const props = defineProps<{
  sale: Sale | null
  shopName: string
  currency: string
}>()
const emit = defineEmits<{ close: [] }>()
const phone = ref('')
const error = ref('')
const isPrintingBt = ref(false)

const receiptText = computed(() => {
  const sale = props.sale
  if (!sale) return ''

  const items = sale.items.map(item =>
    `${formatQuantity(item.quantity)} x ${item.productName} - ${formatMoney(item.lineTotal, props.currency)}`,
  )
  const totals = sale.discount > 0
    ? [
        `Subtotal: ${formatMoney(sale.subtotal, props.currency)}`,
        `Discount: -${formatMoney(sale.discount, props.currency)}`,
      ]
    : []

  return [
    props.shopName,
    `Receipt ${receiptNumber(sale)} · ${formatDate(sale.completedAt)} ${formatTime(sale.completedAt)}`,
    '',
    ...items,
    '',
    ...totals,
    `TOTAL: ${formatMoney(sale.total, props.currency)}`,
    'Thank you for shopping with us!',
  ].join('\n')
})

const DIAL_CODE_BY_COUNTRY: Record<string, string> = {
  uganda: '256',
  kenya: '254',
  tanzania: '255',
  rwanda: '250',
  nigeria: '234',
  ghana: '233',
  'south africa': '27',
  'united kingdom': '44',
  'united states': '1',
}

const DIAL_CODE_BY_CURRENCY: Record<string, string> = {
  UGX: '256',
  KES: '254',
  TZS: '255',
  RWF: '250',
  NGN: '234',
  GHS: '233',
  ZAR: '27',
  GBP: '44',
  USD: '1',
}

const defaultDialCode = computed(() => {
  const country = String(readOnlinePrefs().country || '').trim().toLowerCase()
  if (country && DIAL_CODE_BY_COUNTRY[country]) return DIAL_CODE_BY_COUNTRY[country]
  const cur = String(props.currency || '').trim().toUpperCase()
  return DIAL_CODE_BY_CURRENCY[cur] || '256'
})

const normalizedPhone = computed(() => {
  let digits = phone.value.replace(/\D/g, '')
  if (digits.startsWith('00')) {
    digits = digits.slice(2)
  } else if (digits.startsWith('0') && digits.length >= 9 && digits.length <= 11) {
    digits = `${defaultDialCode.value}${digits.slice(1)}`
  }
  return digits
})
const validPhone = computed(() =>
  !normalizedPhone.value.startsWith('0') &&
  normalizedPhone.value.length >= 9 &&
  normalizedPhone.value.length <= 15,
)
const whatsappUrl = computed(() =>
  validPhone.value
    ? `https://api.whatsapp.com/send?phone=${normalizedPhone.value}&text=${encodeURIComponent(receiptText.value)}`
    : '',
)
const canUseNativeShare = computed(() =>
  typeof navigator !== 'undefined' && typeof navigator.share === 'function',
)

watch(() => props.sale, () => {
  phone.value = ''
  error.value = ''
})

const canUseBluetoothPrint = computed(() => isWebBluetoothSupported())

function printWithSystemSheet() {
  if (!props.sale) return
  const sale = props.sale
  const prefs = readOnlinePrefs()
  const showLogo = prefs.receiptsSettings?.showLogo !== false
  const showCashier = prefs.receiptsSettings?.showCashier !== false
  const headerText = prefs.receiptsSettings?.headerText ?? ''
  const footerText = prefs.receiptsSettings?.footerText || 'Thank you for shopping with us!'
  const receiptMaxWidth = prefs.receiptsSettings?.paperWidth === '58mm' ? '220px' : '300px'
  const logoMaxWidth = prefs.receiptsSettings?.paperWidth === '58mm' ? '140px' : '180px'
  const logoMaxHeight = prefs.receiptsSettings?.paperWidth === '58mm' ? '46px' : '56px'
  const logoHtml = showLogo && prefs.logo
    ? `<div style="display:flex;justify-content:center;margin-bottom:8px;"><img src="${escapeHtml(prefs.logo)}" style="width:auto;max-width:${logoMaxWidth};height:auto;max-height:${logoMaxHeight};object-fit:contain;filter:brightness(0);" /></div>`
    : ''
  const rows = sale.items.map(item => `
    <div style="margin-bottom: 6px;">
      <div style="font-weight: 700;">${escapeHtml(item.productName)}</div>
      <div style="display: flex; justify-content: space-between; padding-left: 8px;">
        <span>${escapeHtml(formatQuantity(item.quantity))} x ${escapeHtml(formatMoney(item.unitPrice, props.currency))}</span>
        <span>${escapeHtml(formatMoney(item.lineTotal, props.currency))}</span>
      </div>
    </div>
  `).join('')

  openPrintDocument(
    `Receipt ${receiptNumber(sale)}`,
    `<div style="max-width: ${receiptMaxWidth}; margin: 0 auto; font-family: 'Courier New', Courier, monospace; font-size: 12px; color: #000; line-height: 1.4;">
      <div style="text-align: center; margin-bottom: 10px;">
        ${logoHtml}
        <h2 style="margin: 0; font-size: 15px; font-weight: 700; text-transform: uppercase;">${escapeHtml(props.shopName)}</h2>
        ${headerText ? `<div style="font-size: 11px; margin-top: 4px;">${escapeHtml(headerText)}</div>` : ''}
      </div>
      <div style="border-top: 1px dashed #000; border-bottom: 1px dashed #000; padding: 6px 0; margin-bottom: 8px; font-size: 11px;">
        <div style="display: flex; justify-content: space-between;"><span>Receipt:</span><span>#${escapeHtml(receiptNumber(sale))}</span></div>
        <div style="display: flex; justify-content: space-between;"><span>Date:</span><span>${escapeHtml(formatDate(sale.completedAt))} ${escapeHtml(formatTime(sale.completedAt))}</span></div>
        ${showCashier ? `<div style="display: flex; justify-content: space-between;"><span>Cashier:</span><span>${escapeHtml(sale.cashierName)}</span></div>` : ''}
      </div>
      <div style="margin-bottom: 8px;">${rows}</div>
      <div style="border-top: 1px dashed #000; padding-top: 6px; font-size: 12px;">
        <div style="display: flex; justify-content: space-between;"><span>Subtotal</span><span>${escapeHtml(formatMoney(sale.subtotal, props.currency))}</span></div>
        ${sale.discount > 0 ? `<div style="display: flex; justify-content: space-between;"><span>Discount</span><span>-${escapeHtml(formatMoney(sale.discount, props.currency))}</span></div>` : ''}
        <div style="display: flex; justify-content: space-between; font-weight: 700; font-size: 14px; margin-top: 4px;"><span>TOTAL</span><span>${escapeHtml(formatMoney(sale.total, props.currency))}</span></div>
      </div>
      <div style="text-align: center; margin-top: 12px; font-size: 11px;">${escapeHtml(footerText)}</div>
    </div>`,
  )
}

async function printWithBluetooth() {
  if (!props.sale) return
  error.value = ''
  const printer = getDefaultBluetoothPrinter()
  if (!printer) {
    error.value = 'No Bluetooth thermal printer configured. Connect your 58mm/80mm printer in More > Settings > Receipt & Printing.'
    return
  }
  isPrintingBt.value = true
  try {
    const prefs = readOnlinePrefs()
    const paperWidth = ((printer.paperWidth || prefs.receiptsSettings?.paperWidth || '58mm') === '80mm' ? '80mm' : '58mm') as '58mm' | '80mm'
    const bytes = buildEscPosReceipt(props.sale, {
      shopName: props.shopName,
      currency: props.currency,
      paperWidth,
      showCashier: prefs.receiptsSettings?.showCashier !== false,
      headerText: prefs.receiptsSettings?.headerText ?? '',
      footerText: prefs.receiptsSettings?.footerText || 'Thank you for shopping with us!',
    })
    await sendToBluetoothPrinter(printer, bytes)
    window.setTimeout(() => {
      emit('close')
    }, 1500)
  } catch (err: any) {
    error.value = err.message || 'Failed to print to Bluetooth printer.'
  } finally {
    isPrintingBt.value = false
  }
}

function openWhatsApp() {
  if (!validPhone.value || !whatsappUrl.value) {
    error.value = 'Enter a valid phone number with its country code (e.g. 256700000000).'
    return
  }
  const isStandalonePwa =
    typeof window !== 'undefined' &&
    ((window.navigator as Navigator & { standalone?: boolean }).standalone === true ||
      window.matchMedia?.('(display-mode: standalone)').matches)
  if (isStandalonePwa) {
    window.location.assign(whatsappUrl.value)
  } else {
    const shareWindow = window.open(whatsappUrl.value, '_blank')
    if (shareWindow) shareWindow.opener = null
    else window.location.assign(whatsappUrl.value)
  }
  emit('close')
}

async function shareWithDevice() {
  if (!props.sale || !canUseNativeShare.value) return
  error.value = ''
  try {
    await navigator.share({ title: `Receipt from ${props.shopName}`, text: receiptText.value })
    emit('close')
  } catch (reason) {
    if (reason instanceof Error && reason.name === 'AbortError') return
    error.value = reason instanceof Error ? reason.message : 'The receipt could not be shared.'
  }
}
</script>

<template>
  <div v-if="sale" class="mobile-share-backdrop" @click.self="emit('close')">
    <section
      class="mobile-share-dialog"
      role="dialog"
      aria-modal="true"
      aria-labelledby="mobile-share-title"
    >
      <header class="mobile-share-heading">
        <div>
          <h2 id="mobile-share-title">Share receipt</h2>
          <p>Receipt {{ receiptNumber(sale) }} · {{ formatMoney(sale.total, currency) }}</p>
        </div>
        <button type="button" class="mobile-share-close" aria-label="Close share dialog" @click="emit('close')">
          <X :size="19" />
        </button>
      </header>

      <label class="mobile-share-label" for="receipt-whatsapp-phone">Customer's WhatsApp number</label>
      <input
        id="receipt-whatsapp-phone"
        v-model="phone"
        class="mobile-share-phone"
        type="tel"
        inputmode="tel"
        autocomplete="off"
        placeholder="Include country code, e.g. 256700000000"
        aria-describedby="receipt-share-privacy"
      />
      <p id="receipt-share-privacy" class="mobile-share-note">
        Used only to open WhatsApp for this receipt; the number is not saved.
      </p>
      <p v-if="error" class="mobile-share-error" role="alert">{{ error }}</p>
      <button
        v-if="canUseBluetoothPrint"
        type="button"
        class="mobile-share-bluetooth"
        :disabled="isPrintingBt"
        @click="printWithBluetooth"
      >
        <Printer :size="18" />
        {{ isPrintingBt ? 'Printing via Bluetooth...' : 'Print (Bluetooth Thermal)' }}
      </button>
      <button
        type="button"
        class="mobile-share-bluetooth"
        @click="printWithSystemSheet"
      >
        <Printer :size="18" />
        Print (AirPrint / PDF)
      </button>

      <div class="mobile-share-divider">
        <span>OR SHARE</span>
      </div>

      <button type="button" class="mobile-share-primary" :disabled="!validPhone" @click="openWhatsApp">
        <MessageCircle :size="18" />
        Open WhatsApp
      </button>
      <button
        v-if="canUseNativeShare"
        type="button"
        class="mobile-share-secondary"
        @click="shareWithDevice"
      >
        <Share2 :size="17" />
        More sharing options
      </button>
    </section>
  </div>
</template>

<style scoped>
.mobile-share-backdrop {
  position: fixed;
  z-index: 120;
  inset: 0;
  display: grid;
  align-items: end;
  padding: 12px max(12px, env(safe-area-inset-right)) max(12px, env(safe-area-inset-bottom)) max(12px, env(safe-area-inset-left));
  background: rgba(2, 6, 23, .76);
  backdrop-filter: blur(6px);
}

@media (min-width: 640px) {
  .mobile-share-backdrop {
    align-items: center;
    padding: 20px;
  }
}

.mobile-share-dialog {
  width: min(100%, 480px);
  justify-self: center;
  padding: 22px 20px 20px;
  border: none !important;
  outline: none !important;
  border-radius: 18px;
  background: #101a2a;
  color: #e5edf7;
  box-shadow: 0 24px 70px rgba(0, 0, 0, .45);
}

.mobile-share-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 20px;
}

.mobile-share-heading h2 {
  margin: 0 0 5px;
  font-size: 19px;
  font-weight: 700;
  color: inherit;
}

.mobile-share-heading p,
.mobile-share-note {
  margin: 0;
  color: #aab8ca;
  font-size: 13px;
  line-height: 1.5;
}

.mobile-share-close {
  width: 36px;
  height: 36px;
  min-width: 36px;
  min-height: 36px;
  border-radius: 50%;
  border: none !important;
  outline: none !important;
  background: rgba(255, 255, 255, 0.08);
  color: #cbd5e1;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  padding: 0;
  transition: background 0.18s ease, transform 0.15s ease, color 0.18s ease;
  -webkit-tap-highlight-color: transparent;
  flex-shrink: 0;
}

.mobile-share-close:hover {
  background: rgba(255, 255, 255, 0.15);
  color: #ffffff;
}

.mobile-share-close:active {
  transform: scale(0.92);
}

.mobile-share-primary,
.mobile-share-secondary {
  min-width: 44px;
  min-height: 46px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 9px;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.15s ease-out;
}

.mobile-share-label {
  display: block;
  margin-bottom: 7px;
  color: #dce5ef;
  font-size: 14px;
  font-weight: 600;
}

.mobile-share-phone {
  width: 100%;
  min-height: 48px;
  padding: 0 14px;
  border: none !important;
  outline: none !important;
  border-radius: 10px;
  color: #f8fafc;
  background: rgba(15, 23, 42, 0.75);
  font: inherit;
  font-size: 15px;
  box-sizing: border-box;
  box-shadow: none !important;
}

.mobile-share-phone:focus-visible,
.mobile-share-phone:focus {
  border: none !important;
  outline: none !important;
  box-shadow: 0 0 0 2px rgba(14, 165, 233, 0.5) !important;
}

.mobile-share-close:focus-visible,
.mobile-share-primary:focus-visible,
.mobile-share-secondary:focus-visible,
.mobile-share-bluetooth:focus-visible {
  outline: none !important;
  box-shadow: 0 0 0 2px rgba(14, 165, 233, 0.5) !important;
}

.mobile-share-note {
  margin: 8px 0 16px;
  font-size: 12px;
}

.mobile-share-error {
  margin: 0 0 12px;
  color: #fca5a5;
  font-size: 13px;
}

.mobile-share-bluetooth {
  width: 100%;
  min-height: 46px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 9px;
  border-radius: 10px;
  cursor: pointer;
  padding: 0 16px;
  font: inherit;
  font-size: 14px;
  font-weight: 700;
  box-sizing: border-box;
  background: #0ea5e9;
  border: none !important;
  outline: none !important;
  color: #ffffff;
  box-shadow: none !important;
  transition: all 0.15s ease-out;
}

.mobile-share-bluetooth:hover:not(:disabled) {
  background: #0284c7;
}

.mobile-share-bluetooth:active:not(:disabled) {
  transform: scale(0.98);
}

.mobile-share-bluetooth:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.mobile-share-divider {
  display: flex;
  align-items: center;
  text-align: center;
  margin: 16px 0 12px;
  color: #64748b;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
}

.mobile-share-divider::before,
.mobile-share-divider::after {
  content: '';
  flex: 1;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
}

.mobile-share-divider span {
  padding: 0 10px;
}

.mobile-share-primary,
.mobile-share-secondary {
  width: 100%;
  padding: 0 16px;
  font: inherit;
  font-size: 14px;
  box-sizing: border-box;
}

.mobile-share-primary {
  border: none !important;
  outline: none !important;
  box-shadow: none !important;
  color: #ffffff;
  background: #10b981;
  font-weight: 700;
}

.mobile-share-primary:hover:not(:disabled) {
  background: #059669;
}

.mobile-share-primary:active:not(:disabled) {
  transform: scale(0.98);
}

.mobile-share-primary:disabled {
  opacity: .48;
  cursor: not-allowed;
}

.mobile-share-secondary {
  margin-top: 8px;
  border: none !important;
  outline: none !important;
  box-shadow: none !important;
  color: #cbd5e1;
  background: rgba(30, 41, 59, 0.72);
  font-weight: 600;
}

.mobile-share-secondary:hover {
  background: rgba(51, 65, 85, 0.8);
  color: #ffffff;
}

.mobile-share-secondary:active {
  transform: scale(0.98);
}

/* Light Mode — Crisp, elevated, high-contrast stand-out UI */
[data-theme='light'] .mobile-share-backdrop {
  background: rgba(15, 23, 42, 0.55);
  backdrop-filter: blur(8px);
}

[data-theme='light'] .mobile-share-dialog {
  background: #ffffff;
  border: none !important;
  outline: none !important;
  color: #0f172a;
  box-shadow: 0 25px 60px -10px rgba(15, 23, 42, 0.28);
}

[data-theme='light'] .mobile-share-heading h2 {
  color: #0f172a;
}

[data-theme='light'] .mobile-share-heading p {
  color: #64748b;
  font-weight: 500;
}

[data-theme='light'] .mobile-share-close {
  border: none !important;
  outline: none !important;
  color: #475569;
  background: #f1f5f9;
}

[data-theme='light'] .mobile-share-close:hover {
  background: #e2e8f0;
  color: #0f172a;
}

[data-theme='light'] .mobile-share-label {
  color: #0f172a;
  font-weight: 700;
}

[data-theme='light'] .mobile-share-phone {
  background: #f8fafc;
  border: none !important;
  outline: none !important;
  color: #0f172a;
  box-shadow: inset 0 1px 2px rgba(15, 23, 42, 0.04);
}

[data-theme='light'] .mobile-share-phone::placeholder {
  color: #94a3b8;
}

[data-theme='light'] .mobile-share-phone:focus-visible,
[data-theme='light'] .mobile-share-phone:focus {
  outline: none !important;
  border: none !important;
  box-shadow: 0 0 0 2px rgba(14, 165, 233, 0.5) !important;
}

[data-theme='light'] .mobile-share-note {
  color: #64748b;
}

[data-theme='light'] .mobile-share-error {
  color: #dc2626;
  font-weight: 600;
}

[data-theme='light'] .mobile-share-bluetooth {
  background: #0284c7;
  border: none !important;
  outline: none !important;
  color: #ffffff;
  box-shadow: none !important;
}

[data-theme='light'] .mobile-share-bluetooth:hover:not(:disabled) {
  background: #0369a1;
}

[data-theme='light'] .mobile-share-divider {
  color: #64748b;
}

[data-theme='light'] .mobile-share-divider::before,
[data-theme='light'] .mobile-share-divider::after {
  border-bottom: 1px solid #e2e8f0;
}

[data-theme='light'] .mobile-share-primary {
  background: #10b981;
  border: none !important;
  outline: none !important;
  color: #ffffff;
  box-shadow: none !important;
}

[data-theme='light'] .mobile-share-primary:hover:not(:disabled) {
  background: #059669;
}

[data-theme='light'] .mobile-share-primary:disabled {
  background: #cbd5e1;
  border: none !important;
  color: #64748b;
  box-shadow: none;
  opacity: 0.7;
}

[data-theme='light'] .mobile-share-secondary {
  background: #f1f5f9;
  border: none !important;
  outline: none !important;
  color: #1e293b;
  font-weight: 650;
  box-shadow: none !important;
}

[data-theme='light'] .mobile-share-secondary:hover {
  background: #e2e8f0;
  color: #0f172a;
}
</style>
