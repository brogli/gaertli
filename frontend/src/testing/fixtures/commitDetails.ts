import type { CommitDetails } from '../../../bindings/github.com/brogli/nebegit/internal/core'

// commitDetailsFixture is a CommitDetails answer: Alice's "Fix the bug",
// committed by Bob, with no files.
export function commitDetailsFixture(overrides: Partial<CommitDetails> = {}): CommitDetails {
  return {
    sha: 'abc123',
    parents: [],
    author: 'Alice',
    authorEmail: 'alice@example.com',
    authorAt: 0,
    committer: 'Bob',
    committerEmail: 'bob@example.com',
    committerAt: 0,
    body: 'Fix the bug',
    files: [],
    ...overrides,
  }
}
