import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { mockFetch } = vi.hoisted(() => ({ mockFetch: vi.fn() }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { Fetch: mockFetch },
}))

import { useAutoFetch } from './useAutoFetch'
import { useSettingsStore } from '../stores/settings'
import { defaultSettingsFixture } from '../testing/fixtures/settings'

describe('useAutoFetch', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    setActivePinia(createPinia())
    mockFetch.mockReset().mockResolvedValue('job-1')
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('FR-SYNC-10 with autoFetchIntervalMinutes 0, advancing 30 minutes never calls Facade.Fetch', async () => {
    const settings = useSettingsStore()
    settings.settings = defaultSettingsFixture({ autoFetchIntervalMinutes: 0 })

    useAutoFetch(() => '/repo')
    await vi.advanceTimersByTimeAsync(30 * 60_000)

    expect(mockFetch).not.toHaveBeenCalled()
  })

  it('FR-SYNC-10 with autoFetchIntervalMinutes 5, Facade.Fetch fires once per 5-minute advance', async () => {
    const settings = useSettingsStore()
    settings.settings = defaultSettingsFixture({ autoFetchIntervalMinutes: 5 })

    useAutoFetch(() => '/repo')
    await vi.advanceTimersByTimeAsync(5 * 60_000)
    expect(mockFetch).toHaveBeenCalledTimes(1)
    expect(mockFetch).toHaveBeenCalledWith('/repo', '')

    await vi.advanceTimersByTimeAsync(5 * 60_000)
    expect(mockFetch).toHaveBeenCalledTimes(2)
  })

  it('FR-SYNC-10 the interval changing to 0 while running stops further fetches', async () => {
    const settings = useSettingsStore()
    settings.settings = defaultSettingsFixture({ autoFetchIntervalMinutes: 5 })

    useAutoFetch(() => '/repo')
    await vi.advanceTimersByTimeAsync(5 * 60_000)
    expect(mockFetch).toHaveBeenCalledTimes(1)

    settings.settings = defaultSettingsFixture({ autoFetchIntervalMinutes: 0 })
    await vi.advanceTimersByTimeAsync(30 * 60_000)

    expect(mockFetch).toHaveBeenCalledTimes(1)
  })
})
