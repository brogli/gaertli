import { DOMWrapper, flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mountInBody } from '../testing/mountInBody'

const { mockLoadLog, mockLogRows, mockCommitDetails, mockCherryPick, mockRevert, mockCherryPickWith, mockRevertWith } =
  vi.hoisted(() => ({
    mockLoadLog: vi.fn(),
    mockLogRows: vi.fn(),
    mockCommitDetails: vi.fn(),
    mockCherryPick: vi.fn(),
    mockRevert: vi.fn(),
    mockCherryPickWith: vi.fn(),
    mockRevertWith: vi.fn(),
  }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: {
    LoadLog: mockLoadLog,
    LogRows: mockLogRows,
    CommitDetails: mockCommitDetails,
    CherryPick: mockCherryPick,
    Revert: mockRevert,
    CherryPickWith: mockCherryPickWith,
    RevertWith: mockRevertWith,
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
import { clickMenuItem } from '../testing/contextMenu'

function row(sha: string, overrides: Partial<LogCommit> = {}): LogCommit {
  return commitRow({ sha, shortSha: sha, subject: sha, ...overrides })
}

const sampleRow = commitRow()

// mountWithRows renders LogList with shas.length rows already cached, ready
// to click/shift-click. clientHeight, not just offsetHeight, is what
// @tanstack/vue-virtual needs to size enough rows for a shift-click range
// to render under happy-dom.
async function mountWithRows(
  shas: string[],
): Promise<{ wrapper: VueWrapper; body: DOMWrapper<Element>; rowEls: DOMWrapper<Element>[] }> {
  Object.defineProperty(HTMLElement.prototype, 'offsetHeight', { configurable: true, value: 500 })
  Object.defineProperty(HTMLElement.prototype, 'clientHeight', { configurable: true, value: 500 })

  const { wrapper, body } = await mountInBody(LogList, { props: { wt: '/repo' } })
  const log = useLogStore()
  const rows = shas.map((sha) => row(sha))
  log.loaded = rows.length
  rows.forEach((r, i) => log.rowCache.set(i, r))
  await wrapper.vm.$nextTick()
  await wrapper.vm.$nextTick()

  return { wrapper, body, rowEls: wrapper.findAll('.row') }
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
    mockCherryPickWith.mockReset()
    mockCherryPickWith.mockResolvedValue(undefined)
    mockRevertWith.mockReset()
    mockRevertWith.mockResolvedValue(undefined)
  })

  it('FR-LOG-11 right-click opens a context menu with copy, branch, checkout-commit, reset, cherry-pick, and revert items', async () => {
    const { wrapper, body } = await mountInBody(LogList, { props: { wt: '/repo' } })
    const log = useLogStore()
    log.rowCache.set(0, sampleRow)
    log.loaded = 1
    await wrapper.vm.$nextTick()

    await wrapper.get('.row').trigger('contextmenu')
    await wrapper.vm.$nextTick()

    const labels = body.findAll('.p-contextmenu-item-label').map((n) => n.text())
    expect(labels).toEqual([
      'Copy hash',
      'Copy subject',
      'New branch from here…',
      'Check out commit',
      'Reset current branch to here…',
      'Cherry-pick',
      'Cherry-pick…',
      'Revert',
      'Revert…',
      'Create worktree here…',
      'Compare with working tree',
    ])

    wrapper.unmount()
  })

  it('FR-WT-06 clicking "Create worktree here…" emits create-worktree with the row\'s sha', async () => {
    const { wrapper, body } = await mountInBody(LogList, { props: { wt: '/repo' } })
    const log = useLogStore()
    log.rowCache.set(0, sampleRow)
    log.loaded = 1
    await wrapper.vm.$nextTick()

    await wrapper.get('.row').trigger('contextmenu')
    await wrapper.vm.$nextTick()

    await clickMenuItem(body, 'Create worktree here…')

    expect(wrapper.emitted('create-worktree')).toEqual([['abc123']])

    wrapper.unmount()
  })

  it('FR-BR-01 clicking "New branch from here…" emits create-branch with the row\'s sha', async () => {
    const { wrapper, body } = await mountInBody(LogList, { props: { wt: '/repo' } })
    const log = useLogStore()
    log.rowCache.set(0, sampleRow)
    log.loaded = 1
    await wrapper.vm.$nextTick()

    await wrapper.get('.row').trigger('contextmenu')
    await wrapper.vm.$nextTick()

    await clickMenuItem(body, 'New branch from here…')

    expect(wrapper.emitted('create-branch')).toEqual([['abc123']])

    wrapper.unmount()
  })

  it('FR-BR-08 clicking "Check out commit" emits checkout-commit with the row\'s sha', async () => {
    const { wrapper, body } = await mountInBody(LogList, { props: { wt: '/repo' } })
    const log = useLogStore()
    log.rowCache.set(0, sampleRow)
    log.loaded = 1
    await wrapper.vm.$nextTick()

    await wrapper.get('.row').trigger('contextmenu')
    await wrapper.vm.$nextTick()

    await clickMenuItem(body, 'Check out commit')

    expect(wrapper.emitted('checkout-commit')).toEqual([['abc123']])

    wrapper.unmount()
  })

  it('FR-BR-02 clicking "Reset current branch to here…" emits reset with the row\'s sha and subject', async () => {
    const { wrapper, body } = await mountInBody(LogList, { props: { wt: '/repo' } })
    const log = useLogStore()
    log.rowCache.set(0, sampleRow)
    log.loaded = 1
    await wrapper.vm.$nextTick()

    await wrapper.get('.row').trigger('contextmenu')
    await wrapper.vm.$nextTick()

    await clickMenuItem(body, 'Reset current branch to here…')

    expect(wrapper.emitted('reset')).toEqual([['abc123', 'Fix the bug']])

    wrapper.unmount()
  })

  it('FR-LOG-12 clicking a menu item copies to the clipboard and closes the menu', async () => {
    const writeText = vi.fn()
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })

    const { wrapper, body } = await mountInBody(LogList, { props: { wt: '/repo' } })
    const log = useLogStore()
    log.rowCache.set(0, sampleRow)
    log.loaded = 1
    await wrapper.vm.$nextTick()

    await wrapper.get('.row').trigger('contextmenu')
    await wrapper.vm.$nextTick()

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

  it('FR-LOG-01 a row shows its short hash', async () => {
    const wrapper = mount(LogList, { props: { wt: '/repo' } })
    await flushPromises()
    const log = useLogStore()
    log.rowCache.set(0, commitRow({ sha: 'abcdef0123456789', shortSha: 'abcdef0' }))
    log.loaded = 1
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.row .hash').text()).toBe('abcdef0')
  })

  async function mountWithRefs(names: string[]) {
    const wrapper = mount(LogList, { props: { wt: '/repo' } })
    await flushPromises()
    const log = useLogStore()
    log.rowCache.set(0, commitRow({ refs: names.map((name) => ({ name, kind: 'branch' })) }))
    log.loaded = 1
    await wrapper.vm.$nextTick()
    return wrapper
  }

  it('FR-LOG-01 each ref label\'s title is its full name', async () => {
    const wrapper = await mountWithRefs(['main', 'origin/main'])

    const labels = wrapper.findAll('.ref-label')
    expect(labels.map((l) => l.attributes('title'))).toEqual(['main', 'origin/main'])
    expect(wrapper.find('.ref-more').exists()).toBe(false)
  })

  it('FR-LOG-01 a row shows two ref labels, then "+N" with every label in its title', async () => {
    const wrapper = await mountWithRefs(['HEAD', 'main', 'origin/main', 'v1.0'])

    expect(wrapper.findAll('.ref-label').map((l) => l.text())).toEqual(['HEAD', 'main'])
    const more = wrapper.get('.ref-more')
    expect(more.text()).toBe('+2')
    expect(more.attributes('title')).toBe('HEAD, main, origin/main, v1.0')
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

  // confirmPick clicks PickDialog's own Confirm button (already open, from
  // a prior "…" menu click, or a plain "Cherry-pick" on a single merge
  // commit).
  async function confirmPick(body: DOMWrapper<Element>): Promise<void> {
    await body.get('[data-test="pick-submit"]').trigger('click')
  }

  it('FR-CP-01, FR-LOG-11 click then shift-click then right-click acts directly on the whole range', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])

    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    await rowEls[1]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()
    await clickMenuItem(body, 'Cherry-pick')

    expect(mockCherryPick).toHaveBeenCalledTimes(1)
    expect(mockCherryPick).toHaveBeenCalledWith('/repo', ['sha-a', 'sha-b', 'sha-c'])
    expect(mockCherryPickWith).not.toHaveBeenCalled()

    wrapper.unmount()
  })

  it('FR-CP-01 a plain click outside the range collapses it before acting', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c', 'sha-d'])

    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    await rowEls[3]!.trigger('click')
    await rowEls[3]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()
    await clickMenuItem(body, 'Cherry-pick')

    expect(mockCherryPick).toHaveBeenCalledWith('/repo', ['sha-d'])

    wrapper.unmount()
  })

  it('FR-CP-01 clears the multi-select once the cherry-pick resolves', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])
    const log = useLogStore()

    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    expect(log.multiSelect).toHaveLength(3)

    await rowEls[1]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()
    await clickMenuItem(body, 'Cherry-pick')
    await flushPromises()

    expect(log.multiSelect).toEqual([])

    wrapper.unmount()
  })

  it("FR-CP-04 a rejection renders in LogList's own ErrorBanner and keeps the multi-select", async () => {
    mockCherryPick.mockRejectedValueOnce(
      Object.assign(new Error('boom'), { cause: { message: 'would be overwritten by merge' } }),
    )
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])
    const log = useLogStore()

    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    await rowEls[1]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()
    await clickMenuItem(body, 'Cherry-pick')
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('would be overwritten by merge')
    expect(log.multiSelect).toHaveLength(3)

    wrapper.unmount()
  })

  it('FR-LOG-11 "Revert" acts directly, the pre-existing call path', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b'])

    await rowEls[0]!.trigger('click')
    await rowEls[0]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()
    await clickMenuItem(body, 'Revert')

    expect(mockRevert).toHaveBeenCalledTimes(1)
    expect(mockRevert).toHaveBeenCalledWith('/repo', ['sha-a'])
    expect(mockRevertWith).not.toHaveBeenCalled()

    wrapper.unmount()
  })

  it('FR-CP-08 a single selected merge commit routes "Cherry-pick" through PickDialog instead of acting directly', async () => {
    const { wrapper, body } = await mountInBody(LogList, { props: { wt: '/repo' } })
    const log = useLogStore()
    log.rowCache.set(0, row('sha-m', { merge: true }))
    log.loaded = 1
    await wrapper.vm.$nextTick()

    await wrapper.get('.row').trigger('contextmenu')
    await wrapper.vm.$nextTick()
    await clickMenuItem(body, 'Cherry-pick')
    await wrapper.vm.$nextTick()

    expect(mockCherryPick).not.toHaveBeenCalled()
    expect(body.find('[data-test="pick-mainline"]').exists()).toBe(true)

    await confirmPick(body)

    expect(mockCherryPickWith).toHaveBeenCalledTimes(1)
    expect(mockCherryPickWith).toHaveBeenCalledWith('/repo', ['sha-m'], {
      noCommit: false,
      recordOrigin: false,
      mainline: 0,
    })

    wrapper.unmount()
  })

  it('FR-CP-08 a multi-select including a merge commit still acts directly (no single merge to choose a parent for)', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-m'])
    const log = useLogStore()
    log.rowCache.set(1, row('sha-m', { merge: true }))

    await rowEls[0]!.trigger('click')
    await rowEls[1]!.trigger('click', { shiftKey: true })
    await rowEls[1]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()
    await clickMenuItem(body, 'Cherry-pick')

    expect(mockCherryPick).toHaveBeenCalledWith('/repo', ['sha-a', 'sha-m'])
    expect(mockCherryPickWith).not.toHaveBeenCalled()

    wrapper.unmount()
  })

  it('FR-CP-06, FR-CP-08 "Cherry-pick…" opens PickDialog with the current selection', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])

    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    await rowEls[1]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()
    await clickMenuItem(body, 'Cherry-pick…')
    await confirmPick(body)

    expect(mockCherryPick).not.toHaveBeenCalled()
    expect(mockCherryPickWith).toHaveBeenCalledTimes(1)
    expect(mockCherryPickWith).toHaveBeenCalledWith('/repo', ['sha-a', 'sha-b', 'sha-c'], {
      noCommit: false,
      recordOrigin: false,
      mainline: 0,
    })

    wrapper.unmount()
  })

  it('FR-RV-04 "Revert…" opens PickDialog with the current selection', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b'])

    await rowEls[0]!.trigger('click')
    await rowEls[0]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()
    await clickMenuItem(body, 'Revert…')
    await confirmPick(body)

    expect(mockRevert).not.toHaveBeenCalled()
    expect(mockRevertWith).toHaveBeenCalledTimes(1)
    expect(mockRevertWith).toHaveBeenCalledWith('/repo', ['sha-a'], {
      noCommit: false,
      recordOrigin: false,
      mainline: 0,
    })

    wrapper.unmount()
  })

  it('FR-CP-06 closing PickDialog after a "…" pick clears the multi-select', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])
    const log = useLogStore()

    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    await rowEls[1]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()
    await clickMenuItem(body, 'Cherry-pick…')
    await confirmPick(body)
    await flushPromises()

    expect(log.multiSelect).toEqual([])

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
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b'])

    await rowEls[0]!.trigger('click')
    await rowEls[1]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()

    const labels = body.findAll('.p-contextmenu-item-label').map((l) => l.text())
    expect(labels).toContain('Compare with selected')

    wrapper.unmount()
  })

  it('FR-CMP-01 clicking "Compare with selected" pairs the previously selected row with the right-clicked one', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b'])
    const compare = useCompareStore()

    await rowEls[0]!.trigger('click')
    await rowEls[1]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()
    await clickMenuItem(body, 'Compare with selected')

    expect(compare.selection.map((r) => r.sha)).toEqual(['sha-a', 'sha-b'])

    wrapper.unmount()
  })

  it('FR-CMP-01 right-click with nothing selected yet shows no "Compare with selected"', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b'])

    await rowEls[0]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()

    const labels = body.findAll('.p-contextmenu-item-label').map((l) => l.text())
    expect(labels).not.toContain('Compare with selected')

    wrapper.unmount()
  })

  it('FR-CMP-04 right-click a row and clicking "Compare with working tree" starts it', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a'])
    const compare = useCompareStore()

    await rowEls[0]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()
    await clickMenuItem(body, 'Compare with working tree')

    expect(compare.workingCommit?.sha).toBe('sha-a')

    wrapper.unmount()
  })

  it('FR-CMP-01 right-click on the already-selected row shows no "Compare with selected"', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b'])

    await rowEls[0]!.trigger('click')
    await rowEls[0]!.trigger('contextmenu')
    await wrapper.vm.$nextTick()

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
