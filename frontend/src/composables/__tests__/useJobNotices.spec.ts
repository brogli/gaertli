import { flushPromises, mount, type DOMWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, nextTick } from 'vue'

const { Facade } = vi.hoisted(() => ({ Facade: { Jobs: vi.fn() } }))
vi.mock('../../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

vi.mock('@wailsio/runtime', () => import('@/testing/runtime'))

import { useJobNotices } from '../useJobNotices'
import { useJobsStore } from '@/stores/jobs'
import { useRefsStore } from '@/stores/refs'
import {
  Code,
  JobKind,
  JobState,
  UpdateOutcome,
  type JobInfo,
} from '../../../bindings/github.com/brogli/nebegit/internal/core'
import NoticeArea from '@/components/NoticeArea.vue'
import { mountInBody } from '@/testing/mountInBody'
import { findButton } from '@/testing/findButton'
import { jobFixture } from '@/testing/fixtures/job'
import { branchFixture } from '@/testing/fixtures/branch'

// setUpstream gives the repo one current branch with an upstream.
function setUpstream(): void {
  useRefsStore().refs = {
    branches: [branchFixture({ upstream: 'origin/main' })],
    remotes: [{ name: 'origin', branches: [] }],
    tags: [],
  }
}

function rejectedPushJob(code: Code): JobInfo {
  return jobFixture({
    kind: JobKind.PushJob,
    state: JobState.JobFailed,
    message: 'Updates were rejected',
    error: { code, message: 'Updates were rejected' },
  })
}

const blockedPullJob = jobFixture({
  kind: JobKind.PullJob,
  state: JobState.JobFailed,
  error: { code: Code.DirtyWorktree, message: 'local changes in the way' },
})
const failedFetchJob = jobFixture({
  state: JobState.JobFailed,
  error: { code: Code.AuthFailed, message: 'Could not sign in to the remote.' },
})
const upToDate = {
  outcome: UpdateOutcome.BranchUpToDate,
  from: 'abc1234full',
  to: 'abc1234full',
  commits: 0,
}

// branchUpdateJob is a branch update of feature that ended up to date.
function branchUpdateJob(overrides: Partial<JobInfo> = {}): JobInfo {
  return jobFixture({
    kind: JobKind.UpdateBranchJob,
    branch: 'feature',
    state: JobState.JobDone,
    progress: 100,
    result: upToDate,
    ...overrides,
  })
}

// Host mounts useJobNotices with recorded actions, as SyncControls.vue
// does: the composable only decides which notice to show, the caller keeps
// the pull-mode/confirm logic behind each action.
const onPull = vi.fn()
const onForcePush = vi.fn()
const onStashPull = vi.fn()
const onRetryJob = vi.fn()
const onConflicts = vi.fn()

const Host = defineComponent({
  setup() {
    useJobNotices({ onPull, onForcePush, onStashPull, onRetryJob, onConflicts })
    return () => null
  },
})

// mountNotices mounts the notice area and Host, returning the notice area.
async function mountNotices(): Promise<DOMWrapper<Element>> {
  const { body } = await mountInBody(NoticeArea)
  mount(Host)
  return body
}

describe('useJobNotices', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    onPull.mockReset()
    onForcePush.mockReset()
    onStashPull.mockReset()
    onRetryJob.mockReset()
    onConflicts.mockReset()
  })

  it('FR-SYNC-04 a failed push job shows a notice with the reason and Pull/Force push, running the actions', async () => {
    const notice = await mountNotices()
    setUpstream()

    useJobsStore().upsert(rejectedPushJob(Code.NotFastForward))
    await nextTick()

    expect(notice.text()).toContain('Updates were rejected')
    await findButton(notice, 'Force push')!.trigger('click')
    expect(onForcePush).toHaveBeenCalledTimes(1)
  })

  it('FR-SYNC-04 clicking Pull in the rejection notice runs the Pull action', async () => {
    const notice = await mountNotices()
    setUpstream()

    useJobsStore().upsert(rejectedPushJob(Code.PushRejected))
    await nextTick()

    await findButton(notice, 'Pull')!.trigger('click')
    expect(onPull).toHaveBeenCalledTimes(1)
  })

  it('FR-SYNC-04 a rejected push on a branch without upstream shows the error and no Pull or Force push', async () => {
    const notice = await mountNotices()
    useRefsStore().refs = { branches: [branchFixture()], remotes: [], tags: [] }

    useJobsStore().upsert(rejectedPushJob(Code.NotFastForward))
    await nextTick()

    expect(notice.text()).toContain('Updates were rejected')
    expect(findButton(notice, 'Pull')).toBeUndefined()
    expect(findButton(notice, 'Force push')).toBeUndefined()
  })

  it('FR-SYNC-04 a later pull clears the rejection notice', async () => {
    const notice = await mountNotices()
    const jobs = useJobsStore()

    jobs.upsert(rejectedPushJob(Code.NotFastForward))
    await nextTick()
    expect(notice.find('.p-toast-message').exists()).toBe(true)

    jobs.upsert(
      jobFixture({ id: 'j2', kind: JobKind.PullJob, state: JobState.JobDone, progress: 100 }),
    )
    await nextTick()

    expect(notice.find('.p-toast-message').exists()).toBe(false)
  })

  it('FR-SYNC-09 a pull job failed DIRTY_WORKTREE shows "Stash and pull", running PullWithStash', async () => {
    const notice = await mountNotices()

    useJobsStore().upsert(blockedPullJob)
    await nextTick()

    expect(notice.text()).toContain('local changes in the way')
    await findButton(notice, 'Stash and pull')!.trigger('click')
    expect(onStashPull).toHaveBeenCalledTimes(1)
  })

  it('FR-CONF-09 a pull job that stopped on conflicts opens the conflicts, with no notice', async () => {
    const notice = await mountNotices()

    useJobsStore().upsert(
      jobFixture({
        kind: JobKind.PullJob,
        state: JobState.JobFailed,
        error: { code: Code.Conflicts, message: 'conflicts' },
      }),
    )
    await nextTick()

    expect(onConflicts).toHaveBeenCalledTimes(1)
    expect(notice.find('.p-toast-message').exists()).toBe(false)
  })

  it('FR-SYNC-08, FR-UX-01 a sync failure not explained elsewhere shows a plain notice', async () => {
    const notice = await mountNotices()

    useJobsStore().upsert(failedFetchJob)
    await nextTick()

    expect(notice.text()).toContain('Could not sign in to the remote.')
  })

  it("FR-UX-01, FR-SYNC-08 a sync failure's notice shows git's command and stderr behind a details toggle", async () => {
    const notice = await mountNotices()

    useJobsStore().upsert(
      jobFixture({
        state: JobState.JobFailed,
        error: {
          code: Code.AuthFailed,
          message: 'Could not sign in to the remote.',
          gitStderr: 'fatal: could not read Username for the remote',
          command: 'git fetch origin',
        },
      }),
    )
    await nextTick()

    expect(notice.text()).toContain('Could not sign in to the remote.')
    expect(notice.text()).not.toContain('could not read Username')

    await findButton(notice, 'show details')!.trigger('click')

    expect(notice.text()).toContain('could not read Username')
    expect(notice.text()).toContain('git fetch origin')
  })

  it('FR-UX-01 a failure another notice explains shows no extra notice', async () => {
    const notice = await mountNotices()
    const jobs = useJobsStore()

    jobs.upsert(
      jobFixture({
        kind: JobKind.PullJob,
        state: JobState.JobFailed,
        error: { code: Code.Conflicts, message: Code.Conflicts },
      }),
    )
    await nextTick()

    expect(notice.find('.p-toast-message').exists()).toBe(false)
  })

  it('FR-UX-16 a job failed LOCKED shows "Try again", running onRetryJob with the job', async () => {
    const notice = await mountNotices()
    const job = jobFixture({
      state: JobState.JobFailed,
      error: {
        code: Code.Locked,
        message: 'Another program is using the repo (holding index.lock).',
      },
    })

    useJobsStore().upsert(job)
    await nextTick()

    await findButton(notice, 'Try again')!.trigger('click')
    expect(onRetryJob).toHaveBeenCalledTimes(1)
    expect(onRetryJob).toHaveBeenCalledWith(job)
  })

  it('FR-UX-12 an update-branch job ends updated: notice names the branch, commit count, from and to', async () => {
    const notice = await mountNotices()

    useJobsStore().upsert(
      branchUpdateJob({
        result: {
          outcome: UpdateOutcome.BranchUpdated,
          from: 'abc1234full',
          to: 'def5678full',
          commits: 3,
        },
      }),
    )
    await nextTick()

    expect(notice.text()).toContain('Updated feature: 3 commits, abc1234 → def5678')
  })

  it('FR-UPD-05 an update-branch job ends up to date: notice says so', async () => {
    const notice = await mountNotices()

    useJobsStore().upsert(branchUpdateJob())
    await nextTick()

    expect(notice.text()).toContain('feature is up to date')
  })

  it('FR-UPD-05 an update-branch job fails: notice shows the failure', async () => {
    const notice = await mountNotices()

    useJobsStore().upsert(
      branchUpdateJob({
        state: JobState.JobFailed,
        progress: 0,
        error: { code: Code.GitFailed, message: 'could not update' },
      }),
    )
    await nextTick()

    expect(notice.text()).toContain('could not update')
  })

  const endedJobs: [string, JobInfo][] = [
    ['a rejected push', rejectedPushJob(Code.PushRejected)],
    ['a pull blocked by local changes', blockedPullJob],
    ['a failed fetch', failedFetchJob],
    ['a branch update', branchUpdateJob()],
  ]

  it.each(endedJobs)(
    'FR-UX-15 %s: its closed notice does not come back on a later job update or a reload of the jobs',
    async (_, job) => {
      const notice = await mountNotices()
      const jobs = useJobsStore()
      jobs.upsert(job)
      await flushPromises()
      await notice.get('.p-toast-close-button').trigger('click')
      await flushPromises()

      jobs.upsert(
        jobFixture({ id: 'j2', kind: JobKind.UpdateBranchJob, branch: 'other', progress: 10 }),
      )
      Facade.Jobs.mockResolvedValue([structuredClone(job)])
      await jobs.load('/repo')
      await flushPromises()

      expect(notice.find('.p-toast-message').exists()).toBe(false)
    },
  )

  it('FR-UX-15 a job that ended before the window listed it shows no notice', async () => {
    const notice = await mountNotices()
    Facade.Jobs.mockResolvedValue(endedJobs.map(([, job], i) => ({ ...job, id: `j${i}` })))

    await useJobsStore().load('/repo')
    await flushPromises()

    expect(notice.find('.p-toast-message').exists()).toBe(false)
  })

  it("FR-UX-15 one branch's update result leaves another branch's open", async () => {
    const notice = await mountNotices()
    const jobs = useJobsStore()

    jobs.upsert(branchUpdateJob())
    jobs.upsert(branchUpdateJob({ id: 'j2', branch: 'topic' }))
    await flushPromises()

    expect(notice.text()).toContain('feature is up to date')
    expect(notice.text()).toContain('topic is up to date')
  })
})
