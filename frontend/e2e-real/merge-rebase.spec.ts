import fs from 'node:fs'
import path from 'node:path'
import { test, expect } from './fixtures'
import type { Git, Repos } from './repos'
import {
  commitFiles,
  conflictBanner,
  currentBranch,
  fileText,
  hunksText,
  logRow,
  notice,
  parents,
  pickRefAction,
  rev,
} from './helpers'

// Merging a branch into the current one and rebasing the current one onto
// a branch, both from the branch picker's row menu.

// divergeFromMain leaves main on its own commit and branch on another, both
// off the same base and touching different files; HEAD is main.
function divergeFromMain(git: Git, repos: Repos, branch: string): void {
  git(repos.work, 'checkout', '-q', '-b', branch)
  commitFiles(git, repos.work, `${branch}: add ${branch}.txt`, { [`${branch}.txt`]: 'b\n' })
  git(repos.work, 'checkout', '-q', 'main')
  commitFiles(git, repos.work, 'main: add m.txt', { 'm.txt': 'm\n' })
}

test('FR-BR-11 Merge into main merges the branch with a merge commit and says so', async ({
  openApp,
  repos,
  git,
}) => {
  divergeFromMain(git, repos, 'dev1')
  const app = await openApp(repos.work)

  await pickRefAction(app, 'dev1', 'Merge into main')

  await expect(notice(app)).toContainText('Merged dev1 into main')
  await expect(logRow(app, "Merge branch 'dev1'")).toBeVisible()
  expect(git(repos.work, 'log', '-1', '--format=%s')).toBe("Merge branch 'dev1'\n")
  expect(parents(git, repos.work)).toBe(2)
})

test('FR-BR-11 a merge that conflicts stops with the conflict banner', async ({
  openApp,
  repos,
}) => {
  const app = await openApp(repos.work)

  await pickRefAction(app, 'feature', 'Merge into main')

  await expect(conflictBanner(app)).toContainText('merge stopped on conflicts')
})

test('FR-BR-12 Rebase onto main replays the current branch on main and says so', async ({
  openApp,
  repos,
  git,
}) => {
  divergeFromMain(git, repos, 'dev1')
  git(repos.work, 'checkout', '-q', 'dev1')
  const app = await openApp(repos.work)

  await pickRefAction(app, 'main', 'Rebase dev1 onto main')

  await expect(notice(app)).toContainText('Rebased dev1 onto main')
  expect(currentBranch(git, repos.work)).toBe('dev1')
  expect(parents(git, repos.work)).toBe(1)
  expect(rev(git, repos.work, 'HEAD~')).toBe(rev(git, repos.work, 'main'))
})

test('FR-BR-12 a rebase that conflicts stops with the conflict banner', async ({
  openApp,
  repos,
  git,
}) => {
  git(repos.work, 'checkout', '-q', 'feature')
  const app = await openApp(repos.work)

  await pickRefAction(app, 'main', 'Rebase feature onto main')

  await expect(conflictBanner(app)).toContainText('rebase stopped on conflicts')
})

test('FR-BR-11 a local change in the way is offered to be stashed, merged around and restored', async ({
  openApp,
  repos,
  git,
}) => {
  git(repos.work, 'checkout', '-q', '-b', 'dev1')
  commitFiles(git, repos.work, 'dev1: hunks line 1', { 'hunks.txt': hunksText({ 1: 'dev1' }) })
  git(repos.work, 'checkout', '-q', 'main')
  commitFiles(git, repos.work, 'main: add m.txt', { 'm.txt': 'm\n' })
  const file = path.join(repos.work, 'hunks.txt')
  fs.writeFileSync(file, hunksText({ 30: 'local' }))
  const app = await openApp(repos.work)

  await pickRefAction(app, 'dev1', 'Merge into main')
  await notice(app).getByRole('button', { name: 'Stash and merge' }).click()

  await expect(notice(app)).toContainText('Merged dev1 into main')
  expect(parents(git, repos.work)).toBe(2)
  await expect.poll(() => fileText(file)).toBe(hunksText({ 1: 'dev1', 30: 'local' }))
})
