import fs from 'node:fs'
import path from 'node:path'
import type { Locator, Page } from '@playwright/test'
import { test, expect } from './fixtures'
import type { Git } from './repos'
import {
  clickConfirm,
  commitSha,
  confirmDialog,
  currentBranch,
  logRow,
  openWorktreePicker,
  pickFromContextMenu,
  pickRefAction,
  pickerAction,
} from './helpers'

// The worktree picker on work's repo: listing, creating, opening and
// removing worktrees, each checked against the real repo. Switching this
// window is covered by the component tests; here a worktree is shown by
// opening a page on it.

// worktreeLi is the picker's <li> of the worktree on branch: the row and
// its sibling buttons both live inside it.
function worktreeLi(page: Page, branch: string): Locator {
  return page.locator('.picker-rows li', {
    has: page.locator(`[data-test=worktree-branch]:text-is("${branch}")`),
  })
}

// worktreeRow is that worktree's own labelled row, its clickable part.
function worktreeRow(page: Page, branch: string): Locator {
  return worktreeLi(page, branch).locator('[data-test=worktree-row]')
}

function removeButton(page: Page, branch: string): Locator {
  return worktreeLi(page, branch).locator('[data-test=remove-worktree]')
}

function newWorktreeDialog(page: Page): Locator {
  return page.getByRole('dialog', { name: 'New worktree' })
}

// createOnNewBranch submits the picker's New worktree dialog for a new
// branch at target, from start if given, and returns the dialog.
async function createOnNewBranch(
  page: Page,
  { branch, start = '', target }: { branch: string; start?: string; target: string },
): Promise<Locator> {
  await openWorktreePicker(page)
  await pickerAction(page, 'New worktree…').click()
  const dialog = newWorktreeDialog(page)
  await dialog.getByLabel('New branch').check()
  await dialog.getByPlaceholder('Branch name').fill(branch)
  await dialog.getByPlaceholder('Start point (branch or commit)').fill(start)
  await dialog.getByPlaceholder('Target folder').fill(target)
  await dialog.getByRole('button', { name: 'Create' }).click()
  return dialog
}

// worktreePaths lists the worktrees git knows of repo, missing ones included.
function worktreePaths(git: Git, repo: string): string[] {
  return git(repo, 'worktree', 'list', '--porcelain')
    .split('\n')
    .filter((line) => line.startsWith('worktree '))
    .map((line) => line.slice('worktree '.length))
}

// branchTip is branch's commit, "" when there is no such branch.
function branchTip(git: Git, repo: string, branch: string): string {
  return git(repo, 'for-each-ref', '--format=%(objectname)', `refs/heads/${branch}`).trim()
}

function addWorktree(git: Git, repo: string, dir: string, branch: string): void {
  git(repo, 'worktree', 'add', '-q', dir, '-b', branch)
}

test("FR-WT-13 the picker names this window's worktree and lists this repo's worktrees at once, this one marked", async ({
  app,
  repos,
}) => {
  await expect(app.locator('[data-test=worktree-picker]')).toContainText('work')
  await openWorktreePicker(app)

  await expect(app.locator('.p-popover [data-test=worktree-row]')).toHaveCount(2)
  await expect(worktreeRow(app, 'main').locator('.current-marker i')).toBeVisible()
  await expect(worktreeRow(app, 'side').locator('.current-marker i')).toHaveCount(0)
  await expect(worktreeRow(app, 'side')).toHaveAttribute('title', repos.work2)
})

test("FR-WT-13 typing part of a worktree's folder name leaves only its row", async ({ app }) => {
  await openWorktreePicker(app)

  await app.locator('.p-popover .picker-list input').fill('work2')

  await expect(app.locator('.p-popover .picker-rows li')).toHaveCount(1)
  await expect(worktreeRow(app, 'side')).toBeVisible()
})

test('FR-WT-01 worktree rows flag a dirty, a locked and a missing worktree', async ({
  openApp,
  repos,
  sandbox,
  git,
}) => {
  fs.writeFileSync(path.join(repos.work2, 'side.txt'), 'edited\n')
  addWorktree(git, repos.work, path.join(sandbox.dir, 'locked'), 'held')
  git(repos.work, 'worktree', 'lock', path.join(sandbox.dir, 'locked'))
  const gone = path.join(sandbox.dir, 'gone')
  addWorktree(git, repos.work, gone, 'lost')
  fs.rmSync(gone, { recursive: true })
  const app = await openApp(repos.work)

  await openWorktreePicker(app)

  await expect(worktreeRow(app, 'side').locator('[data-test=dirty-marker]')).toBeVisible()
  await expect(worktreeRow(app, 'main').locator('[data-test=dirty-marker]')).toHaveCount(0)
  await expect(worktreeRow(app, 'held').locator('[data-test=locked-marker]')).toBeVisible()
  await expect(worktreeRow(app, 'lost')).toContainText('missing')
  await expect(worktreeLi(app, 'lost').locator('[data-test=open-in-editor]')).toHaveCount(0)
})

