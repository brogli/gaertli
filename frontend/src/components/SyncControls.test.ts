import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { mockFetch, mockPull, mockPush, mockForcePush, mockRequire } = vi.hoisted(() => ({
  mockFetch: vi.fn(),
  mockPull: vi.fn(),
  mockPush: vi.fn(),
  mockForcePush: vi.fn(),
  mockRequire: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: { Fetch: mockFetch, Pull: mockPull, Push: mockPush, ForcePush: mockForcePush },
}))

vi.mock('openvue/useconfirm', () => ({
  useConfirm: () => ({ require: mockRequire, close: vi.fn() }),
}))

const mockRefs = vi.fn()
const mockSettings = vi.fn()

vi.mock('../stores/refs', () => ({
  useRefsStore: () => ({ refs: mockRefs() }),
}))
vi.mock('../stores/settings', () => ({
  useSettingsStore: () => ({ settings: mockSettings() }),
}))

import SyncControls from './SyncControls.vue'
import { useJobsStore } from '../stores/jobs'
import { Code } from '../../bindings/github.com/brogli/nebegit/internal/core'
import type { JobInfo } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { defaultSettingsFixture } from '../testing/fixtures/settings'
import { branchFixture } from '../testing/fixtures/branch'

function findButton(wrapper: VueWrapper, text: string) {
  return wrapper.findAll('button').find((b) => b.text() === text)
}

// rejectedPushJob is a failed push/force-push job, ready to seed the jobs
// store directly (JobsStrip.vue is what normally turns a "job:update"
// event into this).
function rejectedPushJob(code: Code, startedAt = '2024-01-01T00:00:00Z'): JobInfo {
  return {
    id: 'j1',
    kind: 'push',
    state: 'failed',
    progress: 0,
    message: '',
    startedAt,
    error: { code, message: 'Updates were rejected' },
  }
}

