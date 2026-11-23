// Requirements: FR-DIFF-19, FR-DIFF-20, FR-DIFF-21
import { expect, test } from '@playwright/test'
import { mockDiffUrl, offCentre } from './helpers'

// happy-dom has no layout, so only a real browser shows that a step brings the
// change to the middle of its pane, where the unified diff scrolls as a whole,
// and that the outline is painted on the removed block @codemirror/merge draws.

test('unified, the diff opens with its first change outlined from its removed block; a step centres the change', async ({
  page,
}) => {
  await page.goto(mockDiffUrl)
  const removed = page.locator('.cm-deletedChunk.cm-chunk-selected')
  await expect(removed).toHaveText('line two')
  expect(await removed.evaluate((el) => getComputedStyle(el).boxShadow)).not.toBe('none')

  await page.locator('[data-test="prev-change"]').click()

  const far = page.locator('.cm-line.cm-chunk-selected', { hasText: 'far new line' })
  await expect(far).toBeInViewport()
  expect(await offCentre(far, page.locator('.diff-view'))).toBeLessThan(2)
})

test('side by side, a step centres the change on both sides', async ({ page }) => {
  await page.goto(`${mockDiffUrl}&diffMode=side-by-side`)
  await page.locator('.cm-merge-b .cm-chunk-selected').first().waitFor()

  await page.locator('[data-test="prev-change"]').click()

  for (const [side, text] of [
    ['a', 'far old line'],
    ['b', 'far new line'],
  ] as const) {
    const far = page.locator(`.cm-merge-${side} .cm-line.cm-chunk-selected`, { hasText: text })
    await expect(far).toBeInViewport()
    expect(await offCentre(far, page.locator(`.cm-merge-${side} .cm-scroller`))).toBeLessThan(2)
  }
})
