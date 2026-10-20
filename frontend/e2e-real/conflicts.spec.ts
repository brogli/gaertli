import fs from 'node:fs'
import path from 'node:path'
import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'
import type { Git, Repos } from './repos'
import {
  clickConfirm,
  commitFiles,
  conflictBanner,
  fileGroup,
  fileRow,
  hunksText,
  parents,
  pull,
  pushFromClone,
  rev,
  stagedText,
} from './helpers'

// Operations stopped on conflicts: started outside the app or from it,
// resolved, continued and aborted through the conflict banner.

// stopOnConflicts runs a git command that must stop on conflicts, which
// git reports with a non-zero exit.
function stopOnConflicts(git: Git, repo: string, ...args: string[]): void {
  expect(() => git(repo, ...args)).toThrow()
}

type OpenDiff = (worktree: string, target: Record<string, string>) => Promise<Page>

// openConflict opens the diff window on file's conflict.
function openConflict(openDiff: OpenDiff, repo: string, file: string): Promise<Page> {
  return openDiff(repo, { kind: 'working', conflicted: 'true', file })
}

async function abort(page: Page): Promise<void> {
  await conflictBanner(page).getByRole('button', { name: 'Abort…' }).click()
  await clickConfirm(page)
}

async function markResolved(page: Page, file: string, text: string, repos: Repos): Promise<void> {
  fs.writeFileSync(path.join(repos.work, file), text)
  await fileRow(page, 'Conflicted', file).getByRole('button', { name: 'Mark resolved' }).click()
}

test.describe('a merge of feature stopped on two conflicting files', () => {
  test.beforeEach(({ repos, git }) => {
    git(repos.work, 'checkout', '-q', 'feature')
    commitFiles(git, repos.work, 'feature: hunks line 1', {
      'hunks.txt': hunksText({ 1: 'feature' }),
    })
    git(repos.work, 'checkout', '-q', 'main')
    commitFiles(git, repos.work, 'main: hunks line 1', { 'hunks.txt': hunksText({ 1: 'main' }) })
  })

  test('FR-CONF-01 FR-CONF-03 FR-CONF-04 the banner names the merge and Conflicted lists both files; a file marked resolved leaves Conflicted; with every file resolved, Continue commits the merge without an editor', async ({
    app,
    repos,
    git,
    editorCalls,
  }) => {
    stopOnConflicts(git, repos.work, 'merge', 'feature')

    const banner = conflictBanner(app)
    const continueButton = banner.getByRole('button', { name: 'Continue' })
    await expect(banner).toContainText('merge stopped on conflicts')
    await expect(continueButton).toBeDisabled()
    await expect(banner.getByRole('button', { name: 'Abort…' })).toBeVisible()
    await expect(fileGroup(app, 'Conflicted').locator('.path')).toHaveText([
      'conflict.txt',
      'hunks.txt',
    ])

    await markResolved(app, 'conflict.txt', 'resolved\n', repos)
    await expect(fileGroup(app, 'Conflicted').locator('.path')).toHaveText(['hunks.txt'])
    expect(git(repos.work, 'diff', '--cached', '--name-only', '--diff-filter=U').trim()).toBe(
      'hunks.txt',
    )
    await expect(continueButton).toBeDisabled()

    await markResolved(app, 'hunks.txt', hunksText({ 1: 'resolved' }), repos)
    await continueButton.click()

    await expect(banner).toBeHidden()
    expect(parents(git, repos.work)).toBe(2)
    expect(editorCalls()).toEqual([])
  })
})

test("FR-CONF-07 a modify/delete conflict offers a side in the diff window; taking the current side's version stages it and enables Continue", async ({
  app,
  openDiff,
  repos,
  git,
}) => {
  git(repos.work, 'checkout', '-q', '-b', 'deleter', 'v1')
  git(repos.work, 'rm', '-q', 'docs/readme.md')
  git(repos.work, 'commit', '-q', '-m', 'deleter: remove readme')
  git(repos.work, 'checkout', '-q', 'main')
  commitFiles(git, repos.work, 'main: edit readme', { 'docs/readme.md': 'edited on main\n' })
  stopOnConflicts(git, repos.work, 'merge', 'deleter')
  await expect(conflictBanner(app).getByRole('button', { name: 'Continue' })).toBeDisabled()

  const diff = await openConflict(openDiff, repos.work, 'docs/readme.md')
  await diff.getByRole('button', { name: 'Keep the version of main' }).click()

  await expect.poll(() => stagedText(git, repos.work, 'docs/readme.md')).toBe('edited on main\n')
  await expect(conflictBanner(app).getByRole('button', { name: 'Continue' })).toBeEnabled()
})

