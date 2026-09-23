import fs from 'node:fs'
import http from 'node:http'
import type net from 'node:net'
import path from 'node:path'
import type { Page } from '@playwright/test'
import { test as base, expect } from './fixtures'
import type { Git, Repos } from './repos'
import { commitFiles, confirmDialog, conflictBanner, fileRow, pushFromClone } from './helpers'

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
    const { port } = server.address() as net.AddressInfo
    await use({ url: `http://127.0.0.1:${port}/repo.git`, authorizations })
    await new Promise((resolve) => server.close(resolve))
  },
})

const syncButton = (page: Page, name: string) =>
  page.locator('.sync-controls > button', { hasText: name })
const pullMode = (page: Page) => page.locator('.sync-controls > select')
const rejection = (page: Page) => page.locator('[data-test=push-rejection]')
const aheadBehind = (page: Page) => page.locator('[data-test=ahead-behind]')

const rev = (git: Git, repo: string, ref: string) => git(repo, 'rev-parse', ref).trim()
const remoteMain = (git: Git, repos: Repos) => rev(git, repos.remote, 'main')
const parents = (git: Git, repo: string) =>
  git(repo, 'log', '-1', '--format=%P').trim().split(' ').length

// hunks is hunks.txt (line 1 … line 30) with the given lines replaced.
function hunks(replaced: Record<number, string>): string {
  return Array.from({ length: 30 }, (_, i) => `${replaced[i + 1] ?? `line ${i + 1}`}\n`).join('')
}

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
  const remotes = app.locator('.remote-group')
  await expect(remotes).toContainText('topic')
  git(repos.other, 'push', '-q', 'origin', 'main:refs/heads/newremote')
  git(repos.other, 'push', '-q', 'origin', '--delete', 'topic/x')

  await syncButton(app, 'Fetch').click()

  await expect(remotes).toContainText('newremote')
  await expect(remotes).not.toContainText('topic')
  expect(git(repos.work, 'branch', '-r')).not.toContain('topic/x')
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
  const strip = app.locator('.jobs-strip')
  await expect(strip).toContainText('fetch')
  await strip.locator('[data-test=job-cancel]').click()

  await expect(strip).toBeHidden()
  expect(git(repos.work, 'branch', '-r')).not.toContain('newremote')
})

test('FR-SYNC-03 Push sends the current branch and the ahead count drops to zero', async ({
  openApp,
  repos,
  git,
}) => {
  commitFiles(git, repos.work, 'local ahead', { 'local.txt': 'local\n' })
  const app = await openApp(repos.work)
  await expect(aheadBehind(app)).toHaveText('↑1 ↓0')

  await syncButton(app, 'Push').click()

  await expect(aheadBehind(app)).toHaveText('↑0 ↓0')
  expect(remoteMain(git, repos)).toBe(rev(git, repos.work, 'HEAD'))
})

test('FR-SYNC-03 a branch without upstream offers to push and set one on a chosen remote', async ({
  openApp,
  repos,
  git,
}) => {
  git(repos.work, 'checkout', '-q', '-b', 'no-upstream')
  const app = await openApp(repos.work)

  const remotePicker = app.locator('.sync-controls > select')
  await expect(syncButton(app, 'Pull')).toHaveCount(0)
  await expect(remotePicker.locator('option')).toHaveText(['origin'])
  await expect(remotePicker).toHaveValue('origin')
  await syncButton(app, 'Push and set upstream').click()

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
  await expect(syncButton(app, 'Pull')).toBeVisible()
})

test("FR-SYNC-04 a rejected push names git's reason and offers Pull and Force push", async ({
  openApp,
  repos,
  git,
}) => {
  remoteAheadAndLocalAhead(git, repos.work, repos.other)
  const app = await openApp(repos.work)
  const remoteTip = remoteMain(git, repos)

  await syncButton(app, 'Push').click()

  await expect(rejection(app)).toContainText('fetch first')
  await expect(rejection(app).getByRole('button', { name: 'Pull' })).toBeVisible()
  await expect(rejection(app).getByRole('button', { name: 'Force push' })).toBeVisible()
  expect(remoteMain(git, repos)).toBe(remoteTip)
})

test('FR-SYNC-02 Pull with Merge merges the upstream into the current branch', async ({
  openApp,
  repos,
  git,
}) => {
  remoteAheadAndLocalAhead(git, repos.work, repos.other)
  const app = await openApp(repos.work)

  await pullMode(app).selectOption({ label: 'Merge' })
  await syncButton(app, 'Pull').click()

  await expect.poll(() => parents(git, repos.work)).toBe(2)
  git(repos.work, 'merge-base', '--is-ancestor', 'origin/main', 'HEAD')
  git(repos.work, 'merge-base', '--is-ancestor', rev(git, repos.other, 'main'), 'HEAD')
})

test('FR-SYNC-02 Pull with Rebase replays local commits on the upstream; the choice is preselected next time', async ({
  openApp,
  repos,
  git,
}) => {
  remoteAheadAndLocalAhead(git, repos.work, repos.other)
  const app = await openApp(repos.work)
  await expect(pullMode(app)).toHaveValue('merge')

  await pullMode(app).selectOption({ label: 'Rebase' })
  await syncButton(app, 'Pull').click()

  await expect.poll(() => rev(git, repos.work, 'HEAD~1')).toBe(rev(git, repos.other, 'main'))
  expect(parents(git, repos.work)).toBe(1)
  expect(git(repos.work, 'log', '-1', '--format=%s').trim()).toBe('local ahead')

  await app.reload()
  await expect(pullMode(app)).toHaveValue('rebase')
})

