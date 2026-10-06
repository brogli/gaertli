// Conflict regions in the merge editor's CodeMirror panes. The result pane
// tracks each region's range through edits and reads a region's status from
// its text: unresolved while it still holds a marker line. The read-only side
// panes only mark their regions' lines.

import {
  type ChangeDesc,
  EditorState,
  type Extension,
  Facet,
  StateEffect,
  type Range,
  type StateEffectType,
  StateField,
  type Text,
  type TransactionSpec,
} from '@codemirror/state'
import { Decoration, type DecorationSet, EditorView } from '@codemirror/view'
import type { SidePane } from './conflictPanes'
import {
  DEFAULT_MARKER_SIZE,
  findBlocks,
  hasMarkerLines,
  isMarkerLine,
  takeLines,
  type MarkerBlock,
  type Take,
} from './conflictMarkers'

export interface Region {
  from: number
  to: number
}

export interface LineRange {
  start: number
  lines: number
}

// regionsFromLineRanges converts 1-based line ranges to offsets from the
// first line's start to the last line's end. A range without lines becomes
// the point before the line it precedes.
export function regionsFromLineRanges(doc: Text, ranges: LineRange[]): Region[] {
  const lineAt = (n: number) => doc.line(Math.min(Math.max(n, 1), doc.lines))
  return ranges.map(({ start, lines }) => {
    if (lines <= 0) {
      const point = start >= doc.lines ? doc.length : lineAt(start + 1).from
      return { from: point, to: point }
    }
    return { from: lineAt(start).from, to: lineAt(start + lines - 1).to }
  })
}

// lastEffectValue is the value of the last effect of the given type, if any.
function lastEffectValue<T>(
  effects: readonly StateEffect<unknown>[],
  type: StateEffectType<T>,
): T | undefined {
  return effects.filter((effect) => effect.is(type)).at(-1)?.value
}

function regionIndexAt(regions: Region[], pos: number): number {
  return regions.findIndex((r) => r.from < r.to && r.from <= pos && pos <= r.to)
}

function linesOf(doc: Text, region: Region): number[] {
  if (region.from >= region.to) return []
  const first = doc.lineAt(region.from).number
  const last = doc.lineAt(region.to).number
  return Array.from({ length: last - first + 1 }, (_, i) => first + i)
}

function lineDecoration(doc: Text, lineNumber: number, classes: string[]): Range<Decoration> {
  return Decoration.line({ class: classes.join(' ') }).range(doc.line(lineNumber).from)
}

// ---- result pane ----

interface ResultValue {
  regions: Region[]
  // Whether each region still holds a marker line.
  unresolved: boolean[]
  selected: number
  decorations: DecorationSet
}

const markerSizeFacet = Facet.define<number, number>({
  combine: (sizes) => sizes[0] ?? DEFAULT_MARKER_SIZE,
})

export const selectRegion = StateEffect.define<number>()

// mapRegions maps the regions through changes. A region that was empty
// shrinks back to empty when text inserted at its point gets mapped into it.
function mapRegions(regions: Region[], changes: ChangeDesc, doc: Text): Region[] {
  return regions.map((r) => {
    let from = changes.mapPos(r.from, -1)
    let to = changes.mapPos(r.to, 1)
    if (r.from === r.to && from < to) {
      if (doc.sliceString(to - 1, to) === '\n') to -= 1
      if (from < to && doc.sliceString(from, from + 1) === '\n') from += 1
    }
    return { from, to }
  })
}

// regionText is the text of the lines the region touches; empty for an empty
// region.
function regionText(state: EditorState, region: Region): string {
  if (region.from >= region.to) return ''
  return state.sliceDoc(state.doc.lineAt(region.from).from, state.doc.lineAt(region.to).to)
}

function blockLineKinds(block: MarkerBlock): string[] {
  return [
    'cm-conflict-marker',
    ...block.current.map(() => 'cm-conflict-current'),
    ...(block.base ? ['cm-conflict-marker', ...block.base.map(() => 'cm-conflict-base')] : []),
    'cm-conflict-marker',
    ...block.incoming.map(() => 'cm-conflict-incoming'),
    'cm-conflict-marker',
  ]
}

