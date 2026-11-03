// Requirements: FR-LOG-01
import { describe, expect, it } from 'vitest'
import { laneSpan } from '../graphLanes'
import { EdgeKind } from '../../../bindings/github.com/brogli/nebegit/internal/core'

describe('laneSpan', () => {
  it('reaches the highest lane a row draws in, its dot or a line', () => {
    expect(laneSpan({ lane: 0, colour: 0, edges: null })).toBe(1)
    expect(
      laneSpan({ lane: 1, colour: 0, edges: [{ lane: 0, kind: EdgeKind.InEdge, colour: 0 }] }),
    ).toBe(2)
    expect(
      laneSpan({ lane: 0, colour: 0, edges: [{ lane: 2, kind: EdgeKind.OutEdge, colour: 1 }] }),
    ).toBe(3)
  })

  it('keeps two more lanes for a "+k" overflow marker', () => {
    expect(laneSpan({ lane: 1, colour: 0, edges: [], overflow: 4 })).toBe(4)
  })
})
