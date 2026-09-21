<script setup lang="ts">
import { computed, onMounted, ref, watch, watchEffect, type ComponentPublicInstance } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'
import { formatTimeAgo } from '@vueuse/core'
import { useLogStore } from '../stores/log'
import GraphCell from './GraphCell.vue'
import type { LogCommit } from '../../bindings/github.com/brogli/nebegit/internal/core'

// LogList is a virtual-scrolled commit list (ADR-5, QA-PERF-03): only the
// rows in view are rendered, and only their window is fetched (FR-LOG-09).
const props = defineProps<{ wt: string }>()
const log = useLogStore()

onMounted(() => {
  void log.load(props.wt)
})

const parentRef = ref<HTMLElement | null>(null)

const virtualizer = useVirtualizer(
  computed(() => ({
    count: log.loaded,
    getScrollElement: () => parentRef.value,
    estimateSize: () => 32,
    overscan: 12,
  })),
)

const visibleRows = computed(() =>
  virtualizer.value.getVirtualItems().map((item) => ({ item, row: log.rowCache.get(item.index) })),
)

watchEffect(() => {
  for (const { item, row } of visibleRows.value) {
    if (!row) void log.rows(item.index, item.index + 1)
  }
})

// A sidebar ref click sets pendingScrollTo (FR-LOG-01, log store's
// jumpTo): scroll that row into view and clear the pending index.
watch(
  () => log.pendingScrollTo,
  (index) => {
    if (index === null) return
    virtualizer.value.scrollToIndex(index, { align: 'center' })
    log.pendingScrollTo = null
  },
)

function measureElement(el: Element | ComponentPublicInstance | null): void {
  if (el instanceof Element) virtualizer.value.measureElement(el)
}

function onRowClick(row: LogCommit | undefined): void {
  if (row) void log.select(row)
}

function timeAgo(unixSeconds: number): string {
  return formatTimeAgo(new Date(unixSeconds * 1000))
}

// Copy actions (FR-LOG-12): browser clipboard API only, no core.API call.
function copyHash(): void {
  const sha = log.selectedRow?.sha
  if (sha) void navigator.clipboard.writeText(sha)
}

function copySubject(): void {
  const subject = log.selectedRow?.subject
  if (subject) void navigator.clipboard.writeText(subject)
}
</script>

<template>
  <div v-if="log.selectedRow" class="row-actions">
    <button type="button" data-test="copy-hash" @click="copyHash">copy hash</button>
    <button type="button" data-test="copy-subject" @click="copySubject">copy subject</button>
  </div>
  <div ref="parentRef" class="log-list">
    <div class="spacer" :style="{ height: `${virtualizer.getTotalSize()}px` }">
      <div
        v-for="{ item, row } in visibleRows"
        :key="item.index"
        :ref="measureElement"
        :data-index="item.index"
        class="row"
        :class="{ selected: row?.sha === log.selectedRow?.sha }"
        :style="{ transform: `translateY(${item.start}px)` }"
        @click="onRowClick(row)"
      >
        <template v-if="row">
          <GraphCell :graph="row.graph" />
          <span class="subject">{{ row.subject }}</span>
          <span class="author">{{ row.author }}</span>
          <span class="date">{{ timeAgo(row.authorAt) }}</span>
          <span
            v-for="label in row.refs ?? []"
            :key="label.name"
            class="ref-label"
            :class="label.kind"
          >
            {{ label.name }}
          </span>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.log-list {
  flex: 1;
  min-height: 0;
  overflow: auto;
  /* Rows above the fold shrink as @tanstack/vue-virtual corrects its
     estimated size to the real measured height; without this, the browser's
     own scroll anchoring "compensates" by moving scrollTop, which the
     virtualizer then reads as a real scroll and chases (visible DOM rows
     drift to whatever index that chase settles on). */
  overflow-anchor: none;
  position: relative;
}
.spacer {
  position: relative;
  width: 100%;
}
.row {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  display: flex;
  align-items: center;
  gap: 0.5em;
  cursor: pointer;
}
.row.selected {
  background: var(--selected-bg, #e6f0ff);
}
.subject {
  flex: 1;
}
</style>
