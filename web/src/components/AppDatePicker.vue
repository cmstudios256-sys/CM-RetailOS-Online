<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { Calendar as CalendarIcon, ChevronLeft, ChevronRight, X, Check, RotateCcw } from '@lucide/vue'

const props = withDefaults(
  defineProps<{
    modelValue: string
    placeholder?: string
    title?: string
    compact?: boolean
    ariaLabel?: string
  }>(),
  {
    placeholder: 'Filter by Date',
    title: 'Filter by Date',
    compact: false,
    ariaLabel: 'Filter by date',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

const isOpen = ref(false)

// Year and Month currently viewed in the calendar
const now = new Date()
const viewYear = ref(now.getFullYear())
const viewMonth = ref(now.getMonth()) // 0 - 11
const tempSelected = ref(props.modelValue || '')

// Month names
const MONTH_NAMES = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December',
]
const DAY_LABELS = ['Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa', 'Su']

function pad(n: number): string {
  return n < 10 ? `0${n}` : String(n)
}

function toYMD(year: number, month: number, day: number): string {
  return `${year}-${pad(month + 1)}-${pad(day)}`
}

function getTodayYMD(): string {
  const d = new Date()
  return toYMD(d.getFullYear(), d.getMonth(), d.getDate())
}

function getYesterdayYMD(): string {
  const d = new Date()
  d.setDate(d.getDate() - 1)
  return toYMD(d.getFullYear(), d.getMonth(), d.getDate())
}

function get7DaysAgoYMD(): string {
  const d = new Date()
  d.setDate(d.getDate() - 7)
  return toYMD(d.getFullYear(), d.getMonth(), d.getDate())
}

const todayYMD = computed(() => getTodayYMD())

// Sync calendar view when opened or modelValue changes
watch(
  () => props.modelValue,
  val => {
    tempSelected.value = val || ''
    if (val) {
      const parts = val.split('-').map(Number)
      if (parts.length === 3 && !isNaN(parts[0]) && !isNaN(parts[1])) {
        viewYear.value = parts[0]
        viewMonth.value = parts[1] - 1
      }
    }
  },
  { immediate: true },
)

function openModal() {
  tempSelected.value = props.modelValue || ''
  if (props.modelValue) {
    const parts = props.modelValue.split('-').map(Number)
    if (parts.length === 3 && !isNaN(parts[0]) && !isNaN(parts[1])) {
      viewYear.value = parts[0]
      viewMonth.value = parts[1] - 1
    }
  } else {
    const d = new Date()
    viewYear.value = d.getFullYear()
    viewMonth.value = d.getMonth()
  }
  isOpen.value = true
}

function closeModal() {
  isOpen.value = false
}

function prevMonth() {
  if (viewMonth.value === 0) {
    viewMonth.value = 11
    viewYear.value--
  } else {
    viewMonth.value--
  }
}

function nextMonth() {
  if (viewMonth.value === 11) {
    viewMonth.value = 0
    viewYear.value++
  } else {
    viewMonth.value++
  }
}

interface CalendarCell {
  day: number
  month: number
  year: number
  ymd: string
  isCurrentMonth: boolean
  isToday: boolean
  isSelected: boolean
}

const calendarDays = computed(() => {
  const year = viewYear.value
  const month = viewMonth.value

  const daysInMonth = new Date(year, month + 1, 0).getDate()
  const daysInPrevMonth = new Date(year, month, 0).getDate()

  // First day of current month: 0 (Sun) .. 6 (Sat)
  // Convert so Monday = 0, Sunday = 6
  const rawFirstDay = new Date(year, month, 1).getDay()
  const firstDay = (rawFirstDay + 6) % 7

  const cells: CalendarCell[] = []

  // Trailing days from previous month
  const prevMonthIdx = month === 0 ? 11 : month - 1
  const prevYearIdx = month === 0 ? year - 1 : year
  for (let i = firstDay - 1; i >= 0; i--) {
    const d = daysInPrevMonth - i
    const ymd = toYMD(prevYearIdx, prevMonthIdx, d)
    cells.push({
      day: d,
      month: prevMonthIdx,
      year: prevYearIdx,
      ymd,
      isCurrentMonth: false,
      isToday: ymd === todayYMD.value,
      isSelected: ymd === tempSelected.value,
    })
  }

  // Days of current month
  for (let d = 1; d <= daysInMonth; d++) {
    const ymd = toYMD(year, month, d)
    cells.push({
      day: d,
      month,
      year,
      ymd,
      isCurrentMonth: true,
      isToday: ymd === todayYMD.value,
      isSelected: ymd === tempSelected.value,
    })
  }

  // Leading days of next month to fill complete weeks (up to 35 or 42 cells)
  const remaining = (7 - (cells.length % 7)) % 7
  const totalNeeded = cells.length + remaining < 35 ? 35 : cells.length + remaining
  const nextMonthIdx = month === 11 ? 0 : month + 1
  const nextYearIdx = month === 11 ? year + 1 : year

  let nextDay = 1
  while (cells.length < totalNeeded) {
    const ymd = toYMD(nextYearIdx, nextMonthIdx, nextDay)
    cells.push({
      day: nextDay,
      month: nextMonthIdx,
      year: nextYearIdx,
      ymd,
      isCurrentMonth: false,
      isToday: ymd === todayYMD.value,
      isSelected: ymd === tempSelected.value,
    })
    nextDay++
  }

  return cells
})

function selectCell(cell: CalendarCell) {
  tempSelected.value = cell.ymd
  if (!cell.isCurrentMonth) {
    viewMonth.value = cell.month
    viewYear.value = cell.year
  }
}

function selectPreset(ymd: string) {
  tempSelected.value = ymd
  if (ymd) {
    const parts = ymd.split('-').map(Number)
    viewYear.value = parts[0]
    viewMonth.value = parts[1] - 1
  }
}

function applyDate() {
  emit('update:modelValue', tempSelected.value)
  closeModal()
}

function clearDateDirectly(event?: Event) {
  if (event) event.stopPropagation()
  tempSelected.value = ''
  emit('update:modelValue', '')
  closeModal()
}

// Formatted display date for trigger and header
const formattedDisplay = computed(() => {
  if (!props.modelValue) return ''
  const parts = props.modelValue.split('-').map(Number)
  if (parts.length !== 3 || isNaN(parts[0]) || isNaN(parts[1]) || isNaN(parts[2])) {
    return props.modelValue
  }
  const d = new Date(parts[0], parts[1] - 1, parts[2])
  return d.toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
})

const formattedTempDisplay = computed(() => {
  if (!tempSelected.value) return 'No date selected'
  const parts = tempSelected.value.split('-').map(Number)
  if (parts.length !== 3) return tempSelected.value
  const d = new Date(parts[0], parts[1] - 1, parts[2])
  return d.toLocaleDateString(undefined, {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
})

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && isOpen.value) {
    closeModal()
  }
}

onMounted(() => {
  if (typeof window !== 'undefined') {
    window.addEventListener('keydown', handleKeydown)
  }
})

onUnmounted(() => {
  if (typeof window !== 'undefined') {
    window.removeEventListener('keydown', handleKeydown)
  }
})
</script>

<template>
  <div class="date-filter-wrapper app-date-picker-root">
    <!-- Trigger Button -->
    <button
      type="button"
      class="filter-btn date-picker-btn"
      :class="{
        'active-filter': Boolean(modelValue),
        'is-compact': compact,
      }"
      :title="formattedDisplay || placeholder"
      :aria-label="ariaLabel"
      @click="openModal"
    >
      <CalendarIcon :size="16" class="date-picker-icon" />
      <span class="date-picker-label">{{ formattedDisplay || placeholder }}</span>
    </button>

    <!-- Quick clear button if a date is currently active -->
    <button
      v-if="modelValue"
      type="button"
      class="clear-date-btn"
      title="Clear Date Filter"
      :aria-label="'Clear ' + ariaLabel"
      @click="clearDateDirectly"
    >
      <X :size="14" />
    </button>

    <!-- Native-styled Mobile Modal / Bottom Sheet -->
    <Teleport to="body">
      <Transition name="mobile-datepicker-fade">
        <div
          v-if="isOpen"
          class="mobile-datepicker-backdrop"
          @click.self="closeModal"
        >
          <div
            class="mobile-datepicker-sheet"
            role="dialog"
            aria-modal="true"
            :aria-label="title"
          >
            <!-- Native drag handle indicator for mobile sheet -->
            <div class="mobile-datepicker-handle-bar">
              <span class="mobile-datepicker-handle"></span>
            </div>

            <!-- Header -->
            <header class="mobile-datepicker-header">
              <div class="mobile-datepicker-titles">
                <h3 class="mobile-datepicker-title">{{ title }}</h3>
                <p class="mobile-datepicker-subtitle">{{ formattedTempDisplay }}</p>
              </div>
              <button
                type="button"
                class="mobile-datepicker-close-btn"
                aria-label="Close date picker"
                @click="closeModal"
              >
                <X :size="18" />
              </button>
            </header>

            <!-- Quick Presets -->
            <div class="mobile-datepicker-presets">
              <button
                type="button"
                class="preset-chip"
                :class="{ active: tempSelected === todayYMD }"
                @click="selectPreset(todayYMD)"
              >
                Today
              </button>
              <button
                type="button"
                class="preset-chip"
                :class="{ active: tempSelected === getYesterdayYMD() }"
                @click="selectPreset(getYesterdayYMD())"
              >
                Yesterday
              </button>
              <button
                type="button"
                class="preset-chip"
                :class="{ active: tempSelected === get7DaysAgoYMD() }"
                @click="selectPreset(get7DaysAgoYMD())"
              >
                7 Days Ago
              </button>
              <button
                v-if="tempSelected"
                type="button"
                class="preset-chip clear-chip"
                @click="tempSelected = ''"
              >
                <RotateCcw :size="12" />
                Clear
              </button>
            </div>

            <!-- Calendar Card -->
            <div class="mobile-datepicker-calendar-card">
              <!-- Month / Year Navigator -->
              <div class="calendar-month-nav">
                <button
                  type="button"
                  class="month-nav-btn"
                  title="Previous month"
                  aria-label="Previous month"
                  @click="prevMonth"
                >
                  <ChevronLeft :size="18" />
                </button>
                <div class="current-month-label">
                  <strong>{{ MONTH_NAMES[viewMonth] }}</strong>
                  <span>{{ viewYear }}</span>
                </div>
                <button
                  type="button"
                  class="month-nav-btn"
                  title="Next month"
                  aria-label="Next month"
                  @click="nextMonth"
                >
                  <ChevronRight :size="18" />
                </button>
              </div>

              <!-- Day of Week Headers -->
              <div class="calendar-weekdays">
                <span v-for="d in DAY_LABELS" :key="d" class="weekday-cell">{{ d }}</span>
              </div>

              <!-- Days Grid -->
              <div class="calendar-days-grid">
                <button
                  v-for="(cell, idx) in calendarDays"
                  :key="idx"
                  type="button"
                  class="calendar-day-btn"
                  :class="{
                    'not-current-month': !cell.isCurrentMonth,
                    'is-today': cell.isToday,
                    'is-selected': cell.isSelected,
                  }"
                  @click="selectCell(cell)"
                >
                  <span class="day-number">{{ cell.day }}</span>
                  <i v-if="cell.isToday && !cell.isSelected" class="today-dot"></i>
                </button>
              </div>
            </div>

            <!-- Action Buttons Footer -->
            <footer class="mobile-datepicker-footer">
              <button
                type="button"
                class="mobile-datepicker-btn btn-secondary"
                @click="clearDateDirectly"
              >
                Reset Filter
              </button>
              <button
                type="button"
                class="mobile-datepicker-btn btn-primary"
                @click="applyDate"
              >
                <Check :size="16" />
                Apply Date
              </button>
            </footer>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.app-date-picker-root {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  position: relative;
}

