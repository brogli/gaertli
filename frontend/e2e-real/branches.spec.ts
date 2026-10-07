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
  notice,
  openBranchPicker,
  pickerAction,
  pickerError,
  pickFromContextMenu,
  pickRefAction,
  refRow,
  rev,
} from './helpers'

// Branch create, checkout, reset, rename and delete through the UI, each
// checked against the real repo.

type ResetMode = 'Soft' | 'Mixed' | 'Hard' | 'Keep'

async function openReset(page: Page, subject: string): Promise<void> {
  await pickFromContextMenu(page, logRow(page, subject), 'Reset current branch to here…')
}

async function resetTo(page: Page, subject: string, mode: ResetMode): Promise<void> {
  await openReset(page, subject)
  const dialog = page.getByRole('dialog')
  await dialog.getByRole('radio', { name: new RegExp(`^${mode}\\b`) }).check()
  await dialog.getByRole('button', { name: 'Reset', exact: true }).click()
}

async function openNewBranch(page: Page, subject: string, name: string): Promise<void> {
  await pickFromContextMenu(page, logRow(page, subject), 'New branch from here…')
  await page.getByRole('dialog').getByLabel('Branch name').fill(name)
}

function branchButton(page: Page) {
  return page.locator('[data-test=branch-picker]')
}

// expectCurrent: the branch picker marks name as the current branch.
async function expectCurrent(page: Page, name: string): Promise<void> {
  await expect(branchButton(page)).toContainText(name)
  await openBranchPicker(page)
  await expect(refRow(page, name).locator('[data-test=current-marker]')).toBeVisible()
  await page.keyboard.press('Escape')
}

