import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockSummary, mockCommandLog } = vi.hoisted(() => ({
  mockSummary: vi.fn(),
  mockCommandLog: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { Summary: mockSummary, CommandLog: mockCommandLog },
}))

import WorktreeView from './WorktreeView.vue'

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
})
