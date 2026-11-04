import { createApp } from 'vue'
import App from './App.vue'
import { applyOnlinePrefsToDom } from './api'
import './style.css'

applyOnlinePrefsToDom()

createApp(App).mount('#app')

if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => {
    void navigator.serviceWorker.register('/service-worker.js').then(registration => {
      void registration.update()
    }).catch(error => {
      console.error('CM RetailOS offline support could not be enabled.', error)
    })
  })

  let refreshing = false
  navigator.serviceWorker.addEventListener('controllerchange', () => {
    if (!refreshing) {
      refreshing = true
      window.location.reload()
    }
  })
}
