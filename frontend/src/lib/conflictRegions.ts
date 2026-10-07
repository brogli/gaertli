// Conflict regions in the merge editor's CodeMirror panes. The result pane
// tracks each region's range through edits; a region is resolved once a take
// or "mark done" says so, never by an edit alone. The read-only side panes
// mark their regions' lines and the lines the side changed.

import {
  type ChangeSet,
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
import { invertedEffects, isolateHistory } from '@codemirror/commands'
import { Decoration, type DecorationSet, EditorView, WidgetType } from '@codemirror/view'
import { DEFAULT_MARKER_SIZE, isMarkerLine } from './conflictMarkers'

export interface Region {
  from: number
  to: number
}

export interface LineRange {
  start: number
  lines: number
}

// SideChange is a run of a side's lines that differ from the base.
export interface SideChange {
  lines: LineRange
  added: boolean
}

// regionFromLineRange converts a 1-based line range to offsets from the first
// line's start to the last line's end. A range without lines becomes the
// point before the line it precedes.
export function regionFromLineRange(doc: Text, { start, lines }: LineRange): Region {
  const lineAt = (n: number) => doc.line(Math.min(Math.max(n, 1), doc.lines))
  if (lines <= 0) {
    const point = start >= doc.lines ? doc.length : lineAt(start + 1).from
    return { from: point, to: point }
  }
  return { from: lineAt(start).from, to: lineAt(start + lines - 1).to }
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

function lineDecoration(doc: Text, lineNumber: number, className: string): Range<Decoration> {
  return Decoration.line({ class: className }).range(doc.line(lineNumber).from)
}

function regionClass(resolved: boolean | undefined, selected: boolean): string {
  const state = resolved ? 'cm-conflict-resolved' : 'cm-conflict-unresolved'
  return selected ? `${state} cm-conflict-selected` : state
}

// ---- result pane ----

export interface ResultRegion {
  range: LineRange
  resolved: boolean
}

interface TrackedRegion extends Region {
  resolved: boolean
}

interface ResultValue {
  regions: TrackedRegion[]
  selected: number
  decorations: DecorationSet
}

const markerSizeFacet = Facet.define<number, number>({
  combine: (sizes) => sizes[0] ?? DEFAULT_MARKER_SIZE,
})

export const selectRegion = StateEffect.define<number>()

const setResolved = StateEffect.define<{ region: number; resolved: boolean }>()

// mapRegions maps the regions through changes. Regions stay on whole lines:
// an empty one is a point before a line and grows only by inserted whole
// lines.
function mapRegions<R extends Region>(regions: R[], changes: ChangeSet, oldDoc: Text): R[] {
  return regions.map((r) => ({
    ...r,
    ...(r.from === r.to
      ? mapPoint(r.from, changes, oldDoc)
      : { from: changes.mapPos(r.from, -1), to: changes.mapPos(r.to, 1) }),
  }))
}

// mapPoint maps an empty region at pos. Text inserted at the point whole lines
// at a time (ending in a line break at a line start, or starting with one at
// the end of the text) becomes the region; any other stays outside it.
function mapPoint(pos: number, changes: ChangeSet, oldDoc: Text): Region {
  const atLineStart = pos === oldDoc.lineAt(pos).from
  const atEnd = pos === oldDoc.length && !atLineStart
  let grown: Region | undefined
  changes.iterChanges((fromA, toA, fromB, toB, inserted) => {
    if (fromA !== pos || toA !== pos) return
    const text = inserted.toString()
    if (atLineStart && text.endsWith('\n')) grown = { from: fromB, to: toB - 1 }
    else if (atEnd && text.startsWith('\n')) grown = { from: fromB + 1, to: toB }
  })
  const point = changes.mapPos(pos, atEnd ? 1 : -1)
  return grown ?? { from: point, to: point }
}

const placeholderFacet = Facet.define<string, string>({
  combine: (labels) => labels[0] ?? 'Unresolved conflict',
})

// Placeholder is the bar that shows an unresolved region with no lines;
// pressing it selects the region.
class Placeholder extends WidgetType {
  constructor(
    readonly label: string,
    readonly selected: boolean,
    readonly select: (view: EditorView) => void,
  ) {
    super()
  }

  eq(other: Placeholder): boolean {
    return other.label === this.label && other.selected === this.selected
  }

  toDOM(view: EditorView): HTMLElement {
    const bar = document.createElement('div')
    bar.className = 'cm-conflict-placeholder cm-conflict-unresolved'
    if (this.selected) bar.classList.add('cm-conflict-selected')
    bar.textContent = this.label
    bar.addEventListener('mousedown', () => this.select(view))
    return bar
  }
}

// placeholders are the bars of the unresolved regions with no lines; pressing
// region i's dispatches pick.of(i).
function placeholders(
  state: EditorState,
  regions: Region[],
  isResolved: (i: number) => boolean | undefined,
  selected: number,
  pick: StateEffectType<number>,
): Range<Decoration>[] {
  const label = state.facet(placeholderFacet)
  return regions.flatMap((region, i) => {
    if (region.from !== region.to || isResolved(i)) return []
    const select = (view: EditorView) => view.dispatch({ effects: pick.of(i) })
    const widget = new Placeholder(label, i === selected, select)
    return [Decoration.widget({ widget, block: true, side: -1 }).range(region.from)]
  })
}

function resultDecorations(
  state: EditorState,
  regions: TrackedRegion[],
  selected: number,
): DecorationSet {
  const lines = regions.flatMap((region, i) =>
    linesOf(state.doc, region).map((n) =>
      lineDecoration(state.doc, n, regionClass(region.resolved, i === selected)),
    ),
  )
  const empty = placeholders(state, regions, (i) => regions[i]?.resolved, selected, selectRegion)
  return Decoration.set([...lines, ...empty], true)
}

const resultField = StateField.define<ResultValue>({
  create: () => ({ regions: [], selected: -1, decorations: Decoration.none }),
  update(value, tr) {
    const moved = tr.docChanged
      ? mapRegions(value.regions, tr.changes, tr.startState.doc)
      : value.regions
    const requested = lastEffectValue(tr.effects, selectRegion)
    const chosen = requested !== undefined && requested < moved.length ? requested : undefined
    // A cursor inside a region selects it; outside, the selection stays.
    const entered = tr.selection ? regionIndexAt(moved, tr.newSelection.main.head) : -1
    const selected = chosen ?? (entered >= 0 ? entered : value.selected)
    const marks = tr.effects.filter((effect) => effect.is(setResolved))
    if (!tr.docChanged && selected === value.selected && marks.length === 0) return value
    const regions = [...moved]
    for (const { value: mark } of marks) {
      const region = regions[mark.region]
      if (region) regions[mark.region] = { ...region, resolved: mark.resolved }
    }
    return { regions, selected, decorations: resultDecorations(tr.state, regions, selected) }
  },
  provide: (field) => EditorView.decorations.from(field, (value) => value.decorations),
})

// Undoing a take or a mark restores the region's previous state.
const undoResolved = invertedEffects.of((tr) =>
  tr.effects
    .filter((effect) => effect.is(setResolved))
    .map((effect) =>
      setResolved.of({
        region: effect.value.region,
        resolved: tr.startState.field(resultField).regions[effect.value.region]?.resolved ?? false,
      }),
    ),
)

// regionsExtension tracks the result's regions and styles their lines.
// markerSize is the size of git's markers, for hasStrayMarkers.
export function regionsExtension(
  initial: ResultRegion[],
  markerSize: number,
  placeholderLabel = 'Unresolved conflict',
): Extension {
  return [
    markerSizeFacet.of(markerSize),
    placeholderFacet.of(placeholderLabel),
    undoResolved,
    resultField.init((state) => {
      const regions = initial.map(({ range, resolved }) => ({
        ...regionFromLineRange(state.doc, range),
        resolved,
      }))
      return { regions, selected: -1, decorations: resultDecorations(state, regions, -1) }
    }),
  ]
}

export function regionCount(state: EditorState): number {
  return state.field(resultField).regions.length
}

export function selectedRegion(state: EditorState): number {
  return state.field(resultField).selected
}

function regionOf(state: EditorState, i: number): TrackedRegion {
  const region = state.field(resultField).regions[i]
  if (!region) throw new RangeError(`no conflict region ${i}`)
  return region
}

export function isUnresolved(state: EditorState, i: number): boolean {
  return !regionOf(state, i).resolved
}

export function unresolvedCount(state: EditorState): number {
  return state.field(resultField).regions.filter((region) => !region.resolved).length
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

// resultRegions are the regions as line ranges, the inverse of
// regionFromLineRange, with their resolved flags.
export function resultRegions(state: EditorState): ResultRegion[] {
  const { doc } = state
  return state.field(resultField).regions.map(({ from, to, resolved }) => {
    const first = doc.lineAt(from)
    const range =
      from >= to
        ? { start: from === first.from ? first.number - 1 : first.number, lines: 0 }
        : { start: first.number, lines: doc.lineAt(to).number - first.number + 1 }
    return { range, resolved }
  })
}

// nextUnresolved returns the nearest unresolved region after (dir 1) or
// before (dir -1) region from, or -1.
export function nextUnresolved(state: EditorState, from: number, dir: 1 | -1): number {
  const { regions } = state.field(resultField)
  for (let i = from + dir; i >= 0 && i < regions.length; i += dir) {
    if (!regions[i]?.resolved) return i
  }
  return -1
}

// markDone marks region i resolved as it stands and selects it, as one undo
// step.
export function markDone(i: number): TransactionSpec {
  return {
    effects: [selectRegion.of(i), setResolved.of({ region: i, resolved: true })],
    annotations: isolateHistory.of('full'),
  }
}

// takeRegion replaces region i by lines, marks it resolved and selects it, as
// one undo step.
export function takeRegion(state: EditorState, i: number, lines: string[]): TransactionSpec {
  return { changes: replacement(state, regionOf(state, i), lines), ...markDone(i) }
}

// replacement is the change that puts lines in place of region's lines. An
// emptied region takes a line break along.
function replacement(state: EditorState, region: Region, lines: string[]) {
  const { doc } = state
  const text = lines.join('\n')
  if (region.from < region.to) {
    if (lines.length > 0) return { from: region.from, to: region.to, insert: text }
    if (region.to < doc.length) return { from: region.from, to: region.to + 1 }
    return { from: Math.max(region.from - 1, 0), to: region.to }
  }
  if (lines.length === 0) return []
  const atLineStart = region.from === doc.lineAt(region.from).from
  return { from: region.from, insert: atLineStart ? `${text}\n` : `\n${text}` }
}

// ---- side panes ----

export interface SideRegionStates {
  resolved: boolean[]
  selected: number
}

interface SideConfig {
  ranges: LineRange[]
  changes: SideChange[]
}

interface SideValue extends SideRegionStates {
  regions: Region[]
  decorations: DecorationSet
}

const sideConfigFacet = Facet.define<SideConfig, SideConfig>({
  combine: (configs) => configs[0] ?? { ranges: [], changes: [] },
})

// pickSideRegion says the user pressed region i's placeholder in a side pane.
export const pickSideRegion = StateEffect.define<number>()

export const setSideRegionStates = StateEffect.define<SideRegionStates>()

// regionStates are the result's regions' states, as the side panes show them.
export function regionStates(state: EditorState): SideRegionStates {
  const { regions, selected } = state.field(resultField)
  return { resolved: regions.map((region) => region.resolved), selected }
}

// changeLineClasses is the class of each line the side changed, by line number.
function changeLineClasses(doc: Text, changes: SideChange[]): Map<number, string> {
  const classes = new Map<number, string>()
  for (const { lines, added } of changes) {
    for (let n = lines.start; n < lines.start + lines.lines && n <= doc.lines; n++) {
      classes.set(n, added ? 'cm-change-added' : 'cm-change-changed')
    }
  }
  return classes
}

// A region's lines win over the change they lie in.
function sideDecorations(state: EditorState, value: Omit<SideValue, 'decorations'>): DecorationSet {
  const classes = changeLineClasses(state.doc, state.facet(sideConfigFacet).changes)
  value.regions.forEach((region, i) => {
    for (const n of linesOf(state.doc, region)) {
      classes.set(n, regionClass(value.resolved[i], i === value.selected))
    }
  })
  const lines = [...classes].map(([n, name]) => lineDecoration(state.doc, n, name))
  const empty = placeholders(
    state,
    value.regions,
    (i) => value.resolved[i],
    value.selected,
    pickSideRegion,
  )
  return Decoration.set([...lines, ...empty], true)
}

const sideField = StateField.define<SideValue>({
  create(state) {
    const { ranges } = state.facet(sideConfigFacet)
    const regions = ranges.map((range) => regionFromLineRange(state.doc, range))
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

// sideRegionsExtension marks the lines of a read-only pane's regions and the
// lines its side changed; the regions' states are set with
// setSideRegionStates.
export function sideRegionsExtension(
  ranges: LineRange[],
  changes: SideChange[],
  placeholderLabel = 'Unresolved conflict',
): Extension {
  return [sideConfigFacet.of({ ranges, changes }), placeholderFacet.of(placeholderLabel), sideField]
}

// sideRegionStart is the offset where side region i begins, if there is one.
export function sideRegionStart(state: EditorState, i: number): number | undefined {
  return state.field(sideField).regions[i]?.from
}

// sideRegionAt returns the index of the side region containing pos, or -1.
export function sideRegionAt(state: EditorState, pos: number): number {
  return regionIndexAt(state.field(sideField).regions, pos)
}
