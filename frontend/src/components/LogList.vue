<script setup lang="ts">
import { computed, onMounted, ref, watch, watchEffect } from 'vue'
import { useVirtualizer } from '@tanstack/vue-virtual'
import { formatTimeAgo } from '@vueuse/core'
import ContextMenu from 'openvue/contextmenu'
import { useLogStore } from '../stores/log'
import { useCompareStore } from '../stores/compare'
import { useChangesStore } from '../stores/changes'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import GraphCell from './GraphCell.vue'
import ErrorBanner from './ErrorBanner.vue'
import { buildCommitMenuItems } from '../menus/commitMenu'
import type { LogCommit } from '../../bindings/github.com/brogli/nebegit/internal/core'

// LogList is a virtual-scrolled commit list (ADR-5, QA-PERF-03): only the
// rows in view are rendered, and only their window is fetched (FR-LOG-09).
const props = defineProps<{ wt: string }>()
const emit = defineEmits<{
  'create-branch': [sha: string]
  'checkout-commit': [sha: string]
  reset: [sha: string, subject: string]
  'create-worktree': [sha: string]
}>()
const log = useLogStore()
const compare = useCompareStore()
const changes = useChangesStore()
const { error, runAndClose } = useCoreCall()

onMounted(() => {
  void log.load(props.wt)
})

const parentRef = ref<HTMLElement | null>(null)

// Rows are one fixed-height line (subject ellipsised): the virtualizer
// never measures them, so it never shifts scrollTop to compensate for a
// size change while rows stream in — dynamic measurement left a freshly
// loaded list scrolled away from the top, or with its rendered range
// outside the viewport (blank), on every load of a real repo.
const ROW_HEIGHT = 24

const virtualizer = useVirtualizer(
  computed(() => ({
    count: log.activeLoaded,
    getScrollElement: () => parentRef.value,
    estimateSize: () => ROW_HEIGHT,
    overscan: 12,
  })),
)

const visibleRows = computed(() =>
  virtualizer.value.getVirtualItems().map((item) => ({ item, row: log.rowCache.get(item.index) })),
)
const multiSelected = computed(() => new Set(log.multiSelect.map((r) => r.sha)))

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

// Ctrl/Cmd-click adds row to the compare selection (FR-CMP-01) instead of
// selecting it in the log; the first Ctrl-click of a pair seeds the
// selection with whatever was plainly selected before it, so two
// Ctrl-clicks still work when the compare selection is empty. Shift-click
// extends a multi-select range (FR-LOG-11); any other plain click leaves
// compare mode and collapses the multi-select to this row.
function onRowClick(row: LogCommit | undefined, index: number, event: MouseEvent): void {
  if (!row) return
  if (event.ctrlKey || event.metaKey) {
    if (compare.selection.length === 0 && log.selectedRow && log.selectedRow.sha !== row.sha) {
      compare.toggleSelect(log.selectedRow) // the plainly selected commit is the "from" side
    }
    compare.toggleSelect(row)
    return
  }
  compare.exit()
  changes.clearSelection() // FR-UX-07: a commit selection wins over a stale changes-panel diff
  if (event.shiftKey) void log.shiftSelectTo(index)
  else log.selectOnly(row, index)
  void log.select(row)
}

function timeAgo(unixSeconds: number): string {
  return formatTimeAgo(new Date(unixSeconds * 1000))
}

// Right-click context menu (FR-LOG-11/12): right-clicking a row selects it
// like a left click, then opens OpenVue's ContextMenu, which owns
// positioning and dismissal. Cherry-pick and revert act on the current
// multi-select if the right-clicked row is part of it, else on that row
// alone; both are stateless writes this component runs itself.
const menuRef = ref<InstanceType<typeof ContextMenu>>()
const menuItems = ref<ReturnType<typeof buildCommitMenuItems>>([])

function onContextMenu(event: MouseEvent, row: LogCommit | undefined, index: number): void {
  if (!row) return
  // compareWithSelected pairs with whatever was selected before this
  // right-click, captured ahead of the selection changes below.
  const previouslySelected = log.selectedRow
  if (!multiSelected.value.has(row.sha)) log.selectOnly(row, index)
  void log.select(row)
  const actions: Parameters<typeof buildCommitMenuItems>[1] = {
    openCreateBranch: (sha) => emit('create-branch', sha),
    openCheckoutCommit: (sha) => emit('checkout-commit', sha),
    openReset: (sha, subject) => emit('reset', sha, subject),
    cherryPick: () => void runAndClose(() => Facade.CherryPick(props.wt, shasOf(log.multiSelect)), log.clearMultiSelect),
    revert: () => void runAndClose(() => Facade.Revert(props.wt, shasOf(log.multiSelect)), log.clearMultiSelect),
    openCreateWorktree: (sha) => emit('create-worktree', sha),
  }
  if (previouslySelected && previouslySelected.sha !== row.sha) {
    actions.compareWithSelected = () => {
      compare.exit() // a clean two-item pair regardless of any prior ctrl-click state
      compare.toggleSelect(previouslySelected)
      compare.toggleSelect(row)
    }
  }
  menuItems.value = buildCommitMenuItems(row, actions)
  menuRef.value?.show(event)
}

function shasOf(rows: LogCommit[]): string[] {
  return rows.map((r) => r.sha)
}
</script>

<template>
  <ErrorBanner v-if="error" :error="error" />
  <div ref="parentRef" class="log-list" :style="{ '--row-height': `${ROW_HEIGHT}px` }">
    <div class="spacer" :style="{ height: `${virtualizer.getTotalSize()}px` }">
      <div
        v-for="{ item, row } in visibleRows"
        :key="item.index"
        :data-index="item.index"
        class="row"
        :class="{
          selected: row?.sha === log.selectedRow?.sha,
          'compare-selected': compare.selection.some((r) => r.sha === row?.sha),
          'multi-selected': multiSelected.size > 1 && multiSelected.has(row?.sha ?? ''),
        }"
        :style="{ transform: `translateY(${item.start}px)` }"
        @click="onRowClick(row, item.index, $event)"
        @contextmenu.prevent="onContextMenu($event, row, item.index)"
      >
        <template v-if="row">
          <GraphCell v-if="!log.flat" :graph="row.graph" />
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
  <ContextMenu ref="menuRef" :model="menuItems" />
</template>

<style scoped>
.log-list {
  flex: 1;
  min-height: 0;
  overflow: auto;
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
  height: var(--row-height);
  display: flex;
  align-items: center;
  gap: 0.5em;
  white-space: nowrap;
  cursor: pointer;
}
.row.selected {
  background: var(--selected-bg, #e6f0ff);
}
.row.multi-selected {
  background: var(--multi-selected-bg, #dce8fb);
}
.subject {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
