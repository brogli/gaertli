import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockSummary, mockLoadLog, mockRefs, mockRecentRepos, mockCommandLog } = vi.hoisted(() => ({
  mockSummary: vi.fn(),
  mockLoadLog: vi.fn(),
  mockRefs: vi.fn(),
  mockRecentRepos: vi.fn(),
  mockCommandLog: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  AppService: { RecentRepos: mockRecentRepos, UpdateWindowTitle: vi.fn().mockResolvedValue(undefined) },
  Facade: {
    Summary: mockSummary,
    LoadLog: mockLoadLog,
    LogRows: vi.fn(),
    Refs: mockRefs,
    CommandLog: mockCommandLog,
    UncommittedCount: vi.fn(),
  },
}))

import MainView from './MainView.vue'
import SelectorPanel from '../components/SelectorPanel.vue'
import WorktreeContent from './WorktreeContent.vue'

describe('MainView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockSummary.mockReset().mockResolvedValue({ path: '/repo', branch: 'main', detached: false, operation: '' })
    mockLoadLog.mockReset().mockResolvedValue(undefined)
    mockRefs.mockReset().mockResolvedValue({ branches: [], remotes: [], tags: [] })
    mockRecentRepos.mockReset().mockResolvedValue([])
    mockCommandLog.mockReset().mockResolvedValue([])
  })

  it('FR-WT-13 with no path, renders the panel and the empty state, not WorktreeContent', async () => {
    const wrapper = mount(MainView, { props: { path: '' } })
    await flushPromises()

    expect(wrapper.findComponent(SelectorPanel).exists()).toBe(true)
    expect(wrapper.findComponent(WorktreeContent).exists()).toBe(false)
    expect(wrapper.text()).toContain('Select a worktree from the panel, or open a folder to get started.')
  })

  it('FR-WT-13 with a path, renders both the panel and WorktreeContent', async () => {
    const wrapper = mount(MainView, { props: { path: '/repo' } })
    await flushPromises()

    expect(wrapper.findComponent(SelectorPanel).exists()).toBe(true)
    expect(wrapper.findComponent(WorktreeContent).exists()).toBe(true)
  })
})
