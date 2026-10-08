import { test, expect, type Locator, type Page } from '@playwright/test'
import { openMockRepo, settleAnimations } from './helpers'

// A click inside a popover that stops its own propagation never reaches the
// document listener that resets OpenVue Popover's outside-click bookkeeping,
// so the next real outside click is swallowed. These checks need a real
// browser's click bubbling and layout; happy-dom does not prove them.

// settledBox is locator's box once the page's animations (the list's and
// the menu's enter) have settled.
async function settledBox(page: Page, locator: Locator) {
  await settleAnimations(page)
  return (await locator.boundingBox())!
}

test('FR-UX-14 a worktree row button click still lets a later outside click close the list', async ({
  page,
}) => {
  await openMockRepo(page)

  await page.locator('[data-test=worktree-picker]').click()
  await page.locator('.p-popover .picker-list li').first().waitFor()
  // Any row's button does: this click only needs to stop propagation inside
  // the popover.
  await page.locator('[data-test=open-in-editor]').first().click()

  await page.locator('.log-column').click()

  await expect(page.locator('.p-popover')).toBeHidden()
})

// openBranchList opens the mock repo's branch picker list and waits until
// it stands still.
async function openBranchList(page: Page): Promise<void> {
  await openMockRepo(page)
  await page.locator('[data-test=branch-picker]').click()
  await settledBox(page, page.locator('.p-popover .branch-list'))
}

test('FR-BR-09, FR-UX-14 a branch entry\'s context menu opens at the pointer and its "…" menu below its button; one outside click closes the menu and the list', async ({
  page,
}) => {
  await openBranchList(page)
  const menu = page.locator('.p-contextmenu')
  const rows = page.locator('.p-popover .picker-rows > li').filter({
    has: page.locator('[data-test=ref-row]'),
  })

  async function expectOutsideClickClosesBoth(): Promise<void> {
    // Outside both: the log column's lower left, clear of the menu.
    const logColumn = (await page.locator('.log-column').boundingBox())!
    await page.mouse.click(logColumn.x + 10, logColumn.y + logColumn.height - 10)
    await expect(menu).toBeHidden()
    await expect(page.locator('.p-popover')).toBeHidden()
  }

  const row = page.locator('.p-popover [data-test=ref-row]').nth(1)
  const rowBox = (await row.boundingBox())!
  await row.click({
    button: 'right',
    position: { x: 20, y: rowBox.height / 2 },
  })
  const atPointer = await settledBox(page, menu)
  expect(Math.abs(atPointer.x - (rowBox.x + 20))).toBeLessThanOrEqual(2)
  expect(Math.abs(atPointer.y - (rowBox.y + rowBox.height / 2))).toBeLessThanOrEqual(2)
  await expectOutsideClickClosesBoth()

  await page.locator('[data-test=branch-picker]').click()
  await settledBox(page, page.locator('.p-popover .branch-list'))
  const button = rows.nth(1).locator('[data-test=row-actions]')
  const buttonBox = (await button.boundingBox())!
  await button.click()
  const belowButton = await settledBox(page, menu)
  expect(Math.abs(belowButton.x - buttonBox.x)).toBeLessThanOrEqual(2)
  expect(Math.abs(belowButton.y - (buttonBox.y + buttonBox.height))).toBeLessThanOrEqual(2)
  await expectOutsideClickClosesBoth()
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
