import fs from 'node:fs'
import path from 'node:path'
import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'
import type { Git, Repos } from './repos'
import {
  commitFiles,
  currentBranch,
  jobsMenuButton,
  logRow,
  notice,
  openBranchPicker,
  openJobsMenu,
  pickerAction,
  pickRefAction,
  pushFromClone,
  refActions,
  refLi,
  rev,
} from './helpers'

// Updating local branches other than work's own from their upstreams,
// with other pushing the new commits to the remote.

async function updateFromUpstream(page: Page, branch: string): Promise<void> {
  await pickRefAction(page, branch, 'Update from upstream')
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

  await expect.poll(() => rev(git, repos.work, 'B')).toBe(rev(git, repos.remote, 'B'))
  expect(rev(git, repos.work, 'HEAD')).toBe(head)
  expect(currentBranch(git, repos.work)).toBe('main')
  expect(git(repos.work, 'status', '--porcelain')).toBe(status)
  expect(fs.readFileSync(path.join(repos.work, 'hunks.txt'), 'utf8')).toBe('local edit\n')
  expect(fs.existsSync(path.join(repos.work, 'q1.txt'))).toBe(false)
})

test('FR-UPD-01 a branch without an upstream offers no Update from upstream', async ({ app }) => {
  expect(await refActions(app, 'B')).toContain('Update from upstream')
  await app.keyboard.press('Escape')
  expect(await refActions(app, 'side')).not.toContain('Update from upstream')
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
  const list = await openJobsMenu(app)
  await expect(list).toContainText('Update B')
  await logRow(app, 'add logo').click()
  await expect(app.locator('.commit-details')).toContainText('add logo and func a')
  await expect(jobsMenuButton(app)).toHaveAttribute('title', '1 running job')

  await expect(jobsMenuButton(app)).toHaveAttribute('title', 'No running jobs')
  expect(rev(git, repos.work, 'B')).toBe(rev(git, repos.remote, 'B'))
})

test('FR-UPD-05 the result names the branch, the commit count and the new commit, then says already up to date', async ({
  openApp,
  repos,
  git,
}) => {
  pushFromClone(git, repos.other, 'B', { 'q1.txt': 'q1\n' }, 'remote q1')
  const app = await openApp(repos.work)
  const from = rev(git, repos.work, 'B')
  const tip = rev(git, repos.remote, 'B')

  await updateFromUpstream(app, 'B')
  await expect(notice(app)).toContainText(
    `Updated B: 1 commit, ${from.slice(0, 7)} → ${tip.slice(0, 7)}`,
  )

  await updateFromUpstream(app, 'B')
  await expect(notice(app)).toContainText('B is up to date')
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

  await expect(notice(app)).toContainText(/diverged/i)
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

    await expect.poll(() => rev(git, repos.work, 'B')).toBe(rev(git, repos.remote, 'B'))
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

    await expect(notice(app)).toContainText('work3')
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

  await openBranchPicker(app)
  await refLi(app, 'B').locator('[data-test=update-select] input').check()
  await refLi(app, 'topic/x').locator('[data-test=update-select] input').check()
  await pickerAction(app, 'Update 2 branches').click()

  await expect.poll(() => rev(git, repos.work, 'B')).toBe(rev(git, repos.remote, 'B'))
  await expect.poll(() => rev(git, repos.work, 'topic/x')).toBe(rev(git, repos.remote, 'topic/x'))
  expect(rev(git, repos.work, 'HEAD')).toBe(rev(git, repos.work, 'main'))
})
