// Requirements: FR-UX-05, FR-UX-08, FR-UX-09, FR-UX-21
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
import { expectIconOnly } from '@/testing/expectIconOnly'
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

// switchTo clicks the tab labelled label (e.g. "Log", "Details", "Changes",
// or "Changes (4)").
async function switchTo(wrapper: VueWrapper, label: string): Promise<void> {
  const tab = wrapper.findAll('[role="tab"]').find((t) => t.text() === label)
  await tab!.trigger('click')
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

  it('wide (1600): all three views show side by side, no switch', async () => {
    const wrapper = await mountAt(1600)
    await clickRow(wrapper)

    expect(wrapper.findComponent(LogView).isVisible()).toBe(true)
    expect(wrapper.findComponent(CommitDetailView).isVisible()).toBe(true)
    expect(wrapper.findComponent(ChangesView).isVisible()).toBe(true)
    expect(wrapper.find('[role="tablist"]').exists()).toBe(false)
  })

  it('medium (1200): the log shows beside a "Views" tablist [Details, Changes (4)] defaulting to Details; clicking Changes shows the changes view', async () => {
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
    await clickRow(wrapper)

    expect(wrapper.get('[role="tablist"]').attributes('aria-label')).toBe('Views')
    const tabs = wrapper.findAll('[role="tab"]')
    expect(tabs.map((tab) => tab.text())).toEqual(['Details', 'Changes (4)'])
    expect(tabs[0]!.attributes('aria-selected')).toBe('true')
    expect(wrapper.findComponent(LogView).isVisible()).toBe(true)
    expect(wrapper.findComponent(CommitDetailView).isVisible()).toBe(true)
    expect(wrapper.findComponent(ChangesView).isVisible()).toBe(false)

    await switchTo(wrapper, 'Changes (4)')

    expect(wrapper.findAll('[role="tab"]')[1]!.attributes('aria-selected')).toBe('true')
    expect(wrapper.findComponent(ChangesView).isVisible()).toBe(true)
  })

  it('medium (1200): clicking a commit switches the side panel back to Details', async () => {
    const wrapper = await mountAt(1200)
    await switchTo(wrapper, 'Changes')
    expect(wrapper.findComponent(ChangesView).isVisible()).toBe(true)

    await clickRow(wrapper)

    expect(wrapper.findComponent(CommitDetailView).isVisible()).toBe(true)
    expect(wrapper.findComponent(ChangesView).isVisible()).toBe(false)
  })

  it('narrow (600): only the log view shows, switch [Log, Details, Changes] starting on Log', async () => {
    const wrapper = await mountAt(600)
    await clickRow(wrapper)

    const log = wrapper.findAll('[role="tab"]').find((tab) => tab.text() === 'Log')!
    expect(log.attributes('aria-selected')).toBe('true')
    expect(wrapper.findComponent(LogView).isVisible()).toBe(true)
    expect(wrapper.findComponent(CommitDetailView).isVisible()).toBe(false)
    expect(wrapper.findComponent(ChangesView).isVisible()).toBe(false)
    expect(wrapper.findAll('[role="tab"]').map((tab) => tab.text())).toEqual([
      'Log',
      'Details',
      'Changes',
    ])
  })

  it('narrow (600): a click keeps the log view; a double-click switches to Details', async () => {
    const wrapper = await mountAt(600)
    await clickRow(wrapper)
    expect(wrapper.findComponent(LogView).isVisible()).toBe(true)

    await wrapper.get('.row').trigger('dblclick')
    await flushPromises()
    expect(wrapper.findComponent(CommitDetailView).isVisible()).toBe(true)
  })

  it.each([
    ['FR-LOG-06 narrow (600): brings the log view forward', 600, false, false],
    ['FR-LOG-06 medium (1200): leaves the side panel as it was', 1200, true, true],
  ])(
    "%s when the branch picker's show-log fires",
    async (_name, width, logShownBefore, changesShownAfter) => {
      const wrapper = await mountAt(width)
      await switchTo(wrapper, 'Changes')
      expect(wrapper.findComponent(LogView).isVisible()).toBe(logShownBefore)

      wrapper.findComponent(BranchPicker).vm.$emit('show-log')
      await flushPromises()

      expect(wrapper.findComponent(LogView).isVisible()).toBe(true)
      expect(wrapper.findComponent(ChangesView).isVisible()).toBe(changesShownAfter)
    },
  )

  it('medium (1200): Fetch and Open in editor show icon only, with title and aria-label', async () => {
    Facade.Refs.mockResolvedValue({
      branches: [],
      remotes: [{ name: 'origin', branches: [] }],
      tags: [],
    })
    const wrapper = await mountAt(1200)

    for (const label of ['Fetch', 'Open in editor']) {
      const button = findButton(wrapper, label)!
      expect(button.classes()).toContain('p-button')
      button.get('.p-button-icon') // throws if the icon is missing
      expectIconOnly(button, label)
    }
  })

  it('Refresh names its shortcut in the tooltip, not in aria-label', async () => {
    const wrapper = await mountAt(1200)

    const button = findButton(wrapper, 'Refresh')!
    expect(button.attributes('title')).toBe('Refresh · F5')
    expect(button.attributes('aria-label')).toBe('Refresh')
  })

  it('medium (1200): "Create branch here" also shows icon only, with title and aria-label', async () => {
    Facade.Summary.mockResolvedValue(summaryFixture({ branch: '', detached: true }))
    const wrapper = await mountAt(1200)

    const button = findButton(wrapper, 'Create branch here')!
    button.get('.p-button-icon') // throws if the icon is missing
    expectIconOnly(button, 'Create branch here')
  })

  it('narrow (600): switching to Changes and back keeps the commit message and the log list scroll position', async () => {
    const wrapper = await mountAt(600)
    const logList = wrapper.get('.log-list').element
    logList.scrollTop = 42

    await switchTo(wrapper, 'Changes')
    await wrapper.get('textarea').setValue('WIP message')
    await switchTo(wrapper, 'Log')

    expect(wrapper.get('.log-list').element).toBe(logList)
    expect(logList.scrollTop).toBe(42)

    await switchTo(wrapper, 'Changes')

    expect(wrapper.get('textarea').element.value).toBe('WIP message')
  })
})
