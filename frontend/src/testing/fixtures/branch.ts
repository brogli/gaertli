import { BranchInUse, type Branch } from '../../../bindings/github.com/brogli/gaertli/internal/core'

// branchFixture is the current "main" branch with no upstream.
export function branchFixture(overrides: Partial<Branch> = {}): Branch {
  return {
    name: 'main',
    sha: 'sha-main',
    current: true,
    upstream: '',
    ahead: 0,
    behind: 0,
    upstreamGone: false,
    checkedOutIn: '',
    inUse: BranchInUse.$zero,
    ...overrides,
  }
}
