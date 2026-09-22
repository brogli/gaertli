import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockRefs, mockEventsOn } = vi.hoisted(() => ({
  mockRefs: vi.fn(),
  mockEventsOn: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { Refs: mockRefs },
}))

vi.mock('@wailsio/runtime', () => ({
  Events: { On: mockEventsOn },
}))

import { useRefsStore } from './refs'

describe('refs store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockRefs.mockReset()
    mockRefs.mockResolvedValue({ branches: [], remotes: [], tags: [] })
    mockEventsOn.mockReset()
    mockEventsOn.mockReturnValue(() => {})
  })

  it('FR-LOG-01 reloads Refs on a refs repo:changed event, not on state', async () => {
    const refs = useRefsStore()
    await refs.load('/repo')
    expect(mockRefs).toHaveBeenCalledTimes(1)

    const handler = mockEventsOn.mock.calls.find((call) => call[0] === 'repo:changed')?.[1]
    handler?.({ data: { scope: 'refs' } })
    await Promise.resolve()
    handler?.({ data: { scope: 'state' } })
    await Promise.resolve()

    expect(mockRefs).toHaveBeenCalledTimes(2)
  })

  it('ADR-4 reloads Refs on a core:reconnected event', async () => {
    const refs = useRefsStore()
    await refs.load('/repo')
    expect(mockRefs).toHaveBeenCalledTimes(1)

    const handler = mockEventsOn.mock.calls.find((call) => call[0] === 'core:reconnected')?.[1]
    handler?.({})
    await Promise.resolve()

    expect(mockRefs).toHaveBeenCalledTimes(2)
  })
})
