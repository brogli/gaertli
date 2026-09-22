import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockLoadLog, mockLogRows, mockReloadLog, mockLogIndexOf, mockEventsOn } = vi.hoisted(
  () => ({
    mockLoadLog: vi.fn(),
    mockLogRows: vi.fn(),
    mockReloadLog: vi.fn(),
    mockLogIndexOf: vi.fn(),
    mockEventsOn: vi.fn(),
  }),
)

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: {
    LoadLog: mockLoadLog,
    LogRows: mockLogRows,
    ReloadLog: mockReloadLog,
    CommitDetails: vi.fn(),
    LogIndexOf: mockLogIndexOf,
    SetLogFilter: vi.fn().mockResolvedValue(undefined),
  },
}))

vi.mock('@wailsio/runtime', () => ({
  Events: { On: mockEventsOn },
}))

import { useLogStore } from './log'

function rowFixture(sha: string) {
  return {
    sha,
    shortSha: sha,
    author: 'Alice',
    authorAt: 0,
    subject: 'x',
    graph: { lane: 0, colour: 0, edges: [] },
  }
}

describe('log store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockLoadLog.mockReset()
    mockLoadLog.mockResolvedValue(undefined)
    mockLogRows.mockReset()
    mockLogRows.mockResolvedValue({
      rows: [rowFixture('a')],
      loaded: 1,
      done: false,
      generation: 0,
    })
    mockReloadLog.mockReset()
    mockReloadLog.mockResolvedValue(undefined)
    mockLogIndexOf.mockReset()
    mockEventsOn.mockReset()
    mockEventsOn.mockReturnValue(() => {})
  })

  function changedHandler() {
    return mockEventsOn.mock.calls.find((call) => call[0] === 'repo:changed')?.[1]
  }

  it('FR-LOG-01 rows(from, to) caches its window: a repeated request does not call LogRows again', async () => {
    const log = useLogStore()
    await log.load('/repo')

    await log.rows(0, 1)
    await log.rows(0, 1)

    expect(mockLogRows).toHaveBeenCalledTimes(1)
  })

  it('rows(from, to) dedupes an in-flight request for the same window', async () => {
    const log = useLogStore()
    await log.load('/repo')

    let resolve!: (value: unknown) => void
    mockLogRows.mockReturnValueOnce(new Promise((r) => (resolve = r)))

    const first = log.rows(0, 1) // not yet cached: fetches
    const second = log.rows(0, 1) // requested again before the first resolves

    resolve({ rows: [rowFixture('a')], loaded: 1, done: false, generation: 0 })
    await Promise.all([first, second])

    expect(mockLogRows).toHaveBeenCalledTimes(1)
  })

  it('FR-LOG-09 done becomes true once a log:progress event with done: true arrives', async () => {
    const log = useLogStore()
    await log.load('/repo')
    expect(log.done).toBe(false)

    const handler = mockEventsOn.mock.calls.find((call) => call[0] === 'log:progress')?.[1]
    handler?.({ data: { loaded: 5, done: true } })

    expect(log.done).toBe(true)
    expect(log.loaded).toBe(5)
  })

  it('FR-REPO-04 calls ReloadLog on a refs repo:changed event, not on state', async () => {
    const log = useLogStore()
    await log.load('/repo')

    const handler = changedHandler()
    handler?.({ data: { scope: 'refs' } })
    await Promise.resolve()
    handler?.({ data: { scope: 'state' } })
    await Promise.resolve()

    expect(mockReloadLog).toHaveBeenCalledTimes(1)
    expect(mockReloadLog).toHaveBeenCalledWith('/repo')
  })

  it('ADR-4 calls LoadLog again on a core:reconnected event', async () => {
    const log = useLogStore()
    await log.load('/repo')
    mockLoadLog.mockClear()

    const handler = mockEventsOn.mock.calls.find((call) => call[0] === 'core:reconnected')?.[1]
    handler?.({})
    await Promise.resolve()

    expect(mockLoadLog).toHaveBeenCalledTimes(1)
    expect(mockLoadLog).toHaveBeenCalledWith('/repo')
  })

  it('FR-REPO-04 drops the row cache when Generation changes', async () => {
    const log = useLogStore()
    await log.load('/repo')

    mockLogRows.mockResolvedValueOnce({
      rows: [rowFixture('a'), rowFixture('b')],
      loaded: 2,
      done: true,
      generation: 0,
    })
    await log.rows(0, 2)
    expect(log.rowCache.size).toBe(2)

    mockLogRows.mockResolvedValueOnce({
      rows: [rowFixture('c')],
      loaded: 1,
      done: true,
      generation: 1,
    })
    await log.rows(5, 6) // a range not yet cached, so this actually fetches

    expect(log.rowCache.size).toBe(1)
    expect(log.rowCache.get(5)?.sha).toBe('c')
    expect(log.rowCache.has(0)).toBe(false)
  })

  it('FR-LOG-01 jumpTo selects the row and sets pendingScrollTo when the sha is found', async () => {
    const log = useLogStore()
    await log.load('/repo')
    mockLogIndexOf.mockResolvedValue({ index: 7, found: true })
    mockLogRows.mockResolvedValue({
      rows: [rowFixture('deadbeef')],
      loaded: 8,
      done: false,
      generation: 0,
    })

    await log.jumpTo('deadbeef')

    expect(mockLogRows).toHaveBeenCalledWith('/repo', 7, 8)
    expect(log.selectedRow?.sha).toBe('deadbeef')
    expect(log.pendingScrollTo).toBe(7)
  })

  it('FR-LOG-01 jumpTo is a no-op when the sha is not found', async () => {
    const log = useLogStore()
    await log.load('/repo')
    mockLogIndexOf.mockResolvedValue({ index: 7, found: true })
    mockLogRows.mockResolvedValue({
      rows: [rowFixture('abc')],
      loaded: 8,
      done: false,
      generation: 0,
    })
    await log.jumpTo('abc')
    expect(log.selectedRow?.sha).toBe('abc')

    mockLogIndexOf.mockResolvedValue({ index: 0, found: false })
    await log.jumpTo('missing')

    expect(log.selectedRow?.sha).toBe('abc')
    expect(log.pendingScrollTo).toBe(7)
  })

  it('FR-LOG-05 knownAuthors dedupes and sorts every author seen in fetched rows', async () => {
    const log = useLogStore()
    await log.load('/repo')
    mockLogRows.mockResolvedValueOnce({
      rows: [
        { ...rowFixture('a'), author: 'Bob' },
        { ...rowFixture('b'), author: 'Alice' },
        { ...rowFixture('c'), author: 'Bob' },
      ],
      loaded: 3,
      done: true,
      generation: 0,
    })

    await log.rows(0, 3)

    expect(log.knownAuthors).toEqual(['Alice', 'Bob'])
  })

  it('FR-LOG-08 clearing a filter shows the base rows already fetched without refetching', async () => {
    const log = useLogStore()
    await log.load('/repo')
    await log.rows(0, 1)

    await log.applyFilter({ query: 'x' })
    await log.rows(0, 1)
    await log.clearFilter()
    await log.rows(0, 1)

    expect(mockLogRows).toHaveBeenCalledTimes(2)
  })
})
