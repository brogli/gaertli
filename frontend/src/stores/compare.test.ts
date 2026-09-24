import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({
  Facade: { CompareCommits: vi.fn(), CompareWithWorking: vi.fn(), LogIndexOf: vi.fn() },
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

import { useCompareStore } from './compare'
import { commitRow } from '../testing/fixtures/log-rows'

const row = (sha: string) => commitRow({ sha, shortSha: sha.slice(0, 7) })
const wt = '/repo'

describe('compare store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.CompareCommits.mockReset()
    Facade.CompareWithWorking.mockReset()
    Facade.LogIndexOf.mockReset().mockResolvedValue({ index: 0, found: false })
  })

  it('FR-CMP-01 toggleSelect keeps the two most recently picked rows (FIFO)', () => {
    const compare = useCompareStore()
    const rowX = row('x')
    const rowY = row('y')
    const rowZ = row('z')

    compare.toggleSelect(wt, rowX, 0)
    compare.toggleSelect(wt, rowY, 1)
    expect(compare.selection).toEqual([rowX, rowY])
    expect(compare.active).toBe(true)

    compare.toggleSelect(wt, rowZ, 2)
    expect(compare.selection).toEqual([rowY, rowZ])
  })

  it('FR-CMP-01 toggleSelect on an already-picked row deselects it', () => {
    const compare = useCompareStore()
    const rowX = row('x')
    compare.toggleSelect(wt, rowX, 0)

    compare.toggleSelect(wt, rowX, 0)

    expect(compare.selection).toEqual([])
  })

  it('FR-CMP-01 from is the row with the higher log index (older), to is the lower (newer), regardless of click order', () => {
    const compare = useCompareStore()
    const newerRow = row('newer')
    const olderRow = row('older')

    compare.toggleSelect(wt, newerRow, 2)
    compare.toggleSelect(wt, olderRow, 7)

    expect(compare.from).toEqual(olderRow)
    expect(compare.to).toEqual(newerRow)
    expect(compare.older).toEqual(olderRow)
    expect(compare.newer).toEqual(newerRow)
  })

  it('FR-CMP-03 swap reverses from/to but not which commit is older/newer', () => {
    const compare = useCompareStore()
    const newerRow = row('newer')
    const olderRow = row('older')
    compare.toggleSelect(wt, newerRow, 2)
    compare.toggleSelect(wt, olderRow, 7)

    compare.swap()

    expect(compare.from).toEqual(newerRow)
    expect(compare.to).toEqual(olderRow)
    expect(compare.older).toEqual(olderRow)
    expect(compare.newer).toEqual(newerRow)
  })

  it('FR-CMP-01 once the pair completes, both picks are resolved against the current log so a reload between them cannot swap older/newer', async () => {
    const compare = useCompareStore()
    const olderRow = row('older')
    const newerRow = row('newer')

    // olderRow was picked while it sat at row 0; an outside commit then
    // reloaded the log and pushed it down to row 1 before newerRow (the
    // reload's new row 0) was picked. Trusting the stale snapshot (0 and 0)
    // would leave the pair unordered; LogIndexOf gives the current truth.
    compare.toggleSelect(wt, olderRow, 0)
    Facade.LogIndexOf.mockImplementation((_wt: string, sha: string) =>
      Promise.resolve({ index: sha === olderRow.sha ? 1 : 0, found: true }),
    )
    compare.toggleSelect(wt, newerRow, 0)

    await vi.waitFor(() => expect(compare.older).toEqual(olderRow))
    expect(compare.newer).toEqual(newerRow)
  })

  it('FR-CMP-01 a resolved index for a pick no longer selected is dropped, not applied to the wrong commit', async () => {
    const compare = useCompareStore()
    const rowA = row('a')
    const rowB = row('b')
    const rowC = row('c')
    Facade.LogIndexOf.mockImplementation((_wt: string, sha: string) =>
      Promise.resolve({ index: sha === 'a' ? 5 : sha === 'c' ? 0 : 1, found: true }),
    )

    compare.toggleSelect(wt, rowA, 5)
    compare.toggleSelect(wt, rowB, 0) // completes (A, B); its resolution is now in flight
    compare.toggleSelect(wt, rowB, 0) // B deselected before that resolution lands
    compare.toggleSelect(wt, rowC, 0) // (A, C) is the pair by the time it does

    await vi.waitFor(() => expect(compare.older).toEqual(rowA))
    expect(compare.newer).toEqual(rowC)
  })

  it('FR-CMP-04 startAgainstWorking sets workingCommit and clears any two-commit selection', () => {
    const compare = useCompareStore()
    compare.toggleSelect(wt, row('x'), 0)
    compare.toggleSelect(wt, row('y'), 1)

    const rowZ = row('z')
    compare.startAgainstWorking(rowZ)

    expect(compare.workingCommit).toEqual(rowZ)
    expect(compare.selection).toEqual([])
  })

  it('FR-CMP-04 toggleSelect clears workingCommit', () => {
    const compare = useCompareStore()
    compare.startAgainstWorking(row('z'))

    compare.toggleSelect(wt, row('x'), 0)

    expect(compare.workingCommit).toBeNull()
  })

  it('FR-CMP-04 loadAgainstWorking populates files from Facade.CompareWithWorking', async () => {
    const compare = useCompareStore()
    const files = [{ status: 'M', path: 'a.txt' }]
    Facade.CompareWithWorking.mockResolvedValue(files)

    await compare.loadAgainstWorking('/repo', 'sha1')

    expect(Facade.CompareWithWorking).toHaveBeenCalledWith('/repo', 'sha1')
    expect(compare.files).toEqual(files)
  })
})
