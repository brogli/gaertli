import fs from 'node:fs'
import path from 'node:path'
import type { Locator, Page } from '@playwright/test'
import { test, expect } from './fixtures'
import type { Git } from './repos'
import { commitSha, confirmDialog, logRow, pickFromContextMenu, refNode } from './helpers'

// The selector panel on work's repo: listing, creating, opening and
// removing worktrees, and branches from the panel, each checked against
// the real repo.

// expandRepo opens the panel's node of the recent repo at repoPath.
async function expandRepo(page: Page, repoPath: string): Promise<void> {
  const repo = page.getByRole('treeitem', { name: repoPath, exact: true })
  await repo.press('ArrowRight')
  await expect(repo).toHaveAttribute('aria-expanded', 'true')
}

// A panel row's tree item is named after its branch; worktree and branch
// rows of the same branch are told apart by their data-test.
function worktreeRow(page: Page, branch: string): Locator {
  return page
    .getByRole('treeitem', { name: branch, exact: true })
    .locator('[data-test=worktree-row]')
}

function removeButton(page: Page, branch: string): Locator {
  return worktreeRow(page, branch).locator('[data-test=remove-worktree]')
}

function branchRow(page: Page, branch: string): Locator {
  return page.getByRole('treeitem', { name: branch, exact: true }).locator('[data-test=branch-row]')
}

function newWorktreeDialog(page: Page): Locator {
  return page.getByRole('dialog', { name: 'New worktree' })
}