test('FR-SYNC-06 force push asks first, warns on a protected branch, and the remote follows', async ({
  openApp,
  repos,
  git,
}) => {
  git(repos.work, 'commit', '-q', '--amend', '-m', 'rewritten merge')
  const app = await openApp(repos.work)

  await syncButton(app, 'Push').click()
  await rejection(app).getByRole('button', { name: 'Force push' }).click()
  await expect(confirmDialog(app)).toContainText('Force push "main"?')
  await expect(confirmDialog(app)).toContainText('"main" is a protected branch.')
  await confirmDialog(app).getByRole('button', { name: 'Confirm' }).click()

  await expect.poll(() => remoteMain(git, repos)).toBe(rev(git, repos.work, 'HEAD'))
  await expect(rejection(app)).toBeHidden()
})

test('FR-SYNC-05 force push is refused while the remote has commits not yet fetched', async ({
  openApp,
  repos,
  git,
}) => {
  remoteAheadAndLocalAhead(git, repos.work, repos.other)
  const app = await openApp(repos.work)
  const remoteTip = remoteMain(git, repos)

  await syncButton(app, 'Push').click()
  await rejection(app).getByRole('button', { name: 'Force push' }).click()
  await confirmDialog(app).getByRole('button', { name: 'Confirm' }).click()

  await expect(rejection(app)).toContainText('stale info')
  expect(remoteMain(git, repos)).toBe(remoteTip)
})

test("FR-SYNC-08 a fetch from a login-protected remote says sign-in failed, shows git's message and never sends credentials", async ({
  app,
  repos,
  git,
  loginProtectedRemote,
}) => {
  git(repos.work, 'remote', 'add', 'locked', loginProtectedRemote.url)

  await syncButton(app, 'Fetch').click()

  const failure = app.locator('[data-test=sync-failure]')
  await expect(failure).toContainText('Could not sign in to the remote')
  await failure.getByRole('button', { name: 'show details' }).click()
  await expect(failure).toContainText('could not read Username')
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
    { 'hunks.txt': hunks({ 2: 'line 2 REMOTE' }) },
    'remote edit',
  )
  const hunksFile = path.join(repos.work, 'hunks.txt')
  fs.writeFileSync(hunksFile, hunks({ 25: 'line 25 LOCAL' }))
  const app = await openApp(repos.work)

  await pullMode(app).selectOption({ label: 'Merge' })
  await syncButton(app, 'Pull').click()
  const blocked = app.locator('[data-test=pull-blocked]')
  await blocked.getByRole('button', { name: 'Stash and pull' }).click()

  await expect
    .poll(() => fs.readFileSync(hunksFile, 'utf8'))
    .toBe(hunks({ 2: 'line 2 REMOTE', 25: 'line 25 LOCAL' }))
  expect(rev(git, repos.work, 'HEAD')).toBe(rev(git, repos.other, 'main'))
  await expect.poll(() => git(repos.work, 'stash', 'list')).toBe('')
  expect(git(repos.work, 'status', '--porcelain')).toBe(' M hunks.txt\n')
  await expect(blocked).toBeHidden()
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
    { 'conflict.txt': 'theirs\n', 'hunks.txt': hunks({ 3: 'line 3 REMOTE' }) },
    'theirs: conflict.txt',
  )
  const file = (name: string) => path.join(repos.work, name)
  fs.writeFileSync(file('conflict.txt'), 'mine\n')
  git(repos.work, 'commit', '-q', '-am', 'mine: conflict.txt')
  fs.writeFileSync(file('hunks.txt'), hunks({ 25: 'line 25 LOCAL' }))
  const app = await openApp(repos.work)

  await pullMode(app).selectOption({ label: 'Merge' })
  await syncButton(app, 'Pull').click()
  await app
    .locator('[data-test=pull-blocked]')
    .getByRole('button', { name: 'Stash and pull' })
    .click()
  const banner = conflictBanner(app)
  await expect(banner).toBeVisible()
  const notice = app.locator('[data-test=stash-notice]')
  await expect(notice).toBeHidden()

  fs.writeFileSync(file('conflict.txt'), 'merged\n')
  await fileRow(app, 'Conflicted', 'conflict.txt').locator('[data-test=action]').click()
  await banner.getByRole('button', { name: 'Continue' }).click()
  await expect(banner).toBeHidden()
  await expect(notice).toBeVisible()
  expect(parents(git, repos.work)).toBe(2)

  fs.writeFileSync(file('other.txt'), 'terminal stash\n')
  git(repos.work, 'add', 'other.txt')
  git(repos.work, 'stash', 'push', '-q', '-m', 'terminal stash')
  await notice.getByRole('button', { name: 'Restore' }).click()

  await expect
    .poll(() => fs.readFileSync(file('hunks.txt'), 'utf8'))
    .toBe(hunks({ 3: 'line 3 REMOTE', 25: 'line 25 LOCAL' }))
  await expect(notice).toBeHidden()
  expect(git(repos.work, 'stash', 'list', '--format=%s')).toBe('On main: terminal stash\n')
  expect(fs.existsSync(file('other.txt'))).toBe(false)
  expect(git(repos.work, 'status', '--porcelain')).toBe(' M hunks.txt\n')
  expect(fs.readFileSync(file('conflict.txt'), 'utf8')).toBe('merged\n')
})
