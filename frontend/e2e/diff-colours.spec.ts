import { expect, test, type Page } from '@playwright/test'
import { mockDiffUrl } from './helpers'

// THEMES are FR-DIFF-11's seven themes minus System, which resolves to
// Light or Dark by the OS (theme.spec.ts already covers that) and needs no
// case of its own.
const THEMES = ['light', 'dark', 'gruvbox-dark', 'catppuccin-mocha', 'nord', 'dracula']
const MODES = ['unified', 'side-by-side'] as const

// ADDED_SELECTOR/REMOVED_SELECTOR/*_MARKER_SELECTOR match
// @codemirror/merge's own classes for a changed line and its gutter marker:
// the new side (b) shows an added line as ".cm-changedLine" inside an
// editor whose root carries "cm-merge-b"; the old side (a) shows a removed
// line the same way in side-by-side, or as unified's inline
// ".cm-deletedChunk" widget, since unified has no side-a editor of its own.
const ADDED_SELECTOR = '.cm-merge-b .cm-changedLine'
const REMOVED_SELECTOR = '.cm-merge-a .cm-changedLine, .cm-deletedChunk'
const ADDED_MARKER_SELECTOR = '.cm-merge-b .cm-changedLineGutter'
const REMOVED_MARKER_SELECTOR = '.cm-merge-a .cm-changedLineGutter, .cm-deletedLineGutter'
const EDITOR_SELECTOR = '.cm-editor'

// RGB is an opaque sRGB colour, channels 0-255.
interface RGB {
  r: number
  g: number
  b: number
}

// resolvedBackground is the opaque colour the page paints behind selector's
// first match: its own background-color, composited ("source over") onto
// each ancestor's in turn, up to <html>. A diff line's background is
// translucent (an alpha over the editor background), so reading it alone
// is not what the user sees. Colours come back as rgb()/rgba() or, for a
// color-mix() result, Chromium's "color(srgb r g b)".
async function resolvedBackground(page: Page, selector: string): Promise<RGB> {
  return page.evaluate((sel) => {
    function channels(css: string): [number, number, number, number] {
      const rgb = /^rgba?\(([\d.]+),\s*([\d.]+),\s*([\d.]+)(?:,\s*([\d.]+))?\)/.exec(css)
      if (rgb)
        return [
          Number(rgb[1]),
          Number(rgb[2]),
          Number(rgb[3]),
          rgb[4] === undefined ? 1 : Number(rgb[4]),
        ]
      const srgb = /^color\(srgb\s+([\d.]+)\s+([\d.]+)\s+([\d.]+)(?:\s*\/\s*([\d.]+))?\)/.exec(css)
      if (srgb)
        return [
          Number(srgb[1]) * 255,
          Number(srgb[2]) * 255,
          Number(srgb[3]) * 255,
          srgb[4] === undefined ? 1 : Number(srgb[4]),
        ]
      return [0, 0, 0, 0]
    }
    const el = document.querySelector(sel)
    if (!el) throw new Error(`no element for "${sel}"`)
    let node: Element | null = el
    const chain: Array<[number, number, number, number]> = []
    while (node) {
      chain.push(channels(getComputedStyle(node).backgroundColor))
      node = node.parentElement
    }
    let [r, g, b, a] = [0, 0, 0, 0]
    for (let i = chain.length - 1; i >= 0; i--) {
      const [cr, cg, cb, ca] = chain[i]!
      r = cr * ca + r * (1 - ca)
      g = cg * ca + g * (1 - ca)
      b = cb * ca + b * (1 - ca)
      a = ca + a * (1 - ca)
    }
    return { r: Math.round(r), g: Math.round(g), b: Math.round(b) }
  }, selector)
}

// computedColor is selector's first match's own CSS prop, unresolved
// against its ancestors: text and a gutter marker are both opaque, so no
// compositing is needed.
async function computedColor(
  page: Page,
  selector: string,
  prop: 'color' | 'backgroundColor',
): Promise<RGB> {
  return page.evaluate(
    ({ sel, prop }) => {
      const rgb = /^rgba?\(([\d.]+),\s*([\d.]+),\s*([\d.]+)/.exec(
        getComputedStyle(document.querySelector(sel)!)[prop],
      )
      if (!rgb) throw new Error(`no element or unreadable colour for "${sel}"`)
      return { r: Number(rgb[1]), g: Number(rgb[2]), b: Number(rgb[3]) }
    },
    { sel: selector, prop },
  )
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
function contrastRatio(a: RGB, b: RGB): number {
  const la = relativeLuminance(a)
  const lb = relativeLuminance(b)
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
function deltaEOK(a: RGB, b: RGB): number {
  const [la, aa, ba] = toOklab(a)
  const [lb, ab, bb] = toOklab(b)
  return Math.hypot(la - lb, aa - ab, ba - bb)
}

interface Measurement {
  editorBg: RGB
  addedBg: RGB
  removedBg: RGB
  addedText: RGB
  removedText: RGB
  addedMarker: RGB
  removedMarker: RGB
}

async function measure(
  page: Page,
  theme: string,
  mode: (typeof MODES)[number],
): Promise<Measurement> {
  await page.goto(`${mockDiffUrl}&theme=${theme}&diffMode=${mode}`)
  await page.locator(ADDED_SELECTOR).first().waitFor()
  await page.locator(REMOVED_SELECTOR).first().waitFor()

  return {
    editorBg: await resolvedBackground(page, EDITOR_SELECTOR),
    addedBg: await resolvedBackground(page, ADDED_SELECTOR),
    removedBg: await resolvedBackground(page, REMOVED_SELECTOR),
    addedText: await computedColor(page, ADDED_SELECTOR, 'color'),
    removedText: await computedColor(page, REMOVED_SELECTOR, 'color'),
    addedMarker: await computedColor(page, ADDED_MARKER_SELECTOR, 'backgroundColor'),
    removedMarker: await computedColor(page, REMOVED_MARKER_SELECTOR, 'backgroundColor'),
  }
}

for (const theme of THEMES) {
  for (const mode of MODES) {
    test(`FR-DIFF-11 ${theme} theme, ${mode}: added and removed lines are visibly distinct`, async ({
      page,
    }) => {
      // Today's colours (@codemirror/merge's default 8% alpha) measure far
      // below FR-DIFF-11's threshold in every theme; task 7 tunes
      // main.css's mix and removes this mark.
      test.fail()

      const m = await measure(page, theme, mode)

      const addedVsBg = deltaEOK(m.addedBg, m.editorBg)
      const removedVsBg = deltaEOK(m.removedBg, m.editorBg)
      const addedVsRemoved = deltaEOK(m.addedBg, m.removedBg)
      console.log(
        `${theme} ${mode}: ΔEOK added/bg=${addedVsBg.toFixed(4)} removed/bg=${removedVsBg.toFixed(4)} added/removed=${addedVsRemoved.toFixed(4)}`,
      )

      expect(addedVsBg).toBeGreaterThanOrEqual(0.08)
      expect(removedVsBg).toBeGreaterThanOrEqual(0.08)
      expect(addedVsRemoved).toBeGreaterThanOrEqual(0.08)
      expect(contrastRatio(m.addedText, m.addedBg)).toBeGreaterThanOrEqual(4.5)
      expect(contrastRatio(m.removedText, m.removedBg)).toBeGreaterThanOrEqual(4.5)
      expect(contrastRatio(m.addedMarker, m.editorBg)).toBeGreaterThanOrEqual(3)
      expect(contrastRatio(m.removedMarker, m.editorBg)).toBeGreaterThanOrEqual(3)
    })
  }
}
