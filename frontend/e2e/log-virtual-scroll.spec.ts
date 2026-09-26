import { test, expect, type Page } from '@playwright/test'
import { openMockRepo } from './helpers'

// happy-dom has no real layout or scrolling, so a component test cannot
// exercise the virtual list. This drives the rendered app instead: only
// the visible window loads up front, and scrolling pulls in further rows
// (window.__e2eLogRowsCalls, set by src/testing/e2e-mock-bindings.ts).

declare global {
  interface Window {
    __settleCheck?: number
  }
}

// waitForLogRowsSettle waits until no new LogRows call has fired for a full
// polling interval.
async function waitForLogRowsSettle(page: Page): Promise<void> {
  await page.waitForFunction(
    () => {
      const current = window.__e2eLogRowsCalls.length
      if (window.__settleCheck === current) return true
      window.__settleCheck = current
      return false
    },
    undefined,
    { polling: 200, timeout: 5000 },
  )
}

test('FR-LOG-09 scrolling the log loads further rows beyond the first screenful', async ({
  page,
}) => {
  await openMockRepo(page)
  await waitForLogRowsSettle(page)

  const furthestFrom = () =>
    page.evaluate(() => Math.max(...window.__e2eLogRowsCalls.map((c) => c.from)))
  const baseline = await furthestFrom()

  await page.locator('.log-list').evaluate((el) => (el.scrollTop = el.scrollHeight))

  await expect.poll(furthestFrom).toBeGreaterThan(baseline)
})

// A streamed load must leave the list at its top, newest commit first: the
// virtualizer must not shift scrollTop while rows arrive.
test('FR-LOG-01 after a streamed load the list is still at the top with row 0 visible', async ({
  page,
}) => {
  await openMockRepo(page)
  await page.waitForFunction(() => window.__e2eLogRowsCalls.length > 0)
  await waitForLogRowsSettle(page)

  const state = await page.locator('.log-list').evaluate((el) => ({
    scrollTop: el.scrollTop,
    firstIndex: Math.min(
      ...[...el.querySelectorAll<HTMLElement>('.row')].map((r) => Number(r.dataset.index)),
    ),
    firstTop:
      el.querySelector('.row')!.getBoundingClientRect().top - el.getBoundingClientRect().top,
  }))
  expect(state.scrollTop).toBe(0)
  expect(state.firstIndex).toBe(0)
  expect(state.firstTop).toBe(0)
})
