import {
  ConflictKind,
  Operation,
  type ConflictFile,
} from '../../../bindings/github.com/brogli/nebegit/internal/core'

// conflictResult is a working file with two marker blocks, as git writes
// them in merge style.
export const conflictResult = [
  'a',
  '<<<<<<< HEAD',
  'c1',
  '=======',
  'i1',
  '>>>>>>> side',
  'b',
  '<<<<<<< HEAD',
  'c2',
  '=======',
  'i2',
  '>>>>>>> side',
  'z',
].join('\n')

// conflictFileFixture is a text ConflictFile of a merge, holding
// conflictResult's two regions.
export function conflictFileFixture(overrides: Partial<ConflictFile> = {}): ConflictFile {
  return {
    path: 'a.txt',
    kind: ConflictKind.ConflictText,
    operation: Operation.MergeOperation,
    base: { present: true, content: 'a\no1\nb\no2\nz' },
    current: {
      present: true,
      content: 'a\nc1\nb\nc2\nz',
      sha: '1234567890abcdef',
      ref: 'main',
      subject: 'on main',
    },
    incoming: {
      present: true,
      content: 'a\ni1\nb\ni2\nz',
      sha: 'abcdef0123456789',
      ref: 'side',
      subject: 'on side',
    },
    result: conflictResult,
    markerSize: 7,
    regions: [
      {
        result: { start: 2, lines: 5 },
        base: { start: 2, lines: 1 },
        current: { start: 2, lines: 1 },
        incoming: { start: 2, lines: 1 },
      },
      {
        result: { start: 8, lines: 5 },
        base: { start: 4, lines: 1 },
        current: { start: 4, lines: 1 },
        incoming: { start: 4, lines: 1 },
      },
    ],
    fingerprint: 'fp',
    ...overrides,
  }
}

// scrollConflictFixture is a ConflictFile of three one-line conflicts far
// apart. The lines between them differ in number per pane, so the panes line
// up only at the conflicts: the current side's conflicts start at lines 20,
// 100 and 180, the incoming side's at 20, 130 and 230, the result's, which
// starts from the base text, at 20, 100 and 180.
export function scrollConflictFixture(): ConflictFile {
  const currentGaps = [19, 79, 79, 100]
  const incomingGaps = [19, 109, 99, 100]
  const withRegions = (gaps: number[], side: string, region: (n: number) => string[]) =>
    gaps.flatMap((gap, i) => [
      ...Array.from({ length: gap }, (_, j) => `${side} filler ${j + 1}`),
      ...(i < 3 ? region(i + 1) : []),
    ])
  const currentLines = withRegions(currentGaps, 'current', (n) => [`current region ${n}`])
  const incomingLines = withRegions(incomingGaps, 'incoming', (n) => [`incoming region ${n}`])
  const baseLines = withRegions(currentGaps, 'base', (n) => [`base region ${n}`])
  const resultLines = withRegions(currentGaps, 'result', (n) => [
    `<<<<<<< HEAD ${n}`,
    `current region ${n}`,
    '=======',
    `incoming region ${n}`,
    `>>>>>>> side ${n}`,
  ])

  // regionStart is the first line of conflict i (0-based) when each holds
  // regionLines lines.
  const regionStart = (gaps: number[], regionLines: number, i: number) =>
    gaps.slice(0, i + 1).reduce((sum, gap) => sum + gap, 0) + i * regionLines + 1
  const regions = [0, 1, 2].map((i) => ({
    result: { start: regionStart(currentGaps, 5, i), lines: 5 },
    base: { start: regionStart(currentGaps, 1, i), lines: 1 },
    current: { start: regionStart(currentGaps, 1, i), lines: 1 },
    incoming: { start: regionStart(incomingGaps, 1, i), lines: 1 },
  }))

  const fixture = conflictFileFixture()
  return {
    ...fixture,
    path: 'scroll.txt',
    base: { present: true, content: baseLines.join('\n') },
    current: { ...fixture.current, content: currentLines.join('\n') },
    incoming: { ...fixture.incoming, content: incomingLines.join('\n') },
    result: resultLines.join('\n'),
    regions,
    // The current side changed line 5 and added line 6, outside the conflicts.
    currentChanges: [
      { lines: { start: 5, lines: 1 }, added: false },
      { lines: { start: 6, lines: 1 }, added: true },
    ],
  }
}
