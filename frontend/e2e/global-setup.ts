import { chromium, type FullConfig } from '@playwright/test'
import { mockDiffUrl, openMockRepo } from './helpers'

// globalSetup loads the main and the diff window once before any test. On a
// cold dev server (a fresh checkout, or other suites loading the machine)
// the first load transforms every module and may re-bundle dependencies and
// reload the page, long enough to take the first tests past their timeout.
export default async function globalSetup(config: FullConfig): Promise<void> {
  const baseURL = config.projects[0]?.use.baseURL
  if (!baseURL) throw new Error('playwright.config.ts sets no baseURL')
  const browser = await chromium.launch()
  const page = await browser.newPage({ baseURL })
  page.setDefaultTimeout(180_000)
  await openMockRepo(page)
  await page.goto(mockDiffUrl)
  await page.waitForSelector('.cm-editor')
  await browser.close()
}
