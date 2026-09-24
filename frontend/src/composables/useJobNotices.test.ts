import { flushPromises, mount, type DOMWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, nextTick } from 'vue'

const { Facade } = vi.hoisted(() => ({ Facade: { Jobs: vi.fn() } }))
vi.mock('../../bindings/github.com/brogli/nebegit/internal/app', () => ({ Facade }))

import { useJobNotices } from './useJobNotices'
import { useJobsStore } from '../stores/jobs'
import { Code } from '../../bindings/github.com/brogli/nebegit/internal/core'
import type { JobInfo } from '../../bindings/github.com/brogli/nebegit/internal/core'
import NoticeArea from '../components/NoticeArea.vue'
import { mountInBody } from '../testing/mountInBody'
import { findButton } from '../testing/findButton'
import { jobFixture } from '../testing/fixtures/job'

function rejectedPushJob(code: Code): JobInfo {
  return jobFixture({
    kind: 'push',
    state: 'failed',
    message: 'Updates were rejected',
    error: { code, message: 'Updates were rejected' },
  })
}

const blockedPullJob = jobFixture({
  kind: 'pull',
  state: 'failed',
  error: { code: Code.DirtyWorktree, message: 'local changes in the way' },
})
const failedFetchJob = jobFixture({
  state: 'failed',
  error: { code: Code.AuthFailed, message: 'Could not sign in to the remote.' },
})
const upToDate = { outcome: 'upToDate', from: 'abc1234full', to: 'abc1234full', commits: 0 }

// branchUpdateJob is a branch update of feature that ended up to date.
function branchUpdateJob(overrides: Partial<JobInfo> = {}): JobInfo {
  return jobFixture({
    kind: 'update-branch',
    branch: 'feature',
    state: 'done',
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

const Host = defineComponent({
  setup() {
    useJobNotices({ onPull, onForcePush, onStashPull })
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
  })

  it('FR-SYNC-04 a failed push job shows a notice with the reason and Pull/Force push, running the actions', async () => {
    const notice = await mountNotices()

    useJobsStore().upsert(rejectedPushJob(Code.NotFastForward))
    await nextTick()

    expect(notice.text()).toContain('Updates were rejected')
    await findButton(notice, 'Force push')!.trigger('click')
    expect(onForcePush).toHaveBeenCalledTimes(1)
  })

  it('FR-SYNC-04 clicking Pull in the rejection notice runs the Pull action', async () => {
    const notice = await mountNotices()

    useJobsStore().upsert(rejectedPushJob(Code.PushRejected))
    await nextTick()

    await findButton(notice, 'Pull')!.trigger('click')
    expect(onPull).toHaveBeenCalledTimes(1)
  })

  it('FR-SYNC-04 a later pull clears the rejection notice', async () => {
    const notice = await mountNotices()
    const jobs = useJobsStore()

    jobs.upsert(rejectedPushJob(Code.NotFastForward))
    await nextTick()
    expect(notice.find('.notice').exists()).toBe(true)

    jobs.upsert(jobFixture({ id: 'j2', kind: 'pull', state: 'done', progress: 100 }))
    await nextTick()

    expect(notice.find('.notice').exists()).toBe(false)
  })

  it('FR-SYNC-09 a pull job failed DIRTY_WORKTREE shows "Stash and pull", running PullWithStash', async () => {
    const notice = await mountNotices()

    useJobsStore().upsert(blockedPullJob)
    await nextTick()

    expect(notice.text()).toContain('local changes in the way')
    await findButton(notice, 'Stash and pull')!.trigger('click')
    expect(onStashPull).toHaveBeenCalledTimes(1)
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
        state: 'failed',
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
        kind: 'pull',
        state: 'failed',
        error: { code: Code.Conflicts, message: Code.Conflicts },
      }),
    )
    await nextTick()

    expect(notice.find('.notice').exists()).toBe(false)
  })

  it('FR-UX-12 an update-branch job ends updated: notice names the branch, commit count, from and to', async () => {
    const notice = await mountNotices()

    useJobsStore().upsert(
      branchUpdateJob({
        result: { outcome: 'updated', from: 'abc1234full', to: 'def5678full', commits: 3 },
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
        state: 'failed',
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
      await notice.get('[data-test="notice-close"]').trigger('click')
      await flushPromises()

      jobs.upsert(jobFixture({ id: 'j2', kind: 'update-branch', branch: 'other', progress: 10 }))
      Facade.Jobs.mockResolvedValue([structuredClone(job)])
      await jobs.load('/repo')
      await flushPromises()

      expect(notice.find('.notice').exists()).toBe(false)
    },
  )

  it('FR-UX-15 a job that ended before the window listed it shows no notice', async () => {
    const notice = await mountNotices()
    Facade.Jobs.mockResolvedValue(endedJobs.map(([, job], i) => ({ ...job, id: `j${i}` })))

    await useJobsStore().load('/repo')
    await flushPromises()

    expect(notice.find('.notice').exists()).toBe(false)
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
