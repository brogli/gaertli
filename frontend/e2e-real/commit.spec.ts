import fs from 'node:fs'
import path from 'node:path'
import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'
import type { Git } from './repos'
import { conflictBanner } from './helpers'

// The commit box on work's real repo: gating, the message as typed, amend,
// hooks, the draft, and push after commit.

const messageBox = (page: Page) => page.getByPlaceholder('Commit message')
const commitButton = (page: Page) => page.locator('[data-test=commit-submit]')
const checkbox = (page: Page, label: string) =>
  page.locator('.commit-box label', { hasText: label }).locator('input')

// stageNewFile writes name into repo and stages it.
function stageNewFile(git: Git, repo: string, name: string): void {
  fs.writeFileSync(path.join(repo, name), `${name}\n`)
  git(repo, 'add', name)
}

const head = (git: Git, repo: string) => git(repo, 'rev-parse', 'HEAD').trim()

test('FR-CHG-05 Commit is blocked with nothing staged or an empty message and counts the staged files', async ({
  app,
  repos,
  git,
}) => {
  await messageBox(app).fill('message only')
  await expect(commitButton(app)).toBeDisabled()

  stageNewFile(git, repos.work, 'staged.txt')
  await expect(commitButton(app)).toHaveText('Commit (1)')
  await expect(commitButton(app)).toBeEnabled()

  await messageBox(app).fill('   ')
  await expect(commitButton(app)).toBeDisabled()
  await messageBox(app).fill('')
  await expect(commitButton(app)).toBeDisabled()
})

test("FR-CHG-05 a commit keeps the message exactly, '#' lines included, without an editor", async ({
  openApp,
  repos,
  git,
  editorCalls,
}) => {
  stageNewFile(git, repos.work, 'staged.txt')
  const app = await openApp(repos.work)
  const message = 'subject\n\n# not a comment\nbody line'

  await messageBox(app).fill(message)
  await commitButton(app).click()

  await expect(messageBox(app)).toHaveValue('')
  expect(git(repos.work, 'log', '-1', '--format=%B').trimEnd()).toBe(message)
  expect(git(repos.work, 'show', '--name-only', '--format=', 'HEAD').trim()).toBe('staged.txt')
  expect(editorCalls()).toEqual([])
})

test('FR-CHG-05 a double click on Commit makes one commit', async ({ openApp, repos, git }) => {
  stageNewFile(git, repos.work, 'staged.txt')
  const app = await openApp(repos.work)
  const before = head(git, repos.work)

  await messageBox(app).fill('once')
  await commitButton(app).dblclick()

  await expect(messageBox(app)).toHaveValue('')
  await expect(app.locator('.commit-box .error-banner')).toHaveCount(0)
  expect(git(repos.work, 'rev-list', '--count', `${before}..HEAD`).trim()).toBe('1')
})

test('FR-CHG-06 Amend fills in the last message and replaces an unpushed commit without a warning', async ({
  openApp,
  repos,
  git,
}) => {
  stageNewFile(git, repos.work, 'local.txt')
  git(repos.work, 'commit', '-q', '-m', 'local subject', '-m', 'local body')
  const parent = git(repos.work, 'rev-parse', 'HEAD~1').trim()
  const app = await openApp(repos.work)

  await checkbox(app, 'Amend last commit').check()
  await expect(messageBox(app)).toHaveValue('local subject\n\nlocal body')
  await expect(app.locator('.commit-box')).not.toContainText('already been pushed')
  await expect(commitButton(app)).toHaveText('Amend')

  await messageBox(app).fill('amended')
  await commitButton(app).click()

  await expect(messageBox(app)).toHaveValue('')
  expect(git(repos.work, 'log', '-1', '--format=%s').trim()).toBe('amended')
  expect(git(repos.work, 'rev-parse', 'HEAD~1').trim()).toBe(parent)
  expect(git(repos.work, 'show', '--name-only', '--format=', 'HEAD').trim()).toBe('local.txt')
})

test('FR-CHG-06 amending a pushed commit warns that it needs a force push', async ({ app }) => {
  await checkbox(app, 'Amend last commit').check()
  await expect(app.locator('.commit-box')).toContainText(
    'This commit has already been pushed. Amending will require a force push.',
  )
})

test('FR-CHG-06 Amend is disabled while a merge is stopped on conflicts', async ({
  app,
  repos,
  git,
}) => {
  const amend = checkbox(app, 'Amend last commit')
  await expect(amend).toBeEnabled()

  expect(() => git(repos.work, 'merge', '-q', 'feature')).toThrow()
  await expect(conflictBanner(app)).toBeVisible()
  await expect(amend).toBeDisabled()
})

test("FR-CHG-07 a failing pre-commit hook shows the hook's output and keeps the message; no commit", async ({
  openApp,
  repos,
  git,
}) => {
  fs.writeFileSync(
    path.join(repos.work, '.git/hooks/pre-commit'),
    '#!/bin/sh\necho "hook says no" >&2\nexit 1\n',
    { mode: 0o755 },
  )
  stageNewFile(git, repos.work, 'staged.txt')
  const app = await openApp(repos.work)
  const before = head(git, repos.work)

  await messageBox(app).fill('blocked by the hook')
  await commitButton(app).click()

  await expect(app.locator('.commit-box .error-banner')).toContainText('hook says no')
  await expect(messageBox(app)).toHaveValue('blocked by the hook')
  expect(head(git, repos.work)).toBe(before)
  expect(git(repos.work, 'diff', '--cached', '--name-only').trim()).toBe('staged.txt')
})

test.describe('with work and work2 open', () => {
  test.use({ open: ['work', 'work2'] })

  test("FR-CHG-08 the message draft survives a reload; another worktree's box is its own", async ({
    app,
    openApp,
    repos,
  }) => {
    const saved = app.waitForResponse((r) => !!r.request().postData()?.includes('"a draft"'))
    await messageBox(app).fill('a draft')
    await saved

    await app.reload()
    await expect(messageBox(app)).toHaveValue('a draft')

    const other = await openApp(repos.work2)
    await expect(messageBox(other)).toHaveValue('')
  })
})

test('FR-CHG-09 Push after commit pushes the new commit; Amend turns it off', async ({
  openApp,
  repos,
  git,
}) => {
  stageNewFile(git, repos.work, 'staged.txt')
  const app = await openApp(repos.work)
  const pushAfter = checkbox(app, 'Push after commit')

  await messageBox(app).fill('commit and push')
  await pushAfter.check()
  await commitButton(app).click()

  await expect(messageBox(app)).toHaveValue('')
  expect(git(repos.work, 'log', '-1', '--format=%s').trim()).toBe('commit and push')
  const committed = head(git, repos.work)
  await expect.poll(() => git(repos.remote, 'rev-parse', 'main').trim()).toBe(committed)

  await checkbox(app, 'Amend last commit').check()
  await expect(pushAfter).toBeDisabled()
})
