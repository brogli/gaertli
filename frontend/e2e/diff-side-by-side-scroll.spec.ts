// Requirements: FR-DIFF-08
import { test, expect, type Page } from '@playwright/test'
import { mockDiffUrl } from './helpers'

// happy-dom has no layout, so only a real browser shows whether a diff taller
// than the window can be scrolled and where its controls stand.

const SIDE_BY_SIDE_URL = `${mockDiffUrl}&diffMode=side-by-side`

const sideA = (page: Page) => page.locator('.cm-merge-a')
const sideB = (page: Page) => page.locator('.cm-merge-b')

for (const [name, side] of [
  ['old', sideA],
  ['new', sideB],
] as const) {
  test(`the wheel over the ${name} side scrolls both sides to their last lines, level`, async ({
    page,
  }) => {
    await page.goto(SIDE_BY_SIDE_URL)
    await side(page).locator('.cm-line').first().hover()

    await page.mouse.wheel(0, 100_000)

    const lastOld = sideA(page).getByText('last old line')
    const lastNew = sideB(page).getByText('last new line')
    await expect(lastOld).toBeInViewport()
    await expect(lastNew).toBeInViewport()
    expect((await lastOld.boundingBox())!.y).toBe((await lastNew.boundingBox())!.y)
  })
}

test('on a long diff, the scrollbars of both sides and the grip between them stay in the window', async ({
  page,
}) => {
  await page.goto(SIDE_BY_SIDE_URL)
  await sideA(page).locator('.cm-line').first().waitFor()

  for (const side of [sideA, sideB]) {
    const box = (await side(page).locator('.cm-scroller').boundingBox())!
    expect(box.y + box.height).toBeLessThanOrEqual(page.viewportSize()!.height)
  }
  await expect(page.locator('.diff-view .p-splitter-gutter-handle')).toBeInViewport()
})

test('a window too short for the diff keeps the sides readable', async ({ page }) => {
  const height = 200
  await page.setViewportSize({ width: 1280, height })
  await page.goto(SIDE_BY_SIDE_URL)
  await sideA(page).locator('.cm-line').first().waitFor()

  expect((await sideA(page).boundingBox())!.height).toBeGreaterThan(height)
})
