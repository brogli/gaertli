// Requirements: FR-SYNC-01, FR-SYNC-02, FR-SYNC-03, FR-SYNC-04, FR-SYNC-05, FR-SYNC-06, FR-SYNC-07, FR-SYNC-08, FR-SYNC-09, FR-UX-12
import fs from 'node:fs'
import http from 'node:http'
import path from 'node:path'
import type { Page } from '@playwright/test'
import { test as base, expect } from './fixtures'
import { gitAfterHistory, type Git } from './repos'
import type { Sandbox } from './sandbox'
import {
  choosePullMode,
  clickConfirm,
  commitFiles,
  confirmDialog,
  conflictBanner,
  fileRow,
  fileText,
  hunksText,
  jobsMenuButton,
  notice,
  openBranchPicker,
  openJobsMenu,
  parents,
  pull,
  pushFromClone,
  refRow,
  rev,
} from './helpers'

// Fetch, pull and push between work, the bare remote and the second clone
// other, plus a remote that demands a login.

interface LoginProtectedRemote {
  url: string
  // authorizations lists each request's Authorization header, '' if none.
  authorizations: string[]
}

const test = base.extend<{ loginProtectedRemote: LoginProtectedRemote }>({
  // oxlint-disable-next-line no-empty-pattern -- Playwright requires a destructuring pattern here
  loginProtectedRemote: async ({}, use) => {
    const authorizations: string[] = []
    const server = http.createServer((req, res) => {
      authorizations.push(req.headers.authorization ?? '')
      res.writeHead(401, { 'WWW-Authenticate': 'Basic realm="nebegit-e2e"' }).end()
    })
    await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve))
    const address = server.address()
    if (address === null || typeof address === 'string') {
      throw new Error('login-protected remote server has no port address')
    }
    await use({ url: `http://127.0.0.1:${address.port}/repo.git`, authorizations })
    await new Promise((resolve) => server.close(resolve))
  },
})

// A compact button (below the wide layout) carries its name in aria-label
// instead of visible text, so this matches by accessible name.
const syncButton = (page: Page, name: string) =>
  page.locator('.sync-controls').getByRole('button', { name })
const pullButton = (page: Page) => page.locator('.sync-controls .p-splitbutton-button')
const aheadBehind = (page: Page) => page.locator('[data-test=ahead-behind]')

// slowUploadPack makes the remote side of a fetch from work take 30 s; its
// delay is a child that holds git's stderr, which cancelling must not wait
// for.
const slowUploadPack = 'sleep 30; exec git-upload-pack'

// remoteAheadAndLocalAhead leaves main diverged: other pushed a commit that
// work lacks, and work has a commit of its own.
function remoteAheadAndLocalAhead(git: Git, work: string, other: string): void {
  pushFromClone(git, other, 'main', { 'remote.txt': 'remote\n' }, 'remote ahead')
  commitFiles(git, work, 'local ahead', { 'local.txt': 'local\n' })
}

test('Fetch lists a new remote branch and drops one deleted on the remote', async ({
  app,
  repos,
  git,
}) => {
  await openBranchPicker(app)
  await expect(refRow(app, 'origin/topic/x')).toBeVisible()
  await app.keyboard.press('Escape')
  git(repos.other, 'push', '-q', 'origin', 'main:refs/heads/newremote')
  git(repos.other, 'push', '-q', 'origin', '--delete', 'topic/x')

  await syncButton(app, 'Fetch').click()

  await expect.poll(() => git(repos.work, 'branch', '-r')).toContain('origin/newremote')
  expect(git(repos.work, 'branch', '-r')).not.toContain('topic/x')
  await openBranchPicker(app)
  await expect(refRow(app, 'origin/newremote')).toBeVisible()
  await expect(refRow(app, 'origin/topic/x')).toHaveCount(0)
})

