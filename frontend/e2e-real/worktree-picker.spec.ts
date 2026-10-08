import fs from 'node:fs'
import path from 'node:path'
import type { Locator, Page } from '@playwright/test'
import { test, expect } from './fixtures'
import type { Git } from './repos'
import {
  clickConfirm,
  confirmDialog,
  currentBranch,
  openWorktreePicker,
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
// branch at target, starting at the branch start picked from the list.
async function createOnNewBranch(
  page: Page,
  { branch, start, target }: { branch: string; start: string; target: string },
): Promise<void> {
  await openWorktreePicker(page)
  await pickerAction(page, 'New worktree…').click()
  const dialog = newWorktreeDialog(page)
  await dialog.getByLabel('New branch').check()
  await dialog.getByLabel('Branch name').fill(branch)
  await dialog.locator('[data-test=start-branch]').click()
  await page.getByRole('option', { name: start, exact: true }).click()
  await dialog.getByLabel('Location').fill(target)
  await dialog.getByRole('button', { name: 'Create' }).click()
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

test("FR-WT-04 Open in editor on another worktree's row opens that folder in one click", async ({
  app,
  repos,
  editorCalls,
}) => {
  await openWorktreePicker(app)

  await worktreeLi(app, 'side').locator('[data-test=open-in-editor]').click()

  await expect.poll(editorCalls).toEqual([`stand-in-editor ${repos.work2}`])
})

test('FR-WT-02 a new worktree on a new branch starts that branch at the branch picked as its start', async ({
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
