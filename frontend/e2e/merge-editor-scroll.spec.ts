// Requirements: FR-CONF-05
import { test, expect, type Locator, type Page } from '@playwright/test'
import { mockConflictUrl } from './helpers'

// happy-dom has no layout, so only a real browser shows where the panes'
// lines stand after a scroll.

test.use({ viewport: { width: 1280, height: 800 } })

const panes = (page: Page) => ({
  current: page.locator('.merge-editor .cm-scroller').nth(0),
  result: page.locator('.merge-editor .cm-scroller').nth(1),
  incoming: page.locator('.merge-editor .cm-scroller').nth(2),
})

// lineHeight is the height of the pane's first rendered line.
async function lineHeight(scroller: Locator): Promise<number> {
  return (await scroller.locator('.cm-line').first().boundingBox())!.height
}

// scrollLineToTop scrolls the pane so that its line n is at the top.
async function scrollLineToTop(scroller: Locator, n: number): Promise<void> {
  const height = await lineHeight(scroller)
  await scroller.evaluate((el, top) => (el.scrollTop = top), (n - 1) * height)
}

// lineTop is the distance of the pane's line with the text from the top of
// its viewport.
async function lineTop(scroller: Locator, text: string): Promise<number> {
  const line = scroller.locator('.cm-line', { hasText: text })
  const [lineBox, scrollerBox] = [await line.boundingBox(), await scroller.boundingBox()]
  return lineBox!.y - scrollerBox!.y
}

// offsetBetween is how far apart two panes' lines stand, measured from their
// viewports' tops.
async function offsetBetween(
  [scrollerA, textA]: [Locator, string],
  [scrollerB, textB]: [Locator, string],
): Promise<number> {
  return Math.abs((await lineTop(scrollerA, textA)) - (await lineTop(scrollerB, textB)))
}

test('scrolling a pane keeps the other panes at the same conflict', async ({ page }) => {
  await page.goto(mockConflictUrl)
  const { current, result, incoming } = panes(page)
  await result.locator('.cm-line').first().waitFor()
  const height = await lineHeight(result)

  await scrollLineToTop(result, 180)
  await expect
    .poll(() => offsetBetween([current, 'current region 3'], [result, 'base region 3']))
    .toBeLessThanOrEqual(height)
  expect(
    await offsetBetween([incoming, 'incoming region 3'], [result, 'base region 3']),
  ).toBeLessThanOrEqual(height)

  await scrollLineToTop(incoming, 130)
  await expect
    .poll(() => offsetBetween([result, 'base region 2'], [incoming, 'incoming region 2']))
    .toBeLessThanOrEqual(height)
  expect(
    await offsetBetween([current, 'current region 2'], [incoming, 'incoming region 2']),
  ).toBeLessThanOrEqual(height)
})
