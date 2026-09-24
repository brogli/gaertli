import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { AppService, Facade, SettingsService } = vi.hoisted(() => ({
  AppService: {
    PickAndOpen: vi.fn(),
    RecentRepos: vi.fn().mockResolvedValue([]),
    OpenPath: vi.fn(),
  },
  Facade: {
    CompareCommits: vi.fn().mockResolvedValue([]),
    DiffRange: vi.fn().mockResolvedValue(null),
  },
  SettingsService: { Get: vi.fn(), Set: vi.fn() },
}))

vi.mock('../bindings/github.com/brogli/nebegit/internal/app', () => ({
  AppService,
  Facade,
  SettingsService,
}))

vi.mock('@wailsio/runtime', () => import('./testing/runtime'))

import App from './App.vue'
import DiffWindowView from './views/DiffWindowView.vue'
import MainView from './views/MainView.vue'
import { useSettingsStore } from './stores/settings'
import { defaultSettingsFixture } from './testing/fixtures/settings'

SettingsService.Get.mockResolvedValue(defaultSettingsFixture())

describe('App', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  afterEach(() => {
    window.history.pushState({}, '', '/')
    document.documentElement.classList.remove('nebegit-dark')
  })

  it('QA-OSS-02 renders the app name with no worktree open', () => {
    expect(mount(App).text()).toContain('nebegit')
  })

  it('FR-DIFF-07 routes ?view=diff to DiffWindowView with its worktree and target, not MainView', async () => {
    window.history.pushState(
      {},
      '',
      '?view=diff&wt=/repo&kind=range&from=sha-a&to=sha-b&file=a.txt&oldFile=old.txt',
    )

    const wrapper = mount(App)
    await flushPromises()

    expect(wrapper.findComponent(DiffWindowView).props()).toEqual({
      wt: '/repo',
      target: {
        kind: 'range',
        sha: '',
        from: 'sha-a',
        to: 'sha-b',
        staged: false,
        path: 'a.txt',
        oldPath: 'old.txt',
      },
    })
    expect(wrapper.findComponent(MainView).exists()).toBe(false)
  })

  it('FR-UX-06 follows the OS by default, then applies a forced theme once settings change', async () => {
    const matchMediaSpy = vi.spyOn(window, 'matchMedia').mockImplementation(
      (query: string) =>
        ({
          matches: query.includes('dark'),
          media: query,
          addEventListener: vi.fn(),
          removeEventListener: vi.fn(),
        }) as unknown as MediaQueryList,
    )
    SettingsService.Get.mockResolvedValueOnce(defaultSettingsFixture({ theme: '' }))

    const wrapper = mount(App)
    await flushPromises()

    expect(document.documentElement.classList.contains('nebegit-dark')).toBe(true)

    useSettingsStore().settings!.theme = 'light'
    await flushPromises()

    expect(document.documentElement.classList.contains('nebegit-dark')).toBe(false)

    wrapper.unmount()
    matchMediaSpy.mockRestore()
  })
})
