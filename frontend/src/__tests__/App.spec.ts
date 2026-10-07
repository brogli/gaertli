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
    OpenFile: vi.fn(),
  },
  SettingsService: { Get: vi.fn(), Set: vi.fn() },
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  AppService,
  Facade,
  SettingsService,
}))

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import App from '../App.vue'
import DiffWindowView from '@/views/DiffWindowView.vue'
import MainView from '@/views/MainView.vue'
import { useSettingsStore } from '@/stores/settings'
import { defaultSettingsFixture } from '@/testing/fixtures/settings'
import { mountInBody } from '@/testing/mountInBody'
import { Theme } from '../../bindings/github.com/brogli/nebegit/internal/settings'

SettingsService.Get.mockResolvedValue(defaultSettingsFixture())

describe('App', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  afterEach(() => {
    window.history.pushState({}, '', '/')
    document.documentElement.classList.remove('nebegit-dark')
  })

  it('OSS-02 renders the app name with no worktree open', () => {
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
        conflicted: false,
        path: 'a.txt',
        oldPath: 'old.txt',
      },
    })
    expect(wrapper.findComponent(MainView).exists()).toBe(false)
  })

  it("FR-UX-01, FR-ED-06 the diff window's own notice area shows an open-editor failure from its file list", async () => {
    window.history.pushState(
      {},
      '',
      '?view=diff&wt=/repo&kind=range&from=sha-a&to=sha-b&file=a.txt&oldFile=',
    )
    Facade.CompareCommits.mockResolvedValueOnce([{ path: 'a.txt', status: 'M' }])
    Facade.OpenFile.mockRejectedValueOnce(
      new Error('boom', { cause: { code: 'EDITOR_NOT_FOUND', message: '"myed" is not on PATH' } }),
    )

    const { body } = await mountInBody(App)
    await body.get('[data-test="open-editor"]').trigger('click')
    await flushPromises()

    expect(body.get('.p-toast-message').text()).toContain('myed')
  })

  it('FR-UX-06 follows the OS by default, then applies a forced theme once settings change', async () => {
    const matchMediaSpy = vi.spyOn(window, 'matchMedia').mockImplementation((query: string) => {
      const fake = {
        matches: query.includes('dark'),
        media: query,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      }
      // eslint-disable-next-line @typescript-eslint/consistent-type-assertions -- test double stands in for MediaQueryList, which it does not fully implement
      return fake as unknown as MediaQueryList
    })
    SettingsService.Get.mockResolvedValueOnce(defaultSettingsFixture({ theme: Theme.SystemTheme }))

    const wrapper = mount(App)
    await flushPromises()

    expect(document.documentElement.classList.contains('nebegit-dark')).toBe(true)

    useSettingsStore().settings!.theme = Theme.LightTheme
    await flushPromises()

    expect(document.documentElement.classList.contains('nebegit-dark')).toBe(false)

    wrapper.unmount()
    matchMediaSpy.mockRestore()
  })
})
