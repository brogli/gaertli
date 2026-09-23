import fs from 'node:fs'
import path from 'node:path'
import { test, expect } from './fixtures'
import { refNode } from './helpers'

// Layout at the 1200 x 800 window the fixtures open, with real refs and a
// real repo path in the selector.

test('FR-UX-09 no tree scrolls sideways; every tree row fits its panel', async ({ app }) => {
  // A sideways-scrolling tree gets an overlay scrollbar under WebKitGTK that
  // covers its last row and swallows clicks on that row's toggle.
  const roots = await app.locator('.p-tree-root').all()
  expect(roots.length).toBeGreaterThan(0)
  for (const root of roots) {
    const { overflowX, fits } = await root.evaluate((el) => ({
      overflowX: getComputedStyle(el).overflowX,
      fits: el.scrollWidth <= el.clientWidth,
    }))
    expect(overflowX).toBe('hidden')
    expect(fits).toBe(true)
  }
})

test('FR-UX-09 a long branch name ends in an ellipsis beside its checkbox and shows whole on hover', async ({
  app,
  git,
  repos,
}) => {
  const long = 'a-rather-long-branch-name-that-does-not-fit-the-sidebar'
  git(repos.work, 'branch', long, 'main')
  git(repos.work, 'push', '-q', '-u', 'origin', long)

  const node = refNode(app, long)
  const checkbox = node.locator('[data-test=update-select]')
  const name = node.locator('.ref-name')
  await expect(checkbox).toBeVisible()
  await expect(name).toHaveAttribute('title', long)
  const [c, n] = [await checkbox.boundingBox(), await name.boundingBox()]
  expect(Math.abs(c!.y + c!.height / 2 - (n!.y + n!.height / 2))).toBeLessThan(4)
  expect(await name.evaluate((el) => el.scrollWidth > el.clientWidth)).toBe(true)
})

test('FR-UX-09 a selector worktree row keeps its name, flags and buttons on one line; a long branch name ends in an ellipsis', async ({
  app,
  git,
  repos,
  sandbox,
}) => {
  const linked = (name: string) => {
    const dir = path.join(sandbox.dir, name)
    git(repos.work, 'worktree', 'add', '-q', '-b', name, dir, 'main')
    return dir
  }
  fs.writeFileSync(path.join(linked('wt-dirty'), 'new.txt'), 'x\n')
  git(repos.work, 'worktree', 'lock', linked('wt-locked'))
  fs.rmSync(linked('wt-missing'), { recursive: true })
  const long = 'a-rather-long-branch-name-that-does-not-fit-the-selector'
  git(repos.work, 'branch', long, 'main')

  await app.locator('.selector-panel .p-tree-node-toggle-button').first().click()
  const rows = app.locator('[data-test=worktree-row]')
  await expect(rows).toHaveCount(5)
  for (const row of await rows.all()) {
    const boxes = await Promise.all(
      (await row.locator(':scope > *').all()).map(async (child) => (await child.boundingBox())!),
    )
    const middle = boxes[0]!.y + boxes[0]!.height / 2
    for (const [i, box] of boxes.entries()) {
      expect(Math.abs(box.y + box.height / 2 - middle)).toBeLessThanOrEqual(4)
      if (i > 0) expect(box.x).toBeGreaterThanOrEqual(boxes[i - 1]!.x + boxes[i - 1]!.width + 2)
    }
  }

  const name = app.locator('[data-test=branch-row]', { hasText: long }).locator('.ellipsis')
  await expect(name).toHaveAttribute('title', long)
  expect(await name.evaluate((el) => el.scrollWidth > el.clientWidth)).toBe(true)
})

test('FR-WT-01 a worktree row gives up its folder name before its branch name', async ({
  app,
  git,
  repos,
  sandbox,
}) => {
  const dir = path.join(sandbox.dir, 'a-worktree-folder-with-a-rather-long-name')
  git(repos.work, 'worktree', 'add', '-q', '-b', 'wip', dir, 'main')

  await app.locator('.selector-panel .p-tree-node-toggle-button').first().click()
  const row = app.locator('[data-test=worktree-row]', { hasText: 'wip' })
  const branch = row.locator('[data-test=worktree-branch]')
  await expect(branch).toHaveText('wip')
  expect(await branch.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true)
  const folder = row.locator('[data-test=worktree-folder]')
  expect(await folder.evaluate((el) => el.scrollWidth > el.clientWidth)).toBe(true)
})

test('FR-UX-09 the changes heading shows whole; Stage all and Unstage all share one row', async ({
  app,
}) => {
  const heading = app.locator('.changes-body header h2')
  expect(await heading.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true)
  const stageAll = await app.getByRole('button', { name: 'Stage all', exact: true }).boundingBox()
  const unstageAll = await app.getByRole('button', { name: 'Unstage all', exact: true }).boundingBox()
  expect(Math.abs(stageAll!.y - unstageAll!.y)).toBeLessThan(4)
})

test('FR-LOG-01 a crowded log row shows two whole ref labels and "+N" naming them all', async ({
  app,
}) => {
  const row = app.locator('.row', { has: app.locator('.ref-label.head') })
  const labels = row.locator('.ref-label')
  await expect(labels).toHaveCount(2)
  for (const label of await labels.all()) {
    expect(await label.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true)
  }
  const more = row.locator('.ref-more')
  await expect(more).toHaveText(/^\+\d+$/)
  await expect(more).toHaveAttribute('title', /origin\/main/)
  expect((await row.locator('.subject').boundingBox())!.width).toBeGreaterThanOrEqual(80)
})

test('FR-UX-09 in a narrow log column the ref labels shrink to an ellipsis; "+N" and the hash stay inside the row', async ({
  app,
}) => {
  await app.setViewportSize({ width: 1100, height: 800 })
  const list = app.locator('.log-list')
  const row = app.locator('.row', { has: app.locator('.ref-label.head') })
  const rowRight = async () => {
    const box = (await row.boundingBox())!
    return box.x + box.width
  }
  for (const part of ['.ref-more', '.hash']) {
    const box = (await row.locator(part).boundingBox())!
    expect(box.x + box.width, part).toBeLessThanOrEqual(await rowRight())
  }
  expect(await list.evaluate((el) => el.scrollWidth <= el.clientWidth)).toBe(true)
  const head = row.locator('.ref-label.head')
  expect(await head.evaluate((el) => el.scrollWidth > el.clientWidth)).toBe(true)
  await expect(head).toHaveAttribute('title', 'HEAD')
})
