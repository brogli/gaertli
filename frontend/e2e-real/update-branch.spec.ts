import fs from 'node:fs'
import path from 'node:path'
import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'
import type { Git, Repos } from './repos'
import {
  commitFiles,
  contextMenuItems,
  logRow,
  pickFromContextMenu,
  pushFromClone,
  refNode,
} from './helpers'

// Updating local branches other than work's own from their upstreams,
// with other pushing the new commits to the remote.

const updateResult = (page: Page) => page.locator('[data-test=update-result]')
const rev = (git: Git, repo: string, ref: string) => git(repo, 'rev-parse', ref).trim()
const remoteTip = (git: Git, repos: Repos, branch: string) => rev(git, repos.remote, branch)

async function updateFromUpstream(page: Page, branch: string): Promise<void> {
  await pickFromContextMenu(page, refNode(page, branch), 'Update from upstream')
}

// commitOnBranch commits file on branch in a throwaway worktree of work, so
// work itself stays on main.
function commitOnBranch(git: Git, repos: Repos, branch: string, file: string): void {
  const tmp = path.join(path.dirname(repos.work), 'tmp-worktree')
  git(repos.work, 'worktree', 'add', '-q', tmp, branch)
  commitFiles(git, tmp, `local ${file}`, { [file]: `${file}\n` })
  git(repos.work, 'worktree', 'remove', tmp)
}

test('FR-UPD-01 Update from upstream fast-forwards a branch that is not checked out; the current branch and files stay', async ({
  openApp,
  repos,
  git,
}) => {
  pushFromClone(git, repos.other, 'B', { 'q1.txt': 'q1\n' }, 'remote q1')
  fs.writeFileSync(path.join(repos.work, 'hunks.txt'), 'local edit\n')
  const app = await openApp(repos.work)
  const head = rev(git, repos.work, 'HEAD')
  const status = git(repos.work, 'status', '--porcelain')

  await updateFromUpstream(app, 'B')

  await expect.poll(() => rev(git, repos.work, 'B')).toBe(remoteTip(git, repos, 'B'))
  expect(rev(git, repos.work, 'HEAD')).toBe(head)
  expect(git(repos.work, 'branch', '--show-current').trim()).toBe('main')
  expect(git(repos.work, 'status', '--porcelain')).toBe(status)
  expect(fs.readFileSync(path.join(repos.work, 'hunks.txt'), 'utf8')).toBe('local edit\n')
  expect(fs.existsSync(path.join(repos.work, 'q1.txt'))).toBe(false)
})

test('FR-UPD-01 a branch without an upstream offers no Update from upstream', async ({ app }) => {
  expect(await contextMenuItems(app, refNode(app, 'B'))).toContain('Update from upstream')
  await app.keyboard.press('Escape')
  expect(await contextMenuItems(app, refNode(app, 'side'))).not.toContain('Update from upstream')
})

test('FR-UPD-02 an update runs as a background job while the app stays usable', async ({
  openApp,
  repos,
  git,
}) => {
  pushFromClone(git, repos.other, 'B', { 'q1.txt': 'q1\n' }, 'remote q1')
  git(repos.work, 'config', 'remote.origin.uploadpack', 'sleep 3; git-upload-pack')
  const app = await openApp(repos.work)

  await updateFromUpstream(app, 'B')
  const strip = app.locator('.jobs-strip')
  await expect(strip).toContainText('Update B')
  await logRow(app, 'add logo').click()
  await expect(app.locator('.commit-details')).toContainText('add logo and func a')
  await expect(strip).toBeVisible()

  await expect(strip).toBeHidden()
  expect(rev(git, repos.work, 'B')).toBe(remoteTip(git, repos, 'B'))
})

test('FR-UPD-05 the result names the branch, the commit count and the new commit, then says already up to date', async ({
  openApp,
  repos,
  git,
}) => {
  pushFromClone(git, repos.other, 'B', { 'q1.txt': 'q1\n' }, 'remote q1')
  const app = await openApp(repos.work)
  const tip = remoteTip(git, repos, 'B')

  await updateFromUpstream(app, 'B')
  await expect(updateResult(app)).toHaveText(`Updated "B": 1 commit(s), now at ${tip.slice(0, 7)}.`)

  await updateFromUpstream(app, 'B')
  await expect(updateResult(app)).toHaveText('"B" is already up to date.')
})

test('FR-UPD-03 a diverged branch is left as it is and the result says it diverged', async ({
  openApp,
  repos,
  git,
}) => {
  commitOnBranch(git, repos, 'B', 'local.txt')
  pushFromClone(git, repos.other, 'B', { 'q3.txt': 'q3\n' }, 'remote q3')
  const app = await openApp(repos.work)
  const local = rev(git, repos.work, 'B')

  await updateFromUpstream(app, 'B')

  await expect(updateResult(app)).toContainText(/diverged/i)
  expect(rev(git, repos.work, 'B')).toBe(local)
})

test.describe('B checked out in another worktree', () => {
  const work3 = (repos: Repos) => path.join(path.dirname(repos.work), 'work3')

  test('FR-UPD-04 a clean worktree on the branch moves with it', async ({
    openApp,
    repos,
    git,
  }) => {
    git(repos.work, 'worktree', 'add', '-q', work3(repos), 'B')
    pushFromClone(git, repos.other, 'B', { 'q4.txt': 'q4\n' }, 'remote q4')
    const app = await openApp(repos.work)

    await updateFromUpstream(app, 'B')

    await expect.poll(() => rev(git, repos.work, 'B')).toBe(remoteTip(git, repos, 'B'))
    expect(fs.readFileSync(path.join(work3(repos), 'q4.txt'), 'utf8')).toBe('q4\n')
    expect(git(work3(repos), 'status', '--porcelain')).toBe('')
  })

  test('FR-UPD-04 a worktree on the branch with local changes refuses the update, naming the worktree', async ({
    openApp,
    repos,
    git,
  }) => {
    git(repos.work, 'worktree', 'add', '-q', work3(repos), 'B')
    fs.appendFileSync(path.join(work3(repos), 'hunks.txt'), 'dirty\n')
    pushFromClone(git, repos.other, 'B', { 'q5.txt': 'q5\n' }, 'remote q5')
    const app = await openApp(repos.work)
    const local = rev(git, repos.work, 'B')

    await updateFromUpstream(app, 'B')

    await expect(updateResult(app)).toContainText('work3')
    expect(rev(git, repos.work, 'B')).toBe(local)
    expect(git(work3(repos), 'status', '--porcelain')).toBe(' M hunks.txt\n')
  })
})

test('FR-UPD-06 Update 2 branches fast-forwards each selected branch', async ({
  openApp,
  repos,
  git,
}) => {
  pushFromClone(git, repos.other, 'B', { 'b.txt': 'b\n' }, 'remote b')
  pushFromClone(git, repos.other, 'topic/x', { 't.txt': 't\n' }, 'remote t')
  const app = await openApp(repos.work)

  await refNode(app, 'B').locator('[data-test=update-select]').check()
  await app.getByRole('treeitem', { name: 'topic' }).locator('button').first().click()
  await refNode(app, 'x').locator('[data-test=update-select]').check()
  await app.getByRole('button', { name: 'Update 2 branches' }).click()

  await expect.poll(() => rev(git, repos.work, 'B')).toBe(remoteTip(git, repos, 'B'))
  await expect.poll(() => rev(git, repos.work, 'topic/x')).toBe(remoteTip(git, repos, 'topic/x'))
  expect(rev(git, repos.work, 'HEAD')).toBe(rev(git, repos.work, 'main'))
})
