import fs from 'node:fs'
import path from 'node:path'
import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'
import {
  commitFiles,
  commitSha,
  diffText,
  hunksText,
} from './helpers'
import type { Git, Repos } from './repos'

// The diff window and every way of opening a file in the external editor.
// Editor launches go to the sandbox's stand-in editor.

const rows = Array.from({ length: 200 }, (_, i) => `row ${i + 1}`)

// commitFarChanges adds long.txt, then commits "two far changes": rows 5
// and 180 of long.txt changed, and src/main.go given a function.
function commitFarChanges(git: Git, repos: Repos): void {
  commitFiles(git, repos.work, 'add long.txt', { 'long.txt': rows.join('\n') + '\n' })
  const changed = rows.map((r, i) => (i === 4 || i === 179 ? `${r} CHANGED` : r))
  commitFiles(git, repos.work, 'two far changes', {
    'long.txt': changed.join('\n') + '\n',
    'src/main.go': 'package main\n\nfunc main() {}\n',
  })
}

const diffPane = (page: Page) => page.locator('.diff-view')
const diffLine = (page: Page, text: string) =>
  diffPane(page).locator('.cm-line', { hasText: text }).first()
const selectedFile = (page: Page) => page.locator('.file-list li.selected')

test("FR-DIFF-07 the diff window lists a commit's files beside the diff", async ({
  openDiff,
  git,
  repos,
}) => {
  const diff = await openDiff(repos.work, {
    kind: 'commit',
    sha: commitSha(git, repos.work, 'docs and func b'),
    file: 'docs/readme.md',
  })

  await expect(diff.locator('.file-list .path')).toHaveText(['docs/readme.md', 'src/main.go'])
  await expect(selectedFile(diff)).toContainText('docs/readme.md')
  await expect(diffText(diff)).toContainText('more docs')
})

test('FR-DIFF-02 FR-ED-09 next/previous change jump between far changes; "Open in editor" opens the file at the clicked line', async ({
  openDiff,
  git,
  repos,
  editorCalls,
}) => {
  commitFarChanges(git, repos)
  const diff = await openDiff(repos.work, {
    kind: 'commit',
    sha: commitSha(git, repos.work, 'two far changes'),
    file: 'long.txt',
  })
  await expect(diffLine(diff, 'row 5 CHANGED')).toBeInViewport()

  await diff.locator('[data-test=next-change]').click()
  await diff.locator('[data-test=next-change]').click()
  await expect(diffLine(diff, 'row 180 CHANGED')).toBeInViewport()
  await expect(diffLine(diff, 'row 5 CHANGED')).not.toBeInViewport()
  await diff.locator('[data-test=prev-change]').click()
  await expect(diffLine(diff, 'row 5 CHANGED')).toBeInViewport()
  await diff.locator('[data-test=next-change]').click()
  await expect(diffLine(diff, 'row 180 CHANGED')).toBeInViewport()

  await diffLine(diff, 'row 180 CHANGED').click()
  await diff.locator('.diff-toolbar [data-test=open-editor]').click()
  await expect
    .poll(editorCalls)
    .toEqual([`stand-in-editor ${path.join(repos.work, 'long.txt')}:180`])
})

test('FR-DIFF-06 "ignore whitespace" hides a whitespace-only change', async ({
  openDiff,
  repos,
}) => {
  fs.writeFileSync(path.join(repos.work, 'hunks.txt'), hunksText({ 7: 'line 7   ' }))
  const diff = await openDiff(repos.work, { kind: 'working', file: 'hunks.txt' })
  const changed = diffPane(diff).locator('.cm-changedLine, .cm-deletedChunk')
  await expect(changed.first()).toBeVisible()

  await diff.locator('[data-test=ignore-whitespace-toggle]').click()
  await expect(changed).toHaveCount(0)
  await diff.locator('[data-test=ignore-whitespace-toggle]').click()
  await expect(changed.first()).toBeVisible()
})
