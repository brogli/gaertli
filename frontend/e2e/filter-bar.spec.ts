import { test, expect } from '@playwright/test'

// The MultiSelect's panel teleports into the body, which happy-dom's
// component test cannot open reliably; a real browser is needed for it.
test('FR-LOG-06 the branch search narrows the list and hides the select-all checkbox', async ({
  page,
}) => {
  await page.goto('/?path=/mock/repo')
  await page.waitForSelector('.row')

  await page.locator('.filter-bar .p-multiselect').click()
  const overlay = page.locator('.p-multiselect-overlay')
  await expect(overlay.locator('.p-multiselect-option').first()).toBeVisible()
  await expect(overlay.locator('.p-multiselect-header input[type=checkbox]')).toHaveCount(0)

  await page.getByPlaceholder('Search branches').fill('feat')
  await expect(overlay.locator('.p-multiselect-option')).toHaveCount(1)
  await expect(overlay.locator('.p-multiselect-option')).toContainText('feature')
})
