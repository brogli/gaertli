import { execFileSync } from 'node:child_process'
import fs from 'node:fs'
import path from 'node:path'

// Repos is the scratch setup every test starts from: a bare remote, the
// main clone "work", a second clone "other", a linked worktree "work2" of
// work on branch side, and "scratch", an unrelated repo with no remote.
export interface Repos {
  remote: string
  work: string
  other: string
  work2: string
  scratch: string
}

export type Git = (repo: string, ...args: string[]) => string

// hunksText is hunks.txt (line 1 … line 30) with the given lines replaced.
export function hunksText(replaced: Record<number, string> = {}): string {
  return Array.from({ length: 30 }, (_, i) => `${replaced[i + 1] ?? `line ${i + 1}`}\n`).join('')
}

export function gitWith(env: NodeJS.ProcessEnv): Git {
  return (repo, ...args) => execFileSync('git', ['-C', repo, ...args], { env, encoding: 'utf8' })
}

// gitAfterHistory dates its commits two seconds past now, so they sort
// first in --topo-order even when the history was built in the same second.
export function gitAfterHistory(env: NodeJS.ProcessEnv): Git {
  const later = `${Math.floor(Date.now() / 1000) + 2} +0000`
  return gitWith({ ...env, GIT_AUTHOR_DATE: later, GIT_COMMITTER_DATE: later })
}

// createRepos builds work's history: main with a merge of side (by Bob),
// branches B and topic/x, feature with a change to conflict.txt that
// conflicts with main's, tag v1, and files for diff edge cases (multi-hunk,
// CRLF, no final newline, binary). main, B, topic/x and feature are pushed;
// other is a clone checked out on B.
export function createRepos(root: string, git: Git): Repos {
  const repos: Repos = {
    remote: path.join(root, 'remote.git'),
    work: path.join(root, 'work'),
    other: path.join(root, 'other'),
    work2: path.join(root, 'work2'),
    scratch: path.join(root, 'scratch'),
  }
  const { work } = repos
  const write = (file: string, text: string | Buffer) => {
    fs.mkdirSync(path.dirname(path.join(work, file)), { recursive: true })
    fs.writeFileSync(path.join(work, file), text)
  }
  const append = (file: string, text: string) => fs.appendFileSync(path.join(work, file), text)

  git(root, 'init', '-q', '--bare', '-b', 'main', repos.remote)
  git(root, 'init', '-q', '-b', 'main', work)
  git(work, 'remote', 'add', 'origin', repos.remote)

  write('hunks.txt', hunksText())
  write('conflict.txt', 'shared\n')
  write('crlf.txt', 'a\r\nb\r\nc\r\n')
  write('nonl.txt', 'no newline at end')
  write('docs/readme.md', '# Docs\n')
  write('src/main.go', 'package main\n')
  commitAll(git, work, 'initial: add base files')

  write('logo.png', Buffer.from('\x89PNG\r\n\x1a\n\x00\x00binary', 'latin1'))
  append('src/main.go', 'func a() {}\n')
  commitAll(git, work, 'add logo and func a')

  append('docs/readme.md', 'more docs\n')
  append('src/main.go', 'func b() {}\n')
  commitAll(git, work, 'docs and func b', 'Body line one.\nBody line two.')
  git(work, 'tag', 'v1')
  git(work, 'branch', 'B')
  git(work, 'branch', 'topic/x')

  git(work, 'checkout', '-q', '-b', 'feature')
  write('conflict.txt', 'feature side\n')
  commitAll(git, work, 'feature: change conflict.txt')
  write('feature.txt', 'feature only\n')
  commitAll(git, work, 'feature: add feature.txt')

  git(work, 'checkout', '-q', 'main')
  write('conflict.txt', 'main side\n')
  commitAll(git, work, 'main: change conflict.txt')

  git(work, 'checkout', '-q', '-b', 'side')
  write('side.txt', 'side file\n')
  commitAll(git, work, 'side: add side.txt')
  git(work, 'checkout', '-q', 'main')
  const asBob = ['-c', 'user.name=Bob', '-c', 'user.email=bob@example.com']
  git(work, ...asBob, 'merge', '-q', '--no-ff', 'side', '-m', "Merge branch 'side'")

  git(work, 'push', '-q', '-u', 'origin', 'main', 'B', 'topic/x', 'feature')
  git(work, 'branch', '-q', '--set-upstream-to=origin/B', 'B')
  git(root, 'clone', '-q', repos.remote, repos.other)
  git(repos.other, 'checkout', '-q', 'B')
  git(work, 'worktree', 'add', '-q', repos.work2, 'side')

  fs.mkdirSync(repos.scratch)
  git(repos.scratch, 'init', '-q', '-b', 'main')
  for (const subject of ['commit A', 'commit B', 'commit C']) {
    fs.appendFileSync(path.join(repos.scratch, 'file.txt'), `${subject}\n`)
    commitAll(git, repos.scratch, subject)
  }
  return repos
}

function commitAll(git: Git, repo: string, subject: string, body?: string): void {
  git(repo, 'add', '-A')
  git(repo, 'commit', '-q', '-m', subject, ...(body ? ['-m', body] : []))
}
