import fs from 'node:fs'
import path from 'node:path'
import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'
import type { Git, Repos } from './repos'
import {
  boundCallEndingWith,
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
  shortRev,
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

  test('FR-CONF-02 double-click opens a conflicted file in the diff window; "Open in editor" opens the editor', async ({
    app,
    repos,
    git,
    editorCalls,
  }) => {
    stopOnConflicts(git, repos.work, 'merge', 'feature')

    const openDiffWindow = boundCallEndingWith(app, [
      { kind: 'working', conflicted: true, path: 'conflict.txt', oldPath: '' },
    ])
    await fileRow(app, 'Conflicted', 'conflict.txt').dblclick()
    await openDiffWindow
    await fileRow(app, 'Conflicted', 'hunks.txt')
      .getByRole('button', { name: 'Open in editor' })
      .click()

    await expect
      .poll(editorCalls)
      .toEqual([`stand-in-editor ${path.join(repos.work, 'hunks.txt')}`])
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

test("FR-CONF-07 a modify/delete conflict shows its panel in the diff window; taking the current side's version stages it and enables Continue", async ({
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
  await expect(diff.getByText('deleter deleted the file;')).toBeVisible()
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

test.describe('the merge editor', () => {
  function pane(page: Page, name: string) {
    return page.locator(`[data-test=pane-header-${name}]`)
  }

  test('FR-CONF-05 a merge conflict with two regions resolves in the merge editor: a side taken, a region edited by hand and marked done, saved and staged with no markers', async ({
    app,
    openDiff,
    repos,
    git,
  }) => {
    git(repos.work, 'checkout', '-q', 'feature')
    commitFiles(git, repos.work, 'feature: hunks lines 1 and 20', {
      'hunks.txt': hunksText({ 1: 'feature 1', 20: 'feature 20' }),
    })
    git(repos.work, 'checkout', '-q', 'main')
    commitFiles(git, repos.work, 'main: hunks lines 1 and 20', {
      'hunks.txt': hunksText({ 1: 'main 1', 20: 'main 20' }),
    })
    stopOnConflicts(git, repos.work, 'merge', 'feature')

    const diff = await openConflict(openDiff, repos.work, 'hunks.txt')
    await expect(pane(diff, 'current')).toHaveText('main · current branch')
    await expect(pane(diff, 'result')).toHaveText('Result')
    await expect(pane(diff, 'incoming')).toHaveText('feature · merged branch')
    const position = diff.locator('[data-test=conflict-position]')
    await expect(position).toHaveText('Conflict 1 of 2 · 2 left')
    const result = diff.locator('[data-test=pane-header-result] ~ div .cm-content')
    await expect(result).toContainText('line 1')
    await expect(result).not.toContainText('<<<<<<<')
    const save = diff.getByRole('button', { name: 'Save and mark resolved' })
    await expect(save).toBeDisabled()

    await diff.getByRole('button', { name: 'Take incoming' }).click()
    await diff.locator('[data-test=next-conflict]').click()
    await expect(position).toHaveText('Conflict 2 of 2 · 1 left')
    await result.focus()
    await diff.keyboard.type('edited ')
    await expect(position).toHaveText('Conflict 2 of 2 · 1 left')
    await expect(save).toBeDisabled()
    await diff.getByRole('button', { name: 'Mark done' }).click()
    await expect(position).toHaveText('No conflicts left')
    await save.click()

    const expected = hunksText({ 1: 'feature 1', 20: 'edited line 20' })
    await expect.poll(() => stagedText(git, repos.work, 'hunks.txt')).toBe(expected)
    expect(fs.readFileSync(path.join(repos.work, 'hunks.txt'), 'utf8')).toBe(expected)
    await expect(fileGroup(app, 'Conflicted').locator('.path')).toHaveText(['conflict.txt'])
  })

  test('FR-CONF-06 a side taken and an edit survive closing the diff window and opening the file again', async ({
    openDiff,
    repos,
    git,
  }) => {
    stopOnConflicts(git, repos.work, 'merge', 'feature')
    const diff = await openConflict(openDiff, repos.work, 'conflict.txt')
    await diff.getByRole('button', { name: 'Take incoming' }).click()
    const result = diff.locator('[data-test=pane-header-result] ~ div .cm-content')
    await result.focus()
    await diff.keyboard.type('kept ')
    const text = await result.innerText()
    await diff.waitForTimeout(1000)

    await diff.close()
    const reopened = await openConflict(openDiff, repos.work, 'conflict.txt')

    const again = reopened.locator('[data-test=pane-header-result] ~ div .cm-content')
    await expect(again).toHaveText(text)
    await expect(reopened.locator('[data-test=conflict-position]')).toHaveText('No conflicts left')
  })

  test('FR-CONF-05 a rebase conflict names the sides: the branch rebased onto and the commit being replayed', async ({
    openDiff,
    repos,
    git,
  }) => {
    git(repos.work, 'checkout', '-q', 'feature')
    stopOnConflicts(git, repos.work, 'rebase', 'main')

    const diff = await openConflict(openDiff, repos.work, 'conflict.txt')

    await expect(pane(diff, 'current')).toHaveText('main · rebased onto')
    const sha = shortRev(git, repos.work, 'REBASE_HEAD')
    await expect(pane(diff, 'incoming')).toHaveText(
      `${sha} feature: change conflict.txt · commit being replayed`,
    )
  })
})
