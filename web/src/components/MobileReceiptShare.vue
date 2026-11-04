<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { MessageCircle, Share2, X, Printer, Bluetooth } from '@lucide/vue'
import { formatDate, formatMoney, formatQuantity, formatTime, type Sale } from '../api'
import { receiptNumber } from '../utils/reporting'
import {
  getDefaultBluetoothPrinter,
  buildEscPosReceipt,
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
const btSuccessMsg = ref('')

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

const normalizedPhone = computed(() => phone.value.replace(/\D/g, ''))
const validPhone = computed(() => normalizedPhone.value.length >= 8 && normalizedPhone.value.length <= 15)
const whatsappUrl = computed(() =>
  validPhone.value
    ? `https://wa.me/${normalizedPhone.value}?text=${encodeURIComponent(receiptText.value)}`
    : '',
)
const canUseNativeShare = computed(() =>
  typeof navigator !== 'undefined' && typeof navigator.share === 'function',
)

watch(() => props.sale, () => {
  phone.value = ''
  error.value = ''
  btSuccessMsg.value = ''
})

async function printWithBluetooth() {
  if (!props.sale) return
  error.value = ''
  btSuccessMsg.value = ''
  const printer = getDefaultBluetoothPrinter()
  if (!printer) {
    error.value = 'No Bluetooth thermal printer configured. Connect your 58mm/80mm printer in More > Settings > Receipt & Printing.'
    return
  }
  isPrintingBt.value = true
  try {
    const bytes = buildEscPosReceipt(props.sale, {
      shopName: props.shopName,
      currency: props.currency,
      paperWidth: printer.paperWidth || '58mm',
    })
    const res = await sendToBluetoothPrinter(printer, bytes)
    btSuccessMsg.value = res.message || `Receipt sent to ${printer.name}!`
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
    error.value = 'Enter a valid phone number with its country code.'
    return
  }
  const shareWindow = window.open(whatsappUrl.value, '_blank')
  if (shareWindow) shareWindow.opener = null
  else window.location.assign(whatsappUrl.value)
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
      <p v-if="btSuccessMsg" class="mobile-share-success" role="status">{{ btSuccessMsg }}</p>

      <button
        type="button"
        class="mobile-share-bluetooth"
        :disabled="isPrintingBt"
        @click="printWithBluetooth"
      >
        <Printer :size="18" />
        {{ isPrintingBt ? 'Printing via Bluetooth...' : 'Print (Bluetooth Thermal)' }}
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
  border: 1px solid rgba(148, 163, 184, .2);
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

.mobile-share-close,
.mobile-share-primary,
.mobile-share-secondary {
  min-width: 44px;
  min-height: 44px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 9px;
  border-radius: 10px;
  cursor: pointer;
  transition: all 0.15s ease-out;
}

.mobile-share-close {
  border: 1px solid rgba(148, 163, 184, .2);
  color: #cbd5e1;
  background: transparent;
}

.mobile-share-close:hover {
  background: rgba(148, 163, 184, .1);
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
  border: 1.5px solid rgba(148, 163, 184, .25);
  border-radius: 10px;
  color: #f8fafc;
  background: #0b1220;
  font: inherit;
  font-size: 16px;
  box-sizing: border-box;
}

.mobile-share-phone:focus-visible,
.mobile-share-close:focus-visible,
.mobile-share-primary:focus-visible,
.mobile-share-secondary:focus-visible {
  outline: 2px solid #38bdf8;
  outline-offset: 2px;
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

.mobile-share-success {
  margin: 0 0 12px;
  padding: 8px 12px;
  background: rgba(34, 197, 94, 0.15);
  border: 1px solid rgba(34, 197, 94, 0.3);
  border-radius: 8px;
  color: #86efac;
  font-size: 13px;
  font-weight: 600;
  text-align: center;
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
  background: #0284c7;
  border: 1px solid #38bdf8;
  color: #ffffff;
  box-shadow: 0 4px 14px rgba(2, 132, 199, 0.3);
  transition: all 0.15s ease-out;
}

.mobile-share-bluetooth:hover:not(:disabled) {
  background: #0369a1;
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
  border-bottom: 1px solid rgba(148, 163, 184, 0.2);
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
  font-weight: 650;
  box-sizing: border-box;
}

.mobile-share-primary {
  border: 1px solid #0f9f70;
  color: #f0fdf4;
  background: #087c5a;
}

.mobile-share-primary:hover:not(:disabled) {
  background: #09946c;
}

.mobile-share-primary:disabled {
  opacity: .48;
  cursor: not-allowed;
}

.mobile-share-secondary {
  margin-top: 8px;
  border: 1px solid rgba(148, 163, 184, .2);
  color: #dbe5f0;
  background: rgba(148, 163, 184, .06);
}

.mobile-share-secondary:hover {
  background: rgba(148, 163, 184, .12);
}

/* Light Mode — Crisp, elevated, high-contrast stand-out UI */
[data-theme='light'] .mobile-share-backdrop {
  background: rgba(15, 23, 42, 0.55);
  backdrop-filter: blur(8px);
}

[data-theme='light'] .mobile-share-dialog {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  color: #0f172a;
  box-shadow: 0 25px 60px -10px rgba(15, 23, 42, 0.28), 0 0 0 1px rgba(15, 23, 42, 0.05);
}

[data-theme='light'] .mobile-share-heading h2 {
  color: #0f172a;
}

[data-theme='light'] .mobile-share-heading p {
  color: #64748b;
  font-weight: 500;
}

[data-theme='light'] .mobile-share-close {
  border: 1px solid #cbd5e1;
  color: #475569;
  background: #f8fafc;
}

[data-theme='light'] .mobile-share-close:hover {
  background: #f1f5f9;
  color: #0f172a;
  border-color: #94a3b8;
}

[data-theme='light'] .mobile-share-label {
  color: #0f172a;
  font-weight: 700;
}

[data-theme='light'] .mobile-share-phone {
  background: #ffffff;
  border: 1.5px solid #cbd5e1;
  color: #0f172a;
  box-shadow: inset 0 1px 2px rgba(15, 23, 42, 0.04);
}

[data-theme='light'] .mobile-share-phone::placeholder {
  color: #94a3b8;
}

[data-theme='light'] .mobile-share-phone:focus-visible {
  outline: 2px solid #16a34a;
  border-color: #16a34a;
}

[data-theme='light'] .mobile-share-note {
  color: #64748b;
}

[data-theme='light'] .mobile-share-error {
  color: #dc2626;
  font-weight: 600;
}

[data-theme='light'] .mobile-share-success {
  background: #f0fdf4;
  border-color: #86efac;
  color: #166534;
}

[data-theme='light'] .mobile-share-bluetooth {
  background: linear-gradient(135deg, #0284c7 0%, #0369a1 100%);
  border: 1px solid #0284c7;
  color: #ffffff;
  box-shadow: 0 4px 14px rgba(2, 132, 199, 0.35);
}

[data-theme='light'] .mobile-share-bluetooth:hover:not(:disabled) {
  background: linear-gradient(135deg, #0369a1 0%, #075985 100%);
  box-shadow: 0 6px 18px rgba(2, 132, 199, 0.45);
}

[data-theme='light'] .mobile-share-divider {
  color: #64748b;
}

[data-theme='light'] .mobile-share-divider::before,
[data-theme='light'] .mobile-share-divider::after {
  border-bottom: 1px solid #e2e8f0;
}

[data-theme='light'] .mobile-share-primary {
  background: linear-gradient(135deg, #22c55e 0%, #16a34a 100%);
  border: 1px solid #15803d;
  color: #ffffff;
  box-shadow: 0 4px 14px rgba(22, 163, 74, 0.35);
  text-shadow: 0 1px 2px rgba(0, 0, 0, 0.15);
}

[data-theme='light'] .mobile-share-primary:hover:not(:disabled) {
  background: linear-gradient(135deg, #16a34a 0%, #15803d 100%);
  box-shadow: 0 6px 18px rgba(22, 163, 74, 0.45);
}

[data-theme='light'] .mobile-share-primary:disabled {
  background: #cbd5e1;
  border-color: #cbd5e1;
  color: #64748b;
  box-shadow: none;
  opacity: 0.7;
}

[data-theme='light'] .mobile-share-secondary {
  background: #f8fafc;
  border: 1.5px solid #cbd5e1;
  color: #1e293b;
  font-weight: 650;
  box-shadow: 0 1px 3px rgba(15, 23, 42, 0.05);
}

[data-theme='light'] .mobile-share-secondary:hover {
  background: #f1f5f9;
  border-color: #94a3b8;
  color: #0f172a;
}
</style>
