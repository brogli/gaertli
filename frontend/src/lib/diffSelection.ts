// The selected change of a diff's editors: outlined and marked in the gutter
// as the merge editor marks its selected region. Every editor of a diff holds
// the same chunk; each marks its own side's lines of it.

import { type EditorState, type Extension, StateEffect, StateField } from '@codemirror/state'
import { type BlockInfo, Decoration, EditorView, ViewPlugin } from '@codemirror/view'
import type { Chunk } from '@codemirror/merge'
import { selectedClasses, selectionGutter } from './codeEditor'

// DiffSide is the document an editor shows: the old (a), the new (b), or the
// new with the removed lines shown inline above each change (unified).
export type DiffSide = 'a' | 'b' | 'unified'

// selectChunk selects a chunk, or none.
export const selectChunk = StateEffect.define<Chunk | null>()

const selectedChunk = StateField.define<Chunk | null>({
  create: () => null,
  update: (chunk, tr) =>
    tr.effects.reduce((value, effect) => (effect.is(selectChunk) ? effect.value : value), chunk),
})

// chunkRange is chunk's range on side; unified shows the new side's lines.
function chunkRange(chunk: Chunk, side: DiffSide): { from: number; to: number } {
  return side === 'a' ? { from: chunk.fromA, to: chunk.toA } : { from: chunk.fromB, to: chunk.toB }
}

// chunkAt is the index of the chunk holding pos on side, or -1. A chunk with
// no lines on side holds its position there.
export function chunkAt(chunks: readonly Chunk[], side: DiffSide, pos: number): number {
  return chunks.findIndex((chunk) => {
    const { from, to } = chunkRange(chunk, side)
    return from === to ? pos === from : from <= pos && pos < to
  })
}

// chunkBeside is the index of the first chunk after pos on side (dir 1), or
// the last before it (dir -1), wrapping round the file.
export function chunkBeside(
  chunks: readonly Chunk[],
  side: DiffSide,
  pos: number,
  dir: 1 | -1,
): number {
  const starts = chunks.map((chunk) => chunkRange(chunk, side).from)
  if (dir > 0)
    return Math.max(
      starts.findIndex((from) => from > pos),
      0,
    )
  const before = starts.filter((from) => from < pos).length - 1
  return before >= 0 ? before : chunks.length - 1
}

// chunkStart is where the cursor goes on side when chunk is stepped to.
export function chunkStart(chunk: Chunk, side: DiffSide): number {
  return chunkRange(chunk, side).from
}

// selectedLines are the first and last line of the selected chunk on side,
// or null when it has none there.
function selectedLines(state: EditorState, side: DiffSide): [number, number] | null {
  const chunk = state.field(selectedChunk)
  if (!chunk) return null
  const { from, to } = chunkRange(chunk, side)
  if (from === to) return null
  return [
    state.doc.lineAt(from).number,
    state.doc.lineAt(Math.min(to - 1, state.doc.length)).number,
  ]
}

// removedAbove says whether unified shows removed lines of chunk above it.
const removedAbove = (chunk: Chunk | null, side: DiffSide): chunk is Chunk =>
  side === 'unified' && !!chunk && chunk.fromA < chunk.toA

function lineDecorations(side: DiffSide) {
  return EditorView.decorations.compute([selectedChunk], (state) => {
    const lines = selectedLines(state, side)
    if (!lines) return Decoration.none
    const [first, last] = lines
    // Unified, the outline's top is on the removed block above.
    const top = removedAbove(state.field(selectedChunk), side) ? 0 : first
    return Decoration.set(
      Array.from({ length: last - first + 1 }, (_, i) => first + i).map((n) =>
        Decoration.line({ class: selectedClasses(n === top, n === last) }).range(
          state.doc.line(n).from,
        ),
      ),
    )
  })
}

// markRemovedBlock gives the unified diff's removed block of the selected
// chunk the outline: @codemirror/merge draws that block as a widget of its
// own, so no decoration reaches it. The widget keeps its element while the
// chunk lasts, and the editor ignores changes inside a widget.
const markRemovedBlock = ViewPlugin.define((view) => {
  const mark = () => {
    const chunk = view.state.field(selectedChunk)
    const selected = removedAbove(chunk, 'unified') ? chunk : null
    for (const block of view.contentDOM.querySelectorAll<HTMLElement>('.cm-deletedChunk')) {
      const on = !!selected && view.posAtDOM(block) === selected.fromB
      block.classList.toggle('cm-chunk-selected', on)
      block.classList.toggle('cm-chunk-first', on)
      block.classList.toggle('cm-chunk-last', on && selected.fromB === selected.toB)
    }
  }
  return {
    update: (update) => {
      if (update.startState.field(selectedChunk) !== update.state.field(selectedChunk)) mark()
    },
    docViewUpdate: mark,
  }
})

// diffSelection marks the selected chunk in an editor showing side.
export function diffSelection(side: DiffSide): Extension {
  const inLines = (_: Chunk | null, line: BlockInfo, state: EditorState) => {
    const lines = selectedLines(state, side)
    const n = state.doc.lineAt(line.from).number
    return !!lines && lines[0] <= n && n <= lines[1]
  }
  const atRemovedBlock = (chunk: Chunk | null, block: BlockInfo) =>
    removedAbove(chunk, side) && block.from === chunk.fromB
  return [
    selectedChunk,
    lineDecorations(side),
    side === 'unified' ? markRemovedBlock : [],
    selectionGutter(selectedChunk, inLines, atRemovedBlock),
  ]
}
