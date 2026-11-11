import type { DirectiveBinding } from 'vue'

export const draggable = {
  mounted(el: HTMLElement, binding: DirectiveBinding) {
    let handle: HTMLElement = el
    const handleSelector = binding.value

    if (handleSelector) {
      const foundHandle = el.querySelector(handleSelector)
      if (foundHandle) {
        handle = foundHandle as HTMLElement
      }
    }

    handle.style.cursor = 'grab'
    handle.style.userSelect = 'none'

    let isDragging = false
    let startX = 0
    let startY = 0
    let initialLeft = 0
    let initialTop = 0

    const style = window.getComputedStyle(el)
    if (style.position === 'static') {
      el.style.position = 'relative'
    }

    const onMouseDown = (e: MouseEvent) => {
      const target = e.target as HTMLElement
      if (['INPUT', 'BUTTON', 'SELECT', 'TEXTAREA'].includes(target.tagName) || target.closest('button')) {
        return
      }
      if (e.button !== 0) return

      isDragging = true
      startX = e.clientX
      startY = e.clientY

      const currentStyle = window.getComputedStyle(el)
      initialLeft = parseFloat(currentStyle.left) || 0
      initialTop = parseFloat(currentStyle.top) || 0

      handle.style.cursor = 'grabbing'
      document.body.style.userSelect = 'none'

      document.addEventListener('mousemove', onMouseMove)
      document.addEventListener('mouseup', onMouseUp)
    }

    const onMouseMove = (e: MouseEvent) => {
      if (!isDragging) return
      const dx = e.clientX - startX
      const dy = e.clientY - startY
      el.style.left = `${initialLeft + dx}px`
      el.style.top = `${initialTop + dy}px`
    }

    const onMouseUp = () => {
      if (!isDragging) return
      isDragging = false
      handle.style.cursor = 'grab'
      document.body.style.userSelect = ''
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
    }

    const onTouchStart = (e: TouchEvent) => {
      const target = e.target as HTMLElement
      if (['INPUT', 'BUTTON', 'SELECT', 'TEXTAREA'].includes(target.tagName) || target.closest('button')) {
        return
      }
      if (e.touches.length !== 1) return

      isDragging = true
      startX = e.touches[0].clientX
      startY = e.touches[0].clientY

      const currentStyle = window.getComputedStyle(el)
      initialLeft = parseFloat(currentStyle.left) || 0
      initialTop = parseFloat(currentStyle.top) || 0

      document.addEventListener('touchmove', onTouchMove, { passive: false })
      document.addEventListener('touchend', onTouchEnd)
      document.addEventListener('touchcancel', onTouchEnd)
    }

    const onTouchMove = (e: TouchEvent) => {
      if (!isDragging || e.touches.length !== 1) return
      e.preventDefault()
      const dx = e.touches[0].clientX - startX
      const dy = e.touches[0].clientY - startY
      el.style.left = `${initialLeft + dx}px`
      el.style.top = `${initialTop + dy}px`
    }

    const onTouchEnd = () => {
      if (!isDragging) return
      isDragging = false
      document.removeEventListener('touchmove', onTouchMove)
      document.removeEventListener('touchend', onTouchEnd)
      document.removeEventListener('touchcancel', onTouchEnd)
    }

    handle.addEventListener('mousedown', onMouseDown)
    handle.addEventListener('touchstart', onTouchStart, { passive: true })

    ;(el as any)._cleanupDrag = () => {
      handle.removeEventListener('mousedown', onMouseDown)
      handle.removeEventListener('touchstart', onTouchStart)
      document.removeEventListener('mousemove', onMouseMove)
      document.removeEventListener('mouseup', onMouseUp)
      document.removeEventListener('touchmove', onTouchMove)
      document.removeEventListener('touchend', onTouchEnd)
      document.removeEventListener('touchcancel', onTouchEnd)
    }
  },
  unmounted(el: any) {
    if (el._cleanupDrag) {
      el._cleanupDrag()
    }
  },
}
