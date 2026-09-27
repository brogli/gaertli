import { expect, test, type Page } from '@playwright/test'
import { contrastRatio, deltaEOK, parseColor } from '../src/testing/colour'
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

// alphaOf reads a CSS colour's alpha channel: 1 for #rrggbb and for
// rgb()/color(srgb …) with none given. parseColor (colour.ts) reads the
// same three forms but only their r/g/b.
function alphaOf(value: string): number {
  const rgb = /^rgba?\(\s*[\d.]+[,\s]+[\d.]+[,\s]+[\d.]+(?:[,\s/]+([\d.]+))?\s*\)$/i.exec(value)
  if (rgb) return rgb[1] === undefined ? 1 : Number(rgb[1])
  const srgb = /^color\(srgb\s+[\d.]+\s+[\d.]+\s+[\d.]+(?:\s*\/\s*([\d.]+))?\s*\)$/i.exec(value)
  if (srgb) return srgb[1] === undefined ? 1 : Number(srgb[1])
  return 1
}

// resolvedBackground is the opaque colour the page paints behind selector's
// first match, as a "rgb()" string: its own background-color, composited
// ("source over") onto each ancestor's in turn, up to <html>. A diff
// line's background is translucent (an alpha over the editor background),
// so reading it alone is not what the user sees.
async function resolvedBackground(page: Page, selector: string): Promise<string> {
  const chain = await page.evaluate((sel) => {
    const el = document.querySelector(sel)
    if (!el) throw new Error(`no element for "${sel}"`)
    const colours: string[] = []
    for (let node: Element | null = el; node; node = node.parentElement)
      colours.push(getComputedStyle(node).backgroundColor)
    return colours
  }, selector)

  let [r, g, b, a] = [0, 0, 0, 0]
  for (let i = chain.length - 1; i >= 0; i--) {
    const css = chain[i]!
    const rgb = parseColor(css)
    const ca = alphaOf(css)
    r = rgb.r * ca + r * (1 - ca)
    g = rgb.g * ca + g * (1 - ca)
    b = rgb.b * ca + b * (1 - ca)
    a = ca + a * (1 - ca)
  }
  return `rgb(${Math.round(r)}, ${Math.round(g)}, ${Math.round(b)})`
}

// computedColor is selector's first match's own CSS prop, unresolved
// against its ancestors: text and a gutter marker are both opaque, so no
// compositing is needed.
async function computedColor(
  page: Page,
  selector: string,
  prop: 'color' | 'backgroundColor',
): Promise<string> {
  return page.evaluate(
    ({ sel, prop }) => {
      const el = document.querySelector(sel)
      if (!el) throw new Error(`no element for "${sel}"`)
      return getComputedStyle(el)[prop]
    },
    { sel: selector, prop },
  )
}

interface Measurement {
  editorBg: string
  addedBg: string
  removedBg: string
  addedText: string
  removedText: string
  addedMarker: string
  removedMarker: string
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
