import { defineConfig, devices } from '@playwright/test'

// Playwright drives the real Vue app against `vite --mode e2e`
// (e2e-mock-bindings.ts stands in for the Go core).
const baseURL = 'http://127.0.0.1:9245'

export default defineConfig({
  testDir: './e2e',
  use: { baseURL },
  webServer: {
    command: 'pnpm exec vite --mode e2e',
    url: baseURL,
    reuseExistingServer: !process.env.CI,
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
