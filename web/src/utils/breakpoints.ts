import { onMounted, onUnmounted, ref } from 'vue'

export function usePhoneBreakpoint() {
  const mediaQuery = window.matchMedia('(max-width: 639px)')
  const isPhone = ref(mediaQuery.matches)

  const update = (event: MediaQueryListEvent | MediaQueryList) => {
    isPhone.value = event.matches
  }

  onMounted(() => mediaQuery.addEventListener('change', update))
  onUnmounted(() => mediaQuery.removeEventListener('change', update))

  return { isPhone }
}

export function useDesktopBreakpoint() {
  const mediaQuery = window.matchMedia('(min-width: 1024px)')
  const isDesktop = ref(mediaQuery.matches)

  const update = (event: MediaQueryListEvent | MediaQueryList) => {
    isDesktop.value = event.matches
  }

  onMounted(() => mediaQuery.addEventListener('change', update))
  onUnmounted(() => mediaQuery.removeEventListener('change', update))

  return { isDesktop }
}
