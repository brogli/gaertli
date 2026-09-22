import type { LogCommit } from '../../../bindings/github.com/brogli/nebegit/internal/core'

const ROW_COUNT = 300

// LOG_ROWS_FIXTURE: canned commit rows, more than a screenful, for the
// virtual-scroll e2e case (frontend/e2e/log-virtual-scroll.spec.ts) and
// reusable by future Vitest component tests wanting the same canned log
// (spec 13 Design).
export const LOG_ROWS_FIXTURE: LogCommit[] = Array.from({ length: ROW_COUNT }, (_, i) => {
  const sha = i.toString(16).padStart(40, '0')
  return {
    sha,
    shortSha: sha.slice(0, 7),
    author: 'Ada Fixture',
    authorAt: 1_700_000_000 - i * 60,
    subject: `commit ${i}`,
    graph: { lane: 0, colour: 0, edges: [] },
  }
})

// commitRow: the single canned LogCommit shared by LogList/WorktreeView/
// commitMenu/CommitDetailsPanel component tests (spec 40), overridable via
// partial for the rare test that needs a different field.
export function commitRow(overrides: Partial<LogCommit> = {}): LogCommit {
  return {
    sha: 'abc123',
    shortSha: 'abc123',
    author: 'Alice',
    authorAt: 0,
    subject: 'Fix the bug',
    graph: { lane: 0, colour: 0, edges: [] },
    ...overrides,
  }
}