describe('SyncControls', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    mockFetch.mockReset().mockResolvedValue('job-1')
    mockPull.mockReset().mockResolvedValue('job-1')
    mockPush.mockReset().mockResolvedValue('job-1')
    mockForcePush.mockReset().mockResolvedValue('job-1')
    mockRequire.mockReset()
    mockRefs.mockReset().mockReturnValue({ branches: [branchFixture()], remotes: [], tags: [] })
    mockSettings.mockReset().mockReturnValue(defaultSettingsFixture())
  })

  it('FR-SYNC-01 clicking Fetch calls Facade.Fetch with the worktree id and no remote', async () => {
    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })

    await findButton(wrapper, 'Fetch')!.trigger('click')

    expect(mockFetch).toHaveBeenCalledTimes(1)
    expect(mockFetch).toHaveBeenCalledWith('/repo', '')
  })

  it('FR-SYNC-02 defaults the pull-mode select from settings, showing Pull and Push with no remote picker', () => {
    mockRefs.mockReturnValue({
      branches: [branchFixture({ upstream: 'origin/main' })],
      remotes: [],
      tags: [],
    })
    mockSettings.mockReturnValue(defaultSettingsFixture({ defaultPullMode: 'rebase' }))

    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })

    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('rebase')
    expect(findButton(wrapper, 'Pull')).toBeTruthy()
    expect(findButton(wrapper, 'Push')).toBeTruthy()
    expect(findButton(wrapper, 'Push and set upstream')).toBeFalsy()
  })

  it('FR-SYNC-02 switching to merge then clicking Pull calls Facade.Pull with the chosen mode', async () => {
    mockRefs.mockReturnValue({
      branches: [branchFixture({ upstream: 'origin/main' })],
      remotes: [],
      tags: [],
    })
    mockSettings.mockReturnValue(defaultSettingsFixture({ defaultPullMode: 'rebase' }))

    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })
    await wrapper.get('select').setValue('merge')
    await findButton(wrapper, 'Pull')!.trigger('click')

    expect(mockPull).toHaveBeenCalledTimes(1)
    expect(mockPull).toHaveBeenCalledWith('/repo', 'merge')
  })

  it('FR-SYNC-03 shows a remote picker and "Push and set upstream" instead of a plain Push button without an upstream', () => {
    mockRefs.mockReturnValue({
      branches: [branchFixture({ upstream: '' })],
      remotes: [{ name: 'origin', branches: [] }],
      tags: [],
    })

    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })

    expect(
      wrapper
        .get('select')
        .findAll('option')
        .map((o) => o.text()),
    ).toEqual(['origin'])
    expect(findButton(wrapper, 'Push and set upstream')).toBeTruthy()
    expect(findButton(wrapper, 'Push')).toBeFalsy()
  })

  it('FR-SYNC-03 picking a remote then clicking "Push and set upstream" calls Facade.Push with that remote', async () => {
    mockRefs.mockReturnValue({
      branches: [branchFixture({ upstream: '' })],
      remotes: [{ name: 'origin', branches: [] }],
      tags: [],
    })

    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })
    await wrapper.get('select').setValue('origin')
    await findButton(wrapper, 'Push and set upstream')!.trigger('click')

    expect(mockPush).toHaveBeenCalledTimes(1)
    expect(mockPush).toHaveBeenCalledWith('/repo', 'origin')
  })

  it('FR-SYNC-06, FR-UX-03, QA-SAFE-01 force push on a protected branch confirms first, naming it protected', async () => {
    mockRefs.mockReturnValue({ branches: [branchFixture({ name: 'main' })], remotes: [], tags: [] })
    mockSettings.mockReturnValue(defaultSettingsFixture({ protectedBranches: ['main'] }))
    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })
    useJobsStore().upsert(rejectedPushJob(Code.NotFastForward))
    await wrapper.vm.$nextTick()

    await findButton(wrapper, 'Force push')!.trigger('click')

    expect(mockRequire).toHaveBeenCalledTimes(1)
    const options = mockRequire.mock.calls[0]![0] as { message: string; accept: () => void }
    expect(options.message).toContain('protected')
    expect(mockForcePush).not.toHaveBeenCalled()

    options.accept()

    expect(mockForcePush).toHaveBeenCalledTimes(1)
  })

  it('FR-SYNC-06 force push on an unprotected branch confirms with no "protected" text', async () => {
    mockRefs.mockReturnValue({ branches: [branchFixture({ name: 'main' })], remotes: [], tags: [] })
    mockSettings.mockReturnValue(defaultSettingsFixture({ protectedBranches: [] }))
    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })
    useJobsStore().upsert(rejectedPushJob(Code.NotFastForward))
    await wrapper.vm.$nextTick()

    await findButton(wrapper, 'Force push')!.trigger('click')

    const options = mockRequire.mock.calls[0]![0] as { message: string }
    expect(options.message).not.toContain('protected')
  })

  it('FR-SYNC-04 a failed push job shows a rejection banner with Pull, Force push, and the error message', async () => {
    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })

    useJobsStore().upsert(rejectedPushJob(Code.NotFastForward))
    await wrapper.vm.$nextTick()

    const banner = wrapper.get('[data-test="push-rejection"]')
    expect(banner.text()).toContain('Updates were rejected')
    expect(findButton(wrapper, 'Pull')).toBeTruthy()
    expect(findButton(wrapper, 'Force push')).toBeTruthy()
  })

  it('FR-SYNC-04 a running push job clears an active rejection banner', async () => {
    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })
    const jobs = useJobsStore()
    jobs.upsert(rejectedPushJob(Code.NotFastForward))
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-test="push-rejection"]').exists()).toBe(true)

    jobs.upsert({
      id: 'j1',
      kind: 'push',
      state: 'running',
      progress: 10,
      message: '',
      startedAt: '2024-01-01T00:00:01Z',
    })
    await wrapper.vm.$nextTick()

    expect(wrapper.find('[data-test="push-rejection"]').exists()).toBe(false)
  })

  it('FR-UX-01 a rejected Fetch renders ErrorBanner with the error', async () => {
    mockFetch.mockRejectedValueOnce(
      Object.assign(new Error('boom'), { cause: { message: 'network unreachable' } }),
    )
    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })

    await findButton(wrapper, 'Fetch')!.trigger('click')
    await flushPromises()

    expect(wrapper.get('.error-banner').text()).toContain('network unreachable')
  })
})
