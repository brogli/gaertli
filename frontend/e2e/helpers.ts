import { expect, type Locator, type Page } from '@playwright/test'

// openMockRepo shows the mock repo's log once a row has rendered; query
// adds the mock's switches, e.g. "&upstream=1".
export async function openMockRepo(page: Page, query = ''): Promise<void> {
  await page.goto(`/?path=/mock/repo${query}`)
  await page.waitForSelector('.row')
}

// mockDiffUrl shows the mock comparison's file in the diff window.
export const mockDiffUrl =
  '/?view=diff&wt=/mock/repo&kind=range&from=aaa&to=bbb&file=src/components/staged-one.ts'

// expectNoOverlap: sorted left to right, each box's x-range ends at or
// before the next one's begins.
export function expectNoOverlap(boxes: { x: number; width: number }[]): void {
  const sorted = [...boxes].sort((a, b) => a.x - b.x)
  for (let i = 1; i < sorted.length; i++) {
    expect(sorted[i]!.x).toBeGreaterThanOrEqual(sorted[i - 1]!.x + sorted[i - 1]!.width)
  }
}

// overflows: locator's content is wider than its box.
export function overflows(locator: Locator): Promise<boolean> {
  return locator.evaluate((el) => el.scrollWidth > el.clientWidth)
}

// openBranchPicker opens the toolbar's branch picker.
export async function openBranchPicker(page: Page): Promise<void> {
  await page.locator('[data-test=branch-picker]').click()
  await page.locator('.p-popover .branch-list').waitFor()
}

// openWorktreePicker opens the toolbar's worktree picker.
export async function openWorktreePicker(page: Page): Promise<void> {
  await page.locator('[data-test=worktree-picker]').click()
  await page.locator('.p-popover .worktree-list').waitFor()
}

// refLi is the open picker's <li> of the ref or worktree named exactly name.
export function refLi(page: Page, name: string): Locator {
  return page.locator('.p-popover .picker-rows li', {
    has: page.locator(`.ref-name:text-is("${name}")`),
  })
}
