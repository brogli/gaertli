import { test, expect, type Locator, type Page } from '@playwright/test'
import { openMockRepo } from './helpers'

// A click inside a popover that stops its own propagation never reaches the
// document listener that resets OpenVue Popover's outside-click bookkeeping,
// so the next real outside click is swallowed. These checks need a real
// browser's click bubbling and layout; happy-dom does not prove them.

// settledBox is locator's box once the page's animations (the list's and
// the menu's enter) have ended.
async function settledBox(page: Page, locator: Locator) {
  await page.evaluate(() => Promise.all(document.getAnimations().map((a) => a.finished)))
  return (await locator.boundingBox())!
}

test('FR-UX-14 a worktree row button click still lets a later outside click close the list', async ({
  page,
}) => {
  await openMockRepo(page)

  await page.click('[data-test=worktree-picker]')
  await page.locator('.p-popover .picker-list li').first().waitFor()
  await page.click('[data-test=open-in-editor]')

  await page.click('.log-column')

  await expect(page.locator('.p-popover')).toBeHidden()
})

// openBranchList opens the mock repo's branch picker list and waits until
// it stands still.
async function openBranchList(page: Page): Promise<void> {
  await openMockRepo(page)
  await page.click('[data-test=branch-picker]')
  await settledBox(page, page.locator('.p-popover .branch-list'))
}

test("FR-BR-09, FR-UX-14 a branch entry's context menu opens at the pointer; one outside click closes the menu and the list", async ({
  page,
}) => {
  await openBranchList(page)
  const row = page.locator('.p-popover [data-test=ref-row]').nth(1)
  const box = (await row.boundingBox())!
  await row.click({ button: 'right', position: { x: 20, y: box.height / 2 } })
  const menu = page.locator('.p-contextmenu')

  const corner = await settledBox(page, menu)
  expect(Math.abs(corner.x - (box.x + 20))).toBeLessThanOrEqual(2)
  expect(Math.abs(corner.y - (box.y + box.height / 2))).toBeLessThanOrEqual(2)

  // Outside both: the log column's lower left, clear of the menu.
  const logColumn = (await page.locator('.log-column').boundingBox())!
  await page.mouse.click(logColumn.x + 10, logColumn.y + logColumn.height - 10)

  await expect(menu).toBeHidden()
  await expect(page.locator('.p-popover')).toBeHidden()
})

test("FR-BR-09 choosing a branch entry's action from its context menu closes the list", async ({
  page,
}) => {
  const pageErrors: Error[] = []
  page.on('pageerror', (error) => pageErrors.push(error))

  await openBranchList(page)
  await page.locator('.p-popover [data-test=ref-row]').nth(1).click({ button: 'right' })

  await page.locator('.p-contextmenu [role=menuitem]', { hasText: 'Show in log' }).click()

  await expect(page.locator('.p-contextmenu')).toBeHidden()
  await expect(page.locator('.p-popover')).toBeHidden()
  expect(pageErrors).toEqual([])
})

test("FR-BR-09 Shift+F10 in the branch list opens the active entry's context menu below it; Escape closes both", async ({
  page,
}) => {
  await openBranchList(page)
  await page.keyboard.press('ArrowDown')
  await page.keyboard.press('Shift+F10')
  const menu = page.locator('.p-contextmenu')

  const corner = await settledBox(page, menu)
  const active = (await page.locator('.p-popover .picker-rows li.active').boundingBox())!
  expect(Math.abs(corner.x - active.x)).toBeLessThanOrEqual(2)
  expect(Math.abs(corner.y - (active.y + active.height))).toBeLessThanOrEqual(2)

  await page.keyboard.press('Escape')
  await expect(menu).toBeHidden()
  await expect(page.locator('.p-popover')).toBeHidden()
})
