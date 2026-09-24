import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({
  Facade: {
    LoadLog: vi.fn(),
    LogRows: vi.fn(),
    ReloadLog: vi.fn(),
    CommitDetails: vi.fn(),
    LogIndexOf: vi.fn(),
    SetLogFilter: vi.fn().mockResolvedValue(undefined),
  },
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

vi.mock('@wailsio/runtime', () => import('../testing/runtime'))
import { Events, emit } from '../testing/runtime'

import { useLogStore } from './log'
import { commitRow } from '../testing/fixtures/log-rows'
import type { LogCommit } from '../../bindings/github.com/brogli/nebegit/internal/core'

function rowFixture(sha: string) {
  return commitRow({ sha, shortSha: sha, subject: 'x' })
}

// rowsResult is a LogRows answer holding rows.
function rowsResult(rows: LogCommit[], generation = 0) {
  return { rows, loaded: rows.length, done: true, generation }
}

describe('log store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.LoadLog.mockReset().mockResolvedValue(undefined)
    Facade.LogRows.mockReset().mockResolvedValue(rowsResult([rowFixture('a')]))
    Facade.ReloadLog.mockReset().mockResolvedValue(undefined)
    Facade.LogIndexOf.mockReset()
    Events.On.mockReset()
  })

  it('FR-LOG-01 rows(from, to) caches its window: a repeated request does not call LogRows again', async () => {
    const log = useLogStore()
    await log.load('/repo')

    await log.rows(0, 1)
    await log.rows(0, 1)

    expect(Facade.LogRows).toHaveBeenCalledTimes(1)
  })

  it('rows(from, to) dedupes an in-flight request for the same window', async () => {
    const log = useLogStore()
    await log.load('/repo')

    let resolve!: (value: unknown) => void
    Facade.LogRows.mockReturnValueOnce(new Promise((r) => (resolve = r)))

    const first = log.rows(0, 1) // not yet cached: fetches
    const second = log.rows(0, 1) // requested again before the first resolves

    resolve(rowsResult([rowFixture('a')]))
    await Promise.all([first, second])

    expect(Facade.LogRows).toHaveBeenCalledTimes(1)
  })

  it('FR-LOG-09 done becomes true once a log:progress event with done: true arrives', async () => {
    const log = useLogStore()
    await log.load('/repo')
    expect(log.done).toBe(false)

    emit('log:progress', { data: { loaded: 5, done: true } })
    await flushPromises()

    expect(log.done).toBe(true)
    expect(log.loaded).toBe(5)
  })

  it("FR-LOG-09 another window's log:progress leaves this window's load as it is", async () => {
    const log = useLogStore()
    await log.load('/repo')
    await log.rows(0, 1)
    const state = () => ({ loaded: log.loaded, done: log.done, cached: log.rowCache.size })
    const before = state()

    emit('log:progress', {
      sender: 'window-2',
      data: { loaded: 9, done: true, filtered: false, generation: 4 },
    })
    await flushPromises()

    expect(state()).toEqual(before)
  })

  it('FR-REPO-04 calls ReloadLog on a refs repo:changed event, not on state', async () => {
    const log = useLogStore()
    await log.load('/repo')

    emit('repo:changed', { data: { scope: 'refs' } })
    emit('repo:changed', { data: { scope: 'state' } })
    await flushPromises()

    expect(Facade.ReloadLog).toHaveBeenCalledTimes(1)
    expect(Facade.ReloadLog).toHaveBeenCalledWith('/repo')
  })

  it('ADR-4 calls LoadLog again on a core:reconnected event', async () => {
    const log = useLogStore()
    await log.load('/repo')
    Facade.LoadLog.mockClear()

    emit('core:reconnected')
    await flushPromises()

    expect(Facade.LoadLog).toHaveBeenCalledTimes(1)
    expect(Facade.LoadLog).toHaveBeenCalledWith('/repo')
  })

  it('FR-REPO-04 drops the row cache when Generation changes', async () => {
    const log = useLogStore()
    await log.load('/repo')

    Facade.LogRows.mockResolvedValueOnce(rowsResult([rowFixture('a'), rowFixture('b')]))
    await log.rows(0, 2)
    expect(log.rowCache.size).toBe(2)

    Facade.LogRows.mockResolvedValueOnce(rowsResult([rowFixture('c')], 1))
    await log.rows(5, 6) // a range not yet cached, so this actually fetches

    expect(log.rowCache.size).toBe(1)
    expect(log.rowCache.get(5)?.sha).toBe('c')
    expect(log.rowCache.has(0)).toBe(false)
  })

  it('FR-REPO-04 a log:progress naming a new generation drops the cached rows, even at the same count', async () => {
    const log = useLogStore()
    await log.load('/repo')
    await log.rows(0, 1)

    emit('log:progress', { data: { loaded: 1, done: true, generation: 1 } })
    await flushPromises()
    expect(log.rowCache.size).toBe(0)

    Facade.LogRows.mockResolvedValueOnce(rowsResult([rowFixture('b')], 1))
    await log.rows(0, 1)
    expect(log.rowCache.get(0)?.sha).toBe('b')
  })

  it('FR-REPO-04 rows fetched before a reload do not refill the cache after it', async () => {
    const log = useLogStore()
    await log.load('/repo')
    let resolve!: (value: unknown) => void
    Facade.LogRows.mockReturnValueOnce(new Promise((r) => (resolve = r)))
    const stale = log.rows(0, 1)

    emit('log:progress', { data: { loaded: 1, done: true, generation: 1 } })
    await flushPromises()
    resolve(rowsResult([rowFixture('old')]))
    await stale

    expect(log.rowCache.has(0)).toBe(false)
  })

  it('FR-LOG-01 jumpTo selects the row and sets pendingScrollTo when the sha is found', async () => {
    const log = useLogStore()
    await log.load('/repo')
    Facade.LogIndexOf.mockResolvedValue({ index: 7, found: true })
    Facade.LogRows.mockResolvedValue(rowsResult([rowFixture('deadbeef')]))

    await log.jumpTo('deadbeef')

    expect(Facade.LogRows).toHaveBeenCalledWith('/repo', 7, 8)
    expect(log.selectedRow?.sha).toBe('deadbeef')
    expect(log.pendingScrollTo).toBe(7)
  })

  it('FR-LOG-01 jumpTo is a no-op when the sha is not found', async () => {
    const log = useLogStore()
    await log.load('/repo')
    Facade.LogIndexOf.mockResolvedValue({ index: 7, found: true })
    Facade.LogRows.mockResolvedValue(rowsResult([rowFixture('abc')]))
    await log.jumpTo('abc')
    expect(log.selectedRow?.sha).toBe('abc')

    Facade.LogIndexOf.mockResolvedValue({ index: 0, found: false })
    await log.jumpTo('missing')

    expect(log.selectedRow?.sha).toBe('abc')
    expect(log.pendingScrollTo).toBe(7)
  })

  it('FR-LOG-05 knownAuthors dedupes and sorts every author seen in fetched rows', async () => {
    const log = useLogStore()
    await log.load('/repo')
    Facade.LogRows.mockResolvedValueOnce(
      rowsResult([
        { ...rowFixture('a'), author: 'Bob' },
        { ...rowFixture('b'), author: 'Alice' },
        { ...rowFixture('c'), author: 'Bob' },
      ]),
    )

    await log.rows(0, 3)

    expect(log.knownAuthors).toEqual(['Alice', 'Bob'])
  })

  it('FR-REPO-04 a filtered log:progress naming a new generation drops the cached rows, even at the same count', async () => {
    const log = useLogStore()
    await log.load('/repo')
    await log.applyFilter({ path: 'x' })
    Facade.LogRows.mockResolvedValueOnce(rowsResult([rowFixture('a')]))
    await log.rows(0, 1)
    expect(log.rowCache.size).toBe(1)

    emit('log:progress', { data: { loaded: 1, done: true, filtered: true, generation: 1 } })
    await flushPromises()

    expect(log.rowCache.size).toBe(0)
  })

  it('FR-LOG-08 clearing a filter shows the base rows already fetched without refetching', async () => {
    const log = useLogStore()
    await log.load('/repo')
    await log.rows(0, 1)

    await log.applyFilter({ query: 'x' })
    await log.rows(0, 1)
    await log.applyFilter({})
    await log.rows(0, 1)

    expect(Facade.LogRows).toHaveBeenCalledTimes(2)
  })
})
