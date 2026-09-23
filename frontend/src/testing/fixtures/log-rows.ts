import type { LogCommit } from '../../../bindings/github.com/brogli/nebegit/internal/core'

const ROW_COUNT = 300

// LOG_ROWS_FIXTURE: canned commit rows, more than a screenful, for the
// virtual-scroll and layout e2e cases (frontend/e2e/). Row 0 (HEAD) carries
// one ref label of every kind, for the layout case that checks a crowded
// row still leaves the subject readable.
export const LOG_ROWS_FIXTURE: LogCommit[] = Array.from({ length: ROW_COUNT }, (_, i) => {
  const sha = i.toString(16).padStart(40, '0')
  return {
    sha,
    shortSha: sha.slice(0, 7),
    author: 'Ada Fixture',
    authorAt: 1_700_000_000 - i * 60,
    subject: `commit ${i}`,
    graph: { lane: 0, colour: 0, edges: [] },
    merge: false,
    ...(i === 0
      ? {
          refs: [
            { name: 'HEAD', kind: 'head' },
            { name: 'main', kind: 'branch' },
            { name: 'origin/main', kind: 'remote' },
            { name: 'v1.2.3-fixture', kind: 'tag' },
          ],
        }
      : {}),
  }
})

// commitRow: the canned LogCommit shared by the component tests,
// overridable field by field.
export function commitRow(overrides: Partial<LogCommit> = {}): LogCommit {
  return {
    sha: 'abc123',
    shortSha: 'abc123',
    author: 'Alice',
    authorAt: 0,
    subject: 'Fix the bug',
    graph: { lane: 0, colour: 0, edges: [] },
    merge: false,
    ...overrides,
  }
}
