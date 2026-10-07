import { describe, expect, it } from 'vitest'
import { EditorState, Text } from '@codemirror/state'
import { EditorView, type DecorationSet } from '@codemirror/view'
import { history, undo } from '@codemirror/commands'
import {
  hasStrayMarkers,
  isUnresolved,
  markDone,
  nextUnresolved,
  regionStart,
  regionsExtension,
  regionFromLineRange,
  resultRegions,
  selectedRegion,
  selectRegion,
  setSideRegionStates,
  sideRegionAt,
  sideRegionStart,
  sideRegionsExtension,
  takeRegion,
  unresolvedCount,
  type LineRange,
} from '../conflictRegions'

const text = ['a', 'o1', 'b', 'o2', 'z'].join('\n')
const ranges = [
  { start: 2, lines: 1 },
  { start: 4, lines: 1 },
]

function resultState(doc = text, initial = ranges): EditorState {
  return EditorState.create({
    doc,
    extensions: [
      history(),
      regionsExtension(
        initial.map((range) => ({ range, resolved: false })),
        7,
      ),
    ],
  })
}

function lineRanges(state: EditorState): LineRange[] {
  return resultRegions(state).map(({ range }) => range)
}

function undone(state: EditorState): EditorState {
  let result = state
  undo({ state, dispatch: (tr) => (result = tr.state) })
  return result
}

// lineClasses maps each decorated line number to its classes.
function lineClasses(state: EditorState): Record<number, string> {
  const result: Record<number, string> = {}
  for (const source of state.facet(EditorView.decorations)) {
    if (typeof source === 'function') continue
    const set: DecorationSet = source
    for (const iter = set.iter(); iter.value; iter.next()) {
      const line = state.doc.lineAt(iter.from).number
      result[line] = String(iter.value.spec.class)
    }
  }
  return result
}

function lineOf(state: EditorState, content: string): { from: number; to: number } {
  for (let n = 1; n <= state.doc.lines; n++) {
    const line = state.doc.line(n)
    if (line.text === content) return line
  }
  throw new Error(`no line ${content}`)
}

