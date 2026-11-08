// Requirements: QA-COMP-04
import { test, expect } from './fixtures'

// The server's git reports 2.30.0 (sandbox.ts, reportedGitVersion) and
// otherwise is the real git.
test.use({ reportedGitVersion: '2.30.0' })

test('an outdated git warns once at start; after OK the app works as usual', async ({
  server,
  repos,
  newPage,
}) => {
  const url = `${server.url}/?path=${encodeURIComponent(repos.work)}`
  const page = await newPage()
  await page.goto(url)

  const warning = page.getByRole('dialog')
  await expect(warning).toContainText('git is out of date')
  await expect(warning).toContainText('The installed git is 2.30.0')
  await warning.getByRole('button', { name: 'OK' }).click()
  await expect(warning).toBeHidden()
  await expect(page.locator('.row').first()).toBeVisible()

  const second = await newPage()
  await second.goto(url)
  await expect(second.locator('.row').first()).toBeVisible()
  await expect(second.getByRole('dialog')).toHaveCount(0)
})
