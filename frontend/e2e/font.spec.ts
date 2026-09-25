import { test, expect, type Page } from '@playwright/test'
import { mockDiffUrl, openMockRepo } from './helpers'

// happy-dom loads no fonts, so these need a real browser: they check that
// the fonts are the bundled ones, whatever the system has installed.

// family is the computed font family of the first element selector matches.
function family(page: Page, selector: string): Promise<string> {
  return page
    .locator(selector)
    .first()
    .evaluate((el) => getComputedStyle(el).fontFamily)
}

async function openLog(page: Page): Promise<void> {
  await openMockRepo(page)
  await page.evaluate(() => document.fonts.ready)
}

test('FR-UX-18 the UI uses the bundled font, OpenVue components included', async ({ page }) => {
  await openLog(page)

  expect(await family(page, 'body')).toMatch(/^"Inter Variable"/)
  expect(await family(page, '.p-button')).toMatch(/^"Inter Variable"/)
  expect(await page.evaluate(() => document.fonts.check('13px "Inter Variable"'))).toBe(true)
})

test('FR-UX-18 hashes and diffs use the bundled fixed-width font', async ({ page }) => {
  const fixedWidth = /^"JetBrains Mono Variable"/
  await openLog(page)

  expect(await family(page, '.row .hash')).toMatch(fixedWidth)
  expect(await page.evaluate(() => document.fonts.check('13px "JetBrains Mono Variable"'))).toBe(
    true,
  )

  await page.goto(mockDiffUrl)
  await expect(page.locator('.cm-scroller').first()).toBeVisible()
  expect(await family(page, '.cm-scroller')).toMatch(fixedWidth)
})
