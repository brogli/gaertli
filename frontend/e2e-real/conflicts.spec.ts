import fs from 'node:fs'
import path from 'node:path'
import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'
import type { Git, Repos } from './repos'
import {
  clickConfirm,
  commitFiles,
  commitSha,
  conflictBanner,
  fileGroup,
  fileRow,
  hunksText,
  logRow,
  parents,
  pickFromContextMenu,
  pull,
  pushFromClone,
  rev,
} from './helpers'

// Operations stopped on conflicts: started outside the app or from it,
// resolved, continued and aborted through the conflict banner.

// stopOnConflicts runs a git command that must stop on conflicts, which
// git reports with a non-zero exit.
function stopOnConflicts(git: Git, repo: string, ...args: string[]): void {
  expect(() => git(repo, ...args)).toThrow()
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

  test('FR-CONF-01 the banner names the merge and Conflicted lists both files', async ({
    app,
    repos,
    git,
  }) => {
    stopOnConflicts(git, repos.work, 'merge', 'feature')

    const banner = conflictBanner(app)
    await expect(banner).toContainText('merge stopped on conflicts')
    await expect(banner.getByRole('button', { name: 'Continue' })).toBeDisabled()
    await expect(banner.getByRole('button', { name: 'Abort…' })).toBeVisible()
    await expect(fileGroup(app, 'Conflicted').locator('.path')).toHaveText([
      'conflict.txt',
      'hunks.txt',
    ])
  })

  test('FR-CONF-02 double-click or "Open in editor" opens a conflicted file in the editor', async ({
    app,
    repos,
    git,
    editorCalls,
  }) => {
    stopOnConflicts(git, repos.work, 'merge', 'feature')

    await fileRow(app, 'Conflicted', 'conflict.txt').dblclick()
    await fileRow(app, 'Conflicted', 'hunks.txt')
      .getByRole('button', { name: 'Open in editor' })
      .click()

    await expect
      .poll(editorCalls)
      .toEqual([
        `stand-in-editor ${path.join(repos.work, 'conflict.txt')}`,
        `stand-in-editor ${path.join(repos.work, 'hunks.txt')}`,
      ])
  })

  test('FR-CONF-03 a file marked resolved leaves Conflicted; Continue waits for the other', async ({
    app,
    repos,
    git,
  }) => {
    stopOnConflicts(git, repos.work, 'merge', 'feature')

    await markResolved(app, 'conflict.txt', 'resolved\n', repos)

    await expect(fileGroup(app, 'Conflicted').locator('.path')).toHaveText(['hunks.txt'])
    expect(git(repos.work, 'diff', '--cached', '--name-only', '--diff-filter=U').trim()).toBe(
      'hunks.txt',
    )
    await expect(conflictBanner(app).getByRole('button', { name: 'Continue' })).toBeDisabled()
  })

  test('FR-CONF-04 with every file resolved, Continue commits the merge without an editor', async ({
    app,
    repos,
    git,
    editorCalls,
  }) => {
    stopOnConflicts(git, repos.work, 'merge', 'feature')
    await markResolved(app, 'conflict.txt', 'resolved\n', repos)
    await markResolved(app, 'hunks.txt', hunksText({ 1: 'resolved' }), repos)

    await conflictBanner(app).getByRole('button', { name: 'Continue' }).click()

    await expect(conflictBanner(app)).toBeHidden()
    expect(parents(git, repos.work)).toBe(2)
    expect(editorCalls()).toEqual([])
  })

  test('FR-CONF-04 Abort asks first, then returns to before the merge', async ({
    app,
    repos,
    git,
  }) => {
    const head = rev(git, repos.work, 'HEAD')
    stopOnConflicts(git, repos.work, 'merge', 'feature')

    await abort(app)

    await expect(conflictBanner(app)).toBeHidden()
    expect(rev(git, repos.work, 'HEAD')).toBe(head)
    expect(git(repos.work, 'status', '--porcelain')).toBe('')
  })
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

test('FR-CONF-01 a revert stopped on conflicts is named in the banner and can be aborted', async ({
  app,
  repos,
  git,
}) => {
  commitFiles(git, repos.work, 'change conflict.txt again', { 'conflict.txt': 'changed again\n' })
  const head = rev(git, repos.work, 'HEAD')
  stopOnConflicts(
    git,
    repos.work,
    'revert',
    '--no-edit',
    commitSha(git, repos.work, 'main: change conflict.txt'),
  )

  await expect(conflictBanner(app)).toContainText('revert stopped on conflicts')
  await abort(app)

  await expect(conflictBanner(app)).toBeHidden()
  expect(rev(git, repos.work, 'HEAD')).toBe(head)
  expect(fs.existsSync(path.join(repos.work, '.git', 'REVERT_HEAD'))).toBe(false)
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

test('FR-CP-03 a cherry-pick from the menu that stops on conflicts is handed to the banner; Abort undoes it', async ({
  app,
  repos,
  git,
}) => {
  const head = rev(git, repos.work, 'HEAD')

  await pickFromContextMenu(app, logRow(app, 'feature: change conflict.txt'), 'Cherry-pick')

  await expect(conflictBanner(app)).toContainText('cherry-pick stopped on conflicts')
  await abort(app)

  await expect(conflictBanner(app)).toBeHidden()
  expect(rev(git, repos.work, 'HEAD')).toBe(head)
})
