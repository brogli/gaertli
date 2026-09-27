import { flushPromises, type DOMWrapper, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mountInBody } from '@/testing/mountInBody'

const { Facade } = vi.hoisted(() => ({
  Facade: {
    LoadLog: vi.fn(),
    LogRows: vi.fn(),
    CommitDetails: vi.fn(),
    CherryPick: vi.fn(),
    Revert: vi.fn(),
    LogIndexOf: vi.fn().mockResolvedValue({ index: 0, found: false }),
    SquashCheck: vi.fn(),
    Squash: vi.fn(),
    Fixup: vi.fn(),
  },
}))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))
vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import LogList from '../LogList.vue'
import { useLogStore } from '@/stores/log'
import { useCompareStore } from '@/stores/compare'
import {
  EdgeKind,
  RefKind,
  type LogCommit,
} from '../../../bindings/github.com/brogli/nebegit/internal/core'
import { commitRow } from '@/testing/fixtures/log-rows'
import { commitDetailsFixture } from '@/testing/fixtures/commitDetails'
import { clickMenuItem, menuLabels } from '@/testing/contextMenu'
import { exactDate } from '@/lib/format'

const gitDefaults = { noCommit: false, recordOrigin: false }

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

// openMenu right-clicks rowEl and waits for its context menu, which on
// several selected commits first asks SquashCheck.
async function openMenu(wrapper: VueWrapper, rowEl: DOMWrapper<Element>): Promise<void> {
  await rowEl.trigger('contextmenu')
  await flushPromises()
  await wrapper.vm.$nextTick()
}

