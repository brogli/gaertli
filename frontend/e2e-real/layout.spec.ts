import type { Locator } from '@playwright/test'
import { test, expect } from './fixtures'
import { dragLogBorder } from './helpers'

// Layout at the 1200 x 800 window the fixtures open. The picker layout
// cases moved to frontend/e2e/layout-pickers.spec.ts, which needs no real
// git; these need a real repo's changes view and log.

// overflows: locator's content is wider than its box.
function overflows(locator: Locator): Promise<boolean> {
  return locator.evaluate((el) => el.scrollWidth > el.clientWidth)
}

test('FR-UX-09 the changes heading shows whole; Stage all and Unstage all share one row', async ({
  app,
}) => {
  const heading = app.locator('.changes-body header h2')
  expect(await overflows(heading)).toBe(false)
  const stageAll = await app.getByRole('button', { name: 'Stage all', exact: true }).boundingBox()
  const unstageAll = await app
    .getByRole('button', { name: 'Unstage all', exact: true })
    .boundingBox()
  expect(Math.abs(stageAll!.y - unstageAll!.y)).toBeLessThan(4)
})

test('FR-LOG-01 a crowded log row shows two whole ref labels and "+N" naming them all', async ({
  app,
}) => {
  const row = app.locator('.row', { has: app.locator('.ref-label.head') })
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
  app,
}) => {
  await dragLogBorder(app, 300)
  await expect
    .poll(async () => (await app.locator('.log-column').boundingBox())!.width)
    .toBeLessThan(380)
  const list = app.locator('.log-list')
  const row = app.locator('.row', { has: app.locator('.ref-label.head') })
  const rowRight = async () => {
    const box = (await row.boundingBox())!
    return box.x + box.width
  }
  for (const part of ['.ref-more', '.hash']) {
    const box = (await row.locator(part).boundingBox())!
    expect(box.x + box.width, part).toBeLessThanOrEqual(await rowRight())
  }
  expect(await overflows(list)).toBe(false)
  const head = row.locator('.ref-label.head')
  expect(await overflows(head)).toBe(true)
  await expect(head).toHaveAttribute('title', 'HEAD')
})
