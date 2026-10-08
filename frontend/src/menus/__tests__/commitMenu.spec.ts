import { describe, expect, it, vi } from 'vitest'
import {
  buildOneCommitMenuItems,
  buildSelectionMenuItems,
  buildSquashMenuItems,
  type CommitMenuItem,
} from '../commitMenu'
import { commitRow as row } from '@/testing/fixtures/log-rows'
import { i18n } from '@/i18n'

const t = i18n.global.t

// labelsOf keeps only the labelled items, in order, skipping separators.
function labelsOf(items: CommitMenuItem[]): string[] {
  return items.flatMap((i) => ('label' in i ? [i.label] : []))
}

function oneCommitActions() {
  return {
    openCreateBranch: vi.fn(),
    openCheckoutCommit: vi.fn(),
    openReset: vi.fn(),
    cherryPick: vi.fn(),
    cherryPickOptions: vi.fn(),
    revert: vi.fn(),
    revertOptions: vi.fn(),
    openCreateWorktree: vi.fn(),
    compareWithWorking: vi.fn(),
  }
}

describe('buildOneCommitMenuItems', () => {
  it('FR-LOG-11 the one-commit items in order, no "Compare with …" without a previous selection', () => {
    const items = buildOneCommitMenuItems(row(), oneCommitActions(), t)

    expect(labelsOf(items)).toEqual([
      'Copy hash',
      'Copy subject',
      'New branch from here…',
      'Check out commit',
      'Reset current branch to here…',
      'Cherry-pick',
      'Cherry-pick…',
      'Revert',
      'Revert…',
      'Create worktree here…',
      'Compare with working tree',
    ])
  })

  it.each([
    ['FR-WT-02, FR-LOG-11', 'Create worktree here…', 'openCreateWorktree', ['abc123']],
    ['FR-BR-01', 'New branch from here…', 'openCreateBranch', ['abc123']],
    ['FR-BR-08', 'Check out commit', 'openCheckoutCommit', ['abc123']],
    ['FR-BR-02', 'Reset current branch to here…', 'openReset', ['abc123', 'Fix the bug']],
  ] as const)('%s "%s" calls %s with the row\'s sha', (_id, label, action, args) => {
    const a = oneCommitActions()
    const items = buildOneCommitMenuItems(row(), a, t)

    findItem(items, label).command()

    expect(a[action]).toHaveBeenCalledTimes(1)
    expect(a[action]).toHaveBeenCalledWith(...args)
  })

  it('FR-LOG-11 "Cherry-pick" and "Revert" each call their own spy once', () => {
    const a = oneCommitActions()
    const items = buildOneCommitMenuItems(row(), a, t)

    findItem(items, 'Cherry-pick').command()
    findItem(items, 'Revert').command()

    expect(a.cherryPick).toHaveBeenCalledTimes(1)
    expect(a.revert).toHaveBeenCalledTimes(1)
  })

  it('FR-CP-06, FR-RV-04 "Cherry-pick…" and "Revert…" each call their own options spy once', () => {
    const a = oneCommitActions()
    const items = buildOneCommitMenuItems(row(), a, t)

    findItem(items, 'Cherry-pick…').command()
    findItem(items, 'Revert…').command()

    expect(a.cherryPickOptions).toHaveBeenCalledTimes(1)
    expect(a.revertOptions).toHaveBeenCalledTimes(1)
  })

  it('FR-LOG-11 the two copy items write to the clipboard', () => {
    const writeText = vi.fn()
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    const items = buildOneCommitMenuItems(row(), oneCommitActions(), t)

    findItem(items, 'Copy hash').command()
    findItem(items, 'Copy subject').command()

    expect(writeText).toHaveBeenNthCalledWith(1, 'abc123')
    expect(writeText).toHaveBeenNthCalledWith(2, 'Fix the bug')
  })

  it('FR-CMP-04 "Compare with working tree" is always present and calls compareWithWorking', () => {
    const a = oneCommitActions()
    const items = buildOneCommitMenuItems(row(), a, t)

    findItem(items, 'Compare with working tree').command()

    expect(a.compareWithWorking).toHaveBeenCalledTimes(1)
  })

  it('FR-CMP-01, FR-LOG-11 with compareWithPrevious, appends "Compare with <short hash>" last and calls it', () => {
    const run = vi.fn()
    const items = buildOneCommitMenuItems(
      row(),
      { ...oneCommitActions(), compareWithPrevious: { shortSha: 'a1b2c3d', run } },
      t,
    )

    expect(labelsOf(items).at(-1)).toBe('Compare with a1b2c3d')
    findItem(items, 'Compare with a1b2c3d').command()

    expect(run).toHaveBeenCalledTimes(1)
  })
})