test("FR-ED-01 Open in editor on the selected worktree's row opens its folder", async ({
  app,
  repos,
  editorCalls,
}) => {
  await openWorktreePicker(app)

  await worktreeLi(app, 'main').locator('[data-test=open-in-editor]').click()

  await expect.poll(editorCalls).toEqual([`stand-in-editor ${repos.work}`])
})

test("FR-WT-04 Open in editor on another worktree's row opens that folder in one click", async ({
  app,
  repos,
  editorCalls,
}) => {
  await openWorktreePicker(app)

  await worktreeLi(app, 'side').locator('[data-test=open-in-editor]').click()

  await expect.poll(editorCalls).toEqual([`stand-in-editor ${repos.work2}`])
})

test('FR-WT-02 a new worktree on an existing branch is created at the chosen folder and listed', async ({
  app,
  repos,
  sandbox,
  git,
}) => {
  const target = path.join(sandbox.dir, 'on-b')
  await openWorktreePicker(app)
  await pickerAction(app, 'New worktree…').click()
  const dialog = newWorktreeDialog(app)
  await dialog.locator('[data-test=existing-branch]').selectOption('B')
  await dialog.getByPlaceholder('Target folder').fill(target)
  await dialog.getByRole('button', { name: 'Create' }).click()

  await expect.poll(() => worktreePaths(git, repos.work)).toContain(target)
  expect(currentBranch(git, target)).toBe('B')
  await dialog.getByRole('button', { name: 'Not now' }).click()
  await expect(dialog).toBeHidden()
  await openWorktreePicker(app)
  await expect(worktreeRow(app, 'B')).toBeVisible()
})

test('FR-WT-02 a new worktree on a new branch starts that branch at the given start point', async ({
  app,
  repos,
  sandbox,
  git,
}) => {
  const target = path.join(sandbox.dir, 'fresh')
  await createOnNewBranch(app, { branch: 'fresh', start: 'feature', target })

  await expect.poll(() => worktreePaths(git, repos.work)).toContain(target)
  expect(currentBranch(git, target)).toBe('fresh')
  expect(branchTip(git, repos.work, 'fresh')).toBe(branchTip(git, repos.work, 'feature'))
})

test('FR-WT-13 a worktree made with New worktree… opens in its own window, which shows it', async ({
  app,
  openApp,
  repos,
  sandbox,
  git,
}) => {
  const target = path.join(sandbox.dir, 'opened')
  const dialog = await createOnNewBranch(app, { branch: 'opened', target })
  await dialog.getByRole('button', { name: 'Open in app window', exact: true }).click()
  await expect(dialog).toBeHidden()
  expect(worktreePaths(git, repos.work)).toContain(target)

  const shown = await openApp(target)

  await expect(shown.locator('[data-test=repo-picker]')).toContainText('work')
  await expect(shown.locator('[data-test=worktree-picker]')).toContainText('opened')
  await expect(shown.locator('[data-test=branch-picker]')).toContainText('opened')
})

test('FR-WT-12 after creating a worktree the dialog offers to open it; Open in editor opens its folder', async ({
  app,
  sandbox,
  editorCalls,
}) => {
  const target = path.join(sandbox.dir, 'follow-up')
  const dialog = await createOnNewBranch(app, { branch: 'follow-up', target })

  for (const offer of ['Open in app window', 'Open in editor', 'Both', 'Not now']) {
    await expect(dialog.getByRole('button', { name: offer, exact: true })).toBeVisible()
  }
  await dialog.getByRole('button', { name: 'Open in editor', exact: true }).click()
  await expect.poll(editorCalls).toEqual([`stand-in-editor ${target}`])
  await expect(dialog).toBeHidden()
  await openWorktreePicker(app)
  await expect(worktreeRow(app, 'follow-up')).toBeVisible()
})

test("FR-WT-03 a target folder with files in it is refused with git's message and nothing is created", async ({
  app,
  repos,
  git,
}) => {
  const before = worktreePaths(git, repos.work)
  const dialog = await createOnNewBranch(app, { branch: 'refused', target: repos.work })

  await expect(dialog.locator('.error-banner')).toContainText('already exists')
  expect(worktreePaths(git, repos.work)).toEqual(before)
  expect(branchTip(git, repos.work, 'refused')).toBe('')
})

test('FR-WT-06 Create worktree here… on a log commit makes a worktree on a new branch at that commit', async ({
  app,
  repos,
  sandbox,
  git,
}) => {
  const target = path.join(sandbox.dir, 'at-logo')
  await pickFromContextMenu(app, logRow(app, 'add logo'), 'Create worktree here…')
  const dialog = newWorktreeDialog(app)
  await expect(dialog.getByLabel('New branch')).toBeChecked()
  await dialog.getByPlaceholder('Branch name').fill('at-logo')
  await dialog.getByPlaceholder('Target folder').fill(target)
  await dialog.getByRole('button', { name: 'Create' }).click()

  await expect.poll(() => worktreePaths(git, repos.work)).toContain(target)
  expect(currentBranch(git, target)).toBe('at-logo')
  expect(branchTip(git, repos.work, 'at-logo')).toBe(commitSha(git, repos.work, 'add logo'))
})