describe('conflictRegions result pane', () => {
  it('FR-CONF-05 reads the regions back as line ranges, empty ones as the line before', () => {
    expect(lineRanges(resultState())).toEqual(ranges)
    const empty = [
      { start: 1, lines: 0 },
      { start: 3, lines: 0 },
    ]
    expect(lineRanges(resultState('a\nb\nc', empty))).toEqual(empty)
  })

  it('FR-CONF-05 converts line ranges to offsets', () => {
    const doc = Text.of(text.split('\n'))
    const regions = ranges.map((range) => regionFromLineRange(doc, range))
    expect(text.slice(regions[0]?.from, regions[0]?.to)).toBe('o1')
    expect(text.slice(regions[1]?.from, regions[1]?.to)).toBe('o2')
  })

  it('FR-CONF-05 every region starts unresolved, and a hand edit leaves it so', () => {
    const state = resultState()
    expect(unresolvedCount(state)).toBe(2)

    const typed = state.update({ changes: { from: lineOf(state, 'o1').to, insert: 'x' } }).state

    expect(typed.doc.line(2).text).toBe('o1x')
    expect(isUnresolved(typed, 0)).toBe(true)
    expect(unresolvedCount(typed)).toBe(2)
  })

  it('FR-CONF-05 take incoming replaces the region, resolves it and keeps its range', () => {
    const state = resultState()
    const next = state.update(takeRegion(state, 0, ['i1', 'i1b'])).state
    expect(next.doc.toString().split('\n')).toEqual(['a', 'i1', 'i1b', 'b', 'o2', 'z'])
    expect(isUnresolved(next, 0)).toBe(false)
    expect(isUnresolved(next, 1)).toBe(true)
    expect(lineRanges(next)).toEqual([
      { start: 2, lines: 2 },
      { start: 5, lines: 1 },
    ])
    expect(regionStart(next, 0)).toBe(lineOf(next, 'i1').from)
    expect(selectedRegion(next)).toBe(0)
  })

  it('FR-CONF-05 a take is one change', () => {
    const state = resultState()
    expect(Array.isArray(takeRegion(state, 1, ['x', 'y']).changes)).toBe(false)
  })

  it('FR-CONF-05 an empty take removes the region lines', () => {
    const state = resultState()
    const next = state.update(takeRegion(state, 0, [])).state
    expect(next.doc.toString()).toBe('a\nb\no2\nz')
    expect(isUnresolved(next, 0)).toBe(false)
    expect(lineRanges(next)[0]).toEqual({ start: 1, lines: 0 })
  })

  it('FR-CONF-05 a take fills an empty region, at the end of the text too', () => {
    const middle = resultState('a\nb', [{ start: 1, lines: 0 }])
    expect(middle.update(takeRegion(middle, 0, ['x', 'y'])).state.doc.toString()).toBe('a\nx\ny\nb')
    expect(lineRanges(middle.update(takeRegion(middle, 0, ['x', 'y'])).state)).toEqual([
      { start: 2, lines: 2 },
    ])

    const end = resultState('a\nb', [{ start: 2, lines: 0 }])
    expect(end.update(takeRegion(end, 0, ['x'])).state.doc.toString()).toBe('a\nb\nx')

    const trailing = resultState('a\nb\n', [{ start: 2, lines: 0 }])
    expect(trailing.update(takeRegion(trailing, 0, ['x'])).state.doc.toString()).toBe('a\nb\nx\n')
  })

  it.each([
    ['text', ['foo'], 'a\nX\nfooc\n'],
    ['text, then a line break', ['foo', '\n'], 'a\nX\nfoo\nc\n'],
  ])(
    'FR-CONF-05 %s typed at the start of the line after an empty region stays outside it',
    (_, keys, taken) => {
      let edited = resultState('a\nc\n', [{ start: 1, lines: 0 }])
      let at = 2
      for (const key of keys) {
        edited = edited.update({ changes: { from: at, insert: key } }).state
        at += key.length
      }

      expect(lineRanges(edited)).toEqual([{ start: 1, lines: 0 }])
      expect(edited.update(takeRegion(edited, 0, ['X'])).state.doc.toString()).toBe(taken)
    },
  )

  it('FR-CONF-05 text typed at the end of a text without a final line break stays outside an empty region there', () => {
    const state = resultState('a\nb', [{ start: 2, lines: 0 }])
    const edited = state.update({ changes: { from: 3, insert: 'foo' } }).state

    expect(edited.update(takeRegion(edited, 0, ['X'])).state.doc.toString()).toBe('a\nbfoo\nX')
  })

  it('FR-CONF-05 mark done resolves the region as it stands', () => {
    const state = resultState()
    const edited = state.update({ changes: { from: lineOf(state, 'o2').to, insert: '!' } }).state

    const done = edited.update(markDone(1)).state

    expect(done.doc.toString()).toBe(edited.doc.toString())
    expect(isUnresolved(done, 1)).toBe(false)
    expect(unresolvedCount(done)).toBe(1)
    expect(selectedRegion(done)).toBe(1)
  })

  it('FR-CONF-05 undoing a take or a mark restores the unresolved region and its text', () => {
    const state = resultState()
    const taken = state.update(takeRegion(state, 0, ['i1'])).state
    const marked = taken.update(markDone(1)).state

    const afterMark = undone(marked)
    expect(isUnresolved(afterMark, 1)).toBe(true)
    expect(isUnresolved(afterMark, 0)).toBe(false)

    const afterTake = undone(afterMark)
    expect(afterTake.doc.toString()).toBe(text)
    expect(unresolvedCount(afterTake)).toBe(2)
  })

  it.each([
    ['in the middle', 'a\no\nb'],
    ['at the end without a final line break', 'a\no'],
  ])('FR-CONF-05 undoing an empty take restores the region exactly, %s', (_, doc) => {
    const state = resultState(doc, [{ start: 2, lines: 1 }])
    const taken = state.update(takeRegion(state, 0, [])).state
    expect(isUnresolved(taken, 0)).toBe(false)

    const back = undone(taken)

    expect(back.doc.toString()).toBe(doc)
    expect(lineRanges(back)).toEqual([{ start: 2, lines: 1 }])
    expect(isUnresolved(back, 0)).toBe(true)
  })

  it('FR-CONF-05 restores the region states it was created with', () => {
    const state = EditorState.create({
      doc: text,
      extensions: regionsExtension(
        [
          { range: ranges[0]!, resolved: true },
          { range: ranges[1]!, resolved: false },
        ],
        7,
      ),
    })
    expect(resultRegions(state).map((region) => region.resolved)).toEqual([true, false])
    expect(unresolvedCount(state)).toBe(1)
  })

  it('FR-CONF-05 next and previous skip resolved regions', () => {
    const state = resultState()
    const afterTake = state.update(takeRegion(state, 1, ['x'])).state
    expect(nextUnresolved(state, 0, 1)).toBe(1)
    expect(nextUnresolved(state, 1, -1)).toBe(0)
    expect(nextUnresolved(afterTake, 0, 1)).toBe(-1)
    expect(nextUnresolved(afterTake, 2, -1)).toBe(0)
    expect(nextUnresolved(state, -1, 1)).toBe(0)
  })

  it('FR-CONF-05 a region starts where its first line starts, following edits', () => {
    const state = resultState()
    expect(regionStart(state, 1)).toBe(state.doc.line(4).from)
    const typed = state.update({ changes: { from: 0, insert: 'xy' } }).state
    expect(regionStart(typed, 1)).toBe(regionStart(state, 1) + 2)
  })

  it('FR-CONF-05 the cursor moving into a region selects it; outside, the selection stays', () => {
    const state = resultState()
    const inside = state.update({ selection: { anchor: lineOf(state, 'o2').from } }).state
    expect(selectedRegion(inside)).toBe(1)
    const outside = inside.update({ selection: { anchor: lineOf(inside, 'b').from } }).state
    expect(selectedRegion(outside)).toBe(1)
  })

  it('FR-CONF-05 selecting a region the result does not have is ignored', () => {
    const state = resultState().update({ effects: selectRegion.of(1) }).state
    const next = state.update({ effects: selectRegion.of(7) }).state

    expect(selectedRegion(next)).toBe(1)
  })

  it('FR-CONF-05 tints unresolved regions red and resolved ones muted, the selected one stronger', () => {
    const state = resultState()
    expect(lineClasses(state)).toEqual({
      2: 'cm-conflict-unresolved',
      4: 'cm-conflict-unresolved',
    })
    const selected = state.update({ effects: selectRegion.of(1) }).state
    expect(lineClasses(selected)[4]).toBe('cm-conflict-unresolved cm-conflict-selected')
    const resolved = selected.update(takeRegion(selected, 1, ['x'])).state
    expect(lineClasses(resolved)[4]).toBe('cm-conflict-resolved cm-conflict-selected')
    expect(lineClasses(resolved)[2]).toBe('cm-conflict-unresolved')
  })

  it('FR-CONF-05 finds marker lines anywhere in the result', () => {
    const state = resultState()
    expect(hasStrayMarkers(state)).toBe(false)
    expect(
      hasStrayMarkers(state.update({ changes: { from: 0, insert: '>>>>>>> x\n' } }).state),
    ).toBe(true)
  })
})

