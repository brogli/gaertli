// Requirements: FR-DIFF-14
import { expect, test } from '@playwright/test'
import { mockDiffUrl, offCentre } from './helpers'

// happy-dom has no layout and does not focus a field on select(), so only a
// real browser shows that opening the search puts the cursor in its field,
// that a match is scrolled to the middle, and that the panel sticks below the
// toolbar while the unified diff scrolls as a whole.
test('unified, the search opens with the cursor in its field, centres a match and stays below the toolbar', async ({
  page,
}) => {
  await page.goto(mockDiffUrl)
  await page.locator('[data-test="search"]').click()
  const field = page.locator('.cm-search input[name="search"]')
  await expect(field).toBeFocused()

  await field.pressSequentially('middle line')
  await field.press('Enter')

  const match = page.locator('.cm-searchMatch-selected')
  await expect(match).toBeInViewport()
  expect(await offCentre(match, page.locator('.diff-view'))).toBeLessThan(2)
  const toolbar = (await page.locator('.diff-toolbar').boundingBox())!
  const panel = (await page.locator('.cm-panels-top').boundingBox())!
  expect(panel.y).toBeCloseTo(toolbar.y + toolbar.height, 0)
})
