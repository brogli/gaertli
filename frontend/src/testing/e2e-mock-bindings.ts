// This module replaces the generated `internal/app` bindings barrel under
// `vite --mode e2e` (vite.config.ts's alias), so it must export every name
// the app imports from the barrel or the module graph fails to load. Facade
// implements only the calls the log and layout scenarios need.

import { objectNames, setTransport, type Events } from '@wailsio/runtime'
import type {
  ChangedFile,
  ChangedFiles,
  CommitDetails,
  DiffFile,
  LogRowsResult,
  Refs,
  Summary,
  Worktree,
  WorktreeID,
} from '../../bindings/github.com/brogli/nebegit/internal/core'
import { PullMode } from '../../bindings/github.com/brogli/nebegit/internal/core'
import type {
  RecentRepo,
  Settings,
} from '../../bindings/github.com/brogli/nebegit/internal/settings'
import { branchFixture } from './fixtures/branch'
import { changedFilesFixture } from './fixtures/changes'
import { LOG_ROWS_FIXTURE } from './fixtures/log-rows'
import { defaultSettingsFixture } from './fixtures/settings'
import { summaryFixture } from './fixtures/summary'
import { worktreeFixture } from './fixtures/worktree'

// A real Facade call is an IPC round-trip; this delay keeps the mock
// realistic enough that only the visible window loads up front.
const ROUND_TRIP_MS = 20

function delay<T>(value: T): Promise<T> {
  return new Promise((resolve) => setTimeout(() => resolve(value), ROUND_TRIP_MS))
}

// STATUS_FIXTURE: a working tree with a file in every group, for the
// layout case that checks a file row's buttons and the "Stage all"/"Unstage
// all" header.
const STATUS_FIXTURE = changedFilesFixture({
  staged: [
    { path: 'src/components/staged-one.ts', status: 'M' },
    { path: 'src/components/staged-two.ts', status: 'A' },
  ],
  unstaged: [{ path: 'src/components/unstaged-one.ts', status: 'M' }],
  untracked: [{ path: 'src/components/untracked-new-file.ts', status: '?' }],
})

// statusFixture: STATUS_FIXTURE, or ?changedFiles=<n> unstaged files for
// the changes view's scrolling case, which needs a change
// list taller than the window.
function statusFixture(): ChangedFiles {
  const count = Number(new URLSearchParams(location.search).get('changedFiles') ?? '0')
  if (!count) return STATUS_FIXTURE
  return changedFilesFixture({
    unstaged: Array.from({ length: count }, (_, i) => ({
      path: `src/components/file-${i}.ts`,
      status: 'M',
    })),
  })
}

// refsFixture: the current branch (no upstream, unless ?upstream=1 asks for
// one, for the toolbar-fit case that needs the Pull/Push pair instead of
// "Push and set upstream"), a second branch with an upstream and a long
// name, a remote branch and a tag, so the branch picker and the sync
// controls render real content.
function refsFixture(): Refs {
  const hasUpstream = new URLSearchParams(location.search).has('upstream')
  return {
    branches: [
      branchFixture({
        sha: LOG_ROWS_FIXTURE[0]!.sha,
        ...(hasUpstream ? { upstream: 'origin/main' } : {}),
      }),
      branchFixture({
        name: 'feature/a-rather-long-branch-name-that-does-not-fit',
        sha: LOG_ROWS_FIXTURE[1]!.sha,
        current: false,
        upstream: 'origin/feature/a-rather-long-branch-name-that-does-not-fit',
        ahead: 1,
        behind: 3,
      }),
    ],
    remotes: [{ name: 'origin', branches: [{ name: 'main', sha: LOG_ROWS_FIXTURE[0]!.sha }] }],
    tags: [{ name: 'v1.2.3-fixture', sha: LOG_ROWS_FIXTURE[0]!.sha }],
  }
}

// WORKTREES_FIXTURE: the mock repo's main worktree and a linked one, for
// the worktree picker; RECENT_FIXTURE lists the repo and a second one.
const WORKTREES_FIXTURE: Worktree[] = [
  worktreeFixture({
    path: '/mock/repo',
    branch: 'main',
    sha: LOG_ROWS_FIXTURE[0]!.sha,
    main: true,
  }),
  worktreeFixture({
    path: '/mock/repo-feature',
    branch: 'feature/a-rather-long-branch-name',
    sha: LOG_ROWS_FIXTURE[1]!.sha,
    dirty: true,
  }),
]
const RECENT_FIXTURE: RecentRepo[] = [
  { path: '/mock/repo', lastOpened: '2026-01-02T00:00:00Z' },
  { path: '/mock/other-repository-with-a-long-name', lastOpened: '2026-01-01T00:00:00Z' },
]

// commitDetailsFixture: a plausible commit for whichever row the layout
// case clicks, with a changed file.
function commitDetailsFixture(sha: string): CommitDetails {
  return {
    sha,
    parents: [],
    author: 'Ada Fixture',
    authorEmail: 'ada@example.com',
    authorAt: 1_700_000_000,
    committer: 'Ada Fixture',
    committerEmail: 'ada@example.com',
    committerAt: 1_700_000_000,
    body: 'Fixture commit for the layout e2e case.',
    files: [{ path: 'src/components/staged-one.ts', status: 'M' }],
  }
}

