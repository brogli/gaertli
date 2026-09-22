import { EditorState } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { getChunks, unifiedMergeView } from '@codemirror/merge'
import { describe, expect, it } from 'vitest'

// getChunks() on a unifiedMergeView (what DiffView.vue mounts) must map
// onto git's hunk boundaries: DiffView places its "Stage hunk" buttons
// from getChunks(). Each fixture's hunks are `git diff --no-index -U0 old
// new` (zero context, comparable to CodeMirror's un-padded chunks),
// expressed as the old/new text each hunk changes.
interface Fixture {
  name: string
  old: string
  new: string
  hunks: { old: string; new: string }[]
}

const fixtures: Fixture[] = [
  {
    // Two single-line changes 3 lines apart: git's default -U3 would merge
    // them into one hunk, CodeMirror's chunks stay separate.
    name: 'adjacent edits',
    old: 'line1\nline2\nline3\nline4\nline5\nline6\nline7\n',
    new: 'line1\nline2-changed\nline3\nline4\nline5-changed\nline6\nline7\n',
    hunks: [
      { old: 'line2', new: 'line2-changed' },
      { old: 'line5', new: 'line5-changed' },
    ],
  },
  {
    name: 'pure insert',
    old: 'a\nb\nc\n',
    new: 'a\nb\nNEW1\nNEW2\nc\n',
    hunks: [{ old: '', new: 'NEW1\nNEW2' }],
  },
  {
    name: 'pure delete',
    old: 'a\nb\nc\nd\ne\n',
    new: 'a\nb\ne\n',
    hunks: [{ old: 'c\nd', new: '' }],
  },
  {
    // Boundary conditions for both git's hunk header and CodeMirror's
    // chunk range: the very first and very last line of the document.
    name: 'edits at file start/end',
    old: 'first\nmid1\nmid2\nmid3\nlast\n',
    new: 'FIRST-CHANGED\nmid1\nmid2\nmid3\nLAST-CHANGED\n',
    hunks: [
      { old: 'first', new: 'FIRST-CHANGED' },
      { old: 'last', new: 'LAST-CHANGED' },
    ],
  },
]

// mountChunks builds the same unifiedMergeView shape DiffView.vue's
// mountView uses (readOnly, mergeControls: false) and returns its chunks.
function mountChunks(old: string, next: string) {
  const parent = document.createElement('div')
  document.body.appendChild(parent)
  const view = new EditorView({
    parent,
    state: EditorState.create({
      doc: next,
      extensions: [EditorState.readOnly.of(true), unifiedMergeView({ original: old, mergeControls: false })],
    }),
  })
  const chunks = getChunks(view.state)?.chunks ?? []
  view.destroy()
  parent.remove()
  return chunks
}

describe('ADR-7 CodeMirror chunk boundaries match git hunks', () => {
  for (const fixture of fixtures) {
    it(`ADR-7 ${fixture.name}: getChunks() maps cleanly onto git's own -U0 hunks`, () => {
      const chunks = mountChunks(fixture.old, fixture.new)

      const got = chunks.map((c) => ({
        old: fixture.old.slice(c.fromA, c.endA),
        new: fixture.new.slice(c.fromB, c.endB),
      }))

      // Every git hunk maps one-to-one, in order, onto getChunks()'s own
      // chunks — no union of consecutive ranges needed.
      expect(got).toEqual(fixture.hunks)
    })
  }
})
