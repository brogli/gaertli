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

// mockConflictUrl shows the mock working tree's conflicted file in the diff
// window.
export const mockConflictUrl =
  '/?view=diff&wt=/mock/repo&kind=working&conflicted=true&file=scroll.txt'

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

// settleAnimations waits until the page's running animations, such as a
// popover's scale-in, have ended or were cancelled (a cancelled one
// rejects). Boxes measured during one are scaled, each by the frame it is
// measured in.
export async function settleAnimations(page: Page): Promise<void> {
  await page.evaluate(() => Promise.allSettled(document.getAnimations().map((a) => a.finished)))
}

// openBranchPicker opens the toolbar's branch picker and waits until it
// stands still.
export async function openBranchPicker(page: Page): Promise<void> {
  await page.locator('[data-test=branch-picker]').click()
  await page.locator('.p-popover .branch-list').waitFor()
  await settleAnimations(page)
}

// openWorktreePicker opens the toolbar's worktree picker and waits until it
// stands still.
export async function openWorktreePicker(page: Page): Promise<void> {
  await page.locator('[data-test=worktree-picker]').click()
  await page.locator('.p-popover .worktree-list').waitFor()
  await settleAnimations(page)
}

// refLi is the open picker's <li> of the ref or worktree named exactly name.
export function refLi(page: Page, name: string): Locator {
  return page.locator('.p-popover .picker-rows li', {
    has: page.locator(`.ref-name:text-is("${name}")`),
  })
}

// switchToChanges picks "Changes" in the medium layout's side-panel switch
// and waits for the change list.
export async function switchToChanges(page: Page): Promise<void> {
  await page.getByRole('tab', { name: /^Changes/ }).click()
  await page.locator('.changes-column .files li').first().waitFor()
}

// dragLogBorder drags the border right of the log view to x.
export async function dragLogBorder(page: Page, x: number): Promise<void> {
  const gutter = (await page.locator('.worktree-body > .p-splitter-gutter').boundingBox())!
  const y = gutter.y + 200
  await page.mouse.move(gutter.x + gutter.width / 2, y)
  await page.mouse.down()
  await page.mouse.move(x, y, { steps: 8 })
  await page.mouse.up()
}
