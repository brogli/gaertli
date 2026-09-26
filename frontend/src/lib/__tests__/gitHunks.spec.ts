import { describe, expect, it } from 'vitest'
import { Text } from '@codemirror/state'
import { Chunk } from '@codemirror/merge'
import type { DiffHunk } from '../../../bindings/github.com/brogli/nebegit/internal/core'
import { hunkChanges } from '../gitHunks'
import { hunk } from '@/testing/fixtures/diff'

// Lines is a first and last 1-based line number, or null for none.
type Lines = [number, number] | null

// gitLines is the lines h changes on each side.
function gitLines(h: DiffHunk): [Lines, Lines] {
  const lines = (start: number, count: number): Lines => (count ? [start, start + count - 1] : null)
  return [lines(h.oldStart, h.oldLines), lines(h.newStart, h.newLines)]
}

// chunkLines builds CodeMirror's chunks from git's hunks between a and b and
// returns the lines each chunk covers on each side.
function chunkLines(a: string, b: string, hunks: DiffHunk[]): [Lines, Lines][] {
  const docA = Text.of(a.split('\n'))
  const docB = Text.of(b.split('\n'))
  const lines = (doc: Text, from: number, to: number): Lines =>
    to > from ? [doc.lineAt(from).number, doc.lineAt(to - 1).number] : null
  return Chunk.build(docA, docB, { override: hunkChanges(hunks) }).map((c) => [
    lines(docA, c.fromA, c.toA),
    lines(docB, c.fromB, c.toB),
  ])
}

// Every hunk below is git diff -U0's for its two texts (myers unless named).
describe('hunkChanges', () => {
  it.each([
    ['a replace at the top', 'a\nb\nc\n', 'A\nb\nc\n', [hunk(1, 1, 1, 1)]],
    ['a replace in the middle', 'a\nb\nc\n', 'a\nB\nc\n', [hunk(2, 1, 2, 1)]],
    ['a replace at the end', 'a\nb\nc\n', 'a\nb\nC\n', [hunk(3, 1, 3, 1)]],
    ['an insert at the top', 'a\nb\n', 'x\na\nb\n', [hunk(0, 0, 1, 1)]],
    ['an insert in the middle', 'a\nb\nc\n', 'a\nb\nX\nc\n', [hunk(2, 0, 3, 1)]],
    ['an insert at the end', 'a\nb\n', 'a\nb\nx\n', [hunk(2, 0, 3, 1)]],
    ['a delete at the top', 'x\na\nb\n', 'a\nb\n', [hunk(1, 1, 0, 0)]],
    ['a delete in the middle', 'a\nb\nX\nc\n', 'a\nb\nc\n', [hunk(3, 1, 2, 0)]],
    ['a delete at the end', 'a\nb\nx\n', 'a\nb\n', [hunk(3, 1, 2, 0)]],
    ['a last line without a newline', 'a\nb', 'a\nB', [hunk(2, 1, 2, 1)]],
    [
      'deletes CodeMirror would slide',
      'baz qux\nbaz qux\n  x\nbar\n',
      'baz qux\n  x\n',
      [hunk(1, 1, 0, 0), hunk(4, 1, 2, 0)],
    ],
    ['an insert ending in a blank line', 'a\n\nb\n', 'a\n\nX\n\nb\n', [hunk(2, 0, 3, 2)]],
    ['a delete ending in a blank line', 'a\n\nX\n\nb\n', 'a\n\nb\n', [hunk(3, 2, 2, 0)]],
    [
      'a replace between blank lines',
      'a\n\nold\n\nb\n',
      'a\n\nnew\nmore\n\nb\n',
      [hunk(3, 1, 3, 2)],
    ],
    ['an empty old side', '', 'a\nb\n', [hunk(0, 0, 1, 2)]],
    ['an empty new side', 'a\nb\n', '', [hunk(1, 2, 0, 0)]],
    [
      'a histogram move',
      'alpha\nbeta\ngamma\nalpha\nbeta\ngamma\ndelta\n',
      'alpha\nbeta\ngamma\ndelta\nalpha\nbeta\ngamma\n',
      [hunk(4, 3, 3, 0), hunk(7, 0, 5, 3)],
    ],
    [
      'the same move by myers',
      'alpha\nbeta\ngamma\nalpha\nbeta\ngamma\ndelta\n',
      'alpha\nbeta\ngamma\ndelta\nalpha\nbeta\ngamma\n',
      [hunk(3, 0, 4, 1), hunk(7, 1, 7, 0)],
    ],
  ])("QA-COMP-01 each git hunk is one chunk on git's lines: %s", (_, a, b, hunks) => {
    expect(chunkLines(a, b, hunks)).toEqual(hunks.map(gitLines))
  })

  it.each([
    [
      'one blank line apart',
      'a\nb\n\nc\nd\n',
      'a\nB\n\nC\nd\n',
      [hunk(2, 1, 2, 1), hunk(4, 1, 4, 1)],
      [
        [2, 4],
        [2, 4],
      ],
    ],
    [
      'one one-character line apart (a histogram move)',
      'A\nB\nC\nA\nB\nC\nD\n',
      'A\nB\nC\nD\nA\nB\nC\n',
      [hunk(4, 3, 3, 0), hunk(7, 0, 5, 3)],
      [
        [4, 7],
        [4, 7],
      ],
    ],
  ])(
    'QA-COMP-01 two hunks fewer than 3 characters apart are one chunk covering both: %s',
    (_, a, b, hunks, want) => {
      expect(chunkLines(a, b, hunks)).toEqual([want])
    },
  )

  it('QA-COMP-01 a change to the final newline also covers the empty line after the side that ends in one', () => {
    expect(chunkLines('a\nb\n', 'a\nB', [hunk(2, 1, 2, 1)])).toEqual([
      [
        [2, 3],
        [2, 2],
      ],
    ])
  })
})
