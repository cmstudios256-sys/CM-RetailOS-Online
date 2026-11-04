import { ref, computed, type Ref } from 'vue'

export function useBottomSheetDrag(onClose: () => void) {
  const sheetRef = ref<HTMLElement | null>(null)
  const translateY = ref(0)
  const isDragging = ref(false)
  const isAnimating = ref(false)

  let startY = 0
  let startTime = 0
  let activePointerId: number | null = null

  function onPointerDown(e: PointerEvent) {
    // Only accept primary button for mouse
    if (e.pointerType === 'mouse' && e.button !== 0) return

    // Do not initiate drag if tapping on interactive buttons or inputs (e.g. close button, clear cart)
    const target = e.target as HTMLElement
    if (target.closest('button, input, select, textarea, a')) return

    startY = e.clientY
    startTime = Date.now()
    activePointerId = e.pointerId
    isDragging.value = true
    isAnimating.value = false

    try {
      target.setPointerCapture(e.pointerId)
    } catch {
      // safe fallback
    }
  }

  function onPointerMove(e: PointerEvent) {
    if (!isDragging.value || e.pointerId !== activePointerId) return
    const deltaY = e.clientY - startY
    if (deltaY > 0) {
      translateY.value = deltaY
    } else {
      translateY.value = deltaY * 0.15 // gentle rubber-band resistance
    }
  }

  function finishDrag(e: PointerEvent) {
    if (!isDragging.value || e.pointerId !== activePointerId) return
    isDragging.value = false
    activePointerId = null

    try {
      (e.target as HTMLElement).releasePointerCapture(e.pointerId)
    } catch {
      // safe fallback
    }

    const deltaY = e.clientY - startY
    const deltaTime = Date.now() - startTime
    const velocity = deltaY / Math.max(deltaTime, 1)
    const height = sheetRef.value?.clientHeight || 400
    const closeThreshold = height * 0.35
    const isFlick = velocity > 0.5 && deltaY > 30

    if (deltaY > closeThreshold || isFlick) {
      // Animate out and close
      isAnimating.value = true
      translateY.value = height
      setTimeout(() => {
        onClose()
        translateY.value = 0
        isAnimating.value = false
      }, 180)
    } else {
      // Spring back
      isAnimating.value = true
      translateY.value = 0
      setTimeout(() => {
        isAnimating.value = false
      }, 200)
    }
  }

  function onPointerUp(e: PointerEvent) {
    finishDrag(e)
  }

  function onPointerCancel(e: PointerEvent) {
    finishDrag(e)
  }

  const sheetStyle = computed(() => {
    if (translateY.value === 0 && !isAnimating.value) return {}
    return {
      transform: `translateY(${translateY.value}px)`,
      transition: isAnimating.value ? 'transform 0.2s cubic-bezier(0.16, 1, 0.3, 1)' : 'none',
      touchAction: 'none',
    }
  })

  return {
    sheetRef,
    sheetStyle,
    isDragging,
    onPointerDown,
    onPointerMove,
    onPointerUp,
    onPointerCancel,
  }
}

