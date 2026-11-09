// Requirements: FR-CP-01, FR-CP-03, FR-CP-06, FR-CP-07, FR-RV-02
import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'
import {
  commitFiles,
  operationBanner,
  fileRow,
  logRow,
  pickFromContextMenu,
  rev,
  switchToChanges,
} from './helpers'

// Cherry-pick and revert from the log's context menu, with and without
// the options dialog, checked against the real repo.

// selectRange clicks first, then shift-clicks last.
async function selectRange(page: Page, first: string, last: string): Promise<void> {
  await logRow(page, first).click()
  await logRow(page, last).click({ modifiers: ['Shift'] })
}

test('cherry-picking a commit adds a commit with its message on top of the branch', async ({
  app,
  repos,
  git,
}) => {
  const head = rev(git, repos.work, 'HEAD')

  await pickFromContextMenu(app, logRow(app, 'feature: add feature.txt'), 'Cherry-pick')

  await expect.poll(() => rev(git, repos.work, 'HEAD~1')).toBe(head)
  expect(git(repos.work, 'log', '-1', '--format=%s').trim()).toBe('feature: add feature.txt')
})

test('"Cherry-pick…" can apply the change without committing', async ({ app, repos, git }) => {
  const head = rev(git, repos.work, 'HEAD')
  await pickFromContextMenu(app, logRow(app, 'feature: add feature.txt'), 'Cherry-pick…')
  const dialog = app.getByRole('dialog')
  await dialog.getByLabel('Apply without committing').check()
  await dialog.getByRole('button', { name: 'Confirm' }).click()

  await expect
    .poll(() => git(repos.work, 'diff', '--cached', '--name-only').trim())
    .toBe('feature.txt')
  expect(rev(git, repos.work, 'HEAD')).toBe(head)
})

test.describe('with three new commits on main', () => {
  test.beforeEach(({ repos, git }) => {
    for (const n of ['one', 'two', 'three']) {
      commitFiles(git, repos.work, `step ${n}`, { [`${n}.txt`]: `${n}\n` })
    }
  })

  test('reverting a selected range makes one revert per commit, newest first', async ({
    openApp,
    repos,
    git,
  }) => {
    const head = rev(git, repos.work, 'HEAD')
    const app = await openApp(repos.work)

    await selectRange(app, 'step one', 'step three')
    await pickFromContextMenu(app, logRow(app, 'step two'), 'Revert')

    await expect
      .poll(() => git(repos.work, 'log', '--reverse', '--format=%s', `${head}..HEAD`).trim())
      .toBe(['Revert "step three"', 'Revert "step two"', 'Revert "step one"'].join('\n'))
  })
})

test('a pick that stops on conflicts shows the operation and the conflicted file; Skip drops the conflicting commit and applies the rest', async ({
  openApp,
  repos,
  git,
}) => {
  git(repos.work, 'checkout', '-q', '-b', 'picks', 'v1')
  commitFiles(git, repos.work, 'pick one', { 'p1.txt': 'p1\n' })
  commitFiles(git, repos.work, 'pick two conflicts', { 'conflict.txt': 'pick side\n' })
  commitFiles(git, repos.work, 'pick three', { 'p3.txt': 'p3\n' })
  git(repos.work, 'checkout', '-q', 'main')
  const head = rev(git, repos.work, 'HEAD')
  const app = await openApp(repos.work)

  await selectRange(app, 'pick three', 'pick one')
  await pickFromContextMenu(app, logRow(app, 'pick two'), 'Cherry-pick')
  const banner = operationBanner(app)
  for (const button of ['Continue', 'Skip', 'Abort…']) {
    await expect(banner.getByRole('button', { name: button })).toBeVisible()
  }
  await expect(app.locator('[data-test=branch-picker]')).toContainText('main (cherry-pick)')
  await switchToChanges(app)
  await expect(fileRow(app, 'Conflicted', 'conflict.txt')).toBeVisible()
  await banner.getByRole('button', { name: 'Skip' }).click()

  await expect(banner).toBeHidden()
  await expect
    .poll(() => git(repos.work, 'log', '--reverse', '--format=%s', `${head}..HEAD`).trim())
    .toBe('pick one\npick three')
})
