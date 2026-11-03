// Requirements: FR-UX-22
import { test, expect, type Page } from '@playwright/test'
import { openMockRepo } from './helpers'

// happy-dom computes no layout, so a splitter drag computes no size; this
// needs a real browser. The wide layout (>=1400px, three views) and the
// medium layout (900-1399px, two views) keep the log view's width under
// separate storage keys, each surviving a reload.

// waitForGutterCount waits until the splitter for the current layout has
// rendered: two gutters (three panels) in wide, one (two panels) in medium.
async function waitForGutterCount(page: Page, count: number): Promise<void> {
  await expect(page.locator('.worktree-body > .p-splitter-gutter')).toHaveCount(count)
}

// dragLogGutter drags the gutter between the log view and its neighbour
// (the first one, in either layout) to x.
async function dragLogGutter(page: Page, x: number): Promise<void> {
  const gutter = (await page.locator('.worktree-body > .p-splitter-gutter').first().boundingBox())!
  const y = gutter.y + gutter.height / 2
  await page.mouse.move(gutter.x + gutter.width / 2, y)
  await page.mouse.down()
  await page.mouse.move(x, y, { steps: 8 })
  await page.mouse.up()
}

async function logWidth(page: Page): Promise<number> {
  return (await page.locator('.log-column').boundingBox())!.width
}

function expectClose(actual: number, expected: number): void {
  expect(Math.abs(actual - expected)).toBeLessThanOrEqual(2)
}

test('a dragged panel width is kept per layout, across a reload', async ({ page }) => {
  await page.setViewportSize({ width: 1600, height: 800 })
  await openMockRepo(page)
  await waitForGutterCount(page, 2)

  await dragLogGutter(page, 500)
  const wideWidth = await logWidth(page)

  await page.reload()
  await expect(page.locator('.row').first()).toBeVisible()
  await waitForGutterCount(page, 2)
  expectClose(await logWidth(page), wideWidth)

  await page.setViewportSize({ width: 1200, height: 800 })
  await waitForGutterCount(page, 1)
  await dragLogGutter(page, 400)
  const mediumWidth = await logWidth(page)
  expect(Math.abs(mediumWidth - wideWidth)).toBeGreaterThan(20)

  await page.reload()
  await expect(page.locator('.row').first()).toBeVisible()
  await waitForGutterCount(page, 1)
  expectClose(await logWidth(page), mediumWidth)

  await page.setViewportSize({ width: 1600, height: 800 })
  await waitForGutterCount(page, 2)
  expectClose(await logWidth(page), wideWidth)
})
