import { defineConfig, devices } from '@playwright/test'

// Playwright against the app built in Wails server mode (task e2e:real):
// the real Go core and git on scratch repos. Every test starts its own
// server and sandbox (e2e-real/fixtures.ts), so there is no webServer here.
export default defineConfig({
  // Each suite keeps its own folders, so the second suite of a CI job
  // doesn't clear the first one's failure files.
  outputDir: './test-results/real',
  reporter: [['list'], ['html', { open: 'never', outputFolder: './playwright-report/real' }]],
  testDir: './e2e-real',
  workers: 3,
  retries: 0,
  expect: { timeout: 10_000 },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
