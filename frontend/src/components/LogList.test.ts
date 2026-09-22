import { DOMWrapper, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockLoadLog, mockLogRows, mockCommitDetails, mockCherryPick, mockRevert } = vi.hoisted(() => ({
  mockLoadLog: vi.fn(),
  mockLogRows: vi.fn(),
  mockCommitDetails: vi.fn(),
  mockCherryPick: vi.fn(),
  mockRevert: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: {
    LoadLog: mockLoadLog,
    LogRows: mockLogRows,
    CommitDetails: mockCommitDetails,
    CherryPick: mockCherryPick,
    Revert: mockRevert,
  },
}))

vi.mock('@wailsio/runtime', () => ({
  Events: { On: vi.fn(() => () => {}) },
}))

import LogList from './LogList.vue'
import { useLogStore } from '../stores/log'
import { useCompareStore } from '../stores/compare'
import type { LogCommit } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { commitRow } from '../testing/fixtures/log-rows'

function row(sha: string): LogCommit {
  return { sha, shortSha: sha, author: '', authorAt: 0, subject: sha, graph: { lane: 0, colour: 0, edges: [] } }
}

const sampleRow = commitRow()

// mountWithRows renders LogList with shas.length rows already cached, ready
// to click/shift-click (spec 42's multi-select cases). clientHeight, not
// just offsetHeight, is what @tanstack/vue-virtual needs to size enough
// rows for a shift-click range to render under happy-dom.
async function mountWithRows(shas: string[]): Promise<{ wrapper: VueWrapper; rowEls: DOMWrapper<Element>[] }> {
  Object.defineProperty(HTMLElement.prototype, 'offsetHeight', { configurable: true, value: 500 })
  Object.defineProperty(HTMLElement.prototype, 'clientHeight', { configurable: true, value: 500 })

  const wrapper = mount(LogList, { props: { wt: '/repo' }, attachTo: document.body })
  await flushPromises()
  const log = useLogStore()
  const rows = shas.map(row)
  log.loaded = rows.length
  rows.forEach((r, i) => log.rowCache.set(i, r))
  await wrapper.vm.$nextTick()
  await wrapper.vm.$nextTick()

  return { wrapper, rowEls: wrapper.findAll('.row') }
}

