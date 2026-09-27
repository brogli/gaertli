import fs from 'node:fs'
import http from 'node:http'
import path from 'node:path'
import type { Page } from '@playwright/test'
import { test as base, expect } from './fixtures'
import { PullMode } from '../bindings/github.com/brogli/nebegit/internal/core/models'
import { gitAfterHistory, type Git } from './repos'
import type { Sandbox } from './sandbox'
import {
  choosePull,
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
const messageBox = (page: Page) => page.getByPlaceholder('Commit message')
const commitButton = (page: Page) => page.locator('[data-test=commit-submit]')
const pushAfterCommitCheckbox = (page: Page) =>
  page.locator('.commit-box label', { hasText: 'Push after commit' }).locator('input')

// remoteAheadAndLocalAhead leaves main diverged: other pushed a commit that
// work lacks, and work has a commit of its own.
function remoteAheadAndLocalAhead(git: Git, work: string, other: string): void {
  pushFromClone(git, other, 'main', { 'remote.txt': 'remote\n' }, 'remote ahead')
  commitFiles(git, work, 'local ahead', { 'local.txt': 'local\n' })
}

test('FR-SYNC-01 Fetch lists a new remote branch and drops one deleted on the remote', async ({
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

test('FR-SYNC-07 a running fetch shows its progress and can be cancelled', async ({
  app,
  repos,
  git,
}) => {
  // The upload pack's delay is a child that holds git's stderr: cancelling
  // must not wait for it.
  git(repos.work, 'config', 'remote.origin.uploadpack', 'sleep 30; exec git-upload-pack')
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

  test("FR-UX-02 a fetch's job shows in its worktree's page only", async ({
    app,
    openApp,
    repos,
    git,
    sandbox,
  }) => {
    const other = await openApp(repos.other)
    git(repos.work, 'config', 'remote.origin.uploadpack', 'sleep 30; exec git-upload-pack')

    await syncButton(app, 'Fetch').click()
    await expect(jobsMenuButton(app)).toHaveAttribute('title', '1 running job')
    await commitSeenBy(other, sandbox, repos.other, 'during the fetch')
    await expect(jobsMenuButton(other)).toHaveAttribute('title', 'No running jobs')

    await openJobsMenu(app)
    await app.locator('[data-test=job-cancel]').click()
  })
})

test('FR-SYNC-03 Push sends the current branch and the ahead count drops to zero', async ({
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

test('FR-SYNC-03 a branch without upstream asks before pushing and sets one on its only remote', async ({
  openApp,
  repos,
  git,
}) => {
  git(repos.work, 'checkout', '-q', '-b', 'no-upstream')
  const app = await openApp(repos.work)

  await expect(syncButton(app, 'Pull')).toHaveCount(0)
  await expect(syncButton(app, 'Push')).toHaveAttribute('title', 'Push')
  await syncButton(app, 'Push').click()

  const dialog = app.getByRole('dialog', { name: 'Push and set upstream' })
  await dialog.getByRole('button', { name: 'Push and set upstream' }).click()

  await expect
    .poll(() =>
      git(
        repos.work,
        'for-each-ref',
        '--format=%(upstream:short)',
        'refs/heads/no-upstream',
      ).trim(),
    )
    .toBe('origin/no-upstream')
  await expect(pullButton(app)).toBeVisible()
})

test('FR-CHG-09 push after commit on a new branch pushes and sets the upstream, without asking, when push.autoSetupRemote is set', async ({
  openApp,
  repos,
  git,
}) => {
  git(repos.work, 'checkout', '-q', '-b', 'auto-upstream')
  git(repos.work, 'config', 'push.autoSetupRemote', 'true')
  fs.writeFileSync(path.join(repos.work, 'staged.txt'), 'staged\n')
  git(repos.work, 'add', 'staged.txt')
  const app = await openApp(repos.work)

  await messageBox(app).fill('first commit on a new branch')
  await pushAfterCommitCheckbox(app).check()
  await commitButton(app).click()

  await expect(app.getByRole('dialog')).toHaveCount(0)
  await expect
    .poll(() =>
      git(
        repos.work,
        'for-each-ref',
        '--format=%(upstream:short)',
        'refs/heads/auto-upstream',
      ).trim(),
    )
    .toBe('origin/auto-upstream')
  expect(rev(git, repos.remote, 'auto-upstream')).toBe(rev(git, repos.work, 'HEAD'))
})

test("FR-SYNC-04 a rejected push names git's reason and offers Pull and Force push", async ({
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
  await expect(notice(app).getByRole('button', { name: 'Force push' })).toBeVisible()
  expect(rev(git, repos.remote, 'main')).toBe(remoteTip)
})

test('FR-SYNC-02 Pull with Merge merges the upstream into the current branch', async ({
  openApp,
  repos,
  git,
}) => {
  remoteAheadAndLocalAhead(git, repos.work, repos.other)
  const app = await openApp(repos.work)

  await choosePull(app, 'merge')

  await expect.poll(() => parents(git, repos.work)).toBe(2)
  git(repos.work, 'merge-base', '--is-ancestor', 'origin/main', 'HEAD')
  git(repos.work, 'merge-base', '--is-ancestor', rev(git, repos.other, 'main'), 'HEAD')
})

test('FR-SYNC-02 Pull with Rebase replays local commits on the upstream', async ({
  openApp,
  repos,
  git,
}) => {
  remoteAheadAndLocalAhead(git, repos.work, repos.other)
  const app = await openApp(repos.work)
  await expect(pullButton(app)).toHaveAttribute('title', 'Pull (merge)')

  await choosePull(app, 'rebase')

  await expect.poll(() => rev(git, repos.work, 'HEAD~1')).toBe(rev(git, repos.other, 'main'))
  expect(parents(git, repos.work)).toBe(1)
  expect(git(repos.work, 'log', '-1', '--format=%s').trim()).toBe('local ahead')

  // The menu's choice does not change the button: a reload still shows
  // Settings' default (merge, unset here).
  await app.reload()
  await expect(pullButton(app)).toHaveAttribute('title', 'Pull (merge)')
})

test.describe('the Pull button follows the default pull mode', () => {
  test.describe('default rebase', () => {
    test.use({ settings: { defaultPullMode: PullMode.PullRebase } })

    test('FR-SYNC-02 clicking Pull replays local commits on the upstream', async ({
      openApp,
      repos,
      git,
    }) => {
      remoteAheadAndLocalAhead(git, repos.work, repos.other)
      const app = await openApp(repos.work)

      await pullButton(app).click()

      await expect.poll(() => rev(git, repos.work, 'HEAD~1')).toBe(rev(git, repos.other, 'main'))
      expect(parents(git, repos.work)).toBe(1)
    })
  })

  test.describe('default merge', () => {
    test.use({ settings: { defaultPullMode: PullMode.PullMerge } })

    test('FR-SYNC-02 clicking Pull merges the upstream into the current branch', async ({
      openApp,
      repos,
      git,
    }) => {
      remoteAheadAndLocalAhead(git, repos.work, repos.other)
      const app = await openApp(repos.work)

      await pullButton(app).click()

      await expect.poll(() => parents(git, repos.work)).toBe(2)
    })
  })
})

test('FR-SYNC-06 force push asks first, warns on a protected branch, and the remote follows', async ({
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

test('FR-SYNC-05 force push is refused while the remote has commits not yet fetched', async ({
  openApp,
  repos,
  git,
}) => {
  remoteAheadAndLocalAhead(git, repos.work, repos.other)
  const app = await openApp(repos.work)
  const remoteTip = rev(git, repos.remote, 'main')

  await syncButton(app, 'Push').click()
  await notice(app).getByRole('button', { name: 'Force push' }).click()
  await clickConfirm(app)

  await expect(notice(app)).toContainText('stale info')
  expect(rev(git, repos.remote, 'main')).toBe(remoteTip)
})

test("FR-SYNC-08 a fetch from a login-protected remote says sign-in failed, shows git's message and never sends credentials", async ({
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

test('FR-SYNC-09 a pull blocked by local changes offers Stash and pull; the pull lands and the edit is back', async ({
  openApp,
  repos,
  git,
}) => {
  pushFromClone(
    git,
    repos.other,
    'main',
    { 'hunks.txt': hunksText({ 2: 'line 2 REMOTE' }) },
    'remote edit',
  )
  const hunksFile = path.join(repos.work, 'hunks.txt')
  fs.writeFileSync(hunksFile, hunksText({ 25: 'line 25 LOCAL' }))
  const app = await openApp(repos.work)

  await choosePull(app, 'merge')
  await notice(app).getByRole('button', { name: 'Stash and pull' }).click()

  await expect
    .poll(() => fileText(hunksFile))
    .toBe(hunksText({ 2: 'line 2 REMOTE', 25: 'line 25 LOCAL' }))
  expect(rev(git, repos.work, 'HEAD')).toBe(rev(git, repos.other, 'main'))
  await expect.poll(() => git(repos.work, 'stash', 'list')).toBe('')
  expect(git(repos.work, 'status', '--porcelain')).toBe(' M hunks.txt\n')
  await expect(notice(app)).toBeHidden()
})

test("FR-SYNC-09 a stash-and-pull that stops on conflicts keeps the stash; Restore brings back the app's stash, not a newer one", async ({
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

  await choosePull(app, 'merge')
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
