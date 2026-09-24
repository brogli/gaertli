import { describe, expect, it, vi } from 'vitest'
import {
  buildComparisonMenuItems,
  buildOneCommitMenuItems,
  buildRangeMenuItems,
  type CommitMenuItem,
} from './commitMenu'
import { commitRow as row } from '../testing/fixtures/log-rows'
import { i18n } from '../i18n'

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

  it('FR-WT-06, FR-LOG-11 "Create worktree here…" calls openCreateWorktree with the row\'s sha', () => {
    const a = oneCommitActions()
    const items = buildOneCommitMenuItems(row(), a, t)

    findItem(items, 'Create worktree here…').command()

    expect(a.openCreateWorktree).toHaveBeenCalledTimes(1)
    expect(a.openCreateWorktree).toHaveBeenCalledWith('abc123')
  })

  it('FR-BR-01 "New branch from here…" calls openCreateBranch with the row\'s sha', () => {
    const a = oneCommitActions()
    const items = buildOneCommitMenuItems(row(), a, t)

    findItem(items, 'New branch from here…').command()

    expect(a.openCreateBranch).toHaveBeenCalledTimes(1)
    expect(a.openCreateBranch).toHaveBeenCalledWith('abc123')
  })

  it('FR-BR-08 "Check out commit" calls openCheckoutCommit with the row\'s sha', () => {
    const a = oneCommitActions()
    const items = buildOneCommitMenuItems(row(), a, t)

    findItem(items, 'Check out commit').command()

    expect(a.openCheckoutCommit).toHaveBeenCalledTimes(1)
    expect(a.openCheckoutCommit).toHaveBeenCalledWith('abc123')
  })

  it('FR-BR-02 "Reset current branch to here…" calls openReset with the row\'s sha and subject', () => {
    const a = oneCommitActions()
    const items = buildOneCommitMenuItems(row(), a, t)

    findItem(items, 'Reset current branch to here…').command()

    expect(a.openReset).toHaveBeenCalledTimes(1)
    expect(a.openReset).toHaveBeenCalledWith('abc123', 'Fix the bug')
  })

  it('FR-LOG-11 "Cherry-pick" and "Revert" each call their own spy once', () => {
    const a = oneCommitActions()
    const items = buildOneCommitMenuItems(row(), a, t)

    findItem(items, 'Cherry-pick').command()
    findItem(items, 'Revert').command()

    expect(a.cherryPick).toHaveBeenCalledTimes(1)
    expect(a.revert).toHaveBeenCalledTimes(1)
  })

  it('FR-CP-06, FR-CP-08, FR-RV-04 "Cherry-pick…" and "Revert…" each call their own options spy once', () => {
    const a = oneCommitActions()
    const items = buildOneCommitMenuItems(row(), a, t)

    findItem(items, 'Cherry-pick…').command()
    findItem(items, 'Revert…').command()

    expect(a.cherryPickOptions).toHaveBeenCalledTimes(1)
    expect(a.revertOptions).toHaveBeenCalledTimes(1)
  })

  it('FR-LOG-12 the two copy items write to the clipboard', () => {
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

describe('buildRangeMenuItems', () => {
  it('FR-LOG-11 range of 3: exactly the count-labelled cherry-pick, revert and copy items', () => {
    const rows = [row({ sha: 'a' }), row({ sha: 'b' }), row({ sha: 'c' })]

    const items = buildRangeMenuItems(
      rows,
      {
        cherryPick: vi.fn(),
        cherryPickOptions: vi.fn(),
        revert: vi.fn(),
        revertOptions: vi.fn(),
        copyHashes: vi.fn(),
      },
      t,
    )

    expect(labelsOf(items)).toEqual([
      'Cherry-pick 3 commits',
      'Cherry-pick 3 commits…',
      'Revert 3 commits',
      'Revert 3 commits…',
      'Copy 3 hashes',
    ])
  })

  it('FR-UX-11 a range of 1 selects the singular plural form', () => {
    const items = buildRangeMenuItems(
      [row()],
      {
        cherryPick: vi.fn(),
        cherryPickOptions: vi.fn(),
        revert: vi.fn(),
        revertOptions: vi.fn(),
        copyHashes: vi.fn(),
      },
      t,
    )

    expect(labelsOf(items)).toEqual([
      'Cherry-pick 1 commit',
      'Cherry-pick 1 commit…',
      'Revert 1 commit',
      'Revert 1 commit…',
      'Copy 1 hash',
    ])
  })

  it('FR-LOG-11 each range item calls its own action', () => {
    const rows = [row({ sha: 'a' }), row({ sha: 'b' })]
    const actions = {
      cherryPick: vi.fn(),
      cherryPickOptions: vi.fn(),
      revert: vi.fn(),
      revertOptions: vi.fn(),
      copyHashes: vi.fn(),
    }
    const items = buildRangeMenuItems(rows, actions, t)

    findItem(items, 'Cherry-pick 2 commits').command()
    findItem(items, 'Cherry-pick 2 commits…').command()
    findItem(items, 'Revert 2 commits').command()
    findItem(items, 'Revert 2 commits…').command()
    findItem(items, 'Copy 2 hashes').command()

    expect(actions.cherryPick).toHaveBeenCalledTimes(1)
    expect(actions.cherryPickOptions).toHaveBeenCalledTimes(1)
    expect(actions.revert).toHaveBeenCalledTimes(1)
    expect(actions.revertOptions).toHaveBeenCalledTimes(1)
    expect(actions.copyHashes).toHaveBeenCalledTimes(1)
  })
})

describe('buildComparisonMenuItems', () => {
  it('FR-LOG-11 exactly: Swap, Stop comparing, Copy both hashes', () => {
    const items = buildComparisonMenuItems(
      { swap: vi.fn(), stopComparing: vi.fn(), copyBothHashes: vi.fn() },
      t,
    )

    expect(labelsOf(items)).toEqual(['Swap', 'Stop comparing', 'Copy both hashes'])
  })

  it('FR-CMP-03 "Swap" and "Stop comparing" call their own actions', () => {
    const actions = { swap: vi.fn(), stopComparing: vi.fn(), copyBothHashes: vi.fn() }
    const items = buildComparisonMenuItems(actions, t)

    findItem(items, 'Swap').command()
    findItem(items, 'Stop comparing').command()

    expect(actions.swap).toHaveBeenCalledTimes(1)
    expect(actions.stopComparing).toHaveBeenCalledTimes(1)
  })

  it('FR-LOG-11 "Copy both hashes" calls copyBothHashes', () => {
    const actions = { swap: vi.fn(), stopComparing: vi.fn(), copyBothHashes: vi.fn() }
    const items = buildComparisonMenuItems(actions, t)

    findItem(items, 'Copy both hashes').command()

    expect(actions.copyBothHashes).toHaveBeenCalledTimes(1)
  })
})

function findItem(items: CommitMenuItem[], label: string): { label: string; command: () => void } {
  const item = items.find(
    (i): i is { label: string; command: () => void } => 'label' in i && i.label === label,
  )
  if (!item) throw new Error(`menu item not found: ${label}`)
  return item
}
