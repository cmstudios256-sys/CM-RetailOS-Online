const CACHE_NAME = 'cm-retailos-shell-v1'
const APP_SHELL = [
  '/',
  '/manifest.webmanifest',
  '/cmretailos-brand-logo.png',
  '/icons/cm-retailos-192.png',
  '/icons/cm-retailos-512.png',
]

self.addEventListener('install', event => {
  event.waitUntil((async () => {
    const cache = await caches.open(CACHE_NAME)
    const response = await fetch('/', { cache: 'reload' })
    if (!response.ok) throw new Error(`App shell request failed with ${response.status}`)
    const html = await response.clone().text()
    await cache.put('/', response)
    const assets = [...html.matchAll(/(?:src|href)="([^"]+\.(?:js|css)(?:\?[^"]*)?)"/g)]
      .map(match => new URL(match[1], self.location.origin))
      .filter(url => url.origin === self.location.origin)
      .map(url => url.pathname + url.search)
    await cache.addAll([...APP_SHELL.slice(1), ...assets])
    await self.skipWaiting()
  })())
})

self.addEventListener('activate', event => {
  event.waitUntil((async () => {
    const keys = await caches.keys()
    await Promise.all(keys.filter(key => key.startsWith('cm-retailos-shell-') && key !== CACHE_NAME)
      .map(key => caches.delete(key)))
    await self.clients.claim()
  })())
})

self.addEventListener('fetch', event => {
  const request = event.request
  const url = new URL(request.url)
  if (request.method !== 'GET' || url.origin !== self.location.origin || url.pathname.startsWith('/api/')) return

  if (request.mode === 'navigate') {
    event.respondWith(fetch(request).then(async response => {
      if (response.ok) {
        const cache = await caches.open(CACHE_NAME)
        await cache.put('/', response.clone())
      }
      return response
    }).catch(async () => (await caches.match(request)) || (await caches.match('/'))))
    return
  }

  if (!['script', 'style', 'image', 'font', 'manifest'].includes(request.destination)) return
  event.respondWith(caches.match(request).then(cached => {
    if (cached) return cached
    return fetch(request).then(async response => {
      if (response.ok) {
        const cache = await caches.open(CACHE_NAME)
        await cache.put(request, response.clone())
      }
      return response
    })
  }))
})
