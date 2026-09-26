import {
  EdgeKind,
  RefKind,
  type GraphRow,
  type LogCommit,
} from '../../../bindings/github.com/brogli/nebegit/internal/core'

const ROW_COUNT = 300
const NOW = Math.floor(Date.now() / 1000)
const AUTHORS = ['Ada Fixture', 'Bo', 'Maximiliane Longname-Fixture']

// LOG_ROWS_FIXTURE: canned commit rows, more than a screenful, for the
// virtual-scroll and layout e2e cases (frontend/e2e/). Rows differ in all
// that could push a column: lane span, ref labels, author, date and short
// hash length. Row 0 (HEAD) carries one ref label of every kind, for the
// layout case that checks a crowded row still leaves the subject readable.
export const LOG_ROWS_FIXTURE: LogCommit[] = Array.from({ length: ROW_COUNT }, (_, i) => {
  const sha = i.toString(16).padStart(40, '0')
  return {
    sha,
    shortSha: sha.slice(0, i % 10 === 5 ? 9 : 7),
    author: AUTHORS[i % AUTHORS.length]!,
    authorAt: NOW - i * i * 600,
    subject: `commit ${i}`,
    graph: sideBranchGraph(i),
    ...(i === 0
      ? {
          refs: [
            { name: 'HEAD', kind: RefKind.HeadRef },
            { name: 'main', kind: RefKind.BranchRef },
            { name: 'origin/main', kind: RefKind.RemoteRef },
            { name: 'v1.2.3-fixture', kind: RefKind.TagRef },
          ],
        }
      : i % 12 === 2
        ? { refs: [{ name: `side-${i}`, kind: RefKind.BranchRef }] }
        : {}),
  }
})

// sideBranchGraph: every six rows a side branch on lane 1, merged at the
// first of them and forked off lane 0 at the fifth; the sixth has lane 0
// alone.
function sideBranchGraph(i: number): GraphRow {
  const sideColour = 1 + Math.floor(i / 6)
  const main = (kind: EdgeKind) => ({ lane: 0, kind, colour: 0 })
  const side = (kind: EdgeKind) => ({ lane: 1, kind, colour: sideColour })
  const { InEdge: IN, OutEdge: OUT, ThroughEdge: THROUGH } = EdgeKind
  const above = i === 0 ? [] : [main(IN)]
  const edges = [
    [...above, main(OUT), side(OUT)],
    [main(IN), main(OUT), side(THROUGH)],
    [side(IN), side(OUT), main(THROUGH)],
    [main(IN), main(OUT), side(THROUGH)],
    [main(IN), side(IN), main(OUT)],
    [main(IN), main(OUT)],
  ][i % 6]!
  return i % 6 === 2 ? { lane: 1, colour: sideColour, edges } : { lane: 0, colour: 0, edges }
}

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
    ...overrides,
  }
}
