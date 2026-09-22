import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockSummary, mockEventsOn } = vi.hoisted(() => ({
  mockSummary: vi.fn(),
  mockEventsOn: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { Summary: mockSummary },
}))

vi.mock('@wailsio/runtime', () => ({
  Events: { On: mockEventsOn },
}))

import { useRepoStore } from './repo'

describe('repo store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockSummary.mockReset()
    mockSummary.mockResolvedValue({
      path: '/repo',
      branch: 'main',
      detached: false,
      operation: '',
    })
    mockEventsOn.mockReset()
    mockEventsOn.mockReturnValue(() => {})
  })

  it('FR-REPO-04 reloads Summary on a state or refs repo:changed event, not on index', async () => {
    const repo = useRepoStore()
    await repo.loadSummary('/repo')
    mockSummary.mockClear()

    const handler = mockEventsOn.mock.calls.find((call) => call[0] === 'repo:changed')?.[1]
    handler?.({ data: { scope: 'state' } })
    await Promise.resolve()
    handler?.({ data: { scope: 'refs' } })
    await Promise.resolve()

    expect(mockSummary).toHaveBeenCalledTimes(2)

    handler?.({ data: { scope: 'index' } })
    await Promise.resolve()

    expect(mockSummary).toHaveBeenCalledTimes(2)
  })

  it('ADR-4 reloads Summary on a core:reconnected event', async () => {
    const repo = useRepoStore()
    await repo.loadSummary('/repo')
    mockSummary.mockClear()

    const handler = mockEventsOn.mock.calls.find((call) => call[0] === 'core:reconnected')?.[1]
    handler?.({})
    await Promise.resolve()

    expect(mockSummary).toHaveBeenCalledTimes(1)
  })
})
