import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { Facade, AppService } = vi.hoisted(() => ({
  Facade: {
    LoadLog: vi.fn(),
    LogRows: vi.fn(),
    CommitDetails: vi.fn(),
    OpenFile: vi.fn(),
    CompareCommits: vi.fn(),
    CompareWithWorking: vi.fn(),
    LogIndexOf: vi.fn().mockResolvedValue({ index: 0, found: false }),
  },
  AppService: { OpenDiff: vi.fn() },
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade, AppService }))

vi.mock('@wailsio/runtime', () => import('../testing/runtime'))

import CommitDetailView from './CommitDetailView.vue'
import CompareView from './CompareView.vue'
import CompareWorkingView from './CompareWorkingView.vue'
import { useLogStore } from '../stores/log'
import { useCompareStore } from '../stores/compare'
import { commitRow } from '../testing/fixtures/log-rows'
import type { CommitDetails } from '../../bindings/github.com/brogli/nebegit/internal/core'

// details: the canned CommitDetails shared by the tests below, overridable
// field by field.
function details(overrides: Partial<CommitDetails> = {}): CommitDetails {
  return {
    sha: 'abc123',
    parents: [],
    author: 'Alice',
    authorEmail: 'alice@example.com',
    authorAt: 0,
    committer: 'Bob',
    committerEmail: 'bob@example.com',
    committerAt: 0,
    body: 'Fix the bug',
    files: [],
    ...overrides,
  }
}

describe('CommitDetailView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.CompareCommits.mockReset().mockResolvedValue([])
    Facade.CompareWithWorking.mockReset().mockResolvedValue([])
    Facade.CommitDetails.mockReset()
    Facade.OpenFile.mockReset().mockResolvedValue(undefined)
    AppService.OpenDiff.mockReset()
  })

  it('FR-LOG-03 renders nothing before a selection', () => {
    const wrapper = mount(CommitDetailView, { props: { wt: '/repo' } })

    expect(wrapper.text()).toBe('')
  })

  it('FR-LOG-03 renders the message, author/committer, parents, and changed files once selected', async () => {
    Facade.CommitDetails.mockResolvedValue(
      details({
        parents: ['parent1', 'parent2'],
        body: 'Fix the bug\n\nSome more detail.',
        files: [
          { path: 'a.txt', status: 'M' },
          { path: 'b.txt', oldPath: 'a2.txt', status: 'R' },
        ],
      }),
    )

    const wrapper = mount(CommitDetailView, { props: { wt: '/repo' } })
    const log = useLogStore()
    await log.select(commitRow())
    await flushPromises()

    expect(wrapper.text()).toContain('Fix the bug')
    expect(wrapper.text()).toContain('Some more detail.')
    expect(wrapper.text()).toContain('Alice')
    expect(wrapper.text()).toContain('Bob')
    expect(wrapper.text()).toContain('parent1')
    expect(wrapper.text()).toContain('parent2')
    expect(wrapper.text()).toContain('a.txt')
    expect(wrapper.text()).toContain('b.txt')
  })

  it('FR-LOG-04 a click only marks a file row; a double-click opens it in the diff window', async () => {
    Facade.CommitDetails.mockResolvedValue(
      details({
        files: [
          { path: 'a.txt', status: 'M' },
          { path: 'b.txt', oldPath: 'a2.txt', status: 'R' },
        ],
      }),
    )

    const wrapper = mount(CommitDetailView, { props: { wt: '/repo' } })
    const log = useLogStore()
    await log.select(commitRow())
    await flushPromises()

    const file = wrapper.findAll('.files li')[1]!
    await file.trigger('click')
    expect(file.classes()).toContain('selected')
    expect(AppService.OpenDiff).not.toHaveBeenCalled()

    await file.trigger('dblclick')
    expect(AppService.OpenDiff).toHaveBeenCalledWith('/repo', {
      kind: 'commit',
      sha: 'abc123',
      path: 'b.txt',
      oldPath: 'a2.txt',
    })
  })

  it('FR-UX-10 labels Author and Committer with dates when they differ, and shows short-hash parents with the full SHA in title', async () => {
    Facade.CommitDetails.mockResolvedValue(
      details({
        sha: 'a'.repeat(40),
        parents: ['b'.repeat(40), 'c'.repeat(40)],
        authorAt: 1_700_000_000,
        committerAt: 1_700_000_100,
      }),
    )

    const wrapper = mount(CommitDetailView, { props: { wt: '/repo' } })
    const log = useLogStore()
    await log.select(commitRow())
    await flushPromises()

    expect(wrapper.text()).toContain('Author')
    expect(wrapper.text()).toContain('Committer')
    expect(wrapper.find('li').exists()).toBe(false)
    const parents = wrapper.findAll('.parents [title]')
    expect(parents).toHaveLength(2)
    expect(parents[0]!.text()).toBe('b'.repeat(7))
    expect(parents[0]!.attributes('title')).toBe('b'.repeat(40))
  })

  it('FR-UX-10 hides the Committer field when author and committer match', async () => {
    Facade.CommitDetails.mockResolvedValue(
      details({
        sha: 'a'.repeat(40),
        authorAt: 1_700_000_000,
        committer: 'Alice',
        committerEmail: 'alice@example.com',
        committerAt: 1_700_000_000,
      }),
    )

    const wrapper = mount(CommitDetailView, { props: { wt: '/repo' } })
    const log = useLogStore()
    await log.select(commitRow())
    await flushPromises()

    expect(wrapper.text()).toContain('Author')
    expect(wrapper.text()).not.toContain('Committer')
  })

  it('FR-ED-02 an open-editor event from the file list calls Facade.OpenFile with wt, the path, and line 0', async () => {
    Facade.CommitDetails.mockResolvedValue(details({ files: [{ path: 'a.txt', status: 'M' }] }))

    const wrapper = mount(CommitDetailView, { props: { wt: '/repo' } })
    const log = useLogStore()
    await log.select(commitRow())
    await flushPromises()

    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(Facade.OpenFile).toHaveBeenCalledTimes(1)
    expect(Facade.OpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 0)
  })

  it("FR-CMP-02 shows the comparison instead of the selected commit's fields once two commits are compared", async () => {
    const wrapper = mount(CommitDetailView, { props: { wt: '/repo' } })
    const log = useLogStore()
    await log.select(commitRow())
    const compare = useCompareStore()

    compare.toggleSelect('/repo', commitRow({ sha: 'sha-a' }), 0)
    compare.toggleSelect('/repo', commitRow({ sha: 'sha-b' }), 1)
    await flushPromises()

    expect(wrapper.findComponent(CompareView).exists()).toBe(true)
    expect(wrapper.find('.commit-details').exists()).toBe(false)
  })

  it("FR-CMP-04 shows the working-tree comparison instead of the selected commit's fields while one is active", async () => {
    const wrapper = mount(CommitDetailView, { props: { wt: '/repo' } })
    const log = useLogStore()
    await log.select(commitRow())
    const compare = useCompareStore()

    compare.startAgainstWorking(commitRow({ sha: 'sha-a' }))
    await flushPromises()

    expect(wrapper.findComponent(CompareWorkingView).exists()).toBe(true)
    expect(wrapper.find('.commit-details').exists()).toBe(false)
  })
})
