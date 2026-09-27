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

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade, AppService }))

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import Splitter from 'openvue/splitter'
import SplitterPanel from 'openvue/splitterpanel'
import CommitDetailView from '../CommitDetailView.vue'
import CompareView from '../CompareView.vue'
import CompareWorkingView from '../CompareWorkingView.vue'
import { useLogStore } from '@/stores/log'
import { useCompareStore } from '@/stores/compare'
import { commitRow } from '@/testing/fixtures/log-rows'
import { commitDetailsFixture } from '@/testing/fixtures/commitDetails'
import { exactDate } from '@/lib/format'

describe('CommitDetailView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
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
      commitDetailsFixture({
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

  it('FR-LOG-15 splits a commit, starting in the middle: message and fields above, changed files below', async () => {
    Facade.CommitDetails.mockResolvedValue(
      commitDetailsFixture({ files: [{ path: 'a.txt', status: 'M' }] }),
    )

    const wrapper = mount(CommitDetailView, { props: { wt: '/repo' } })
    const log = useLogStore()
    await log.select(commitRow())
    await flushPromises()

    const splitter = wrapper.getComponent(Splitter)
    const [message, files] = splitter.findAllComponents(SplitterPanel)
    expect([message!.props('size'), files!.props('size')]).toEqual([50, 50])
    expect(message!.find('.subject').text()).toBe('Fix the bug')
    expect(message!.find('.fields').exists()).toBe(true)
    expect(message!.find('.changed-files').exists()).toBe(false)
    expect(files!.findAll('.files li')).toHaveLength(1)
  })

  it('FR-LOG-04 a click only marks a file row; a double-click opens it in the diff window', async () => {
    Facade.CommitDetails.mockResolvedValue(
      commitDetailsFixture({
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

  it("FR-LOG-16 in the folder tree a double-click opens a commit's file in the diff window", async () => {
    Facade.CommitDetails.mockResolvedValue(
      commitDetailsFixture({ files: [{ path: 'src/b.txt', status: 'M' }] }),
    )

    const wrapper = mount(CommitDetailView, { props: { wt: '/repo' } })
    const log = useLogStore()
    await log.select(commitRow())
    await flushPromises()
    await wrapper.get('[data-test="file-tree-toggle"]').trigger('click')

    await wrapper
      .get('[role="treeitem"][aria-label="b.txt"] [data-pc-section="nodecontent"]')
      .trigger('dblclick')

    expect(AppService.OpenDiff).toHaveBeenCalledWith('/repo', {
      kind: 'commit',
      sha: 'abc123',
      path: 'src/b.txt',
      oldPath: '',
    })
  })

  it('FR-UX-10 labels Author and Committer with dates when they differ, and shows short-hash parents with the full SHA in title', async () => {
    Facade.CommitDetails.mockResolvedValue(
      commitDetailsFixture({
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

    const authorField = wrapper.findAll('dd').find((d) => d.text().includes('alice@example.com'))
    expect(authorField!.attributes('title')).toBe(exactDate(1_700_000_000))
    const committerField = wrapper.findAll('dd').find((d) => d.text().includes('bob@example.com'))
    expect(committerField!.attributes('title')).toBe(exactDate(1_700_000_100))
  })

  it('FR-UX-10 hides the Committer field when author and committer match', async () => {
    Facade.CommitDetails.mockResolvedValue(
      commitDetailsFixture({
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
    Facade.CommitDetails.mockResolvedValue(
      commitDetailsFixture({ files: [{ path: 'a.txt', status: 'M' }] }),
    )

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

    log.selection = [commitRow({ sha: 'sha-a' }), commitRow({ sha: 'sha-b' })]
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

  it("FR-LOG-13 says how many commits are selected instead of a commit's fields when three or more are picked", async () => {
    const wrapper = mount(CommitDetailView, { props: { wt: '/repo' } })
    const log = useLogStore()
    await log.select(commitRow())

    log.selection = ['sha-a', 'sha-b', 'sha-c'].map((sha) => commitRow({ sha }))
    await flushPromises()

    expect(wrapper.text()).toBe('3 commits selected')
  })
})
