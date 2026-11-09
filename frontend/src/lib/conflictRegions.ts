// The merge editor's changes in its CodeMirror panes: the conflicts and the
// changes only one side made. A region is the editor's span of document
// offsets over one change; the core's types describe the change itself. The result pane tracks each region's range
// through edits; a region is resolved once a take or "mark done" says so,
// never by an edit alone. The read-only side panes mark their regions' lines
// and the lines the side changed.

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
import {
  Decoration,
  type DecorationSet,
  EditorView,
  gutter,
  GutterMarker,
  WidgetType,
} from '@codemirror/view'
import {
  type DraftChange,
  type LineRange,
  MergeChangeKind,
  type MergeChange,
  type SideChange,
} from '../../bindings/github.com/brogli/gaertli/internal/core'
import { DEFAULT_MARKER_SIZE, isMarkerLine } from './conflictMarkers'
import type { Pane } from './conflictPanes'

export interface Region {
  from: number
  to: number
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

// RegionTint is the line class of an unresolved region: red for a conflict,
// blue for a change, green for added lines.
export type RegionTint = 'cm-conflict-unresolved' | 'cm-change-changed' | 'cm-change-added'

const CONFLICT_TINT: RegionTint = 'cm-conflict-unresolved'
const HANDLED = 'cm-conflict-resolved'

// regionTint is the tint of a region in a pane. A side pane shows a change
// only one side made on the side that made it; its other side has no tint.
export function regionTint(
  { kind, added }: Pick<MergeChange, 'kind' | 'added'>,
  pane: Pane,
): RegionTint | undefined {
  if (kind === MergeChangeKind.ConflictChange) return CONFLICT_TINT
  const changedHere =
    pane === 'result' ||
    (kind === MergeChangeKind.CurrentOnlyChange && pane === 'current') ||
    (kind === MergeChangeKind.IncomingOnlyChange && pane === 'incoming')
  if (!changedHere) return undefined
  return added ? 'cm-change-added' : 'cm-change-changed'
}

const tintsFacet = Facet.define<(RegionTint | undefined)[], (RegionTint | undefined)[]>({
  combine: (tints) => tints[0] ?? [],
})

// tintOf is region i's tint; a region without one given is a conflict.
function tintOf(state: EditorState, i: number): RegionTint | undefined {
  const tints = state.facet(tintsFacet)
  return i < tints.length ? tints[i] : CONFLICT_TINT
}

interface RegionLook {
  resolved?: boolean | undefined
  selected: boolean
  tint?: RegionTint | undefined
}

// lineClass is the class of line n of a region lying on lines: its tint or
// resolved mark, and the outline of a selected region.
function lineClass(look: RegionLook, n: number, lines: number[]): string {
  const classes = [look.resolved ? 'cm-conflict-resolved' : look.tint]
  if (look.selected) {
    classes.push('cm-chunk-selected')
    if (n === lines[0]) classes.push('cm-chunk-first')
    if (n === lines.at(-1)) classes.push('cm-chunk-last')
  }
  return classes.filter(Boolean).join(' ')
}

// regionLineDecorations marks each line of region with its class.
function regionLineDecorations(doc: Text, region: Region, look: RegionLook): Range<Decoration>[] {
  const lines = linesOf(doc, region)
  const classes = lines.map((n) => lineClass(look, n, lines))
  return lines.flatMap((n, i) => (classes[i] ? [lineDecoration(doc, n, classes[i])] : []))
}

// ---- result pane ----

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
  combine: (labels) => labels[0] ?? 'Unresolved change',
})

// Placeholder is the bar that shows an unresolved region with no lines;
// pressing it selects the region.
class Placeholder extends WidgetType {
  constructor(
    readonly label: string,
    readonly className: string,
    readonly select: (view: EditorView) => void,
  ) {
    super()
  }

  eq(other: Placeholder): boolean {
    return other.label === this.label && other.className === this.className
  }

  toDOM(view: EditorView): HTMLElement {
    const bar = document.createElement('div')
    bar.className = this.className
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
    const className = `cm-conflict-placeholder ${lineClass({ selected: i === selected, tint: tintOf(state, i) }, 0, [0])}`
    const widget = new Placeholder(label, className, select)
    // A point after the last line puts the bar after it, not before an empty line.
    const afterLastLine =
      region.from === state.doc.length &&
      state.doc.length > 0 &&
      region.from !== state.doc.lineAt(region.from).from
    return [
      Decoration.widget({ widget, block: true, side: afterLastLine ? 1 : -1 }).range(region.from),
    ]
  })
}

