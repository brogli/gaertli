// Requirements: FR-UX-20
import { test, expect, type Page } from '@playwright/test'
import { mockDiffUrl } from './helpers'

// happy-dom has no layout and no default actions, so a mouse drag neither
// selects text nor resizes a splitter: only a real browser shows either.
//
// In Chromium, a mouse-down on the gutter or its handle (both childless)
// never grows into a selection as the mouse moves, over any target text;
// the same drag starting on real text extends normally. A mouse-down there
// selecting neighbouring text is covered by the manual check only.

// selectionText reads the page's current selected text.
function selectionText(page: Page): Promise<string> {
  return page.evaluate(() => window.getSelection()?.toString() ?? '')
}

test('dragging the file-list/diff divider over the toolbar keeps a selection made before', async ({
  page,
}) => {
  await page.goto(mockDiffUrl)
  const toolbar = page.locator('.diff-toolbar')
  await toolbar.locator('.p-button').first().waitFor()

  const line = page.locator('.cm-line').filter({ hasText: 'line' }).first()
  await line.dblclick({ position: { x: 10, y: 5 } })
  const selectedBefore = await selectionText(page)
  expect(selectedBefore).not.toBe('')

  const gutterBox = (await page.locator('.p-splitter-gutter').first().boundingBox())!
  const toolbarBox = (await toolbar.boundingBox())!
  const toolbarY = toolbarBox.y + toolbarBox.height / 2

  await page.mouse.move(gutterBox.x + gutterBox.width / 2, gutterBox.y + gutterBox.height / 2)
  await page.mouse.down()
  await page.mouse.move(toolbarBox.x + toolbarBox.width - 20, toolbarY, { steps: 8 })
  await page.mouse.move(toolbarBox.x + 20, toolbarY, { steps: 8 })
  await page.mouse.up()

  expect(await selectionText(page)).toBe(selectedBefore)
})
