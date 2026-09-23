import type {
  ChangedFile,
  ChangedFiles,
  CommandLogEntry,
  CommitDetails,
  DiffFile,
  LogRowsResult,
  Refs,
  Summary,
  WorktreeID,
} from '../../bindings/github.com/brogli/nebegit/internal/core'
import type { Settings } from '../../bindings/github.com/brogli/nebegit/internal/settings'
import { LOG_ROWS_FIXTURE } from './fixtures/log-rows'
import { defaultSettingsFixture } from './fixtures/settings'

// A real Facade call is an IPC round-trip; this delay keeps the mock
// realistic enough that only the visible window loads up front.
const ROUND_TRIP_MS = 20

function delay<T>(value: T): Promise<T> {
  return new Promise((resolve) => setTimeout(() => resolve(value), ROUND_TRIP_MS))
}

// This module replaces the generated `internal/app` bindings barrel under
// `vite --mode e2e` (vite.config.ts's alias), so it must export every name
// the barrel does or the module graph fails to load. Facade implements only
// the calls the log and layout scenarios need.

// STATUS_FIXTURE: a working tree with a file in every group, for the
// layout case that checks a file row's buttons and the "Stage all"/"Unstage
// all" header.
const STATUS_FIXTURE: ChangedFiles = {
  staged: [
    { path: 'src/components/staged-one.ts', status: 'M' },
    { path: 'src/components/staged-two.ts', status: 'A' },
  ],
  unstaged: [{ path: 'src/components/unstaged-one.ts', status: 'M' }],
  untracked: [{ path: 'src/components/untracked-new-file.ts', status: '?' }],
  conflicted: [],
}

// REFS_FIXTURE: one current branch with an upstream remote, so the sidebar
// and sync controls render real content instead of staying empty.
const REFS_FIXTURE: Refs = {
  branches: [
    {
      name: 'main',
      sha: LOG_ROWS_FIXTURE[0]!.sha,
      current: true,
      upstream: '',
      ahead: 0,
      behind: 0,
      upstreamGone: false,
      checkedOutIn: '',
    },
  ],
  remotes: [{ name: 'origin', branches: [] }],
  tags: [],
}

// commitDetailsFixture: a plausible commit for whichever row the layout
// case clicks, with a file to click through to its diff.
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
}

declare global {
  interface Window {
    __e2eLogRowsCalls: Array<{ from: number; to: number }>
    _wails?: { dispatchWailsEvent?: (event: { name: string; data: unknown }) => void }
  }
}

// calls: every LogRows(from, to) call, read back by Playwright via
// `page.evaluate(() => window.__e2eLogRowsCalls)`.
const calls: Array<{ from: number; to: number }> = []
window.__e2eLogRowsCalls = calls

// dispatchLogProgress feeds the log store's 'log:progress' listener
// directly, bypassing the IPC round-trip `Events.Emit` would need (there is
// no live Go core to answer it) — @wailsio/runtime sets up this hook itself
// whenever a DOM is present.
function dispatchLogProgress(loaded: number, done: boolean): void {
  window._wails?.dispatchWailsEvent?.({
    name: 'log:progress',
    data: { loaded, done, generation: 0 },
  })
}

// PROGRESS_STEPS: LoadLog streams in a few growing chunks, as the real core
// does.
const PROGRESS_STEPS = 6

async function streamLogProgress(): Promise<void> {
  for (let step = 1; step <= PROGRESS_STEPS; step++) {
    await delay(undefined)
    const loaded = Math.round((LOG_ROWS_FIXTURE.length * step) / PROGRESS_STEPS)
    dispatchLogProgress(loaded, step === PROGRESS_STEPS)
  }
}

export const Facade = {
  LoadLog(_wt: WorktreeID): Promise<void> {
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
      flat: false,
    })
  },

  Summary(wt: WorktreeID): Promise<Summary> {
    return Promise.resolve({
      path: wt,
      branch: 'main',
      detached: false,
      operation: '',
      editorName: 'the editor',
      messageDraft: '',
      pullStash: '',
    })
  },

  CommandLog(_wt: WorktreeID): Promise<CommandLogEntry[]> {
    return Promise.resolve([])
  },

  Jobs(_wt: WorktreeID): Promise<[]> {
    return Promise.resolve([])
  },

  Refs(_wt: WorktreeID): Promise<Refs> {
    return delay(REFS_FIXTURE)
  },

  Status(_wt: WorktreeID): Promise<ChangedFiles> {
    return delay(STATUS_FIXTURE)
  },

  CommitDetails(_wt: WorktreeID, sha: string): Promise<CommitDetails> {
    return delay(commitDetailsFixture(sha))
  },

  DiffFile(
    _wt: WorktreeID,
    _sha: string,
    _path: string,
    _oldPath: string,
    _force: boolean,
  ): Promise<DiffFile> {
    return delay(DIFF_FILE_FIXTURE)
  },

  CompareCommits(_wt: WorktreeID, _from: string, _to: string): Promise<ChangedFile[]> {
    return delay([{ path: DIFF_FILE_FIXTURE.path, status: 'M' }])
  },

  DiffRange(
    _wt: WorktreeID,
    _from: string,
    _to: string,
    _path: string,
    _oldPath: string,
    _force: boolean,
  ): Promise<DiffFile> {
    return delay(DIFF_FILE_FIXTURE)
  },
}

export const AppService = {
  RecentRepos(): Promise<[]> {
    return Promise.resolve([])
  },
  UpdateWindowTitle(_name: string, _branch: string): Promise<void> {
    return Promise.resolve(undefined)
  },
}
export const SettingsService = {
  Get(): Promise<Settings> {
    return Promise.resolve(defaultSettingsFixture())
  },
}
