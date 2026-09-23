import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'

const { mockFetch, mockPull, mockPush, mockForcePush, mockPullWithStash, mockRequire } = vi.hoisted(() => ({
  mockFetch: vi.fn(),
  mockPull: vi.fn(),
  mockPush: vi.fn(),
  mockForcePush: vi.fn(),
  mockPullWithStash: vi.fn(),
  mockRequire: vi.fn(),
}))

vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({
  Facade: {
    Fetch: mockFetch,
    Pull: mockPull,
    Push: mockPush,
    ForcePush: mockForcePush,
    PullWithStash: mockPullWithStash,
  },
}))

vi.mock('openvue/useconfirm', () => ({
  useConfirm: () => ({ require: mockRequire, close: vi.fn() }),
}))

// refsState is reactive so a test can update it after mount (FR-SYNC-03's
// late-arriving remotes) and have SyncControls' own computed/watch react,
// the same way the real Pinia store would.
const refsState = reactive<{ refs: Refs | undefined }>({ refs: undefined })
const mockSettings = vi.fn()

vi.mock('../stores/refs', () => ({
  useRefsStore: () => refsState,
}))
vi.mock('../stores/settings', () => ({
  useSettingsStore: () => ({ settings: mockSettings() }),
}))

import SyncControls from './SyncControls.vue'
import { useJobsStore } from '../stores/jobs'
import { Code, PullMode } from '../../bindings/github.com/brogli/nebegit/internal/core'
import type { JobInfo, Refs } from '../../bindings/github.com/brogli/nebegit/internal/core'
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
    localStorage.clear()
    mockFetch.mockReset().mockResolvedValue('job-1')
    mockPull.mockReset().mockResolvedValue('job-1')
    mockPush.mockReset().mockResolvedValue('job-1')
    mockForcePush.mockReset().mockResolvedValue('job-1')
    mockPullWithStash.mockReset().mockResolvedValue('job-1')
    mockRequire.mockReset()
    refsState.refs = { branches: [branchFixture()], remotes: [], tags: [] }
    mockSettings.mockReset().mockReturnValue(defaultSettingsFixture())
  })

  it('FR-SYNC-01 clicking Fetch calls Facade.Fetch with the worktree id and no remote', async () => {
    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })

    await findButton(wrapper, 'Fetch')!.trigger('click')

    expect(mockFetch).toHaveBeenCalledTimes(1)
    expect(mockFetch).toHaveBeenCalledWith('/repo', '')
  })

  it('FR-SYNC-02 defaults the pull-mode select from settings, showing Pull and Push with no remote picker', () => {
    refsState.refs = {
      branches: [branchFixture({ upstream: 'origin/main' })],
      remotes: [],
      tags: [],
    }
    mockSettings.mockReturnValue(defaultSettingsFixture({ defaultPullMode: 'rebase' }))

    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })

    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('rebase')
    expect(findButton(wrapper, 'Pull')).toBeTruthy()
    expect(findButton(wrapper, 'Push')).toBeTruthy()
    expect(findButton(wrapper, 'Push and set upstream')).toBeFalsy()
  })

  it('FR-SYNC-02 switching to merge then clicking Pull calls Facade.Pull with the chosen mode', async () => {
    refsState.refs = {
      branches: [branchFixture({ upstream: 'origin/main' })],
      remotes: [],
      tags: [],
    }
    mockSettings.mockReturnValue(defaultSettingsFixture({ defaultPullMode: 'rebase' }))

    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })
    await wrapper.get('select').setValue('merge')
    await findButton(wrapper, 'Pull')!.trigger('click')

    expect(mockPull).toHaveBeenCalledTimes(1)
    expect(mockPull).toHaveBeenCalledWith('/repo', 'merge')
  })

  it('FR-SYNC-03 shows a remote picker and "Push and set upstream" instead of a plain Push button without an upstream', () => {
    refsState.refs = {
      branches: [branchFixture({ upstream: '' })],
      remotes: [{ name: 'origin', branches: [] }],
      tags: [],
    }

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

  it('FR-SYNC-03 preselects "origin" among several remotes, enabling "Push and set upstream" right away', () => {
    refsState.refs = {
      branches: [branchFixture({ upstream: '' })],
      remotes: [{ name: 'upstream', branches: [] }, { name: 'origin', branches: [] }],
      tags: [],
    }

    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })

    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('origin')
    expect((findButton(wrapper, 'Push and set upstream')!.element as HTMLButtonElement).disabled).toBe(false)
  })

  it('FR-SYNC-03 preselects the first remote when refs with no "origin" arrive after mount', async () => {
    refsState.refs = { branches: [branchFixture({ upstream: '' })], remotes: [], tags: [] }
    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })

    refsState.refs = { branches: [branchFixture({ upstream: '' })], remotes: [{ name: 'fork', branches: [] }], tags: [] }
    await wrapper.vm.$nextTick()

    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('fork')
  })

  it('FR-SYNC-03 picking a remote then clicking "Push and set upstream" calls Facade.Push with that remote', async () => {
    refsState.refs = {
      branches: [branchFixture({ upstream: '' })],
      remotes: [{ name: 'origin', branches: [] }],
      tags: [],
    }

    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })
    await wrapper.get('select').setValue('origin')
    await findButton(wrapper, 'Push and set upstream')!.trigger('click')

    expect(mockPush).toHaveBeenCalledTimes(1)
    expect(mockPush).toHaveBeenCalledWith('/repo', 'origin')
  })

  it('FR-SYNC-06, FR-UX-03, QA-SAFE-01 force push on a protected branch confirms first, naming it protected', async () => {
    refsState.refs = { branches: [branchFixture({ name: 'main' })], remotes: [], tags: [] }
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
    refsState.refs = { branches: [branchFixture({ name: 'main' })], remotes: [], tags: [] }
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

  it('FR-SYNC-04 a push job failed PUSH_REJECTED also shows the rejection banner with Pull, Force push, and the error message', async () => {
    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })

    const job = rejectedPushJob(Code.PushRejected)
    job.error = { code: Code.PushRejected, message: 'The remote refused the push (hook declined).' }
    useJobsStore().upsert(job)
    await wrapper.vm.$nextTick()

    const banner = wrapper.get('[data-test="push-rejection"]')
    expect(banner.text()).toContain('hook declined')
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

  it('FR-SYNC-04 a later pull clears the rejection banner', async () => {
    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })
    const jobs = useJobsStore()
    jobs.upsert(rejectedPushJob(Code.NotFastForward))
    await wrapper.vm.$nextTick()

    jobs.upsert({ id: 'j2', kind: 'pull', state: 'done', progress: 100, message: '', startedAt: '2024-01-01T00:00:05Z' })
    await wrapper.vm.$nextTick()

    expect(wrapper.find('[data-test="push-rejection"]').exists()).toBe(false)
  })

  it('FR-SYNC-02 the last pull mode chosen is preselected again after a reload', async () => {
    refsState.refs = { branches: [branchFixture({ upstream: 'origin/main' })], remotes: [], tags: [] }
    const first = mount(SyncControls, { props: { wt: '/repo' } })
    await first.get('select').setValue(PullMode.PullRebase)
    first.unmount()

    const second = mount(SyncControls, { props: { wt: '/repo' } })

    expect((second.get('select').element as HTMLSelectElement).value).toBe(PullMode.PullRebase)
  })

  it('FR-SYNC-09 a pull job failed DIRTY_WORKTREE renders a "Stash and pull" button calling Facade.PullWithStash', async () => {
    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })

    useJobsStore().upsert({
      id: 'j1',
      kind: 'pull',
      state: 'failed',
      progress: 0,
      message: '',
      startedAt: '2024-01-01T00:00:00Z',
      error: { code: Code.DirtyWorktree, message: 'local changes in the way' },
    })
    await wrapper.vm.$nextTick()

    const banner = wrapper.get('[data-test="pull-blocked"]')
    expect(banner.text()).toContain('local changes in the way')
    const stashPull = findButton(wrapper, 'Stash and pull')
    expect(stashPull).toBeTruthy()

    await stashPull!.trigger('click')

    expect(mockPullWithStash).toHaveBeenCalledTimes(1)
    expect(mockPullWithStash).toHaveBeenCalledWith('/repo', PullMode.PullMerge)
  })

  it('FR-SYNC-09 a pull job failed for another reason renders no "Stash and pull" button', async () => {
    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })

    useJobsStore().upsert({
      id: 'j1',
      kind: 'pull',
      state: 'failed',
      progress: 0,
      message: '',
      startedAt: '2024-01-01T00:00:00Z',
      error: { code: Code.Conflicts, message: 'conflict' },
    })
    await wrapper.vm.$nextTick()

    expect(wrapper.find('[data-test="pull-blocked"]').exists()).toBe(false)
  })

  it('FR-SYNC-08, FR-UX-01 a fetch job failed AUTH_FAILED shows its sign-in message', async () => {
    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })

    useJobsStore().upsert({
      id: 'j1',
      kind: 'fetch',
      state: 'failed',
      progress: 0,
      message: '',
      startedAt: '2024-01-01T00:00:00Z',
      error: { code: Code.AuthFailed, message: 'Could not sign in to the remote.' },
    })
    await wrapper.vm.$nextTick()

    expect(wrapper.get('[data-test="sync-failure"]').text()).toContain('Could not sign in to the remote.')
  })

  it('FR-UX-01 a failure another banner explains, or a later successful job, shows no extra error', async () => {
    const wrapper = mount(SyncControls, { props: { wt: '/repo' } })
    const jobs = useJobsStore()
    const failedPull = (code: Code): JobInfo => ({
      id: 'j1',
      kind: 'pull',
      state: 'failed',
      progress: 0,
      message: '',
      startedAt: '2024-01-01T00:00:00Z',
      error: { code, message: code },
    })

    for (const code of [Code.Conflicts, Code.DirtyWorktree, Code.Cancelled]) {
      jobs.upsert(failedPull(code))
      await wrapper.vm.$nextTick()
      expect(wrapper.find('[data-test="sync-failure"]').exists()).toBe(false)
    }

    jobs.upsert(failedPull(Code.GitFailed))
    await wrapper.vm.$nextTick()
    expect(wrapper.get('[data-test="sync-failure"]').text()).toContain(Code.GitFailed)

    jobs.upsert({ id: 'j2', kind: 'fetch', state: 'done', progress: 100, message: '', startedAt: '2024-01-01T00:00:05Z' })
    await wrapper.vm.$nextTick()
    expect(wrapper.find('[data-test="sync-failure"]').exists()).toBe(false)
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

  it('FR-UX-05 Ctrl+T pulls, same as clicking Pull', async () => {
    refsState.refs = {
      branches: [branchFixture({ upstream: 'origin/main' })],
      remotes: [],
      tags: [],
    }
    mount(SyncControls, { props: { wt: '/repo' } })

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 't', ctrlKey: true }))
    await flushPromises()

    expect(mockPull).toHaveBeenCalledTimes(1)
  })

  it('FR-UX-05 Ctrl+Shift+K pushes, same as clicking Push', async () => {
    refsState.refs = {
      branches: [branchFixture({ upstream: 'origin/main' })],
      remotes: [],
      tags: [],
    }
    mount(SyncControls, { props: { wt: '/repo' } })

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'K', ctrlKey: true, shiftKey: true }))
    await flushPromises()

    expect(mockPush).toHaveBeenCalledTimes(1)
  })
})
