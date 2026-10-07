import { i18n } from '@/i18n'
import { useJobsStore } from '@/stores/jobs'
import { useRefsStore } from '@/stores/refs'
import {
  Code,
  JobKind,
  JobState,
  UpdateOutcome,
  type JobInfo,
} from '../../bindings/github.com/brogli/nebegit/internal/core'
import type { CoreError } from './useCoreCall'
import { errorNotice, useNotices, type AppNotice } from '@/lib/notice'
import { shortSha } from '@/lib/format'

// EXPLAINED_ELSEWHERE are the job failures another part of the UI already
// shows: the rejection and blocked-pull notices below, ConflictBanner, and
// a cancel the user asked for.
const EXPLAINED_ELSEWHERE: Code[] = [
  Code.PushRejected,
  Code.NotFastForward,
  Code.DirtyWorktree,
  Code.Conflicts,
  Code.Cancelled,
]

const ENDED = [JobState.JobDone, JobState.JobFailed, JobState.JobCancelled]

interface JobNoticeActions {
  onPull: () => void
  onForcePush: () => void
  onStashPull: () => void
  // onRetryJob starts a new job like the one that just failed: the job
  // itself carries no closure to call again.
  onRetryJob: (job: JobInfo) => void
  // onConflicts is called with a pull job that stopped on conflicts.
  onConflicts: (job: JobInfo) => void
}

// useJobNotices shows the result a sync or branch-update job ends with as a
// notice, once, when the job's end arrives; a job that ended before this
// window listed it shows none. Actions supplies the pull-mode-aware,
// confirm-gated recovery calls the rejection and blocked-pull notices offer.
export function useJobNotices(actions: JobNoticeActions): void {
  const jobs = useJobsStore()
  const refs = useRefsStore()
  const { show, clear } = useNotices()
  const t = i18n.global.t

  jobs.$onAction(({ name, args }) => {
    if (name !== 'upsert') return
    const [job] = args
    staleNotices(job).forEach(clear)
    if (!ENDED.includes(job.state)) return
    // error.retry, once set, becomes the notice's "Try again" (lib/notice.ts).
    const error: CoreError | null | undefined = job.error
    if (job.kind === JobKind.PullJob && error?.code === Code.Conflicts) actions.onConflicts(job)
    if (error?.code === Code.Locked) error.retry = () => actions.onRetryJob(job)
    const ended = endNotice(job)
    if (ended) show(...ended)
  })

  // endNotice is the notice job ended with and its kind, or null for none.
  function endNotice(job: JobInfo): [kind: string, notice: AppNotice] | null {
    if (job.kind === JobKind.UpdateBranchJob) return updateNotice(job)
    const error = job.error
    if (job.state !== JobState.JobFailed || !error) return null
    const push = job.kind === JobKind.PushJob || job.kind === JobKind.ForcePushJob
    if (push && (error.code === Code.PushRejected || error.code === Code.NotFastForward)) {
      // Pull and force push both need an upstream; a first push's rejection
      // sets none.
      const offers = refs.current?.upstream
        ? [
            { label: t('toolbar.pull'), run: actions.onPull },
            { label: t('toolbar.forcePush'), run: actions.onForcePush },
          ]
        : []
      return ['rejection', errorNotice(error, offers)]
    }
    if (job.kind === JobKind.PullJob && error.code === Code.DirtyWorktree) {
      return [
        'pull-blocked',
        errorNotice(error, [{ label: t('notices.stashAndPull'), run: actions.onStashPull }]),
      ]
    }
    // A sync failure no other notice explains, e.g. a failed sign-in.
    return EXPLAINED_ELSEWHERE.includes(error.code) ? null : ['sync-failure', errorNotice(error)]
  }

  // updateNotice is a branch update's result: its commits, up to date, or
  // its failure.
  function updateNotice(job: JobInfo): [kind: string, notice: AppNotice] | null {
    const branch = job.branch ?? ''
    const kind = updateBranchKind(job)
    if (job.state === JobState.JobFailed) {
      return [
        kind,
        errorNotice(job.error ?? { message: t('notices.updateBranchFailed', { branch }) }),
      ]
    }
    if (job.state !== JobState.JobDone) return null
    // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- an UpdateBranchJob's result is set whenever state is JobDone
    const result = job.result!
    const summary =
      result.outcome === UpdateOutcome.BranchUpdated
        ? t(
            'notices.branchUpdated',
            {
              branch,
              n: result.commits,
              from: shortSha(result.from),
              to: shortSha(result.to),
            },
            result.commits,
          )
        : t('notices.branchUpToDate', { branch })
    return [kind, { severity: 'success', summary }]
  }
}

// STALE_NOTICES are the kinds of notice an update of a sync job makes
// stale, by the job's kind.
const STALE_NOTICES: Partial<Record<JobKind, string[]>> = {
  [JobKind.FetchJob]: ['sync-failure'],
  [JobKind.PushJob]: ['rejection', 'sync-failure'],
  [JobKind.ForcePushJob]: ['rejection', 'sync-failure'],
  [JobKind.PullJob]: ['rejection', 'pull-blocked', 'sync-failure'],
}

// staleNotices are the kinds of notice an update of job makes stale.
function staleNotices(job: JobInfo): string[] {
  return job.kind === JobKind.UpdateBranchJob
    ? [updateBranchKind(job)]
    : (STALE_NOTICES[job.kind] ?? [])
}

// updateBranchKind is the notice kind of a branch update: one per branch.
function updateBranchKind(job: JobInfo): string {
  return `update-branch:${job.branch ?? ''}`
}