test('FR-CONF-01 a rebase stopped on conflicts is named in the banner and can be aborted', async ({
  app,
  repos,
  git,
}) => {
  const head = rev(git, repos.work, 'HEAD')
  stopOnConflicts(git, repos.work, 'rebase', 'feature')

  await expect(conflictBanner(app)).toContainText('rebase stopped on conflicts')
  await abort(app)

  await expect(conflictBanner(app)).toBeHidden()
  expect(rev(git, repos.work, 'HEAD')).toBe(head)
  expect(fs.existsSync(path.join(repos.work, '.git', 'rebase-merge'))).toBe(false)
})

test('FR-CONF-01 a pull that stops on conflicts shows the merge in the banner; Abort undoes it', async ({
  app,
  repos,
  git,
}) => {
  pushFromClone(
    git,
    repos.other,
    'main',
    { 'conflict.txt': 'remote side\n' },
    'remote: conflict.txt',
  )
  commitFiles(git, repos.work, 'local: conflict.txt', { 'conflict.txt': 'local side\n' })
  const head = rev(git, repos.work, 'HEAD')

  await pull(app)

  await expect(conflictBanner(app)).toContainText('merge stopped on conflicts')
  await abort(app)

  await expect(conflictBanner(app)).toBeHidden()
  expect(rev(git, repos.work, 'HEAD')).toBe(head)
})

test('FR-CONF-10 FR-CONF-06 a merge conflict resolves in the merge editor: a side taken, changes only one side made applied, a region edited by hand and kept across closing the window, marked done, saved and staged with no markers', async ({
  app,
  openDiff,
  repos,
  git,
}) => {
  git(repos.work, 'checkout', '-q', 'feature')
  commitFiles(git, repos.work, 'feature: hunks lines 1, 10 and 20', {
    'hunks.txt': hunksText({ 1: 'feature 1', 10: 'feature 10', 20: 'feature 20' }),
  })
  git(repos.work, 'checkout', '-q', 'main')
  commitFiles(git, repos.work, 'main: hunks lines 1 and 20', {
    'hunks.txt': hunksText({ 1: 'main 1', 20: 'main 20' }),
  })
  stopOnConflicts(git, repos.work, 'merge', 'feature')

  const diff = await openConflict(openDiff, repos.work, 'hunks.txt')
  const resultOf = (page: Page) => page.locator('[data-test=pane-header-result] ~ div .cm-content')
  const result = resultOf(diff)
  await expect(result).toContainText('line 1')
  await expect(result).not.toContainText('<<<<<<<')
  await expect(result).not.toContainText('feature 10')
  const save = diff.getByRole('button', { name: 'Save and mark resolved' })
  await expect(save).toBeDisabled()

  await diff.getByRole('button', { name: 'Take right' }).click()
  await diff.getByRole('button', { name: 'Apply non-conflicting changes' }).click()
  await expect(result).toContainText('feature 10')
  await diff.locator('[data-test=next-conflict]').click()
  await expect(diff.locator('[data-test=conflict-position]')).toContainText('1 left')
  await result.focus()
  await diff.keyboard.type('edited ')
  await expect(save).toBeDisabled()
  const text = (await result.textContent()) ?? ''
  await diff.waitForResponse(
    (res) =>
      res.url().endsWith('/wails/runtime') && (res.request().postData() ?? '').includes('edited '),
  )

  await diff.close()
  const reopened = await openConflict(openDiff, repos.work, 'hunks.txt')
  await expect(resultOf(reopened)).toHaveText(text)

  await reopened.getByRole('button', { name: 'Mark done' }).click()
  await expect(reopened.locator('[data-test=conflict-position]')).toHaveText('No changes left')
  await reopened.getByRole('button', { name: 'Save and mark resolved' }).click()

  const expected = hunksText({ 1: 'feature 1', 10: 'feature 10', 20: 'edited line 20' })
  await expect.poll(() => stagedText(git, repos.work, 'hunks.txt')).toBe(expected)
  expect(fs.readFileSync(path.join(repos.work, 'hunks.txt'), 'utf8')).toBe(expected)
  await expect(fileGroup(app, 'Conflicted').locator('.path')).toHaveText(['conflict.txt'])
})
