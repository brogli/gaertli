// Requirements: FR-BR-11, FR-BR-12
import { test, expect } from './fixtures'
import type { Git, Repos } from './repos'
import { commitFiles, currentBranch, logRow, notice, parents, pickRefAction, rev } from './helpers'

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

test('Merge into main merges the branch and says so', async ({ openApp, repos, git }) => {
  divergeFromMain(git, repos, 'dev1')
  const app = await openApp(repos.work)

  await pickRefAction(app, 'dev1', 'Merge into main')

  await expect(notice(app)).toContainText('Merged dev1 into main')
  await expect(logRow(app, "Merge branch 'dev1'")).toBeVisible()
})

test('Rebase onto main replays the current branch on main and says so', async ({
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
