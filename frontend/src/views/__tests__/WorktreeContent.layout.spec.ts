import { flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

// WorktreeContent's own three layouts: which of the log view,
// the commit detail view and the changes view show at a given window
// width, and the switch between them below wide.

vi.mock(
  '../../../bindings/github.com/brogli/nebegit/internal/app',
  () => import('@/testing/mainWindowApp'),
)
vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))
import { Facade, resetMainWindowApp } from '@/testing/mainWindowApp'

import WorktreeContent from '../WorktreeContent.vue'
import LogView from '@/components/LogView.vue'
import CommitDetailView from '@/components/CommitDetailView.vue'
import ChangesView from '@/components/ChangesView.vue'
import BranchPicker from '@/components/BranchPicker.vue'
import { useLogStore } from '@/stores/log'
import { commitRow } from '@/testing/fixtures/log-rows'
import { findButton } from '@/testing/findButton'
import { mountInBody } from '@/testing/mountInBody'
import { changedFilesFixture } from '@/testing/fixtures/changes'
import { summaryFixture } from '@/testing/fixtures/summary'

const DEFAULT_WIDTH = 1024

// mountAt sets the window width before mounting: happy-dom's
// MediaQueryList only tracks changes correctly from the width current when
// its listener was added, so every layout test starts fresh
// at its target width rather than resizing an existing mount. Mounted in
// document.body: happy-dom's getComputedStyle (isVisible's v-show check)
// only resolves an element's style once it is connected to a document.
async function mountAt(width: number): Promise<VueWrapper> {
  window.innerWidth = width
  const { wrapper } = await mountInBody(WorktreeContent, { props: { path: '/repo' } })
  return wrapper
}

// switchTo clicks the switch option labelled label (e.g. "Log", "Details",
// "Changes", or "Changes (4)").
async function switchTo(wrapper: VueWrapper, label: string): Promise<void> {
  const button = wrapper.findAll('.p-togglebutton').find((b) => b.text() === label)
  await button!.trigger('click')
}

// clickRow selects a commit so the commit detail view has content to show
// or hide; without one it renders nothing, which would make its
// visibility checks pass by accident either way.
async function clickRow(wrapper: VueWrapper): Promise<void> {
  const log = useLogStore()
  log.rowCache.set(0, commitRow())
  log.loaded = 1
  await wrapper.vm.$nextTick()
  await wrapper.get('.row').trigger('click')
  await flushPromises()
}

