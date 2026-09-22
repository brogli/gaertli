import type { ChangedFiles } from '../../../bindings/github.com/brogli/nebegit/internal/core'

// changedFilesFixture is a Status answer; a ChangedFiles shape change is
// updated here once, not per test file.
export function changedFilesFixture(overrides: Partial<ChangedFiles> = {}): ChangedFiles {
  return { staged: [], unstaged: [], untracked: [], conflicted: [], ...overrides }
}