.date-picker-btn {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  height: 38px;
  padding: 0 12px;
  border-radius: 9px;
  border: none !important;
  outline: none !important;
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-secondary, #cbd5e1);
  font-size: 13px;
  font-weight: 550;
  cursor: pointer;
  white-space: nowrap;
  transition: all 0.18s ease;
  box-sizing: border-box;
}

.date-picker-btn:hover {
  background: rgba(255, 255, 255, 0.14);
  color: #ffffff;
}

.date-picker-btn.active-filter {
  background: color-mix(in srgb, var(--accent-color, #0ea5e9) 18%, transparent) !important;
  color: var(--accent-color, #38bdf8) !important;
  border: none !important;
}

.clear-date-btn {
  width: 34px;
  height: 34px;
  border-radius: 9px;
  border: none !important;
  outline: none !important;
  background: rgba(239, 68, 68, 0.12);
  color: #ef4444;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  padding: 0;
  transition: all 0.16s ease;
  flex-shrink: 0;
}

.clear-date-btn:hover {
  background: rgba(239, 68, 68, 0.22);
  color: #f87171;
}

.is-compact {
  padding: 0;
  width: 38px;
  min-width: 38px;
  justify-content: center;
}

.is-compact .date-picker-label {
  display: none;
}

@media (max-width: 639px) {
  .date-picker-btn {
    width: 40px;
    min-width: 40px;
    height: 40px;
    padding: 0;
    justify-content: center;
  }
  .date-picker-label {
    display: none;
  }
}

/* Modal Backdrop */
.mobile-datepicker-backdrop {
  position: fixed;
  inset: 0;
  z-index: 99999;
  background: rgba(4, 9, 20, 0.72);
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  display: flex;
  align-items: flex-end;
  justify-content: center;
  padding: 0;
  overflow: hidden;
}

@media (min-width: 641px) {
  .mobile-datepicker-backdrop {
    align-items: center;
    padding: 20px;
  }
}

/* Sheet Box */
.mobile-datepicker-sheet {
  width: 100%;
  max-width: 420px;
  background: var(--bg-card, #101a2a);
  color: var(--text-primary, #e2e8f0);
  border-radius: 24px 24px 0 0;
  border: none !important;
  outline: none !important;
  box-shadow: 0 -12px 48px rgba(0, 0, 0, 0.6);
  padding: 12px 20px 24px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  animation: slideUp 0.24s cubic-bezier(0.16, 1, 0.3, 1);
  box-sizing: border-box;
}

@media (min-width: 641px) {
  .mobile-datepicker-sheet {
    border-radius: 20px;
    box-shadow: 0 24px 60px rgba(0, 0, 0, 0.55);
    padding: 20px 22px 22px;
    animation: scaleIn 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  }
}

/* Native Drag Handle for Mobile */
.mobile-datepicker-handle-bar {
  display: flex;
  justify-content: center;
  padding-bottom: 4px;
}

.mobile-datepicker-handle {
  width: 36px;
  height: 4px;
  border-radius: 2px;
  background: rgba(255, 255, 255, 0.22);
}

@media (min-width: 641px) {
  .mobile-datepicker-handle-bar {
    display: none;
  }
}

/* Header */
.mobile-datepicker-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.mobile-datepicker-titles {
  min-width: 0;
  flex: 1;
}

.mobile-datepicker-title {
  margin: 0;
  font-size: 17px;
  font-weight: 700;
  color: #f8fafc;
  letter-spacing: -0.01em;
}

.mobile-datepicker-subtitle {
  margin: 3px 0 0;
  font-size: 13px;
  color: var(--accent-color, #38bdf8);
  font-weight: 600;
}

.mobile-datepicker-close-btn {
  width: 34px;
  height: 34px;
  min-width: 34px;
  border-radius: 50%;
  border: none !important;
  outline: none !important;
  background: rgba(255, 255, 255, 0.08);
  color: #94a3b8;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.15s ease;
  flex-shrink: 0;
}

.mobile-datepicker-close-btn:hover {
  background: rgba(255, 255, 255, 0.16);
  color: #ffffff;
  transform: scale(1.05);
}

/* Quick Preset Chips */
.mobile-datepicker-presets {
  display: flex;
  align-items: center;
  gap: 8px;
  overflow-x: auto;
  padding: 2px 0 4px;
  scrollbar-width: none;
}

.mobile-datepicker-presets::-webkit-scrollbar {
  display: none;
}

.preset-chip {
  padding: 6px 13px;
  border-radius: 9999px;
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
  border: none !important;
  outline: none !important;
  background: rgba(255, 255, 255, 0.08);
  color: #cbd5e1;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 5px;
  transition: all 0.16s ease;
  -webkit-tap-highlight-color: transparent;
}

.preset-chip:hover {
  background: rgba(255, 255, 255, 0.14);
  color: #ffffff;
}

.preset-chip.active {
  background: var(--accent-color, #0ea5e9);
  color: #ffffff;
  box-shadow: 0 4px 14px color-mix(in srgb, var(--accent-color, #0ea5e9) 40%, transparent);
}

.preset-chip.clear-chip {
  background: rgba(239, 68, 68, 0.15);
  color: #fca5a5;
  margin-left: auto;
}

.preset-chip.clear-chip:hover {
  background: rgba(239, 68, 68, 0.25);
  color: #ffffff;
}

/* Calendar Card */
.mobile-datepicker-calendar-card {
  background: rgba(15, 23, 42, 0.55);
  border-radius: 18px;
  padding: 14px 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

/* Month & Year Navigator */
.calendar-month-nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 6px 4px;
}

.current-month-label {
  display: flex;
  align-items: baseline;
  gap: 6px;
  font-size: 15px;
  color: #f1f5f9;
}

.current-month-label strong {
  font-weight: 700;
}

.current-month-label span {
  font-size: 13px;
  color: #94a3b8;
  font-weight: 500;
}

.month-nav-btn {
  width: 34px;
  height: 34px;
  border-radius: 10px;
  border: none !important;
  outline: none !important;
  background: rgba(255, 255, 255, 0.08);
  color: #cbd5e1;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.15s ease;
}

.month-nav-btn:hover {
  background: rgba(255, 255, 255, 0.18);
  color: #ffffff;
  transform: scale(1.05);
}

/* Weekday Labels */
.calendar-weekdays {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  text-align: center;
  padding: 2px 0 6px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.06);
}

.weekday-cell {
  font-size: 11px;
  font-weight: 650;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

/* Days Grid */
.calendar-days-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  gap: 4px;
}

.calendar-day-btn {
  aspect-ratio: 1;
  min-height: 38px;
  border-radius: 12px;
  border: none !important;
  outline: none !important;
  background: transparent;
  color: #e2e8f0;
  font-size: 13px;
  font-weight: 500;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  position: relative;
  cursor: pointer;
  transition: all 0.14s ease;
  padding: 0;
  -webkit-tap-highlight-color: transparent;
}

.calendar-day-btn:hover:not(.is-selected) {
  background: rgba(255, 255, 255, 0.1);
  color: #ffffff;
}

.calendar-day-btn.not-current-month {
  color: #475569;
  opacity: 0.55;
}

.calendar-day-btn.is-today:not(.is-selected) {
  font-weight: 700;
  color: var(--accent-color, #38bdf8);
}

.today-dot {
  position: absolute;
  bottom: 4px;
  width: 4px;
  height: 4px;
  border-radius: 50%;
  background: var(--accent-color, #38bdf8);
}

.calendar-day-btn.is-selected {
  background: var(--accent-color, #0ea5e9) !important;
  color: #ffffff !important;
  font-weight: 750;
  box-shadow: 0 4px 16px color-mix(in srgb, var(--accent-color, #0ea5e9) 45%, transparent);
  transform: scale(1.04);
}

/* Footer Buttons */
.mobile-datepicker-footer {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 4px;
}

.mobile-datepicker-btn {
  flex: 1;
  height: 44px;
  border-radius: 14px;
  border: none !important;
  outline: none !important;
  font-size: 14px;
  font-weight: 650;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 7px;
  transition: all 0.16s ease;
  -webkit-tap-highlight-color: transparent;
}

.btn-secondary {
  background: rgba(255, 255, 255, 0.08);
  color: #cbd5e1;
}

.btn-secondary:hover {
  background: rgba(255, 255, 255, 0.14);
  color: #ffffff;
}

.btn-primary {
  background: var(--accent-color, #0ea5e9);
  color: #ffffff;
  box-shadow: 0 4px 16px color-mix(in srgb, var(--accent-color, #0ea5e9) 35%, transparent);
}

.btn-primary:hover {
  filter: brightness(1.1);
  transform: translateY(-1px);
}

.btn-primary:active,
.btn-secondary:active {
  transform: scale(0.98);
}

/* Transitions */
.mobile-datepicker-fade-enter-active,
.mobile-datepicker-fade-leave-active {
  transition: opacity 0.2s ease;
}

.mobile-datepicker-fade-enter-from,
.mobile-datepicker-fade-leave-to {
  opacity: 0;
}

@keyframes slideUp {
  from {
    transform: translateY(100%);
  }
  to {
    transform: translateY(0);
  }
}

@keyframes scaleIn {
  from {
    transform: scale(0.95);
    opacity: 0;
  }
  to {
    transform: scale(1);
    opacity: 1;
  }
}

/* Light Theme overrides */
[data-theme='light'] .mobile-datepicker-sheet {
  background: #ffffff;
  color: #0f172a;
  box-shadow: 0 -12px 48px rgba(0, 0, 0, 0.18);
}

[data-theme='light'] .mobile-datepicker-handle {
  background: rgba(15, 23, 42, 0.2);
}

[data-theme='light'] .mobile-datepicker-title {
  color: #0f172a;
}

[data-theme='light'] .mobile-datepicker-close-btn,
[data-theme='light'] .btn-secondary,
[data-theme='light'] .month-nav-btn {
  background: rgba(15, 23, 42, 0.06);
  color: #475569;
}

[data-theme='light'] .mobile-datepicker-close-btn:hover,
[data-theme='light'] .btn-secondary:hover,
[data-theme='light'] .month-nav-btn:hover {
  background: rgba(15, 23, 42, 0.12);
  color: #0f172a;
}

[data-theme='light'] .mobile-datepicker-calendar-card {
  background: #f8fafc;
}

[data-theme='light'] .current-month-label {
  color: #0f172a;
}

[data-theme='light'] .calendar-day-btn {
  color: #1e293b;
}

[data-theme='light'] .calendar-day-btn:hover:not(.is-selected) {
  background: rgba(15, 23, 42, 0.08);
}

[data-theme='light'] .preset-chip {
  background: rgba(15, 23, 42, 0.06);
  color: #334155;
}

[data-theme='light'] .preset-chip:hover {
  background: rgba(15, 23, 42, 0.12);
  color: #0f172a;
}

[data-theme='light'] .date-picker-btn {
  background: #ffffff !important;
  color: #334155 !important;
  border: 1px solid #e2e8f0 !important;
}

[data-theme='light'] .date-picker-btn:hover {
  background: #f8fafc !important;
  color: #0f172a !important;
}
</style>
