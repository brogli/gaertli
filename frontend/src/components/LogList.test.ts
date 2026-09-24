import { DOMWrapper, flushPromises, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mountInBody } from '../testing/mountInBody'

const { Facade } = vi.hoisted(() => ({
  Facade: {
    LoadLog: vi.fn(),
    LogRows: vi.fn(),
    CommitDetails: vi.fn(),
    CherryPick: vi.fn(),
    Revert: vi.fn(),
    LogIndexOf: vi.fn().mockResolvedValue({ index: 0, found: false }),
  },
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

const gitDefaults = { noCommit: false, recordOrigin: false, mainline: 0 }

vi.mock('@wailsio/runtime', () => import('../testing/runtime'))

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

// mountWithRows renders LogList with rows already cached, ready to
// click/shift-click; a sha stands for row(sha). clientHeight, not just
// offsetHeight, is what @tanstack/vue-virtual needs to size enough rows for
// a shift-click range to render under happy-dom.
async function mountWithRows(
  rows: (LogCommit | string)[],
): Promise<{ wrapper: VueWrapper; body: DOMWrapper<Element>; rowEls: DOMWrapper<Element>[] }> {
  Object.defineProperty(HTMLElement.prototype, 'offsetHeight', { configurable: true, value: 500 })
  Object.defineProperty(HTMLElement.prototype, 'clientHeight', { configurable: true, value: 500 })

  const { wrapper, body } = await mountInBody(LogList, { props: { wt: '/repo' } })
  const log = useLogStore()
  log.loaded = rows.length
  rows.forEach((r, i) => log.rowCache.set(i, typeof r === 'string' ? row(r) : r))
  await wrapper.vm.$nextTick()
  await wrapper.vm.$nextTick()

  return { wrapper, body, rowEls: wrapper.findAll('.row') }
}

// openMenu right-clicks rowEl and waits for its context menu.
async function openMenu(wrapper: VueWrapper, rowEl: DOMWrapper<Element>): Promise<void> {
  await rowEl.trigger('contextmenu')
  await wrapper.vm.$nextTick()
}

describe('LogList', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.LoadLog.mockReset().mockResolvedValue(undefined)
    Facade.LogRows.mockReset().mockResolvedValue({ rows: [], loaded: 0, done: true })
    Facade.CommitDetails.mockReset().mockResolvedValue({
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
    Facade.CherryPick.mockReset().mockResolvedValue(undefined)
    Facade.Revert.mockReset().mockResolvedValue(undefined)
  })

  it('FR-LOG-11 right-click opens a context menu with copy, branch, checkout-commit, reset, cherry-pick, and revert items', async () => {
    const { wrapper, body, rowEls } = await mountWithRows([sampleRow])
    await openMenu(wrapper, rowEls[0]!)

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
  })

  it('FR-WT-06 clicking "Create worktree here…" emits create-worktree with the row\'s sha', async () => {
    const { wrapper, body, rowEls } = await mountWithRows([sampleRow])
    await openMenu(wrapper, rowEls[0]!)

    await clickMenuItem(body, 'Create worktree here…')

    expect(wrapper.emitted('create-worktree')).toEqual([['abc123']])
  })

  it('FR-BR-01 clicking "New branch from here…" emits create-branch with the row\'s sha', async () => {
    const { wrapper, body, rowEls } = await mountWithRows([sampleRow])
    await openMenu(wrapper, rowEls[0]!)

    await clickMenuItem(body, 'New branch from here…')

    expect(wrapper.emitted('create-branch')).toEqual([['abc123']])
  })

  it('FR-BR-08 clicking "Check out commit" emits checkout-commit with the row\'s sha', async () => {
    const { wrapper, body, rowEls } = await mountWithRows([sampleRow])
    await openMenu(wrapper, rowEls[0]!)

    await clickMenuItem(body, 'Check out commit')

    expect(wrapper.emitted('checkout-commit')).toEqual([['abc123']])
  })

  it('FR-BR-02 clicking "Reset current branch to here…" emits reset with the row\'s sha and subject', async () => {
    const { wrapper, body, rowEls } = await mountWithRows([sampleRow])
    await openMenu(wrapper, rowEls[0]!)

    await clickMenuItem(body, 'Reset current branch to here…')

    expect(wrapper.emitted('reset')).toEqual([['abc123', 'Fix the bug']])
  })

  it('FR-LOG-12 clicking a menu item copies to the clipboard and closes the menu', async () => {
    const writeText = vi.fn()
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })

    const { wrapper, body, rowEls } = await mountWithRows([sampleRow])
    await openMenu(wrapper, rowEls[0]!)

    await clickMenuItem(body, 'Copy hash')

    expect(writeText).toHaveBeenCalledTimes(1)
    expect(writeText).toHaveBeenCalledWith('abc123')

    await wrapper.vm.$nextTick()
    expect(body.find('.p-contextmenu').exists()).toBe(false)
  })

  it('FR-LOG-02 hides GraphCell when the view is flat, shows it otherwise', async () => {
    const { wrapper } = await mountWithRows([sampleRow])
    const log = useLogStore()

    expect(wrapper.find('.graph-cell').exists()).toBe(true)

    log.filter = { query: 'x' }
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.graph-cell').exists()).toBe(false)
  })

  it('FR-LOG-09 the visible rows not yet cached load in one LogRows call', async () => {
    await mountInBody(LogList, { props: { wt: '/repo' } })
    useLogStore().loaded = 3
    await flushPromises()

    expect(Facade.LogRows).toHaveBeenCalledTimes(1)
    expect(Facade.LogRows).toHaveBeenCalledWith('/repo', 0, 3)
  })

  it('FR-LOG-01 a row shows its short hash', async () => {
    const { wrapper } = await mountWithRows([
      commitRow({ sha: 'abcdef0123456789', shortSha: 'abcdef0' }),
    ])

    expect(wrapper.find('.row .hash').text()).toBe('abcdef0')
  })

  async function mountWithRefs(names: string[]) {
    const refs = names.map((name) => ({ name, kind: 'branch' }))
    return (await mountWithRows([commitRow({ refs })])).wrapper
  }

  it("FR-LOG-01 each ref label's title is its full name", async () => {
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
    const { rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])
    const compare = useCompareStore()
    expect(rowEls.length).toBe(3)

    await rowEls[0]!.trigger('click', { ctrlKey: true })
    await rowEls[1]!.trigger('click', { ctrlKey: true })
    expect(compare.selection.map((r) => r.sha)).toEqual(['sha-a', 'sha-b'])

    await rowEls[2]!.trigger('click')
    expect(compare.selection).toEqual([])
    expect(Facade.CommitDetails).toHaveBeenCalledWith('/repo', 'sha-c')
  })

  it('FR-CMP-05 a comparison of rows 2 and 7 marks row 7 compare-older, row 2 compare-newer, neither selected', async () => {
    const shas = Array.from({ length: 8 }, (_, i) => `sha-${i}`)
    const { wrapper, rowEls } = await mountWithRows(shas)

    await rowEls[2]!.trigger('click', { ctrlKey: true })
    await rowEls[7]!.trigger('click', { ctrlKey: true })

    const classesOf = (i: number) => rowEls[i]!.classes()
    expect(classesOf(7)).toContain('compare-older')
    expect(classesOf(2)).toContain('compare-newer')
    expect(wrapper.findAll('.row').some((r) => r.classes('selected'))).toBe(false)
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
    await openMenu(wrapper, rowEls[1]!)
    await clickMenuItem(body, 'Cherry-pick 3 commits')

    expect(Facade.CherryPick).toHaveBeenCalledTimes(1)
    expect(Facade.CherryPick).toHaveBeenCalledWith(
      '/repo',
      ['sha-a', 'sha-b', 'sha-c'],
      gitDefaults,
    )
  })

  it('FR-CP-01 a plain click outside the range collapses it before acting', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c', 'sha-d'])

    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    await rowEls[3]!.trigger('click')
    await openMenu(wrapper, rowEls[3]!)
    await clickMenuItem(body, 'Cherry-pick')

    expect(Facade.CherryPick).toHaveBeenCalledWith('/repo', ['sha-d'], gitDefaults)
  })

  it('FR-CP-01 clears the multi-select once the cherry-pick resolves', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])
    const log = useLogStore()

    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    expect(log.multiSelect).toHaveLength(3)

    await openMenu(wrapper, rowEls[1]!)
    await clickMenuItem(body, 'Cherry-pick 3 commits')
    await flushPromises()

    expect(log.multiSelect).toEqual([])
  })

  it("FR-CP-04 a rejection renders in LogList's own ErrorBanner and keeps the multi-select", async () => {
    Facade.CherryPick.mockRejectedValueOnce(
      Object.assign(new Error('boom'), { cause: { message: 'would be overwritten by merge' } }),
    )
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])
    const log = useLogStore()

    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    await openMenu(wrapper, rowEls[1]!)
    await clickMenuItem(body, 'Cherry-pick 3 commits')
    await flushPromises()
    await wrapper.vm.$nextTick()

    expect(wrapper.text()).toContain('would be overwritten by merge')
    expect(log.multiSelect).toHaveLength(3)
  })

  it('FR-LOG-11 "Revert" acts directly, the pre-existing call path', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b'])

    await rowEls[0]!.trigger('click')
    await openMenu(wrapper, rowEls[0]!)
    await clickMenuItem(body, 'Revert')

    expect(Facade.Revert).toHaveBeenCalledTimes(1)
    expect(Facade.Revert).toHaveBeenCalledWith('/repo', ['sha-a'], gitDefaults)
  })

  it('FR-CP-08 a single selected merge commit routes "Cherry-pick" through PickDialog instead of acting directly', async () => {
    const { wrapper, body, rowEls } = await mountWithRows([row('sha-m', { merge: true })])
    await openMenu(wrapper, rowEls[0]!)
    await clickMenuItem(body, 'Cherry-pick')
    await wrapper.vm.$nextTick()

    expect(Facade.CherryPick).not.toHaveBeenCalled()
    expect(body.find('[data-test="pick-mainline"]').exists()).toBe(true)

    await confirmPick(body)

    expect(Facade.CherryPick).toHaveBeenCalledTimes(1)
    expect(Facade.CherryPick).toHaveBeenCalledWith('/repo', ['sha-m'], gitDefaults)
  })

  it('FR-CP-08 a multi-select including a merge commit still acts directly (no single merge to choose a parent for)', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', row('sha-m', { merge: true })])

    await rowEls[0]!.trigger('click')
    await rowEls[1]!.trigger('click', { shiftKey: true })
    await openMenu(wrapper, rowEls[1]!)
    await clickMenuItem(body, 'Cherry-pick 2 commits')

    expect(Facade.CherryPick).toHaveBeenCalledWith('/repo', ['sha-a', 'sha-m'], gitDefaults)
  })

  it('FR-CP-06, FR-CP-08 "Cherry-pick…" opens PickDialog with the current selection', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])

    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    await openMenu(wrapper, rowEls[1]!)
    await clickMenuItem(body, 'Cherry-pick 3 commits…')
    await confirmPick(body)

    expect(Facade.CherryPick).toHaveBeenCalledTimes(1)
    expect(Facade.CherryPick).toHaveBeenCalledWith(
      '/repo',
      ['sha-a', 'sha-b', 'sha-c'],
      gitDefaults,
    )
  })

  it('FR-RV-04 "Revert…" opens PickDialog with the current selection', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b'])

    await rowEls[0]!.trigger('click')
    await openMenu(wrapper, rowEls[0]!)
    await clickMenuItem(body, 'Revert…')
    await confirmPick(body)

    expect(Facade.Revert).toHaveBeenCalledTimes(1)
    expect(Facade.Revert).toHaveBeenCalledWith('/repo', ['sha-a'], gitDefaults)
  })

  it('FR-CP-06 closing PickDialog after a "…" pick clears the multi-select', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])
    const log = useLogStore()

    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    await openMenu(wrapper, rowEls[1]!)
    await clickMenuItem(body, 'Cherry-pick 3 commits…')
    await confirmPick(body)
    await flushPromises()

    expect(log.multiSelect).toEqual([])
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
  })

  it('FR-CMP-01, FR-LOG-11 right-click B after selecting A: B alone selected, menu offers "Compare with <A short>"', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b'])
    const log = useLogStore()

    await rowEls[0]!.trigger('click')
    await openMenu(wrapper, rowEls[1]!)

    const labels = body.findAll('.p-contextmenu-item-label').map((l) => l.text())
    expect(labels).toContain('Compare with sha-a')
    expect(log.multiSelect.map((r) => r.sha)).toEqual(['sha-b'])
  })

  it('FR-CMP-01 choosing "Compare with <A short>" compares A and B', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b'])
    const compare = useCompareStore()

    await rowEls[0]!.trigger('click')
    await openMenu(wrapper, rowEls[1]!)
    await clickMenuItem(body, 'Compare with sha-a')

    expect(compare.selection.map((r) => r.sha).sort()).toEqual(['sha-a', 'sha-b'])
    expect(compare.active).toBe(true)
  })

  it('FR-CMP-01 right-click with nothing selected yet shows no "Compare with …" item', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b'])

    await openMenu(wrapper, rowEls[0]!)

    const labels = body.findAll('.p-contextmenu-item-label').map((l) => l.text())
    expect(
      labels.some((l) => l !== 'Compare with working tree' && l.startsWith('Compare with ')),
    ).toBe(false)
  })

  it('FR-CMP-04 right-click a row and clicking "Compare with working tree" starts it', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a'])
    const compare = useCompareStore()

    await openMenu(wrapper, rowEls[0]!)
    await clickMenuItem(body, 'Compare with working tree')

    expect(compare.workingCommit?.sha).toBe('sha-a')
  })

  it('FR-CMP-01 right-click on the already-selected row shows no "Compare with …" item', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b'])

    await rowEls[0]!.trigger('click')
    await openMenu(wrapper, rowEls[0]!)

    const labels = body.findAll('.p-contextmenu-item-label').map((l) => l.text())
    expect(
      labels.some((l) => l !== 'Compare with working tree' && l.startsWith('Compare with ')),
    ).toBe(false)
  })

  it('FR-CMP-01 ctrl-click with the compare selection empty pairs the selected row with the ctrl-clicked one', async () => {
    const { rowEls } = await mountWithRows(['sha-a', 'sha-b'])
    const compare = useCompareStore()

    await rowEls[0]!.trigger('click')
    await rowEls[1]!.trigger('click', { ctrlKey: true })

    expect(compare.selection.map((r) => r.sha)).toEqual(['sha-a', 'sha-b'])
    expect(compare.active).toBe(true)
  })

  it('FR-LOG-11 right-click inside a range of 3 opens the range menu, labelled with the count', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])

    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    await openMenu(wrapper, rowEls[1]!)

    const labels = body.findAll('.p-contextmenu-item-label').map((l) => l.text())
    expect(labels).toEqual([
      'Cherry-pick 3 commits',
      'Cherry-pick 3 commits…',
      'Revert 3 commits',
      'Revert 3 commits…',
      'Copy 3 hashes',
    ])
  })

  it('FR-LOG-11 "Copy 3 hashes" copies the full hashes, one per line, in log order', async () => {
    const writeText = vi.fn()
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, configurable: true })
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])

    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    await openMenu(wrapper, rowEls[1]!)
    await clickMenuItem(body, 'Copy 3 hashes')

    expect(writeText).toHaveBeenCalledWith('sha-a\nsha-b\nsha-c')
  })

  it('FR-LOG-11 right-click one of the two compared commits opens the comparison menu', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b'])
    const compare = useCompareStore()
    compare.toggleSelect('/repo', row('sha-a'), 0)
    compare.toggleSelect('/repo', row('sha-b'), 1)

    await openMenu(wrapper, rowEls[0]!)

    const labels = body.findAll('.p-contextmenu-item-label').map((l) => l.text())
    expect(labels).toEqual(['Swap', 'Stop comparing', 'Copy both hashes'])
  })

  it('FR-UX-08 double-click or Enter on a row emits open', async () => {
    const { wrapper, rowEls } = await mountWithRows(['sha-a', 'sha-b'])

    await rowEls[0]!.trigger('dblclick')
    expect(wrapper.emitted('open')).toHaveLength(1)

    await rowEls[1]!.trigger('keydown', { key: 'Enter' })
    expect(wrapper.emitted('open')).toHaveLength(2)
  })

  it('FR-LOG-11 right-click a commit outside a comparison ends it and selects that commit alone', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])
    const compare = useCompareStore()
    const log = useLogStore()
    compare.toggleSelect('/repo', row('sha-a'), 0)
    compare.toggleSelect('/repo', row('sha-b'), 1)

    await openMenu(wrapper, rowEls[2]!)

    expect(compare.active).toBe(false)
    expect(log.multiSelect.map((r) => r.sha)).toEqual(['sha-c'])
    const labels = body.findAll('.p-contextmenu-item-label').map((l) => l.text())
    expect(labels).toContain('Copy hash')
    expect(labels).not.toContain('Swap')
  })
})
