import { createHash } from 'node:crypto'
import { defineConfig, devices } from '@playwright/test'

// Playwright drives the real Vue app against `vite --mode e2e`
// (e2e-mock-bindings.ts stands in for the Go core). Each checkout serves it
// on its own port, derived from its path, so runs in two worktrees at once
// never test each other's server.
const port = 10000 + (createHash('sha1').update(process.cwd()).digest().readUInt16BE(0) % 10000)
const baseURL = `http://127.0.0.1:${port}`

export default defineConfig({
  testDir: './e2e',
  globalSetup: './e2e/global-setup.ts',
  use: { baseURL },
  webServer: {
    command: `pnpm exec vite --mode e2e --port ${port} --strictPort`,
    url: baseURL,
    reuseExistingServer: !process.env.CI,
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
