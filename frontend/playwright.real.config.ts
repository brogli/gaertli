import { defineConfig, devices } from '@playwright/test'

// Playwright against the app built in Wails server mode (task e2e:real):
// the real Go core and git on scratch repos. Every test starts its own
// server and sandbox (e2e-real/fixtures.ts), so there is no webServer here.
export default defineConfig({
  testDir: './e2e-real',
  workers: 3,
  retries: 0,
  expect: { timeout: 10_000 },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