describe('WorktreeContent layouts', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    resetMainWindowApp()
  })

  afterEach(() => {
    window.innerWidth = DEFAULT_WIDTH
  })

  it('FR-UX-08 wide (1600): all three views show side by side, no switch', async () => {
    const wrapper = await mountAt(1600)
    await clickRow(wrapper)

    expect(wrapper.findComponent(LogView).isVisible()).toBe(true)
    expect(wrapper.findComponent(CommitDetailView).isVisible()).toBe(true)
    expect(wrapper.findComponent(ChangesView).isVisible()).toBe(true)
    expect(wrapper.find('.p-selectbutton').exists()).toBe(false)
  })

  it('FR-UX-08 medium (1200): the switch starts on Details, before any commit is selected', async () => {
    const wrapper = await mountAt(1200)

    const details = wrapper.findAll('.p-togglebutton').find((b) => b.text() === 'Details')!
    expect(details.attributes('aria-pressed')).toBe('true')
  })

  it('FR-UX-08 narrow (600): the switch starts on Log, before any commit is selected', async () => {
    const wrapper = await mountAt(600)

    expect(wrapper.findComponent(LogView).isVisible()).toBe(true)
    const log = wrapper.findAll('.p-togglebutton').find((b) => b.text() === 'Log')!
    expect(log.attributes('aria-pressed')).toBe('true')
  })

  it('FR-UX-08 medium (1200): the log shows; the switch defaults to Details', async () => {
    const wrapper = await mountAt(1200)
    await clickRow(wrapper)

    expect(wrapper.findComponent(LogView).isVisible()).toBe(true)
    expect(wrapper.findComponent(CommitDetailView).isVisible()).toBe(true)
    expect(wrapper.findComponent(ChangesView).isVisible()).toBe(false)
    expect(wrapper.findAll('.p-togglebutton').map((b) => b.text())).toEqual(['Details', 'Changes'])
  })

  it('FR-UX-08 medium (1200): clicking a commit switches the side panel back to Details', async () => {
    const wrapper = await mountAt(1200)
    await switchTo(wrapper, 'Changes')
    expect(wrapper.findComponent(ChangesView).isVisible()).toBe(true)

    await clickRow(wrapper)

    expect(wrapper.findComponent(CommitDetailView).isVisible()).toBe(true)
    expect(wrapper.findComponent(ChangesView).isVisible()).toBe(false)
  })

  it('FR-UX-08 narrow (600): only the log view shows, switch [Log, Details, Changes]', async () => {
    const wrapper = await mountAt(600)
    await clickRow(wrapper)

    expect(wrapper.findComponent(LogView).isVisible()).toBe(true)
    expect(wrapper.findComponent(CommitDetailView).isVisible()).toBe(false)
    expect(wrapper.findComponent(ChangesView).isVisible()).toBe(false)
    expect(wrapper.findAll('.p-togglebutton').map((b) => b.text())).toEqual([
      'Log',
      'Details',
      'Changes',
    ])
  })

  it('FR-UX-08 narrow (600): a click keeps the log view; a double-click switches to Details', async () => {
    const wrapper = await mountAt(600)
    await clickRow(wrapper)
    expect(wrapper.findComponent(LogView).isVisible()).toBe(true)

    await wrapper.get('.row').trigger('dblclick')
    await flushPromises()
    expect(wrapper.findComponent(CommitDetailView).isVisible()).toBe(true)
  })

  it("FR-LOG-06 narrow (600): the branch picker's show-log brings the log view forward", async () => {
    const wrapper = await mountAt(600)
    await switchTo(wrapper, 'Changes')
    expect(wrapper.findComponent(LogView).isVisible()).toBe(false)

    wrapper.findComponent(BranchPicker).vm.$emit('show-log')
    await flushPromises()

    expect(wrapper.findComponent(LogView).isVisible()).toBe(true)
  })

  it("FR-LOG-06 medium (1200): the branch picker's show-log leaves the side panel as it was", async () => {
    const wrapper = await mountAt(1200)
    await switchTo(wrapper, 'Changes')

    wrapper.findComponent(BranchPicker).vm.$emit('show-log')
    await flushPromises()

    expect(wrapper.findComponent(LogView).isVisible()).toBe(true)
    expect(wrapper.findComponent(ChangesView).isVisible()).toBe(true)
  })

  it('FR-UX-08 medium (1200): the switch reads "Changes (4)" with 3 unstaged and 1 untracked', async () => {
    Facade.Status.mockResolvedValue(
      changedFilesFixture({
        unstaged: [
          { path: 'a', status: 'M' },
          { path: 'b', status: 'M' },
          { path: 'c', status: 'M' },
        ],
        untracked: [{ path: 'd', status: '?' }],
      }),
    )
    const wrapper = await mountAt(1200)

    expect(wrapper.findAll('.p-togglebutton').map((b) => b.text())).toContain('Changes (4)')
  })

  it('FR-UX-09 medium (1200): Fetch, Refresh and Open in editor show icon only, with title and aria-label', async () => {
    Facade.Refs.mockResolvedValue({
      branches: [],
      remotes: [{ name: 'origin', branches: [] }],
      tags: [],
    })
    const wrapper = await mountAt(1200)

    for (const label of ['Fetch', 'Refresh', 'Open in editor']) {
      const button = findButton(wrapper, label)!
      expect(button.classes()).toContain('p-button')
      button.get('.p-button-icon') // throws if the icon is missing
      expect(button.attributes('title')).toBe(label)
      expect(button.attributes('aria-label')).toBe(label)
      expect(button.find('.p-button-label').exists()).toBe(false)
    }
  })

  it('FR-UX-09 medium (1200): "Create branch here" also shows icon only, with title and aria-label', async () => {
    Facade.Summary.mockResolvedValue(summaryFixture({ branch: '', detached: true }))
    const wrapper = await mountAt(1200)

    const button = findButton(wrapper, 'Create branch here')!
    button.get('.p-button-icon') // throws if the icon is missing
    expect(button.attributes('title')).toBe('Create branch here')
    expect(button.attributes('aria-label')).toBe('Create branch here')
    expect(button.find('.p-button-label').exists()).toBe(false)
  })

  it('FR-UX-09 Fetch, Refresh, Open in editor and Settings are OpenVue Buttons, secondary and outlined', async () => {
    const wrapper = await mountAt(1600)

    for (const label of ['Fetch', 'Refresh', 'Open in editor', 'Settings']) {
      const classes = findButton(wrapper, label)!.classes()
      expect(classes).toContain('p-button-secondary')
      expect(classes).toContain('p-button-outlined')
    }
  })

  it('FR-UX-09 narrow (600): the toolbar buttons shrink to the small size', async () => {
    const wrapper = await mountAt(600)

    expect(findButton(wrapper, 'Refresh')!.classes()).toContain('p-button-sm')
  })

  it('FR-UX-07 narrow (600): a commit message typed in Changes survives switching to Log and back', async () => {
    const wrapper = await mountAt(600)

    await switchTo(wrapper, 'Changes')
    await wrapper.get('textarea').setValue('WIP message')

    await switchTo(wrapper, 'Log')
    await switchTo(wrapper, 'Changes')

    expect(wrapper.get('textarea').element.value).toBe('WIP message')
  })

  it('FR-UX-08 narrow (600): switching to Changes and back keeps the log list scroll position', async () => {
    const wrapper = await mountAt(600)
    const logList = wrapper.get('.log-list').element
    logList.scrollTop = 42

    await switchTo(wrapper, 'Changes')
    await switchTo(wrapper, 'Log')

    expect(wrapper.get('.log-list').element).toBe(logList)
    expect(logList.scrollTop).toBe(42)
  })
})