// regionLineKinds returns the line class of each of the region's lines for
// an unresolved region.
function regionLineKinds(
  state: EditorState,
  region: Region,
  lines: number[],
): (string | undefined)[] {
  const size = state.facet(markerSizeFacet)
  const kinds: (string | undefined)[] = lines.map((n) =>
    isMarkerLine(state.doc.line(n).text, size) ? 'cm-conflict-marker' : undefined,
  )
  for (const block of findBlocks(regionText(state, region), size)) {
    blockLineKinds(block).forEach((kind, i) => {
      kinds[block.startLine - 1 + i] = kind
    })
  }
  return kinds
}

function resultDecorations(
  state: EditorState,
  regions: Region[],
  unresolved: boolean[],
  selected: number,
): DecorationSet {
  const ranges = regions.flatMap((region, i) => {
    const lines = linesOf(state.doc, region)
    const kinds = unresolved[i]
      ? regionLineKinds(state, region, lines)
      : lines.map(() => 'cm-conflict-resolved')
    return lines.flatMap((n, j) => {
      const classes = [kinds[j], i === selected ? 'cm-conflict-selected' : undefined].filter(
        (c) => c !== undefined,
      )
      return classes.length > 0 ? [lineDecoration(state.doc, n, classes)] : []
    })
  })
  return Decoration.set(ranges, true)
}

function unresolvedRegions(state: EditorState, regions: Region[]): boolean[] {
  const size = state.facet(markerSizeFacet)
  return regions.map((region) => hasMarkerLines(regionText(state, region), size))
}

const resultField = StateField.define<ResultValue>({
  create: () => ({ regions: [], unresolved: [], selected: -1, decorations: Decoration.none }),
  update(value, tr) {
    const regions = tr.docChanged ? mapRegions(value.regions, tr.changes, tr.newDoc) : value.regions
    const requested = lastEffectValue(tr.effects, selectRegion)
    const chosen = requested !== undefined && requested < regions.length ? requested : undefined
    // A cursor inside a region selects it; outside, the selection stays.
    const entered = tr.selection ? regionIndexAt(regions, tr.newSelection.main.head) : -1
    const selected = chosen ?? (entered >= 0 ? entered : value.selected)
    if (!tr.docChanged && selected === value.selected) return value
    const unresolved = tr.docChanged ? unresolvedRegions(tr.state, regions) : value.unresolved
    return {
      regions,
      unresolved,
      selected,
      decorations: resultDecorations(tr.state, regions, unresolved, selected),
    }
  },
  provide: (field) => EditorView.decorations.from(field, (value) => value.decorations),
})

// regionsExtension tracks the result's regions, given as line ranges, and
// styles their lines. markerSize is the size of git's markers in the text.
export function regionsExtension(initial: LineRange[], markerSize: number): Extension {
  return [
    markerSizeFacet.of(markerSize),
    resultField.init((state) => {
      const regions = regionsFromLineRanges(state.doc, initial)
      const unresolved = unresolvedRegions(state, regions)
      return {
        regions,
        unresolved,
        selected: -1,
        decorations: resultDecorations(state, regions, unresolved, -1),
      }
    }),
  ]
}

export function regionCount(state: EditorState): number {
  return state.field(resultField).regions.length
}

export function selectedRegion(state: EditorState): number {
  return state.field(resultField).selected
}

function regionOf(state: EditorState, i: number): Region {
  const region = state.field(resultField).regions[i]
  if (!region) throw new RangeError(`no conflict region ${i}`)
  return region
}

export function isUnresolved(state: EditorState, i: number): boolean {
  const unresolved = state.field(resultField).unresolved[i]
  if (unresolved === undefined) throw new RangeError(`no conflict region ${i}`)
  return unresolved
}

export function unresolvedCount(state: EditorState): number {
  return state.field(resultField).unresolved.filter(Boolean).length
}

// hasStrayMarkers reports whether any line of the document is a marker line.
export function hasStrayMarkers(state: EditorState): boolean {
  const size = state.facet(markerSizeFacet)
  for (const line of state.doc.iterLines()) {
    if (isMarkerLine(line, size)) return true
  }
  return false
}

// regionStart is the offset where region i begins.
export function regionStart(state: EditorState, i: number): number {
  return regionOf(state, i).from
}

// regionLineRanges are the regions' 1-based line ranges in the doc, the
// inverse of regionsFromLineRanges.
export function regionLineRanges(state: EditorState): LineRange[] {
  const { doc } = state
  return state.field(resultField).regions.map(({ from, to }) => {
    const first = doc.lineAt(from)
    if (from >= to)
      return { start: from === first.from ? first.number - 1 : first.number, lines: 0 }
    return { start: first.number, lines: doc.lineAt(to).number - first.number + 1 }
  })
}

