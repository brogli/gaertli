// Requirements: FR-BR-01, FR-BR-03, FR-BR-04, FR-BR-06, FR-BR-09
import fs from 'node:fs'
import path from 'node:path'
import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'
import {
  clickConfirm,
  commitSha,
  confirmDialog,
  currentBranch,
  logRow,
  openBranchPicker,
  pickerAction,
  pickFromContextMenu,
  pickRefAction,
  refRow,
  rev,
} from './helpers'

// Branch create, checkout, reset, rename and delete through the UI, each
// checked against the real repo.

type ResetMode = 'Soft' | 'Mixed' | 'Hard' | 'Keep'

async function resetTo(page: Page, subject: string, mode: ResetMode): Promise<void> {
  await pickFromContextMenu(page, logRow(page, subject), 'Reset current branch to here…')
  const dialog = page.getByRole('dialog')
  await dialog.getByRole('radio', { name: new RegExp(`^${mode}\\b`) }).check()
  await dialog.getByRole('button', { name: 'Reset', exact: true }).click()
}

async function openNewBranch(page: Page, subject: string, name: string): Promise<void> {
  await pickFromContextMenu(page, logRow(page, subject), 'New branch from here…')
  await page.getByRole('dialog').getByLabel('Branch name').fill(name)
}

// expectCurrent: the branch picker marks name as the current branch.
async function expectCurrent(page: Page, name: string): Promise<void> {
  await expect(page.locator('[data-test=branch-picker]')).toContainText(name)
  await openBranchPicker(page)
  await expect(refRow(page, name).locator('[data-test=current-marker]')).toBeVisible()
  await page.keyboard.press('Escape')
}

test('a new branch from a commit is checked out by default and shown as current', async ({
  app,
  repos,
  git,
}) => {
  await openNewBranch(app, 'add logo', 'fresh')
  const dialog = app.getByRole('dialog')
  await expect(dialog.getByLabel('Check out after creation')).toBeChecked()
  await dialog.getByRole('button', { name: 'Create' }).click()

  await expect.poll(() => currentBranch(git, repos.work)).toBe('fresh')
  expect(rev(git, repos.work, 'fresh')).toBe(commitSha(git, repos.work, 'add logo'))
  await expectCurrent(app, 'fresh')
})

test('a branch picked in the branch picker is checked out', async ({ app, repos, git }) => {
  await openBranchPicker(app)
  await refRow(app, 'feature').click()

  await expect.poll(() => currentBranch(git, repos.work)).toBe('feature')
  await expectCurrent(app, 'feature')
})

test('"New branch…" starts the branch at the branch picked from the list', async ({
  app,
  repos,
  git,
}) => {
  await openBranchPicker(app)
  await pickerAction(app, 'New branch…').click()
  const dialog = app.getByRole('dialog', { name: 'New branch' })
  await dialog.getByLabel('Branch name').fill('from-list')
  await dialog.locator('[data-test=start-branch]').click()
  await app.getByRole('option', { name: 'feature', exact: true }).click()
  await dialog.getByRole('button', { name: 'Create' }).click()

  await expect.poll(() => currentBranch(git, repos.work)).toBe('from-list')
  expect(rev(git, repos.work, 'from-list')).toBe(rev(git, repos.work, 'feature'))
})

test('a hard reset asks first, names how many files are lost, then discards them', async ({
  app,
  repos,
  git,
}) => {
  fs.appendFileSync(path.join(repos.work, 'hunks.txt'), 'uncommitted\n')
  fs.appendFileSync(path.join(repos.work, 'nonl.txt'), '\nuncommitted\n')

  await resetTo(app, 'initial', 'Hard')

  await expect(confirmDialog(app)).toContainText('Discard 2 uncommitted changes')
  expect(git(repos.work, 'status', '--porcelain', '--untracked-files=no')).not.toBe('')
  await clickConfirm(app)

  await expect.poll(() => rev(git, repos.work, 'HEAD')).toBe(commitSha(git, repos.work, 'initial'))
  expect(git(repos.work, 'status', '--porcelain', '--untracked-files=no')).toBe('')
  await expect(app.getByRole('dialog')).toBeHidden()
})

test.describe('a repo with no remote', () => {
  test.use({ open: ['scratch'] })

  test('a hard reset drops the undone commits from the log view at once, filtered or not, without another action', async ({
    openApp,
    repos,
  }) => {
    const app = await openApp(repos.scratch)
    await expect(logRow(app, 'commit C')).toBeVisible()

    // A branch and a path filter, both matching every commit here: the
    // filtered view is what the reset must refresh too.
    await app.locator('.filter-bar .p-multiselect').click()
    await app.getByRole('option', { name: 'main', exact: true }).click()
    await app.keyboard.press('Escape')
    await app.getByPlaceholder('Path').fill('file.txt')
    await expect(app.locator('[data-test=clear-path]')).toBeVisible()
    await expect(logRow(app, 'commit C')).toBeVisible()

    await resetTo(app, 'commit A', 'Hard')
    await clickConfirm(app)

    await expect(logRow(app, 'commit B')).toBeHidden()
    await expect(logRow(app, 'commit C')).toBeHidden()
  })
})

test('a renamed branch shows under its new name in the branch picker', async ({
  app,
  repos,
  git,
}) => {
  const dialog = app.getByRole('dialog')
  await pickRefAction(app, 'B', 'Rename…')
  await dialog.getByLabel('Branch name').fill('B2')
  await dialog.getByRole('button', { name: 'Rename' }).click()

  await expect.poll(() => git(repos.work, 'branch', '--list', 'B', 'B2').trim()).toBe('B2')
  await openBranchPicker(app)
  await expect(refRow(app, 'B2')).toBeVisible()
})
