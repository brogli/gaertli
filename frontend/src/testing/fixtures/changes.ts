import type { ChangedFiles } from '../../../bindings/github.com/brogli/nebegit/internal/core'

// changedFilesFixture is a Status answer.
export function changedFilesFixture(overrides: Partial<ChangedFiles> = {}): ChangedFiles {
  return { staged: [], unstaged: [], untracked: [], conflicted: [], ...overrides }
}
