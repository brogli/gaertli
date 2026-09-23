import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockCommitDetails, mockOpenFile } = vi.hoisted(() => ({
  mockCommitDetails: vi.fn(),
  mockOpenFile: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { LoadLog: vi.fn(), LogRows: vi.fn(), CommitDetails: mockCommitDetails, OpenFile: mockOpenFile },
}))

vi.mock('@wailsio/runtime', () => ({
  Events: { On: vi.fn(() => () => {}) },
}))

import CommitDetailsPanel from './CommitDetailsPanel.vue'
import { useLogStore } from '../stores/log'
import { commitRow } from '../testing/fixtures/log-rows'

describe('CommitDetailsPanel', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockCommitDetails.mockReset()
    mockOpenFile.mockReset()
    mockOpenFile.mockResolvedValue(undefined)
  })

  it('FR-LOG-03 renders nothing before a selection', () => {
    const wrapper = mount(CommitDetailsPanel, { props: { wt: '/repo' } })

    expect(wrapper.text()).toBe('')
  })

  it('FR-LOG-03 renders the message, author/committer, parents, and changed files once selected', async () => {
    mockCommitDetails.mockResolvedValue({
      sha: 'abc123',
      parents: ['parent1', 'parent2'],
      author: 'Alice',
      authorEmail: 'alice@example.com',
      authorAt: 0,
      committer: 'Bob',
      committerEmail: 'bob@example.com',
      committerAt: 0,
      body: 'Fix the bug\n\nSome more detail.',
      files: [
        { path: 'a.txt', status: 'M' },
        { path: 'b.txt', oldPath: 'a2.txt', status: 'R' },
      ],
    })

    const wrapper = mount(CommitDetailsPanel, { props: { wt: '/repo' } })
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

  it('FR-LOG-04 clicking a file row selects it in the log store', async () => {
    mockCommitDetails.mockResolvedValue({
      sha: 'abc123',
      parents: [],
      author: 'Alice',
      authorEmail: 'alice@example.com',
      authorAt: 0,
      committer: 'Bob',
      committerEmail: 'bob@example.com',
      committerAt: 0,
      body: 'Fix the bug',
      files: [
        { path: 'a.txt', status: 'M' },
        { path: 'b.txt', oldPath: 'a2.txt', status: 'R' },
      ],
    })

    const wrapper = mount(CommitDetailsPanel, { props: { wt: '/repo' } })
    const log = useLogStore()
    await log.select(commitRow())
    await flushPromises()

    await wrapper.get('.files li').trigger('click')

    expect(log.selectedFile).toEqual({ path: 'a.txt', status: 'M' })
  })

  it('FR-UX-10 labels Author and Committer with dates when they differ, and shows short-hash parents with the full SHA in title', async () => {
    mockCommitDetails.mockResolvedValue({
      sha: 'a'.repeat(40),
      parents: ['b'.repeat(40), 'c'.repeat(40)],
      author: 'Alice',
      authorEmail: 'alice@example.com',
      authorAt: 1_700_000_000,
      committer: 'Bob',
      committerEmail: 'bob@example.com',
      committerAt: 1_700_000_100,
      body: 'Fix the bug',
      files: [],
    })

    const wrapper = mount(CommitDetailsPanel, { props: { wt: '/repo' } })
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
    mockCommitDetails.mockResolvedValue({
      sha: 'a'.repeat(40),
      parents: [],
      author: 'Alice',
      authorEmail: 'alice@example.com',
      authorAt: 1_700_000_000,
      committer: 'Alice',
      committerEmail: 'alice@example.com',
      committerAt: 1_700_000_000,
      body: 'Fix the bug',
      files: [],
    })

    const wrapper = mount(CommitDetailsPanel, { props: { wt: '/repo' } })
    const log = useLogStore()
    await log.select(commitRow())
    await flushPromises()

    expect(wrapper.text()).toContain('Author')
    expect(wrapper.text()).not.toContain('Committer')
  })

  it('FR-ED-02 an open-editor event from the file list calls Facade.OpenFile with wt, the path, and line 0', async () => {
    mockCommitDetails.mockResolvedValue({
      sha: 'abc123',
      parents: [],
      author: 'Alice',
      authorEmail: 'alice@example.com',
      authorAt: 0,
      committer: 'Bob',
      committerEmail: 'bob@example.com',
      committerAt: 0,
      body: 'Fix the bug',
      files: [{ path: 'a.txt', status: 'M' }],
    })

    const wrapper = mount(CommitDetailsPanel, { props: { wt: '/repo' } })
    const log = useLogStore()
    await log.select(commitRow())
    await flushPromises()

    await wrapper.get('[data-test="open-editor"]').trigger('click')

    expect(mockOpenFile).toHaveBeenCalledTimes(1)
    expect(mockOpenFile).toHaveBeenCalledWith('/repo', 'a.txt', 0)
  })
})
