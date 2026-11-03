import type { GraphRow } from '../../bindings/github.com/brogli/nebegit/internal/core'

// A "+k" overflow marker takes two lanes' width right of the highest lane.
const OVERFLOW_LANES = 2

// highestLane is the rightmost lane a row draws in: its dot or a line.
export function highestLane(graph: GraphRow): number {
  return Math.max(graph.lane, ...(graph.edges ?? []).map((e) => e.lane))
}

// laneSpan is how many lanes wide a row's graph is, its overflow marker
// included.
export function laneSpan(graph: GraphRow): number {
  return highestLane(graph) + 1 + (graph.overflow ? OVERFLOW_LANES : 0)
}
