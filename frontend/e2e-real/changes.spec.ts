import fs from 'node:fs'
import path from 'node:path'
import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'
import type { Git } from './repos'
import { confirmDialog, fileGroup, fileRow, logRow } from './helpers'

// The changes panel on work's real working tree: groups, staging whole
// files, hunks and lines, diffs, outside edits and discards.

const diffText = (page: Page) => page.locator('.diff-view .cm-content')
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

test('FR-CHG-01 the changes panel lists staged, unstaged, untracked and conflicted files', async ({
  openApp,
  repos,
  git,
}) => {
  changeEveryKind(repos.work, git)
  const app = await openApp(repos.work)

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
})

test('FR-UX-07 the changes panel collapses and expands', async ({ app }) => {
  const collapse = app.locator('[data-test=changes-collapse]')
  await collapse.click()
  await expect(app.locator('.changes-body')).toBeHidden()
  await collapse.click()
  await expect(app.locator('.changes-body')).toBeVisible()
})

test('FR-CHG-02 staging and unstaging a file moves it between the groups and the index', async ({
  openApp,
  repos,
  git,
}) => {
  changeEveryKind(repos.work, git)
  const app = await openApp(repos.work)
  const staged = () => git(repos.work, 'diff', '--cached', '--name-only')

  await fileRow(app, 'Unstaged', 'nonl.txt').locator('[data-test=action]').click()
  await expect(fileRow(app, 'Staged', 'nonl.txt')).toBeVisible()
  expect(staged()).toContain('nonl.txt')

  await fileRow(app, 'Staged', 'nonl.txt').locator('[data-test=action]').click()
  await expect(fileRow(app, 'Unstaged', 'nonl.txt')).toBeVisible()
  expect(staged()).not.toContain('nonl.txt')
})

test('FR-CHG-02 Stage all and Unstage all move every change at once', async ({
  openApp,
  repos,
  git,
}) => {
  changeEveryKind(repos.work, git)
  const app = await openApp(repos.work)

  await app.getByRole('button', { name: 'Stage all', exact: true }).click()
  await expect(fileGroup(app, 'Staged').locator('li')).toHaveCount(7)
  await expect(app.getByText('No unstaged changes')).toBeVisible()
  await expect(app.getByText('No untracked files')).toBeVisible()
  expect(git(repos.work, 'status', '--porcelain')).not.toMatch(/^.[^ ]|^\?\?/m)

  await app.getByRole('button', { name: 'Unstage all' }).click()
  await expect(app.getByText('No staged changes')).toBeVisible()
  expect(git(repos.work, 'diff', '--cached', '--name-only')).toBe('')
})

test('FR-CHG-04 clicking a changed file shows its diff; an untracked file shows fully added', async ({
  openApp,
  repos,
  git,
}) => {
  changeEveryKind(repos.work, git)
  const app = await openApp(repos.work)

  await fileRow(app, 'Untracked', 'new.txt').click()
  await expect(diffText(app)).toContainText('brand new')
  await expect(diffText(app)).toContainText('second')

  await fileRow(app, 'Unstaged', 'hunks.txt').click()
  await expect(diffText(app)).toContainText('line 5 EDITED')

  await fileRow(app, 'Unstaged', 'docs/readme.md').click()
  await expect(diffText(app)).toContainText('more docs')
})

test('FR-CHG-03 untracked, deleted, added and renamed files offer no hunk or line staging', async ({
  openApp,
  repos,
  git,
}) => {
  changeEveryKind(repos.work, git)
  const app = await openApp(repos.work)
  const cases: [group: string, file: string, shows: string][] = [
    ['Untracked', 'new.txt', 'brand new'],
    ['Unstaged', 'readme.md', 'more docs'],
    ['Staged', 'added.txt', 'added'],
    ['Staged', 'app.go', 'func b() {}'],
  ]
  for (const [group, file, shows] of cases) {
    await fileRow(app, group, file).click()
    await expect(diffText(app)).toContainText(shows)
    await expect(stageHunk(app)).toHaveCount(0)
    await expect(app.locator('[data-test=stage-selected-lines]')).toHaveCount(0)
  }
})

