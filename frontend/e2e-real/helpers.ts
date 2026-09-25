import fs from 'node:fs'
import path from 'node:path'
import { expect, type Locator, type Page } from '@playwright/test'
import type { Git } from './repos'

// Locators and actions the area specs share; an area-only helper stays in
// its spec file.

// logRow is the log row whose subject contains subject.
export function logRow(page: Page, subject: string): Locator {
  return page.locator('.row', { has: page.locator('.subject', { hasText: subject }) }).first()
}

// openBranchPicker opens the toolbar's branch picker.
export async function openBranchPicker(page: Page): Promise<void> {
  await page.locator('[data-test=branch-picker]').click()
  await page.locator('.p-popover .branch-list').waitFor()
}

// refRow is the open branch picker's row of the ref named exactly name, e.g.
// "origin/main" for a remote branch.
export function refRow(page: Page, name: string): Locator {
  return page.locator('.p-popover [data-test=ref-row]', {
    has: page.locator(`.ref-name:text-is("${name}")`),
  })
}

// refLi is that row's <li>: the row and its sibling checkbox both live
// inside it.
export function refLi(page: Page, name: string): Locator {
  return page.locator('.p-popover .picker-rows li', {
    has: page.locator(`.ref-name:text-is("${name}")`),
  })
}

// refActions opens the branch picker and right-clicks name's row, and lists
// its context menu's items.
export async function refActions(page: Page, name: string): Promise<string[]> {
  await openBranchPicker(page)
  return contextMenuItems(page, refRow(page, name))
}

// pickRefAction picks item from name's context menu in the branch picker.
export async function pickRefAction(page: Page, name: string, item: string): Promise<void> {
  await openBranchPicker(page)
  await pickFromContextMenu(page, refRow(page, name), item)
}

// openRepoPicker opens the toolbar's repo picker.
export async function openRepoPicker(page: Page): Promise<void> {
  await page.locator('[data-test=repo-picker]').click()
  await page.locator('.p-popover .repo-list').waitFor()
}

// openWorktreePicker opens the toolbar's worktree picker.
export async function openWorktreePicker(page: Page): Promise<void> {
  await page.locator('[data-test=worktree-picker]').click()
  await page.locator('.p-popover .worktree-list').waitFor()
}

// pickerAction is the open picker's (or the start page's) action row
// labelled label, e.g. "Open folder…".
export function pickerAction(page: Page, label: string): Locator {
  return page.locator('[data-test=picker-action]', { hasText: label })
}

// pickerError is the open picker's error banner.
export function pickerError(page: Page): Locator {
  return page.locator('.p-popover .error-banner')
}

export async function contextMenuItems(page: Page, target: Locator): Promise<string[]> {
  await target.click({ button: 'right' })
  const items = page.locator('.p-contextmenu [role=menuitem]')
  await items.first().waitFor()
  return items.allTextContents()
}

export async function pickFromContextMenu(
  page: Page,
  target: Locator,
  item: string,
): Promise<void> {
  await target.click({ button: 'right' })
  await page.locator('.p-contextmenu [role=menuitem]', { hasText: item }).first().click()
}

// switchToChanges picks "Changes" in the medium layout's side-panel switch
// (the fixture's default 1200 x 800), so the changes view shows beside the
// log instead of the commit detail view it defaults to.
export async function switchToChanges(page: Page): Promise<void> {
  await page.getByRole('button', { name: /^Changes/ }).click()
  await page.locator('.changes-body').waitFor()
}

// fileGroup is the file list under the heading title (Staged, Unstaged,
// Untracked, Conflicted): the changes view's, or with within '.file-list'
// the working tree's diff window's.
export function fileGroup(page: Page, title: string, within = '.changes-body'): Locator {
  return page
    .locator(`${within} h3`, { hasText: new RegExp(`^${title}$`, 'i') })
    .locator('xpath=following-sibling::*[1]')
}

export function fileRow(page: Page, group: string, name: string, within?: string): Locator {
  return fileGroup(page, group, within)
    .locator('li', { has: page.locator('.path', { hasText: name }) })
    .first()
}

// diffRow is the working tree's diff window's row of name under group.
export function diffRow(page: Page, group: string, name: string): Locator {
  return fileRow(page, group, name, '.file-list')
}

// diffText is the diff window's diff text.
export function diffText(page: Page): Locator {
  return page.locator('.diff-view .cm-content')
}

