import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({ Facade: { DiffWorking: vi.fn() } }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

import { useDiffStore } from './diff'
import { diffFileFixture } from '../testing/fixtures/diff'

const opts = { force: false, ignoreWhitespace: false }

describe('diff store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.DiffWorking.mockReset()
  })

  it('FR-CHG-04 loadWorking caches the staged and unstaged diff of the same path separately', async () => {
    const diff = useDiffStore()
    const staged = diffFileFixture({ oldContent: 'staged old', newContent: 'staged new' })
    const unstaged = diffFileFixture({ oldContent: 'unstaged old', newContent: 'unstaged new' })
    Facade.DiffWorking.mockResolvedValueOnce(staged).mockResolvedValueOnce(unstaged)

    await diff.loadWorking('/repo', 'a.txt', '', true, opts)
    expect(diff.file).toEqual(staged)

    await diff.loadWorking('/repo', 'a.txt', '', false, opts)
    expect(diff.file).toEqual(unstaged)

    await diff.loadWorking('/repo', 'a.txt', '', true, opts)
    expect(diff.file).toEqual(staged)
    expect(Facade.DiffWorking).toHaveBeenCalledTimes(2)
  })

  it("FR-DIFF-07 a diff window moved to another worktree fetches that worktree's diff of the same path", async () => {
    const diff = useDiffStore()
    const here = diffFileFixture({ oldContent: 'old', newContent: 'in /repo' })
    const there = diffFileFixture({ oldContent: 'old', newContent: 'in /other' })
    Facade.DiffWorking.mockResolvedValueOnce(here).mockResolvedValueOnce(there)

    await diff.loadWorking('/repo', 'a.txt', '', false, opts)
    await diff.loadWorking('/other', 'a.txt', '', false, opts)

    expect(diff.file).toEqual(there)
  })

  it('FR-REPO-04 refetch re-runs the last load past the cache; unchanged content keeps the same file object', async () => {
    const diff = useDiffStore()
    const first = diffFileFixture({ oldContent: 'old', newContent: 'new' })
    Facade.DiffWorking.mockResolvedValueOnce(first)
      .mockResolvedValueOnce({ ...first })
      .mockResolvedValueOnce({ ...first, newContent: 'newer' })

    await diff.loadWorking('/repo', 'a.txt', '', false, opts)
    const shown = diff.file
    await diff.refetch()
    expect(diff.file).toBe(shown)

    await diff.refetch()
    expect(diff.file?.newContent).toBe('newer')
    expect(Facade.DiffWorking).toHaveBeenCalledTimes(3)
    expect(Facade.DiffWorking).toHaveBeenLastCalledWith('/repo', 'a.txt', '', false, opts)
  })
})
