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
