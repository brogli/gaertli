import { test, expect } from '@playwright/test'
import { openMockRepo } from './helpers'

// The MultiSelect's panel teleports into the body, which happy-dom's
// component test cannot open reliably; a real browser is needed for it.
test('FR-LOG-06 the branch search narrows the list', async ({ page }) => {
  await openMockRepo(page)

  await page.locator('.filter-bar .p-multiselect').click()
  const overlay = page.locator('.p-multiselect-overlay')
  await expect(overlay.locator('.p-multiselect-option').first()).toBeVisible()

  await page.getByPlaceholder('Search branches').fill('feat')
  await expect(overlay.locator('.p-multiselect-option')).toHaveCount(1)
  await expect(overlay.locator('.p-multiselect-option')).toContainText('feature')
})
