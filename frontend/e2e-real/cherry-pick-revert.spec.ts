import fs from 'node:fs'
import path from 'node:path'
import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'
import {
  commitFiles,
  commitSha,
  conflictBanner,
  fileRow,
  logRow,
  pickFromContextMenu,
  rev,
  switchToChanges,
} from './helpers'

// Cherry-pick and revert from the log's context menu, with and without
// the options dialog, checked against the real repo.

function logError(page: Page) {
  return page.locator('.log-column .error-banner')
}

// selectRange clicks first, then shift-clicks last.
async function selectRange(page: Page, first: string, last: string): Promise<void> {
  await logRow(page, first).click()
  await logRow(page, last).click({ modifiers: ['Shift'] })
}

test('FR-CP-01 cherry-picking a commit adds a commit with its message on top of the branch', async ({
  app,
  repos,
  git,
}) => {
  const head = rev(git, repos.work, 'HEAD')

  await pickFromContextMenu(app, logRow(app, 'feature: add feature.txt'), 'Cherry-pick')

  await expect.poll(() => rev(git, repos.work, 'HEAD~1')).toBe(head)
  expect(git(repos.work, 'log', '-1', '--format=%s').trim()).toBe('feature: add feature.txt')
})

test("FR-CP-04 a pick git refuses to start shows git's message, applies nothing and keeps the selection", async ({
  app,
  repos,
  git,
}) => {
  fs.writeFileSync(path.join(repos.work, 'feature.txt'), 'untracked local\n')
  const head = rev(git, repos.work, 'HEAD')
  const row = logRow(app, 'feature: add feature.txt')
  await row.click()

  await pickFromContextMenu(app, row, 'Cherry-pick')

  await expect(logError(app)).toContainText(/untracked working tree files would be overwritten/)
  expect(rev(git, repos.work, 'HEAD')).toBe(head)
  await expect(row).toHaveClass(/\bselected\b/)
})

test('FR-CP-03 a cherry-pick that stops on conflicts shows the operation and the conflicted file', async ({
  app,
}) => {
  await pickFromContextMenu(app, logRow(app, 'feature: change conflict.txt'), 'Cherry-pick')

  await expect(app.locator('[data-test=branch-picker]')).toContainText('main (cherry-pick)')
  await switchToChanges(app)
  await expect(fileRow(app, 'Conflicted', 'conflict.txt')).toBeVisible()
})

test('FR-CP-06 "Cherry-pick…" can add the "cherry picked from" line to the message', async ({
  app,
  repos,
  git,
}) => {
  const picked = commitSha(git, repos.work, 'feature: add feature.txt')
  await pickFromContextMenu(app, logRow(app, 'feature: add feature.txt'), 'Cherry-pick…')
  const dialog = app.getByRole('dialog')
  await expect(dialog.getByLabel('Apply without committing')).toBeVisible()
  await dialog.getByLabel('Add "cherry picked from" line').check()
  await dialog.getByRole('button', { name: 'Confirm' }).click()

  await expect
    .poll(() => git(repos.work, 'log', '-1', '--format=%B').trim())
    .toMatch(new RegExp(`\\(cherry picked from commit ${picked}\\)$`))
})

test('FR-CP-06 "Cherry-pick…" can apply the change without committing', async ({
  app,
  repos,
  git,
}) => {
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

test('FR-RV-04 "Revert…" can apply the reverse change without committing', async ({
  app,
  repos,
  git,
}) => {
  const head = rev(git, repos.work, 'HEAD')
  await pickFromContextMenu(app, logRow(app, 'side: add side.txt'), 'Revert…')
  const dialog = app.getByRole('dialog')
  await dialog.getByLabel('Apply without committing').check()
  await dialog.getByRole('button', { name: 'Confirm' }).click()

  await expect
    .poll(() => git(repos.work, 'diff', '--cached', '--name-status').trim())
    .toBe('D\tside.txt')
  expect(rev(git, repos.work, 'HEAD')).toBe(head)
})

test.describe('with three new commits on main', () => {
  test.beforeEach(({ repos, git }) => {
    for (const n of ['one', 'two', 'three']) {
      commitFiles(git, repos.work, `step ${n}`, { [`${n}.txt`]: `${n}\n` })
    }
  })

  test('FR-RV-01 shift-click selects the range of commits, a plain click collapses it', async ({
    openApp,
    repos,
  }) => {
    const app = await openApp(repos.work)
    const multiSelected = app.locator('.row.multi-selected .subject')

    await selectRange(app, 'step three', 'step one')
    await expect(multiSelected).toHaveText(['step three', 'step two', 'step one'])

    await logRow(app, 'initial').click()
    await expect(multiSelected).toHaveCount(0)
  })

  test('FR-RV-02 reverting a selected range makes one revert per commit, newest first', async ({
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

test('FR-CP-07 Skip drops the conflicting commit of a stopped pick and applies the rest', async ({
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
  const banner = conflictBanner(app)
  for (const button of ['Continue', 'Skip', 'Abort…']) {
    await expect(banner.getByRole('button', { name: button })).toBeVisible()
  }
  await banner.getByRole('button', { name: 'Skip' }).click()

  await expect(banner).toBeHidden()
  await expect
    .poll(() => git(repos.work, 'log', '--reverse', '--format=%s', `${head}..HEAD`).trim())
    .toBe('pick one\npick three')
})
