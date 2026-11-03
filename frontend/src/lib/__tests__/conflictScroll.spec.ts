// Requirements: FR-CONF-05
import { afterEach, describe, expect, it } from 'vitest'
import { EditorView } from '@codemirror/view'
import type { Pane } from '../conflictPanes'
import { linePairs, mapLine, regionAnchors, tieScrollY, type Anchor } from '../conflictScroll'

const map = (anchors: Anchor[], from: Pane, to: Pane, line: number) =>
  mapLine(linePairs(anchors, from, to), line)

describe('conflictScroll', () => {
  it('builds anchors from file starts, region edges and file ends', () => {
    const anchors = regionAnchors(
      [
        {
          current: { start: 3, lines: 2 },
          result: { start: 4, lines: 6 },
          incoming: { start: 3, lines: 0 },
        },
      ],
      { current: 10, result: 20, incoming: 8 },
    )
    expect(anchors).toEqual([
      { current: 1, result: 1, incoming: 1 },
      { current: 3, result: 4, incoming: 4 },
      { current: 5, result: 10, incoming: 4 },
      { current: 11, result: 21, incoming: 9 },
    ])
  })

  it('drops anchors that would go backwards in a pane', () => {
    const anchors = regionAnchors(
      [
        { current: { start: 5, lines: 1 }, result: { start: 5, lines: 1 } },
        { current: { start: 3, lines: 1 }, result: { start: 8, lines: 1 } },
      ],
      { current: 10, result: 10 },
    )
    expect(anchors).toEqual([
      { current: 1, result: 1 },
      { current: 5, result: 5 },
      { current: 6, result: 6 },
      { current: 11, result: 11 },
    ])
  })

  const anchors: Anchor[] = [
    { current: 1, result: 1, incoming: 1 },
    { current: 5, result: 9, incoming: 3 },
    { current: 7, result: 11, incoming: 5 },
    { current: 11, result: 15, incoming: 9 },
  ]

  it('maps anchor lines exactly', () => {
    expect(map(anchors, 'current', 'result', 5)).toBe(9)
    expect(map(anchors, 'incoming', 'current', 5)).toBe(7)
    expect(map(anchors, 'result', 'incoming', 1)).toBe(1)
  })

  it('maps lines between anchors linearly', () => {
    expect(map(anchors, 'current', 'result', 3)).toBe(5)
    expect(map(anchors, 'current', 'incoming', 2)).toBe(1.5)
    expect(map(anchors, 'current', 'result', 9.5)).toBe(13.5)
  })

  it('maps lines before the first and after the last anchor by offset', () => {
    const inner = anchors.slice(1, 3)
    expect(map(inner, 'current', 'result', 4)).toBe(8)
    expect(map(inner, 'current', 'result', 9)).toBe(13)
  })

  it('maps by the anchors that have both panes, whatever their spacing', () => {
    const partial: Anchor[] = [
      { current: 1, result: 1 },
      { current: 4, result: 4, incoming: 2 },
      { current: 4, result: 6 },
      { current: 10, result: 12 },
    ]
    expect(map(partial, 'current', 'result', 7)).toBe(9)
    expect(map(partial, 'incoming', 'result', 2)).toBe(4)
    expect(map([], 'current', 'result', 3)).toBe(3)
  })

  describe('tieScrollY', () => {
    const views: EditorView[] = []
    afterEach(() => views.splice(0).forEach((v) => v.destroy()))

    function pane(): EditorView {
      const view = new EditorView({
        doc: Array.from({ length: 100 }, (_, i) => `line ${i + 1}`).join('\n'),
        parent: document.body,
      })
      views.push(view)
      return view
    }

    // lineTop is a line's offset from the top of the document.
    const lineTop = (view: EditorView, n: number) =>
      view.lineBlockAt(view.state.doc.line(n).from).top

    function scrollTo(view: EditorView, top: number): void {
      view.scrollDOM.scrollTop = top
      view.scrollDOM.dispatchEvent(new Event('scroll'))
    }

    it('scrolls the other panes to the line at the same height', () => {
      const [current, result] = [pane(), pane()]
      const anchors: Anchor[] = [
        { current: 1, result: 1 },
        { current: 11, result: 31 },
        { current: 101, result: 101 },
      ]
      const untie = tieScrollY(
        { current, result } satisfies Partial<Record<Pane, EditorView>>,
        () => anchors,
      )

      scrollTo(current, lineTop(current, 11))
      expect(result.scrollDOM.scrollTop).toBe(lineTop(result, 31))

      scrollTo(result, lineTop(result, 1))
      expect(current.scrollDOM.scrollTop).toBe(lineTop(current, 1))

      untie()
      scrollTo(current, lineTop(current, 11))
      expect(result.scrollDOM.scrollTop).toBe(lineTop(result, 1))
    })

    it('does not bounce a scroll back to the pane it came from', () => {
      const [current, result] = [pane(), pane()]
      const anchors: Anchor[] = [
        { current: 1, result: 1 },
        { current: 11, result: 31 },
        { current: 101, result: 101 },
      ]
      tieScrollY({ current, result }, () => anchors)

      const origin = lineTop(current, 5)
      scrollTo(current, origin)
      result.scrollDOM.dispatchEvent(new Event('scroll'))
      expect(current.scrollDOM.scrollTop).toBe(origin)
    })
  })
})
