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
    const withWorktree = mount(MainView, { props: { path: '/repo' } })
    await flushPromises()

    expect(withoutWorktree.findComponent(DebugArea).exists()).toBe(true)
    expect(withWorktree.findComponent(DebugArea).exists()).toBe(true)
  })
})