export function confirmDialog(page: Page): Locator {
  return page.locator('.p-confirmdialog')
}

export async function clickConfirm(page: Page): Promise<void> {
  await confirmDialog(page).getByRole('button', { name: 'Confirm' }).click()
}

export function conflictBanner(page: Page): Locator {
  return page.locator('[data-test=conflict-banner]')
}

// notice is the app's current notice (NoticeArea.vue); last() rides out the
// brief overlap while an older one leaves and a newer one enters.
export function notice(page: Page): Locator {
  return page.locator('.p-toast-message').last()
}

// jobsMenuButton is the toolbar's jobs menu button.
export function jobsMenuButton(page: Page): Locator {
  return page.locator('[data-test=jobs-menu]')
}

// openJobsMenu opens the jobs menu's list.
export async function openJobsMenu(page: Page): Promise<Locator> {
  await jobsMenuButton(page).click()
  const list = page.locator('.jobs-list')
  await list.waitFor()
  return list
}

export async function openSettings(page: Page): Promise<Locator> {
  await page.locator('[data-test=open-settings]').click()
  const dialog = page.getByRole('dialog', { name: 'Settings' })
  await expect(dialog).toBeVisible()
  return dialog
}

// dragLogBorder drags the border right of the log view to x.
export async function dragLogBorder(page: Page, x: number): Promise<void> {
  const gutter = (await page.locator('.worktree-body > .p-splitter-gutter').boundingBox())!
  const y = gutter.y + 200
  await page.mouse.move(gutter.x + gutter.width / 2, y)
  await page.mouse.down()
  await page.mouse.move(x, y, { steps: 8 })
  await page.mouse.up()
}

// choosePull opens the sync controls' Pull split button's menu and picks
// mode, running a pull with it once.
export async function choosePull(page: Page, mode: 'merge' | 'rebase'): Promise<void> {
  await page.locator('.sync-controls .p-splitbutton-dropdown').click()
  await page.locator('.p-tieredmenu-item-content', { hasText: `Pull with ${mode}` }).click()
}

// commitFiles writes files (path to content) in repo and commits them.
export function commitFiles(
  git: Git,
  repo: string,
  subject: string,
  files: Record<string, string>,
): void {
  for (const [file, text] of Object.entries(files)) fs.writeFileSync(path.join(repo, file), text)
  git(repo, 'add', '--', ...Object.keys(files))
  git(repo, 'commit', '-q', '-m', subject)
}

// commitSha is the full hash of the commit whose message contains subject.
export function commitSha(git: Git, repo: string, subject: string): string {
  return git(repo, 'log', '--all', '-1', '--format=%H', '-F', `--grep=${subject}`).trim()
}

// pushFromClone commits files (path to content) on branch in clone and
// pushes it: a change that reaches the remote from someone else.
export function pushFromClone(
  git: Git,
  clone: string,
  branch: string,
  files: Record<string, string>,
  subject: string,
): void {
  git(clone, 'checkout', '-q', branch)
  git(clone, 'pull', '-q', '--ff-only')
  commitFiles(git, clone, subject, files)
  git(clone, 'push', '-q', 'origin', branch)
}

// rev is the commit ref resolves to in repo.
export function rev(git: Git, repo: string, ref: string): string {
  return git(repo, 'rev-parse', ref).trim()
}

export function shortRev(git: Git, repo: string, ref: string): string {
  return git(repo, 'rev-parse', '--short', ref).trim()
}

// currentBranch is repo's checked-out branch, '' when HEAD is detached.
export function currentBranch(git: Git, repo: string): string {
  return git(repo, 'branch', '--show-current').trim()
}

// parents counts the parents of repo's HEAD commit.
export function parents(git: Git, repo: string): number {
  return git(repo, 'log', '-1', '--format=%P').trim().split(' ').length
}

// hunksText is hunks.txt (line 1 … line 30) with the given lines replaced.
export function hunksText(replaced: Record<number, string> = {}): string {
  return Array.from({ length: 30 }, (_, i) => `${replaced[i + 1] ?? `line ${i + 1}`}\n`).join('')
}

// fileText is file's content, or null while it is missing. Polls read files
// through it: git rewrites a file by removing and recreating it, and
// expect.poll fails at once when its callback throws.
export function fileText(file: string): string | null {
  try {
    return fs.readFileSync(file, 'utf8')
  } catch (e) {
    if ((e as NodeJS.ErrnoException).code === 'ENOENT') return null
    throw e
  }
}
