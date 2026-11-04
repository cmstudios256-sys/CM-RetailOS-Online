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
  background-color: color-mix(in srgb, var(--accent-color) 8%, var(--bg-surface));
  color: var(--accent-color);
  padding: 0.5rem 2rem 0.5rem 1rem;
  border-radius: 0.375rem;
  border: 1px solid color-mix(in srgb, var(--accent-color) 30%, var(--border-color));
  cursor: pointer;
  font-weight: 500;
  transition: all 0.2s;
  white-space: nowrap;
  text-align: left;
  justify-content: space-between;
  min-width: 140px;
  min-height: 36px;
  appearance: none;
  background-image: var(--select-chevron-url, url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='%230284c7' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'%3E%3C/polyline%3E%3C/svg%3E"));
  background-repeat: no-repeat;
  background-position: right 0.5rem center;
  background-size: 16px;
}
.custom-select-trigger.filter-select:hover,
.export-btn.filter-select:hover {
  background-color: color-mix(in srgb, var(--accent-color) 15%, var(--bg-surface));
  border-color: var(--accent-color);
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
  top: 100%;
  right: 0;
  margin-top: 0.5rem;
  background-color: var(--bg-surface);
  border: 1px solid var(--border-color);
  border-radius: 8px;
  padding: 0.5rem;
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  z-index: 50;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.5);
  min-width: 160px;
}
.custom-select-menu {
  max-height: 160px;
  overflow-y: auto;
  min-width: 100%;
}
.custom-select-menu.drop-up {
  top: auto;
  bottom: 100%;
  margin-top: 0;
  margin-bottom: 0.5rem;
}
.dropdown-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  background-color: transparent;
  border: none;
  color: var(--text-primary);
  padding: 0.5rem 0.75rem;
  border-radius: 0.375rem;
  font-size: 0.875rem;
  cursor: pointer;
  width: 100%;
  justify-content: flex-start;
  transition: background-color 0.2s;
}
.dropdown-item:hover {
  background-color: var(--border-color);
}
.dropdown-item.active {
  background-color: var(--accent-color);
  color: white;
}
.dropdown-item.active:hover {
  background-color: var(--accent-hover);
}
</style>
