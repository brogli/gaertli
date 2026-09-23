import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mountInBody } from '../testing/mountInBody'

const { mockSummary, mockLoadLog, mockRefs, mockRecentRepos, mockCommandLog, mockStatus, mockSettingsGet } =
  vi.hoisted(() => ({
    mockSummary: vi.fn(),
    mockLoadLog: vi.fn(),
    mockRefs: vi.fn(),
    mockRecentRepos: vi.fn(),
    mockCommandLog: vi.fn(),
    mockStatus: vi.fn(),
    mockSettingsGet: vi.fn(),
  }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  AppService: {
    RecentRepos: mockRecentRepos,
    UpdateWindowTitle: vi.fn().mockResolvedValue(undefined),
    ListWSLDistros: vi.fn().mockResolvedValue([]),
  },
  Facade: {
    Summary: mockSummary,
    LoadLog: mockLoadLog,
    LogRows: vi.fn(),
    Refs: mockRefs,
    CommandLog: mockCommandLog,
    UncommittedCount: vi.fn(),
    Jobs: vi.fn().mockResolvedValue([]),
    CancelJob: vi.fn(),
    Status: mockStatus,
    SetMessageDraft: vi.fn().mockResolvedValue(undefined),
    Version: vi.fn().mockResolvedValue({ app: 'dev', protocol: 1 }),
  },
  SettingsService: { Get: mockSettingsGet, Set: vi.fn().mockResolvedValue(undefined) },
}))

import MainView from './MainView.vue'
import SelectorPanel from '../components/SelectorPanel.vue'
import WorktreeContent from './WorktreeContent.vue'
import { changedFilesFixture } from '../testing/fixtures/changes'
import { defaultSettingsFixture } from '../testing/fixtures/settings'
import { summaryFixture } from '../testing/fixtures/summary'

describe('MainView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockSummary.mockReset().mockResolvedValue(summaryFixture({ path: '/repo' }))
    mockLoadLog.mockReset().mockResolvedValue(undefined)
    mockRefs.mockReset().mockResolvedValue({ branches: [], remotes: [], tags: [] })
    mockRecentRepos.mockReset().mockResolvedValue([])
    mockCommandLog.mockReset().mockResolvedValue([])
    mockStatus.mockReset().mockResolvedValue(changedFilesFixture())
    mockSettingsGet.mockReset().mockResolvedValue(defaultSettingsFixture())
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

  it('FR-REPO-07 forwards distro to WorktreeContent', async () => {
    const wrapper = mount(MainView, { props: { path: '/home/u/r', distro: 'Ubuntu' } })
    await flushPromises()

    expect(wrapper.findComponent(WorktreeContent).props('distro')).toBe('Ubuntu')
  })

  it('FR-UX-04 clicking Settings opens a dialog titled Settings with SettingsPanel inside, WorktreeContent still rendered', async () => {
    const { wrapper, body } = await mountInBody(MainView, { props: { path: '/repo' } })

    await wrapper.get('[data-test="open-settings"]').trigger('click')
    await flushPromises()

    const dialog = body.get('[role="dialog"]')
    expect(dialog.find('.p-dialog-title').text()).toBe('Settings')
    expect(dialog.find('[data-test="app-version"]').exists()).toBe(true)
    expect(wrapper.findComponent(WorktreeContent).exists()).toBe(true)

    wrapper.unmount()
  })

  it("FR-UX-04 clicking the dialog's own close button closes it, WorktreeContent still rendered", async () => {
    const { wrapper, body } = await mountInBody(MainView, { props: { path: '/repo' } })
    await wrapper.get('[data-test="open-settings"]').trigger('click')
    await flushPromises()

    await body.get('.p-dialog-close-button').trigger('click')
    await new Promise((resolve) => requestAnimationFrame(resolve))
    await flushPromises()

    expect(document.querySelector('[role="dialog"]')).toBeNull()
    expect(wrapper.findComponent(WorktreeContent).exists()).toBe(true)

    wrapper.unmount()
  })
})
