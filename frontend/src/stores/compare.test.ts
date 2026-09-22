import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockCompareCommits } = vi.hoisted(() => ({ mockCompareCommits: vi.fn() }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { CompareCommits: mockCompareCommits },
}))

import { useCompareStore } from './compare'
import type { LogCommit } from '../../bindings/github.com/brogli/nebegit/internal/core'

function row(sha: string): LogCommit {
  return { sha, shortSha: sha.slice(0, 7), author: '', authorAt: 0, subject: '', graph: { lane: 0, colour: 0, edges: [] } }
}

describe('compare store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockCompareCommits.mockReset()
  })

  it('FR-CMP-01 toggleSelect keeps the two most recently picked rows (FIFO)', () => {
    const compare = useCompareStore()
    const rowX = row('x')
    const rowY = row('y')
    const rowZ = row('z')

    compare.toggleSelect(rowX)
    compare.toggleSelect(rowY)
    expect(compare.selection).toEqual([rowX, rowY])
    expect(compare.active).toBe(true)

    compare.toggleSelect(rowZ)
    expect(compare.selection).toEqual([rowY, rowZ])
  })

  it('FR-CMP-01 toggleSelect on an already-picked row deselects it', () => {
    const compare = useCompareStore()
    const rowX = row('x')
    compare.toggleSelect(rowX)

    compare.toggleSelect(rowX)

    expect(compare.selection).toEqual([])
  })
})
