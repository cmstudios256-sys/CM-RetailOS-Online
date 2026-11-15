import { createApp } from 'vue'
import App from './App.vue'
import { applyOnlinePrefsToDom } from './api'
import { draggable } from './directives/draggable'
import './style.css'

applyOnlinePrefsToDom()

if (typeof document !== 'undefined') {
  const preventEvent = (event: Event) => {
    event.preventDefault()
  }

  document.addEventListener('dblclick', preventEvent, { passive: false })
  document.addEventListener('gesturestart', preventEvent, { passive: false })
  document.addEventListener('gesturechange', preventEvent, { passive: false })
  document.addEventListener('gestureend', preventEvent, { passive: false })

  // Prevent multi-touch pinch zoom on iPadOS/Android tablets and phones
  const preventPinchZoom = (event: TouchEvent) => {
    if (event.touches.length > 1) {
      event.preventDefault()
    }
  }
  document.addEventListener('touchstart', preventPinchZoom, { passive: false })
  document.addEventListener('touchmove', preventPinchZoom, { passive: false })
}

const app = createApp(App)
app.directive('draggable', draggable)
app.mount('#app')

if ('serviceWorker' in navigator) {
  const hadController = Boolean(navigator.serviceWorker.controller)
  let refreshing = false

  navigator.serviceWorker.addEventListener('controllerchange', () => {
    if (hadController && !refreshing) {
      refreshing = true
      window.location.reload()
    }
  })

  window.addEventListener('load', () => {
    void navigator.serviceWorker.register('/service-worker.js', { updateViaCache: 'none' }).then(registration => {
      const checkForUpdate = () => {
        if (typeof navigator !== 'undefined' && !navigator.onLine) return
        void registration.update().catch(() => {})
      }

      checkForUpdate()

      document.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'visible') {
          checkForUpdate()
        }
      })

      window.addEventListener('focus', checkForUpdate)
      window.addEventListener('online', checkForUpdate)
      window.setInterval(() => {
        if (document.visibilityState === 'visible') {
          checkForUpdate()
        }
      }, 60_000)
    }).catch(error => {
      console.error('CM RetailOS offline support could not be enabled.', error)
    })
  })
}
