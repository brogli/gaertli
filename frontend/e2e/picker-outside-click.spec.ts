import { test, expect } from '@playwright/test'

// A click inside a popover that stops its own propagation never reaches the
// document listener that resets OpenVue Popover's outside-click bookkeeping,
// so the next real outside click is swallowed. These checks need a real
// browser's click bubbling; happy-dom does not prove it.

test('FR-UX-14 a worktree row button click still lets a later outside click close the list', async ({
  page,
}) => {
  await page.goto('/?path=/mock/repo')
  await page.waitForSelector('.row')

  await page.click('[data-test=worktree-picker]')
  await page.locator('.p-popover .picker-list li').first().waitFor()
  await page.click('[data-test=open-in-editor]')

  await page.click('.log-column')

  await expect(page.locator('.p-popover')).toBeHidden()
})

test("FR-UX-14 opening a branch row's actions menu, then one outside click closes the menu and the list", async ({
  page,
}) => {
  await page.goto('/?path=/mock/repo')
  await page.waitForSelector('.row')

  await page.click('[data-test=branch-picker]')
  await page.locator('.p-popover .branch-list').waitFor()
  const row = page.locator('.picker-rows li', { has: page.locator('[data-test=ref-row]') }).first()
  await row.locator('[data-test=ref-actions]').click()
  await page.locator('.p-menu [role=menuitem]').first().waitFor()

  await page.click('.log-column')

  await expect(page.locator('.p-menu')).toBeHidden()
  await expect(page.locator('.p-popover')).toBeHidden()
})
