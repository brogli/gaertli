import { test, expect, type Page } from '@playwright/test'
import { openMockRepo } from './helpers'

// happy-dom computes no layout, so whether the log's columns line up, and
// whether a column too narrow to read hides instead of shrinking to a
// sliver, needs a real browser. The fixture rows differ in lane span, ref
// labels, author, date and short hash length (src/testing/fixtures/log-rows.ts).

const PARTS = ['.message', '.subject', '.author', '.date', '.hash']

// expectColumnsLineUp: each of parts starts at the same x on every loaded
// row in view.
async function expectColumnsLineUp(page: Page, parts: string[] = PARTS): Promise<void> {
  const lefts = await page.locator('.log-list').evaluate((list, parts) => {
    const view = list.getBoundingClientRect()
    const rows = [...list.querySelectorAll('.row')].filter((row) => {
      const box = row.getBoundingClientRect()
      return row.childElementCount > 0 && box.top >= view.top && box.bottom <= view.bottom
    })
    return parts.map((part) =>
      rows.map((row) => Math.round(row.querySelector(part)!.getBoundingClientRect().left)),
    )
  }, parts)

  expect(lefts[0]!.length).toBeGreaterThan(10)
  lefts.forEach((xs, i) => expect(new Set(xs), parts[i]).toHaveProperty('size', 1))
}

test('FR-LOG-01 at 1200 px the subject, author, date and hash start at the same x on every row, a selected row included', async ({
  page,
}) => {
  await page.setViewportSize({ width: 1200, height: 800 })
  await openMockRepo(page)
  await page.locator('.row .hash').first().waitFor()

  await page.locator('.row').nth(3).click()
  await expect(page.locator('.row.selected')).toHaveCount(1)

  await expectColumnsLineUp(page)
})

test('FR-UX-09 at 420 px the author column hides instead of shrinking to a sliver; the rest still line up', async ({
  page,
}) => {
  await page.setViewportSize({ width: 420, height: 800 })
  await openMockRepo(page)
  await page.locator('.row .hash').first().waitFor()

  await expect(page.locator('.row .author').first()).toBeHidden()
  await expectColumnsLineUp(page, ['.message', '.subject', '.date', '.hash'])
})

test('FR-LOG-01 FR-CMP-05 compared rows keep their content where it is', async ({ page }) => {
  await page.setViewportSize({ width: 1200, height: 800 })
  await openMockRepo(page)
  await page.locator('.row .hash').first().waitFor()
  const rows = page.locator('.row')
  const subjectX = async () => (await rows.nth(1).locator('.subject').boundingBox())!.x

  const before = await subjectX()
  await rows.nth(1).click({ modifiers: ['Control'] })
  await rows.nth(4).click({ modifiers: ['Control'] })
  await expect(page.locator('.row.compare-from, .row.compare-to')).toHaveCount(2)

  expect(await subjectX()).toBe(before)
  await expectColumnsLineUp(page)
})

test('FR-LOG-01 in a narrow window the HEAD row\'s "+N" stays inside its message column', async ({
  page,
}) => {
  await page.setViewportSize({ width: 400, height: 800 })
  await openMockRepo(page)
  const head = page.locator('.row', { has: page.locator('.ref-label.head') })
  await expect(head.locator('.ref-more')).toBeVisible()

  const message = (await head.locator('.message').boundingBox())!
  const more = (await head.locator('.ref-more').boundingBox())!
  expect(more.x + more.width).toBeLessThanOrEqual(message.x + message.width)
})

test('FR-LOG-02 the graph draws its branch lines in more than one colour, none in the text colour', async ({
  page,
}) => {
  await openMockRepo(page)
  await page.locator('.graph-cell circle').first().waitFor()

  const colours = await page.evaluate(() => ({
    text: getComputedStyle(document.body).color,
    lanes: [...document.querySelectorAll('.graph-cell circle, .graph-cell path')].map(
      (el) => getComputedStyle(el).color,
    ),
  }))

  expect(new Set(colours.lanes).size).toBeGreaterThan(1)
  expect(colours.lanes).not.toContain(colours.text)
})