function selectionActions() {
  return {
    cherryPick: vi.fn(),
    cherryPickOptions: vi.fn(),
    revert: vi.fn(),
    revertOptions: vi.fn(),
    copyHashes: vi.fn(),
    swap: vi.fn(),
    stopComparing: vi.fn(),
  }
}

describe('buildSelectionMenuItems', () => {
  it('FR-LOG-11 three commits: pick, revert, squash, fixup, copy hashes, no swap', () => {
    const rows = [row({ sha: 'a' }), row({ sha: 'b' }), row({ sha: 'c' })]
    const squash = buildSquashMenuItems(3, null, { squash: vi.fn(), fixup: vi.fn() }, t)

    const items = buildSelectionMenuItems(rows, selectionActions(), squash, t)

    expect(labelsOf(items)).toEqual([
      'Cherry-pick 3 commits',
      'Cherry-pick 3 commits…',
      'Revert 3 commits',
      'Revert 3 commits…',
      'Squash 3 commits…',
      'Fixup 3 commits…',
      'Copy 3 hashes',
    ])
  })

  it('FR-LOG-11 two commits: the same shape, plus swap and stop comparing last', () => {
    const rows = [row({ sha: 'a' }), row({ sha: 'b' })]

    const items = buildSelectionMenuItems(rows, selectionActions(), [], t)

    expect(labelsOf(items)).toEqual([
      'Cherry-pick 2 commits',
      'Cherry-pick 2 commits…',
      'Revert 2 commits',
      'Revert 2 commits…',
      'Copy 2 hashes',
      'Swap',
      'Stop comparing',
    ])
  })

  it('FR-UX-05 "Stop comparing" carries its shortcut; other items carry none', () => {
    const rows = [row({ sha: 'a' }), row({ sha: 'b' })]

    const items = buildSelectionMenuItems(rows, selectionActions(), [], t)

    expect(findItem(items, 'Stop comparing')).toMatchObject({ shortcut: 'Esc' })
    expect(findItem(items, 'Swap')).not.toHaveProperty('shortcut')
    expect(findItem(items, 'Cherry-pick 2 commits')).not.toHaveProperty('shortcut')
  })

  it('FR-LOG-11 each item calls its own action', () => {
    const rows = [row({ sha: 'a' }), row({ sha: 'b' })]
    const actions = selectionActions()
    const items = buildSelectionMenuItems(rows, actions, [], t)

    findItem(items, 'Cherry-pick 2 commits').command()
    findItem(items, 'Cherry-pick 2 commits…').command()
    findItem(items, 'Revert 2 commits').command()
    findItem(items, 'Revert 2 commits…').command()
    findItem(items, 'Copy 2 hashes').command()
    findItem(items, 'Swap').command()
    findItem(items, 'Stop comparing').command()

    expect(actions.cherryPick).toHaveBeenCalledTimes(1)
    expect(actions.cherryPickOptions).toHaveBeenCalledTimes(1)
    expect(actions.revert).toHaveBeenCalledTimes(1)
    expect(actions.revertOptions).toHaveBeenCalledTimes(1)
    expect(actions.copyHashes).toHaveBeenCalledTimes(1)
    expect(actions.swap).toHaveBeenCalledTimes(1)
    expect(actions.stopComparing).toHaveBeenCalledTimes(1)
  })
})

function findItem(items: CommitMenuItem[], label: string): { label: string; command: () => void } {
  const item = items.find(
    (i): i is { label: string; command: () => void } => 'label' in i && i.label === label,
  )
  if (!item) throw new Error(`menu item not found: ${label}`)
  return item
}

describe('buildSquashMenuItems', () => {
  it('FR-LOG-14 with a blocker: one disabled item saying why, no actions', () => {
    const actions = { squash: vi.fn(), fixup: vi.fn() }
    const items = buildSquashMenuItems(2, 'a rebase is in progress', actions, t)

    expect(items).toEqual([
      { label: 'Squash and fixup not possible: a rebase is in progress', disabled: true },
    ])
  })
})
