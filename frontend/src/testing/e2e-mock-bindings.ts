// This module replaces the generated `internal/app` bindings barrel under
// `vite --mode e2e` (vite.config.ts's alias), so it must export every name
// the app imports from the barrel or the module graph fails to load. Facade
// implements only the calls the log and layout scenarios need.

import { objectNames, setTransport, type Events } from '@wailsio/runtime'
import {
  IntegrateOutcome,
  JobState,
  type ChangedFile,
  type ChangedFiles,
  type CommitDetails,
  type ConflictFile,
  type DiffFile,
  type IntegrateResult,
  type LogIndexResult,
  type LogRowsResult,
  type LogSearchResult,
  type PushTarget,
  type Refs,
  type Summary,
  type Worktree,
  type WorktreeID,
} from '../../bindings/github.com/brogli/nebegit/internal/core'
import {
  DiffMode,
  Theme,
  type RecentRepo,
  type Preferences,
} from '../../bindings/github.com/brogli/nebegit/internal/settings'
import type { LogPathInfo } from '../../bindings/github.com/brogli/nebegit/internal/app/models'
import { branchFixture } from '@/testing/fixtures/branch'
import { changedFilesFixture } from '@/testing/fixtures/changes'
import { diffFileFixture, hunk } from '@/testing/fixtures/diff'
import { scrollConflictFixture } from '@/testing/fixtures/conflicts'
import { jobFixture } from '@/testing/fixtures/job'
import { LOG_ROWS_FIXTURE } from '@/testing/fixtures/log-rows'
import { defaultSettingsFixture } from '@/testing/fixtures/settings'
import { summaryFixture } from '@/testing/fixtures/summary'
import { worktreeFixture } from '@/testing/fixtures/worktree'

// A real Facade call is an IPC round-trip; this delay keeps the mock
// realistic enough that only the visible window loads up front.
const ROUND_TRIP_MS = 20

function delay<T>(value: T): Promise<T> {
  return new Promise((resolve) => setTimeout(() => resolve(value), ROUND_TRIP_MS))
}