test('FR-BR-01 a new branch from a commit is checked out by default and shown as current', async ({
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

test('FR-BR-01 a new branch created without checkout appears in the branch picker and HEAD stays', async ({
  app,
  repos,
  git,
}) => {
  await openNewBranch(app, 'initial', 'later')
  const dialog = app.getByRole('dialog')
  await dialog.getByLabel('Check out after creation').uncheck()
  await dialog.getByRole('button', { name: 'Create' }).click()

  await expect.poll(() => git(repos.work, 'branch', '--list', 'later').trim()).toBe('later')
  await openBranchPicker(app)
  await expect(refRow(app, 'later')).toBeVisible()
  expect(rev(git, repos.work, 'later')).toBe(commitSha(git, repos.work, 'initial'))
  expect(currentBranch(git, repos.work)).toBe('main')
})

test('FR-BR-01 an invalid or taken branch name is refused with a message', async ({
  app,
  repos,
  git,
}) => {
  const branchesBefore = git(repos.work, 'branch', '--list')
  const dialog = app.getByRole('dialog')
  for (const [name, message] of [
    ['bad..name', /not a valid branch name/],
    ['main', /already exists/],
  ] as const) {
    await openNewBranch(app, 'initial', name)
    await dialog.getByRole('button', { name: 'Create' }).click()
    await expect(dialog.locator('.error-banner')).toContainText(message)
    await app.keyboard.press('Escape')
    await expect(dialog).toBeHidden()
  }
  expect(git(repos.work, 'branch', '--list')).toBe(branchesBefore)
})

test('FR-BR-09 a branch picked in the branch picker is checked out; a new branch from it starts at its commit', async ({
  app,
  repos,
  git,
}) => {
  await openBranchPicker(app)
  await refRow(app, 'feature').click()

  await expect.poll(() => currentBranch(git, repos.work)).toBe('feature')
  await expectCurrent(app, 'feature')

  await pickRefAction(app, 'feature', 'New branch from here…')
  const dialog = app.getByRole('dialog', { name: 'New branch' })
  await dialog.getByLabel('Branch name').fill('from-feature')
  await dialog.getByLabel('Check out after creation').uncheck()
  await dialog.getByRole('button', { name: 'Create' }).click()

  await expect
    .poll(() => git(repos.work, 'branch', '--list', 'from-feature').trim())
    .toBe('from-feature')
  expect(rev(git, repos.work, 'from-feature')).toBe(rev(git, repos.work, 'feature'))
})

test('FR-BR-09 "New branch…" in the branch picker creates a branch at the current branch and checks it out', async ({
  app,
  repos,
  git,
}) => {
  const head = rev(git, repos.work, 'HEAD')
  await openBranchPicker(app)
  await pickerAction(app, 'New branch…').click()
  const dialog = app.getByRole('dialog', { name: 'New branch' })
  await dialog.getByLabel('Branch name').fill('from-picker')
  await dialog.getByRole('button', { name: 'Create' }).click()

  await expect.poll(() => currentBranch(git, repos.work)).toBe('from-picker')
  expect(rev(git, repos.work, 'from-picker')).toBe(head)
})

test('FR-BR-01 "New branch…" starts the branch at the branch picked from the list', async ({
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

test('FR-BR-09 a remote-only branch checks out as a new branch tracking it', async ({
  openApp,
  repos,
  git,
}) => {
  git(repos.other, 'push', '-q', 'origin', 'HEAD:refs/heads/remote-only')
  git(repos.work, 'fetch', '-q')
  const app = await openApp(repos.work)

  await openBranchPicker(app)
  await refRow(app, 'origin/remote-only').click()

  await expect.poll(() => currentBranch(git, repos.work)).toBe('remote-only')
  expect(git(repos.work, 'rev-parse', '--abbrev-ref', 'remote-only@{upstream}').trim()).toBe(
    'origin/remote-only',
  )
})

test('FR-BR-09 a tag picked in the branch picker warns, then checks out detached at its commit', async ({
  app,
  repos,
  git,
}) => {
  await openBranchPicker(app)
  await refRow(app, 'v1').click()
  const dialog = app.getByRole('dialog', { name: 'Check out commit' })
  await expect(dialog).toContainText('detaches HEAD')
  await dialog.getByRole('button', { name: 'Check out' }).click()

  await expect.poll(() => rev(git, repos.work, 'HEAD')).toBe(rev(git, repos.work, 'v1^{commit}'))
  expect(currentBranch(git, repos.work)).toBe('')
  await expect(branchButton(app)).toContainText('detached')
})

test("FR-BR-05 a checkout blocked by local changes shows git's reason and nothing changes", async ({
  app,
  repos,
  git,
}) => {
  const file = path.join(repos.work, 'conflict.txt')
  fs.writeFileSync(file, 'local edit\n')

  await openBranchPicker(app)
  await refRow(app, 'feature').click()

  await expect(pickerError(app)).toContainText(/would be overwritten by checkout/)
  expect(currentBranch(git, repos.work)).toBe('main')
  expect(fs.readFileSync(file, 'utf8')).toBe('local edit\n')
})

test('FR-BR-08 checking out a commit warns first, then shows detached HEAD and offers a branch there', async ({
  app,
  repos,
  git,
}) => {
  const target = commitSha(git, repos.work, 'docs and func b')
  await pickFromContextMenu(app, logRow(app, 'docs and func b'), 'Check out commit')
  const dialog = app.getByRole('dialog')
  await expect(dialog).toContainText('Commits made in this state belong to no branch')
  await dialog.getByRole('button', { name: 'Check out' }).click()

  await expect.poll(() => rev(git, repos.work, 'HEAD')).toBe(target)
  expect(currentBranch(git, repos.work)).toBe('')
  await expect(branchButton(app)).toContainText(`${target.slice(0, 7)} (detached)`)

  await app.getByRole('button', { name: 'Create branch here' }).click()
  await dialog.getByLabel('Branch name').fill('rescued')
  await dialog.getByRole('button', { name: 'Create' }).click()
  await expect.poll(() => currentBranch(git, repos.work)).toBe('rescued')
  expect(rev(git, repos.work, 'rescued')).toBe(target)
})

test('FR-BR-02 a soft reset moves the branch and keeps the changes staged', async ({
  app,
  repos,
  git,
}) => {
  await resetTo(app, 'docs and func b', 'Soft')

  await expect
    .poll(() => rev(git, repos.work, 'HEAD'))
    .toBe(commitSha(git, repos.work, 'docs and func b'))
  expect(git(repos.work, 'diff', '--cached', '--name-only').split('\n')).toEqual(
    expect.arrayContaining(['conflict.txt', 'side.txt']),
  )
})

test('FR-BR-02 a mixed reset moves the branch and empties the index, keeping the files', async ({
  app,
  repos,
  git,
}) => {
  await resetTo(app, 'add logo', 'Mixed')

  await expect.poll(() => rev(git, repos.work, 'HEAD')).toBe(commitSha(git, repos.work, 'add logo'))
  expect(git(repos.work, 'diff', '--cached', '--name-only')).toBe('')
  expect(fs.readFileSync(path.join(repos.work, 'side.txt'), 'utf8')).toBe('side file\n')
})

test('FR-BR-03 a hard reset asks first, names how many files are lost, then discards them', async ({
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

  test('FR-BR-04 a hard reset drops the undone commits from the log view at once, filtered or not, without another action', async ({
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

test("FR-UX-01 a reset git refuses during a merge shows git's message and changes nothing", async ({
  app,
  repos,
  git,
}) => {
  expect(() => git(repos.work, 'merge', 'feature')).toThrow()
  const head = rev(git, repos.work, 'HEAD')

  await resetTo(app, 'add logo', 'Soft')

  await expect(app.getByRole('dialog').locator('.error-banner')).toContainText(
    /cannot do a soft reset in the middle of a merge/i,
  )
  expect(rev(git, repos.work, 'HEAD')).toBe(head)
})

test('FR-BR-02 a keep reset refuses when a local change would be lost and nothing moves', async ({
  app,
  repos,
  git,
}) => {
  const file = path.join(repos.work, 'conflict.txt')
  fs.appendFileSync(file, 'local change\n')
  const head = rev(git, repos.work, 'HEAD')

  await resetTo(app, 'initial', 'Keep')

  await expect(app.getByRole('dialog').locator('.error-banner')).toContainText('conflict.txt')
  expect(rev(git, repos.work, 'HEAD')).toBe(head)
  expect(fs.readFileSync(file, 'utf8')).toContain('local change')
})

test('FR-BR-02 a keep reset on a clean tree moves the branch like hard, without confirmation', async ({
  app,
  repos,
  git,
}) => {
  await resetTo(app, 'add logo', 'Keep')

  await expect(app.getByRole('dialog')).toBeHidden()
  expect(rev(git, repos.work, 'HEAD')).toBe(commitSha(git, repos.work, 'add logo'))
  expect(git(repos.work, 'status', '--porcelain', '--untracked-files=no')).toBe('')
  await expect(confirmDialog(app)).toBeHidden()
})

test('FR-BR-06 renaming refuses an invalid or taken name with a message, then renames', async ({
  app,
  repos,
  git,
}) => {
  const dialog = app.getByRole('dialog')
  for (const [name, message] of [
    ['-x', /not a valid branch name/],
    ['feature', /already exists/],
  ] as const) {
    await pickRefAction(app, 'B', 'Rename…')
    await dialog.getByLabel('Branch name').fill(name)
    await dialog.getByRole('button', { name: 'Rename' }).click()
    await expect(dialog.locator('.error-banner')).toContainText(message)
    await app.keyboard.press('Escape')
    await expect(dialog).toBeHidden()
  }
  expect(git(repos.work, 'branch', '--list', 'B').trim()).toBe('B')

  await pickRefAction(app, 'B', 'Rename…')
  await dialog.getByLabel('Branch name').fill('B2')
  await dialog.getByRole('button', { name: 'Rename' }).click()

  await expect.poll(() => git(repos.work, 'branch', '--list', 'B', 'B2').trim()).toBe('B2')
  await openBranchPicker(app)
  await expect(refRow(app, 'B2')).toBeVisible()
})

test('FR-BR-06 a merged branch is deleted without confirmation', async ({ app, repos, git }) => {
  await pickRefAction(app, 'B', 'Delete')

  await expect.poll(() => git(repos.work, 'branch', '--list', 'B')).toBe('')
  await expect(confirmDialog(app)).toBeHidden()
})

test("FR-UX-03 deleting an unmerged branch asks first, naming git's reason", async ({
  openApp,
  repos,
  git,
}) => {
  const onlyHere = git(repos.work, 'commit-tree', 'HEAD^{tree}', '-p', 'HEAD', '-m', 'only here')
  git(repos.work, 'branch', 'unmerged', onlyHere.trim())
  const app = await openApp(repos.work)

  await pickRefAction(app, 'unmerged', 'Delete')

  await expect(confirmDialog(app)).toContainText(/not fully merged/)
  expect(git(repos.work, 'branch', '--list', 'unmerged').trim()).toBe('unmerged')
  await clickConfirm(app)
  await expect.poll(() => git(repos.work, 'branch', '--list', 'unmerged')).toBe('')
})

test("FR-BR-06 a branch checked out in another worktree is not deleted: git's refusal, no forced retry", async ({
  app,
  repos,
  git,
}) => {
  await pickRefAction(app, 'side', 'Delete')

  await expect(notice(app)).toContainText(repos.work2)
  await expect(confirmDialog(app)).toBeHidden()
  expect(git(repos.work, 'branch', '--list', 'side').trim()).toContain('side')
})
