import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ProgressBar from 'openvue/progressbar'

const { mockJobs, mockCancelJob, mockEventsOn } = vi.hoisted(() => ({
  mockJobs: vi.fn(),
  mockCancelJob: vi.fn(),
  mockEventsOn: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { Jobs: mockJobs, CancelJob: mockCancelJob },
}))

vi.mock('@wailsio/runtime', () => ({
  Events: { On: mockEventsOn },
}))

import JobsStrip from './JobsStrip.vue'

describe('JobsStrip', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockJobs.mockReset()
    mockCancelJob.mockReset().mockResolvedValue(undefined)
    mockEventsOn.mockReset()
    mockEventsOn.mockReturnValue(() => {})
  })

  it('QA-RESP-01 renders an indeterminate progress indicator with no numeric percent for a job with no progress yet', async () => {
    mockJobs.mockResolvedValue([
      { id: 'j1', kind: 'fetch', state: 'running', progress: -1, message: '', startedAt: '' },
    ])
    const wrapper = mount(JobsStrip, { props: { wt: '/repo' } })
    await flushPromises()

    const bar = wrapper.findComponent(ProgressBar)
    expect(bar.props('mode')).toBe('indeterminate')
    expect(wrapper.text()).not.toMatch(/\d+%/)
  })

  it('FR-SYNC-07 Cancel calls Facade.CancelJob, and a cancelled job:update removes the row', async () => {
    mockJobs.mockResolvedValue([
      { id: 'j1', kind: 'fetch', state: 'running', progress: 45, message: 'halfway', startedAt: '' },
    ])
    const wrapper = mount(JobsStrip, { props: { wt: '/repo' } })
    await flushPromises()

    const bar = wrapper.findComponent(ProgressBar)
    expect(bar.props('mode')).toBe('determinate')
    expect(bar.props('value')).toBe(45)

    await wrapper.get('[data-test="job-cancel"]').trigger('click')
    expect(mockCancelJob).toHaveBeenCalledWith('/repo', 'j1')

    const handler = mockEventsOn.mock.calls.find((call) => call[0] === 'job:update')?.[1]
    handler?.({
      data: { id: 'j1', kind: 'fetch', state: 'cancelled', progress: 45, message: 'halfway', startedAt: '' },
    })
    await wrapper.vm.$nextTick()

    expect(wrapper.findAll('.job-row')).toHaveLength(0)
  })
})