test('a running fetch shows its progress and can be cancelled', async ({ app, repos, git }) => {
  git(repos.work, 'config', 'remote.origin.uploadpack', slowUploadPack)
  git(repos.other, 'push', '-q', 'origin', 'main:refs/heads/newremote')

  await syncButton(app, 'Fetch').click()
  const list = await openJobsMenu(app)
  await expect(list).toContainText('Fetch')
  await list.locator('[data-test=job-cancel]').click()

  await expect(list).toContainText('No running jobs')
  expect(git(repos.work, 'branch', '-r')).not.toContain('newremote')
})

test.describe('with work and other open', () => {
  test.use({ open: ['work', 'other'] })

  // Every page gets every page's events, in the order they were sent: once
  // a commit made after the fetch's events tops a page's log, the page has
  // had those events too.
  async function commitSeenBy(
    page: Page,
    sandbox: Sandbox,
    repo: string,
    subject: string,
  ): Promise<void> {
    gitAfterHistory(sandbox.env)(repo, 'commit', '-q', '--allow-empty', '-m', subject)
    await expect(page.locator('.row .subject').first()).toHaveText(subject)
  }

  test("a fetch's job shows in its worktree's page only", async ({
    app,
    openApp,
    repos,
    git,
    sandbox,
  }) => {
    const other = await openApp(repos.other)
    git(repos.work, 'config', 'remote.origin.uploadpack', slowUploadPack)

    await syncButton(app, 'Fetch').click()
    await expect(jobsMenuButton(app)).toHaveAttribute('title', '1 running job')
    await commitSeenBy(other, sandbox, repos.other, 'during the fetch')
    await expect(jobsMenuButton(other)).toHaveAttribute('title', 'No running jobs')

    await openJobsMenu(app)
    await app.locator('[data-test=job-cancel]').click()
  })
})

test('Push sends the current branch and the ahead count drops to zero', async ({
  openApp,
  repos,
  git,
}) => {
  commitFiles(git, repos.work, 'local ahead', { 'local.txt': 'local\n' })
  const app = await openApp(repos.work)
  await expect(aheadBehind(app)).toHaveText('1 0')

  await syncButton(app, 'Push').click()

  await expect(aheadBehind(app)).toHaveText('0 0')
  expect(rev(git, repos.remote, 'main')).toBe(rev(git, repos.work, 'HEAD'))
})

test('choosing Pull (rebase) changes the button without pulling; Pull then rebases, and the mode survives a reload', async ({
  openApp,
  repos,
  git,
}) => {
  remoteAheadAndLocalAhead(git, repos.work, repos.other)
  const app = await openApp(repos.work)
  await expect(pullButton(app)).toHaveAttribute('title', 'Pull (merge) · Ctrl+T')
  const head = rev(git, repos.work, 'HEAD')

  await choosePullMode(app, 'rebase')

  await expect(pullButton(app)).toHaveAttribute('title', 'Pull (rebase) · Ctrl+T')
  expect(rev(git, repos.work, 'HEAD')).toBe(head)

  await pull(app)

  await expect.poll(() => rev(git, repos.work, 'HEAD~1')).toBe(rev(git, repos.other, 'main'))
  expect(parents(git, repos.work)).toBe(1)
  expect(git(repos.work, 'log', '-1', '--format=%s').trim()).toBe('local ahead')

  await app.reload()
  await expect(pullButton(app)).toHaveAttribute('title', 'Pull (rebase) · Ctrl+T')
})

test('force push asks first, warns on a protected branch, and the remote follows', async ({
  openApp,
  repos,
  git,
}) => {
  git(repos.work, 'commit', '-q', '--amend', '-m', 'rewritten merge')
  const app = await openApp(repos.work)

  await syncButton(app, 'Push').click()
  await notice(app).getByRole('button', { name: 'Force push' }).click()
  await expect(confirmDialog(app)).toContainText('Force push "main"?')
  await expect(confirmDialog(app)).toContainText('"main" is a protected branch.')
  await clickConfirm(app)

  await expect.poll(() => rev(git, repos.remote, 'main')).toBe(rev(git, repos.work, 'HEAD'))
  await expect(notice(app)).toBeHidden()
})