// DIFF_FILE_FIXTURE: real old/new content, so DiffView mounts CodeMirror
// and shows its toolbar instead of a loading/binary placeholder.
const DIFF_FILE_FIXTURE: DiffFile = {
  path: 'src/components/staged-one.ts',
  oldPath: '',
  binary: false,
  tooLarge: false,
  // Long enough to need more height than the side column has, as a real
  // source file does.
  oldContent: 'line one\nline two\nline three\n' + 'unchanged line\n'.repeat(200),
  newContent:
    'line one\nline TWO, changed\nline three\nline four\n' + 'unchanged line\n'.repeat(200),
  hunks: [
    { oldStart: 2, oldLines: 1, newStart: 2, newLines: 1 },
    { oldStart: 3, oldLines: 0, newStart: 4, newLines: 1 },
  ],
}

declare global {
  interface Window {
    __e2eLogRowsCalls: Array<{ from: number; to: number }>
    _wails?: {
      dispatchWailsEvent?: (event: { name: string; data: unknown; sender: string }) => void
    }
  }
}

// calls: every LogRows(from, to) call, read back by Playwright via
// `page.evaluate(() => window.__e2eLogRowsCalls)`.
const calls: Array<{ from: number; to: number }> = []
window.__e2eLogRowsCalls = calls

// E2E_WINDOW is this page's window name. With no Go core to ask, the
// runtime's window calls (the app makes only Window.Name()) answer it, and
// dispatchEvent names it as every event's sender.
const E2E_WINDOW = 'window-1'
setTransport({
  call: (objectID: number) =>
    objectID === objectNames.Window
      ? Promise.resolve(E2E_WINDOW)
      : Promise.reject(new Error('no Go core in e2e mode')),
})

// dispatchEvent feeds an event's listeners directly, bypassing the IPC
// round-trip `Events.Emit` would need (there is no live Go core to answer
// it) — @wailsio/runtime sets up this hook itself whenever a DOM is present.
// data has the type generated from the event's Go registration.
function dispatchEvent<E extends keyof Events.CustomEvents>(
  name: E,
  data: Events.CustomEvents[E],
): void {
  window._wails?.dispatchWailsEvent?.({ name, data, sender: E2E_WINDOW })
}

// PROGRESS_STEPS: LoadLog streams in a few growing chunks, as the real core
// does.
const PROGRESS_STEPS = 6

async function streamLogProgress(): Promise<void> {
  for (let step = 1; step <= PROGRESS_STEPS; step++) {
    await delay(undefined)
    const loaded = Math.round((LOG_ROWS_FIXTURE.length * step) / PROGRESS_STEPS)
    dispatchEvent('log:progress', {
      loaded,
      done: step === PROGRESS_STEPS,
      filtered: false,
      generation: 0,
    })
  }
}

// FETCH_JOB_MS: how long the fake fetch job stays "running", long enough
// for the jobs-menu layout case to read the toolbar mid-job.
const FETCH_JOB_MS = 300
let fetchJobSeq = 0

// runFetchJob sends a fetch job's "job:update" events, as JobsMenu.vue
// receives them from the real core.
function runFetchJob(): string {
  const id = `job-fetch-${fetchJobSeq++}`
  dispatchEvent('job:update', { id, kind: 'fetch', state: 'running', progress: -1, message: '' })
  setTimeout(() => {
    dispatchEvent('job:update', { id, kind: 'fetch', state: 'done', progress: 100, message: '' })
  }, FETCH_JOB_MS)
  return id
}

export const Facade = {
  LoadLog(): Promise<void> {
    void streamLogProgress()
    return Promise.resolve()
  },

  LogRows(_wt: WorktreeID, from: number, to: number): Promise<LogRowsResult> {
    calls.push({ from, to })
    return delay({
      rows: LOG_ROWS_FIXTURE.slice(from, to),
      loaded: LOG_ROWS_FIXTURE.length,
      done: true,
      generation: 0,
    })
  },

  Summary(): Promise<Summary> {
    return Promise.resolve(summaryFixture({ editorName: 'the editor' }))
  },

  Jobs(): Promise<[]> {
    return Promise.resolve([])
  },

  Fetch(): Promise<string> {
    return Promise.resolve(runFetchJob())
  },

  Refs(): Promise<Refs> {
    return delay(refsFixture())
  },

  ConfiguredPullMode(): Promise<PullMode> {
    return Promise.resolve(PullMode.$zero)
  },

  Status(): Promise<ChangedFiles> {
    return delay(statusFixture())
  },

  CommitDetails(_wt: WorktreeID, sha: string): Promise<CommitDetails> {
    return delay(commitDetailsFixture(sha))
  },

  CompareCommits(): Promise<ChangedFile[]> {
    return delay([{ path: DIFF_FILE_FIXTURE.path, status: 'M' }])
  },

  DiffRange(): Promise<DiffFile> {
    return delay(DIFF_FILE_FIXTURE)
  },

  ListWorktrees(): Promise<Worktree[]> {
    return delay(WORKTREES_FIXTURE)
  },
}

export const AppService = {
  RecentRepos(): Promise<RecentRepo[]> {
    return Promise.resolve(RECENT_FIXTURE)
  },
  ListWSLDistros(): Promise<[]> {
    return Promise.resolve([])
  },
  UpdateWindowTitle(): Promise<void> {
    return Promise.resolve(undefined)
  },
  LogPath(): Promise<{ path: string; error: string }> {
    return Promise.resolve({ path: '/mock/nebegit.log', error: '' })
  },
}
export const SettingsService = {
  Get(): Promise<Settings> {
    return Promise.resolve(defaultSettingsFixture())
  },
}
