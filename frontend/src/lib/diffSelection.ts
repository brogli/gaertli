// The selected change of a diff's editors: outlined and marked in the gutter
// as the merge editor marks its selected region. Every editor of a diff holds
// the same chunk; each marks its own side's lines of it.

import {
  type Extension,
  type Line,
  RangeSetBuilder,
  StateEffect,
  StateField,
  type Text,
} from '@codemirror/state'
import { Decoration, EditorView, ViewPlugin } from '@codemirror/view'
import type { Chunk } from '@codemirror/merge'
import { selectedClasses, selectionGutter } from './selectedChange'

// DiffSide is the document an editor shows: the old (a), the new (b), or the
// new with the removed lines shown inline above each change (unified).
export type DiffSide = 'a' | 'b' | 'unified'

// selectChunk selects a chunk, or none.
export const selectChunk = StateEffect.define<Chunk | null>()

// A Selection is the selected chunk and its first and last line on the
// editor's side, or null lines when it has none there.
interface Selection {
  chunk: Chunk
  lines: { first: Line; last: Line } | null
}

// chunkRange is chunk's range on side; unified shows the new side's lines.
export function chunkRange(chunk: Chunk, side: DiffSide): { from: number; to: number } {
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
  const order = [...chunks.entries()]
  if (dir < 0) order.reverse()
  const beside = order.find(([, chunk]) => dir * (chunkRange(chunk, side).from - pos) > 0)
  return (beside ?? order[0])?.[0] ?? -1
}

// newSidePos is old-side pos on the new side: a change's start for a pos in
// it, else pos shifted by the changes before it.
export function newSidePos(chunks: readonly Chunk[], pos: number): number {
  let shift = 0
  for (const chunk of chunks) {
    if (pos < chunk.fromA) break
    if (pos < chunk.toA) return chunk.fromB
    shift = chunk.toB - chunk.toA
  }
  return pos + shift
}

function selectionOf(chunk: Chunk | null, side: DiffSide, doc: Text): Selection | null {
  if (!chunk) return null
  const { from, to } = chunkRange(chunk, side)
  if (from === to) return { chunk, lines: null }
  return {
    chunk,
    lines: { first: doc.lineAt(from), last: doc.lineAt(Math.min(to - 1, doc.length)) },
  }
}

// selectionField holds the selection of an editor showing side; its
// documents are read-only, so the lines last as long as the chunk.
function selectionField(side: DiffSide) {
  return StateField.define<Selection | null>({
    create: () => null,
    update: (selection, tr) =>
      tr.effects.reduce(
        (value, effect) =>
          effect.is(selectChunk) ? selectionOf(effect.value, side, tr.newDoc) : value,
        selection,
      ),
  })
}

// removedAbove says whether unified shows removed lines of chunk above it.
const removedAbove = (chunk: Chunk | undefined, side: DiffSide): chunk is Chunk =>
  side === 'unified' && !!chunk && chunk.fromA < chunk.toA

function lineDecorations(field: StateField<Selection | null>, side: DiffSide): Extension {
  return EditorView.decorations.compute([field], (state) => {
    const selection = state.field(field)
    if (!selection?.lines) return Decoration.none
    const { first, last } = selection.lines
    // Unified, the outline's top is on the removed block above.
    const top = removedAbove(selection.chunk, side) ? 0 : first.number
    const builder = new RangeSetBuilder<Decoration>()
    for (let n = first.number; n <= last.number; n++) {
      const from = state.doc.line(n).from
      builder.add(
        from,
        from,
        Decoration.line({ class: selectedClasses(n === top, n === last.number) }),
      )
    }
    return builder.finish()
  })
}

// markRemovedBlock gives the unified diff's removed block of the selected
// chunk the outline: @codemirror/merge draws that block as a widget of its
// own, so no decoration reaches it. The widget keeps its element while the
// chunk lasts, and the editor ignores changes inside a widget.
function markRemovedBlock(field: StateField<Selection | null>): Extension {
  return ViewPlugin.define((view) => {
    let marked = false
    const mark = () => {
      const chunk = view.state.field(field)?.chunk
      const selected = removedAbove(chunk, 'unified') ? chunk : null
      if (!selected && !marked) return
      marked = !!selected
      for (const block of view.contentDOM.querySelectorAll<HTMLElement>('.cm-deletedChunk')) {
        const on = !!selected && view.posAtDOM(block) === selected.fromB
        block.classList.toggle('cm-chunk-selected', on)
        block.classList.toggle('cm-chunk-first', on)
        block.classList.toggle('cm-chunk-last', on && selected.fromB === selected.toB)
      }
    }
    return {
      update: (update) => {
        if (update.startState.field(field) !== update.state.field(field)) mark()
      },
      docViewUpdate: mark,
    }
  })
}

// diffSelection marks the selected chunk in an editor showing side.
export function diffSelection(side: DiffSide): Extension {
  const field = selectionField(side)
  return [
    field,
    lineDecorations(field, side),
    side === 'unified' ? markRemovedBlock(field) : [],
    selectionGutter(
      field,
      (selection, line) =>
        !!selection?.lines &&
        selection.lines.first.from <= line.from &&
        line.from <= selection.lines.last.from,
      (selection, block) =>
        block.from === selection?.chunk.fromB && removedAbove(selection.chunk, side),
    ),
  ]
}
