import fs from 'node:fs'
import path from 'node:path'
import type { Locator, Page } from '@playwright/test'
import { test, expect } from './fixtures'
import { fileRow, logRow } from './helpers'
import type { Git, Repos } from './repos'

// The diff pane, the command log, themes, layout, keyboard shortcuts and
// every way of opening a file in the external editor. Editor launches go
// to the sandbox's stand-in editor and stand-in xdg-open.

const rows = Array.from({ length: 200 }, (_, i) => `row ${i + 1}`)

// commitFarChanges adds long.txt, then commits "two far changes": rows 5
// and 180 of long.txt changed, and src/main.go given a function.
function commitFarChanges(git: Git, repos: Repos): void {
  const long = path.join(repos.work, 'long.txt')
  fs.writeFileSync(long, rows.join('\n') + '\n')
  git(repos.work, 'add', 'long.txt')
  git(repos.work, 'commit', '-q', '-m', 'add long.txt')
  const changed = rows.map((r, i) => (i === 4 || i === 179 ? `${r} CHANGED` : r))
  fs.writeFileSync(long, changed.join('\n') + '\n')
  fs.writeFileSync(path.join(repos.work, 'src/main.go'), 'package main\n\nfunc main() {}\n')
  git(repos.work, 'commit', '-q', '-a', '-m', 'two far changes')
}

const diffPane = (page: Page) => page.locator('.side-column .diff-view')
const diffLine = (page: Page, text: string) =>
  diffPane(page).locator('.cm-line', { hasText: text }).first()
const commitFile = (page: Page, name: string) =>
  page.locator('.commit-details .files li', { hasText: name })

async function showCommitFile(page: Page, subject: string, file: string): Promise<void> {
  await logRow(page, subject).click()
  await commitFile(page, file).click()
}

async function openSettings(page: Page): Promise<Locator> {
  await page.locator('[data-test=open-settings]').click()
  const dialog = page.getByRole('dialog', { name: 'Settings' })
  await expect(dialog).toBeVisible()
  return dialog
}

test('FR-DIFF-02 next/previous change jump between far changes; next/previous file step through the commit', async ({
  openApp,
  git,
  repos,
}) => {
  commitFarChanges(git, repos)
  const app = await openApp(repos.work)
  await showCommitFile(app, 'two far changes', 'long.txt')
  await expect(diffLine(app, 'row 5 CHANGED')).toBeInViewport()

  await app.locator('[data-test=next-change]').click()
  await app.locator('[data-test=next-change]').click()
  await expect(diffLine(app, 'row 180 CHANGED')).toBeInViewport()
  await expect(diffLine(app, 'row 5 CHANGED')).not.toBeInViewport()
  await app.locator('[data-test=prev-change]').click()
  await expect(diffLine(app, 'row 5 CHANGED')).toBeInViewport()

  await app.locator('[data-test=next-file]').click()
  await expect(app.locator('.commit-details .files li.selected')).toContainText('main.go')
  await expect(diffLine(app, 'func main() {}')).toBeVisible()
  await app.locator('[data-test=prev-file]').click()
  await expect(app.locator('.commit-details .files li.selected')).toContainText('long.txt')
})

test('FR-DIFF-03 a binary file shows a placeholder instead of content, and file stepping stays', async ({
  app,
}) => {
  await showCommitFile(app, 'add logo and func a', 'logo.png')
  await expect(diffPane(app)).toContainText('Binary file, no preview available.')
  await expect(diffPane(app).locator('.cm-editor')).toHaveCount(0)
  await expect(app.locator('[data-test=next-file]')).toBeVisible()
})

test('FR-DIFF-05 a Go file is syntax-highlighted', async ({ app }) => {
  await showCommitFile(app, 'add logo and func a', 'src/main.go')
  await expect(diffPane(app).locator('.tok-keyword').first()).toBeVisible()
})

test('FR-DIFF-06 "ignore whitespace" hides a whitespace-only change', async ({ app, repos }) => {
  const hunks = path.join(repos.work, 'hunks.txt')
  fs.writeFileSync(hunks, fs.readFileSync(hunks, 'utf8').replace('line 7\n', 'line 7   \n'))
  await fileRow(app, 'Unstaged', 'hunks.txt').click()
  const changed = diffPane(app).locator('.cm-changedLine, .cm-deletedChunk')
  await expect(changed.first()).toBeVisible()

  await app.locator('[data-test=ignore-whitespace-toggle]').click()
  await expect(changed).toHaveCount(0)
  await app.locator('[data-test=ignore-whitespace-toggle]').click()
  await expect(changed.first()).toBeVisible()
})

