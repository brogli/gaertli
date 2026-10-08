import type { Locator, Page } from '@playwright/test'
import { test, expect } from './fixtures'
import { commitFiles, commitSha, logRow, pickRefAction } from './helpers'
import { gitAfterHistory } from './repos'

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

test('FR-LOG-05 a search combines with an active filter: a hash outside it is "No match", one inside is highlighted and the filtered history still shows in full', async ({
  app,
  git,
  repos,
}) => {
  await pickRefAction(app, 'v1', 'Show history')
  const v1History = ['initial: add base files', 'add logo and func a', 'docs and func b']
  await expectSubjects(app, v1History)

  const search = app.getByPlaceholder('Search message')
  const label = app.locator('[data-test="match-label"]')

  await search.fill(commitSha(git, repos.work, 'side: add side.txt')) // outside v1's history
  await expect(label).toHaveText('No match')

  await search.fill(commitSha(git, repos.work, 'docs and func b')) // inside it
  await expect(label).toHaveText('1 of 1')
  await expect(logRow(app, 'docs and func b')).toHaveClass(/match-current/)
  await expect(logRow(app, 'docs and func b')).toBeInViewport()
  await expectSubjects(app, v1History) // still the whole filtered history
})

test('FR-LOG-06 FR-LOG-07 FR-LOG-08 branch and path filters narrow the log, combine to their intersection and clear one by one', async ({
  app,
}) => {
  const path = app.getByPlaceholder('Path')
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

  await path.fill('conflict.txt')
  await expectSubjects(app, ['initial: add base files', 'feature: change conflict.txt'])

  await app.locator('[data-test=clear-path]').click()
  await expect(path).toHaveValue('')
  await expect.poll(() => shownSubjects(app)).not.toEqual([...allSubjects].sort())

  await app.locator('.p-multiselect-clear-icon').click()
  await expectSubjects(app, allSubjects)

  await path.fill('docs')
  await expectSubjects(app, ['initial: add base files', 'docs and func b'])
})

test('FR-REPO-04 a commit, and a tag created or deleted, outside the app show in the log by themselves', async ({
  app,
  git,
  repos,
  sandbox,
}) => {
  commitFiles(gitAfterHistory(sandbox.env), repos.work, 'outside commit', {
    'outside.txt': 'outside\n',
  })
  await expect(app.locator('.row .subject').first()).toHaveText('outside commit')

  const head = logRow(app, 'outside commit')
  git(repos.work, 'tag', 'outside')
  await expect.poll(() => refNames(head)).toContain('outside')

  git(repos.work, 'tag', '-d', 'outside')
  await expect.poll(() => refNames(head)).not.toContain('outside')
})