test("a rejected push names git's reason and offers Pull and Force push; forcing it is refused while the remote has commits not yet fetched", async ({
  openApp,
  repos,
  git,
}) => {
  remoteAheadAndLocalAhead(git, repos.work, repos.other)
  const app = await openApp(repos.work)
  const remoteTip = rev(git, repos.remote, 'main')

  await syncButton(app, 'Push').click()

  await expect(notice(app)).toContainText('fetch first')
  await expect(notice(app).getByRole('button', { name: 'Pull' })).toBeVisible()
  expect(rev(git, repos.remote, 'main')).toBe(remoteTip)

  await notice(app).getByRole('button', { name: 'Force push' }).click()
  await clickConfirm(app)

  await expect(notice(app)).toContainText('stale info')
  expect(rev(git, repos.remote, 'main')).toBe(remoteTip)
})

test("a fetch from a login-protected remote says sign-in failed, shows git's message and never sends credentials", async ({
  app,
  repos,
  git,
  loginProtectedRemote,
}) => {
  git(repos.work, 'remote', 'add', 'locked', loginProtectedRemote.url)

  await syncButton(app, 'Fetch').click()

  await expect(notice(app)).toContainText('Could not sign in to the remote')
  await notice(app).getByRole('button', { name: 'show details' }).click()
  await expect(notice(app)).toContainText('could not read Username')
  expect(loginProtectedRemote.authorizations.length).toBeGreaterThan(0)
  expect(loginProtectedRemote.authorizations.every((a) => a === '')).toBe(true)
})

test("a stash-and-pull that stops on conflicts keeps the stash; Restore brings back the app's stash, not a newer one", async ({
  openApp,
  repos,
  git,
}) => {
  pushFromClone(
    git,
    repos.other,
    'main',
    { 'conflict.txt': 'theirs\n', 'hunks.txt': hunksText({ 3: 'line 3 REMOTE' }) },
    'theirs: conflict.txt',
  )
  const file = (name: string) => path.join(repos.work, name)
  commitFiles(git, repos.work, 'mine: conflict.txt', { 'conflict.txt': 'mine\n' })
  fs.writeFileSync(file('hunks.txt'), hunksText({ 25: 'line 25 LOCAL' }))
  const app = await openApp(repos.work)

  await pull(app)
  await notice(app).getByRole('button', { name: 'Stash and pull' }).click()
  const banner = conflictBanner(app)
  await expect(banner).toBeVisible()
  const stashBanner = app.locator('[data-test=stash-banner]')
  await expect(stashBanner).toBeHidden()

  fs.writeFileSync(file('conflict.txt'), 'merged\n')
  await fileRow(app, 'Conflicted', 'conflict.txt').locator('[data-test=action]').click()
  await banner.getByRole('button', { name: 'Continue' }).click()
  await expect(banner).toBeHidden()
  await expect(stashBanner).toBeVisible()
  expect(parents(git, repos.work)).toBe(2)

  fs.writeFileSync(file('other.txt'), 'terminal stash\n')
  git(repos.work, 'add', 'other.txt')
  git(repos.work, 'stash', 'push', '-q', '-m', 'terminal stash')
  await stashBanner.getByRole('button', { name: 'Restore' }).click()

  await expect
    .poll(() => fileText(file('hunks.txt')))
    .toBe(hunksText({ 3: 'line 3 REMOTE', 25: 'line 25 LOCAL' }))
  await expect(stashBanner).toBeHidden()
  expect(git(repos.work, 'stash', 'list', '--format=%s')).toBe('On main: terminal stash\n')
  expect(fs.existsSync(file('other.txt'))).toBe(false)
  expect(git(repos.work, 'status', '--porcelain')).toBe(' M hunks.txt\n')
  expect(fs.readFileSync(file('conflict.txt'), 'utf8')).toBe('merged\n')
})
