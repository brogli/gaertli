import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'

// A Sandbox is one test's private world: HOME, the app's config dir, git's
// global config, stand-in programs and the scratch repos. It lives under
// the OS temp dir, not the checkout: a repo nested in the checkout would
// see gaertli's own .git.
export interface Sandbox {
  dir: string
  // env is what the server and every helper git call run with.
  env: NodeJS.ProcessEnv
  editorLog: string
  settingsPath: string
}

export function createSandbox(): Sandbox {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'gaertli-e2e-'))
  const bin = path.join(dir, 'bin')
  for (const sub of [bin, path.join(dir, 'home'), path.join(dir, 'config', 'gaertli')]) {
    fs.mkdirSync(sub, { recursive: true })
  }
  const editorLog = path.join(dir, 'editor.log')
  const editor = writeStandIn(bin, 'stand-in-editor', editorLog)
  fs.writeFileSync(path.join(dir, 'gitconfig'), gitConfig(editor))
  return {
    dir,
    env: sandboxEnv(dir, bin),
    editorLog,
    settingsPath: path.join(dir, 'config', 'gaertli', 'settings.json'),
  }
}

// editorCalls lists every stand-in launch, one "<program> <args>" line each.
export function editorCalls(sandbox: Sandbox): string[] {
  if (!fs.existsSync(sandbox.editorLog)) return []
  return fs.readFileSync(sandbox.editorLog, 'utf8').split('\n').filter(Boolean)
}

// writeStandIn writes a program that logs its name and arguments instead
// of opening anything.
function writeStandIn(bin: string, name: string, log: string): string {
  const file = path.join(bin, name)
  fs.writeFileSync(file, `#!/bin/sh\nprintf '%s\\n' "${name} $*" >> '${log}'\n`, { mode: 0o755 })
  return file
}

function gitConfig(editor: string): string {
  return [
    '[user]',
    '\tname = Ann',
    '\temail = ann@example.com',
    '[init]',
    '\tdefaultBranch = main',
    '[core]',
    `\teditor = ${editor}`,
    '\tautocrlf = false',
    '',
  ].join('\n')
}

// sandboxEnv drops every inherited GIT_* variable and the editor variables,
// so git's editor resolves to the stand-in and nothing of the caller's git
// setup leaks in.
function sandboxEnv(dir: string, bin: string): NodeJS.ProcessEnv {
  const inherited = Object.fromEntries(
    Object.entries(process.env).filter(
      ([key]) => !key.startsWith('GIT_') && key !== 'VISUAL' && key !== 'EDITOR',
    ),
  )
  return {
    ...inherited,
    HOME: path.join(dir, 'home'),
    XDG_CONFIG_HOME: path.join(dir, 'config'),
    GIT_CONFIG_GLOBAL: path.join(dir, 'gitconfig'),
    GIT_CONFIG_NOSYSTEM: '1',
    PATH: `${bin}${path.delimiter}${process.env.PATH ?? ''}`,
  }
}
