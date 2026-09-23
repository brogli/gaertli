import fs from 'node:fs'
import { test, expect } from './fixtures'

// The suite's own guarantees: real git behind the page, a sandbox per test,
// no reach into the owner's settings or editor.

test('FR-LOG-01 the log of the real repo lists commits of all branches', async ({
  app,
  sandbox,
  repos,
}) => {
  await expect(app.locator('.row .subject', { hasText: 'feature: add feature.txt' })).toBeVisible()
  await expect(app.locator('.row .subject', { hasText: "Merge branch 'side'" })).toBeVisible()
  const settings = JSON.parse(fs.readFileSync(sandbox.settingsPath, 'utf8')) as {
    openWorktrees: string[]
  }
  expect(settings.openWorktrees).toEqual([repos.work])
})

test("FR-ED-01 Open in editor runs the stand-in, never git's editor from the caller's shell", async ({
  app,
  repos,
  server,
  editorCalls,
}) => {
  await app.getByRole('button', { name: 'Open in Stand-in' }).click()
  await expect.poll(editorCalls).toEqual([`stand-in-editor ${repos.work}`])
  const environ = fs.readFileSync(`/proc/${server.pid}/environ`, 'utf8').split('\0')
  expect(environ.filter((v) => v.startsWith('GIT_EDITOR='))).toEqual([])
})

test.describe('in parallel', () => {
  test.describe.configure({ mode: 'parallel' })
  for (const n of [1, 2]) {
    test(`TR-31 parallel test ${n} gets its own server and sandbox`, async ({
      app,
      server,
      sandbox,
    }) => {
      await expect(app.locator('.row').first()).toBeVisible()
      expect(app.url()).toContain(server.url)
      expect(fs.existsSync(sandbox.dir)).toBe(true)
    })
  }
})
