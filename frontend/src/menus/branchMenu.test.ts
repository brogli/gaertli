import { describe, expect, it, vi } from 'vitest'
import { buildBranchMenuItems } from './branchMenu'

describe('buildBranchMenuItems', () => {
  it('FR-BR-05 is empty for the current branch', () => {
    expect(buildBranchMenuItems('main', true, false, true, vi.fn(), vi.fn(), vi.fn())).toEqual([])
  })

  it('FR-BR-05 offers Checkout for a non-current branch, calling checkout with its name', () => {
    const checkout = vi.fn()
    const items = buildBranchMenuItems('feature', false, false, false, checkout, vi.fn(), vi.fn())

    expect(items.map((i) => i.label)).toEqual(['Checkout', 'Create worktree here…'])
    items[0]!.command()
    expect(checkout).toHaveBeenCalledTimes(1)
    expect(checkout).toHaveBeenCalledWith('feature')
  })

  it('FR-WT-06 "Create worktree here…" calls createWorktree with the name and isRemote, for a local branch', () => {
    const createWorktree = vi.fn()
    const items = buildBranchMenuItems(
      'feature',
      false,
      false,
      false,
      vi.fn(),
      createWorktree,
      vi.fn(),
    )

    items.find((i) => i.label === 'Create worktree here…')?.command()

    expect(createWorktree).toHaveBeenCalledTimes(1)
    expect(createWorktree).toHaveBeenCalledWith('feature', false)
  })

  it('FR-WT-06 "Create worktree here…" calls createWorktree with the name and isRemote, for a remote branch', () => {
    const createWorktree = vi.fn()
    const items = buildBranchMenuItems(
      'origin/feature',
      false,
      true,
      false,
      vi.fn(),
      createWorktree,
      vi.fn(),
    )

    items.find((i) => i.label === 'Create worktree here…')?.command()

    expect(createWorktree).toHaveBeenCalledTimes(1)
    expect(createWorktree).toHaveBeenCalledWith('origin/feature', true)
  })

  it('FR-UPD-01 offers "Update from upstream" for a non-current local branch with an upstream, calling updateBranch with its name', () => {
    const updateBranch = vi.fn()
    const items = buildBranchMenuItems(
      'feature',
      false,
      false,
      true,
      vi.fn(),
      vi.fn(),
      updateBranch,
    )

    expect(items.map((i) => i.label)).toEqual([
      'Checkout',
      'Create worktree here…',
      'Update from upstream',
    ])
    items[2]!.command()
    expect(updateBranch).toHaveBeenCalledTimes(1)
    expect(updateBranch).toHaveBeenCalledWith('feature')
  })

  it('FR-UPD-01 omits "Update from upstream" for the current branch, a remote branch, or one with no upstream', () => {
    const withoutItem = (current: boolean, isRemote: boolean, hasUpstream: boolean) =>
      buildBranchMenuItems(
        'feature',
        current,
        isRemote,
        hasUpstream,
        vi.fn(),
        vi.fn(),
        vi.fn(),
      ).map((i) => i.label)

    expect(withoutItem(true, false, true)).not.toContain('Update from upstream')
    expect(withoutItem(false, true, true)).not.toContain('Update from upstream')
    expect(withoutItem(false, false, false)).not.toContain('Update from upstream')
  })
})
