import { expect, test, type Page } from '@playwright/test'
import { contrastRatio, deltaEOK, parseColor } from '../src/testing/colour'
import { mockConflictUrl } from './helpers'

// The themes of FR-DIFF-11 minus System, as diff-colours.spec.ts.
const THEMES = ['light', 'dark', 'gruvbox-dark', 'catppuccin-mocha', 'nord', 'dracula']

const TINTS = {
  changed: '.cm-change-changed',
  added: '.cm-change-added',
  unresolved: '.cm-conflict-unresolved',
  resolved: '.cm-conflict-resolved',
}

interface Painted {
  bg: string
  text: string
}

// backgroundChain is the background colours from selector's first match up
// to <html>, which composite() lays over each other.
async function backgroundChain(page: Page, selector: string): Promise<string> {
  return page.evaluate((sel) => {
    const el = document.querySelector(sel)
    if (!el) throw new Error(`no element for "${sel}"`)
    const chain: string[] = []
    for (let node: Element | null = el; node; node = node.parentElement)
      chain.push(getComputedStyle(node).backgroundColor)
    return chain.join('|')
  }, selector)
}

// composite is the opaque colour the page shows for a background chain.
function composite(chain: string): string {
  let [r, g, b] = [0, 0, 0]
  for (const css of chain.split('|').reverse()) {
    const c = parseColor(css)
    r = c.r * c.a + r * (1 - c.a)
    g = c.g * c.a + g * (1 - c.a)
    b = c.b * c.a + b * (1 - c.a)
  }
  return `rgb(${Math.round(r)}, ${Math.round(g)}, ${Math.round(b)})`
}

// measure reads the plain tint of a class in the current side's pane: no
// region counts as selected.
async function measure(page: Page, className: string): Promise<Painted> {
  await page.evaluate(() =>
    document
      .querySelectorAll('.cm-conflict-selected')
      .forEach((el) => el.classList.remove('cm-conflict-selected')),
  )
  const selector = `.merge-editor .cm-scroller ${className}`
  const text = await page.evaluate(
    (sel) => getComputedStyle(document.querySelector(sel)!).color,
    selector,
  )
  return { bg: composite(await backgroundChain(page, selector)), text }
}

for (const theme of THEMES) {
  test(`FR-CONF-05 ${theme} theme: changed, added, unresolved and resolved lines are visibly distinct`, async ({
    page,
  }) => {
    await page.goto(`${mockConflictUrl}&theme=${theme}`)
    await page.locator('.merge-editor .cm-line').first().waitFor()
    const editorBg = composite(await backgroundChain(page, '.merge-editor .cm-editor'))
    const painted: Record<string, Painted> = {
      changed: await measure(page, TINTS.changed),
      added: await measure(page, TINTS.added),
      unresolved: await measure(page, TINTS.unresolved),
    }
    await page.getByRole('button', { name: 'Take current' }).click()
    painted.resolved = await measure(page, TINTS.resolved)

    const names = Object.keys(painted)
    for (const name of names) {
      const { bg, text } = painted[name]!
      expect(deltaEOK(bg, editorBg), `${name} against the background`).toBeGreaterThanOrEqual(0.08)
      expect(contrastRatio(text, bg), `${name} text`).toBeGreaterThanOrEqual(4.5)
    }
    for (const [i, a] of names.entries()) {
      for (const b of names.slice(i + 1)) {
        const distance = deltaEOK(painted[a]!.bg, painted[b]!.bg)
        console.log(`${theme}: ΔEOK ${a}/${b}=${distance.toFixed(4)}`)
        expect(distance, `${a} against ${b}`).toBeGreaterThanOrEqual(0.04)
      }
    }
  })
}
