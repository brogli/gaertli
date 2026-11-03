import { test as base, expect, type Page } from '@playwright/test'
import { spawn, type ChildProcess } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'
import { PullMode } from '../bindings/github.com/brogli/nebegit/internal/core/models'
import {
  DiffMode,
  type Preferences,
} from '../bindings/github.com/brogli/nebegit/internal/settings/models'
import { switchToChanges } from './helpers'
import { createRepos, gitWith, type Git, type Repos } from './repos'
import { createSandbox, editorCalls, type Sandbox } from './sandbox'

export { expect }

const serverBin = path.resolve(process.env.NEBEGIT_SERVER_BIN ?? '../bin/nebegit-server')

const firstPort = 20000
const portsPerWorker = 100
const maxStartAttempts = 5
const portTaken = 'failed to listen on'

interface Server {
  url: string
  pid: number
}

interface Options {
  // open names the worktrees the server opens at startup; only those can
  // be shown in a page.
  open: (keyof Repos)[]
  // settings overrides the seeded settings file's fields.
  settings: Partial<Preferences>
}

interface Fixtures {
  sandbox: Sandbox
  repos: Repos
  git: Git
  editorCalls: () => string[]
  server: Server
  // newPage is a page whose errors fail the test.
  newPage: () => Promise<Page>
  // openApp shows worktree (one of the opened ones) in a new page, loaded.
  openApp: (worktree: string) => Promise<Page>
  // openDiff shows worktree's diff window on target (the URL's kind, sha,
  // from, to, staged, file, oldFile) in a new page, its events connected.
  openDiff: (worktree: string, target: Record<string, string>) => Promise<Page>
  // app is work, opened and loaded.
  app: Page
}

export const test = base.extend<Options & Fixtures>({
  open: [['work'], { option: true }],
  settings: [{}, { option: true }],

  // oxlint-disable-next-line no-empty-pattern -- Playwright requires a destructuring pattern here
  sandbox: async ({}, use, testInfo) => {
    const sandbox = createSandbox()
    await use(sandbox)
    if (testInfo.status === testInfo.expectedStatus) {
      fs.rmSync(sandbox.dir, { recursive: true, force: true })
    } else {
      await testInfo.attach('sandbox', { body: sandbox.dir })
    }
  },

  git: async ({ sandbox }, use) => use(gitWith(sandbox.env)),

  repos: async ({ sandbox, git }, use) => use(createRepos(sandbox.dir, git)),

  editorCalls: async ({ sandbox }, use) => use(() => editorCalls(sandbox)),

  server: async ({ sandbox, repos, open, settings }, use, testInfo) => {
    if (!fs.existsSync(serverBin)) throw new Error(`${serverBin} is missing: run task e2e:real`)
    fs.writeFileSync(
      sandbox.settingsPath,
      JSON.stringify(seedSettings(sandbox, repos, open, settings)),
    )
    const logPath = path.join(sandbox.dir, 'server.log')
    let proc: ChildProcess | undefined
    try {
      const started = await startServer(sandbox, logPath, testInfo.parallelIndex)
      proc = started.proc
      expectNoGitEditor(proc.pid!)
      await use({ url: started.url, pid: proc.pid! })
    } finally {
      if (proc) await stop(proc)
      if (testInfo.status !== testInfo.expectedStatus)
        await testInfo.attach('server.log', { path: logPath })
    }
  },

  newPage: async ({ context }, use) => {
    const pageErrors: string[] = []
    await use(async () => {
      const page = await context.newPage()
      page.on('pageerror', (e) => pageErrors.push(e.message))
      page.on('console', (m) => {
        if (m.type() === 'error' && !isRefusedCall(m.text())) pageErrors.push(m.text())
      })
      await page.setViewportSize({ width: 1200, height: 800 })
      return page
    })
    expect(pageErrors, 'errors in the page').toEqual([])
  },

  openApp: async ({ server, newPage }, use) => {
    await use(async (worktree) => {
      const page = await newPage()
      await load(page, server.url, worktree)
      return page
    })
  },

  // Server mode holds a page's events until its event websocket connects,
  // so the page is ready once it has.
  openDiff: async ({ server, newPage }, use) => {
    await use(async (worktree, target) => {
      const page = await newPage()
      const connected = waitForEvents(page)
      await page.goto(
        `${server.url}/?${new URLSearchParams({ view: 'diff', wt: worktree, ...target })}`,
      )
      await connected
      return page
    })
  },

  app: async ({ openApp, repos }, use) => use(await openApp(repos.work)),
})

// isRefusedCall: in server mode a binding call that returns an error (git
// refused, validation failed) answers HTTP 422, which the browser logs as a
// failed resource load. That is the app's normal error path; tests assert
// the message the app shows instead.
function isRefusedCall(message: string): boolean {
  return message.includes('the server responded with a status of 422')
}

