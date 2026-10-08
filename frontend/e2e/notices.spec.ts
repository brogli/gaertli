import { test, expect } from '@playwright/test'
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

test('FR-UX-15 in the light and the dark theme an error, a warning and a success notice are each opaque', async ({
  page,
}) => {
  await page.emulateMedia({ colorScheme: 'light' })
  await openMockRepo(page, '&debug=1')
  for (const label of ['Error', 'Warning', 'Success']) {
    await page.getByRole('button', { name: label }).click()
  }
  await page.locator('.p-toast-message-success').waitFor()

  for (const scheme of ['light', 'dark'] as const) {
    await page.emulateMedia({ colorScheme: scheme })
    for (const severity of ['error', 'warn', 'success']) {
      const toast = page.locator(`.p-toast-message-${severity}`)
      await expect
        .poll(
          async () => alphaOf(await toast.evaluate((el) => getComputedStyle(el).backgroundColor)),
          { message: scheme },
        )
        .toBe(1)
    }
  }
})