test('FR-UX-02 the command log shows user operations; background reads only once switched on', async ({
  app,
  repos,
}) => {
  const entries = app.locator('[data-test=command-log-entry]')
  const background = app.getByLabel('show background commands')
  await expect(entries).toHaveCount(0)

  await background.check()
  await expect(entries.first()).toBeVisible()
  const panel = await app.locator('.command-log').boundingBox()
  expect(panel!.height).toBeLessThanOrEqual(800 / 4)
  await background.uncheck()
  await expect(entries).toHaveCount(0)

  fs.writeFileSync(path.join(repos.work, 'new.txt'), 'new\n')
  await fileRow(app, 'Untracked', 'new.txt').locator('[data-test=action]').click()
  await expect(entries).toHaveText([/add -- new\.txt/])
})

test('FR-UX-05 Ctrl+K commits, Ctrl+Shift+K pushes, Ctrl+T pulls; F5 refreshes in place instead of reloading the page', async ({
  app,
  git,
  repos,
}) => {
  fs.writeFileSync(path.join(repos.work, 'staged.txt'), 'staged\n')
  git(repos.work, 'add', 'staged.txt')
  await expect(fileRow(app, 'Staged', 'staged.txt')).toBeVisible()
  await app.getByPlaceholder('Commit message').fill('commit via Ctrl+K')
  await app.keyboard.press('Control+k')
  await expect
    .poll(() => git(repos.work, 'log', '-1', '--format=%s').trim())
    .toBe('commit via Ctrl+K')

  await app.keyboard.press('Control+Shift+K')
  await expect
    .poll(() => git(repos.remote, 'rev-parse', 'main').trim())
    .toBe(git(repos.work, 'rev-parse', 'HEAD').trim())

  git(repos.other, 'checkout', '-q', 'main')
  git(repos.other, 'pull', '-q')
  fs.writeFileSync(path.join(repos.other, 'pulled.txt'), 'pulled\n')
  git(repos.other, 'add', 'pulled.txt')
  git(repos.other, 'commit', '-q', '-m', 'from the other clone')
  git(repos.other, 'push', '-q')
  await app.keyboard.press('Control+t')
  await expect.poll(() => fs.existsSync(path.join(repos.work, 'pulled.txt'))).toBe(true)

  // A listener added after the app's own runs after it, so it sees whether
  // the app kept F5 from reloading the page.
  type Marked = { f5Prevented?: boolean }
  await app.evaluate(() =>
    window.addEventListener(
      'keydown',
      (e) => ((window as Marked).f5Prevented = e.defaultPrevented),
      { once: true },
    ),
  )
  await app.keyboard.press('F5')
  expect(await app.evaluate(() => (window as Marked).f5Prevented)).toBe(true)
})

test('FR-UX-06 Light and Dark apply at once; System follows the OS setting', async ({ app }) => {
  const dark = () => app.evaluate(() => document.documentElement.classList.contains('nebegit-dark'))
  const setTheme = async (theme: string) => {
    const dialog = await openSettings(app)
    await dialog.locator('[data-test=theme-select]').selectOption({ label: theme })
    await dialog.getByRole('button', { name: 'Save' }).click()
    await expect(dialog).toBeHidden()
  }

  await setTheme('Dark')
  await expect.poll(dark).toBe(true)
  await setTheme('Light')
  await expect.poll(dark).toBe(false)

  await setTheme('System')
  await app.emulateMedia({ colorScheme: 'dark' })
  await expect.poll(dark).toBe(true)
  await app.emulateMedia({ colorScheme: 'light' })
  await expect.poll(dark).toBe(false)
})

test('FR-UX-07 collapsing the selector and changes panels widens the log; dragging a column border resizes it', async ({
  app,
}) => {
  const logWidth = async () => (await app.locator('.log-column').boundingBox())!.width
  const before = await logWidth()
  await app.locator('[data-test=panel-collapse]').click()
  await app.locator('[data-test=changes-collapse]').click()
  await expect.poll(logWidth).toBeGreaterThan(before + 100)

  const collapsed = await logWidth()
  const gutter = (await app.locator('.worktree-body .p-splitter-gutter').last().boundingBox())!
  const y = gutter.y + 200
  await app.mouse.move(gutter.x + gutter.width / 2, y)
  await app.mouse.down()
  await app.mouse.move(gutter.x - 150, y, { steps: 8 })
  await app.mouse.up()
  await expect.poll(logWidth).toBeLessThan(collapsed - 100)
})

