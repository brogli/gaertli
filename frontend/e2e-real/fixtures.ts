import { test as base, expect, type Page } from '@playwright/test'
import { spawn, type ChildProcess } from 'node:child_process'
import fs from 'node:fs'
import net from 'node:net'
import path from 'node:path'
import { ChangeScope, PullMode } from '../bindings/github.com/brogli/nebegit/internal/core/models'
import {
  DiffMode,
  type Preferences,
} from '../bindings/github.com/brogli/nebegit/internal/settings/models'
import { switchToChanges } from './helpers'
import { createRepos, gitWith, type Git, type Repos } from './repos'
import { createSandbox, editorCalls, type Sandbox } from './sandbox'

export { expect }

const serverBin = path.resolve(process.env.NEBEGIT_SERVER_BIN ?? '../bin/nebegit-server')

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
    const port = await freePort()
    const proc = spawn(serverBin, [], {
      cwd: sandbox.dir,
      env: { ...sandbox.env, WAILS_SERVER_PORT: String(port) },
      stdio: ['ignore', fs.openSync(logPath, 'w'), fs.openSync(logPath, 'a')],
    })
    const url = `http://localhost:${port}`
    try {
      await waitUntilServing(url, proc)
      await use({ url, pid: proc.pid! })
    } finally {
      await stop(proc)
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

  openApp: async ({ server, newPage, git }, use) => {
    await use(async (worktree) => {
      const page = await newPage()
      await load(page, server.url, worktree, git)
      return page
    })
  },

  // Server mode drops events emitted before the page's event websocket
  // connects, so the page is ready once it has.
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
    editorProfiles: {
      '': {
        name: 'Stand-in',
        folderCmd: `${editor} {path}`,
        fileCmd: `${editor} {path}`,
        fileAtLineCmd: `${editor} {path}:{line}`,
      },
    },
    protectedBranches: ['main', 'master'],
    defaultPullMode: PullMode.PullMerge,
    diffMode: DiffMode.UnifiedDiff,
    autoFetchIntervalMinutes: 0,
    ...overrides,
  }
}

// load opens worktree's page and waits until its log shows. Server mode
// drops events emitted before the page's event websocket connects, and the
// log's first load can be among them. Once connected, load creates and
// then deletes a tag, waiting each time for the log to reload; a reload
// happens only when the refs differ, so both steps are needed.
async function load(page: Page, url: string, worktree: string, git: Git): Promise<void> {
  const events: WailsEvent[] = []
  page.on('websocket', (ws) =>
    ws.on('framereceived', ({ payload }) => events.push(JSON.parse(String(payload)))),
  )
  const connected = waitForEvents(page)
  await page.goto(`${url}/?path=${encodeURIComponent(worktree)}`)
  await connected
  await changeRefsAndWaitForReload(events, () => git(worktree, 'tag', 'e2e-nudge'))
  await changeRefsAndWaitForReload(events, () => git(worktree, 'tag', '-d', 'e2e-nudge'))
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
  data?: { scope?: ChangeScope; done?: boolean; filtered?: boolean }
}

async function changeRefsAndWaitForReload(events: WailsEvent[], change: () => void): Promise<void> {
  const before = events.length
  change()
  await expect.poll(() => reloadedAfterRefsChange(events.slice(before))).toBe(true)
}

// reloadedAfterRefsChange: a refs change arrived, and after it the full
// log finished loading.
function reloadedAfterRefsChange(events: WailsEvent[]): boolean {
  const changed = events.findIndex(
    (e) => e.name === 'repo:changed' && e.data?.scope === ChangeScope.RefsChanged,
  )
  return (
    changed >= 0 &&
    events
      .slice(changed + 1)
      .some((e) => e.name === 'log:progress' && e.data?.done === true && !e.data.filtered)
  )
}

function freePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const probe = net.createServer()
    probe.once('error', reject)
    probe.listen(0, 'localhost', () => {
      const { port } = probe.address() as net.AddressInfo
      probe.close(() => resolve(port))
    })
  })
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
