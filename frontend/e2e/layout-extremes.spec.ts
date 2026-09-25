import { test, expect, type Page } from '@playwright/test'
import { expectNoOverlap, openMockRepo } from './helpers'

// happy-dom computes no layout, so these width extremes need a real
// browser: wide's changes view scrolling, and the toolbar at each layout's
// width down to the window's own minimum.

test('FR-CHG-11 wide (1600), 60 changed files: scrolling the change list never moves the Commit button', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1600, height: 900 })
  await openMockRepo(page, '&changedFiles=60')
  await page.waitForSelector('.changes-column .files li')

  const commitButton = page.locator('[data-test="commit-submit"]')
  const before = await commitButton.boundingBox()

  await page.locator('.changes-body').evaluate((el) => (el.scrollTop = el.scrollHeight))

  const after = await commitButton.boundingBox()
  expect(after).toEqual(before)
  expect(after!.y + after!.height).toBeLessThanOrEqual(900)
})

// assertToolbarFitsOneRow opens the mock repo (with query) at width x 800
// and checks every toolbar control shares one row, none overlapping
// another, with no horizontal overflow.
async function assertToolbarFitsOneRow(page: Page, width: number, query = ''): Promise<void> {
  await page.setViewportSize({ width, height: 800 })
  await openMockRepo(page, query)

  const toolbar = page.locator('.toolbar')
  const controls = await toolbar.locator('button, select').all()
  expect(controls.length).toBeGreaterThanOrEqual(7)
  const boxes = await Promise.all(controls.map((c) => c.boundingBox()))
  const known = boxes.filter((b): b is NonNullable<typeof b> => b !== null)
  // Middle, not top: a SplitButton's wrapper can be a shade taller than a
  // plain button, so it centers on the row instead of sharing its top edge.
  const middles = known.map((b) => b.y + b.height / 2)
  expect(Math.max(...middles) - Math.min(...middles)).toBeLessThanOrEqual(4)
  expect(await toolbar.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true)
  expectNoOverlap(known)
}

test('FR-UX-09 narrow (400 x 800, the window minimum): the toolbar stays one row, a branch without an upstream ("Push and set upstream")', async ({
  page,
}) => {
  await assertToolbarFitsOneRow(page, 400)
})

test('FR-UX-09 narrow (400 x 800, the window minimum): the toolbar stays one row, a branch with an upstream (Pull/Push)', async ({
  page,
}) => {
  await assertToolbarFitsOneRow(page, 400, '&upstream=1')
})

test('FR-UX-09 medium (900 x 800): the toolbar still stays one row, no overlap', async ({
  page,
}) => {
  await assertToolbarFitsOneRow(page, 900, '&upstream=1')
})

test('FR-UX-09 wide (1400 x 800): the toolbar still stays one row, no overlap', async ({
  page,
}) => {
  await assertToolbarFitsOneRow(page, 1400, '&upstream=1')
})
