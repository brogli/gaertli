import { test, expect } from '@playwright/test'

// happy-dom has no real layout or scrolling, so a component test cannot
// exercise the virtual list. This drives the rendered app instead: only
// the visible window loads up front, and scrolling pulls in further rows
// (window.__e2eLogRowsCalls, set by src/testing/e2e-mock-bindings.ts).
test('FR-LOG-09 scrolling the log loads further rows beyond the first screenful', async ({ page }) => {
  await page.goto('/?path=/mock/repo')
  await page.waitForSelector('.row')

  // Let the initial load settle: wait until no new LogRows call has fired
  // for a full polling interval.
  await page.waitForFunction(
    () => {
      const w = window as unknown as { __settleCheck?: number }
      const current = window.__e2eLogRowsCalls.length
      if (w.__settleCheck === current) return true
      w.__settleCheck = current
      return false
    },
    undefined,
    { polling: 200, timeout: 5000 },
  )

  const baseline = await page.evaluate(() => Math.max(...window.__e2eLogRowsCalls.map((c) => c.from)))

  await page.locator('.log-list').evaluate((el) => {
    el.scrollTop = el.scrollHeight
  })

  await page.waitForFunction((base) => window.__e2eLogRowsCalls.some((c) => c.from > base), baseline, {
    timeout: 5000,
  })

  const afterScroll = await page.evaluate(() => Math.max(...window.__e2eLogRowsCalls.map((c) => c.from)))
  expect(afterScroll).toBeGreaterThan(baseline)
})

// A streamed load must leave the list at its top, newest commit first: the
// virtualizer must not shift scrollTop while rows arrive.
test('FR-LOG-01 after a streamed load the list is still at the top with row 0 visible', async ({ page }) => {
  await page.goto('/?path=/mock/repo')
  await page.waitForSelector('.row')
  await page.waitForFunction(() => window.__e2eLogRowsCalls.length > 0)
  await page.waitForTimeout(1000) // let every progress step and row fetch land

  const state = await page.locator('.log-list').evaluate((el) => ({
    scrollTop: el.scrollTop,
    firstIndex: Math.min(...[...el.querySelectorAll('.row')].map((r) => Number((r as HTMLElement).dataset.index))),
    firstTop: el.querySelector('.row')!.getBoundingClientRect().top - el.getBoundingClientRect().top,
  }))
  expect(state.scrollTop).toBe(0)
  expect(state.firstIndex).toBe(0)
  expect(state.firstTop).toBe(0)
})
