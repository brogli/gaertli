import { describe, expect, it, vi } from 'vitest'
import { buildCommitMenuItems } from './commitMenu'
import { commitRow as row } from '../testing/fixtures/log-rows'

function actions() {
  return {
    openCreateBranch: vi.fn(),
    openCheckoutCommit: vi.fn(),
    openReset: vi.fn(),
    cherryPick: vi.fn(),
    revert: vi.fn(),
  }
}

describe('buildCommitMenuItems', () => {
  it('FR-LOG-11 returns copy hash, copy subject, new branch, check out commit, reset, cherry-pick, and revert, in that order', () => {
    const items = buildCommitMenuItems(row(), actions())

    expect(items.map((i) => i.label)).toEqual([
      'Copy hash',
      'Copy subject',
      'New branch from here…',
      'Check out commit',
      'Reset current branch to here…',
      'Cherry-pick',
      'Revert',
    ])
  })

  it('FR-BR-01 "New branch from here…" calls openCreateBranch with the row\'s sha', () => {
    const a = actions()
    const items = buildCommitMenuItems(row(), a)

    items.find((i) => i.label === 'New branch from here…')?.command()

    expect(a.openCreateBranch).toHaveBeenCalledTimes(1)
    expect(a.openCreateBranch).toHaveBeenCalledWith('abc123')
  })

  it('FR-BR-08 "Check out commit" calls openCheckoutCommit with the row\'s sha', () => {
    const a = actions()
    const items = buildCommitMenuItems(row(), a)

    items.find((i) => i.label === 'Check out commit')?.command()

    expect(a.openCheckoutCommit).toHaveBeenCalledTimes(1)
    expect(a.openCheckoutCommit).toHaveBeenCalledWith('abc123')
  })

  it('FR-BR-02 "Reset current branch to here…" calls openReset with the row\'s sha and subject', () => {
    const a = actions()
    const items = buildCommitMenuItems(row(), a)

    items.find((i) => i.label === 'Reset current branch to here…')?.command()

    expect(a.openReset).toHaveBeenCalledTimes(1)
    expect(a.openReset).toHaveBeenCalledWith('abc123', 'Fix the bug')
  })

  it('FR-LOG-11 "Cherry-pick" and "Revert" each call their own spy once', () => {
    const a = actions()
    const items = buildCommitMenuItems(row(), a)

    items.find((i) => i.label === 'Cherry-pick')?.command()
    items.find((i) => i.label === 'Revert')?.command()

    expect(a.cherryPick).toHaveBeenCalledTimes(1)
    expect(a.revert).toHaveBeenCalledTimes(1)
  })

  it('FR-LOG-12 the two existing copy items are unchanged', () => {
    const writeText = vi.fn()
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    const items = buildCommitMenuItems(row(), actions())

    items.find((i) => i.label === 'Copy hash')?.command()
    items.find((i) => i.label === 'Copy subject')?.command()

    expect(writeText).toHaveBeenNthCalledWith(1, 'abc123')
    expect(writeText).toHaveBeenNthCalledWith(2, 'Fix the bug')
  })

  it('FR-CMP-01, FR-LOG-11 with compareWithSelected provided, appends "Compare with selected" last and calls it', () => {
    const compareWithSelected = vi.fn()
    const items = buildCommitMenuItems(row(), { ...actions(), compareWithSelected })

    expect(items.at(-1)?.label).toBe('Compare with selected')
    items.at(-1)?.command()

    expect(compareWithSelected).toHaveBeenCalledTimes(1)
  })

  it('FR-CMP-01 with compareWithSelected omitted, returns exactly the existing 7 items', () => {
    const items = buildCommitMenuItems(row(), actions())

    expect(items).toHaveLength(7)
    expect(items.map((i) => i.label)).not.toContain('Compare with selected')
  })
})
