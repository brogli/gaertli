// Requirements: FR-UPD-01, FR-UPD-02, FR-UPD-05
import fs from 'node:fs'
import path from 'node:path'
import { test, expect } from './fixtures'
import {
  currentBranch,
  jobsMenuButton,
  logRow,
  notice,
  openJobsMenu,
  pickRefAction,
  pushFromClone,
  rev,
} from './helpers'

// Updating local branches other than work's own from their upstreams,
// with other pushing the new commits to the remote.

test('Update from upstream fast-forwards a branch that is not checked out and names the result; the current branch and files stay; a second update says up to date', async ({
  openApp,
  repos,
  git,
}) => {
  pushFromClone(git, repos.other, 'B', { 'q1.txt': 'q1\n' }, 'remote q1')
  fs.writeFileSync(path.join(repos.work, 'hunks.txt'), 'local edit\n')
  const app = await openApp(repos.work)
  const head = rev(git, repos.work, 'HEAD')
  const status = git(repos.work, 'status', '--porcelain')
  const from = rev(git, repos.work, 'B')
  const tip = rev(git, repos.remote, 'B')

  await pickRefAction(app, 'B', 'Update from upstream')

  await expect.poll(() => rev(git, repos.work, 'B')).toBe(tip)
  await expect(notice(app)).toContainText(
    `Updated B: 1 commit, ${from.slice(0, 7)} → ${tip.slice(0, 7)}`,
  )
  expect(rev(git, repos.work, 'HEAD')).toBe(head)
  expect(currentBranch(git, repos.work)).toBe('main')
  expect(git(repos.work, 'status', '--porcelain')).toBe(status)
  expect(fs.readFileSync(path.join(repos.work, 'hunks.txt'), 'utf8')).toBe('local edit\n')
  expect(fs.existsSync(path.join(repos.work, 'q1.txt'))).toBe(false)

  await pickRefAction(app, 'B', 'Update from upstream')
  await expect(notice(app)).toContainText('B is up to date')
})

test('an update runs as a background job while the app stays usable', async ({
  openApp,
  repos,
  git,
}) => {
  pushFromClone(git, repos.other, 'B', { 'q1.txt': 'q1\n' }, 'remote q1')
  git(repos.work, 'config', 'remote.origin.uploadpack', 'sleep 3; git-upload-pack')
  const app = await openApp(repos.work)

  await pickRefAction(app, 'B', 'Update from upstream')
  const list = await openJobsMenu(app)
  await expect(list).toContainText('Update B')
  await logRow(app, 'add logo').click()
  await expect(app.locator('.commit-details')).toContainText('add logo and func a')
  await expect(jobsMenuButton(app)).toHaveAttribute('title', '1 running job')

  await expect(jobsMenuButton(app)).toHaveAttribute('title', 'No running jobs')
  expect(rev(git, repos.work, 'B')).toBe(rev(git, repos.remote, 'B'))
})
