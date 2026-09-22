import type {
  CommandLogEntry,
  LogRowsResult,
  Summary,
  WorktreeID,
} from '../../bindings/github.com/brogli/nebegit/internal/core'
import { LOG_ROWS_FIXTURE } from './fixtures/log-rows'

// A real Facade call is an IPC round-trip; this delay keeps the mock
// realistic enough that only the visible window loads up front.
const ROUND_TRIP_MS = 20

function delay<T>(value: T): Promise<T> {
  return new Promise((resolve) => setTimeout(() => resolve(value), ROUND_TRIP_MS))
}

// This module replaces the generated `internal/app` bindings barrel under
// `vite --mode e2e` (vite.config.ts's alias), so it must export every name
// the barrel does or the module graph fails to load. Facade implements only
// the calls the log scenario needs, backed by LOG_ROWS_FIXTURE.

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
  window._wails?.dispatchWailsEvent?.({ name: 'log:progress', data: { loaded, done } })
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
    })
  },

  CommandLog(_wt: WorktreeID): Promise<CommandLogEntry[]> {
    return Promise.resolve([])
  },
}

export const AppService = {}
export const SettingsService = {}
