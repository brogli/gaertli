import { test, expect, type Page } from '@playwright/test'
import { mockDiffUrl } from './helpers'

// happy-dom computes no cascade from style.css: this needs a real browser.
// WebView2 on Windows shows its host window (black) through any part of the
// page that paints no background of its own, so every point of the diff
// window must have an opaque theme background in its own chain.

// RGB is a resolved, opaque background colour.
interface RGB {
  r: number
  g: number
  b: number
}

// effectiveColor is the background colour the page paints at (x, y): the
// nearest fully opaque background from the hit element up through its
// ancestors, or null if every layer up to <html> is transparent, so the
// host window shows through. A translucent layer (a diff line's highlight)
// is skipped, since only the base colour matters here.
async function effectiveColor(page: Page, x: number, y: number): Promise<RGB | null> {
  return page.evaluate(
    ({ x, y }) => {
      let el: Element | null = document.elementFromPoint(x, y)
      while (el) {
        const bg = getComputedStyle(el).backgroundColor
        const m = /rgba?\((\d+), *(\d+), *(\d+)(?:, *([\d.]+))?\)/.exec(bg)
        if (m && (m[4] === undefined || Number(m[4]) === 1)) {
          return { r: Number(m[1]), g: Number(m[2]), b: Number(m[3]) }
        }
        el = el.parentElement
      }
      return null
    },
    { x, y },
  )
}

function isLight(c: RGB | null): boolean {
  return !!c && c.r > 200 && c.g > 200 && c.b > 200
}

function isDarkNotBlack(c: RGB | null): boolean {
  return !!c && c.r < 200 && c.g < 200 && c.b < 200 && (c.r > 8 || c.g > 8 || c.b > 8)
}

// openDiffBackgrounds opens the mock diff and reads the background the page
// paints in its code area and below its file list's last row.
async function openDiffBackgrounds(
  page: Page,
): Promise<{ codeArea: RGB | null; fileList: RGB | null }> {
  await page.goto(mockDiffUrl)
  await page.waitForSelector('.cm-scroller')
  await page.waitForSelector('.file-list li')

  // The diff's content is taller than the window, so any point inside the
  // scroller is the code area; a glyph or blank space makes no difference.
  const scroller = await page.locator('.cm-scroller').first().boundingBox()
  if (!scroller) throw new Error('no .cm-scroller')
  const codeArea = await effectiveColor(page, scroller.x + 10, scroller.y + 40)

  // A point below the last file row, inside the file list's own container.
  const rows = page.locator('.file-list li')
  const lastRow = await rows.last().boundingBox()
  const list = await page.locator('.file-list').boundingBox()
  if (!lastRow || !list) throw new Error('no file list rows')
  const fileList = await effectiveColor(page, list.x + 10, lastRow.y + lastRow.height + 20)

  return { codeArea, fileList }
}

test('FR-UX-06 light theme: the page paints the code area and the empty file list area light', async ({
  page,
}) => {
  await page.emulateMedia({ colorScheme: 'light' })

  const { codeArea, fileList } = await openDiffBackgrounds(page)

  expect(isLight(codeArea)).toBe(true)
  expect(isLight(fileList)).toBe(true)
})

test('FR-UX-06 dark theme: the page paints the code area dark itself, not black', async ({
  page,
}) => {
  await page.emulateMedia({ colorScheme: 'dark' })

  const { codeArea } = await openDiffBackgrounds(page)

  expect(isDarkNotBlack(codeArea)).toBe(true)
})
