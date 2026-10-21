// Requirements: FR-REPO-07, FR-UX-06
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'

const { AppService, SettingsService } = vi.hoisted(() => ({
  AppService: { ListWSLDistros: vi.fn() },
  SettingsService: { Get: vi.fn(), Set: vi.fn() },
}))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  AppService,
  SettingsService,
}))

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))
import { Events, emit } from '@/testing/runtime'

import { useSettingsStore } from '../settings'
import { defaultSettingsFixture } from '@/testing/fixtures/settings'
import { Theme } from '../../../bindings/github.com/brogli/nebegit/internal/settings'

describe('settings store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    AppService.ListWSLDistros.mockReset().mockResolvedValue(['Ubuntu-22.04'])
    SettingsService.Get.mockReset()
    Events.On.mockReset()
  })

  it('a settings change saved in any window reloads the settings here and applies its theme', async () => {
    SettingsService.Get.mockResolvedValue(defaultSettingsFixture({ theme: Theme.LightTheme }))
    const store = useSettingsStore()
    await store.load()
    expect(document.documentElement.classList.contains('nebegit-dark')).toBe(false)

    SettingsService.Get.mockResolvedValue(defaultSettingsFixture({ theme: Theme.DarkTheme }))
    emit('settings:changed')
    await flushPromises()

    expect(document.documentElement.classList.contains('nebegit-dark')).toBe(true)
  })

  it('with System, the theme follows the OS setting as it changes', async () => {
    let handler: ((event: { matches: boolean }) => void) | undefined
    const matchMediaSpy = vi.spyOn(window, 'matchMedia').mockImplementation((query: string) => {
      const fake = {
        matches: false,
        media: query,
        addEventListener: (_event: string, cb: (event: { matches: boolean }) => void) => {
          handler = cb
        },
        removeEventListener: vi.fn(),
      }
      // eslint-disable-next-line @typescript-eslint/consistent-type-assertions -- test double stands in for MediaQueryList, which it does not fully implement
      return fake as unknown as MediaQueryList
    })

    SettingsService.Get.mockResolvedValue(defaultSettingsFixture({ theme: Theme.SystemTheme }))
    const store = useSettingsStore()
    await store.load()
    await flushPromises()

    handler!({ matches: true })
    await nextTick()
    expect(document.documentElement.classList.contains('nebegit-dark')).toBe(true)

    handler!({ matches: false })
    await nextTick()
    expect(document.documentElement.classList.contains('nebegit-dark')).toBe(false)

    matchMediaSpy.mockRestore()
  })

  it('lists the WSL distros once and reuses the answer', async () => {
    const store = useSettingsStore()

    const first = await store.loadDistros()
    const second = await store.loadDistros()

    expect(AppService.ListWSLDistros).toHaveBeenCalledTimes(1)
    expect(second).toBe(first)
    expect(first).toEqual(['Ubuntu-22.04'])
  })
})
