<script setup lang="ts">
import { computed } from 'vue'
import {
  EdgeKind,
  type GraphEdge,
  type GraphRow,
} from '../../bindings/github.com/brogli/nebegit/internal/core'
import { laneColourClass } from '../lib/laneColours'
import { highestLane } from '../lib/graphLanes'

// GraphCell draws one row's GraphRow as an SVG strip `lanes` wide and
// `height` tall: a line per edge, the dot at the row's own lane on top of
// them, and a "+k" marker when lanes overflowed Layout's cap. Lines end at
// the strip's top and bottom edges, where they meet the rows around it.
const props = defineProps<{ graph: GraphRow; lanes: number; height: number }>()

const LANE_WIDTH = 14

type Point = [x: number, y: number]

const edges = computed(() => props.graph.edges ?? [])
const dot = computed<Point>(() => [x(props.graph.lane), props.height / 2])

function x(lane: number): number {
  return lane * LANE_WIDTH + LANE_WIDTH / 2
}

// ends: `in` runs from its lane's top into the dot, `out` from the dot to
// its lane's bottom, `through` from its lane's top to its bottom.
function ends(edge: GraphEdge): [Point, Point] {
  const top: Point = [x(edge.lane), 0]
  const bottom: Point = [x(edge.lane), props.height]
  if (edge.kind === EdgeKind.InEdge) return [top, dot.value]
  if (edge.kind === EdgeKind.OutEdge) return [dot.value, bottom]
  return [top, bottom]
}

// path is a cubic curve that leaves and arrives vertically; with both ends
// in one lane it is a straight line.
function path(edge: GraphEdge): string {
  const [[x1, y1], [x2, y2]] = ends(edge)
  const midY = (y1 + y2) / 2
  return `M${x1} ${y1} C${x1} ${midY} ${x2} ${midY} ${x2} ${y2}`
}
</script>

<template>
  <svg class="graph-cell" :width="lanes * LANE_WIDTH" :height="height">
    <path
      v-for="edge in edges"
      :key="`${edge.lane}-${edge.kind}`"
      :d="path(edge)"
      class="edge"
      :class="[edge.kind, laneColourClass(edge.colour)]"
    />
    <circle :cx="dot[0]" :cy="dot[1]" r="4" class="dot" :class="laneColourClass(graph.colour)" />
    <text
      v-if="graph.overflow"
      :x="(highestLane(graph) + 1) * LANE_WIDTH"
      :y="height / 2 + 4"
      class="overflow"
    >
      +{{ graph.overflow }}
    </text>
  </svg>
</template>

<style scoped>
.graph-cell {
  flex: none;
}
.edge {
  fill: none;
  stroke: currentColor;
  stroke-width: 2;
}
.dot {
  fill: currentColor;
}
.overflow {
  font-size: 0.75em;
}

/* The lane palette (`lib/laneColours.ts`): distinct from the diff's
   green/red and the comparison's from/to blue/orange. */
.colour-0 {
  color: var(--nb-tone-violet);
}
.colour-1 {
  color: var(--nb-tone-teal);
}
.colour-2 {
  color: var(--nb-tone-pink);
}
.colour-3 {
  color: var(--nb-tone-cyan);
}
.colour-4 {
  color: var(--nb-tone-purple);
}
.colour-5 {
  color: var(--nb-tone-indigo);
}
</style>