describe('LogList', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.LoadLog.mockReset().mockResolvedValue(undefined)
    Facade.LogRows.mockReset().mockResolvedValue({ rows: [], loaded: 0, done: true })
    Facade.CommitDetails.mockReset().mockResolvedValue(commitDetailsFixture())
    Facade.CherryPick.mockReset().mockResolvedValue(undefined)
    Facade.Revert.mockReset().mockResolvedValue(undefined)
    Facade.SquashCheck.mockReset().mockResolvedValue({ pushed: false, message: 'A\n\nB' })
    Facade.Fixup.mockReset().mockResolvedValue({ previousHead: 'prev' })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('FR-LOG-11 right-click opens a context menu with copy, branch, checkout-commit, reset, cherry-pick, and revert items', async () => {
    const { wrapper, body, rowEls } = await mountWithRows([sampleRow])
    await openMenu(wrapper, rowEls[0]!)

    const labels = menuLabels(body)
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

  it('FR-WT-02 clicking "Create worktree here…" emits create-worktree with the row\'s sha', async () => {
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

  it('FR-LOG-12 the list cannot be text-selected', async () => {
    const { wrapper } = await mountWithRows([sampleRow])

    expect(wrapper.get<HTMLElement>('.log-list').element.style.userSelect).toBe('none')
  })

  it('FR-LOG-02 hides GraphCell when the view is flat, shows it otherwise', async () => {
    const { wrapper } = await mountWithRows([sampleRow])
    const log = useLogStore()

    expect(wrapper.find('.graph-cell').exists()).toBe(true)

    log.filter = { author: 'x' }
    await wrapper.vm.$nextTick()

    expect(wrapper.find('.graph-cell').exists()).toBe(false)
  })

  it('FR-LOG-05 matched rows carry the highlight class, the current one the strong class, no row lost', async () => {
    const { rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])
    const log = useLogStore()

    log.matches = [0, 2]
    log.current = 1
    await flushPromises()

    expect(rowEls[0]!.classes()).toContain('match')
    expect(rowEls[0]!.classes()).not.toContain('match-current')
    expect(rowEls[1]!.classes()).not.toContain('match')
    expect(rowEls[2]!.classes()).toContain('match')
    expect(rowEls[2]!.classes()).toContain('match-current')
    expect(rowEls.length).toBe(3)
  })

  it('FR-CMP-05 a compared row that is also a search match keeps its comparison bar', async () => {
    const { rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])
    const log = useLogStore()

    await rowEls[0]!.trigger('click')
    await rowEls[1]!.trigger('click', { shiftKey: true })
    log.matches = [0]
    log.current = 0
    await flushPromises()

    expect(rowEls[0]!.classes()).toContain('match-current')
    expect(rowEls[0]!.classes()).toContain('shadow-[inset_3px_0_var(--nb-from)]')
  })

  it('FR-LOG-01 rows not loaded yet show no selection while nothing is selected', async () => {
    const { wrapper } = await mountWithRows(['sha-a'])
    const log = useLogStore()
    log.loaded = 3
    await flushPromises()

    const rowEls = wrapper.findAll('.row')
    expect(rowEls.length).toBe(3)
    for (const el of rowEls) expect(el.classes()).not.toContain('bg-highlight')
  })

  it('FR-LOG-01 every graph strip is as wide as the widest graph among the rendered rows', async () => {
    const through = (lane: number) => ({ lane, kind: EdgeKind.ThroughEdge, colour: lane })
    const { wrapper } = await mountWithRows([
      row('a', { graph: { lane: 0, colour: 0, edges: [] } }),
      row('b', { graph: { lane: 0, colour: 0, edges: [through(1), through(2)] } }),
      row('c', { graph: { lane: 1, colour: 1, edges: [through(0)] } }),
    ])

    const widths = wrapper.findAll('.graph-cell').map((svg) => svg.attributes('width'))
    expect(widths).toEqual(['42', '42', '42'])
  })

  it('FR-LOG-01 the hash column is as wide as the longest short hash among the rendered rows', async () => {
    const { wrapper } = await mountWithRows([
      row('a', { shortSha: 'abcdef0' }),
      row('b', { shortSha: 'abcdef012' }),
    ])

    const list = wrapper.get<HTMLElement>('.log-list').element
    expect(list.style.getPropertyValue('--hash-width')).toBe('9ch')
  })

  it('FR-LOG-01 ref labels follow the subject inside the message column, before the author', async () => {
    const { wrapper } = await mountWithRows([
      commitRow({ refs: [{ name: 'main', kind: RefKind.BranchRef }] }),
    ])

    const parts = wrapper.get('.row').element.children
    expect([...parts].map((el) => el.classList[0])).toEqual([
      'graph-cell',
      'message',
      'author',
      'date',
      'hash',
    ])
    const message = wrapper.get('.row .message')
    expect(message.findAll(':scope > *').map((el) => el.classes()[0])).toEqual([
      'subject',
      'ref-label',
    ])
  })

  it('FR-UX-09 the subject and author show their whole text on hover', async () => {
    const { wrapper } = await mountWithRows([
      commitRow({ subject: 'A long subject', author: 'Ada Lovelace', authorAt: 0 }),
    ])

    for (const part of ['subject', 'author']) {
      const el = wrapper.get(`.row .${part}`)
      expect(el.attributes('title')).toBe(el.text())
    }
  })

  it('FR-UX-10 hovering the date shows its exact date and time, not the relative text again', async () => {
    vi.useFakeTimers({ now: new Date('2026-09-24T12:00:00Z') })
    const authorAt = Date.now() / 1000 - 3 * 24 * 3600

    const { wrapper } = await mountWithRows([commitRow({ authorAt })])

    const date = wrapper.get('.row .date')
    expect(date.text()).toBe('3 days ago')
    expect(date.attributes('title')).toBe(exactDate(authorAt))
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
    const refs = names.map((name) => ({ name, kind: RefKind.BranchRef }))
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
    const log = useLogStore()
    expect(rowEls.length).toBe(3)

    await rowEls[0]!.trigger('click', { ctrlKey: true })
    await rowEls[1]!.trigger('click', { ctrlKey: true })
    expect(log.selection.map((r) => r.sha)).toEqual(['sha-a', 'sha-b'])

    await rowEls[2]!.trigger('click')
    expect(log.selection.map((r) => r.sha)).toEqual(['sha-c'])
    expect(Facade.CommitDetails).toHaveBeenCalledWith('/repo', 'sha-c')
  })

  it('FR-LOG-13 shift-click the next row selects both, in order, and compares them', async () => {
    const { rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])
    const log = useLogStore()
    const compare = useCompareStore()

    await rowEls[0]!.trigger('click')
    await rowEls[1]!.trigger('click', { shiftKey: true })

    expect(log.selection.map((r) => r.sha)).toEqual(['sha-a', 'sha-b'])
    expect(compare.from?.sha).toBe('sha-a')
    expect(compare.to?.sha).toBe('sha-b')
    expect(rowEls[0]!.classes()).toContain('compare-from')
    expect(rowEls[1]!.classes()).toContain('compare-to')
  })

  it('FR-LOG-13 ctrl-click on the next row gives the same selection and comparison as a shift-click would', async () => {
    const { rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])
    const log = useLogStore()
    const compare = useCompareStore()

    await rowEls[0]!.trigger('click')
    await rowEls[1]!.trigger('click', { ctrlKey: true })

    expect(log.selection.map((r) => r.sha)).toEqual(['sha-a', 'sha-b'])
    expect(compare.from?.sha).toBe('sha-a')
    expect(compare.to?.sha).toBe('sha-b')
  })

  it('FR-LOG-13 shift-click three rows down selects the whole range in order, no comparison', async () => {
    const { rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c', 'sha-d'])
    const log = useLogStore()
    const compare = useCompareStore()

    await rowEls[0]!.trigger('click')
    await rowEls[3]!.trigger('click', { shiftKey: true })

    expect(log.selection.map((r) => r.sha)).toEqual(['sha-a', 'sha-b', 'sha-c', 'sha-d'])
    expect(compare.active).toBe(false)
  })

  it('FR-LOG-13 ctrl-click taking one commit out of a shift-click range keeps the rest, still no comparison', async () => {
    const { rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c', 'sha-d'])
    const log = useLogStore()

    await rowEls[0]!.trigger('click')
    await rowEls[3]!.trigger('click', { shiftKey: true })
    await rowEls[2]!.trigger('click', { ctrlKey: true })

    expect(log.selection.map((r) => r.sha)).toEqual(['sha-a', 'sha-b', 'sha-d'])
  })

  it('FR-CMP-05 a comparison marks the row clicked first compare-from, the one clicked second compare-to, whatever their log order', async () => {
    const shas = Array.from({ length: 8 }, (_, i) => `sha-${i}`)
    const { wrapper, rowEls } = await mountWithRows(shas)

    await rowEls[7]!.trigger('click', { ctrlKey: true }) // clicked first, despite the higher row index
    await rowEls[2]!.trigger('click', { ctrlKey: true })

    const classesOf = (i: number) => rowEls[i]!.classes()
    expect(classesOf(7)).toContain('compare-from')
    expect(classesOf(2)).toContain('compare-to')
    expect(wrapper.findAll('.row').some((r) => r.classes('selected'))).toBe(false)
  })

  it('FR-CMP-03 swap moves the log view marks to the other row', async () => {
    const { wrapper, rowEls } = await mountWithRows(['sha-a', 'sha-b'])
    const compare = useCompareStore()

    await rowEls[0]!.trigger('click', { ctrlKey: true }) // sha-a clicked first: from
    await rowEls[1]!.trigger('click', { ctrlKey: true }) // sha-b clicked second: to

    compare.swap()
    await wrapper.vm.$nextTick()

    const classesOf = (i: number) => rowEls[i]!.classes()
    expect(classesOf(0)).toContain('compare-to') // sha-a's mark moved
    expect(classesOf(1)).toContain('compare-from') // sha-b's mark moved
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
    expect(log.selection).toHaveLength(3)

    await openMenu(wrapper, rowEls[1]!)
    await clickMenuItem(body, 'Cherry-pick 3 commits')
    await flushPromises()

    expect(log.selection).toEqual([])
  })

  it("FR-CP-04 a rejection renders in LogList's own ErrorBanner and keeps the multi-select", async () => {
    Facade.CherryPick.mockRejectedValueOnce(
      new Error('boom', { cause: { message: 'would be overwritten by merge' } }),
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
    expect(log.selection).toHaveLength(3)
  })

  it('FR-LOG-11 "Revert" acts directly, the pre-existing call path', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b'])

    await rowEls[0]!.trigger('click')
    await openMenu(wrapper, rowEls[0]!)
    await clickMenuItem(body, 'Revert')

    expect(Facade.Revert).toHaveBeenCalledTimes(1)
    expect(Facade.Revert).toHaveBeenCalledWith('/repo', ['sha-a'], gitDefaults)
  })

  it("FR-CP-05 a merge commit's refusal shows in the error banner; no dialog opens", async () => {
    Facade.CherryPick.mockRejectedValueOnce(
      new Error('boom', {
        cause: {
          code: 'GIT_FAILED',
          key: 'mergeCommitNotPickable',
          args: { sha: 'sha-m', verb: 'cherry-pick' },
          message: '',
        },
      }),
    )
    const { wrapper, body, rowEls } = await mountWithRows([row('sha-m')])
    await openMenu(wrapper, rowEls[0]!)
    await clickMenuItem(body, 'Cherry-pick')
    await flushPromises()

    expect(Facade.CherryPick).toHaveBeenCalledTimes(1)
    expect(Facade.CherryPick).toHaveBeenCalledWith('/repo', ['sha-m'], gitDefaults)
    expect(body.find('[data-test="pick-submit"]').exists()).toBe(false)
    expect(wrapper.text()).toContain(
      'sha-m is a merge commit; cherry-pick does not support a merge commit',
    )
  })

  it('FR-CP-06 "Cherry-pick…" opens PickDialog with the current selection', async () => {
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

    expect(log.selection).toEqual([])
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

    const labels = menuLabels(body)
    expect(labels).toContain('Compare with sha-a')
    expect(log.selection.map((r) => r.sha)).toEqual(['sha-b'])
  })

  it('FR-CMP-01 choosing "Compare with <A short>" compares A and B, A (selected before) from', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b'])
    const log = useLogStore()
    const compare = useCompareStore()

    await rowEls[0]!.trigger('click')
    await openMenu(wrapper, rowEls[1]!)
    await clickMenuItem(body, 'Compare with sha-a')

    expect(log.selection.map((r) => r.sha)).toEqual(['sha-a', 'sha-b'])
    expect(compare.active).toBe(true)
    expect(compare.from?.sha).toBe('sha-a')
    expect(compare.to?.sha).toBe('sha-b')
  })

  it('FR-CMP-01 right-click with nothing selected yet shows no "Compare with …" item', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b'])

    await openMenu(wrapper, rowEls[0]!)

    const labels = menuLabels(body)
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

    const labels = menuLabels(body)
    expect(
      labels.some((l) => l !== 'Compare with working tree' && l.startsWith('Compare with ')),
    ).toBe(false)
  })

  it('FR-CMP-01 ctrl-click with nothing selected pairs the selected row with the ctrl-clicked one', async () => {
    const { rowEls } = await mountWithRows(['sha-a', 'sha-b'])
    const log = useLogStore()
    const compare = useCompareStore()

    await rowEls[0]!.trigger('click')
    await rowEls[1]!.trigger('click', { ctrlKey: true })

    expect(log.selection.map((r) => r.sha)).toEqual(['sha-a', 'sha-b'])
    expect(compare.active).toBe(true)
  })

  it('FR-LOG-13 ctrl-clicks add commits: three are all marked selected, with no comparison; one taken out leaves a comparison', async () => {
    const { rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])
    const log = useLogStore()

    await rowEls[0]!.trigger('click')
    await rowEls[1]!.trigger('click', { ctrlKey: true })
    await rowEls[2]!.trigger('click', { ctrlKey: true })

    expect(log.selection.map((r) => r.sha)).toEqual(['sha-a', 'sha-b', 'sha-c'])
    expect(rowEls.every((r) => r.classes('selected'))).toBe(true)
    expect(rowEls.some((r) => r.classes('compare-from') || r.classes('compare-to'))).toBe(false)

    await rowEls[2]!.trigger('click', { ctrlKey: true })

    expect(rowEls[0]!.classes()).toContain('compare-from')
    expect(rowEls[1]!.classes()).toContain('compare-to')
  })

  it('FR-LOG-13 ctrl-click after a shift-click range adds to the range', async () => {
    const { rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c', 'sha-d'])
    const log = useLogStore()

    await rowEls[0]!.trigger('click')
    await rowEls[1]!.trigger('click', { shiftKey: true })
    await rowEls[3]!.trigger('click', { ctrlKey: true })

    expect(log.selection.map((r) => r.sha)).toEqual(['sha-a', 'sha-b', 'sha-d'])
    expect(rowEls.map((r) => r.classes('selected'))).toEqual([true, true, false, true])
  })

  it('FR-LOG-13 ctrl-click after "Show in log" adds to the commit it selected', async () => {
    const { rowEls } = await mountWithRows(['A', 'B', 'C', 'D'])
    const log = useLogStore()
    await rowEls[0]!.trigger('click')
    Facade.LogIndexOf.mockResolvedValueOnce({ index: 1, found: true })
    await log.jumpTo('B')

    await rowEls[2]!.trigger('click', { ctrlKey: true })
    await rowEls[3]!.trigger('click', { ctrlKey: true })

    expect(log.selection.map((p) => p.sha)).toEqual(['B', 'C', 'D'])
  })

  it('FR-LOG-13 ctrl-click taking out one of two picked commits leaves the other selected and shown', async () => {
    const { rowEls } = await mountWithRows(['A', 'B'])
    const log = useLogStore()

    await rowEls[0]!.trigger('click', { ctrlKey: true })
    await rowEls[1]!.trigger('click', { ctrlKey: true })
    await rowEls[0]!.trigger('click', { ctrlKey: true })
    await flushPromises()

    expect(rowEls.map((r) => r.classes('selected'))).toEqual([false, true])
    expect(log.selectedRow?.sha).toBe('B')
  })

  it('FR-LOG-13 ctrl-click on the only selected commit takes it out: nothing stays selected', async () => {
    const { rowEls } = await mountWithRows(['sha-a', 'sha-b'])
    const log = useLogStore()

    await rowEls[0]!.trigger('click')
    await flushPromises()
    await rowEls[0]!.trigger('click', { ctrlKey: true })

    expect(rowEls.some((r) => r.classes('selected'))).toBe(false)
    expect(log.details).toBeNull()
  })

  it('FR-LOG-11 right-click inside three commits selected by shift-click opens the menu for three, no swap', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])

    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    await openMenu(wrapper, rowEls[1]!)

    expect(menuLabels(body)).toEqual([
      'Cherry-pick 3 commits',
      'Cherry-pick 3 commits…',
      'Revert 3 commits',
      'Revert 3 commits…',
      'Squash 3 commits…',
      'Fixup 3 commits…',
      'Copy 3 hashes',
    ])
    expect(Facade.SquashCheck).toHaveBeenCalledWith('/repo', ['sha-a', 'sha-b', 'sha-c'])
  })

  it('FR-LOG-11 right-click one of three commits selected by ctrl-click opens the same menu as a shift-click range', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c', 'sha-d'])
    await rowEls[0]!.trigger('click')
    await rowEls[1]!.trigger('click', { ctrlKey: true })
    await rowEls[3]!.trigger('click', { ctrlKey: true })

    await openMenu(wrapper, rowEls[3]!)

    expect(menuLabels(body)).toEqual([
      'Cherry-pick 3 commits',
      'Cherry-pick 3 commits…',
      'Revert 3 commits',
      'Revert 3 commits…',
      'Squash 3 commits…',
      'Fixup 3 commits…',
      'Copy 3 hashes',
    ])
    expect(Facade.SquashCheck).toHaveBeenCalledWith('/repo', ['sha-a', 'sha-b', 'sha-d'])
  })

  it('FR-LOG-14 a blocker replaces squash and fixup with one disabled item saying why', async () => {
    Facade.SquashCheck.mockResolvedValue({
      pushed: false,
      message: '',
      blocker: { code: 'GIT_FAILED', key: 'squashMergeInTheWay', args: { sha: 'm1' }, message: '' },
    })
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])
    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })

    await openMenu(wrapper, rowEls[1]!)

    const labels = menuLabels(body)
    expect(labels).toContain('Squash and fixup not possible: the merge commit m1 is in the way')
    expect(labels.some((l) => l.startsWith('Squash 3') || l.startsWith('Fixup 3'))).toBe(false)
  })

  it('FR-LOG-14 "Squash 3 commits…" opens the squash dialog with the proposed message', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])
    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    await openMenu(wrapper, rowEls[1]!)

    await clickMenuItem(body, 'Squash 3 commits…')
    await flushPromises()

    const field = body.get<HTMLTextAreaElement>('[data-test="squash-message"]')
    expect(field.element.value).toBe('A\n\nB')
  })

  it('FR-LOG-14 a fixup asks first, then clears the selection once done', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b', 'sha-c'])
    const log = useLogStore()
    await rowEls[0]!.trigger('click')
    await rowEls[2]!.trigger('click', { shiftKey: true })
    await openMenu(wrapper, rowEls[1]!)

    await clickMenuItem(body, 'Fixup 3 commits…')
    await flushPromises()
    expect(Facade.Fixup).not.toHaveBeenCalled()
    await body.get('[data-test="squash-submit"]').trigger('click')
    await flushPromises()

    expect(Facade.Fixup).toHaveBeenCalledWith('/repo', ['sha-a', 'sha-b', 'sha-c'])
    expect(log.selection).toEqual([])
    expect(body.find('[data-test="squash-submit"]').exists()).toBe(false)
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

  it('FR-LOG-11 right-click one of two commits selected by shift-click opens the menu for two, plus swap and stop comparing', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b'])
    const log = useLogStore()
    log.selection = [row('sha-a'), row('sha-b')]

    await openMenu(wrapper, rowEls[0]!)

    expect(menuLabels(body)).toEqual([
      'Cherry-pick 2 commits',
      'Cherry-pick 2 commits…',
      'Revert 2 commits',
      'Revert 2 commits…',
      'Squash 2 commits…',
      'Fixup 2 commits…',
      'Copy 2 hashes',
      'Swap',
      'Stop comparing',
    ])
  })

  it('FR-UX-05 FR-CMP-01 "Stop comparing" shows its shortcut; other entries show none', async () => {
    const { wrapper, body, rowEls } = await mountWithRows(['sha-a', 'sha-b'])
    const log = useLogStore()
    log.selection = [row('sha-a'), row('sha-b')]

    await openMenu(wrapper, rowEls[0]!)

    const items = body.findAll('.p-contextmenu-item-content')
    const stopComparing = items[menuLabels(body).indexOf('Stop comparing')]!
    expect(stopComparing.find('[data-test="shortcut-hint"]').text()).toBe('Esc')
    const swap = items[menuLabels(body).indexOf('Swap')]!
    expect(swap.find('[data-test="shortcut-hint"]').exists()).toBe(false)
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
    log.selection = [row('sha-a'), row('sha-b')]

    await openMenu(wrapper, rowEls[2]!)

    expect(compare.active).toBe(false)
    expect(log.selection.map((r) => r.sha)).toEqual(['sha-c'])
    const labels = menuLabels(body)
    expect(labels).toContain('Copy hash')
    expect(labels).not.toContain('Swap')
  })
})
