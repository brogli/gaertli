import fs from 'node:fs'
import path from 'node:path'
import type { Locator, Page } from '@playwright/test'
import { test, expect } from './fixtures'
import { contextMenuItems, logRow, pickFromContextMenu } from './helpers'

// Comparing two commits, or a commit with the working tree, in the main
// window's side column. The compare-diff window is a native window and is
// not covered here.

const compareView = (page: Page) => page.locator('.compare-view')

// listedFiles is a file list's rows as "<status> <path>", sorted.
async function listedFiles(view: Locator): Promise<string[]> {
  const rows = await view
    .locator('.files li')
    .evaluateAll((lis) =>
      lis.map(
        (li) =>
          `${li.querySelector('.status')?.textContent} ${li.querySelector('.path')?.textContent}`,
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
  await expect(view.locator('.from')).toHaveText(
    git(repos.work, 'rev-parse', '--short', 'v1~2').trim(),
  )
  await expect(view.locator('.to')).toHaveText(
    git(repos.work, 'rev-parse', '--short', 'feature').trim(),
  )
  await expect
    .poll(() => listedFiles(view))
    .toEqual(['A feature.txt', 'A logo.png', 'M conflict.txt', 'M docs/readme.md', 'M src/main.go'])
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
    .toEqual(['D crlf.txt', 'R nonl.txt → moved.txt'])
})

test('FR-CMP-01 "Compare with selected" on another commit compares it with the selected one', async ({
  app,
  git,
  repos,
}) => {
  await logRow(app, 'add logo and func a').click()
  await pickFromContextMenu(app, logRow(app, 'docs and func b'), 'Compare with selected')
  const view = compareView(app)
  await expect(view.locator('.from')).toHaveText(
    git(repos.work, 'rev-parse', '--short', 'v1~1').trim(),
  )
  await expect(view.locator('.to')).toHaveText(git(repos.work, 'rev-parse', '--short', 'v1').trim())
  await expect.poll(() => listedFiles(view)).toEqual(['M docs/readme.md', 'M src/main.go'])
})

test('FR-CMP-01 "Compare with selected" is offered neither with nothing selected nor on the selected commit itself', async ({
  app,
}) => {
  const row = logRow(app, 'add logo and func a')
  const withNothingSelected = await contextMenuItems(app, row)
  await app.keyboard.press('Escape')
  await expect(row).toHaveClass(/selected/)
  const onTheSelectedCommit = await contextMenuItems(app, row)

  expect(withNothingSelected).toContain('Copy hash')
  expect(withNothingSelected).not.toContain('Compare with selected')
  expect(onTheSelectedCommit).not.toContain('Compare with selected')
})

test('FR-CMP-02 a click on a compared file previews its diff between the two commits; Escape leaves the compare view', async ({
  app,
}) => {
  await ctrlClickCompare(app, 'initial: add base files', 'feature: add feature.txt')
  await compareView(app).locator('.files li', { hasText: 'feature.txt' }).click()
  await expect(app.locator('.side-column .cm-content')).toContainText('feature only')

  await app.keyboard.press('Escape')
  await expect(compareView(app)).toBeHidden()
  await expect(app.locator('.commit-details')).toBeVisible()
})

test('FR-CMP-03 the compare view shows from → to; Swap reverses the direction', async ({
  app,
  git,
  repos,
}) => {
  await ctrlClickCompare(app, 'initial: add base files', 'feature: add feature.txt')
  const view = compareView(app)
  const initial = git(repos.work, 'rev-parse', '--short', 'v1~2').trim()
  const feature = git(repos.work, 'rev-parse', '--short', 'feature').trim()
  await expect(view.locator('header')).toContainText(`${initial} → ${feature}`)
  await expect.poll(() => listedFiles(view)).toContain('A feature.txt')

  await view.locator('[data-test=swap]').click()
  await expect(view.locator('header')).toContainText(`${feature} → ${initial}`)
  await expect.poll(() => listedFiles(view)).toContain('D feature.txt')
})

test('FR-CMP-04 compare with working tree lists committed and uncommitted differences, leaves out untracked files and says so', async ({
  app,
  git,
  repos,
}) => {
  const hunks = path.join(repos.work, 'hunks.txt')
  fs.writeFileSync(hunks, fs.readFileSync(hunks, 'utf8').replace('line 7\n', 'line 7 changed\n'))
  fs.writeFileSync(path.join(repos.work, 'untracked.txt'), 'untracked\n')

  await pickFromContextMenu(app, logRow(app, 'add logo and func a'), 'Compare with working tree')
  const view = app.locator('.compare-working-view')
  await expect(view.locator('header')).toContainText(
    `${git(repos.work, 'rev-parse', '--short', 'v1~1').trim()} add logo and func a → working tree`,
  )
  await expect
    .poll(() => listedFiles(view))
    .toEqual(['A side.txt', 'M conflict.txt', 'M docs/readme.md', 'M hunks.txt', 'M src/main.go'])
  await expect(view).toContainText('New, untracked files are not part of this comparison.')

  await view.locator('.files li', { hasText: 'hunks.txt' }).click()
  await expect(app.locator('.side-column .cm-content')).toContainText('line 7 changed')
})
