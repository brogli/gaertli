// Requirements: FR-DIFF-08
import { test, expect } from '@playwright/test'
import { mockDiffUrl } from './helpers'

// happy-dom has no layout, so only a real browser shows whether a diff taller
// than the window can be scrolled.
test('the wheel over a side-by-side diff scrolls both sides to their last lines', async ({
  page,
}) => {
  await page.goto(`${mockDiffUrl}&diffMode=side-by-side`)
  const firstOldLine = page.locator('.cm-merge-a .cm-line').first()
  await firstOldLine.hover()

  await page.mouse.wheel(0, 100_000)

  await expect(page.locator('.cm-merge-a').getByText('last old line')).toBeInViewport()
  await expect(page.locator('.cm-merge-b').getByText('last new line')).toBeInViewport()
})
