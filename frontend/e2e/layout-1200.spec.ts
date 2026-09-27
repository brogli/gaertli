import { test, expect, type Locator, type Page } from '@playwright/test'
import { expectNoOverlap, mockDiffUrl, openMockRepo } from './helpers'

// happy-dom computes no layout, so these checks (widths, wrapping, shared
// rows) need a real browser, at 1200 x 800.
test.beforeEach(async ({ page }) => {
  await page.setViewportSize({ width: 1200, height: 800 })
})

// stableBox polls for a non-null bounding box: right after a route or
// selection change, a locator can briefly report null before its layout is
// ready.
async function stableBox(locator: Locator) {
  let box: Awaited<ReturnType<Locator['boundingBox']>> = null
  await expect.poll(async () => (box = await locator.boundingBox())).not.toBeNull()
  return box!
}

// middle is the vertical center of locator's box.
async function middle(locator: Locator): Promise<number> {
  const box = await stableBox(locator)
  return box.y + box.height / 2
}

// openDiffWindow shows mockDiffUrl once its toolbar has rendered.
async function openDiffWindow(page: Page): Promise<void> {
  await page.goto(mockDiffUrl)
  await page.locator('.diff-toolbar .p-button').first().waitFor()
}

// expectInWindow: box lies wholly inside the 1200 x 800 window.
function expectInWindow(box: { x: number; y: number; width: number; height: number }): void {
  expect(box.x).toBeGreaterThanOrEqual(0)
  expect(box.y).toBeGreaterThanOrEqual(0)
  expect(box.x + box.width).toBeLessThanOrEqual(1200)
  expect(box.y + box.height).toBeLessThanOrEqual(800)
}

test('FR-UX-07 the toolbar keeps the pickers, sync controls, Refresh, Open in editor and the gear on one row', async ({
  page,
}) => {
  await openMockRepo(page)
  const toolbar = page.locator('.toolbar')
  await expect(toolbar.locator('[data-test=repo-picker]')).toContainText('repo')
  await expect(toolbar.locator('[data-test=worktree-picker]')).toContainText('repo')

  const controls = await toolbar.locator('button, select').all()
  expect(controls.length).toBeGreaterThanOrEqual(7)
  const middles = await Promise.all(controls.map(middle))
  expect(Math.max(...middles) - Math.min(...middles)).toBeLessThanOrEqual(4)
  expect(await toolbar.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true)
  expectInWindow(await stableBox(toolbar.locator('[data-test=open-settings]')))
})

test('FR-UX-07 FR-UX-14 each picker opens inside the window, its rows on one line each, and Escape closes it', async ({
  page,
}) => {
  await openMockRepo(page)

  for (const picker of ['repo-picker', 'worktree-picker', 'branch-picker']) {
    await page.locator(`[data-test=${picker}]`).click()
    const list = page.locator('.p-popover .picker-list')
    await expect(list.locator('li').first()).toBeVisible()
    expectInWindow(await stableBox(page.locator('.p-popover')))
    for (const row of await list.locator('li').all()) {
      expect((await stableBox(row)).height).toBeLessThanOrEqual(30)
      expect(await row.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true)
    }
    await page.keyboard.press('Escape')
    await expect(page.locator('.p-popover')).toBeHidden()
  }
})

// switchToChanges picks "Changes (n)" in the medium layout's side-panel
// switch and waits for the change list.
async function switchToChanges(page: Page): Promise<void> {
  await page.getByRole('tab', { name: /^Changes/ }).click()
  await page.locator('.changes-column .files li').first().waitFor()
}

test('FR-UX-08 the log is the left column; a switch above it picks Details or Changes to its right', async ({
  page,
}) => {
  await openMockRepo(page)

  const log = await stableBox(page.locator('.log-column'))
  const details = await stableBox(page.locator('.side-column'))
  expect(details.x).toBeGreaterThanOrEqual(log.x + log.width - 1)

  await switchToChanges(page)

  const changes = await stableBox(page.locator('.changes-column'))
  expect(Math.abs(changes.x - details.x)).toBeLessThanOrEqual(1)
  expect(changes.y + changes.height).toBeLessThanOrEqual(800)
})

test('FR-UX-09 a file row keeps its buttons on the name\'s line; "Stage all" stays one line', async ({
  page,
}) => {
  await openMockRepo(page)
  await switchToChanges(page)

  const rows = await page.locator('.changes-column .files li').all()
  expect(rows.length).toBeGreaterThan(0)
  for (const row of rows) {
    const nameY = await middle(row.locator('.path'))
    for (const button of await row.locator('.p-button').all()) {
      expect(Math.abs((await middle(button)) - nameY)).toBeLessThanOrEqual(4)
    }
  }

  const stageAll = page.getByRole('button', { name: 'Stage all', exact: true })
  // A wrapped two-line label roughly doubles the button's height; a single
  // line stays well under that at this app's compact 13px root font.
  expect((await stableBox(stageAll)).height).toBeLessThanOrEqual(30)
})

