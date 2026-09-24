import { test, expect } from './fixtures'
import { openRepoPicker, openSettings, pickerAction } from './helpers'

// The repo picker on work's repo: naming, the start page with no selection,
// and Open folder/Open WSL folder. Switching repos is covered by the
// component tests.

test('FR-WT-05 with no worktree selected the window shows the repo list as its page', async ({
  page,
  server,
  repos,
}) => {
  await page.goto(server.url)

  await expect(
    page.locator(`.start-page [data-test=repo-row][title="${repos.work}"]`),
  ).toBeVisible()
  await expect(pickerAction(page, 'Open folder…')).toBeVisible()
})

test('FR-UX-04 Settings opens as a dialog from the gear, also with no worktree selected', async ({
  page,
  server,
}) => {
  await page.goto(server.url)

  await openSettings(page)
})

test('FR-REPO-07 a host without WSL offers no "Open WSL folder…", and Settings shows no error about it', async ({
  app,
}) => {
  await openRepoPicker(app)
  await expect(pickerAction(app, 'Open folder…')).toBeVisible()
  await expect(pickerAction(app, 'Open WSL folder…')).toHaveCount(0)
  await app.keyboard.press('Escape')

  const dialog = await openSettings(app)
  await expect(dialog.locator('form')).toBeVisible()
  await expect(dialog.locator('.error-banner')).toHaveCount(0)
})

test("FR-WT-13 the button names this window's repo by its main worktree's folder", async ({
  app,
}) => {
  await expect(app.locator('[data-test=repo-picker]')).toContainText('work')
  await openRepoPicker(app)

  await expect(app.locator('.p-popover [data-test=repo-row]')).toHaveCount(1)
})
