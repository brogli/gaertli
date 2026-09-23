import fs from 'node:fs'
import path from 'node:path'
import type { Locator, Page } from '@playwright/test'
import type { Git } from './repos'

// Locators and actions the area specs share. Selectors follow the app's
// markup; an area-only helper stays in its spec file.

// logRow is the log row whose subject contains subject.
export function logRow(page: Page, subject: string): Locator {
  return page.locator('.row', { has: page.locator('.subject', { hasText: subject }) }).first()
}

// refNode is the refs sidebar's node named exactly name.
export function refNode(page: Page, name: string): Locator {
  return page.locator('.ref-node', { has: page.locator(`span:text-is("${name}")`) }).first()
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

// fileGroup is the changes panel's file list under the heading title
// (Staged, Unstaged, Untracked, Conflicted).
export function fileGroup(page: Page, title: string): Locator {
  return page
    .locator('.changes-body h3', { hasText: new RegExp(`^${title}$`, 'i') })
    .locator('xpath=following-sibling::*[1]')
}

export function fileRow(page: Page, group: string, name: string): Locator {
  return fileGroup(page, group)
    .locator('li', { has: page.locator('.path', { hasText: name }) })
    .first()
}

export function confirmDialog(page: Page): Locator {
  return page.locator('.p-confirmdialog')
}

export function conflictBanner(page: Page): Locator {
  return page.locator('[data-test=conflict-banner]')
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
