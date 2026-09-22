import type { Worktree } from '../../../bindings/github.com/brogli/nebegit/internal/core'

// worktreeFixture is one panel row; a Worktree shape change is updated here
// once, not per test file.
export function worktreeFixture(overrides: Partial<Worktree> = {}): Worktree {
  return {
    path: '/repoA/wt',
    branch: 'feature',
    detached: false,
    sha: 'sha1',
    main: false,
    locked: false,
    missing: false,
    dirty: false,
    ...overrides,
  }
}
