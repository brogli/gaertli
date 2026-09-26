import fs from 'node:fs'
import path from 'node:path'
import type { Locator, Page } from '@playwright/test'
import { test, expect } from './fixtures'
import { dragLogBorder, openBranchPicker, openWorktreePicker, refLi } from './helpers'

// Layout at the 1200 x 800 window the fixtures open, with real refs and a
// real repo path in the pickers.

// overflows: locator's content is wider than its box.
function overflows(locator: Locator): Promise<boolean> {
  return locator.evaluate((el) => el.scrollWidth > el.clientWidth)
}

// expectRowsFit: no row of the open picker list is wider than the list,
// and the list does not scroll sideways.
async function expectRowsFit(page: Page, list: string): Promise<void> {
  // A sideways-scrolling list gets an overlay scrollbar under WebKitGTK that
  // covers its last row and swallows clicks on that row.
  const scroller = page.locator(`.p-popover ${list} .picker-scroll`)
  expect(await scroller.evaluate((el) => getComputedStyle(el).overflowX)).toBe('hidden')
  expect(await overflows(scroller)).toBe(false)
  for (const row of await scroller.locator('li').all()) {
    expect(await overflows(row)).toBe(false)
  }
}

test('FR-UX-09 no picker list scrolls sideways; every row fits the list', async ({ app }) => {
  await openWorktreePicker(app)
  await expectRowsFit(app, '.worktree-list')
  await app.keyboard.press('Escape')

  await openBranchPicker(app)
  await expectRowsFit(app, '.branch-list')
})

test('FR-UX-09 a long branch name ends in an ellipsis beside its checkbox and shows whole on hover', async ({
  app,
  git,
  repos,
}) => {
  const long = 'a-rather-long-branch-name-that-does-not-fit-the-branch-picker'
  git(repos.work, 'branch', long, 'main')
  git(repos.work, 'push', '-q', '-u', 'origin', long)

  await openBranchPicker(app)
  const li = refLi(app, long)
  const checkbox = li.locator('[data-test=update-select]')
  const name = li.locator('.ref-name')
  await expect(checkbox).toBeVisible()
  await expect(name).toHaveAttribute('title', long)
  const [c, n] = [await checkbox.boundingBox(), await name.boundingBox()]
  expect(Math.abs(c!.y + c!.height / 2 - (n!.y + n!.height / 2))).toBeLessThan(4)
  expect(await overflows(name)).toBe(true)
})

test('FR-BR-09 every ref name in the branch picker starts at the same x, with an update checkbox or without', async ({
  app,
}) => {
  await openBranchPicker(app)
  await expect(refLi(app, 'B').locator('[data-test=update-select]')).toBeVisible()
  await expect(refLi(app, 'side').locator('[data-test=update-select]')).toHaveCount(0)

  const names = ['main', 'B', 'side', 'origin/B', 'v1']
  const lefts = await Promise.all(
    names.map(async (name) => (await refLi(app, name).locator('.ref-name').boundingBox())!.x),
  )
  expect(new Set(lefts.map(Math.round)).size, lefts.join(', ')).toBe(1)
})

test('FR-UX-09 a worktree row keeps its name, flags and buttons on one line; a long branch name ends in an ellipsis', async ({
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
  const long = 'a-rather-long-branch-name-that-does-not-fit-the-worktree-picker'
  linked(long)

  await openWorktreePicker(app)
  // The row's own li: its label span and its sibling buttons, all on one
  // line.
  const rows = app.locator('.p-popover .picker-rows li', {
    has: app.locator('[data-test=worktree-row]'),
  })
  await expect(rows).toHaveCount(6)
  for (const row of await rows.all()) {
    const boxes = await Promise.all(
      (await row.locator(':scope > *').all()).map(async (child) => (await child.boundingBox())!),
    )
    const middle = boxes[0]!.y + boxes[0]!.height / 2
    for (const box of boxes) {
      expect(Math.abs(box.y + box.height / 2 - middle)).toBeLessThanOrEqual(4)
    }
    for (let i = 1; i < boxes.length; i++) {
      expect(boxes[i]!.x).toBeGreaterThanOrEqual(boxes[i - 1]!.x + boxes[i - 1]!.width + 2)
    }
  }

  const name = rows.locator(`[data-test=worktree-branch][title="${long}"]`)
  expect(await overflows(name)).toBe(true)
})

test('FR-WT-13 a worktree row gives up its branch name before its folder name', async ({
  app,
  git,
  repos,
  sandbox,
}) => {
  const branchName = 'a-branch-with-a-rather-long-name-that-cannot-fit-in-a-picker-row'
  git(repos.work, 'worktree', 'add', '-q', '-b', branchName, path.join(sandbox.dir, 'wip'), 'main')

  await openWorktreePicker(app)
  const row = app.locator('.p-popover [data-test=worktree-row]', { hasText: 'wip' })
  const folder = row.locator('[data-test=worktree-folder]')
  await expect(folder).toHaveText('wip')
  expect(await overflows(folder)).toBe(false)
  const branch = row.locator('[data-test=worktree-branch]')
  expect(await overflows(branch)).toBe(true)
})

test('FR-UX-09 the changes heading shows whole; Stage all and Unstage all share one row', async ({
  app,
}) => {
  const heading = app.locator('.changes-body header h2')
  expect(await overflows(heading)).toBe(false)
  const stageAll = await app.getByRole('button', { name: 'Stage all', exact: true }).boundingBox()
  const unstageAll = await app
    .getByRole('button', { name: 'Unstage all', exact: true })
    .boundingBox()
  expect(Math.abs(stageAll!.y - unstageAll!.y)).toBeLessThan(4)
})

test('FR-LOG-01 a crowded log row shows two whole ref labels and "+N" naming them all', async ({
  app,
}) => {
  const row = app.locator('.row', { has: app.locator('.ref-label.head') })
  const labels = row.locator('.ref-label')
  await expect(labels).toHaveCount(2)
  for (const label of await labels.all()) {
    expect(await overflows(label)).toBe(false)
  }
  const more = row.locator('.ref-more')
  await expect(more).toHaveText(/^\+\d+$/)
  await expect(more).toHaveAttribute('title', /origin\/main/)
  expect((await row.locator('.subject').boundingBox())!.width).toBeGreaterThanOrEqual(80)
})

test('FR-UX-09 in a narrow log view the ref labels shrink to an ellipsis; "+N" and the hash stay inside the row', async ({
  app,
}) => {
  await dragLogBorder(app, 300)
  await expect
    .poll(async () => (await app.locator('.log-column').boundingBox())!.width)
    .toBeLessThan(380)
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
  expect(await overflows(list)).toBe(false)
  const head = row.locator('.ref-label.head')
  expect(await overflows(head)).toBe(true)
  await expect(head).toHaveAttribute('title', 'HEAD')
})
