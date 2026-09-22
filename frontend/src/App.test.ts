import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { mockCompareCommits, mockDiffRange, mockSettingsGet } = vi.hoisted(() => ({
  mockCompareCommits: vi.fn().mockResolvedValue([]),
  mockDiffRange: vi.fn(),
  mockSettingsGet: vi.fn(),
}))

vi.mock('../bindings/github.com/brogli/nebegit/internal/app', () => ({
  AppService: {
    PickAndOpen: vi.fn(),
    RecentRepos: vi.fn().mockResolvedValue([]),
    OpenRecent: vi.fn(),
  },
  Facade: { CompareCommits: mockCompareCommits, DiffRange: mockDiffRange },
  SettingsService: { Get: mockSettingsGet, Set: vi.fn() },
}))

vi.mock('@wailsio/runtime', () => ({
  Events: { On: vi.fn(() => () => {}) },
  Window: { Close: vi.fn() },
}))

import App from './App.vue'
import CompareDiffView from './views/CompareDiffView.vue'
import MainView from './views/MainView.vue'
import { defaultSettingsFixture } from './testing/fixtures/settings'

mockSettingsGet.mockResolvedValue(defaultSettingsFixture())

describe('App', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  afterEach(() => {
    window.history.pushState({}, '', '/')
  })

  it('QA-OSS-02 renders the app name with no worktree open', () => {
    expect(mount(App).text()).toContain('nebegit')
  })

  it('FR-CMP-02 routes ?view=compare-diff to CompareDiffView, not MainView', async () => {
    window.history.pushState(
      {},
      '',
      '?view=compare-diff&wt=/repo&from=sha-a&to=sha-b&file=a.txt&oldFile=old.txt',
    )

    const wrapper = mount(App)
    await flushPromises()

    expect(wrapper.findComponent(CompareDiffView).exists()).toBe(true)
    expect(wrapper.findComponent(MainView).exists()).toBe(false)
  })
})
