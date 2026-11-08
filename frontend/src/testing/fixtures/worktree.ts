import type { Worktree } from '../../../bindings/github.com/brogli/gaertli/internal/core'

// worktreeFixture is one worktree.
export function worktreeFixture(overrides: Partial<Worktree> = {}): Worktree {
  return {
    path: '/repoA/wt',
    branch: 'feature',
    sha: 'sha1',
    main: false,
    locked: false,
    missing: false,
    dirty: false,
    ...overrides,
  }
}
