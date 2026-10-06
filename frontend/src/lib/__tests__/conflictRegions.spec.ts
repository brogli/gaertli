import { describe, expect, it } from 'vitest'
import { EditorState, Text } from '@codemirror/state'
import { EditorView, type DecorationSet } from '@codemirror/view'
import { history, undo } from '@codemirror/commands'
import {
  canTakeRegion,
  isUnresolved,
  nextUnresolved,
  regionLineRanges,
  regionStart,
  selectedRegion,
  regionsExtension,
  regionsFromLineRanges,
  selectRegion,
  setSideRegionStates,
  sideRegionAt,
  sideRegionStart,
  sideRegionsExtension,
  takeRegion,
  unresolvedCount,
} from '../conflictRegions'

const text = [
  'a',
  '<<<<<<< HEAD',
  'c1',
  '=======',
  'i1',
  '>>>>>>> s',
  'b',
  '<<<<<<< HEAD',
  'c2',
  '||||||| base',
  'o2',
  '=======',
  'i2',
  '>>>>>>> s',
  'z',
].join('\n')
const ranges = [
  { start: 2, lines: 5 },
  { start: 8, lines: 7 },
]

it('FR-CONF-05 reads the regions back as line ranges, empty ones as the line before', () => {
  expect(regionLineRanges(resultState())).toEqual(ranges)
  const doc = 'a\nb\nc'
  const empty = [
    { start: 1, lines: 0 },
    { start: 3, lines: 0 },
  ]
  const state = EditorState.create({ doc, extensions: regionsExtension(empty, 7) })
  expect(regionLineRanges(state)).toEqual(empty)
})