test('FR-CHG-03 Stage hunk stages only its own block, even one two lines from the next', async ({
  openApp,
  repos,
  git,
}) => {
  changeEveryKind(repos.work, git)
  const app = await openApp(repos.work)

  await fileRow(app, 'Unstaged', 'hunks.txt').click()
  await expect(stageHunk(app)).toHaveCount(5)
  await stageHunk(app).first().click()
  await expect.poll(() => cachedDiff(git, repos.work)).toContain('+line 5 EDITED')
  expect(cachedDiff(git, repos.work)).not.toContain('line 15')

  await expect(stageHunk(app)).toHaveCount(4)
  await stageHunk(app).first().click()
  await expect.poll(() => cachedDiff(git, repos.work)).toContain('+line 15 EDITED')
  expect(cachedDiff(git, repos.work)).not.toContain('line 18')
})

test('FR-CHG-03 Stage selected lines stages only the selected lines of a block', async ({
  openApp,
  repos,
  git,
}) => {
  changeEveryKind(repos.work, git)
  const app = await openApp(repos.work)
  const line = (text: string) =>
    app.locator('.diff-view .cm-line', { hasText: new RegExp(`^${text}$`) }).first()

  await fileRow(app, 'Unstaged', 'hunks.txt').click()
  await line('add1').click()
  await line('add2').click({ modifiers: ['Shift'] })
  const stageLines = app.locator('[data-test=stage-selected-lines]')
  await expect(stageLines).toBeEnabled()
  await stageLines.click()

  await expect.poll(() => cachedDiff(git, repos.work)).toContain('+add2')
  const cached = cachedDiff(git, repos.work)
  expect(cached).toContain('+add1')
  expect(cached).not.toContain('+add3')
  expect(cached).not.toContain('EDITED')
})

test('FR-CHG-03 Unstage hunk returns only its block to the working tree', async ({
  openApp,
  repos,
  git,
}) => {
  fs.writeFileSync(path.join(repos.work, 'hunks.txt'), hunksWith([5, 15]))
  git(repos.work, 'add', 'hunks.txt')
  const app = await openApp(repos.work)

  await fileRow(app, 'Staged', 'hunks.txt').click()
  await expect(stageHunk(app)).toHaveText(['Unstage hunk', 'Unstage hunk'])
  await stageHunk(app).first().click()

  await expect.poll(() => cachedDiff(git, repos.work)).not.toContain('line 5 EDITED')
  expect(cachedDiff(git, repos.work)).toContain('+line 15 EDITED')
  expect(fs.readFileSync(path.join(repos.work, 'hunks.txt'), 'utf8')).toBe(hunksWith([5, 15]))
})

test('FR-CHG-03 staging a hunk keeps CRLF endings and a missing final newline exactly', async ({
  openApp,
  repos,
  git,
}) => {
  changeEveryKind(repos.work, git)
  const app = await openApp(repos.work)
  const cases: [file: string, edited: string][] = [
    ['crlf.txt', 'a\r\nB\r\nc\r\n'],
    ['nonl.txt', 'no newline at END'],
  ]
  for (const [file, edited] of cases) {
    await fileRow(app, 'Unstaged', file).click()
    await expect(stageHunk(app)).toHaveCount(1)
    await stageHunk(app).click()
    await expect.poll(() => git(repos.work, 'show', `:${file}`)).toBe(edited)
  }
})