test('FR-UX-08 the columns sit side by side without overlapping', async ({ app }) => {
  const columns = ['.selector-panel', '.changes-column', '.log-column', '.side-column']
  const boxes = await Promise.all(columns.map(async (c) => (await app.locator(c).boundingBox())!))
  for (let i = 1; i < boxes.length; i++) {
    expect(boxes[i]!.x).toBeGreaterThanOrEqual(boxes[i - 1]!.x + boxes[i - 1]!.width - 1)
  }
})

test('FR-ED-02 a file opens in the editor from the commit details and the changes list; a file gone from disk says so', async ({
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
  await fileRow(app, 'Unstaged', 'hunks.txt').locator('[data-test=open-editor]').click()
  await expect.poll(editorCalls).toHaveLength(2)
  expect(editorCalls()[1]).toBe(`stand-in-editor ${path.join(repos.work, 'hunks.txt')}`)

  fs.rmSync(path.join(repos.work, 'docs/readme.md'))
  await commitFile(app, 'readme.md').locator('[data-test=open-editor]').click()
  await expect(app.locator('.commit-details .error-banner')).toContainText(
    'docs/readme.md no longer exists in this worktree',
  )
  expect(editorCalls()).toHaveLength(2)
})

test('FR-ED-09 "Open in editor" from the diff opens the file at the clicked line', async ({
  openApp,
  git,
  repos,
  editorCalls,
}) => {
  commitFarChanges(git, repos)
  const app = await openApp(repos.work)
  await showCommitFile(app, 'two far changes', 'long.txt')
  await app.locator('[data-test=next-change]').click()
  await app.locator('[data-test=next-change]').click()
  await diffLine(app, 'row 180 CHANGED').click()
  await app.locator('.diff-toolbar [data-test=open-editor]').click()
  await expect
    .poll(editorCalls)
    .toEqual([`stand-in-editor ${path.join(repos.work, 'long.txt')}:180`])
})

test('FR-ED-07 a preset fills the editor command fields', async ({ app }) => {
  const dialog = await openSettings(app)
  await dialog.locator('[data-test=editor-preset]', { hasText: 'VS Code' }).click()
  await expect(dialog.getByLabel('Folder command')).toHaveValue('code {path}')
  await expect(dialog.getByLabel('File command', { exact: true })).toHaveValue('code {path}')
  await expect(dialog.getByLabel('File-at-line command')).toHaveValue('code -g {path}:{line}')
})

test('FR-ED-08 "Test" in the settings opens the home folder with the configured editor', async ({
  app,
  sandbox,
  editorCalls,
}) => {
  const dialog = await openSettings(app)
  await dialog.locator('[data-test=test-editor-profile]').click()
  await expect.poll(editorCalls).toEqual([`stand-in-editor ${path.join(sandbox.dir, 'home')}`])
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

  test('FR-ED-06 the error says so and links to the editor setting', async ({ app }) => {
    await app.getByRole('button', { name: 'Open in Missing' }).click()
    const banner = app.locator('.toolbar .error-banner')
    await expect(banner).toContainText('missing-editor')
    await banner.getByRole('button', { name: 'Open editor setting' }).click()
    await expect(app.getByRole('dialog', { name: 'Settings' })).toBeVisible()
  })

  test("FR-ED-06 a file's editor error links to the editor setting too", async ({ app }) => {
    await logRow(app, 'docs and func b').click()
    await commitFile(app, 'readme.md').locator('[data-test=open-editor]').click()
    const banner = app.locator('.commit-details .error-banner')
    await banner.getByRole('button', { name: 'Open editor setting' }).click()
    await expect(app.getByRole('dialog', { name: 'Settings' })).toBeVisible()
  })

  test("FR-ED-10 the error offers git's editor and the system default instead", async ({
    app,
    repos,
    editorCalls,
  }) => {
    const readme = path.join(repos.work, 'docs/readme.md')
    const openReadme = () => commitFile(app, 'readme.md').locator('[data-test=open-editor]').click()
    const banner = app.locator('.commit-details .error-banner')
    await logRow(app, 'docs and func b').click()

    await openReadme()
    await banner.getByRole('button', { name: "Open with git's editor" }).click()
    await expect.poll(editorCalls).toEqual([`stand-in-editor ${readme}`])

    await openReadme()
    await banner.getByRole('button', { name: 'Open with system default' }).click()
    await expect.poll(editorCalls).toEqual([`stand-in-editor ${readme}`, `xdg-open ${readme}`])
  })
})
