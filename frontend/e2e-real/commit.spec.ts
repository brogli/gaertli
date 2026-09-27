import fs from 'node:fs'
import path from 'node:path'
import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'
import type { Git } from './repos'
import { rev } from './helpers'

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

test('FR-CHG-06 Amend fills in the last message and replaces an unpushed commit without a warning', async ({
  openApp,
  repos,
  git,
}) => {
  stageNewFile(git, repos.work, 'local.txt')
  git(repos.work, 'commit', '-q', '-m', 'local subject', '-m', 'local body')
  const parent = rev(git, repos.work, 'HEAD~1')
  const app = await openApp(repos.work)

  await checkbox(app, 'Amend last commit').check()
  await expect(messageBox(app)).toHaveValue('local subject\n\nlocal body')
  await expect(app.locator('.commit-box')).not.toContainText('already been pushed')
  await expect(commitButton(app)).toHaveAccessibleName('Amend')

  await messageBox(app).fill('amended')
  await commitButton(app).click()

  await expect(messageBox(app)).toHaveValue('')
  expect(git(repos.work, 'log', '-1', '--format=%s').trim()).toBe('amended')
  expect(rev(git, repos.work, 'HEAD~1')).toBe(parent)
  expect(git(repos.work, 'show', '--name-only', '--format=', 'HEAD').trim()).toBe('local.txt')
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
  const before = rev(git, repos.work, 'HEAD')

  await messageBox(app).fill('blocked by the hook')
  await commitButton(app).click()

  await expect(app.locator('.commit-box .error-banner')).toContainText('hook says no')
  await expect(messageBox(app)).toHaveValue('blocked by the hook')
  expect(rev(git, repos.work, 'HEAD')).toBe(before)
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
  const committed = rev(git, repos.work, 'HEAD')
  await expect.poll(() => rev(git, repos.remote, 'main')).toBe(committed)

  await checkbox(app, 'Amend last commit').check()
  await expect(pushAfter).toBeDisabled()
})
