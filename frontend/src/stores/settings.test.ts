import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises } from '@vue/test-utils'

const { AppService, SettingsService } = vi.hoisted(() => ({
  AppService: { ListWSLDistros: vi.fn() },
  SettingsService: { Get: vi.fn(), Set: vi.fn() },
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  AppService,
  SettingsService,
}))

vi.mock('@wailsio/runtime', () => import('../testing/runtime'))
import { Events, emit } from '../testing/runtime'

import { useSettingsStore } from './settings'
import { defaultSettingsFixture } from '../testing/fixtures/settings'
import { Theme } from '../../bindings/github.com/brogli/nebegit/internal/settings'

describe('settings store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    AppService.ListWSLDistros.mockReset().mockResolvedValue(['Ubuntu-22.04'])
    SettingsService.Get.mockReset()
    Events.On.mockReset()
  })

  it('FR-UX-06 a settings change saved in any window reloads the settings here and applies its theme', async () => {
    SettingsService.Get.mockResolvedValue(defaultSettingsFixture({ theme: Theme.LightTheme }))
    const store = useSettingsStore()
    await store.load()
    expect(document.documentElement.classList.contains('nebegit-dark')).toBe(false)

    SettingsService.Get.mockResolvedValue(defaultSettingsFixture({ theme: Theme.DarkTheme }))
    emit('settings:changed')
    await flushPromises()

    expect(document.documentElement.classList.contains('nebegit-dark')).toBe(true)
  })

  it('FR-REPO-07 lists the WSL distros once and reuses the answer', async () => {
    const store = useSettingsStore()

    const first = await store.loadDistros()
    const second = await store.loadDistros()

    expect(AppService.ListWSLDistros).toHaveBeenCalledTimes(1)
    expect(second).toBe(first)
    expect(first).toEqual(['Ubuntu-22.04'])
  })
})