// query is the page's URL query, where a test asks for a fixture variant.
const query = new URLSearchParams(location.search)

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
  // The diff window's ?conflicted=true target lists the conflicted file.
  if (query.get('conflicted') === 'true') {
    return changedFilesFixture({
      conflicted: [{ path: scrollConflictFixture().path, status: 'UU' }],
    })
  }
  const count = Number(query.get('changedFiles') ?? '0')
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
// Push alone), a second branch with an upstream and a long name, a remote
// branch and a tag, so the branch picker and the sync controls render real
// content; or, with ?crowdedRefs=1, the picker case's mixed local branches
// (one with an upstream, one without), a remote branch and a tag, all
// short, to check where their names line up.
function refsFixture(): Refs {
  if (query.has('crowdedRefs')) {
    return {
      branches: [
        branchFixture({ sha: LOG_ROWS_FIXTURE[0]!.sha }),
        branchFixture({
          name: 'B',
          sha: LOG_ROWS_FIXTURE[1]!.sha,
          current: false,
          upstream: 'origin/B',
        }),
        branchFixture({ name: 'side', sha: LOG_ROWS_FIXTURE[2]!.sha, current: false }),
      ],
      remotes: [{ name: 'origin', branches: [{ name: 'B', sha: LOG_ROWS_FIXTURE[1]!.sha }] }],
      tags: [{ name: 'v1', sha: LOG_ROWS_FIXTURE[0]!.sha }],
    }
  }
  const hasUpstream = query.has('upstream')
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
// the worktree picker.
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

// worktreesFixture: WORKTREES_FIXTURE, or a picker case's variant asked for
// by ?worktrees=: "crowded" adds a dirty, a locked and a missing worktree,
// plus one whose branch name is too long for the picker row, for the case
// that checks every row still keeps its name, flags and buttons on one
// line; "wip" is a worktree whose folder ("wip") is short but whose branch
// name is not, for the case that checks the branch name gives way first.
function worktreesFixture(): Worktree[] {
  const variant = query.get('worktrees')
  if (variant === 'crowded') {
    const long = 'a-rather-long-branch-name-that-does-not-fit-the-worktree-picker'
    return [
      worktreeFixture({
        path: '/mock/repo',
        branch: 'main',
        sha: LOG_ROWS_FIXTURE[0]!.sha,
        main: true,
      }),
      worktreeFixture({ path: '/mock/repo-side', branch: 'side', sha: LOG_ROWS_FIXTURE[1]!.sha }),
      worktreeFixture({
        path: '/mock/repo-dirty',
        branch: 'wt-dirty',
        sha: LOG_ROWS_FIXTURE[1]!.sha,
        dirty: true,
      }),
      worktreeFixture({
        path: '/mock/repo-locked',
        branch: 'wt-locked',
        sha: LOG_ROWS_FIXTURE[1]!.sha,
        locked: true,
      }),
      worktreeFixture({
        path: '/mock/repo-missing',
        branch: 'wt-missing',
        sha: LOG_ROWS_FIXTURE[1]!.sha,
        missing: true,
      }),
      worktreeFixture({ path: '/mock/repo-long', branch: long, sha: LOG_ROWS_FIXTURE[1]!.sha }),
    ]
  }
  if (variant === 'wip') {
    const long = 'a-branch-with-a-rather-long-name-that-cannot-fit-in-a-picker-row'
    return [
      worktreeFixture({
        path: '/mock/repo',
        branch: 'main',
        sha: LOG_ROWS_FIXTURE[0]!.sha,
        main: true,
      }),
      worktreeFixture({ path: '/mock/wip', branch: long, sha: LOG_ROWS_FIXTURE[1]!.sha }),
    ]
  }
  return WORKTREES_FIXTURE
}

// RECENT_FIXTURE lists the mock repo and a second one.
const RECENT_FIXTURE: RecentRepo[] = [
  { path: '/mock/repo', lastOpened: '2026-01-02T00:00:00Z' },
  { path: '/mock/other-repository-with-a-long-name', lastOpened: '2026-01-01T00:00:00Z' },
]

// commitDetailsFixture: a plausible commit for whichever row the layout
// case clicks, with a changed file; or, with ?commitFiles=<n>, n files and
// a message as many lines long, for the commit detail view's scrolling
// case, which needs both parts taller than the window.
function commitDetailsFixture(sha: string): CommitDetails {
  const count = Number(query.get('commitFiles') ?? '0')
  const subject = 'Fixture commit for the layout e2e case.'
  return {
    sha,
    parents: [],
    author: 'Ada Fixture',
    authorEmail: 'ada@example.com',
    authorAt: 1_700_000_000,
    committer: 'Ada Fixture',
    committerEmail: 'ada@example.com',
    committerAt: 1_700_000_000,
    body: [subject, '', ...Array.from({ length: count }, (_, i) => `Line ${i}.`)].join('\n'),
    files: count
      ? Array.from({ length: count }, (_, i) => ({ path: `src/file-${i}.ts`, status: 'M' }))
      : [{ path: 'src/components/staged-one.ts', status: 'M' }],
  }
}

// DIFF_FILE_FIXTURE: real old/new content, so DiffView mounts CodeMirror
// and shows its toolbar instead of a loading/binary placeholder.
const DIFF_FILE_FIXTURE = diffFileFixture({
  path: 'src/components/staged-one.ts',
  oldPath: '',
  // Long enough to need more height than the diff window has, as a real
  // source file does.
  oldContent:
    'line one\nline two\nline three\n' + 'unchanged line\n'.repeat(200) + 'last old line\n',
  newContent:
    'line one\nline TWO, changed\nline three\nline four\n' +
    'unchanged line\n'.repeat(200) +
    'last new line\n',
  hunks: [hunk(2, 1, 2, 1), hunk(3, 0, 4, 1)],
})

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
  dispatchEvent('job:update', jobFixture({ id, progress: -1 }))
  setTimeout(() => {
    dispatchEvent('job:update', jobFixture({ id, state: JobState.JobDone, progress: 100 }))
  }, FETCH_JOB_MS)
  return id
}

