import { test, expect, type Locator } from '@playwright/test'

// happy-dom computes no layout, so these checks (widths, wrapping, shared
// rows) need a real browser. One shared 1200 x 800 window: the size the
// "before" screenshots that drove this spec were taken at.
test.beforeEach(async ({ page }) => {
  await page.setViewportSize({ width: 1200, height: 800 })
})

// stableBox polls for a non-null bounding box: right after a route or
// selection change, a locator can briefly report null before its layout is
// ready. boundingBox() itself returns {x, y, width, height}, not
// {top, left, ...}.
async function stableBox(locator: Locator) {
  let box: Awaited<ReturnType<Locator['boundingBox']>> = null
  await expect
    .poll(async () => {
      box = await locator.boundingBox()
      return box !== null
    })
    .toBe(true)
  return box!
}

test('FR-UX-09 a file row keeps its buttons on the name\'s line; "Stage all" stays one line', async ({
  page,
}) => {
  await page.goto('/?path=/mock/repo')
  await page.waitForSelector('.changes-column .files li')

  const rows = await page.locator('.changes-column .files li').all()
  expect(rows.length).toBeGreaterThan(0)
  for (const row of rows) {
    const nameY = (await stableBox(row.locator('.path'))).y
    for (const button of await row.locator('.glyph-btn').all()) {
      const buttonY = (await stableBox(button)).y
      expect(Math.abs(buttonY - nameY)).toBeLessThanOrEqual(4)
    }
  }

  const stageAll = page.getByRole('button', { name: 'Stage all', exact: true })
  // A wrapped two-line label roughly doubles the button's height; a single
  // line stays well under that at this app's compact 13px root font.
  expect((await stableBox(stageAll)).height).toBeLessThanOrEqual(30)
})

test('FR-UX-09 the diff toolbar shares one row; commit-box checkboxes align with their label text', async ({
  page,
}) => {
  await page.goto('/?path=/mock/repo')
  await page.waitForSelector('.row')
  await page.locator('.row').first().click()
  await page.waitForSelector('.side-column .files li')
  await page.locator('.side-column .files li').first().click()
  await page.waitForSelector('.side-column .diff-toolbar .glyph-btn')

  const tops = await Promise.all(
    (await page.locator('.side-column .diff-toolbar .glyph-btn').all()).map(
      async (button) => (await stableBox(button)).y,
    ),
  )
  expect(Math.max(...tops) - Math.min(...tops)).toBeLessThanOrEqual(4)

  for (const label of await page.locator('.commit-box label').all()) {
    const checkboxBox = await stableBox(label.locator('input[type=checkbox]'))
    const labelBox = await stableBox(label)
    const checkboxCenter = checkboxBox.y + checkboxBox.height / 2
    expect(checkboxCenter).toBeGreaterThanOrEqual(labelBox.y)
    expect(checkboxCenter).toBeLessThanOrEqual(labelBox.y + labelBox.height)
  }
})

test('FR-UX-07 with a file diff open, the commit details stay in sight above it', async ({
  page,
}) => {
  await page.goto('/?path=/mock/repo')
  await page.waitForSelector('.row')
  await page.locator('.row').first().click()
  await page.waitForSelector('.side-column .files li')
  await page.locator('.side-column .files li').first().click()
  await page.waitForSelector('.side-column .diff-toolbar')

  const subject = await stableBox(page.locator('.commit-details .subject'))
  const toolbar = await stableBox(page.locator('.side-column .diff-toolbar'))
  expect(subject.height).toBeGreaterThan(10)
  expect(subject.y + subject.height).toBeLessThanOrEqual(toolbar.y)
})

test('FR-DIFF-02 the diff toolbar stays in sight while a long diff scrolls', async ({ page }) => {
  await page.goto('/?path=/mock/repo')
  await page.waitForSelector('.row')
  await page.locator('.row').first().click()
  await page.waitForSelector('.side-column .files li')
  await page.locator('.side-column .files li').first().click()
  await page.waitForSelector('.side-column .diff-toolbar')

  await page.locator('.side-column').evaluate((el) => (el.scrollTop = el.scrollHeight))
  const column = await stableBox(page.locator('.side-column'))
  const toolbar = await stableBox(page.locator('.side-column .diff-toolbar'))
  expect(toolbar.y).toBeGreaterThanOrEqual(column.y - 1)
  expect(toolbar.y + toolbar.height).toBeLessThanOrEqual(column.y + column.height)
})

test("FR-LOG-01 the HEAD row's four ref labels still leave the subject at least 80px wide", async ({
  page,
}) => {
  await page.goto('/?path=/mock/repo')
  await page.waitForSelector('.row .ref-label')

  const subjectBox = await stableBox(page.locator('.row').first().locator('.subject'))
  expect(subjectBox.width).toBeGreaterThanOrEqual(80)
})

test('FR-CMP-02 in the compare diff window the diff fills the width beside the file list', async ({
  page,
}) => {
  await page.goto(
    '/?view=compare-diff&wt=/mock/repo&from=aaa&to=bbb&file=src/components/staged-one.ts',
  )
  await page.waitForSelector('.cm-editor')

  const diff = await stableBox(page.locator('.diff-view'))
  expect(diff.width).toBeGreaterThanOrEqual(1200 * 0.6)
})

test('FR-UX-04 opening Settings shows a dialog without moving the log column', async ({ page }) => {
  await page.goto('/?path=/mock/repo')
  await page.waitForSelector('.log-column')
  const beforeY = (await stableBox(page.locator('.log-column'))).y

  await page.click('[data-test="open-settings"]')
  await expect(page.getByRole('dialog')).toBeVisible()

  const afterY = (await stableBox(page.locator('.log-column'))).y
  expect(afterY).toBe(beforeY)
})
