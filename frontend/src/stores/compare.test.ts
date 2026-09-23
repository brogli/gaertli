import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockCompareCommits, mockCompareWithWorking } = vi.hoisted(() => ({
  mockCompareCommits: vi.fn(),
  mockCompareWithWorking: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { CompareCommits: mockCompareCommits, CompareWithWorking: mockCompareWithWorking },
}))

import { useCompareStore } from './compare'
import { commitRow } from '../testing/fixtures/log-rows'

const row = (sha: string) => commitRow({ sha, shortSha: sha.slice(0, 7) })

describe('compare store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockCompareCommits.mockReset()
    mockCompareWithWorking.mockReset()
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

  it('FR-CMP-04 startAgainstWorking sets workingCommit and clears any two-commit selection', () => {
    const compare = useCompareStore()
    compare.toggleSelect(row('x'))
    compare.toggleSelect(row('y'))

    const rowZ = row('z')
    compare.startAgainstWorking(rowZ)

    expect(compare.workingCommit).toEqual(rowZ)
    expect(compare.selection).toEqual([])
  })

  it('FR-CMP-04 toggleSelect clears workingCommit', () => {
    const compare = useCompareStore()
    compare.startAgainstWorking(row('z'))

    compare.toggleSelect(row('x'))

    expect(compare.workingCommit).toBeNull()
  })

  it('FR-CMP-04 loadAgainstWorking populates files from Facade.CompareWithWorking', async () => {
    const compare = useCompareStore()
    const files = [{ status: 'M', path: 'a.txt' }]
    mockCompareWithWorking.mockResolvedValue(files)

    await compare.loadAgainstWorking('/repo', 'sha1')

    expect(mockCompareWithWorking).toHaveBeenCalledWith('/repo', 'sha1')
    expect(compare.files).toEqual(files)
  })
})
