// Requirements: FR-CONF-11
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({
  Facade: { MergeDraft: vi.fn(), SetMergeDraft: vi.fn() },
}))
vi.mock('../../../bindings/github.com/brogli/gaertli/internal/app', () => ({ Facade }))

import { useConflictsStore } from '../conflicts'

const read = (text: string) => () => ({ text, fingerprint: 'fp', changes: [] })
const stored = (conflictId: string, text = 'core') => ({
  text,
  fingerprint: 'fp',
  conflictId,
  changes: null,
})

describe('conflicts store', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    setActivePinia(createPinia())
    Facade.MergeDraft.mockReset().mockResolvedValue(null)
    Facade.SetMergeDraft.mockReset().mockResolvedValue(undefined)
  })
  afterEach(() => vi.useRealTimers())

  it('a draft belongs to its conflict: another conflict in the path sees none', () => {
    const conflicts = useConflictsStore()
    conflicts.set('/repo', 'a.txt', 'c1', read('x'))

    expect(conflicts.get('/repo', 'a.txt', 'c1')?.text).toBe('x')
    expect(conflicts.has('/repo', 'a.txt', 'c2')).toBe(false)
    expect(conflicts.get('/repo', 'a.txt', 'c2')).toBeUndefined()
  })

  it("load keeps the page's draft over the core's", async () => {
    Facade.MergeDraft.mockResolvedValue(stored('c1'))
    const conflicts = useConflictsStore()
    conflicts.set('/repo', 'a.txt', 'c1', read('page'))

    await conflicts.load('/repo', 'a.txt', 'c1')

    expect(conflicts.get('/repo', 'a.txt', 'c1')?.text).toBe('page')
    expect(Facade.MergeDraft).not.toHaveBeenCalled()
  })

  it("load takes the core's draft for the conflict, with no changes as an empty list", async () => {
    Facade.MergeDraft.mockResolvedValue(stored('c1'))
    const conflicts = useConflictsStore()

    await conflicts.load('/repo', 'a.txt', 'c1')

    expect(conflicts.get('/repo', 'a.txt', 'c1')).toEqual({
      text: 'core',
      fingerprint: 'fp',
      conflictId: 'c1',
      changes: [],
    })
  })

  it("load ignores the core's draft of another conflict and drops the page's earlier one", async () => {
    Facade.MergeDraft.mockResolvedValue(stored('c1', 'old'))
    const conflicts = useConflictsStore()
    conflicts.set('/repo', 'a.txt', 'c1', read('page'))

    await conflicts.load('/repo', 'a.txt', 'c2')

    expect(conflicts.get('/repo', 'a.txt', 'c1')).toBeUndefined()
    expect(conflicts.get('/repo', 'a.txt', 'c2')).toBeUndefined()
  })

  it('set writes at once and once more after the pause for a burst of edits', async () => {
    const conflicts = useConflictsStore()

    conflicts.set('/repo', 'a.txt', 'c1', read('1'))
    conflicts.set('/repo', 'a.txt', 'c1', read('2'))
    conflicts.set('/repo', 'a.txt', 'c1', read('3'))
    expect(Facade.SetMergeDraft).toHaveBeenCalledTimes(1)

    await vi.advanceTimersByTimeAsync(500)

    expect(Facade.SetMergeDraft).toHaveBeenCalledTimes(2)
    expect(Facade.SetMergeDraft.mock.calls[1]![2]).toMatchObject({ text: '3', conflictId: 'c1' })
  })

  it('flush sends the queued write without waiting for the pause', () => {
    const conflicts = useConflictsStore()
    conflicts.set('/repo', 'a.txt', 'c1', read('1'))
    conflicts.set('/repo', 'a.txt', 'c1', read('2'))

    conflicts.flush('/repo', 'a.txt')

    expect(Facade.SetMergeDraft).toHaveBeenCalledTimes(2)
    expect(Facade.SetMergeDraft.mock.calls[1]![2]).toMatchObject({ text: '2' })
  })

  it('discard clears the draft in the core and resolves after that write', async () => {
    let cleared = false
    Facade.SetMergeDraft.mockImplementation(async (_wt, _path, draft) => {
      if (draft !== null) return
      await Promise.resolve()
      cleared = true
    })
    const conflicts = useConflictsStore()
    conflicts.set('/repo', 'a.txt', 'c1', read('x'))

    await conflicts.discard('/repo', 'a.txt')

    expect(cleared).toBe(true)
    expect(Facade.SetMergeDraft).toHaveBeenLastCalledWith('/repo', 'a.txt', null)
    expect(conflicts.has('/repo', 'a.txt', 'c1')).toBe(false)
  })

  it('a failed write is reported until a later one succeeds', async () => {
    Facade.SetMergeDraft.mockRejectedValueOnce(new Error('disk full'))
    const conflicts = useConflictsStore()

    conflicts.set('/repo', 'a.txt', 'c1', read('x'))
    await vi.advanceTimersByTimeAsync(0)
    expect(conflicts.writeError('/repo', 'a.txt')).toBeDefined()

    await vi.advanceTimersByTimeAsync(500)
    conflicts.set('/repo', 'a.txt', 'c1', read('y'))
    await vi.advanceTimersByTimeAsync(0)
    expect(conflicts.writeError('/repo', 'a.txt')).toBeUndefined()
  })

  it('keepOnly drops the other paths of the worktree and leaves other worktrees alone', () => {
    const conflicts = useConflictsStore()
    conflicts.set('/repo', 'one.txt', 'c', read('1'))
    conflicts.set('/repo', 'two.txt', 'c', read('2'))
    conflicts.set('/other', 'two.txt', 'c', read('3'))

    conflicts.keepOnly('/repo', ['one.txt'])

    expect(conflicts.has('/repo', 'one.txt', 'c')).toBe(true)
    expect(conflicts.has('/repo', 'two.txt', 'c')).toBe(false)
    expect(conflicts.has('/other', 'two.txt', 'c')).toBe(true)
  })
})
