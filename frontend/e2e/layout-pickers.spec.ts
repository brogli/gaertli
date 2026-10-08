import { test, expect } from '@playwright/test'
import { openBranchPicker, openMockRepo, openWorktreePicker, overflows, refLi } from './helpers'

// Layout at the 1200 x 800 window the fixtures open, with the mock's refs
// and worktrees in the pickers.
test.beforeEach(async ({ page }) => {
  await page.setViewportSize({ width: 1200, height: 800 })
})

test('FR-UX-09 a long branch name ends in an ellipsis and shows whole on hover', async ({
  page,
}) => {
  await openMockRepo(page)
  const long = 'feature/a-rather-long-branch-name-that-does-not-fit'

  await openBranchPicker(page)
  const name = refLi(page, long).locator('.ref-name')
  await expect(name).toHaveAttribute('title', long)
  expect(await overflows(name)).toBe(true)
})

test('FR-UX-09 a worktree row keeps its name, flags and buttons on one line; a long branch name ends in an ellipsis', async ({
  page,
}) => {
  await openMockRepo(page, '&worktrees=crowded')
  const long = 'a-rather-long-branch-name-that-does-not-fit-the-worktree-picker'

  await openWorktreePicker(page)
  // The row's own li: its label span and its sibling buttons, all on one
  // line.
  const rows = page.locator('.p-popover .picker-rows li', {
    has: page.locator('[data-test=worktree-row]'),
  })
  await expect(rows).toHaveCount(6)
  for (const row of await rows.all()) {
    const boxes = await Promise.all(
      (await row.locator(':scope > *').all()).map(async (child) => (await child.boundingBox())!),
    )
    const middle = boxes[0]!.y + boxes[0]!.height / 2
    for (const box of boxes) {
      expect(Math.abs(box.y + box.height / 2 - middle)).toBeLessThanOrEqual(4)
    }
    for (let i = 1; i < boxes.length; i++) {
      expect(boxes[i]!.x).toBeGreaterThanOrEqual(boxes[i - 1]!.x + boxes[i - 1]!.width + 2)
    }
  }

  const name = rows.locator(`[data-test=worktree-branch][title="${long}"]`)
  expect(await overflows(name)).toBe(true)
})
