import { test, expect, type Page } from '@playwright/test'
import { openMockRepo } from './helpers'

// alphaOf reads a computed background colour's alpha channel, from either
// rgba(r, g, b, a) or a resolved color-mix's color(srgb r g b / a); a
// colour with neither carries none, which means fully opaque.
function alphaOf(color: string): number {
  const slash = /\/\s*([\d.]+)\s*\)\s*$/.exec(color)
  if (slash) return Number(slash[1])
  const rgba = /^rgba\([^)]*,\s*([\d.]+)\)$/.exec(color)
  return rgba ? Number(rgba[1]) : 1
}

// showSampleNotices opens debug mode's strip and clicks its three sample
// buttons, so an error, a warning and a success notice all show at once.
async function showSampleNotices(page: Page): Promise<void> {
  await openMockRepo(page, '&debug=1')
  for (const label of ['Error', 'Warning', 'Success']) {
    await page.getByRole('button', { name: label }).click()
  }
  await page.locator('.p-toast-message-success').waitFor()
}

for (const scheme of ['light', 'dark'] as const) {
  test(`FR-UX-15 ${scheme} theme: an error, a warning and a success notice are each opaque`, async ({
    page,
  }) => {
    await page.emulateMedia({ colorScheme: scheme })
    await showSampleNotices(page)

    for (const severity of ['error', 'warn', 'success']) {
      const background = await page
        .locator(`.p-toast-message-${severity}`)
        .evaluate((el) => getComputedStyle(el).backgroundColor)
      expect(alphaOf(background)).toBe(1)
    }
  })
}