// createOnNewBranch submits the panel's New worktree dialog for a new
// branch at target, from start if given, and returns the dialog.
async function createOnNewBranch(
  page: Page,
  repo: string,
  { branch, start = '', target }: { branch: string; start?: string; target: string },
): Promise<Locator> {
  await expandRepo(page, repo)
  await page.locator('[data-test=new-worktree]').click()
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

test('FR-WT-05 with no worktree selected the window shows the panel and asks to pick one', async ({
  page,
  server,
  repos,
}) => {
  await page.goto(server.url)

  await expect(page.getByText('Select a worktree from the panel')).toBeVisible()
  await expect(page.getByRole('treeitem', { name: repos.work, exact: true })).toBeVisible()
})

test('FR-UX-04 Settings opens as a dialog from the panel, also with no worktree selected', async ({
  page,
  server,
}) => {
  await page.goto(server.url)

  await page.locator('[data-test=open-settings]').click()
  await expect(page.getByRole('dialog', { name: 'Settings' })).toBeVisible()
})

test('FR-WT-13 the panel collapses and expands', async ({ app }) => {
  const collapse = app.locator('[data-test=panel-collapse]')
  await collapse.click()
  await expect(app.locator('[data-test=open-settings]')).toBeHidden()
  await collapse.click()
  await expect(app.locator('[data-test=open-settings]')).toBeVisible()
})

test('FR-WT-13 expanding a recent repo lists its worktrees, the selected one marked, and its branches', async ({
  app,
  repos,
}) => {
  await expandRepo(app, repos.work)

  await expect(app.locator('[data-test=worktree-row]')).toHaveCount(2)
  await expect(worktreeRow(app, 'main')).toContainText('●')
  await expect(worktreeRow(app, 'side')).toContainText('○')
  for (const branch of ['B', 'feature', 'main', 'side', 'topic/x']) {
    await expect(branchRow(app, branch)).toBeVisible()
  }
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

  await expandRepo(app, repos.work)

  await expect(worktreeRow(app, 'side')).toContainText('*')
  await expect(worktreeRow(app, 'main')).not.toContainText('*')
  await expect(worktreeRow(app, 'held')).toContainText('🔒')
  await expect(worktreeRow(app, 'lost')).toContainText('missing')
  await expect(worktreeRow(app, 'lost').locator('[data-test=open-in-editor]')).toHaveCount(0)
})

test("FR-ED-01 Open in editor on the selected worktree's row opens its folder", async ({
  app,
  repos,
  editorCalls,
}) => {
  await expandRepo(app, repos.work)

  await worktreeRow(app, 'main').locator('[data-test=open-in-editor]').click()

  await expect.poll(editorCalls).toEqual([`stand-in-editor ${repos.work}`])
})

test("FR-WT-04 Open in editor on another worktree's row opens that folder in one click", async ({
  app,
  repos,
  editorCalls,
}) => {
  await expandRepo(app, repos.work)

  await worktreeRow(app, 'side').locator('[data-test=open-in-editor]').click()

  await expect.poll(editorCalls).toEqual([`stand-in-editor ${repos.work2}`])
})

test('FR-WT-02 a new worktree on an existing branch is created at the chosen folder and listed', async ({
  app,
  repos,
  sandbox,
  git,
}) => {
  const target = path.join(sandbox.dir, 'on-b')
  await expandRepo(app, repos.work)
  await app.locator('[data-test=new-worktree]').click()
  const dialog = newWorktreeDialog(app)
  await dialog.locator('[data-test=existing-branch]').selectOption('B')
  await dialog.getByPlaceholder('Target folder').fill(target)
  await dialog.getByRole('button', { name: 'Create' }).click()

  await expect.poll(() => worktreePaths(git, repos.work)).toContain(target)
  expect(git(target, 'branch', '--show-current').trim()).toBe('B')
  await dialog.getByRole('button', { name: 'Not now' }).click()
  await expect(dialog).toBeHidden()
  await expect(worktreeRow(app, 'B')).toBeVisible()
})

test('FR-WT-02 a new worktree on a new branch starts that branch at the given start point', async ({
  app,
  repos,
  sandbox,
  git,
}) => {
  const target = path.join(sandbox.dir, 'fresh')
  await createOnNewBranch(app, repos.work, { branch: 'fresh', start: 'feature', target })

  await expect.poll(() => worktreePaths(git, repos.work)).toContain(target)
  expect(git(target, 'branch', '--show-current').trim()).toBe('fresh')
  expect(branchTip(git, repos.work, 'fresh')).toBe(branchTip(git, repos.work, 'feature'))
})

test('FR-WT-12 after creating a worktree the dialog offers to open it; Open in editor opens its folder', async ({
  app,
  repos,
  sandbox,
  editorCalls,
}) => {
  const target = path.join(sandbox.dir, 'follow-up')
  const dialog = await createOnNewBranch(app, repos.work, { branch: 'follow-up', target })

  for (const offer of ['Open in app window', 'Open in editor', 'Both', 'Not now']) {
    await expect(dialog.getByRole('button', { name: offer, exact: true })).toBeVisible()
  }
  await dialog.getByRole('button', { name: 'Open in editor', exact: true }).click()
  await expect.poll(editorCalls).toEqual([`stand-in-editor ${target}`])
  await expect(dialog).toBeHidden()
  await expect(worktreeRow(app, 'follow-up')).toBeVisible()
})

test("FR-WT-03 a target folder with files in it is refused with git's message and nothing is created", async ({
  app,
  repos,
  git,
}) => {
  const before = worktreePaths(git, repos.work)
  const dialog = await createOnNewBranch(app, repos.work, {
    branch: 'refused',
    target: repos.work,
  })

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
  expect(git(target, 'branch', '--show-current').trim()).toBe('at-logo')
  expect(branchTip(git, repos.work, 'at-logo')).toBe(commitSha(git, repos.work, 'add logo'))
})

test('FR-WT-06 Create worktree here… on a sidebar branch makes a worktree on that branch', async ({
  app,
  repos,
  sandbox,
  git,
}) => {
  const target = path.join(sandbox.dir, 'on-feature')
  await pickFromContextMenu(app, refNode(app, 'feature'), 'Create worktree here…')
  const dialog = newWorktreeDialog(app)
  await expect(dialog.locator('[data-test=existing-branch]')).toHaveValue('feature')
  await dialog.getByPlaceholder('Target folder').fill(target)
  await dialog.getByRole('button', { name: 'Create' }).click()

  await expect.poll(() => worktreePaths(git, repos.work)).toContain(target)
  expect(git(target, 'branch', '--show-current').trim()).toBe('feature')
})

test('FR-WT-13 New branch… in the panel creates a branch at HEAD and checks it out', async ({
  app,
  repos,
  git,
}) => {
  const head = git(repos.work, 'rev-parse', 'HEAD').trim()
  await expandRepo(app, repos.work)
  await app.locator('[data-test=new-branch]').click()
  const dialog = app.getByRole('dialog', { name: 'New branch' })
  await dialog.getByPlaceholder('Branch name').fill('from-panel')
  await dialog.getByRole('button', { name: 'Create' }).click()

  await expect.poll(() => git(repos.work, 'branch', '--show-current').trim()).toBe('from-panel')
  expect(branchTip(git, repos.work, 'from-panel')).toBe(head)
})

test("FR-WT-13 a branch row's Checkout switches the selected worktree to that branch", async ({
  app,
  repos,
  git,
}) => {
  await expandRepo(app, repos.work)

  await branchRow(app, 'feature').locator('[data-test=checkout-branch]').click()

  await expect.poll(() => git(repos.work, 'branch', '--show-current').trim()).toBe('feature')
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

    await expandRepo(app, repos.work2)

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
  await expandRepo(app, repos.work)

  await removeButton(app, 'clean').click()
  await expect(confirmDialog(app)).toContainText(clean)
  await expect(confirmDialog(app)).not.toContainText('will be lost')
  expect(worktreePaths(git, repos.work)).toContain(clean)
  await confirmDialog(app).getByRole('button', { name: 'Confirm' }).click()

  await expect.poll(() => worktreePaths(git, repos.work)).not.toContain(clean)
  expect(fs.existsSync(clean)).toBe(false)
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
  await expandRepo(app, repos.work)

  await removeButton(app, 'dirty').click()
  await expect(confirmDialog(app)).toContainText('uncommitted changes that will be lost')
  expect(worktreePaths(git, repos.work)).toContain(dirty)
  await confirmDialog(app).getByRole('button', { name: 'Confirm' }).click()

  await expect.poll(() => worktreePaths(git, repos.work)).not.toContain(dirty)
  expect(fs.existsSync(dirty)).toBe(false)
})

test('FR-WT-07 a worktree changed after the panel listed it is refused by git, then removed only after a second confirmation', async ({
  openApp,
  repos,
  sandbox,
  git,
}) => {
  const late = path.join(sandbox.dir, 'late')
  addWorktree(git, repos.work, late, 'late')
  const app = await openApp(repos.work)
  await expandRepo(app, repos.work)
  await expect(worktreeRow(app, 'late')).not.toContainText('*')
  fs.writeFileSync(path.join(late, 'unsaved.txt'), 'work in progress\n')

  await removeButton(app, 'late').click()
  await expect(confirmDialog(app)).not.toContainText('will be lost')
  await confirmDialog(app).getByRole('button', { name: 'Confirm' }).click()
  await expect(confirmDialog(app)).toContainText(
    `${late} has uncommitted changes that will be lost`,
  )
  expect(worktreePaths(git, repos.work)).toContain(late)
  expect(fs.existsSync(path.join(late, 'unsaved.txt'))).toBe(true)
  await confirmDialog(app).getByRole('button', { name: 'Confirm' }).click()

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
  await expandRepo(app, repos.work)
  await expect(worktreeRow(app, 'lost')).toContainText('missing')

  await app.locator('[data-test=prune-worktrees]').click()

  await expect.poll(() => worktreePaths(git, repos.work)).not.toContain(gone)
  await expect(worktreeRow(app, 'lost')).toHaveCount(0)
  await expect(confirmDialog(app)).toBeHidden()
})
