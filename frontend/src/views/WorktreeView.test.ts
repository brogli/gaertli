import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockSummary, mockCommandLog, mockLoadLog, mockLogRows, mockRefresh, mockRefs } = vi.hoisted(
  () => ({
    mockSummary: vi.fn(),
    mockCommandLog: vi.fn(),
    mockLoadLog: vi.fn(),
    mockLogRows: vi.fn(),
    mockRefresh: vi.fn(),
    mockRefs: vi.fn(),
  }),
)

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: {
    Summary: mockSummary,
    CommandLog: mockCommandLog,
    LoadLog: mockLoadLog,
    LogRows: mockLogRows,
    CommitDetails: vi.fn(),
    Refresh: mockRefresh,
    Refs: mockRefs,
  },
}))

import WorktreeView from './WorktreeView.vue'

function branch(fields: Record<string, unknown>) {
  return {
    name: 'main',
    sha: 'sha-main',
    current: true,
    upstream: '',
    ahead: 0,
    behind: 0,
    upstreamGone: false,
    checkedOutIn: '',
    ...fields,
  }
}

describe('WorktreeView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockSummary.mockReset()
    mockSummary.mockResolvedValue({
      path: '/home/user/project',
      branch: 'main',
      detached: false,
      operation: '',
    })
    mockCommandLog.mockReset()
    mockCommandLog.mockResolvedValue([])
    mockLoadLog.mockReset()
    mockLoadLog.mockResolvedValue(undefined)
    mockLogRows.mockReset()
    mockLogRows.mockResolvedValue({ rows: [], loaded: 0, done: true })
    mockRefresh.mockReset()
    mockRefresh.mockResolvedValue(undefined)
    mockRefs.mockReset()
    mockRefs.mockResolvedValue({ branches: [], remotes: [], tags: [] })
  })

  it('FR-WT-09 shows the worktree name from the path prop', () => {
    expect(mount(WorktreeView, { props: { path: '/home/user/project' } }).text()).toContain(
      'project',
    )
  })

  it('FR-UX-01 shows the error message and reveals gitStderr behind show details', async () => {
    mockSummary.mockRejectedValueOnce(
      Object.assign(new Error('boom'), {
        cause: { message: 'not a git repository', gitStderr: 'fatal: not a git repository' },
      }),
    )

    const wrapper = mount(WorktreeView, { props: { path: '/home/user/project' } })
    await flushPromises()

    expect(wrapper.text()).toContain('not a git repository')
    expect(wrapper.text()).not.toContain('fatal: not a git repository')

    await wrapper.get('.error-banner button').trigger('click')

    expect(wrapper.text()).toContain('fatal: not a git repository')
  })

  it('FR-REPO-04 clicking Refresh calls the Refresh binding with the worktree id', async () => {
    const wrapper = mount(WorktreeView, { props: { path: '/home/user/project' } })
    await flushPromises()

    const refreshButton = wrapper.findAll('button').find((b) => b.text() === 'Refresh')
    await refreshButton?.trigger('click')

    expect(mockRefresh).toHaveBeenCalledTimes(1)
    expect(mockRefresh).toHaveBeenCalledWith('/home/user/project')
  })

  it('FR-REPO-03 shows an ahead/behind indicator when the branch has an upstream', async () => {
    mockRefs.mockResolvedValue({
      branches: [branch({ upstream: 'origin/main', ahead: 2, behind: 1 })],
      remotes: [],
      tags: [],
    })

    const wrapper = mount(WorktreeView, { props: { path: '/home/user/project' } })
    await flushPromises()

    const indicator = wrapper.get('[data-test="ahead-behind"]')
    expect(indicator.text()).toContain('2')
    expect(indicator.text()).toContain('1')
  })

  it('FR-REPO-03 shows no ahead/behind indicator without an upstream', async () => {
    mockRefs.mockResolvedValue({ branches: [branch({})], remotes: [], tags: [] })

    const wrapper = mount(WorktreeView, { props: { path: '/home/user/project' } })
    await flushPromises()

    expect(wrapper.find('[data-test="ahead-behind"]').exists()).toBe(false)
  })
})
