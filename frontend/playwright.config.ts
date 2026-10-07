import { createHash } from 'node:crypto'
import { defineConfig, devices } from '@playwright/test'

// Playwright drives the real Vue app built with `vite build --mode e2e`
// (e2e-mock-bindings.ts stands in for the Go core) and served as the static
// bundle: a dev server would transform and serve hundreds of modules per
// page load, which on a small CI runner costs more than the tests. Each
// checkout serves it on its own port, derived from its path, so runs in two
// worktrees at once never test each other's server.
const port = 10000 + (createHash('sha1').update(process.cwd()).digest().readUInt16BE(0) % 10000)
const baseURL = `http://127.0.0.1:${port}`

// Tests share nothing (own browser context, the mock bindings live in the
// page), so every test runs in parallel; Playwright's default on CI is one
// worker.
export default defineConfig({
  testDir: './e2e',
  fullyParallel: true,
  workers: process.env.CI ? 4 : '50%',
  use: { baseURL },
  webServer: {
    command: `pnpm exec vite build --mode e2e --outDir dist-e2e && pnpm exec vite preview --outDir dist-e2e --port ${port} --strictPort`,
    url: baseURL,
    reuseExistingServer: !process.env.CI,
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
