// RGB is an opaque sRGB colour, channels 0-255.
export interface RGB {
  r: number
  g: number
  b: number
}

// parseColor reads a CSS colour in any form the app or a browser's
// getComputedStyle produces: #rrggbb, rgb()/rgba(), or Chromium's
// color(srgb r g b) serialization of a color-mix() result.
export function parseColor(value: string): RGB {
  const hex = /^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(value)
  if (hex) return { r: parseInt(hex[1]!, 16), g: parseInt(hex[2]!, 16), b: parseInt(hex[3]!, 16) }

  const rgb = /^rgba?\(\s*([\d.]+)[,\s]+([\d.]+)[,\s]+([\d.]+)/i.exec(value)
  if (rgb) return { r: Number(rgb[1]), g: Number(rgb[2]), b: Number(rgb[3]) }

  const srgb = /^color\(srgb\s+([\d.]+)\s+([\d.]+)\s+([\d.]+)/i.exec(value)
  if (srgb) return { r: Number(srgb[1]) * 255, g: Number(srgb[2]) * 255, b: Number(srgb[3]) * 255 }

  throw new Error(`unrecognised colour: ${value}`)
}

function srgbToLinear(c: number): number {
  return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4
}

function relativeLuminance({ r, g, b }: RGB): number {
  return (
    0.2126 * srgbToLinear(r / 255) + 0.7152 * srgbToLinear(g / 255) + 0.0722 * srgbToLinear(b / 255)
  )
}

// contrastRatio is the WCAG relative-luminance contrast ratio between two
// colours (1 to 21).
export function contrastRatio(a: string, b: string): number {
  const la = relativeLuminance(parseColor(a))
  const lb = relativeLuminance(parseColor(b))
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05)
}

// toOklab converts an sRGB colour to OKLab (Björn Ottosson,
// https://bottosson.github.io/posts/oklab/).
function toOklab({ r, g, b }: RGB): [number, number, number] {
  const rl = srgbToLinear(r / 255)
  const gl = srgbToLinear(g / 255)
  const bl = srgbToLinear(b / 255)

  const l = Math.cbrt(0.4122214708 * rl + 0.5363325363 * gl + 0.0514459929 * bl)
  const m = Math.cbrt(0.2119034982 * rl + 0.6806995451 * gl + 0.1073969566 * bl)
  const s = Math.cbrt(0.0883024619 * rl + 0.2817188376 * gl + 0.6299787005 * bl)

  return [
    0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s,
    1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s,
    0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s,
  ]
}

// deltaEOK is the perceptual colour difference between two colours in
// OKLab: the Euclidean distance between their OKLab coordinates.
export function deltaEOK(a: string, b: string): number {
  const [la, aa, ba] = toOklab(parseColor(a))
  const [lb, ab, bb] = toOklab(parseColor(b))
  return Math.hypot(la - lb, aa - ab, ba - bb)
}
