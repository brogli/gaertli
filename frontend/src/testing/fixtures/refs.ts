import type { Branch, Refs } from '../../../bindings/github.com/brogli/nebegit/internal/core'
import { branchFixture } from './branch'

// refsFixture holds branchFixture's main with overrides and the named
// remotes, each without branches; no tags.
export function refsFixture(overrides: Partial<Branch> = {}, remotes: string[] = []): Refs {
  return {
    branches: [branchFixture(overrides)],
    remotes: remotes.map((name) => ({ name, branches: [] })),
    tags: [],
  }
}

// refsOfEveryKind holds branches, the remote branch origin/x and the tag v1.
export function refsOfEveryKind(branches: Branch[]): Refs {
  return {
    branches,
    remotes: [{ name: 'origin', branches: [{ name: 'x', sha: 'sha-x' }] }],
    tags: [{ name: 'v1', sha: 'sha-v1' }],
  }
}
