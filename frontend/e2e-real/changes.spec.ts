// Requirements: FR-CHG-01, FR-CHG-02, FR-CHG-03, FR-CHG-04, FR-CHG-10, FR-REPO-04
import fs from 'node:fs'
import path from 'node:path'
import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'
import type { Git } from './repos'
import {
  clickConfirm,
  confirmDialog,
  diffRow,
  diffText,
  fileGroup,
  fileRow,
  fileText,
} from './helpers'

// The changes view on work's real working tree: groups, staging whole
// files and discards; and the working tree's diff window: diffs, hunks,
// outside edits.

const stageHunk = (page: Page) => page.locator('[data-test=stage-hunk]')

// hunksWith returns hunks.txt (line 1 … line 30) with the given lines
// suffixed " EDITED" and extra lines inserted after line 25.
function hunksWith(edited: number[], insertedAfter25: string[] = []): string {
  const lines = Array.from({ length: 30 }, (_, i) =>
    edited.includes(i + 1) ? `line ${i + 1} EDITED` : `line ${i + 1}`,
  )
  lines.splice(25, 0, ...insertedAfter25)
  return lines.map((l) => `${l}\n`).join('')
}

// changeEveryKind leaves work with each kind of change: edited tracked
// files (five change blocks in hunks.txt, CRLF, no final newline), a deleted
// file, an untracked file, a staged new file and a staged rename.
function changeEveryKind(work: string, git: Git): void {
  const write = (file: string, text: string) => fs.writeFileSync(path.join(work, file), text)
  write('hunks.txt', hunksWith([5, 15, 18, 28], ['add1', 'add2', 'add3']))
  write('crlf.txt', 'a\r\nB\r\nc\r\n')
  write('nonl.txt', 'no newline at END')
  write('new.txt', 'brand new\nsecond\n')
  fs.rmSync(path.join(work, 'docs/readme.md'))
  write('added.txt', 'added\n')
  git(work, 'add', 'added.txt')
  git(work, 'mv', 'src/main.go', 'src/app.go')
}

const cachedDiff = (git: Git, work: string) => git(work, 'diff', '--cached', '-U0', 'hunks.txt')

const unstaged = (file: string) => ({ kind: 'working', file })

test('the changes view lists staged, unstaged, untracked and conflicted files; staging moves them between the groups and the index, singly or all at once', async ({
  openApp,
  repos,
  git,
}) => {
  changeEveryKind(repos.work, git)
  const app = await openApp(repos.work)
  const stagedNames = () => git(repos.work, 'diff', '--cached', '--name-only')

  await expect(fileGroup(app, 'Staged').locator('.path')).toHaveText([
    'added.txt',
    'src/main.go → src/app.go',
  ])
  await expect(fileGroup(app, 'Unstaged').locator('.path')).toHaveText([
    'crlf.txt',
    'docs/readme.md',
    'hunks.txt',
    'nonl.txt',
  ])
  await expect(fileGroup(app, 'Untracked').locator('.path')).toHaveText(['new.txt'])
  await expect(app.locator('.changes-body h3', { hasText: 'Conflicted' })).toBeVisible()

  await fileRow(app, 'Unstaged', 'nonl.txt').locator('[data-test=action]').click()
  await expect(fileRow(app, 'Staged', 'nonl.txt')).toBeVisible()
  expect(stagedNames()).toContain('nonl.txt')

  await fileRow(app, 'Staged', 'nonl.txt').locator('[data-test=action]').click()
  await expect(fileRow(app, 'Unstaged', 'nonl.txt')).toBeVisible()
  expect(stagedNames()).not.toContain('nonl.txt')

  await app.getByRole('button', { name: 'Stage all', exact: true }).click()
  await expect(fileGroup(app, 'Staged').locator('li')).toHaveCount(7)
  await expect(app.getByText('No unstaged changes')).toBeVisible()
  await expect(app.getByText('No untracked files')).toBeVisible()
  expect(git(repos.work, 'status', '--porcelain')).not.toMatch(/^.[^ ]|^\?\?/m)

  await app.getByRole('button', { name: 'Unstage all' }).click()
  await expect(app.getByText('No staged changes')).toBeVisible()
  expect(stagedNames()).toBe('')
})