describe('conflictRegions side panes', () => {
  const sideDoc = ['a', 'c1', 'b', 'c2', 'd', 'e', 'f'].join('\n')
  const sideRanges = [
    { start: 2, lines: 1 },
    { start: 4, lines: 1 },
  ]
  const changes = [
    { lines: { start: 2, lines: 1 }, added: false },
    { lines: { start: 5, lines: 1 }, added: false },
    { lines: { start: 6, lines: 2 }, added: true },
  ]

  it('FR-CONF-05 tints changed lines blue and added lines green, region lines over them', () => {
    const state = EditorState.create({
      doc: sideDoc,
      extensions: sideRegionsExtension(sideRanges, changes),
    })
    expect(lineClasses(state)).toEqual({
      2: 'cm-conflict-unresolved',
      4: 'cm-conflict-unresolved',
      5: 'cm-change-changed',
      6: 'cm-change-added',
      7: 'cm-change-added',
    })
  })

  it('FR-CONF-05 shows resolved and selected regions once told', () => {
    const state = EditorState.create({
      doc: sideDoc,
      extensions: sideRegionsExtension(sideRanges, []),
    })
    const next = state.update({
      effects: setSideRegionStates.of({ resolved: [true, false], selected: 1 }),
    }).state
    expect(lineClasses(next)).toEqual({
      2: 'cm-conflict-resolved',
      4: 'cm-conflict-unresolved cm-conflict-selected',
    })
  })

  it('FR-CONF-05 finds the side region at a position', () => {
    const state = EditorState.create({
      doc: sideDoc,
      extensions: sideRegionsExtension(sideRanges, []),
    })
    expect(sideRegionAt(state, state.doc.line(4).from + 1)).toBe(1)
    expect(sideRegionAt(state, 0)).toBe(-1)
    expect(sideRegionStart(state, 1)).toBe(state.doc.line(4).from)
    expect(sideRegionStart(state, 2)).toBeUndefined()
  })
})
