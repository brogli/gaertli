import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockCommitDetails } = vi.hoisted(() => ({ mockCommitDetails: vi.fn() }))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { LoadLog: vi.fn(), LogRows: vi.fn(), CommitDetails: mockCommitDetails },
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
  })

  it('FR-LOG-03 renders nothing before a selection', () => {
    const wrapper = mount(CommitDetailsPanel)

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

    const wrapper = mount(CommitDetailsPanel)
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

    const wrapper = mount(CommitDetailsPanel)
    const log = useLogStore()
    await log.select(commitRow())
    await flushPromises()

    await wrapper.get('.files li').trigger('click')

    expect(log.selectedFile).toEqual({ path: 'a.txt', status: 'M' })
  })
})
