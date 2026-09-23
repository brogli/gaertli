import fs from 'node:fs'
import path from 'node:path'
import type { Locator, Page } from '@playwright/test'
import { test, expect } from './fixtures'
import { contextMenuItems, logRow, pickFromContextMenu } from './helpers'
import { gitWith } from './repos'

// The log's rows, search, filters and copy actions against work's real
// history (8 commits over main, feature and side).

const allSubjects = [
  'initial: add base files',
  'add logo and func a',
  'docs and func b',
  'feature: change conflict.txt',
  'feature: add feature.txt',
  'main: change conflict.txt',
  'side: add side.txt',
  "Merge branch 'side'",
]

// shownSubjects is the log's subjects, sorted: commits made within one
// second have no fixed order among branches.
async function shownSubjects(page: Page): Promise<string[]> {
  return (await page.locator('.row .subject').allTextContents()).sort()
}

function expectSubjects(page: Page, subjects: string[]) {
  return expect.poll(() => shownSubjects(page)).toEqual([...subjects].sort())
}

// refNames is every ref label on row: the ones shown and those behind "+N".
async function refNames(row: Locator): Promise<string[]> {
  const more = row.locator('.ref-more')
  if ((await more.count()) > 0) return (await more.getAttribute('title'))!.split(', ')
  return row.locator('.ref-label').allTextContents()
}

const graphs = (page: Page) => page.locator('.row .graph-cell')
const chips = (page: Page) => page.locator('[data-test^=chip-]')

test('FR-LOG-01 a row shows the graph, subject, author, date, short hash and ref labels', async ({
  app,
  git,
  repos,
}) => {
  const merge = logRow(app, "Merge branch 'side'")
  await expect(merge.locator('.graph-cell')).toBeVisible()
  await expect(merge.locator('.author')).toHaveText('Bob')
  await expect(merge.locator('.date')).not.toBeEmpty()
  await expect(merge.locator('.hash')).toHaveText(
    git(repos.work, 'rev-parse', '--short', 'main').trim(),
  )
  expect(await refNames(merge)).toEqual(['HEAD', 'main', 'origin/main'])
  await expect(logRow(app, 'docs and func b').locator('.ref-label')).toContainText(['v1'])
  await expect(logRow(app, 'side: add side.txt').locator('.author')).toHaveText('Ann')
})

test('FR-LOG-05 a message search narrows the log to a flat list with its chip; clearing it brings all back', async ({
  app,
}) => {
  const search = app.getByPlaceholder('Search message')
  await search.fill('conflict')
  await expectSubjects(app, ['feature: change conflict.txt', 'main: change conflict.txt'])
  await expect(graphs(app)).toHaveCount(0)
  await expect(app.locator('[data-test=chip-query]')).toHaveText('conflict')

  await search.fill('')
  await expectSubjects(app, allSubjects)
  await expect(graphs(app).first()).toBeVisible()
})

test("FR-LOG-05 the author field suggests known authors; picking one shows only that author's commits", async ({
  app,
}) => {
  await app.getByPlaceholder('Author').pressSequentially('Bo')
  const bob = app.getByRole('option', { name: 'Bob' })
  await expect(bob).toBeVisible()
  await bob.click()
  await expectSubjects(app, ["Merge branch 'side'"])

  await app.locator('[data-test=clear-filters]').click()
  await expectSubjects(app, allSubjects)
  await expect(app.getByPlaceholder('Author')).toHaveValue('')
})

test('FR-LOG-05 a short or a full hash finds exactly that commit', async ({ app, git, repos }) => {
  const search = app.getByPlaceholder('Search message')
  await search.fill(git(repos.work, 'rev-parse', '--short', 'side').trim())
  await expectSubjects(app, ['side: add side.txt'])
  await search.fill(git(repos.work, 'rev-parse', 'feature').trim())
  await expectSubjects(app, ['feature: add feature.txt'])
})

test("FR-LOG-06 a branch filter shows that branch's own history, graph included", async ({
  app,
}) => {
  await app.locator('.filter-bar .p-multiselect').click()
  await app.getByRole('option', { name: 'feature', exact: true }).click()
  await app.keyboard.press('Escape')
  await expectSubjects(app, [
    'initial: add base files',
    'add logo and func a',
    'docs and func b',
    'feature: change conflict.txt',
    'feature: add feature.txt',
  ])
  await expect(graphs(app).first()).toBeVisible()
  await expect(app.locator('[data-test=chip-branches]')).toHaveText('feature')
})

test('FR-LOG-07 a path filter shows only the commits touching that folder', async ({ app }) => {
  await app.getByPlaceholder('Path').fill('docs')
  await expectSubjects(app, ['initial: add base files', 'docs and func b'])
})

test('FR-LOG-08 filters combine to their intersection, show one chip each, and Escape clears them all', async ({
  app,
}) => {
  const search = app.getByPlaceholder('Search message')
  await search.fill('func')
  await app.getByPlaceholder('Path').fill('src')
  await expectSubjects(app, ['add logo and func a', 'docs and func b'])
  await expect(chips(app)).toHaveText(['func', 'src'])

  await search.press('Escape')
  await expectSubjects(app, allSubjects)
  await expect(chips(app)).toHaveCount(0)
  await expect(search).toHaveValue('')
})

test('FR-LOG-12 Copy hash and Copy subject put them on the clipboard', async ({
  app,
  context,
  git,
  repos,
}) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write'])
  const row = logRow(app, 'add logo and func a')
  const clipboard = () => app.evaluate(() => navigator.clipboard.readText())

  expect(await contextMenuItems(app, row)).toEqual(
    expect.arrayContaining(['Copy hash', 'Copy subject']),
  )
  await app.keyboard.press('Escape')
  await pickFromContextMenu(app, row, 'Copy subject')
  await expect.poll(clipboard).toBe('add logo and func a')

  await pickFromContextMenu(app, row, 'Copy hash')
  const sha = git(repos.work, 'rev-parse', 'v1~1').trim()
  await expect.poll(clipboard).toBe(sha)
})

test('FR-REPO-04 a commit made outside the app appears at the top of the log by itself', async ({
  app,
  repos,
  sandbox,
}) => {
  // Two seconds past the history, so the new commit sorts first in
  // --topo-order even when the history was built in the same second.
  const later = `${Math.floor(Date.now() / 1000) + 2} +0000`
  const git = gitWith({ ...sandbox.env, GIT_AUTHOR_DATE: later, GIT_COMMITTER_DATE: later })
  fs.writeFileSync(path.join(repos.work, 'outside.txt'), 'outside\n')
  git(repos.work, 'add', 'outside.txt')
  git(repos.work, 'commit', '-q', '-m', 'outside commit')

  await expect(app.locator('.row .subject').first()).toHaveText('outside commit')
})

test('FR-REPO-04 a tag created or deleted outside the app shows on or leaves its row by itself', async ({
  app,
  git,
  repos,
}) => {
  const head = logRow(app, "Merge branch 'side'")
  git(repos.work, 'tag', 'outside')
  await expect.poll(() => refNames(head)).toContain('outside')

  git(repos.work, 'tag', '-d', 'outside')
  await expect.poll(() => refNames(head)).not.toContain('outside')
})
