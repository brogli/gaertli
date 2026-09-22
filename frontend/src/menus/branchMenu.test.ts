import { describe, expect, it, vi } from 'vitest'
import { buildBranchMenuItems } from './branchMenu'

describe('buildBranchMenuItems', () => {
  it('FR-BR-05 is empty for the current branch', () => {
    expect(buildBranchMenuItems('main', true, vi.fn())).toEqual([])
  })

  it('FR-BR-05 offers Checkout for a non-current branch, calling checkout with its name', () => {
    const checkout = vi.fn()
    const items = buildBranchMenuItems('feature', false, checkout)

    expect(items.map((i) => i.label)).toEqual(['Checkout'])
    items[0]!.command()
    expect(checkout).toHaveBeenCalledTimes(1)
    expect(checkout).toHaveBeenCalledWith('feature')
  })
})