test('FR-UX-09 the diff toolbar shares one row', async ({ page }) => {
  await openDiffWindow(page)

  const tops = await Promise.all(
    (await page.locator('.diff-toolbar .p-button').all()).map(
      async (button) => (await stableBox(button)).y,
    ),
  )
  expect(Math.max(...tops) - Math.min(...tops)).toBeLessThanOrEqual(4)
})

test('FR-UX-09 commit-box checkboxes align with their label text', async ({ page }) => {
  await openMockRepo(page)
  await switchToChanges(page)
  await page.locator('.commit-box label').first().waitFor()

  for (const label of await page.locator('.commit-box label').all()) {
    const checkboxCenter = await middle(label.locator('input[type=checkbox]'))
    const labelBox = await stableBox(label)
    expect(checkboxCenter).toBeGreaterThanOrEqual(labelBox.y)
    expect(checkboxCenter).toBeLessThanOrEqual(labelBox.y + labelBox.height)
  }
})

test('FR-CHG-11 the commit box spaces its message field, checkbox rows and Commit button apart', async ({
  page,
}) => {
  await openMockRepo(page)
  await switchToChanges(page)
  await page.locator('.commit-box label').first().waitFor()

  const parts = [
    page.locator('.commit-box textarea'),
    ...(await page.locator('.commit-box label').all()),
    page.locator('[data-test="commit-submit"]'),
  ]
  const boxes = await Promise.all(parts.map((part) => stableBox(part)))
  for (let i = 1; i < boxes.length; i++) {
    expect(boxes[i]!.y).toBeGreaterThanOrEqual(boxes[i - 1]!.y + boxes[i - 1]!.height + 4)
  }
})

test('FR-DIFF-02 the diff toolbar stays in sight while a long diff scrolls', async ({ page }) => {
  await openDiffWindow(page)

  await page.locator('.diff-view').evaluate((el) => (el.scrollTop = el.scrollHeight))
  const view = await stableBox(page.locator('.diff-view'))
  const toolbar = await stableBox(page.locator('.diff-toolbar'))
  expect(toolbar.y).toBeGreaterThanOrEqual(view.y - 1)
  expect(toolbar.y + toolbar.height).toBeLessThanOrEqual(view.y + view.height)
})

test("FR-LOG-01 the HEAD row's four ref labels still leave the subject at least 80px wide", async ({
  page,
}) => {
  await openMockRepo(page)
  await page.locator('.row .ref-label').first().waitFor()

  const subjectBox = await stableBox(page.locator('.row').first().locator('.subject'))
  expect(subjectBox.width).toBeGreaterThanOrEqual(80)
})

test('FR-DIFF-07 in the diff window the diff fills the width beside the file list', async ({
  page,
}) => {
  await openDiffWindow(page)
  await page.locator('.cm-editor').first().waitFor()

  const diff = await stableBox(page.locator('.diff-view'))
  expect(diff.width).toBeGreaterThanOrEqual(1200 * 0.6)
})

test('FR-UX-04 opening Settings shows a dialog without moving the log view', async ({ page }) => {
  await openMockRepo(page)
  const beforeY = (await stableBox(page.locator('.log-column'))).y

  await page.locator('[data-test="open-settings"]').click()
  await expect(page.getByRole('dialog')).toBeVisible()

  const afterY = (await stableBox(page.locator('.log-column'))).y
  expect(afterY).toBe(beforeY)
})

test('FR-UX-09 the log filter bar keeps its four fields on one row, none overlapping', async ({
  page,
}) => {
  await openMockRepo(page)

  const fields = page.locator('.filter-bar > *')
  await expect(fields).toHaveCount(4)
  const boxes = await Promise.all((await fields.all()).map(stableBox))
  const tops = boxes.map((b) => b.y)
  expect(Math.max(...tops) - Math.min(...tops)).toBeLessThanOrEqual(4)
  expectNoOverlap(boxes)
})

test("FR-UX-09 the comparison header is inset from the panel's edge like the commit's own fields", async ({
  page,
}) => {
  await openMockRepo(page)
  const rows = page.locator('.row')
  await rows.nth(0).click()
  const subject = await stableBox(page.locator('.side-column .subject'))
  const panel = await stableBox(page.locator('.side-column'))
  expect(subject.x).toBeGreaterThan(panel.x + 4)

  await rows.nth(1).click({ modifiers: ['Control'] })
  const header = await stableBox(page.locator('.side-column .compare-view header'))
  expect(Math.abs(header.x - subject.x)).toBeLessThanOrEqual(1)
})