export const Facade = {
  Merge(): Promise<IntegrateResult> {
    return delay({ outcome: IntegrateOutcome.IntegrateMerged })
  },

  Rebase(): Promise<IntegrateResult> {
    return delay({ outcome: IntegrateOutcome.IntegrateRebased })
  },

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

  LogIndexOf(_wt: WorktreeID, sha: string): Promise<LogIndexResult> {
    const index = LOG_ROWS_FIXTURE.findIndex((row) => row.sha === sha)
    return delay(index === -1 ? { index: -1, found: false } : { index, found: true })
  },

  SearchLog(_wt: WorktreeID, query: string): Promise<LogSearchResult> {
    if (!query) return delay({ generation: 0, filtered: false })
    const bySha = LOG_ROWS_FIXTURE.findIndex((row) => row.sha === query)
    if (bySha !== -1) return delay({ indices: [bySha], generation: 0, filtered: false })
    const q = query.toLowerCase()
    const indices = LOG_ROWS_FIXTURE.flatMap((row, i) =>
      row.subject.toLowerCase().includes(q) ? [i] : [],
    )
    return delay({ indices, generation: 0, filtered: false })
  },

  Summary(): Promise<Summary> {
    return Promise.resolve(summaryFixture({ editorName: 'Alpha' }))
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

  PushTarget(): Promise<PushTarget> {
    return delay({
      remote: 'origin',
      autoSetUpstream: false,
      branch: 'main',
      hasUpstream: query.has('upstream'),
    })
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

  Conflict(): Promise<ConflictFile> {
    return delay(scrollConflictFixture())
  },

  MergeDraft(): Promise<null> {
    return delay(null)
  },

  SetMergeDraft(): Promise<void> {
    return delay(undefined)
  },

  ListWorktrees(): Promise<Worktree[]> {
    return delay(worktreesFixture())
  },
}

export const AppService = {
  RecentRepos(): Promise<RecentRepo[]> {
    return Promise.resolve(RECENT_FIXTURE)
  },
  ListWSLDistros(): Promise<[]> {
    return Promise.resolve([])
  },
  NewWindow(): Promise<void> {
    return Promise.resolve(undefined)
  },
  UpdateWindowTitle(): Promise<void> {
    return Promise.resolve(undefined)
  },
  LogPath(): Promise<LogPathInfo> {
    return Promise.resolve({ path: '/mock/nebegit.log', error: '' })
  },
  SetUnsavedWork(): Promise<void> {
    return Promise.resolve(undefined)
  },
  OpenRecentRepo(): Promise<void> {
    return Promise.resolve(undefined)
  },
  RemoveRecentRepo(): Promise<void> {
    return Promise.resolve(undefined)
  },
}
// themeParam/diffModeParam: ?theme= and ?diffMode=, matched against the
// enums' own values, so a test reaches a theme or diff mode the app would
// otherwise only apply by saving Settings.
const themeParam = Object.values(Theme).find((t) => t === query.get('theme'))
const diffModeParam = Object.values(DiffMode).find((m) => m === query.get('diffMode'))

// storedSettings is the mock's whole persisted state: Set replaces it, Get
// reads it back, so a save shows up without a reload, as the shell's
// settings:changed event does.
let storedSettings: Preferences = defaultSettingsFixture({
  debugMode: query.has('debug'),
  ...(themeParam !== undefined ? { theme: themeParam } : {}),
  ...(diffModeParam !== undefined ? { diffMode: diffModeParam } : {}),
})

export const SettingsService = {
  Get(): Promise<Preferences> {
    return Promise.resolve(storedSettings)
  },
  Set(next: Preferences): Promise<void> {
    storedSettings = next
    dispatchEvent('settings:changed', undefined)
    return Promise.resolve(undefined)
  },
}
