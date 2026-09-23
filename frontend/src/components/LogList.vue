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
import PickDialog from './PickDialog.vue'
import { buildCommitMenuItems } from '../menus/commitMenu'
import type { LogCommit } from '../../bindings/github.com/brogli/nebegit/internal/core'

// LogList is a virtual-scrolled commit list: only the rows in view are
// rendered, and only their window is fetched.
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

// The cherry-pick/revert options dialog: opened by the "…" menu items, and
// by a plain "Cherry-pick" on a single selected merge commit, which needs a
// parent choice cherry-pick has no default for.
const pickDialog = ref<{ verb: 'cherry-pick' | 'revert'; shas: string[]; singleMerge: boolean } | null>(null)
function closePickDialog(): void {
  pickDialog.value = null
  log.clearMultiSelect()
}

onMounted(() => {
  void log.load(props.wt)
})

const parentRef = ref<HTMLElement | null>(null)

// Rows are one fixed-height line (subject ellipsised), so the virtualizer
// never measures them: dynamic measurement shifted scrollTop while rows
// streamed in, leaving a fresh list scrolled away from the top or blank.
const ROW_HEIGHT = 24

// A row shows this many ref labels, then "+N" naming them all on hover.
const MAX_LABELS = 2

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

// log.jumpTo sets pendingScrollTo: scroll that row into view and clear it.
watch(
  () => log.pendingScrollTo,
  (index) => {
    if (index === null) return
    virtualizer.value.scrollToIndex(index, { align: 'center' })
    log.pendingScrollTo = null
  },
)

// Ctrl/Cmd-click adds row to the compare selection instead of selecting it
// in the log; the first Ctrl-click of a pair seeds the selection with the
// plainly selected row. Shift-click extends a multi-select range; any
// other plain click leaves compare mode and collapses the multi-select to
// this row.
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
  changes.clearSelection() // a commit selection wins over a stale changes-panel diff
  if (event.shiftKey) void log.shiftSelectTo(index)
  else log.selectOnly(row, index)
  void log.select(row)
}

function timeAgo(unixSeconds: number): string {
  return formatTimeAgo(new Date(unixSeconds * 1000))
}

// Right-clicking a row selects it like a left click, then opens the context
// menu. Cherry-pick and revert act on the current multi-select if the
// right-clicked row is part of it, else on that row alone.
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
    cherryPick,
    cherryPickOptions: () => (pickDialog.value = { verb: 'cherry-pick', ...pickTarget() }),
    revert: () => void runAndClose(() => Facade.Revert(props.wt, shasOf(log.multiSelect)), log.clearMultiSelect),
    revertOptions: () => (pickDialog.value = { verb: 'revert', ...pickTarget() }),
    openCreateWorktree: (sha) => emit('create-worktree', sha),
    compareWithWorking: () => compare.startAgainstWorking(row),
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

// pickTarget reads shas and the merge-parent-choice gate off log.multiSelect
// (always at least the right-clicked row — set just above) — no extra core
// call, PickDialog's own doc comment.
function pickTarget(): { shas: string[]; singleMerge: boolean } {
  return {
    shas: shasOf(log.multiSelect),
    singleMerge: log.multiSelect.length === 1 && !!log.multiSelect[0]?.merge,
  }
}

// cherryPick acts directly with the default options, the same call path as
// "Revert" above — except a single selected merge commit, where PickDialog
// takes over for its parent choice.
function cherryPick(): void {
  const target = pickTarget()
  if (target.singleMerge) pickDialog.value = { verb: 'cherry-pick', ...target }
  else void runAndClose(() => Facade.CherryPick(props.wt, target.shas), log.clearMultiSelect)
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
          <span class="hash">{{ row.shortSha }}</span>
          <span
            v-for="label in row.refs?.slice(0, MAX_LABELS)"
            :key="label.name"
            class="ref-label"
            :class="label.kind"
            :title="label.name"
          >
            {{ label.name }}
          </span>
          <span
            v-if="(row.refs?.length ?? 0) > MAX_LABELS"
            class="ref-more"
            :title="row.refs!.map((r) => r.name).join(', ')"
            >+{{ row.refs!.length - MAX_LABELS }}</span
          >
        </template>
      </div>
    </div>
  </div>
  <ContextMenu ref="menuRef" :model="menuItems" />
  <PickDialog
    v-if="pickDialog"
    :wt="wt"
    :verb="pickDialog.verb"
    :shas="pickDialog.shas"
    :single-merge="pickDialog.singleMerge"
    @close="closePickDialog"
  />
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
  gap: 0.4em;
  padding: 0 0.5em;
  white-space: nowrap;
  cursor: pointer;
  overflow: hidden;
}
.row:hover {
  background: var(--p-content-hover-background);
}
/* Aura's own highlight tokens, not a literal colour: readable in both
   themes (dark's highlight background is a translucent tint of the primary
   colour, paired with a light text colour). */
.row.selected {
  background: var(--p-highlight-background);
  color: var(--p-highlight-color);
}
.row.multi-selected {
  background: var(--p-content-hover-background);
}
/* The subject takes the space the other parts leave, up to its whole
   text; only then does the author get any. At 6.5em the subject stops
   shrinking and the ref labels shrink instead, each down to 2em, ending
   in an ellipsis. The row clips what still does not fit, so
   the list never scrolls sideways. */
.subject {
  flex: 1000 1 0;
  min-width: 6.5em;
  max-width: max-content;
  overflow: hidden;
  text-overflow: ellipsis;
}
.author {
  flex: 1 1 0;
  min-width: 0;
  max-width: 8em;
  overflow: hidden;
  text-overflow: ellipsis;
}
.date {
  flex: none;
  color: var(--p-text-muted-color);
}
.hash {
  flex: none;
  font-family: monospace;
  color: var(--p-text-muted-color);
}
.ref-label,
.ref-more {
  flex: none;
  max-width: 6em;
  overflow: hidden;
  text-overflow: ellipsis;
  padding: 0 0.4em;
  border-radius: 3px;
  background: var(--p-content-hover-background);
  font-size: 0.85em;
}
.ref-label {
  flex: 0 1 auto;
  min-width: 2em;
}
</style>