test('FR-REPO-04 an open diff of a changed file follows an outside edit', async ({
  openApp,
  repos,
}) => {
  const file = (name: string) => path.join(repos.work, name)
  fs.writeFileSync(file('new.txt'), 'brand new\n')
  fs.appendFileSync(file('hunks.txt'), 'first edit\n')
  const app = await openApp(repos.work)

  const cases: [group: string, name: string, shows: string][] = [
    ['Untracked', 'new.txt', 'brand new'],
    ['Unstaged', 'hunks.txt', 'first edit'],
  ]
  for (const [group, name, shows] of cases) {
    await fileRow(app, group, name).click()
    await expect(diffText(app)).toContainText(shows)
    fs.appendFileSync(file(name), `${name} from outside\n`)
    await expect(diffText(app)).toContainText(`${name} from outside`)
  }
})

test("FR-LOG-03 clicking a commit while a changed file's diff is open shows the commit's details instead", async ({
  openApp,
  repos,
}) => {
  fs.writeFileSync(path.join(repos.work, 'new.txt'), 'brand new\n')
  const app = await openApp(repos.work)

  await fileRow(app, 'Untracked', 'new.txt').click()
  await expect(diffText(app)).toContainText('brand new')
  await logRow(app, 'add logo').click()
  await expect(app.locator('.commit-details')).toContainText('add logo and func a')
  await expect(app.locator('.diff-view')).toBeHidden()
})

// discard clicks a row's Discard, checks the confirmation names the file,
// and confirms.
async function discard(page: Page, group: string, file: string): Promise<void> {
  await fileRow(page, group, file).locator('[data-test=second-action]').click()
  await expect(confirmDialog(page)).toContainText(file)
  await confirmDialog(page).getByRole('button', { name: 'Confirm' }).click()
}

test('FR-CHG-10 discarding an unstaged file asks first and restores the committed content', async ({
  openApp,
  repos,
  git,
}) => {
  fs.writeFileSync(path.join(repos.work, 'conflict.txt'), 'local edit\n')
  fs.writeFileSync(path.join(repos.work, 'new.txt'), 'kept\n')
  const app = await openApp(repos.work)

  await discard(app, 'Unstaged', 'conflict.txt')
  await expect.poll(() => git(repos.work, 'status', '--porcelain')).toBe('?? new.txt\n')
  expect(fs.readFileSync(path.join(repos.work, 'conflict.txt'), 'utf8')).toBe('main side\n')
})

test('FR-CHG-10 discarding an untracked file, also one in an untracked folder, asks first and deletes it', async ({
  openApp,
  repos,
  git,
}) => {
  const work = (name: string) => path.join(repos.work, name)
  fs.writeFileSync(work('junk.txt'), 'junk\n')
  fs.mkdirSync(work('junkdir'))
  fs.writeFileSync(work('junkdir/a.txt'), 'a\n')
  fs.writeFileSync(work('conflict.txt'), 'kept\n')
  const app = await openApp(repos.work)

  await discard(app, 'Untracked', 'junk.txt')
  await expect.poll(() => fs.existsSync(work('junk.txt'))).toBe(false)
  await discard(app, 'Untracked', 'junkdir/a.txt')
  await expect.poll(() => fs.existsSync(work('junkdir/a.txt'))).toBe(false)
  expect(git(repos.work, 'status', '--porcelain')).toBe(' M conflict.txt\n')
  expect(fs.readFileSync(work('conflict.txt'), 'utf8')).toBe('kept\n')
})

test('FR-CHG-10 Discard hunk asks first and reverts only its block', async ({ openApp, repos }) => {
  const hunks = path.join(repos.work, 'hunks.txt')
  fs.writeFileSync(hunks, hunksWith([5, 20]))
  const app = await openApp(repos.work)

  await fileRow(app, 'Unstaged', 'hunks.txt').click()
  await expect(app.locator('[data-test=discard-hunk]')).toHaveCount(2)
  await app.locator('[data-test=discard-hunk]').first().click()
  await expect(confirmDialog(app)).toContainText('hunks.txt')
  await confirmDialog(app).getByRole('button', { name: 'Confirm' }).click()

  await expect.poll(() => fs.readFileSync(hunks, 'utf8')).toBe(hunksWith([20]))
})