describe('LogList', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockLoadLog.mockReset()
    mockLoadLog.mockResolvedValue(undefined)
    mockLogRows.mockReset()
    mockLogRows.mockResolvedValue({ rows: [], loaded: 0, done: true })
    mockCommitDetails.mockReset()
    mockCommitDetails.mockResolvedValue({
      sha: 'abc123',
      parents: [],
      author: '',
      authorEmail: '',
      authorAt: 0,
      committer: '',
      committerEmail: '',
      committerAt: 0,
      body: '',
      files: [],
    })
    mockCherryPick.mockReset()
    mockCherryPick.mockResolvedValue(undefined)
    mockRevert.mockReset()
    mockRevert.mockResolvedValue(undefined)
  })

  // clickMenuItem finds the context-menu item content by its label text and
  // clicks it — robust to the item list's own order/length (spec 42 appends
  // two items after the ones spec 22/40 already added).
  async function clickMenuItem(body: DOMWrapper<Element>, label: string): Promise<void> {
    const labels = body.findAll('.p-contextmenu-item-label')
    const index = labels.findIndex((l) => l.text() === label)
    const items = body.findAll('.p-contextmenu-item-content')
    await items[index]!.trigger('click')
  }

  it('FR-LOG-11 right-click opens a context menu with copy, branch, checkout-commit, reset, cherry-pick, and revert items', async () => {
    const wrapper = mount(LogList, { props: { wt: '/repo' }, attachTo: document.body })
    await flushPromises()
    const log = useLogStore()
    log.rowCache.set(0, sampleRow)
    log.loaded = 1
    await wrapper.vm.$nextTick()

    await wrapper.get('.row').trigger('contextmenu')
    await wrapper.vm.$nextTick()

    const body = new DOMWrapper(document.body)
    const labels = body.findAll('.p-contextmenu-item-label').map((n) => n.text())
    expect(labels).toEqual([
      'Copy hash',
      'Copy subject',
      'New branch from here…',
      'Check out commit',
      'Reset current branch to here…',
      'Cherry-pick',
      'Revert',
    ])

    wrapper.unmount()
  })

  it('FR-BR-01 clicking "New branch from here…" emits create-branch with the row\'s sha', async () => {
    const wrapper = mount(LogList, { props: { wt: '/repo' }, attachTo: document.body })
    await flushPromises()
    const log = useLogStore()
    log.rowCache.set(0, sampleRow)
    log.loaded = 1
    await wrapper.vm.$nextTick()

    await wrapper.get('.row').trigger('contextmenu')
    await wrapper.vm.$nextTick()

    await clickMenuItem(new DOMWrapper(document.body), 'New branch from here…')

    expect(wrapper.emitted('create-branch')).toEqual([['abc123']])

    wrapper.unmount()
  })

  it('FR-BR-08 clicking "Check out commit" emits checkout-commit with the row\'s sha', async () => {
    const wrapper = mount(LogList, { props: { wt: '/repo' }, attachTo: document.body })
    await flushPromises()
    const log = useLogStore()
    log.rowCache.set(0, sampleRow)
    log.loaded = 1
    await wrapper.vm.$nextTick()

    await wrapper.get('.row').trigger('contextmenu')
    await wrapper.vm.$nextTick()

    await clickMenuItem(new DOMWrapper(document.body), 'Check out commit')

    expect(wrapper.emitted('checkout-commit')).toEqual([['abc123']])

    wrapper.unmount()
  })

  it('FR-BR-02 clicking "Reset current branch to here…" emits reset with the row\'s sha and subject', async () => {
    const wrapper = mount(LogList, { props: { wt: '/repo' }, attachTo: document.body })
    await flushPromises()
    const log = useLogStore()
    log.rowCache.set(0, sampleRow)
    log.loaded = 1
    await wrapper.vm.$nextTick()

    await wrapper.get('.row').trigger('contextmenu')
    await wrapper.vm.$nextTick()

    await clickMenuItem(new DOMWrapper(document.body), 'Reset current branch to here…')

    expect(wrapper.emitted('reset')).toEqual([['abc123', 'Fix the bug']])

    wrapper.unmount()
  })

  it('FR-LOG-12 clicking a menu item copies to the clipboard and closes the menu', async () => {
    const writeText = vi.fn()
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })

    const wrapper = mount(LogList, { props: { wt: '/repo' }, attachTo: document.body })
    await flushPromises()
    const log = useLogStore()
    log.rowCache.set(0, sampleRow)
    log.loaded = 1
    await wrapper.vm.$nextTick()

    await wrapper.get('.row').trigger('contextmenu')
    await wrapper.vm.$nextTick()

    const body = new DOMWrapper(document.body)
    await clickMenuItem(body, 'Copy hash')

    expect(writeText).toHaveBeenCalledTimes(1)
    expect(writeText).toHaveBeenCalledWith('abc123')

    await wrapper.vm.$nextTick()
    expect(body.find('.p-contextmenu').exists()).toBe(false)

    wrapper.unmount()
  })

  it('FR-LOG-02 hides GraphCell when the view is flat, shows it otherwise', async () => {
    const wrapper = mount(LogList, { props: { wt: '/repo' } })
    await flushPromises()
    const log = useLogStore()
    log.rowCache.set(0, sampleRow)
    log.loaded = 1
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.graph-cell').exists()).toBe(true)

    log.filter = { query: 'x' }
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.graph-cell').exists()).toBe(false)
  })

  it('FR-CMP-01 ctrl/cmd-click selects two rows to compare; a plain click exits compare and selects the row', async () => {
    const { wrapper, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])
    const compare = useCompareStore()
    expect(rowEls.length).toBe(3)

    await rowEls[0]!.trigger('click', { ctrlKey: true })
    await rowEls[1]!.trigger('click', { ctrlKey: true })
    expect(compare.selection.map((r) => r.sha)).toEqual(['sha-a', 'sha-b'])

    await rowEls[2]!.trigger('click')
    expect(compare.selection).toEqual([])
    expect(mockCommitDetails).toHaveBeenCalledWith('/repo', 'sha-c')

    wrapper.unmount()
  })

  it('FR-CP-01, FR-LOG-11 click then shift-click then right-click acts on the whole range', async () => {
    const { wrapper, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])

    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    await rowEls[1]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()
    await clickMenuItem(new DOMWrapper(document.body), 'Cherry-pick')

    expect(mockCherryPick).toHaveBeenCalledTimes(1)
    expect(mockCherryPick).toHaveBeenCalledWith('/repo', ['sha-a', 'sha-b', 'sha-c'])

    wrapper.unmount()
  })

  it('FR-CP-01 a plain click outside the range collapses it before acting', async () => {
    const { wrapper, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c', 'sha-d'])

    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    await rowEls[3]!.trigger('click')
    await rowEls[3]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()
    await clickMenuItem(new DOMWrapper(document.body), 'Cherry-pick')

    expect(mockCherryPick).toHaveBeenCalledWith('/repo', ['sha-d'])

    wrapper.unmount()
  })

  it('FR-CP-01 clears the multi-select once the cherry-pick resolves', async () => {
    const { wrapper, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])
    const log = useLogStore()

    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    expect(log.multiSelect).toHaveLength(3)

    await rowEls[1]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()
    await clickMenuItem(new DOMWrapper(document.body), 'Cherry-pick')
    await flushPromises()

    expect(log.multiSelect).toEqual([])

    wrapper.unmount()
  })

  it("FR-CP-04 a rejection renders in LogList's own ErrorBanner and keeps the multi-select", async () => {
    mockCherryPick.mockRejectedValueOnce(
      Object.assign(new Error('boom'), { cause: { message: 'would be overwritten by merge' } }),
    )
    const { wrapper, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])
    const log = useLogStore()

    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    await rowEls[1]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()
    await clickMenuItem(new DOMWrapper(document.body), 'Cherry-pick')
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('would be overwritten by merge')
    expect(log.multiSelect).toHaveLength(3)

    wrapper.unmount()
  })

  it('FR-CP-01 a shift-click range renders multi-selected on the range only', async () => {
    const { wrapper, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c', 'sha-d'])

    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    await wrapper.vm.$nextTick()

    expect(rowEls[0]!.classes()).toContain('multi-selected')
    expect(rowEls[1]!.classes()).toContain('multi-selected')
    expect(rowEls[2]!.classes()).toContain('multi-selected')
    expect(rowEls[3]!.classes()).not.toContain('multi-selected')

    wrapper.unmount()
  })

  it('FR-CMP-01, FR-LOG-11 right-click a different row after an ordinary selection includes "Compare with selected"', async () => {
    const { wrapper, rowEls } = await mountWithRows(['sha-a', 'sha-b'])

    await rowEls[0]!.trigger('click')
    await rowEls[1]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()

    const body = new DOMWrapper(document.body)
    const labels = body.findAll('.p-contextmenu-item-label').map((l) => l.text())
    expect(labels).toContain('Compare with selected')

    wrapper.unmount()
  })

  it('FR-CMP-01 clicking "Compare with selected" pairs the previously selected row with the right-clicked one', async () => {
    const { wrapper, rowEls } = await mountWithRows(['sha-a', 'sha-b'])
    const compare = useCompareStore()

    await rowEls[0]!.trigger('click')
    await rowEls[1]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()
    await clickMenuItem(new DOMWrapper(document.body), 'Compare with selected')

    expect(compare.selection.map((r) => r.sha)).toEqual(['sha-a', 'sha-b'])

    wrapper.unmount()
  })

  it('FR-CMP-01 right-click with nothing selected yet shows no "Compare with selected"', async () => {
    const { wrapper, rowEls } = await mountWithRows(['sha-a', 'sha-b'])

    await rowEls[0]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()

    const body = new DOMWrapper(document.body)
    const labels = body.findAll('.p-contextmenu-item-label').map((l) => l.text())
    expect(labels).not.toContain('Compare with selected')

    wrapper.unmount()
  })

  it('FR-CMP-01 right-click on the already-selected row shows no "Compare with selected"', async () => {
    const { wrapper, rowEls } = await mountWithRows(['sha-a', 'sha-b'])

    await rowEls[0]!.trigger('click')
    await rowEls[0]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()

    const body = new DOMWrapper(document.body)
    const labels = body.findAll('.p-contextmenu-item-label').map((l) => l.text())
    expect(labels).not.toContain('Compare with selected')

    wrapper.unmount()
  })

  it('FR-CMP-01 ctrl-click with the compare selection empty pairs the selected row with the ctrl-clicked one', async () => {
    const { wrapper, rowEls } = await mountWithRows(['sha-a', 'sha-b'])
    const compare = useCompareStore()

    await rowEls[0]!.trigger('click')
    await rowEls[1]!.trigger('click', { ctrlKey: true })

    expect(compare.selection.map((r) => r.sha)).toEqual(['sha-a', 'sha-b'])
    expect(compare.active).toBe(true)

    wrapper.unmount()
  })
})
