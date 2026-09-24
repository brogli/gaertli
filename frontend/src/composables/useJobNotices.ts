import { i18n } from '../i18n'
import { useJobsStore } from '../stores/jobs'
import { Code, type JobInfo } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { useNotices, type AppNotice, type NoticeAction } from '../lib/notice'
import { shortSha } from '../lib/format'
import type { CoreError } from './useCoreCall'

// UpdateBranchResult is core.UpdateBranchResult, UpdateBranch's
// JobInfo.result; the bindings type JobInfo.result as `any`.
interface UpdateBranchResult {
  outcome: 'updated' | 'upToDate'
  from: string
  to: string
  commits: number
}

// A success notice closes itself; a failure or offer stays until closed.
const SUCCESS_LIFE = 8000

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

const ENDED = ['done', 'failed', 'cancelled']

interface JobNoticeActions {
  onPull: () => void
  onForcePush: () => void
  onStashPull: () => void
}

// useJobNotices shows the result a sync or branch-update job ends with as a
// notice, once, when the job's end arrives; a job that ended before this
// window listed it shows none. Actions supplies the pull-mode-aware,
// confirm-gated recovery calls the rejection and blocked-pull notices offer.
export function useJobNotices(actions: JobNoticeActions): void {
  const jobs = useJobsStore()
  const { show, clear } = useNotices()
  const t = i18n.global.t

  jobs.$onAction(({ name, args }) => {
    if (name !== 'upsert') return
    const [job] = args
    staleNotices(job).forEach(clear)
    if (!ENDED.includes(job.state)) return
    const ended = endNotice(job)
    if (ended) show(...ended)
  })

  // endNotice is the notice job ended with and its kind, or null for none.
  function endNotice(job: JobInfo): [kind: string, notice: AppNotice] | null {
    if (job.kind === 'update-branch') return updateNotice(job)
    const error = job.error
    if (job.state !== 'failed' || !error) return null
    const push = job.kind === 'push' || job.kind === 'force-push'
    if (push && (error.code === Code.PushRejected || error.code === Code.NotFastForward)) {
      return [
        'rejection',
        errorNotice(error, [
          { label: t('toolbar.pull'), run: actions.onPull },
          { label: t('toolbar.forcePush'), run: actions.onForcePush },
        ]),
      ]
    }
    if (job.kind === 'pull' && error.code === Code.DirtyWorktree) {
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
    if (job.state === 'failed') {
      return [
        kind,
        errorNotice(job.error ?? { message: t('notices.updateBranchFailed', { branch }) }),
      ]
    }
    if (job.state !== 'done') return null
    const result = job.result as UpdateBranchResult
    const summary =
      result.outcome === 'updated'
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
    return [kind, { severity: 'success', summary, life: SUCCESS_LIFE }]
  }

  // errorNotice renders its body as an ErrorBanner (the message plus its
  // details toggle for git's command and stderr), actions adding recovery
  // buttons beside it.
  function errorNotice(error: CoreError, actions?: NoticeAction[]): AppNotice {
    return {
      severity: 'error',
      summary: error.message,
      data: { error, ...(actions && { actions }) },
    }
  }
}

// STALE_NOTICES are the kinds of notice an update of a sync job makes
// stale, by the job's kind.
const STALE_NOTICES: Record<string, string[]> = {
  fetch: ['sync-failure'],
  push: ['rejection', 'sync-failure'],
  'force-push': ['rejection', 'sync-failure'],
  pull: ['rejection', 'pull-blocked', 'sync-failure'],
}

// staleNotices are the kinds of notice an update of job makes stale.
function staleNotices(job: JobInfo): string[] {
  return job.kind === 'update-branch' ? [updateBranchKind(job)] : (STALE_NOTICES[job.kind] ?? [])
}

// updateBranchKind is the notice kind of a branch update: one per branch.
function updateBranchKind(job: JobInfo): string {
  return `update-branch:${job.branch ?? ''}`
}
