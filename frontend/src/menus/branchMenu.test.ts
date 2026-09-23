import { describe, expect, it, vi } from 'vitest'
import { buildBranchMenuItems } from './branchMenu'

// menuItems calls buildBranchMenuItems with vi.fn() for every callback not
// explicitly given, so each test only names the ones it cares about.
function menuItems(
  name: string,
  current: boolean,
  isRemote: boolean,
  hasUpstream: boolean,
  overrides: Partial<{
    checkout: (name: string) => void
    createWorktree: (name: string, isRemote: boolean) => void
    updateBranch: (name: string) => void
    openRename: (name: string) => void
    deleteBranch: (name: string) => void
  }> = {},
) {
  return buildBranchMenuItems(
    { name, current, isRemote, hasUpstream },
    {
      checkout: vi.fn(),
      createWorktree: vi.fn(),
      updateBranch: vi.fn(),
      openRename: vi.fn(),
      deleteBranch: vi.fn(),
      ...overrides,
    },
  )
}

describe('buildBranchMenuItems', () => {
  it('FR-BR-05 is empty for the current branch', () => {
    expect(menuItems('main', true, false, true)).toEqual([])
  })

  it('FR-BR-05 offers Checkout for a non-current branch, calling checkout with its name', () => {
    const checkout = vi.fn()
    const items = menuItems('feature', false, false, false, { checkout })

    expect(items.map((i) => i.label)).toEqual(['Checkout', 'Create worktree here…', 'Rename…', 'Delete'])
    items[0]!.command()
    expect(checkout).toHaveBeenCalledTimes(1)
    expect(checkout).toHaveBeenCalledWith('feature')
  })

  it('FR-WT-06 "Create worktree here…" calls createWorktree with the name and isRemote, for a local branch', () => {
    const createWorktree = vi.fn()
    const items = menuItems('feature', false, false, false, { createWorktree })

    items.find((i) => i.label === 'Create worktree here…')?.command()

    expect(createWorktree).toHaveBeenCalledTimes(1)
    expect(createWorktree).toHaveBeenCalledWith('feature', false)
  })

  it('FR-WT-06 "Create worktree here…" calls createWorktree with the name and isRemote, for a remote branch', () => {
    const createWorktree = vi.fn()
    const items = menuItems('origin/feature', false, true, false, { createWorktree })

    items.find((i) => i.label === 'Create worktree here…')?.command()

    expect(createWorktree).toHaveBeenCalledTimes(1)
    expect(createWorktree).toHaveBeenCalledWith('origin/feature', true)
  })

  it('FR-UPD-01 offers "Update from upstream" for a non-current local branch with an upstream, calling updateBranch with its name', () => {
    const updateBranch = vi.fn()
    const items = menuItems('feature', false, false, true, { updateBranch })

    expect(items.map((i) => i.label)).toEqual([
      'Checkout',
      'Create worktree here…',
      'Update from upstream',
      'Rename…',
      'Delete',
    ])
    items[2]!.command()
    expect(updateBranch).toHaveBeenCalledTimes(1)
    expect(updateBranch).toHaveBeenCalledWith('feature')
  })

  it('FR-UPD-01 omits "Update from upstream" for the current branch, a remote branch, or one with no upstream', () => {
    const withoutItem = (current: boolean, isRemote: boolean, hasUpstream: boolean) =>
      menuItems('feature', current, isRemote, hasUpstream).map((i) => i.label)

    expect(withoutItem(true, false, true)).not.toContain('Update from upstream')
    expect(withoutItem(false, true, true)).not.toContain('Update from upstream')
    expect(withoutItem(false, false, false)).not.toContain('Update from upstream')
  })

  it('FR-BR-06 offers Rename…/Delete for a non-current local branch, calling openRename/deleteBranch with its name', () => {
    const openRename = vi.fn()
    const deleteBranch = vi.fn()
    const items = menuItems('feature', false, false, false, { openRename, deleteBranch })

    expect(items.map((i) => i.label)).toEqual(['Checkout', 'Create worktree here…', 'Rename…', 'Delete'])
    items.find((i) => i.label === 'Rename…')?.command()
    items.find((i) => i.label === 'Delete')?.command()

    expect(openRename).toHaveBeenCalledTimes(1)
    expect(openRename).toHaveBeenCalledWith('feature')
    expect(deleteBranch).toHaveBeenCalledTimes(1)
    expect(deleteBranch).toHaveBeenCalledWith('feature')
  })

  it('FR-BR-06 omits Rename…/Delete for a non-current remote branch', () => {
    const items = menuItems('origin/feature', false, true, false)

    expect(items.map((i) => i.label)).not.toContain('Rename…')
    expect(items.map((i) => i.label)).not.toContain('Delete')
    expect(items.map((i) => i.label)).toEqual(['Checkout', 'Create worktree here…'])
  })

  it('FR-BR-06 stays empty for the current branch (Rename…/Delete included in the same early return)', () => {
    expect(menuItems('main', true, false, true)).toEqual([])
  })
})