// nextUnresolved returns the nearest unresolved region after (dir 1) or
// before (dir -1) region from, or -1.
export function nextUnresolved(state: EditorState, from: number, dir: 1 | -1): number {
  const { unresolved } = state.field(resultField)
  for (let i = from + dir; i >= 0 && i < unresolved.length; i += dir) {
    if (unresolved[i]) return i
  }
  return -1
}

// canTakeRegion reports whether region i holds a complete marker block.
export function canTakeRegion(state: EditorState, i: number): boolean {
  if (!isUnresolved(state, i)) return false
  const text = regionText(state, regionOf(state, i))
  return findBlocks(text, state.facet(markerSizeFacet)).length > 0
}

// takeRegion replaces the marker block in region i by the taken lines and
// selects the region. It does nothing when the region holds no complete block.
export function takeRegion(state: EditorState, i: number, take: Take): TransactionSpec {
  const region = regionOf(state, i)
  const text = regionText(state, region)
  const block = findBlocks(text, state.facet(markerSizeFacet))[0]
  if (!block) return {}

  const firstLine = state.doc.lineAt(region.from).number
  let from = state.doc.line(firstLine + block.startLine - 1).from
  let to = state.doc.line(firstLine + block.endLine - 1).to
  const taken = takeLines(block, take)
  if (taken.length === 0) {
    // Remove the emptied lines with one line break.
    if (to < state.doc.length) to += 1
    else if (from > 0) from -= 1
  }
  return {
    changes: { from, to, insert: taken.join('\n') },
    effects: selectRegion.of(i),
  }
}

// ---- side panes ----

export interface SideRegionStates {
  resolved: boolean[]
  selected: number
}

interface SideConfig {
  ranges: LineRange[]
  tone: SidePane
}

interface SideValue extends SideRegionStates {
  regions: Region[]
  decorations: DecorationSet
}

const sideConfigFacet = Facet.define<SideConfig, SideConfig>({
  combine: (configs) => configs[0] ?? { ranges: [], tone: 'current' },
})

export const setSideRegionStates = StateEffect.define<SideRegionStates>()

// regionStates are the result's regions' states, as the side panes show them.
export function regionStates(state: EditorState): SideRegionStates {
  const { unresolved, selected } = state.field(resultField)
  return { resolved: unresolved.map((open) => !open), selected }
}

function sideDecorations(state: EditorState, value: Omit<SideValue, 'decorations'>): DecorationSet {
  const { tone } = state.facet(sideConfigFacet)
  const ranges = value.regions.flatMap((region, i) =>
    linesOf(state.doc, region).map((n) =>
      lineDecoration(
        state.doc,
        n,
        [
          value.resolved[i] ? 'cm-conflict-resolved' : `cm-conflict-${tone}`,
          i === value.selected ? 'cm-conflict-selected' : undefined,
        ].filter((c) => c !== undefined),
      ),
    ),
  )
  return Decoration.set(ranges, true)
}

const sideField = StateField.define<SideValue>({
  create(state) {
    const regions = regionsFromLineRanges(state.doc, state.facet(sideConfigFacet).ranges)
    const value = { regions, resolved: regions.map(() => false), selected: -1 }
    return { ...value, decorations: sideDecorations(state, value) }
  },
  update(value, tr) {
    const states = lastEffectValue(tr.effects, setSideRegionStates)
    if (!states) return value
    const next = { regions: value.regions, ...states }
    return { ...next, decorations: sideDecorations(tr.state, next) }
  },
  provide: (field) => EditorView.decorations.from(field, (value) => value.decorations),
})

// sideRegionsExtension marks the lines of a read-only pane's regions; the
// regions' states are set with setSideRegionStates.
export function sideRegionsExtension(ranges: LineRange[], tone: SidePane): Extension {
  return [sideConfigFacet.of({ ranges, tone }), sideField]
}

// sideRegionStart is the offset where side region i begins, if there is one.
export function sideRegionStart(state: EditorState, i: number): number | undefined {
  return state.field(sideField).regions[i]?.from
}

// sideRegionAt returns the index of the side region containing pos, or -1.
export function sideRegionAt(state: EditorState, pos: number): number {
  return regionIndexAt(state.field(sideField).regions, pos)
}
