import { describe, expect, it, vi } from 'vitest'
import { buildBranchMenuItems } from './branchMenu'

describe('buildBranchMenuItems', () => {
  it('FR-BR-05 is empty for the current branch', () => {
    expect(buildBranchMenuItems('main', true, false, vi.fn(), vi.fn())).toEqual([])
  })

  it('FR-BR-05 offers Checkout for a non-current branch, calling checkout with its name', () => {
    const checkout = vi.fn()
    const items = buildBranchMenuItems('feature', false, false, checkout, vi.fn())

    expect(items.map((i) => i.label)).toEqual(['Checkout', 'Create worktree here…'])
    items[0]!.command()
    expect(checkout).toHaveBeenCalledTimes(1)
    expect(checkout).toHaveBeenCalledWith('feature')
  })

  it('FR-WT-06 "Create worktree here…" calls createWorktree with the name and isRemote, for a local branch', () => {
    const createWorktree = vi.fn()
    const items = buildBranchMenuItems('feature', false, false, vi.fn(), createWorktree)

    items.find((i) => i.label === 'Create worktree here…')?.command()

    expect(createWorktree).toHaveBeenCalledTimes(1)
    expect(createWorktree).toHaveBeenCalledWith('feature', false)
  })

  it('FR-WT-06 "Create worktree here…" calls createWorktree with the name and isRemote, for a remote branch', () => {
    const createWorktree = vi.fn()
    const items = buildBranchMenuItems('origin/feature', false, true, vi.fn(), createWorktree)

    items.find((i) => i.label === 'Create worktree here…')?.command()

    expect(createWorktree).toHaveBeenCalledTimes(1)
    expect(createWorktree).toHaveBeenCalledWith('origin/feature', true)
  })
})
