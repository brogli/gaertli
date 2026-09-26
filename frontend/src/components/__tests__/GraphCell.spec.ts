import { mount, type DOMWrapper } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import GraphCell from '../GraphCell.vue'
import { LANE_PALETTE_SIZE, laneColourClass } from '@/lib/laneColours'
import { EdgeKind, type GraphRow } from '../../../bindings/github.com/brogli/nebegit/internal/core'

const HEIGHT = 24
const LANE_WIDTH = 14
const laneX = (lane: number) => lane * LANE_WIDTH + LANE_WIDTH / 2

function mountCell(graph: GraphRow, lanes = 4) {
  return mount(GraphCell, { props: { graph, lanes, height: HEIGHT } })
}

// ends is a path's first and last point.
function ends(path: Pick<DOMWrapper<Element>, 'attributes'>): { from: number[]; to: number[] } {
  const numbers = path
    .attributes('d')!
    .match(/-?\d+(\.\d+)?/g)!
    .map(Number)
  return { from: numbers.slice(0, 2), to: numbers.slice(-2) }
}

describe('GraphCell', () => {
  // Commit on lane 0: its own line and a fork from lane 2 come in, lines go
  // out to its first parent on lane 0 and a merged parent on lane 1, and
  // lane 3 passes by.
  const merge: GraphRow = {
    lane: 0,
    colour: 0,
    edges: [
      { lane: 0, kind: EdgeKind.InEdge, colour: 0 },
      { lane: 2, kind: EdgeKind.InEdge, colour: 2 },
      { lane: 0, kind: EdgeKind.OutEdge, colour: 0 },
      { lane: 1, kind: EdgeKind.OutEdge, colour: 4 },
      { lane: 3, kind: EdgeKind.ThroughEdge, colour: 3 },
    ],
  }

  it('FR-LOG-02 a passing lane is a vertical line from the top of the row to its bottom', () => {
    const through = mountCell(merge).get(`path.through.${laneColourClass(3)}`)

    expect(ends(through)).toEqual({ from: [laneX(3), 0], to: [laneX(3), HEIGHT] })
  })

  it('FR-LOG-02 a fork curves from the top of its lane into the dot; a merge curves from the dot to the bottom of its lane', () => {
    const wrapper = mountCell(merge)
    const dot = wrapper.get('circle')
    const dotAt = [Number(dot.attributes('cx')), Number(dot.attributes('cy'))]
    const fork = wrapper.get(`path.in.${laneColourClass(2)}`)
    const merged = wrapper.get(`path.out.${laneColourClass(4)}`)

    expect(dotAt).toEqual([laneX(0), HEIGHT / 2])
    expect(ends(fork)).toEqual({ from: [laneX(2), 0], to: dotAt })
    expect(ends(merged)).toEqual({ from: dotAt, to: [laneX(1), HEIGHT] })
    expect(fork.attributes('d')).toContain('C')
    expect(merged.attributes('d')).toContain('C')
  })

  it('FR-LOG-02 the dot and every line take their own lane colour, the palette repeating', () => {
    const wrapper = mountCell({
      lane: 0,
      colour: LANE_PALETTE_SIZE,
      edges: [
        { lane: 0, kind: EdgeKind.OutEdge, colour: LANE_PALETTE_SIZE },
        { lane: 1, kind: EdgeKind.ThroughEdge, colour: 1 },
      ],
    })

    expect(wrapper.get('circle').classes()).toContain(laneColourClass(0))
    expect(wrapper.findAll('path').map((p) => p.classes())).toEqual([
      expect.arrayContaining(['out', laneColourClass(0)]),
      expect.arrayContaining(['through', laneColourClass(1)]),
    ])
  })

  it('FR-LOG-02 the strip is as wide as the lanes it is given, not the lanes its row uses', () => {
    const wrapper = mountCell(
      { lane: 1, colour: 1, edges: [{ lane: 0, kind: EdgeKind.ThroughEdge, colour: 0 }] },
      5,
    )

    expect(wrapper.get('svg').attributes('width')).toBe(String(5 * LANE_WIDTH))
  })

  it('FR-LOG-02 the "+k" overflow marker sits right of the highest lane, inside the strip', () => {
    const wrapper = mountCell(
      {
        lane: 1,
        colour: 1,
        edges: [
          { lane: 0, kind: EdgeKind.ThroughEdge, colour: 0 },
          { lane: 1, kind: EdgeKind.OutEdge, colour: 1 },
        ],
        overflow: 3,
      },
      4,
    )

    const marker = wrapper.get('text')
    expect(marker.text()).toBe('+3')
    expect(Number(marker.attributes('x'))).toBe(2 * LANE_WIDTH)
    expect(Number(wrapper.get('svg').attributes('width'))).toBe(4 * LANE_WIDTH)
  })
})
