import { Change, type Chunk } from '@codemirror/merge'
import type { Text } from '@codemirror/state'
import type { DiffHunk } from '../../bindings/github.com/brogli/nebegit/internal/core'

// hunksIn returns the hunks chunk shows: one, or those CodeMirror joined
// into it. a and b are the chunk's old and new documents. A hunk belongs to
// the chunk whose lines overlap its lines on a side where it has some.
export function hunksIn(chunk: Chunk, hunks: readonly DiffHunk[], a: Text, b: Text): DiffHunk[] {
  const linesA = chunkLines(a, chunk.fromA, chunk.toA)
  const linesB = chunkLines(b, chunk.fromB, chunk.toB)
  return hunks.filter((h) =>
    h.oldLines
      ? overlaps(linesA, h.oldStart, h.oldLines)
      : overlaps(linesB, h.newStart, h.newLines),
  )
}

// chunkLines is the first and last line [from, to) covers in doc, or null
// for an empty range. to may point one past the end of doc.
function chunkLines(doc: Text, from: number, to: number): [number, number] | null {
  return to > from ? [doc.lineAt(from).number, doc.lineAt(to - 1).number] : null
}

function overlaps(lines: [number, number] | null, start: number, count: number): boolean {
  return !!lines && start <= lines[1] && start + count - 1 >= lines[0]
}

// hunkChanges is a DiffConfig.override that hands git's hunks to CodeMirror
// instead of letting it diff a and b itself. Each hunk becomes one Change
// over whole lines, from the start of its first line to the start of the
// line after it; a side with no lines sits at the start of the line after
// git's start line. CodeMirror's presentation pass slides a pure insert or delete to an
// equal neighbouring line; a zero-length Change pinned at each edge stops
// it. It still joins changes fewer than 3 characters apart into one chunk.
export function hunkChanges(
  hunks: readonly DiffHunk[],
): (a: string, b: string) => readonly Change[] {
  return (a, b) => {
    const lineA = lineStart(a)
    const lineB = lineStart(b)
    return hunks.flatMap((h) => {
      const [fromA, toA] = lineRange(lineA, h.oldStart, h.oldLines)
      const [fromB, toB] = lineRange(lineB, h.newStart, h.newLines)
      return [
        new Change(fromA, fromA, fromB, fromB),
        new Change(fromA, toA, fromB, toB),
        new Change(toA, toA, toB, toB),
      ]
    })
  }
}

// lineStart returns the offset in text where 1-based line n starts; the text
// length past the last line.
function lineStart(text: string): (n: number) => number {
  const starts = [0]
  for (let i = text.indexOf('\n'); i >= 0; i = text.indexOf('\n', i + 1)) starts.push(i + 1)
  return (n) => starts[n - 1] ?? text.length
}

// lineRange is the [from, to) offsets of count lines at git's start.
function lineRange(lineAt: (n: number) => number, start: number, count: number): [number, number] {
  const first = count ? start : start + 1
  return [lineAt(first), lineAt(first + count)]
}
