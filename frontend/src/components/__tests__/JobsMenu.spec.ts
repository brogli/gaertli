// Requirements: FR-SYNC-07, FR-UX-01, FR-UX-09, FR-UX-12
import { flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ProgressBar from 'openvue/progressbar'
import Badge from 'openvue/badge'
import { JobKind, JobState } from '../../../bindings/github.com/brogli/nebegit/internal/core'

const { Facade } = vi.hoisted(() => ({
  Facade: { Jobs: vi.fn(), CancelJob: vi.fn() },
}))

vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))
import { wailsError } from '@/testing/errors'
import { Events, emit } from '@/testing/runtime'

import JobsMenu from '../JobsMenu.vue'
import ErrorBanner from '../ErrorBanner.vue'
import { mountInBody } from '@/testing/mountInBody'
import { jobFixture } from '@/testing/fixtures/job'

function mountMenu() {
  return mountInBody(JobsMenu, { props: { wt: '/repo' } })
}

describe('JobsMenu', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Facade.Jobs.mockReset()
    Facade.CancelJob.mockReset().mockResolvedValue(undefined)
    Events.On.mockReset()
  })

  it('no jobs: the button is present with the idle tooltip, no badge', async () => {
    Facade.Jobs.mockResolvedValue([])
    const { wrapper, body } = await mountMenu()

    const button = body.get('[data-test="jobs-menu"]')
    expect(button.attributes('title')).toBe('No running jobs')
    expect(wrapper.findComponent(Badge).exists()).toBe(false)
  })

  it('the trigger is an OpenVue Button, secondary and outlined', async () => {
    Facade.Jobs.mockResolvedValue([])
    const { body } = await mountMenu()

    const classes = body.get('[data-test="jobs-menu"]').classes()
    expect(classes).toContain('p-button')
    expect(classes).toContain('p-button-secondary')
    expect(classes).toContain('p-button-outlined')
  })

  it('opening the idle list shows its one line', async () => {
    Facade.Jobs.mockResolvedValue([])
    const { body } = await mountMenu()

    await body.get('[data-test="jobs-menu"]').trigger('click')

    expect(body.text()).toContain('No running jobs')
  })

  it('two running jobs: the badge shows 2 and the list has two rows with label, message and a progress bar', async () => {
    Facade.Jobs.mockResolvedValue([
      jobFixture({ message: 'halfway' }),
      jobFixture({ id: 'j2', kind: JobKind.PushJob, progress: -1 }),
    ])
    const { wrapper, body } = await mountMenu()

    const button = body.get('[data-test="jobs-menu"]')
    expect(button.attributes('title')).toBe('2 running jobs')
    expect(wrapper.findComponent(Badge).props('value')).toBe('2')

    await button.trigger('click')
    const rows = body.findAll('[data-test="job-row"]')
    expect(rows).toHaveLength(2)
    expect(rows[0]!.text()).toContain('Fetch')
    expect(rows[0]!.text()).toContain('halfway')
    expect(wrapper.findAllComponents(ProgressBar)[0]!.props('mode')).toBe('determinate')
    expect(wrapper.findAllComponents(ProgressBar)[1]!.props('mode')).toBe('indeterminate')
  })

  it("an update-branch job's row names the branch, not the raw kind", async () => {
    Facade.Jobs.mockResolvedValue([
      jobFixture({ kind: JobKind.UpdateBranchJob, branch: 'feature', progress: 40 }),
    ])
    const { body } = await mountMenu()
    await body.get('[data-test="jobs-menu"]').trigger('click')

    expect(body.get('[data-test="job-row"]').text()).toContain('Update feature')
  })

  it("clicking a row's abort button calls CancelJob(id)", async () => {
    Facade.Jobs.mockResolvedValue([jobFixture()])
    const { body } = await mountMenu()
    await body.get('[data-test="jobs-menu"]').trigger('click')

    await body.get('[data-test="job-cancel"]').trigger('click')

    expect(Facade.CancelJob).toHaveBeenCalledWith('/repo', 'j1')
  })

  it('a cancelled job:update removes its row and the badge', async () => {
    Facade.Jobs.mockResolvedValue([jobFixture()])
    const { wrapper, body } = await mountMenu()

    emit('job:update', { data: jobFixture({ state: JobState.JobCancelled }) })
    await flushPromises()

    expect(wrapper.findComponent(Badge).exists()).toBe(false)
    expect(body.get('[data-test="jobs-menu"]').attributes('title')).toBe('No running jobs')
  })

  it('a rejected cancel renders ErrorBanner with the error, in the list', async () => {
    Facade.Jobs.mockResolvedValue([jobFixture()])
    Facade.CancelJob.mockRejectedValueOnce(wailsError('cancel failed'))
    const { wrapper, body } = await mountMenu()
    await body.get('[data-test="jobs-menu"]').trigger('click')

    await body.get('[data-test="job-cancel"]').trigger('click')
    await flushPromises()

    const error = wrapper.findComponent(ErrorBanner)
    expect(error.exists()).toBe(true)
    expect(error.props('error')).toEqual({ message: 'cancel failed' })
  })

  it("another window's job:update shows no job, so none can be cancelled", async () => {
    Facade.Jobs.mockResolvedValue([])
    const { body } = await mountMenu()

    emit('job:update', { sender: 'window-2', data: jobFixture({ id: 'j9', progress: 10 }) })
    await flushPromises()

    expect(body.get('[data-test="jobs-menu"]').attributes('title')).toBe('No running jobs')
  })
})
