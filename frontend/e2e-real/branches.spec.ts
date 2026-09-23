import fs from 'node:fs'
import path from 'node:path'
import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'
import {
  commitSha,
  confirmDialog,
  contextMenuItems,
  logRow,
  pickFromContextMenu,
  refNode,
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
  await dialog.getByRole('radio', { name: new RegExp(`^${mode} — `) }).check()
  await dialog.getByRole('button', { name: 'Reset', exact: true }).click()
}

async function openNewBranch(page: Page, subject: string, name: string): Promise<void> {
  await pickFromContextMenu(page, logRow(page, subject), 'New branch from here…')
  await page.getByRole('dialog').getByPlaceholder('Branch name').fill(name)
}

function branchSummary(page: Page) {
  return page.locator('.branch-summary')
}

function sidebarError(page: Page) {
  return page.locator('.refs-sidebar .error-banner')
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

  await expect.poll(() => git(repos.work, 'branch', '--show-current').trim()).toBe('fresh')
  expect(git(repos.work, 'rev-parse', 'fresh').trim()).toBe(commitSha(git, repos.work, 'add logo'))
  await expect(refNode(app, 'fresh').locator('[data-test=current-marker]')).toBeVisible()
  await expect(branchSummary(app)).toContainText('fresh')
})

test('FR-BR-01 a new branch created without checkout appears in the sidebar and HEAD stays', async ({
  app,
  repos,
  git,
}) => {
  await openNewBranch(app, 'initial', 'later')
  const dialog = app.getByRole('dialog')
  await dialog.getByLabel('Check out after creation').uncheck()
  await dialog.getByRole('button', { name: 'Create' }).click()

  await expect(refNode(app, 'later')).toBeVisible()
  expect(git(repos.work, 'rev-parse', 'later').trim()).toBe(commitSha(git, repos.work, 'initial'))
  expect(git(repos.work, 'branch', '--show-current').trim()).toBe('main')
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

test('FR-BR-05 Checkout in the refs sidebar switches the worktree to the branch', async ({
  app,
  repos,
  git,
}) => {
  await pickFromContextMenu(app, refNode(app, 'feature'), 'Checkout')

  await expect.poll(() => git(repos.work, 'branch', '--show-current').trim()).toBe('feature')
  await expect(refNode(app, 'feature').locator('[data-test=current-marker]')).toBeVisible()
  await expect(branchSummary(app)).toContainText('feature')
})

test("FR-BR-05 a checkout blocked by local changes shows git's reason and nothing changes", async ({
  app,
  repos,
  git,
}) => {
  const file = path.join(repos.work, 'conflict.txt')
  fs.writeFileSync(file, 'local edit\n')

  await pickFromContextMenu(app, refNode(app, 'feature'), 'Checkout')

  await expect(sidebarError(app)).toContainText(/would be overwritten by checkout/)
  expect(git(repos.work, 'branch', '--show-current').trim()).toBe('main')
  expect(fs.readFileSync(file, 'utf8')).toBe('local edit\n')
})

test("FR-BR-05 checking out a branch in use by another worktree shows git's refusal naming it", async ({
  app,
  repos,
  git,
}) => {
  await pickFromContextMenu(app, refNode(app, 'side'), 'Checkout')

  await expect(sidebarError(app)).toContainText(repos.work2)
  expect(git(repos.work, 'branch', '--show-current').trim()).toBe('main')
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

  await expect.poll(() => git(repos.work, 'rev-parse', 'HEAD').trim()).toBe(target)
  expect(git(repos.work, 'branch', '--show-current').trim()).toBe('')
  await expect(branchSummary(app)).toContainText('detached HEAD')

  await app.getByRole('button', { name: 'Create branch here' }).click()
  await dialog.getByPlaceholder('Branch name').fill('rescued')
  await dialog.getByRole('button', { name: 'Create' }).click()
  await expect.poll(() => git(repos.work, 'branch', '--show-current').trim()).toBe('rescued')
  expect(git(repos.work, 'rev-parse', 'rescued').trim()).toBe(target)
})

test('FR-BR-02 the reset dialog names the branch and the commit and explains each mode', async ({
  app,
}) => {
  await openReset(app, 'docs and func b')

  const dialog = app.getByRole('dialog')
  await expect(dialog).toContainText('Reset main to docs and func b')
  for (const mode of ['Soft', 'Mixed', 'Hard', 'Keep']) {
    await expect(dialog.getByRole('radio', { name: new RegExp(`^${mode} — .+`) })).toBeVisible()
  }
})

test('FR-BR-02 a soft reset moves the branch and keeps the changes staged', async ({
  app,
  repos,
  git,
}) => {
  await resetTo(app, 'docs and func b', 'Soft')

  await expect
    .poll(() => git(repos.work, 'rev-parse', 'HEAD').trim())
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

  await expect
    .poll(() => git(repos.work, 'rev-parse', 'HEAD').trim())
    .toBe(commitSha(git, repos.work, 'add logo'))
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
  await confirmDialog(app).getByRole('button', { name: 'Confirm' }).click()

  await expect
    .poll(() => git(repos.work, 'rev-parse', 'HEAD').trim())
    .toBe(commitSha(git, repos.work, 'initial'))
  expect(git(repos.work, 'status', '--porcelain', '--untracked-files=no')).toBe('')
  await expect(app.getByRole('dialog')).toContainText('Previous HEAD')
})

test('FR-BR-04 after a reset the dialog shows the previous HEAD', async ({ app, repos, git }) => {
  const before = git(repos.work, 'rev-parse', 'HEAD').trim()

  await resetTo(app, 'docs and func b', 'Soft')

  await expect(app.getByRole('dialog')).toContainText(`Previous HEAD: ${before}`)
})

test("FR-UX-01 a reset git refuses during a merge shows git's message and changes nothing", async ({
  app,
  repos,
  git,
}) => {
  expect(() => git(repos.work, 'merge', 'feature')).toThrow()
  const head = git(repos.work, 'rev-parse', 'HEAD').trim()

  await resetTo(app, 'add logo', 'Soft')

  await expect(app.getByRole('dialog').locator('.error-banner')).toContainText(
    /cannot do a soft reset in the middle of a merge/i,
  )
  expect(git(repos.work, 'rev-parse', 'HEAD').trim()).toBe(head)
})

test('FR-BR-07 a keep reset refuses when a local change would be lost and nothing moves', async ({
  app,
  repos,
  git,
}) => {
  const file = path.join(repos.work, 'conflict.txt')
  fs.appendFileSync(file, 'local change\n')
  const head = git(repos.work, 'rev-parse', 'HEAD').trim()

  await resetTo(app, 'initial', 'Keep')

  await expect(app.getByRole('dialog').locator('.error-banner')).toContainText('conflict.txt')
  expect(git(repos.work, 'rev-parse', 'HEAD').trim()).toBe(head)
  expect(fs.readFileSync(file, 'utf8')).toContain('local change')
})

test('FR-BR-07 a keep reset on a clean tree moves the branch like hard, without confirmation', async ({
  app,
  repos,
  git,
}) => {
  await resetTo(app, 'add logo', 'Keep')

  await expect(app.getByRole('dialog')).toContainText('Previous HEAD')
  expect(git(repos.work, 'rev-parse', 'HEAD').trim()).toBe(commitSha(git, repos.work, 'add logo'))
  expect(git(repos.work, 'status', '--porcelain', '--untracked-files=no')).toBe('')
  await expect(confirmDialog(app)).toBeHidden()
})

test('FR-BR-06 Rename… and Delete are offered on local branches other than the current one', async ({
  app,
}) => {
  expect(await contextMenuItems(app, refNode(app, 'B'))).toEqual(
    expect.arrayContaining(['Rename…', 'Delete']),
  )
  await app.keyboard.press('Escape')

  const remoteFeature = app.locator('.remote-group .ref-node', {
    has: app.locator('span:text-is("feature")'),
  })
  const remoteItems = await contextMenuItems(app, remoteFeature)
  expect(remoteItems).toContain('Checkout')
  expect(remoteItems).not.toContain('Rename…')
  expect(remoteItems).not.toContain('Delete')

  await refNode(app, 'main').click({ button: 'right' })
  await expect(app.locator('.p-contextmenu [role=menuitem]')).toHaveCount(0)
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
    await pickFromContextMenu(app, refNode(app, 'B'), 'Rename…')
    await dialog.getByPlaceholder('Branch name').fill(name)
    await dialog.getByRole('button', { name: 'Rename' }).click()
    await expect(dialog.locator('.error-banner')).toContainText(message)
    await app.keyboard.press('Escape')
    await expect(dialog).toBeHidden()
  }
  expect(git(repos.work, 'branch', '--list', 'B').trim()).toBe('B')

  await pickFromContextMenu(app, refNode(app, 'B'), 'Rename…')
  await dialog.getByPlaceholder('Branch name').fill('B2')
  await dialog.getByRole('button', { name: 'Rename' }).click()

  await expect.poll(() => git(repos.work, 'branch', '--list', 'B', 'B2').trim()).toBe('B2')
  await expect(refNode(app, 'B2')).toBeVisible()
})

test('FR-BR-06 a merged branch is deleted without confirmation', async ({ app, repos, git }) => {
  await pickFromContextMenu(app, refNode(app, 'B'), 'Delete')

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

  await pickFromContextMenu(app, refNode(app, 'unmerged'), 'Delete')

  await expect(confirmDialog(app)).toContainText(/not fully merged/)
  expect(git(repos.work, 'branch', '--list', 'unmerged').trim()).toBe('unmerged')
  await confirmDialog(app).getByRole('button', { name: 'Confirm' }).click()
  await expect.poll(() => git(repos.work, 'branch', '--list', 'unmerged')).toBe('')
})

test("FR-BR-06 a branch checked out in another worktree is not deleted: git's refusal, no forced retry", async ({
  app,
  repos,
  git,
}) => {
  await pickFromContextMenu(app, refNode(app, 'side'), 'Delete')

  await expect(sidebarError(app)).toContainText(repos.work2)
  await expect(confirmDialog(app)).toBeHidden()
  expect(git(repos.work, 'branch', '--list', 'side').trim()).toContain('side')
})
