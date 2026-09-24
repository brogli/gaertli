<script setup lang="ts">
import { computed, onMounted, ref, watch, watchEffect } from 'vue'
import { useI18n } from '../i18n'
import { useVirtualizer } from '@tanstack/vue-virtual'
import ContextMenu from 'openvue/contextmenu'
import { useLogStore } from '../stores/log'
import { useCompareStore } from '../stores/compare'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import GraphCell from './GraphCell.vue'
import ErrorBanner from './ErrorBanner.vue'
import PickDialog from './PickDialog.vue'
import {
  buildComparisonMenuItems,
  buildOneCommitMenuItems,
  buildRangeMenuItems,
  type CommitMenuItem,
} from '../menus/commitMenu'
import { timeAgo } from '../lib/format'
import type { LogCommit, PickOptions } from '../../bindings/github.com/brogli/nebegit/internal/core'

// LogList is a virtual-scrolled commit list: only the rows in view are
// rendered, and only their window is fetched.
const props = defineProps<{ wt: string }>()
const emit = defineEmits<{
  'create-branch': [sha: string]
  'checkout-commit': [sha: string]
  reset: [sha: string, subject: string]
  'create-worktree': [sha: string]
  open: []
}>()
const log = useLogStore()
const compare = useCompareStore()
const { error, runAndClose } = useCoreCall()
const { t } = useI18n()

// The cherry-pick/revert options dialog: opened by the "…" menu items, and
// by a plain "Cherry-pick" on a single selected merge commit, which needs a
// parent choice cherry-pick has no default for.
const pickDialog = ref<{
  verb: 'cherry-pick' | 'revert'
  shas: string[]
  singleMerge: boolean
} | null>(null)
function closePickDialog(): void {
  pickDialog.value = null
  log.clearMultiSelect()
}

onMounted(() => {
  void log.load(props.wt)
})

const parentRef = ref<HTMLElement | null>(null)

// Rows are one fixed-height line (subject ellipsised), so the virtualizer
// never measures them: measuring shifted scrollTop while rows streamed in,
// leaving a fresh list scrolled away from the top or blank.
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

// The rows marked selected. An active comparison marks its two rows
// compare-older/compare-newer instead (below), not selected.
const selectedShas = computed(() => {
  if (compare.active) return new Set<string | undefined>()
  const rows = compare.selection.length ? compare.selection : [log.selectedRow]
  return new Set(rows.map((r) => r?.sha))
})

