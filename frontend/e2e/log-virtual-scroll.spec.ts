import { test, expect } from '@playwright/test'

// FR-LOG-09, TR-31 (moved from spec 10): a component test runs in happy-dom,
// which has no real layout/scroll, so it cannot exercise the virtual list's
// actual behavior. This drives the real, rendered app instead: only the
// visible window loads up front, and scrolling past it pulls in further,
// higher-index rows (window.__e2eLogRowsCalls, set by
// src/testing/e2e-mock-bindings.ts).
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

// A streamed load must leave the list at its top, newest commit first
// (FR-LOG-01): rows are fixed-height, so the virtualizer never remeasures
// and never shifts scrollTop while rows arrive (the drift left the list
// scrolled, or blank, after every load of a real repo).
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
