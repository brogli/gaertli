<script setup lang="ts">
import { computed } from 'vue'
import type { GraphRow } from '../../bindings/github.com/brogli/nebegit/internal/core'

// GraphCell renders one row's GraphRow as a small inline SVG strip (ADR-5):
// a vertical line per edge, a dot at the row's own lane, a "+k" badge when
// lanes overflowed Layout's cap.
const props = defineProps<{ graph: GraphRow }>()

const laneWidth = 14
const height = 22

const edges = computed(() => props.graph.edges ?? [])
const laneCount = computed(() => {
  const lanes = edges.value.map((e) => e.lane).concat(props.graph.lane)
  return Math.max(0, ...lanes) + 1
})
const width = computed(() => laneCount.value * laneWidth)

function x(lane: number): number {
  return lane * laneWidth + laneWidth / 2
}
</script>

<template>
  <svg class="graph-cell" :width="width" :height="height">
    <line
      v-for="edge in edges"
      :key="`${edge.lane}-${edge.kind}`"
      :x1="x(edge.lane)"
      :x2="x(edge.lane)"
      y1="0"
      :y2="height"
      class="edge"
      :class="edge.kind"
    />
    <circle
      :cx="x(graph.lane)"
      :cy="height / 2"
      r="4"
      class="dot"
      :class="`colour-${graph.colour}`"
    />
    <text v-if="graph.overflow" :x="width" :y="height / 2 + 4" class="overflow">
      +{{ graph.overflow }}
    </text>
  </svg>
</template>

<style scoped>
.graph-cell {
  overflow: visible;
}
.edge {
  stroke: currentColor;
  stroke-width: 2;
}
.edge.start,
.edge.end {
  stroke-dasharray: 3 2;
}
.dot {
  fill: currentColor;
}
.overflow {
  font-size: 0.75em;
}
</style>