// seedSettings is a fresh install's settings plus the stand-in editor, main
// and master as protected branches, and the worktrees to restore.
function seedSettings(
  sandbox: Sandbox,
  repos: Repos,
  open: (keyof Repos)[],
  overrides: Partial<Preferences>,
): Partial<Preferences> & { openWorktrees: string[]; recentRepos: [] } {
  const editor = path.join(sandbox.dir, 'bin', 'stand-in-editor')
  return {
    openWorktrees: open.map((name) => repos[name]),
    recentRepos: [],
    editorProfile: {
      name: 'Stand-in',
      folderCmd: `${editor} {path}`,
      fileCmd: `${editor} {path}`,
      fileAtLineCmd: `${editor} {path}:{line}`,
    },
    protectedBranches: ['main', 'master'],
    defaultPullMode: PullMode.PullMerge,
    diffMode: DiffMode.UnifiedDiff,
    autoFetchIntervalMinutes: 0,
    ...overrides,
  }
}

// load opens worktree's page and waits until its log shows. Server mode
// holds a page's events until its event websocket connects, so the page's
// first full log load is announced to load. The websocket carries every
// page's events; a test opening two pages at once would see the other
// page's too.
async function load(page: Page, url: string, worktree: string): Promise<void> {
  const events: WailsEvent[] = []
  page.on('websocket', (ws) =>
    ws.on('framereceived', ({ payload }) => events.push(JSON.parse(String(payload)))),
  )
  await page.goto(`${url}/?path=${encodeURIComponent(worktree)}`)
  await expect.poll(() => events.some(isFullLogLoaded)).toBe(true)
  await expect(page.locator('.row').first()).toBeVisible()
  // The fixture's 1200 x 800 viewport is the medium layout: the switch
  // above the log's side panel defaults to Details. Most area specs work
  // the changes view, so every opened page starts on Changes instead; a
  // spec that wants Details switches to it itself (selecting a commit
  // does too).
  await switchToChanges(page)
}

// waitForEvents resolves once page's event websocket is up.
async function waitForEvents(page: Page): Promise<void> {
  await page.waitForEvent('console', (m) => m.text().includes('Event WebSocket connected'))
}

interface WailsEvent {
  name?: string
  data?: { done?: boolean; filtered?: boolean }
}

function isFullLogLoaded(e: WailsEvent): boolean {
  return e.name === 'log:progress' && e.data?.done === true && !e.data.filtered
}

// startServer spawns the server on a port of this worker's own range, so
// no two workers' servers can take each other's port. A port some other
// process holds makes the server exit with a listen failure; the next port
// is tried.
async function startServer(
  sandbox: Sandbox,
  logPath: string,
  worker: number,
): Promise<{ proc: ChildProcess; url: string }> {
  for (let attempt = 0; ; attempt++) {
    const port = firstPort + worker * portsPerWorker + attempt
    const out = fs.openSync(logPath, 'w')
    const proc = spawn(serverBin, [], {
      cwd: sandbox.dir,
      env: { ...sandbox.env, WAILS_SERVER_PORT: String(port) },
      stdio: ['ignore', out, out],
    })
    fs.closeSync(out)
    const url = `http://localhost:${port}`
    try {
      await waitUntilServing(url, proc)
      return { proc, url }
    } catch (e) {
      await stop(proc)
      if (attempt + 1 === maxStartAttempts || !fs.readFileSync(logPath, 'utf8').includes(portTaken))
        throw e
    }
  }
}

// expectNoGitEditor: a GIT_EDITOR from the caller's shell would let tests
// that commit without an editor pass for the wrong reason.
function expectNoGitEditor(pid: number): void {
  const environ = fs.readFileSync(`/proc/${pid}/environ`, 'utf8').split('\0')
  expect(environ.filter((v) => v.startsWith('GIT_EDITOR='))).toEqual([])
}

async function waitUntilServing(url: string, proc: ChildProcess): Promise<void> {
  const deadline = Date.now() + 10_000
  while (Date.now() < deadline) {
    if (proc.exitCode !== null) throw new Error(`the server exited with code ${proc.exitCode}`)
    try {
      if ((await fetch(url)).ok) return
    } catch {
      // not listening yet
    }
    await new Promise((r) => setTimeout(r, 100))
  }
  throw new Error(`the server did not answer at ${url} within 10 s`)
}

async function stop(proc: ChildProcess): Promise<void> {
  if (proc.exitCode !== null) return
  const exited = new Promise((resolve) => proc.once('exit', resolve))
  proc.kill()
  await exited
}
