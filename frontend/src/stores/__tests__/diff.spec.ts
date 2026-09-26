import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({ Facade: { DiffWorking: vi.fn() } }))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

import { useDiffStore } from '../diff'
import { diffFileFixture } from '@/testing/fixtures/diff'
import { DiffKind, type DiffTarget } from '@/lib/diffTarget'
import type { DiffFile } from '../../../bindings/github.com/brogli/nebegit/internal/core'

const opts = { force: false, ignoreWhitespace: false }

// working is a.txt's working-tree diff on the staged or unstaged side.
function working(staged: boolean): DiffTarget {
  return { kind: DiffKind.WorkingDiff, path: 'a.txt', oldPath: '', staged }
}

describe('diff store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.DiffWorking.mockReset()
  })

  it('FR-CHG-04 load caches the staged and unstaged diff of the same path separately', async () => {
    const diff = useDiffStore()
    const staged = diffFileFixture({ oldContent: 'staged old', newContent: 'staged new' })
    const unstaged = diffFileFixture({ oldContent: 'unstaged old', newContent: 'unstaged new' })
    Facade.DiffWorking.mockResolvedValueOnce(staged).mockResolvedValueOnce(unstaged)

    await diff.load('/repo', working(true), opts)
    expect(diff.file).toEqual(staged)

    await diff.load('/repo', working(false), opts)
    expect(diff.file).toEqual(unstaged)

    await diff.load('/repo', working(true), opts)
    expect(diff.file).toEqual(staged)
    expect(Facade.DiffWorking).toHaveBeenCalledTimes(2)
  })

  it("FR-DIFF-07 a diff window moved to another worktree fetches that worktree's diff of the same path", async () => {
    const diff = useDiffStore()
    const here = diffFileFixture({ oldContent: 'old', newContent: 'in /repo' })
    const there = diffFileFixture({ oldContent: 'old', newContent: 'in /other' })
    Facade.DiffWorking.mockResolvedValueOnce(here).mockResolvedValueOnce(there)

    await diff.load('/repo', working(false), opts)
    await diff.load('/other', working(false), opts)

    expect(diff.file).toEqual(there)
  })

  it('FR-REPO-04 refetch re-runs the last load past the cache; unchanged content keeps the same file object', async () => {
    const diff = useDiffStore()
    const first = diffFileFixture({ oldContent: 'old', newContent: 'new' })
    Facade.DiffWorking.mockResolvedValueOnce(first)
      .mockResolvedValueOnce({ ...first })
      .mockResolvedValueOnce({ ...first, newContent: 'newer' })

    await diff.load('/repo', working(false), opts)
    const shown = diff.file
    await diff.refetch()
    expect(diff.file).toBe(shown)

    await diff.refetch()
    expect(diff.file?.newContent).toBe('newer')
    expect(Facade.DiffWorking).toHaveBeenCalledTimes(3)
    expect(Facade.DiffWorking).toHaveBeenLastCalledWith('/repo', 'a.txt', '', false, opts)
  })

  it('FR-DIFF-07 a late answer for a file left meanwhile is not shown', async () => {
    const diff = useDiffStore()
    const file = (path: string): DiffTarget => ({ ...working(false), path })
    let answerA: (f: DiffFile) => void = () => {}
    Facade.DiffWorking.mockResolvedValueOnce(
      diffFileFixture({ path: 'b.txt' }),
    ).mockReturnValueOnce(new Promise<DiffFile>((resolve) => (answerA = resolve)))
    await diff.load('/repo', file('b.txt'), opts)

    const slowA = diff.load('/repo', file('a.txt'), opts)
    await diff.load('/repo', file('b.txt'), opts) // from the cache
    expect(diff.file?.path).toBe('b.txt')
    expect(diff.loading).toBe(false)

    answerA(diffFileFixture({ path: 'a.txt' }))
    await slowA
    expect(diff.file?.path).toBe('b.txt')
    expect(diff.loading).toBe(false)
  })

  it('FR-REPO-04 refetch reloads the file shown, also when it came from the cache', async () => {
    const diff = useDiffStore()
    Facade.DiffWorking.mockImplementation((_wt, path: string) =>
      Promise.resolve(diffFileFixture({ path, newContent: `${path} now` })),
    )
    const file = (path: string): DiffTarget => ({ ...working(false), path })

    await diff.load('/repo', file('hunks.txt'), opts)
    await diff.load('/repo', file('readme.md'), opts)
    await diff.load('/repo', file('hunks.txt'), opts)
    await diff.refetch()

    expect(Facade.DiffWorking).toHaveBeenLastCalledWith('/repo', 'hunks.txt', '', false, opts)
    expect(diff.file?.path).toBe('hunks.txt')
  })
})
