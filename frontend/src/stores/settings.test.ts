import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const { mockListWSLDistros } = vi.hoisted(() => ({ mockListWSLDistros: vi.fn() }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  AppService: { ListWSLDistros: mockListWSLDistros },
  SettingsService: { Get: vi.fn(), Set: vi.fn() },
}))

import { useSettingsStore } from './settings'

describe('settings store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockListWSLDistros.mockReset()
    mockListWSLDistros.mockResolvedValue([{ name: 'Ubuntu-22.04', version: 2, running: true }])
  })

  it('FR-REPO-07 lists the WSL distros once and reuses the answer', async () => {
    const store = useSettingsStore()

    const first = await store.loadDistros()
    const second = await store.loadDistros()

    expect(mockListWSLDistros).toHaveBeenCalledTimes(1)
    expect(second).toBe(first)
    expect(first.map((d) => d.name)).toEqual(['Ubuntu-22.04'])
  })
})
