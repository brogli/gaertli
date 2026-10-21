// Requirements: FR-UX-12
import { test, expect } from '@playwright/test'
import { openMockRepo } from './helpers'

// happy-dom computes no layout, so this needs a real browser: it checks
// that starting and ending a job never resizes the toolbar.
test('starting and ending a job never moves the log view', async ({ page }) => {
  await page.setViewportSize({ width: 1200, height: 800 })
  await openMockRepo(page)
  const log = page.locator('.log-column')
  const badge = page.locator('[data-test="jobs-badge"]')
  const before = await log.boundingBox()

  await page.getByRole('button', { name: 'Fetch', exact: true }).click()
  await expect(badge).toBeVisible()
  const during = await log.boundingBox()

  await expect(badge).toBeHidden()
  const after = await log.boundingBox()

  expect(during).toEqual(before)
  expect(after).toEqual(before)
})
