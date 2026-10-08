import { test, expect, type Locator } from '@playwright/test'
import { dragLogBorder, openMockRepo, overflows, switchToChanges } from './helpers'

// Layout at a 1200 x 800 window, with the mock's
// default log rows: row 0 (HEAD) carries four ref labels, and the changes
// view has a file in each group.
test.beforeEach(async ({ page }) => {
  await page.setViewportSize({ width: 1200, height: 800 })
})

test("FR-UX-09 the changes heading shows whole; Stage all and Unstage all share one row, each file row keeps its buttons on the name's line", async ({
  page,
}) => {
  await openMockRepo(page)
  await switchToChanges(page)

  const heading = page.locator('.changes-body header h2')
  expect(await overflows(heading)).toBe(false)
  const stageAll = await page.getByRole('button', { name: 'Stage all', exact: true }).boundingBox()
  const unstageAll = await page
    .getByRole('button', { name: 'Unstage all', exact: true })
    .boundingBox()
  expect(Math.abs(stageAll!.y - unstageAll!.y)).toBeLessThan(4)
  // A wrapped two-line label roughly doubles the button's height; a single
  // line stays well under that at this app's compact 13px root font.
  expect(stageAll!.height).toBeLessThanOrEqual(30)

  const centre = async (locator: Locator) => {
    const box = (await locator.boundingBox())!
    return box.y + box.height / 2
  }
  const rows = await page.locator('.changes-column .files li').all()
  expect(rows.length).toBeGreaterThan(0)
  for (const row of rows) {
    const nameY = await centre(row.locator('.path'))
    for (const button of await row.locator('.p-button').all()) {
      expect(Math.abs((await centre(button)) - nameY)).toBeLessThanOrEqual(4)
    }
  }
})

test('FR-LOG-01 a crowded log row shows two whole ref labels and "+N" naming them all', async ({
  page,
}) => {
  await openMockRepo(page)

  const row = page.locator('.row', { has: page.locator('.ref-label.head') })
  const labels = row.locator('.ref-label')
  await expect(labels).toHaveCount(2)
  for (const label of await labels.all()) {
    expect(await overflows(label)).toBe(false)
  }
  const more = row.locator('.ref-more')
  await expect(more).toHaveText(/^\+\d+$/)
  await expect(more).toHaveAttribute('title', /origin\/main/)
  expect((await row.locator('.subject').boundingBox())!.width).toBeGreaterThanOrEqual(80)
})

test('FR-UX-09 in a narrow log view the ref labels shrink to an ellipsis; "+N" and the hash stay inside the row', async ({
  page,
}) => {
  await openMockRepo(page)

  await dragLogBorder(page, 300)
  await expect
    .poll(async () => (await page.locator('.log-column').boundingBox())!.width)
    .toBeLessThan(380)
  const list = page.locator('.log-list')
  const row = page.locator('.row', { has: page.locator('.ref-label.head') })
  const rowRight = async () => {
    const box = (await row.boundingBox())!
    return box.x + box.width
  }
  const message = (await row.locator('.message').boundingBox())!
  const more = (await row.locator('.ref-more').boundingBox())!
  expect(more.x + more.width, '.ref-more within .message').toBeLessThanOrEqual(
    message.x + message.width,
  )
  for (const part of ['.ref-more', '.hash']) {
    const box = (await row.locator(part).boundingBox())!
    expect(box.x + box.width, part).toBeLessThanOrEqual(await rowRight())
  }
  expect(await overflows(list)).toBe(false)
  const head = row.locator('.ref-label.head')
  expect(await overflows(head)).toBe(true)
  await expect(head).toHaveAttribute('title', 'HEAD')
})
