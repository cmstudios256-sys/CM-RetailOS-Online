import { ref, watch, onUnmounted, type Ref } from 'vue'

export function useDelayedSkeleton(loading: Ref<boolean>, delay = 200) {
  const showSkeleton = ref(false)
  let timer: ReturnType<typeof setTimeout> | null = null

  watch(
    loading,
    (isLoading) => {
      if (isLoading) {
        if (timer) clearTimeout(timer)
        timer = setTimeout(() => {
          if (loading.value) {
            showSkeleton.value = true
          }
        }, delay)
      } else {
        if (timer) clearTimeout(timer)
        timer = null
        showSkeleton.value = false
      }
    },
    { immediate: true },
  )

  onUnmounted(() => {
    if (timer) clearTimeout(timer)
  })

  return showSkeleton
}

