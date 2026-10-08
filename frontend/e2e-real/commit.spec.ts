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

test('FR-CHG-08 the message draft survives a reload', async ({ app }) => {
  const saved = app.waitForResponse((r) => !!r.request().postData()?.includes('"a draft"'))
  await messageBox(app).fill('a draft')
  await saved

  await app.reload()
  await expect(messageBox(app)).toHaveValue('a draft')
})

test("FR-CHG-05 FR-CHG-09 a commit keeps the message exactly, '#' lines included, without an editor; Push after commit pushes it; Amend turns it off", async ({
  openApp,
  repos,
  git,
  editorCalls,
}) => {
  stageNewFile(git, repos.work, 'staged.txt')
  const app = await openApp(repos.work)
  const pushAfter = checkbox(app, 'Push after commit')
  const message = 'subject\n\n# not a comment\nbody line'

  await messageBox(app).fill(message)
  await pushAfter.check()
  await commitButton(app).click()

  await expect(messageBox(app)).toHaveValue('')
  expect(git(repos.work, 'log', '-1', '--format=%B').trimEnd()).toBe(message)
  expect(git(repos.work, 'show', '--name-only', '--format=', 'HEAD').trim()).toBe('staged.txt')
  expect(editorCalls()).toEqual([])
  const committed = rev(git, repos.work, 'HEAD')
  await expect.poll(() => rev(git, repos.remote, 'main')).toBe(committed)

  await checkbox(app, 'Amend last commit').check()
  await expect(pushAfter).toBeDisabled()
})
