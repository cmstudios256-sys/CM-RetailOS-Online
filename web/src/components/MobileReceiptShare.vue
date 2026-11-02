<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { MessageCircle, Share2, X } from '@lucide/vue'
import { formatDate, formatMoney, formatQuantity, formatTime, type Sale } from '../api'
import { receiptNumber } from '../utils/reporting'

const props = defineProps<{
  sale: Sale | null
  shopName: string
  currency: string
}>()
const emit = defineEmits<{ close: [] }>()
const phone = ref('')
const error = ref('')

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
})

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
}

.mobile-share-dialog {
  width: min(100%, 480px);
  justify-self: center;
  padding: 20px;
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
  border-radius: 9px;
  cursor: pointer;
}

.mobile-share-close {
  border: 1px solid rgba(148, 163, 184, .2);
  color: #cbd5e1;
  background: transparent;
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
  padding: 0 12px;
  border: 1px solid rgba(148, 163, 184, .25);
  border-radius: 9px;
  color: #f8fafc;
  background: #0b1220;
  font: inherit;
  font-size: 16px;
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

.mobile-share-primary,
.mobile-share-secondary {
  width: 100%;
  padding: 0 14px;
  font: inherit;
  font-size: 14px;
  font-weight: 650;
}

.mobile-share-primary {
  border: 1px solid #0f9f70;
  color: #f0fdf4;
  background: #087c5a;
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
</style>
