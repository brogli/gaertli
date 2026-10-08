import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { pressKey } from '@/testing/pressKey'
import { mountInBody } from '@/testing/mountInBody'

vi.mock(
  '../../../bindings/github.com/brogli/nebegit/internal/app',
  () => import('@/testing/mainWindowApp'),
)
import { AppService, SettingsService, resetMainWindowApp } from '@/testing/mainWindowApp'

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import MainView from '../MainView.vue'
import RepoPicker from '@/components/RepoPicker.vue'
import DebugArea from '@/components/DebugArea.vue'
import WorktreeContent from '../WorktreeContent.vue'
import { defaultSettingsFixture } from '@/testing/fixtures/settings'
import { useSettingsStore } from '@/stores/settings'

describe('MainView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    resetMainWindowApp()
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

  it('FR-WT-13 Ctrl+Shift+N opens a new window', async () => {
    mount(MainView, { props: { path: '' } })
    await flushPromises()

    pressKey('N', { ctrlKey: true, shiftKey: true })

    expect(AppService.NewWindow).toHaveBeenCalledTimes(1)
  })

  it.each([
    ['FR-UX-04 with no path, the gear opens the Settings dialog', ''],
    [
      'FR-UX-04 with a path, the toolbar gear opens the Settings dialog, WorktreeContent still rendered',
      '/repo',
    ],
  ])('%s', async (_name, path) => {
    const { wrapper, body } = await mountInBody(MainView, { props: { path } })

    await wrapper.get('[data-test="open-settings"]').trigger('click')
    await flushPromises()

    const dialog = body.get('[role="dialog"]')
    expect(dialog.get('.p-dialog-title').text()).toBe('Settings')
    expect(dialog.find('[data-test="app-version"]').exists()).toBe(true)
    expect(wrapper.findComponent(WorktreeContent).exists()).toBe(path !== '')
  })

  it('FR-WT-13, FR-REPO-07 with a path, renders WorktreeContent with the path and distro', async () => {
    const wrapper = mount(MainView, {
      props: { path: '/home/u/r', distro: 'Ubuntu' },
      global: { stubs: { WorktreeContent: true } },
    })
    await flushPromises()

    const content = wrapper.findComponent(WorktreeContent)
    expect(content.props()).toMatchObject({ path: '/home/u/r', distro: 'Ubuntu' })
  })

  it('FR-UX-19 with debug mode off (the default), no debug strip shows', async () => {
    const wrapper = mount(MainView, { props: { path: '' } })
    await flushPromises()

    expect(wrapper.findComponent(DebugArea).exists()).toBe(false)
  })

  it('FR-UX-19 with debug mode on, the debug strip shows along the bottom, worktree or not', async () => {
    SettingsService.Get.mockResolvedValue(defaultSettingsFixture({ debugMode: true }))
    // MainView reads the settings store rather than loading it itself: a
    // real window's App.vue loads it once for every view.
    await useSettingsStore().load()

    const withoutWorktree = mount(MainView, { props: { path: '' } })
    const withWorktree = mount(MainView, {
      props: { path: '/repo' },
      global: { stubs: { WorktreeContent: true } },
    })
    await flushPromises()

    expect(withoutWorktree.findComponent(DebugArea).exists()).toBe(true)
    expect(withWorktree.findComponent(DebugArea).exists()).toBe(true)
  })
})
