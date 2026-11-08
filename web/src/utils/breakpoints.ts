import { onMounted, onUnmounted, ref } from 'vue'

export function usePhoneBreakpoint() {
  const getMatches = () => {
    if (typeof window === 'undefined') return false
    return window.innerWidth <= 639 || window.matchMedia('(max-width: 639px)').matches
  }
  const isPhone = ref(getMatches())

  const update = () => {
    isPhone.value = getMatches()
  }

  onMounted(() => {
    isPhone.value = getMatches()
    window.addEventListener('resize', update, { passive: true })
    window.addEventListener('orientationchange', update, { passive: true })
    try {
      window.matchMedia('(max-width: 639px)').addEventListener('change', update)
    } catch {}
  })

  onUnmounted(() => {
    window.removeEventListener('resize', update)
    window.removeEventListener('orientationchange', update)
    try {
      window.matchMedia('(max-width: 639px)').removeEventListener('change', update)
    } catch {}
  })

  return { isPhone }
}

export function useTabletBreakpoint() {
  const getMatches = () => {
    if (typeof window === 'undefined') return false
    const w = window.innerWidth
    return (w >= 640 && w <= 1080) || window.matchMedia('(min-width: 640px) and (max-width: 1080px)').matches
  }
  const isTablet = ref(getMatches())

  const update = () => {
    isTablet.value = getMatches()
  }

  onMounted(() => {
    isTablet.value = getMatches()
    window.addEventListener('resize', update, { passive: true })
    window.addEventListener('orientationchange', update, { passive: true })
    try {
      window.matchMedia('(min-width: 640px) and (max-width: 1080px)').addEventListener('change', update)
    } catch {}
  })

  onUnmounted(() => {
    window.removeEventListener('resize', update)
    window.removeEventListener('orientationchange', update)
    try {
      window.matchMedia('(min-width: 640px) and (max-width: 1080px)').removeEventListener('change', update)
    } catch {}
  })

  return { isTablet }
}

export function useMobileOrTabletBreakpoint() {
  const getMatches = () => {
    if (typeof window === 'undefined') return false
    return window.innerWidth <= 1080 || window.matchMedia('(max-width: 1080px)').matches
  }
  const isMobileOrTablet = ref(getMatches())

  const update = () => {
    isMobileOrTablet.value = getMatches()
  }

  onMounted(() => {
    isMobileOrTablet.value = getMatches()
    window.addEventListener('resize', update, { passive: true })
    window.addEventListener('orientationchange', update, { passive: true })
    try {
      window.matchMedia('(max-width: 1080px)').addEventListener('change', update)
    } catch {}
  })

  onUnmounted(() => {
    window.removeEventListener('resize', update)
    window.removeEventListener('orientationchange', update)
    try {
      window.matchMedia('(max-width: 1080px)').removeEventListener('change', update)
    } catch {}
  })

  return { isMobileOrTablet }
}

export function useDesktopBreakpoint() {
  const getMatches = () => {
    if (typeof window === 'undefined') return false
    return window.innerWidth > 1080 || window.matchMedia('(min-width: 1081px)').matches
  }
  const isDesktop = ref(getMatches())

  const update = () => {
    isDesktop.value = getMatches()
  }

  onMounted(() => {
    isDesktop.value = getMatches()
    window.addEventListener('resize', update, { passive: true })
    window.addEventListener('orientationchange', update, { passive: true })
    try {
      window.matchMedia('(min-width: 1081px)').addEventListener('change', update)
    } catch {}
  })

  onUnmounted(() => {
    window.removeEventListener('resize', update)
    window.removeEventListener('orientationchange', update)
    try {
      window.matchMedia('(min-width: 1081px)').removeEventListener('change', update)
    } catch {}
  })

  return { isDesktop }
}