test('FR-WT-06 New worktree from here… on a branch in the branch picker makes a worktree on that branch', async ({
  app,
  repos,
  sandbox,
  git,
}) => {
  const target = path.join(sandbox.dir, 'on-feature')
  await pickRefAction(app, 'feature', 'New worktree from here…')
  const dialog = newWorktreeDialog(app)
  await expect(dialog.locator('[data-test=existing-branch]')).toHaveValue('feature')
  await dialog.getByPlaceholder('Target folder').fill(target)
  await dialog.getByRole('button', { name: 'Create' }).click()

  await expect.poll(() => worktreePaths(git, repos.work)).toContain(target)
  expect(currentBranch(git, target)).toBe('feature')
})

test.describe('with work2 selected', () => {
  test.use({ open: ['work2'] })

  test('FR-WT-07 the main worktree and the selected one offer no Remove, a linked one does', async ({
    openApp,
    repos,
    sandbox,
    git,
  }) => {
    addWorktree(git, repos.work, path.join(sandbox.dir, 'third'), 'third')
    const app = await openApp(repos.work2)

    await openWorktreePicker(app)

    await expect(removeButton(app, 'third')).toBeVisible()
    for (const branch of ['main', 'side']) {
      await expect(worktreeRow(app, branch)).toBeVisible()
      await expect(removeButton(app, branch)).toHaveCount(0)
    }
  })
})

test('FR-WT-07 removing a clean linked worktree asks first, then it is gone', async ({
  openApp,
  repos,
  sandbox,
  git,
}) => {
  const clean = path.join(sandbox.dir, 'clean')
  addWorktree(git, repos.work, clean, 'clean')
  const app = await openApp(repos.work)
  await openWorktreePicker(app)

  await removeButton(app, 'clean').click()
  await expect(confirmDialog(app)).toContainText(clean)
  await expect(confirmDialog(app)).not.toContainText('will be lost')
  expect(worktreePaths(git, repos.work)).toContain(clean)
  await clickConfirm(app)

  await expect.poll(() => worktreePaths(git, repos.work)).not.toContain(clean)
  expect(fs.existsSync(clean)).toBe(false)
  await openWorktreePicker(app)
  await expect(worktreeRow(app, 'main')).toBeVisible()
  await expect(worktreeRow(app, 'clean')).toHaveCount(0)
})

test('FR-WT-07 removing a worktree with uncommitted changes says they will be lost, then removes it', async ({
  openApp,
  repos,
  sandbox,
  git,
}) => {
  const dirty = path.join(sandbox.dir, 'dirty')
  addWorktree(git, repos.work, dirty, 'dirty')
  fs.writeFileSync(path.join(dirty, 'unsaved.txt'), 'work in progress\n')
  const app = await openApp(repos.work)
  await openWorktreePicker(app)

  await removeButton(app, 'dirty').click()
  await expect(confirmDialog(app)).toContainText('uncommitted changes that will be lost')
  expect(worktreePaths(git, repos.work)).toContain(dirty)
  await clickConfirm(app)

  await expect.poll(() => worktreePaths(git, repos.work)).not.toContain(dirty)
  expect(fs.existsSync(dirty)).toBe(false)
})

test('FR-WT-07 a worktree changed after the picker listed it is refused by git, then removed only after a second confirmation', async ({
  openApp,
  repos,
  sandbox,
  git,
}) => {
  const late = path.join(sandbox.dir, 'late')
  addWorktree(git, repos.work, late, 'late')
  const app = await openApp(repos.work)
  await openWorktreePicker(app)
  await expect(worktreeRow(app, 'late').locator('[data-test=dirty-marker]')).toHaveCount(0)
  fs.writeFileSync(path.join(late, 'unsaved.txt'), 'work in progress\n')

  await removeButton(app, 'late').click()
  await expect(confirmDialog(app)).not.toContainText('will be lost')
  await clickConfirm(app)
  await expect(confirmDialog(app)).toContainText(
    `${late} has uncommitted changes that will be lost`,
  )
  expect(worktreePaths(git, repos.work)).toContain(late)
  expect(fs.existsSync(path.join(late, 'unsaved.txt'))).toBe(true)
  await clickConfirm(app)

  await expect.poll(() => worktreePaths(git, repos.work)).not.toContain(late)
  expect(fs.existsSync(late)).toBe(false)
})

test('FR-WT-07 Clean up stale entries prunes a missing worktree without asking', async ({
  openApp,
  repos,
  sandbox,
  git,
}) => {
  const gone = path.join(sandbox.dir, 'gone')
  addWorktree(git, repos.work, gone, 'lost')
  fs.rmSync(gone, { recursive: true })
  const app = await openApp(repos.work)
  await openWorktreePicker(app)
  await expect(worktreeRow(app, 'lost')).toContainText('missing')

  await pickerAction(app, 'Clean up stale entries').click()

  await expect.poll(() => worktreePaths(git, repos.work)).not.toContain(gone)
  await expect(worktreeRow(app, 'lost')).toHaveCount(0)
  await expect(confirmDialog(app)).toBeHidden()
})
