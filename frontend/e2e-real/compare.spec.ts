import type { Locator } from '@playwright/test'
import { test, expect } from './fixtures'
import { logRow, shortRev } from './helpers'

// Comparing two commits in the main window's side column.

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

test('FR-CMP-01 a click, then a Ctrl-click on another commit, compares them and lists the changed files with their type', async ({
  app,
  git,
  repos,
}) => {
  await logRow(app, 'initial: add base files').click()
  await logRow(app, 'feature: add feature.txt').click({ modifiers: ['Control'] })
  const view = app.locator('.compare-view')
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
