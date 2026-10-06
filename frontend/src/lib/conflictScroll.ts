// Line mapping and scroll tying between the merge editor's panes. An anchor is a set of
// lines, one per pane, that sit at the same height; between anchors lines map linearly.
import type { EditorView } from '@codemirror/view'
import { PANES, type Pane } from './conflictPanes'

// Lines are 1-based and may be fractional.
export type Anchor = Partial<Record<Pane, number>>

// A region's lines in a pane: Start is its first line; with Lines 0, the line
// after which the region would be.
export type PaneRegion = { start: number; lines: number }
export type RegionRanges = Partial<Record<Pane, PaneRegion>>

// regionAnchors builds the anchors of regions listed in file order: every
// file's start, each region's first line and the line after its last, and
// every file's end. An anchor that goes backwards in some pane is dropped.
export function regionAnchors(
  regions: RegionRanges[],
  lineCounts: Partial<Record<Pane, number>>,
): Anchor[] {
  const fileStart: Anchor = {}
  const fileEnd: Anchor = {}
  for (const pane of PANES) {
    const count = lineCounts[pane]
    if (count === undefined) continue
    fileStart[pane] = 1
    fileEnd[pane] = count + 1
  }
  const candidates: Anchor[] = [fileStart]
  for (const region of regions) {
    const first: Anchor = {}
    const after: Anchor = {}
    for (const pane of PANES) {
      const range = region[pane]
      if (!range) continue
      first[pane] = range.lines > 0 ? range.start : range.start + 1
      after[pane] = range.lines > 0 ? range.start + range.lines : range.start + 1
    }
    candidates.push(first, after)
  }
  candidates.push(fileEnd)

  const last: Anchor = {}
  const anchors: Anchor[] = []
  for (const anchor of candidates) {
    const goesBackwards = PANES.some(
      (pane) => (anchor[pane] ?? Infinity) < (last[pane] ?? -Infinity),
    )
    if (goesBackwards) continue
    anchors.push(anchor)
    Object.assign(last, anchor)
  }
  return anchors
}

// LinePair is the lines of two panes that sit at the same height.
export interface LinePair {
  from: number
  to: number
}

// linePairs lists the anchors' lines in the panes from and to, for mapLine.
export function linePairs(anchors: Anchor[], from: Pane, to: Pane): LinePair[] {
  return anchors.flatMap((a) => {
    const [fromLine, toLine] = [a[from], a[to]]
    return fromLine !== undefined && toLine !== undefined ? [{ from: fromLine, to: toLine }] : []
  })
}

// mapLine maps a line of one pane to the line of another at the same height,
// given their linePairs: exact at anchors, linear between them, by offset
// beyond the first and last.
export function mapLine(pairs: LinePair[], line: number): number {
  const upperIndex = pairs.findIndex((p) => p.from > line)
  const lower = pairs[upperIndex < 0 ? pairs.length - 1 : upperIndex - 1]
  const upper = pairs[upperIndex]
  if (!lower) return upper ? upper.to + line - upper.from : line
  if (!upper) return lower.to + line - lower.from
  return lower.to + ((line - lower.from) / (upper.from - lower.from)) * (upper.to - lower.to)
}

// topLine is the line at the top of view's viewport, with the fraction of
// it scrolled past.
function topLine(view: EditorView): number {
  const block = view.lineBlockAtHeight(view.scrollDOM.scrollTop)
  const fraction = (view.scrollDOM.scrollTop - block.top) / block.height
  return view.state.doc.lineAt(block.from).number + fraction
}

// scrollToLine scrolls view so that line, which may be fractional, is at
// its top. A line past the end scrolls to the document's end.
function scrollToLine(view: EditorView, line: number): void {
  const { doc } = view.state
  const n = Math.min(Math.max(Math.floor(line), 1), doc.lines)
  const block = view.lineBlockAt(doc.line(n).from)
  view.scrollDOM.scrollTop = block.top + (line - n) * block.height
}

// tieScrollY keeps the panes' vertical scrolling aligned: scrolling one
// scrolls every other to the line at the same height, as the current anchors
// map it. Setting scrollTop fires a scroll event of its own; one that reports
// the value just written is that echo and is ignored, so the panes never
// bounce off each other. Returns untie.
export function tieScrollY(
  panes: Partial<Record<Pane, EditorView>>,
  anchors: () => Anchor[],
): () => void {
  const entries = PANES.flatMap((name) => {
    const view = panes[name]
    return view ? [{ name, view }] : []
  })
  // The scrollTop our own write last set into each pane.
  const echoes = new Map<Pane, number>()

  const listeners = entries.map(({ name, view }) => {
    const onScroll = (): void => {
      if (view.scrollDOM.scrollTop === echoes.get(name)) {
        echoes.delete(name)
        return
      }
      const line = topLine(view)
      const current = anchors()
      for (const other of entries) {
        if (other.name === name) continue
        const scroller = other.view.scrollDOM
        const before = scroller.scrollTop
        scrollToLine(other.view, mapLine(linePairs(current, name, other.name), line))
        if (scroller.scrollTop !== before) echoes.set(other.name, scroller.scrollTop)
      }
    }
    view.scrollDOM.addEventListener('scroll', onScroll, { passive: true })
    return () => view.scrollDOM.removeEventListener('scroll', onScroll)
  })
  return () => listeners.forEach((untie) => untie())
}