function resultDecorations(
  state: EditorState,
  regions: TrackedRegion[],
  selected: number,
): DecorationSet {
  const lines = regions.flatMap((region, i) =>
    regionLineDecorations(state.doc, region, {
      resolved: region.resolved,
      selected: i === selected,
      tint: tintOf(state, i),
    }),
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
  initial: DraftChange[],
  markerSize: number,
  placeholderLabel = 'Unresolved change',
  tints?: (RegionTint | undefined)[],
): Extension {
  return [
    markerSizeFacet.of(markerSize),
    placeholderFacet.of(placeholderLabel),
    tintsFacet.of(tints ?? initial.map(() => CONFLICT_TINT)),
    selectionGutter(resultField, (value) => value.regions[value.selected]),
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
  if (!region) throw new RangeError(`no change ${i}`)
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
export function resultRegions(state: EditorState): DraftChange[] {
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

// takeRegions replaces each listed region by its lines and marks it
// resolved, as one undo step; the selection stays.
export function takeRegions(
  state: EditorState,
  takes: { region: number; lines: string[] }[],
): TransactionSpec {
  return {
    changes: takes.map(({ region, lines }) => replacement(state, regionOf(state, region), lines)),
    effects: takes.map(({ region }) => setResolved.of({ region, resolved: true })),
    annotations: isolateHistory.of('full'),
  }
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

// ---- selection marker ----

const selectionMarker = new (class extends GutterMarker {
  elementClass = 'cm-chunk-gutter-mark'
})()

// selectionGutter marks the lines of the selected region in a gutter beside
// the pane's lines; selectedOf finds that region in the pane's field.
function selectionGutter<V>(
  field: StateField<V>,
  selectedOf: (value: V) => Region | undefined,
): Extension {
  return gutter({
    class: 'cm-chunk-gutter',
    lineMarker: (view, line) => {
      const region = selectedOf(view.state.field(field))
      const inside =
        region && region.from < region.to && region.from <= line.from && line.from <= region.to
      return inside ? selectionMarker : null
    },
    lineMarkerChange: (update) => update.startState.field(field) !== update.state.field(field),
  })
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

// changeLineClasses marks each line the side changed. Those no region takes over are changes
// both sides made alike, which are handled from the start.
function changeLineClasses(doc: Text, changes: SideChange[]): Map<number, string> {
  const classes = new Map<number, string>()
  for (const { lines } of changes) {
    for (let n = lines.start; n < lines.start + lines.lines && n <= doc.lines; n++) {
      classes.set(n, HANDLED)
    }
  }
  return classes
}

// A region's lines win over the change they lie in.
function sideDecorations(state: EditorState, value: Omit<SideValue, 'decorations'>): DecorationSet {
  const classes = changeLineClasses(state.doc, state.facet(sideConfigFacet).changes)
  value.regions.forEach((region, i) => {
    const look = {
      resolved: value.resolved[i],
      selected: i === value.selected,
      tint: tintOf(state, i),
    }
    const lines = linesOf(state.doc, region)
    for (const n of lines) classes.set(n, lineClass(look, n, lines))
  })
  const lines = [...classes]
    .filter(([, name]) => name)
    .map(([n, name]) => lineDecoration(state.doc, n, name))
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
  placeholderLabel = 'Unresolved change',
  tints?: (RegionTint | undefined)[],
): Extension {
  return [
    sideConfigFacet.of({ ranges, changes }),
    placeholderFacet.of(placeholderLabel),
    tintsFacet.of(tints ?? ranges.map(() => CONFLICT_TINT)),
    selectionGutter(sideField, (value) => value.regions[value.selected]),
    sideField,
  ]
}

// sideRegionStart is the offset where side region i begins, if there is one.
export function sideRegionStart(state: EditorState, i: number): number | undefined {
  return state.field(sideField).regions[i]?.from
}

// sideRegionAt returns the index of the side region containing pos, or -1.
export function sideRegionAt(state: EditorState, pos: number): number {
  return regionIndexAt(state.field(sideField).regions, pos)
}
