import { test, expect } from '@playwright/test'
import { openMockRepo } from './helpers'
import { contextMenuItems, logRow } from '../e2e-real/helpers'

// FR-LOG-13: Shift-click and Ctrl-click both build the same one selection,
// so two commits picked either way compare the same and offer the same
// menu. The virtualized list's own row click handlers, and real modifier
// keys, need a real browser; a component test cannot exercise either.

test('FR-LOG-13 Shift-click and Ctrl-click compare two commits the same way', async ({ page }) => {
  await openMockRepo(page)
  const view = page.locator('.compare-view')

  await logRow(page, 'commit 0').click()
  await logRow(page, 'commit 1').click({ modifiers: ['Shift'] })
  await expect(view).toBeVisible()
  await expect(logRow(page, 'commit 0')).toHaveClass(/compare-from/)
  await expect(logRow(page, 'commit 1')).toHaveClass(/compare-to/)
  const shiftLabels = await contextMenuItems(page, logRow(page, 'commit 1'))
  expect(shiftLabels).toEqual(
    expect.arrayContaining(['Cherry-pick 2 commits', 'Revert 2 commits', 'Swap', 'Stop comparing']),
  )
  await page.locator('.p-contextmenu [role=menuitem]', { hasText: 'Stop comparing' }).click()

  await logRow(page, 'commit 2').click({ modifiers: ['Control'] })
  await logRow(page, 'commit 3').click({ modifiers: ['Control'] })
  await expect(view).toBeVisible()
  await expect(logRow(page, 'commit 2')).toHaveClass(/compare-from/)
  await expect(logRow(page, 'commit 3')).toHaveClass(/compare-to/)
  const ctrlLabels = await contextMenuItems(page, logRow(page, 'commit 3'))
  expect(ctrlLabels).toEqual(shiftLabels)
})