function resultState(doc = text): EditorState {
  return EditorState.create({ doc, extensions: regionsExtension(ranges, 7) })
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
  it('FR-CONF-05 converts line ranges to offsets', () => {
    const regions = regionsFromLineRanges(Text.of(text.split('\n')), ranges)
    expect(text.slice(regions[0]?.from, regions[0]?.to)).toBe(
      '<<<<<<< HEAD\nc1\n=======\ni1\n>>>>>>> s',
    )
    expect(text.slice(regions[1]?.from, regions[1]?.to).endsWith('>>>>>>> s')).toBe(true)
  })

  it('FR-CONF-05 counts unresolved regions', () => {
    const state = resultState()
    expect(isUnresolved(state, 0)).toBe(true)
    expect(unresolvedCount(state)).toBe(2)
  })

  it('FR-CONF-05 take incoming resolves one region and keeps its range around the inserted lines', () => {
    const state = resultState()
    const next = state.update(takeRegion(state, 0, 'incoming')).state
    expect(next.doc.toString().split('\n').slice(0, 3)).toEqual(['a', 'i1', 'b'])
    expect(isUnresolved(next, 0)).toBe(false)
    expect(isUnresolved(next, 1)).toBe(true)
    const line = lineOf(next, 'i1')
    expect(regionStart(next, 0)).toBe(line.from)
    expect(selectedRegion(next)).toBe(0)
  })

  it('FR-CONF-05 take builds both orders as one change', () => {
    const state = resultState()
    const spec = takeRegion(state, 1, 'incomingFirst')
    const next = state.update(spec).state
    expect(next.doc.toString().split('\n').slice(6)).toEqual(['b', 'i2', 'c2', 'z'])
    expect(Array.isArray(spec.changes)).toBe(false)
  })

  it('FR-CONF-05 an empty section removes the block lines', () => {
    const doc = ['a', '<<<<<<< x', '=======', 'i', '>>>>>>> y', 'b'].join('\n')
    const state = EditorState.create({
      doc,
      extensions: regionsExtension([{ start: 2, lines: 4 }], 7),
    })
    const next = state.update(takeRegion(state, 0, 'current')).state
    expect(next.doc.toString()).toBe('a\nb')
    expect(isUnresolved(next, 0)).toBe(false)
  })

  it('FR-CONF-05 typing inside a region keeps it unresolved; deleting its markers resolves it', () => {
    const state = resultState()
    const c2 = lineOf(state, 'c2')
    const typed = state.update({ changes: { from: c2.to, insert: 'x' } }).state
    expect(isUnresolved(typed, 1)).toBe(true)
    expect(isUnresolved(typed, 0)).toBe(true)

    const changes = [8, 10, 12, 14]
      .map((n) => typed.doc.line(n))
      .map(({ from, to }) => ({ from, to }))
    const resolved = typed.update({ changes }).state
    expect(unresolvedCount(resolved)).toBe(1)
    expect(isUnresolved(resolved, 1)).toBe(false)
  })

  it('FR-CONF-05 a stray marker line makes a resolved region unresolved again', () => {
    const state = resultState()
    const taken = state.update(takeRegion(state, 0, 'current')).state
    const line = lineOf(taken, 'c1')
    const stray = taken.update({ changes: { from: line.to, insert: '\n>>>>>>> s' } }).state
    expect(isUnresolved(stray, 0)).toBe(true)
  })

  it('FR-CONF-05 take is a no-op once the block has no markers', () => {
    const state = resultState()
    const resolved = state.update(takeRegion(state, 0, 'current')).state
    expect(takeRegion(resolved, 0, 'incoming')).toEqual({})
  })

  it('FR-CONF-05 next and previous skip resolved regions', () => {
    const state = resultState()
    const afterTake = state.update(takeRegion(state, 1, 'current')).state
    expect(nextUnresolved(state, 0, 1)).toBe(1)
    expect(nextUnresolved(state, 1, -1)).toBe(0)
    expect(nextUnresolved(afterTake, 0, 1)).toBe(-1)
    expect(nextUnresolved(afterTake, 2, -1)).toBe(0)
    expect(nextUnresolved(state, -1, 1)).toBe(0)
  })

  it('FR-CONF-05 a region starts where its first line starts, following edits', () => {
    const state = resultState()
    expect(regionStart(state, 1)).toBe(state.doc.line(8).from)
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

  it('FR-CONF-05 styles marker and section lines, resolved and selected regions', () => {
    const state = resultState()
    expect(lineClasses(state)).toEqual({
      2: 'cm-conflict-marker',
      3: 'cm-conflict-current',
      4: 'cm-conflict-marker',
      5: 'cm-conflict-incoming',
      6: 'cm-conflict-marker',
      8: 'cm-conflict-marker',
      9: 'cm-conflict-current',
      10: 'cm-conflict-marker',
      11: 'cm-conflict-base',
      12: 'cm-conflict-marker',
      13: 'cm-conflict-incoming',
      14: 'cm-conflict-marker',
    })
    const selected = state.update({ effects: selectRegion.of(1) }).state
    expect(lineClasses(selected)[11]).toBe('cm-conflict-base cm-conflict-selected')
    expect(lineClasses(selected)[3]).toBe('cm-conflict-current')
    const resolved = selected.update(takeRegion(selected, 1, 'current')).state
    expect(lineClasses(resolved)[8]).toBe('cm-conflict-resolved cm-conflict-selected')
  })
})

describe('conflictRegions side panes', () => {
  const sideDoc = ['a', 'c1', 'b', 'c2', 'd'].join('\n')
  const sideRanges = [
    { start: 2, lines: 1 },
    { start: 4, lines: 1 },
  ]

  it('FR-CONF-05 styles each side region by its tone, and by resolved and selected once told', () => {
    const state = EditorState.create({
      doc: sideDoc,
      extensions: sideRegionsExtension(sideRanges, 'incoming'),
    })
    expect(lineClasses(state)).toEqual({ 2: 'cm-conflict-incoming', 4: 'cm-conflict-incoming' })
    const next = state.update({
      effects: setSideRegionStates.of({ resolved: [true, false], selected: 1 }),
    }).state
    expect(lineClasses(next)).toEqual({
      2: 'cm-conflict-resolved',
      4: 'cm-conflict-incoming cm-conflict-selected',
    })
  })

  it('FR-CONF-05 finds the side region at a position', () => {
    const state = EditorState.create({
      doc: sideDoc,
      extensions: sideRegionsExtension(sideRanges, 'base'),
    })
    expect(sideRegionAt(state, state.doc.line(4).from + 1)).toBe(1)
    expect(sideRegionAt(state, 0)).toBe(-1)
    expect(sideRegionStart(state, 1)).toBe(state.doc.line(4).from)
    expect(sideRegionStart(state, 2)).toBeUndefined()
  })
})

describe('conflictRegions edits', () => {
  function blockState(doc: string, lines: number): EditorState {
    const initial = [{ start: 2, lines }]
    return EditorState.create({ doc, extensions: [history(), regionsExtension(initial, 7)] })
  }

  function undone(state: EditorState): EditorState {
    let result = state
    undo({ state, dispatch: (tr) => (result = tr.state) })
    return result
  }

  it.each([
    ['in the middle', 'a\n<<<<<<< x\nc\n=======\n>>>>>>> y\nb'],
    ['at the end without a final line break', 'a\n<<<<<<< x\nc\n=======\n>>>>>>> y'],
  ])('FR-CONF-05 undoing an empty take restores the region exactly, %s', (_, doc) => {
    const state = blockState(doc, 4)
    const taken = state.update({
      ...takeRegion(state, 0, 'incoming'),
      userEvent: 'input',
    }).state
    expect(isUnresolved(taken, 0)).toBe(false)

    expect(regionLineRanges(undone(taken))).toEqual([{ start: 2, lines: 4 }])
  })

  it('FR-CONF-05 a region whose marker lines were joined onto a previous line is resolved', () => {
    const doc = 'a\n<<<<<<< x\nc\n=======\ni\n>>>>>>> y\nb'
    const state = blockState(doc, 5)
    const edited = state.update({
      changes: [
        { from: 1, to: 2 },
        { from: doc.indexOf('=======') - 1, to: doc.indexOf('=======') + 7 },
        { from: doc.indexOf('>>>>>>>') - 1, to: doc.indexOf('>>>>>>>') + 9 },
      ],
    }).state

    expect(edited.doc.toString()).toBe('a<<<<<<< x\nc\ni\nb')
    expect(isUnresolved(edited, 0)).toBe(false)
    expect(unresolvedCount(edited)).toBe(0)
  })

  it('FR-CONF-05 a region without a complete block cannot be taken from', () => {
    const state = resultState()
    expect(canTakeRegion(state, 0)).toBe(true)
    const from = lineOf(state, 'c1').from
    const to = lineOf(state, 'c2').to
    const broken = state.update({ changes: { from, to } }).state

    expect([isUnresolved(broken, 0), isUnresolved(broken, 1)]).toEqual([true, true])
    expect([canTakeRegion(broken, 0), canTakeRegion(broken, 1)]).toEqual([false, false])
  })

  it('FR-CONF-05 selecting a region the result does not have is ignored', () => {
    const state = resultState().update({ effects: selectRegion.of(1) }).state
    const next = state.update({ effects: selectRegion.of(7) }).state

    expect(selectedRegion(next)).toBe(1)
    expect(unresolvedCount(next)).toBe(2)
  })
})
