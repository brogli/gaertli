import { test, expect } from '@playwright/test'

// happy-dom computes no layout, so this needs a real browser: it checks
// that starting and ending a job never resizes the toolbar.
test.beforeEach(async ({ page }) => {
  await page.setViewportSize({ width: 1200, height: 800 })
})

test('FR-UX-12 starting and ending a job never moves the log view', async ({ page }) => {
  await page.goto('/?path=/mock/repo')
  await page.waitForSelector('.row')
  const log = page.locator('.log-column')
  const before = await log.boundingBox()

  await page.getByRole('button', { name: 'Fetch', exact: true }).click()
  await expect(page.locator('[data-test="jobs-badge"]')).toBeVisible()
  const during = await log.boundingBox()

  await expect(page.locator('[data-test="jobs-badge"]')).toBeHidden()
  const after = await log.boundingBox()

  expect(during).toEqual(before)
  expect(after).toEqual(before)
})
