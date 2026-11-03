// Requirements: FR-SYNC-10
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade } = vi.hoisted(() => ({ Facade: { Fetch: vi.fn() } }))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))
vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import { useAutoFetch } from '../useAutoFetch'
import { useSettingsStore } from '@/stores/settings'
import { defaultSettingsFixture } from '@/testing/fixtures/settings'

describe('useAutoFetch', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    setActivePinia(createPinia())
    Facade.Fetch.mockReset().mockResolvedValue('job-1')
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  it('with autoFetchIntervalMinutes 0, advancing 30 minutes never calls Facade.Fetch', async () => {
    const settings = useSettingsStore()
    settings.settings = defaultSettingsFixture({ autoFetchIntervalMinutes: 0 })

    useAutoFetch(() => '/repo')
    await vi.advanceTimersByTimeAsync(30 * 60_000)

    expect(Facade.Fetch).not.toHaveBeenCalled()
  })

  it('with autoFetchIntervalMinutes 5, Facade.Fetch fires once per 5-minute advance', async () => {
    const settings = useSettingsStore()
    settings.settings = defaultSettingsFixture({ autoFetchIntervalMinutes: 5 })

    useAutoFetch(() => '/repo')
    await vi.advanceTimersByTimeAsync(5 * 60_000)
    expect(Facade.Fetch).toHaveBeenCalledTimes(1)
    expect(Facade.Fetch).toHaveBeenCalledWith('/repo', '')

    await vi.advanceTimersByTimeAsync(5 * 60_000)
    expect(Facade.Fetch).toHaveBeenCalledTimes(2)
  })

  it('the interval changing to 0 while running stops further fetches', async () => {
    const settings = useSettingsStore()
    settings.settings = defaultSettingsFixture({ autoFetchIntervalMinutes: 5 })

    useAutoFetch(() => '/repo')
    await vi.advanceTimersByTimeAsync(5 * 60_000)
    expect(Facade.Fetch).toHaveBeenCalledTimes(1)

    settings.settings = defaultSettingsFixture({ autoFetchIntervalMinutes: 0 })
    await vi.advanceTimersByTimeAsync(30 * 60_000)

    expect(Facade.Fetch).toHaveBeenCalledTimes(1)
  })
})
