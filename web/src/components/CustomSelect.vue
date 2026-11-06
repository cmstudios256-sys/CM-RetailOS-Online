<template>
  <div class="dropdown-container custom-select-container" :class="{ 'w-full': fullWidth }" ref="containerRef">
    <button class="custom-select-trigger filter-select" :class="{ 'w-full': fullWidth, 'text-muted': !modelValue }" @click="toggleDropdown" type="button">
      {{ selectedLabel }}
    </button>
    <div v-if="isOpen" class="dropdown-menu custom-select-menu" :class="{ 'drop-up': dropUp }">
      <button
        v-for="option in options"
        :key="option.value"
        class="dropdown-item"
        :class="{ active: modelValue === option.value }"
        @click="selectOption(option.value)"
        type="button"
      >
        {{ option.label }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'

interface Option {
  label: string
  value: any
}

const props = defineProps<{
  modelValue: any
  options: Option[]
  fullWidth?: boolean
  placeholder?: string
  dropUp?: boolean
}>()

const emit = defineEmits(['update:modelValue'])

const isOpen = ref(false)
const containerRef = ref<HTMLElement | null>(null)

const selectedLabel = computed(() => {
  const selected = props.options.find(opt => opt.value === props.modelValue)
  if (selected) return selected.label
  if (props.modelValue) return props.modelValue
  return props.placeholder || 'Select...'
})

const toggleDropdown = () => {
  isOpen.value = !isOpen.value
}

const selectOption = (value: any) => {
  emit('update:modelValue', value)
  isOpen.value = false
}

const closeDropdowns = (e: Event) => {
  if (containerRef.value && !containerRef.value.contains(e.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => {
  document.addEventListener('click', closeDropdowns)
})

onUnmounted(() => {
  document.removeEventListener('click', closeDropdowns)
})
</script>

<style scoped>
.custom-select-container {
  position: relative;
  display: inline-block;
}
.custom-select-container.w-full {
  display: block;
  width: 100%;
}
.custom-select-trigger.filter-select,
.export-btn.filter-select {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  background-color: rgba(30, 41, 59, 0.75);
  color: #f1f5f9;
  padding: 0.55rem 2.2rem 0.55rem 1rem;
  border-radius: 10px;
  border: none !important;
  outline: none !important;
  box-shadow: none !important;
  cursor: pointer;
  font-weight: 600;
  transition: all 0.2s;
  white-space: nowrap;
  text-align: left;
  justify-content: space-between;
  min-width: 140px;
  min-height: 42px;
  appearance: none;
  background-image: var(--select-chevron-url, url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%2338bdf8' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'%3E%3C/polyline%3E%3C/svg%3E"));
  background-repeat: no-repeat;
  background-position: right 0.75rem center;
  background-size: 16px;
}
.custom-select-trigger.filter-select:hover,
.export-btn.filter-select:hover {
  background-color: rgba(51, 65, 85, 0.85);
  color: #ffffff;
}
.filter-select.w-full {
  width: 100%;
  min-width: 0;
}
.text-muted {
  color: var(--text-muted);
}
.dropdown-menu {
  position: absolute;
  top: calc(100% + 6px);
  right: 0;
  margin-top: 0;
  background-color: #141f32;
  border: none !important;
  outline: none !important;
  border-radius: 14px;
  padding: 6px;
  display: flex;
  flex-direction: column;
  gap: 3px;
  z-index: 9999;
  box-shadow: 0 20px 45px -4px rgba(0, 0, 0, 0.65), 0 0 0 1px rgba(255, 255, 255, 0.08);
  min-width: 160px;
}
.custom-select-menu {
  max-height: 250px;
  overflow-y: auto;
  min-width: 100%;
}
.custom-select-menu.drop-up {
  top: auto;
  bottom: calc(100% + 6px);
  margin-top: 0;
  margin-bottom: 0;
}
.dropdown-item {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  background-color: transparent;
  border: none !important;
  outline: none !important;
  color: #e2e8f0;
  padding: 0.65rem 0.85rem;
  min-height: 44px;
  border-radius: 10px;
  font-size: 0.92rem;
  font-weight: 550;
  cursor: pointer;
  width: 100%;
  justify-content: flex-start;
  transition: background-color 0.15s, transform 0.1s;
}
.dropdown-item:hover {
  background-color: rgba(255, 255, 255, 0.08);
  color: #ffffff;
}
.dropdown-item:active {
  background-color: rgba(255, 255, 255, 0.14);
  transform: scale(0.98);
}
.dropdown-item.active {
  background-color: #0ea5e9 !important;
  color: white !important;
  font-weight: 650;
}
.dropdown-item.active:hover {
  background-color: #0284c7 !important;
}
</style>
