import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mountInBody } from '../testing/mountInBody'

const { AppService, Facade, SettingsService } = vi.hoisted(() => ({
  AppService: {
    RecentRepos: vi.fn(),
    UpdateWindowTitle: vi.fn().mockResolvedValue(undefined),
    ListWSLDistros: vi.fn().mockResolvedValue([]),
    LogPath: vi.fn().mockResolvedValue({ path: '/mock/nebegit.log', error: '' }),
  },
  Facade: {
    Summary: vi.fn(),
    LoadLog: vi.fn(),
    LogRows: vi.fn(),
    Refs: vi.fn(),
    UncommittedCount: vi.fn(),
    Jobs: vi.fn().mockResolvedValue([]),
    CancelJob: vi.fn(),
    Status: vi.fn(),
    SetMessageDraft: vi.fn().mockResolvedValue(undefined),
    Version: vi.fn().mockResolvedValue({ app: 'dev', protocol: 1 }),
    ListWorktrees: vi.fn().mockResolvedValue([]),
  },
  SettingsService: { Get: vi.fn(), Set: vi.fn().mockResolvedValue(undefined) },
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  AppService,
  Facade,
  SettingsService,
}))

import MainView from './MainView.vue'
import RepoPicker from '../components/RepoPicker.vue'
import WorktreeContent from './WorktreeContent.vue'
import { changedFilesFixture } from '../testing/fixtures/changes'
import { defaultSettingsFixture } from '../testing/fixtures/settings'
import { summaryFixture } from '../testing/fixtures/summary'

describe('MainView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.Summary.mockReset().mockResolvedValue(summaryFixture())
    Facade.LoadLog.mockReset().mockResolvedValue(undefined)
    Facade.Refs.mockReset().mockResolvedValue({ branches: [], remotes: [], tags: [] })
    AppService.RecentRepos.mockReset().mockResolvedValue([])
    Facade.Status.mockReset().mockResolvedValue(changedFilesFixture())
    SettingsService.Get.mockReset().mockResolvedValue(defaultSettingsFixture())
  })

  it('FR-WT-13 with no path, the repo list is the page, not WorktreeContent', async () => {
    AppService.RecentRepos.mockResolvedValue([
      { path: '/repoA', lastOpened: '2026-01-01T00:00:00Z' },
    ])
    const wrapper = mount(MainView, { props: { path: '' } })
    await flushPromises()

    expect(wrapper.findComponent(RepoPicker).props('asPage')).toBe(true)
    expect(wrapper.find('[data-test="repo-row"]').attributes('title')).toBe('/repoA')
    expect(wrapper.findComponent(WorktreeContent).exists()).toBe(false)
  })

  it('FR-UX-04 with no path, the gear opens the Settings dialog', async () => {
    const { wrapper, body } = await mountInBody(MainView, { props: { path: '' } })

    await wrapper.get('[data-test="open-settings"]').trigger('click')
    await flushPromises()

    expect(body.get('[role="dialog"] .p-dialog-title').text()).toBe('Settings')
    wrapper.unmount()
  })

  it('FR-WT-13 with a path, renders WorktreeContent, the repo picker in its toolbar', async () => {
    const wrapper = mount(MainView, { props: { path: '/repo' } })
    await flushPromises()

    expect(wrapper.findComponent(WorktreeContent).exists()).toBe(true)
    expect(wrapper.get('.toolbar').findComponent(RepoPicker).props('asPage')).toBe(false)
  })

  it('FR-REPO-07 forwards distro to WorktreeContent', async () => {
    const wrapper = mount(MainView, { props: { path: '/home/u/r', distro: 'Ubuntu' } })
    await flushPromises()

    expect(wrapper.findComponent(WorktreeContent).props('distro')).toBe('Ubuntu')
  })

  it("FR-UX-04 the toolbar's gear opens a dialog titled Settings with SettingsPanel inside, WorktreeContent still rendered", async () => {
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
