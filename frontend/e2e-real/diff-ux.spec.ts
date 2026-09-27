import fs from 'node:fs'
import path from 'node:path'
import type { Page } from '@playwright/test'
import { test, expect } from './fixtures'
import {
  commitFiles,
  commitSha,
  diffText,
  fileRow,
  hunksText,
  logRow,
  notice,
  openSettings,
  switchToChanges,
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
const commitFile = (page: Page, name: string) =>
  page.locator('.commit-details .files li', { hasText: name })
const selectedFile = (page: Page) => page.locator('.file-list li.selected')

test("FR-DIFF-07 the diff window lists what it diffs beside the diff: a commit's files, a comparison's, the working tree's changes", async ({
  openDiff,
  git,
  repos,
}) => {
  fs.appendFileSync(path.join(repos.work, 'hunks.txt'), 'line 31\n')
  const sha = (subject: string) => commitSha(git, repos.work, subject)
  const cases: [target: Record<string, string>, files: string[], shows: string][] = [
    [
      { kind: 'commit', sha: sha('docs and func b'), file: 'docs/readme.md' },
      ['docs/readme.md', 'src/main.go'],
      'more docs',
    ],
    [
      {
        kind: 'range',
        from: sha('add logo and func a'),
        to: sha('docs and func b'),
        file: 'src/main.go',
      },
      ['docs/readme.md', 'src/main.go'],
      'func b() {}',
    ],
    [{ kind: 'working', file: 'hunks.txt' }, ['hunks.txt'], 'line 31'],
  ]
  for (const [target, files, shows] of cases) {
    const diff = await openDiff(repos.work, target)
    await expect(diff.locator('.file-list .path')).toHaveText(files)
    await expect(selectedFile(diff)).toContainText(target.file!)
    await expect(diffText(diff)).toContainText(shows)
  }
})

test('FR-DIFF-02 next/previous change jump between far changes; next/previous file step through the commit', async ({
  openDiff,
  git,
  repos,
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

  await diff.locator('[data-test=next-file]').click()
  await expect(selectedFile(diff)).toContainText('main.go')
  await expect(diffLine(diff, 'func main() {}')).toBeVisible()
  await diff.locator('[data-test=prev-file]').click()
  await expect(selectedFile(diff)).toContainText('long.txt')
})

test('FR-DIFF-03 a binary file shows a placeholder instead of content, and file stepping stays', async ({
  openDiff,
  git,
  repos,
}) => {
  const sha = commitSha(git, repos.work, 'add logo and func a')
  const diff = await openDiff(repos.work, { kind: 'commit', sha, file: 'logo.png' })
  await expect(diffPane(diff)).toContainText('Binary file, no preview available.')
  await expect(diffPane(diff).locator('.cm-editor')).toHaveCount(0)
  await expect(diff.locator('[data-test=next-file]')).toBeVisible()
})

test('FR-DIFF-05 a Go file is syntax-highlighted', async ({ openDiff, git, repos }) => {
  const sha = commitSha(git, repos.work, 'add logo and func a')
  const diff = await openDiff(repos.work, { kind: 'commit', sha, file: 'src/main.go' })
  await expect(diffPane(diff).locator('.tok-keyword').first()).toBeVisible()
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

test('FR-ED-02 a file opens in the editor from the commit detail view and the change list; a file gone from disk says so', async ({
  app,
  repos,
  editorCalls,
}) => {
  await logRow(app, 'docs and func b').click()
  await commitFile(app, 'readme.md').locator('[data-test=open-editor]').click()
  await expect
    .poll(editorCalls)
    .toEqual([`stand-in-editor ${path.join(repos.work, 'docs/readme.md')}`])

  fs.appendFileSync(path.join(repos.work, 'hunks.txt'), 'line 31\n')
  await switchToChanges(app)
  await fileRow(app, 'Unstaged', 'hunks.txt').locator('[data-test=open-editor]').click()
  await expect.poll(editorCalls).toHaveLength(2)
  expect(editorCalls()[1]).toBe(`stand-in-editor ${path.join(repos.work, 'hunks.txt')}`)

  fs.rmSync(path.join(repos.work, 'docs/readme.md'))
  await app.getByRole('tab', { name: 'Details', exact: true }).click()
  await commitFile(app, 'readme.md').locator('[data-test=open-editor]').click()
  await expect(app.locator('.commit-details .error-banner')).toContainText(
    'docs/readme.md no longer exists in this worktree',
  )
  expect(editorCalls()).toHaveLength(2)
})

test('FR-ED-09 "Open in editor" from the diff opens the file at the clicked line', async ({
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
  await diff.locator('[data-test=next-change]').click()
  await diff.locator('[data-test=next-change]').click()
  await diffLine(diff, 'row 180 CHANGED').click()
  await diff.locator('.diff-toolbar [data-test=open-editor]').click()
  await expect
    .poll(editorCalls)
    .toEqual([`stand-in-editor ${path.join(repos.work, 'long.txt')}:180`])
})

test('FR-ED-05 a saved editor change relabels "Open in …" in every window at once', async ({
  app,
  openApp,
  repos,
}) => {
  const other = await openApp(repos.work)
  const dialog = await openSettings(app)
  await dialog.locator('[data-test=editor-preset]', { hasText: 'VS Code' }).click()
  await dialog.getByRole('button', { name: 'Save' }).click()
  await expect(dialog).toBeHidden()
  for (const page of [app, other]) {
    await expect(page.getByRole('button', { name: 'Open in VS Code' })).toBeVisible()
  }
})

test.describe('with an editor that cannot be started', () => {
  test.use({
    settings: {
      editorProfiles: {
        '': {
          name: 'Missing',
          folderCmd: 'missing-editor {path}',
          fileCmd: 'missing-editor {path}',
          fileAtLineCmd: 'missing-editor {path}:{line}',
        },
      },
    },
  })

  test('FR-ED-06 the error names the command that failed, and nothing more', async ({ app }) => {
    await app.getByRole('button', { name: 'Open in Missing' }).click()
    await expect(notice(app)).toContainText('"missing-editor" is not on PATH')
    await expect(notice(app).getByRole('button', { name: 'show details' })).toHaveCount(0)
    await expect(app.locator('.toolbar .error-banner')).toHaveCount(0)
  })

  test("FR-ED-06 a file's editor error names the command too", async ({ app }) => {
    await logRow(app, 'docs and func b').click()
    await commitFile(app, 'readme.md').locator('[data-test=open-editor]').click()
    await expect(notice(app)).toContainText('"missing-editor" is not on PATH')
    await expect(app.locator('.commit-details .error-banner')).toHaveCount(0)
  })
})
