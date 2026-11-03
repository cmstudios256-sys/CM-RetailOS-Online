/**
 * Chart color helper utilities matching the mother repository.
 * Generates dynamic accent colors, complementary colors, and opacities for charts.
 */

export const hexToRgba = (hex: string, alpha: number): string => {
  let r = 0, g = 0, b = 0
  const cleanHex = hex.trim()
  if (cleanHex.length === 4) {
    r = parseInt(cleanHex[1] + cleanHex[1], 16)
    g = parseInt(cleanHex[2] + cleanHex[2], 16)
    b = parseInt(cleanHex[3] + cleanHex[3], 16)
  } else if (cleanHex.length === 7) {
    r = parseInt(cleanHex.substring(1, 3), 16)
    g = parseInt(cleanHex.substring(3, 5), 16)
    b = parseInt(cleanHex.substring(5, 7), 16)
  }
  return `rgba(${r}, ${g}, ${b}, ${alpha})`
}

export const hexToComplementaryHsla = (hex: string, opacity: number): string => {
  const result = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex.trim())
  if (!result) return `hsla(199, 89%, 48%, ${opacity})`

  let r = parseInt(result[1], 16) / 255
  let g = parseInt(result[2], 16) / 255
  let b = parseInt(result[3], 16) / 255

  const max = Math.max(r, g, b)
  const min = Math.min(r, g, b)
  let h = 0, s = 0
  const l = (max + min) / 2

  if (max !== min) {
    const d = max - min
    s = l > 0.5 ? d / (2 - max - min) : d / (max + min)
    switch (max) {
      case r: h = (g - b) / d + (g < b ? 6 : 0); break
      case g: h = (b - r) / d + 2; break
      case b: h = (r - g) / d + 4; break
    }
    h /= 6
  }
  h = Math.round((h * 360 + 180) % 360)
  s = Math.round(s * 100)
  const lRound = Math.round(l * 100)

  return `hsla(${h}, ${s}%, ${lRound}%, ${opacity})`
}

export const getComplementaryHex = (hex: string): string => {
  let r = 0, g = 0, b = 0
  const cleanHex = hex.trim()
  if (cleanHex.length === 4) {
    r = parseInt(cleanHex[1] + cleanHex[1], 16)
    g = parseInt(cleanHex[2] + cleanHex[2], 16)
    b = parseInt(cleanHex[3] + cleanHex[3], 16)
  } else if (cleanHex.length === 7) {
    r = parseInt(cleanHex.substring(1, 3), 16)
    g = parseInt(cleanHex.substring(3, 5), 16)
    b = parseInt(cleanHex.substring(5, 7), 16)
  }

  r /= 255; g /= 255; b /= 255
  const max = Math.max(r, g, b), min = Math.min(r, g, b)
  let h = 0, s = 0
  const l = (max + min) / 2

  if (max !== min) {
    const d = max - min
    s = l > 0.5 ? d / (2 - max - min) : d / (max + min)
    switch (max) {
      case r: h = (g - b) / d + (g < b ? 6 : 0); break
      case g: h = (b - r) / d + 2; break
      case b: h = (r - g) / d + 4; break
    }
    h /= 6
  }

  h = (h + 0.5) % 1

  let r2: number, g2: number, b2: number
  if (s === 0) {
    r2 = g2 = b2 = l
  } else {
    const hue2rgb = (p: number, q: number, t: number) => {
      if (t < 0) t += 1
      if (t > 1) t -= 1
      if (t < 1 / 6) return p + (q - p) * 6 * t
      if (t < 1 / 2) return q
      if (t < 2 / 3) return p + (q - p) * (2 / 3 - t) * 6
      return p
    }
    const q = l < 0.5 ? l * (1 + s) : l + s - l * s
    const p = 2 * l - q
    r2 = hue2rgb(p, q, h + 1 / 3)
    g2 = hue2rgb(p, q, h)
    b2 = hue2rgb(p, q, h - 1 / 3)
  }

  const toHex = (x: number) => {
    const hexVal = Math.round(x * 255).toString(16)
    return hexVal.length === 1 ? '0' + hexVal : hexVal
  }
  return `#${toHex(r2)}${toHex(g2)}${toHex(b2)}`
}

export const getThemeColors = () => {
  if (typeof window === 'undefined') {
    return { accent: '#0ea5e9', complement: '#f59e0b' }
  }
  const root = document.documentElement
  const accent = getComputedStyle(root).getPropertyValue('--accent-color').trim() || '#0ea5e9'
  let complement = getComputedStyle(root).getPropertyValue('--accent-complement').trim()
  if (!complement) {
    complement = getComplementaryHex(accent)
  }
  return { accent, complement }
}

