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
    SearchLog: vi.fn(),
  },
}))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))
import { Events, emit } from '@/testing/runtime'

import { useLogStore } from '../log'
import { commitRow } from '@/testing/fixtures/log-rows'
import {
  ChangeScope,
  type LogCommit,
  type LogSearchResult,
} from '../../../bindings/github.com/brogli/nebegit/internal/core'

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
    Facade.SearchLog.mockReset().mockResolvedValue({ indices: [], generation: 0, filtered: false })
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

  it("FR-LOG-09 a failed load's error shows for its own view until a newer progress", async () => {
    const log = useLogStore()
    await log.load('/repo')
    const failure = { code: 'GIT_FAILED', message: 'bad object HEAD' }

    emit('log:progress', { data: { loaded: 0, done: true, generation: 0, error: failure } })
    await flushPromises()
    expect(log.error).toEqual(failure)

    await log.applyFilter({ path: 'x' })
    expect(log.error).toBeNull()
    emit('log:progress', {
      data: { loaded: 0, done: true, filtered: true, generation: 1, error: failure },
    })
    await flushPromises()
    expect(log.error).toEqual(failure)

    await log.applyFilter({})
    expect(log.error).toEqual(failure)
    emit('log:progress', { data: { loaded: 3, done: true, generation: 1 } })
    await flushPromises()
    expect(log.error).toBeNull()
  })

  it("FR-LOG-09 another window's log:progress leaves this window's load as it is", async () => {
    const log = useLogStore()
    await log.load('/repo')
    await log.rows(0, 1)
    const state = () => ({ loaded: log.loaded, cached: log.rowCache.size })
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

    emit('repo:changed', { data: { scope: ChangeScope.RefsChanged } })
    emit('repo:changed', { data: { scope: ChangeScope.StateChanged } })
    await flushPromises()

    expect(Facade.ReloadLog).toHaveBeenCalledTimes(1)
    expect(Facade.ReloadLog).toHaveBeenCalledWith('/repo')
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

    await log.applyFilter({ path: 'x' })
    await log.rows(0, 1)
    await log.applyFilter({})
    await log.rows(0, 1)

    expect(Facade.LogRows).toHaveBeenCalledTimes(2)
  })

  it('FR-LOG-05 a resolved search selects its first match and scrolls to it', async () => {
    vi.useFakeTimers()
    try {
      const log = useLogStore()
      await log.load('/repo')
      Facade.SearchLog.mockResolvedValueOnce({ indices: [9], generation: 0, filtered: false })
      Facade.LogRows.mockResolvedValueOnce(rowsResult([rowFixture('row9')]))

      log.query = 'fix'
      await vi.advanceTimersByTimeAsync(250)
      await flushPromises()

      expect(Facade.LogRows).toHaveBeenCalledWith('/repo', 9, 10)
      expect(log.selectedRow?.sha).toBe('row9')
      expect(log.pendingScrollTo).toBe(9)
    } finally {
      vi.useRealTimers()
    }
  })

  it('FR-LOG-05 drops a stale search answer whose generation is older than the current view', async () => {
    vi.useFakeTimers()
    try {
      const log = useLogStore()
      await log.load('/repo')

      let resolveFirst!: (value: LogSearchResult) => void
      Facade.SearchLog.mockReturnValueOnce(
        new Promise((resolve) => {
          resolveFirst = resolve
        }),
      )

      log.query = 'fix'
      await vi.advanceTimersByTimeAsync(250) // fires the debounced search; still pending

      // the view moves on to a newer generation before that search answers
      Facade.SearchLog.mockResolvedValueOnce({ indices: [5], generation: 1, filtered: false })
      emit('log:progress', { data: { loaded: 1, done: true, generation: 1 } })
      await flushPromises()
      expect(log.matches).toEqual([5])

      // the first (generation 0) search now answers, late: it must not
      // override the generation-1 result already showing
      resolveFirst({ indices: [2], generation: 0, filtered: false })
      await flushPromises()

      expect(log.matches).toEqual([5])
    } finally {
      vi.useRealTimers()
    }
  })

  it('FR-LOG-05 drops a base-view search answer once a filter is active, even at a higher raw generation number', async () => {
    vi.useFakeTimers()
    try {
      const log = useLogStore()
      await log.load('/repo')

      // the base view is already well past a few reloads
      emit('log:progress', { data: { loaded: 1, done: true, filtered: false, generation: 5 } })
      await flushPromises()

      let resolveBase!: (value: LogSearchResult) => void
      Facade.SearchLog.mockReturnValueOnce(
        new Promise((resolve) => {
          resolveBase = resolve
        }),
      )
      log.query = 'fix'
      await vi.advanceTimersByTimeAsync(250) // fires the base-view search; still pending

      // a filter is applied while that search is in flight: its own view
      // starts at generation 1, a lower number than the base view's 5
      log.filter = { path: 'x' }
      Facade.SearchLog.mockResolvedValueOnce({ indices: [3], generation: 1, filtered: true })
      emit('log:progress', { data: { loaded: 1, done: true, filtered: true, generation: 1 } })
      await flushPromises()
      expect(log.matches).toEqual([3])

      // the base-view search now answers, late, with a higher raw
      // generation number (5) than the filtered view's (1): comparing
      // generation alone would accept it; it must still be dropped, since
      // it belongs to the wrong view
      resolveBase({ indices: [7], generation: 5, filtered: false })
      await flushPromises()

      expect(log.matches).toEqual([3])
    } finally {
      vi.useRealTimers()
    }
  })

  it('FR-LOG-05 re-runs the search once the active view finishes loading, catching matches in rows that streamed in after', async () => {
    vi.useFakeTimers()
    try {
      const log = useLogStore()
      await log.load('/repo')

      Facade.SearchLog.mockResolvedValueOnce({ indices: [], generation: 0, filtered: false })
      log.query = 'fix'
      await vi.advanceTimersByTimeAsync(250)
      await flushPromises()
      expect(log.matches).toEqual([])

      // more rows stream in under the same generation, one of them a match
      Facade.SearchLog.mockResolvedValueOnce({ indices: [3], generation: 0, filtered: false })
      emit('log:progress', { data: { loaded: 10, done: true, filtered: false, generation: 0 } })
      await flushPromises()

      expect(Facade.SearchLog).toHaveBeenCalledTimes(2)
      expect(log.matches).toEqual([3])
    } finally {
      vi.useRealTimers()
    }
  })

  it('FR-LOG-05 nextMatch and prevMatch wrap at the ends', async () => {
    vi.useFakeTimers()
    try {
      const log = useLogStore()
      await log.load('/repo')
      Facade.SearchLog.mockResolvedValueOnce({ indices: [1, 2, 3], generation: 0, filtered: false })

      log.query = 'fix'
      await vi.advanceTimersByTimeAsync(250)
      await flushPromises()
      log.nextMatch() // current: first -> second
      await flushPromises()
      log.nextMatch() // current: second -> last
      await flushPromises()
      expect(log.current).toBe(2)

      log.nextMatch() // wraps: last -> first
      await flushPromises()
      expect(log.current).toBe(0)

      log.prevMatch() // wraps: first -> last
      await flushPromises()
      expect(log.current).toBe(2)
    } finally {
      vi.useRealTimers()
    }
  })
})
