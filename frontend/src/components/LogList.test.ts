import { DOMWrapper, flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockLoadLog, mockLogRows, mockCommitDetails } = vi.hoisted(() => ({
  mockLoadLog: vi.fn(),
  mockLogRows: vi.fn(),
  mockCommitDetails: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { LoadLog: mockLoadLog, LogRows: mockLogRows, CommitDetails: mockCommitDetails },
}))

vi.mock('@wailsio/runtime', () => ({
  Events: { On: vi.fn(() => () => {}) },
}))

import LogList from './LogList.vue'
import { useLogStore } from '../stores/log'
import { useCompareStore } from '../stores/compare'
import type { LogCommit } from '../../bindings/github.com/brogli/nebegit/internal/core'

function row(sha: string): LogCommit {
  return { sha, shortSha: sha, author: '', authorAt: 0, subject: sha, graph: { lane: 0, colour: 0, edges: [] } }
}

const sampleRow = {
  sha: 'abc123',
  shortSha: 'abc123',
  author: 'Alice',
  authorAt: 0,
  subject: 'Fix the bug',
  graph: { lane: 0, colour: 0, edges: [] },
}

// @tanstack/vue-virtual measures its scroll container via offsetWidth/
// offsetHeight, which happy-dom always reports as 0 (no real layout engine
// — the same limitation spec 10 named for its own scrolling e2e case).
// Stubbing a real size is the standard workaround for testing a virtualized
// list under happy-dom/jsdom; it's the minimal mock this suite needs, not a
// library-behaviour assumption.
function stubScrollContainerSize(): void {
  Object.defineProperty(HTMLElement.prototype, 'offsetWidth', { configurable: true, value: 800 })
  Object.defineProperty(HTMLElement.prototype, 'offsetHeight', { configurable: true, value: 600 })
}

describe('LogList', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    stubScrollContainerSize()
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
  })

  it('FR-LOG-11 right-click opens a context menu with copy hash/subject only', async () => {
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
    expect(labels).toEqual(['Copy hash', 'Copy subject'])

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
    const items = body.findAll('.p-contextmenu-item-content')
    await items[0]!.trigger('click')

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
    // @tanstack/vue-virtual needs a measured, non-zero viewport to render
    // any row at all; happy-dom's elements report 0 by default.
    Object.defineProperty(HTMLElement.prototype, 'offsetHeight', { configurable: true, value: 500 })
    Object.defineProperty(HTMLElement.prototype, 'clientHeight', { configurable: true, value: 500 })

    const wrapper = mount(LogList, { props: { wt: '/repo' }, attachTo: document.body })
    await flushPromises()
    const log = useLogStore()
    const compare = useCompareStore()
    const rows = [row('sha-a'), row('sha-b'), row('sha-c')]
    log.loaded = rows.length
    rows.forEach((r, i) => log.rowCache.set(i, r))
    await wrapper.vm.$nextTick()
    await wrapper.vm.$nextTick()

    const rowEls = wrapper.findAll('.row')
    expect(rowEls.length).toBe(3)

    await rowEls[0]!.trigger('click', { ctrlKey: true })
    await rowEls[1]!.trigger('click', { ctrlKey: true })
    expect(compare.selection.map((r) => r.sha)).toEqual(['sha-a', 'sha-b'])

    await rowEls[2]!.trigger('click')
    expect(compare.selection).toEqual([])
    expect(mockCommitDetails).toHaveBeenCalledWith('/repo', 'sha-c')

    wrapper.unmount()
  })
})
