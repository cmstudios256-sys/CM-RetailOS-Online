const CACHE_NAME = 'cm-retailos-shell-v14'
const APP_SHELL = [
  '/',
  '/index.html',
  '/manifest.webmanifest',
  '/cmretailos-brand-logo.png',
  '/icons/cm-retailos-192.png',
  '/icons/cm-retailos-512.png',
  '/icons/cm-retailos-maskable-512.png',
  '/icons/apple-touch-icon.png',
  '/developer-portrait.jpg',
]

const isCacheSafeResponse = response => {
  if (!response || (!response.ok && response.type !== 'opaque')) return false
  const cacheControl = (response.headers.get('cache-control') || '').toLowerCase()
  return !cacheControl.includes('no-store')
}

self.addEventListener('install', event => {
  event.waitUntil((async () => {
    const cache = await caches.open(CACHE_NAME)
    
    // Cache static shell entries
    await Promise.allSettled(
      APP_SHELL.map(async url => {
        try {
          const res = await fetch(url, { cache: 'reload' })
          if (res.ok) await cache.put(url, res)
        } catch {
          // ignore individual asset load failures during install
        }
      })
    )

    // Precache assets referenced in root index.html
    try {
      const response = await fetch('/', { cache: 'reload' })
      if (response.ok) {
        const html = await response.clone().text()
        await cache.put('/', response)
        const assets = [...html.matchAll(/(?:src|href)="([^"]+\.(?:js|css)(?:\?[^"]*)?)"/g)]
          .map(match => new URL(match[1], self.location.origin))
          .filter(url => url.origin === self.location.origin)
          .map(url => url.pathname + url.search)
        
        await Promise.allSettled(
          assets.map(async assetUrl => {
            const assetRes = await fetch(assetUrl)
            if (assetRes.ok) await cache.put(assetUrl, assetRes)
          })
        )
      }
    } catch {
      // offline during install or dev
    }

    await self.skipWaiting()
  })())
})

self.addEventListener('activate', event => {
  event.waitUntil((async () => {
    const keys = await caches.keys()
    await Promise.all(
      keys
        .filter(key => key.startsWith('cm-retailos-shell-') && key !== CACHE_NAME)
        .map(key => caches.delete(key))
    )
    await self.clients.claim()
  })())
})

self.addEventListener('fetch', event => {
  const request = event.request
  const url = new URL(request.url)

  // API requests stay network-only; never cached
  if (url.pathname.startsWith('/api/')) return

  // Only handle GET requests
  if (request.method !== 'GET') return

  // Bypass Vite dev server internals
  if (
    url.pathname.startsWith('/@') ||
    url.pathname.startsWith('/src/') ||
    url.pathname.includes('/node_modules/') ||
    url.search.includes('t=')
  ) {
    return
  }

  // Navigation mode: Network-first when online (guarantees fresh deployed HTML/JS hashes), falls back to cached shell when offline
  if (request.mode === 'navigate') {
    event.respondWith((async () => {
      const cache = await caches.open(CACHE_NAME)
      try {
        const networkResponse = await fetch(request)
        if (networkResponse && networkResponse.ok && isCacheSafeResponse(networkResponse)) {
          await cache.put('/', networkResponse.clone())
          await cache.put('/index.html', networkResponse.clone())
        }
        return networkResponse
      } catch {
        return (await cache.match(request)) || (await cache.match('/')) || (await cache.match('/index.html')) || Response.error()
      }
    })())
    return
  }

  // Cache fonts (including Google fonts) and static assets
  const isFont = request.destination === 'font' || url.hostname.includes('gstatic.com') || url.hostname.includes('fonts.googleapis.com')
  const isAsset = ['script', 'style', 'image', 'manifest'].includes(request.destination) ||
    url.pathname.endsWith('.js') ||
    url.pathname.endsWith('.css') ||
    url.pathname.endsWith('.png') ||
    url.pathname.endsWith('.svg') ||
    url.pathname.endsWith('.ico')

  if (isFont || (isAsset && url.origin === self.location.origin)) {
    event.respondWith((async () => {
      const cache = await caches.open(CACHE_NAME)
      const cached = await cache.match(request)
      if (cached) return cached

      try {
        const response = await fetch(request)
        if (isCacheSafeResponse(response)) {
          await cache.put(request, response.clone())
        }
        return response
      } catch {
        return cached || Response.error()
      }
    })())
  }
})