// One LogRows call per window: from the first visible row not yet cached to
// the last.
watchEffect(() => {
  const missing = visibleRows.value.filter(({ row }) => !row).map(({ item }) => item.index)
  if (missing.length) void log.rows(missing[0]!, missing.at(-1)! + 1)
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
// other plain click leaves the comparison and collapses the multi-select to
// this row.
function onRowClick(row: LogCommit | undefined, index: number, event: MouseEvent): void {
  if (!row) return
  if (event.ctrlKey || event.metaKey) {
    const previous = plainSelectionOtherThan(row)
    if (compare.selection.length === 0 && previous) {
      compare.toggleSelect(props.wt, previous.row, previous.index)
    }
    compare.toggleSelect(props.wt, row, index)
    return
  }
  compare.exit()
  if (event.shiftKey) void log.shiftSelectTo(index)
  else log.selectOnly(row, index)
  void log.select(row)
}

// Right-clicking a row opens the menu that fits what is selected: the pair
// being compared, a range of 2+, or (default) this row alone, which leaves
// any comparison or range behind it.
const menuRef = ref<InstanceType<typeof ContextMenu>>()
const menuItems = ref<CommitMenuItem[]>([])

function onContextMenu(event: MouseEvent, row: LogCommit | undefined, index: number): void {
  if (!row) return

  if (compare.active && isCompared(row)) {
    menuItems.value = buildComparisonMenuItems(comparisonActions(), t)
    menuRef.value?.show(event)
    return
  }

  if (log.multiSelect.length >= 2 && multiSelected.value.has(row.sha)) {
    void log.select(row)
    menuItems.value = buildRangeMenuItems(log.multiSelect, rangeActions(), t)
    menuRef.value?.show(event)
    return
  }

  // compareWithPrevious pairs with whatever was plainly selected before
  // this right-click, captured ahead of the selection changes below.
  const previous = plainSelectionOtherThan(row)
  compare.exit()
  log.selectOnly(row, index)
  void log.select(row)

  const oneActions: Parameters<typeof buildOneCommitMenuItems>[1] = {
    ...pickAndRevertActions(),
    openCreateBranch: (sha) => emit('create-branch', sha),
    openCheckoutCommit: (sha) => emit('checkout-commit', sha),
    openReset: (sha, subject) => emit('reset', sha, subject),
    openCreateWorktree: (sha) => emit('create-worktree', sha),
    compareWithWorking: () => compare.startAgainstWorking(row),
  }
  if (previous) {
    oneActions.compareWithPrevious = {
      shortSha: previous.row.shortSha,
      run: () => {
        compare.toggleSelect(props.wt, previous.row, previous.index)
        compare.toggleSelect(props.wt, row, index)
      },
    }
  }
  menuItems.value = buildOneCommitMenuItems(row, oneActions, t)
  menuRef.value?.show(event)
}

// plainSelectionOtherThan is the plainly selected row and its log index,
// the other half of a comparison with row; null if none or row itself.
function plainSelectionOtherThan(row: LogCommit): { row: LogCommit; index: number } | null {
  const selected = log.selectedRow
  const index = log.anchorIndex
  return selected && index !== null && selected.sha !== row.sha ? { row: selected, index } : null
}

function isCompared(row: LogCommit): boolean {
  return row.sha === compare.older?.sha || row.sha === compare.newer?.sha
}

// pickAndRevertActions is the cherry-pick/revert action set shared by the
// one-commit and range menus: both act on log.multiSelect, which
// selectOnly/shiftSelectTo keep in sync with the visible selection.
function pickAndRevertActions() {
  return {
    cherryPick,
    cherryPickOptions: () => (pickDialog.value = { verb: 'cherry-pick', ...pickTarget() }),
    revert: () =>
      void runAndClose(
        () => Facade.Revert(props.wt, shasOf(log.multiSelect), gitDefaults),
        log.clearMultiSelect,
      ),
    revertOptions: () => (pickDialog.value = { verb: 'revert', ...pickTarget() }),
  }
}

function rangeActions(): Parameters<typeof buildRangeMenuItems>[1] {
  return {
    ...pickAndRevertActions(),
    copyHashes: () => void navigator.clipboard.writeText(shasOf(log.multiSelect).join('\n')),
  }
}

function comparisonActions(): Parameters<typeof buildComparisonMenuItems>[0] {
  return {
    swap: compare.swap,
    stopComparing: compare.exit,
    copyBothHashes: () =>
      void navigator.clipboard.writeText(`${compare.from?.sha}\n${compare.to?.sha}`),
  }
}

function shasOf(rows: LogCommit[]): string[] {
  return rows.map((r) => r.sha)
}

// pickTarget is the multi-select's shas, and whether they are one merge
// commit. The multi-select always holds at least the right-clicked row.
function pickTarget(): { shas: string[]; singleMerge: boolean } {
  return {
    shas: shasOf(log.multiSelect),
    singleMerge: log.multiSelect.length === 1 && !!log.multiSelect[0]?.merge,
  }
}

// gitDefaults are the plain "Cherry-pick" and "Revert" items' options.
const gitDefaults: PickOptions = { noCommit: false, recordOrigin: false, mainline: 0 }

// cherryPick picks with git's defaults, except a single merge commit:
// PickDialog asks for its parent.
function cherryPick(): void {
  const target = pickTarget()
  if (target.singleMerge) pickDialog.value = { verb: 'cherry-pick', ...target }
  else
    void runAndClose(
      () => Facade.CherryPick(props.wt, target.shas, gitDefaults),
      log.clearMultiSelect,
    )
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
          selected: selectedShas.has(row?.sha ?? ''),
          'multi-selected': multiSelected.size > 1 && multiSelected.has(row?.sha ?? ''),
          'compare-older': compare.active && row?.sha === compare.older?.sha,
          'compare-newer': compare.active && row?.sha === compare.newer?.sha,
        }"
        :style="{ transform: `translateY(${item.start}px)` }"
        tabindex="0"
        @click="onRowClick(row, item.index, $event)"
        @dblclick="emit('open')"
        @keydown.enter.self="emit('open')"
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
/* Aura's highlight tokens, not a literal colour: readable in both themes. */
.row.selected {
  background: var(--p-highlight-background);
  color: var(--p-highlight-color);
}
.row.multi-selected {
  background: var(--p-content-hover-background);
}
/* A compared row's own colour replaces the shared .selected look: a left
   border plus a light tint of the row background in that colour. */
.row.compare-older {
  border-left: 3px solid var(--nb-older);
  background: color-mix(in srgb, var(--nb-older) 15%, var(--p-content-background));
}
.row.compare-newer {
  border-left: 3px solid var(--nb-newer);
  background: color-mix(in srgb, var(--nb-newer) 15%, var(--p-content-background));
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
