import fs from 'node:fs'
import path from 'node:path'
import type { Locator, Page } from '@playwright/test'
import { test, expect } from './fixtures'
import {
  boundCallEndingWith,
  diffText,
  hunksText,
  logRow,
  pickFromContextMenu,
  rev,
  shortRev,
} from './helpers'

// Comparing two commits, or a commit with the working tree, in the main
// window's side column. A double-click's diff window is a native window:
// the call that opens it is observed, and its URL opened in a page.

const compareView = (page: Page) => page.locator('.compare-view')

// listedFiles is a file list's rows as "<status label> <path>", sorted.
async function listedFiles(view: Locator): Promise<string[]> {
  const rows = await view
    .locator('.files li')
    .evaluateAll((lis) =>
      lis.map(
        (li) =>
          `${li.querySelector('.status')?.getAttribute('title')} ${li.querySelector('.path')?.textContent}`,
      ),
    )
  return rows.map((row) => row.replace(/\s+/g, ' ').trim()).sort()
}

async function ctrlClickCompare(page: Page, from: string, to: string): Promise<void> {
  await logRow(page, from).click()
  await logRow(page, to).click({ modifiers: ['Control'] })
}

test('FR-CMP-01 a click, then a Ctrl-click on another commit, compares them and lists the changed files with their type', async ({
  app,
  git,
  repos,
}) => {
  await ctrlClickCompare(app, 'initial: add base files', 'feature: add feature.txt')
  const view = compareView(app)
  await expect(view.locator('.from')).toHaveText(shortRev(git, repos.work, 'v1~2'))
  await expect(view.locator('.to')).toHaveText(shortRev(git, repos.work, 'feature'))
  await expect
    .poll(() => listedFiles(view))
    .toEqual([
      'Added feature.txt',
      'Added logo.png',
      'Modified conflict.txt',
      'Modified docs/readme.md',
      'Modified src/main.go',
    ])
})

test('FR-CMP-01 the compared files mark a deletion, and a rename with its old path', async ({
  app,
  git,
  repos,
}) => {
  git(repos.work, 'mv', 'nonl.txt', 'moved.txt')
  git(repos.work, 'rm', '-q', 'crlf.txt')
  git(repos.work, 'commit', '-q', '-m', 'move and delete')

  await ctrlClickCompare(app, "Merge branch 'side'", 'move and delete')
  await expect
    .poll(() => listedFiles(compareView(app)))
    .toEqual(['Deleted crlf.txt', 'Renamed nonl.txt → moved.txt'])
})

test('FR-CMP-01 "Compare with <hash>" on another commit compares it with the selected one', async ({
  app,
  git,
  repos,
}) => {
  const from = shortRev(git, repos.work, 'v1~1')
  await logRow(app, 'add logo and func a').click()
  await pickFromContextMenu(app, logRow(app, 'docs and func b'), `Compare with ${from}`)
  const view = compareView(app)
  await expect(view.locator('.from')).toHaveText(from)
  await expect(view.locator('.to')).toHaveText(shortRev(git, repos.work, 'v1'))
  await expect
    .poll(() => listedFiles(view))
    .toEqual(['Modified docs/readme.md', 'Modified src/main.go'])
})

test('FR-CMP-02 a click on a compared file shows no diff; a double-click opens its diff window; Escape leaves the comparison', async ({
  app,
  git,
  repos,
}) => {
  await ctrlClickCompare(app, 'initial: add base files', 'feature: add feature.txt')
  const file = compareView(app).locator('.files li', { hasText: 'feature.txt' })
  await file.click()
  await expect(file).toHaveClass(/selected/)
  await expect(app.locator('.side-column .diff-view')).toHaveCount(0)

  const [from, to] = ['v1~2', 'feature'].map((ref) => rev(git, repos.work, ref))
  const openDiffWindow = boundCallEndingWith(app, [
    { kind: 'range', from, to, path: 'feature.txt', oldPath: '' },
  ])
  await file.dblclick()
  await openDiffWindow
  await expect(app.locator('.side-column .diff-view')).toHaveCount(0)

  await app.keyboard.press('Escape')
  await expect(compareView(app)).toBeHidden()
  await expect(app.locator('.commit-details')).toBeHidden() // the whole selection is dropped
})

test('FR-CMP-04 compare with working tree lists committed and uncommitted differences, leaves out untracked files and says so', async ({
  app,
  openDiff,
  git,
  repos,
}) => {
  fs.writeFileSync(path.join(repos.work, 'hunks.txt'), hunksText({ 7: 'line 7 changed' }))
  fs.writeFileSync(path.join(repos.work, 'untracked.txt'), 'untracked\n')

  await pickFromContextMenu(app, logRow(app, 'add logo and func a'), 'Compare with working tree')
  const view = app.locator('.compare-working-view')
  await expect(view.locator('header')).toContainText(
    `${shortRev(git, repos.work, 'v1~1')} add logo and func a → working tree`,
  )
  await expect
    .poll(() => listedFiles(view))
    .toEqual([
      'Added side.txt',
      'Modified conflict.txt',
      'Modified docs/readme.md',
      'Modified hunks.txt',
      'Modified src/main.go',
    ])
  await expect(view).toContainText('New, untracked files are not part of this comparison.')

  const file = view.locator('.files li', { hasText: 'hunks.txt' })
  await file.click()
  await expect(file).toHaveClass(/selected/)
  await expect(app.locator('.diff-view')).toHaveCount(0)
  const sha = rev(git, repos.work, 'v1~1')
  const openDiffWindow = boundCallEndingWith(app, [
    { kind: 'againstWorking', sha, path: 'hunks.txt', oldPath: '' },
  ])
  await file.dblclick()
  await openDiffWindow

  const diff = await openDiff(repos.work, { kind: 'againstWorking', sha, file: 'hunks.txt' })
  await expect(diff.locator('.file-list li')).toHaveCount(5)
  await expect(diffText(diff)).toContainText('line 7 changed')
})