test('Stage hunk stages only its own block, even one two lines from the next', async ({
  openDiff,
  repos,
  git,
}) => {
  changeEveryKind(repos.work, git)
  const diff = await openDiff(repos.work, unstaged('hunks.txt'))

  await expect(stageHunk(diff)).toHaveCount(5)
  await stageHunk(diff).first().click()
  await expect.poll(() => cachedDiff(git, repos.work)).toContain('+line 5 EDITED')
  expect(cachedDiff(git, repos.work)).not.toContain('line 15')
  await expect(diffRow(diff, 'Staged', 'hunks.txt')).toBeVisible()

  await expect(stageHunk(diff)).toHaveCount(4)
  await stageHunk(diff).first().click()
  await expect.poll(() => cachedDiff(git, repos.work)).toContain('+line 15 EDITED')
  expect(cachedDiff(git, repos.work)).not.toContain('line 18')
})

test("the diff window shows a changed file's diff, lists the other changes, shows an untracked file fully added, and follows an outside edit and its list", async ({
  openDiff,
  repos,
  git,
}) => {
  const file = (name: string) => path.join(repos.work, name)
  changeEveryKind(repos.work, git)
  fs.appendFileSync(file('hunks.txt'), 'first edit\n')
  const diff = await openDiff(repos.work, unstaged('new.txt'))

  await expect(diffText(diff)).toContainText('brand new')
  await expect(diffText(diff)).toContainText('second')
  await expect(fileGroup(diff, 'Staged', '.file-list').locator('.path')).toHaveText([
    'added.txt',
    'src/main.go → src/app.go',
  ])
  await diffRow(diff, 'Unstaged', 'docs/readme.md').click()
  await expect(diffText(diff)).toContainText('more docs')

  const cases: [group: string, name: string, shows: string][] = [
    ['Untracked', 'new.txt', 'brand new'],
    ['Unstaged', 'hunks.txt', 'first edit'],
  ]
  for (const [group, name, shows] of cases) {
    await diffRow(diff, group, name).click()
    await expect(diffText(diff)).toContainText(shows)
    fs.appendFileSync(file(name), `${name} from outside\n`)
    await expect(diffText(diff)).toContainText(`${name} from outside`)
  }

  fs.writeFileSync(file('later.txt'), 'later\n')
  await expect(diffRow(diff, 'Untracked', 'later.txt')).toBeVisible()
})

// discard clicks a row's Discard, checks the confirmation names the file,
// and confirms.
async function discard(page: Page, group: string, file: string): Promise<void> {
  await fileRow(page, group, file).locator('[data-test=second-action]').click()
  await expect(confirmDialog(page)).toContainText(file)
  await clickConfirm(page)
}

test('discarding a file asks first: an unstaged one is restored, an untracked one, also in an untracked folder, is deleted', async ({
  openApp,
  repos,
  git,
}) => {
  const work = (name: string) => path.join(repos.work, name)
  fs.writeFileSync(work('conflict.txt'), 'local edit\n')
  fs.writeFileSync(work('new.txt'), 'kept\n')
  fs.writeFileSync(work('junk.txt'), 'junk\n')
  fs.mkdirSync(work('junkdir'))
  fs.writeFileSync(work('junkdir/a.txt'), 'a\n')
  const app = await openApp(repos.work)

  await discard(app, 'Unstaged', 'conflict.txt')
  await expect.poll(() => fileText(work('conflict.txt'))).toBe('main side\n')
  await discard(app, 'Untracked', 'junk.txt')
  await expect.poll(() => fs.existsSync(work('junk.txt'))).toBe(false)
  await discard(app, 'Untracked', 'junkdir/a.txt')
  await expect.poll(() => fs.existsSync(work('junkdir/a.txt'))).toBe(false)
  expect(git(repos.work, 'status', '--porcelain')).toBe('?? new.txt\n')
  expect(fs.readFileSync(work('new.txt'), 'utf8')).toBe('kept\n')
})
