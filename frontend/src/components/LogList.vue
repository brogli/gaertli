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
import SquashDialog from './SquashDialog.vue'
import {
  buildOneCommitMenuItems,
  buildSelectionMenuItems,
  buildSquashMenuItems,
  type CommitMenuItem,
} from '../menus/commitMenu'
import { timeAgo } from '../lib/format'
import { laneSpan } from '../lib/graphLanes'
import { errorHeadline } from '../lib/errorText'
import type {
  LogCommit,
  PickOptions,
  SquashCheck,
} from '../../bindings/github.com/brogli/nebegit/internal/core'

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
const { error, run, runAndClose } = useCoreCall()
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
  log.dropSelection()
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
const renderedRows = computed(() => visibleRows.value.flatMap(({ row }) => (row ? [row] : [])))
// The graph and hash columns are as wide as the widest graph and the
// longest short hash among the rendered rows, so every row's parts start
// at the same x.
const graphLanes = computed(() => Math.max(0, ...renderedRows.value.map((r) => laneSpan(r.graph))))
const hashWidth = computed(
  () => `${Math.max(0, ...renderedRows.value.map((r) => r.shortSha.length))}ch`,
)
const multiSelected = computed(() => new Set(log.selection.map((r) => r.sha)))

// The search's own highlight: every matched row index, and the one the log
// is currently showing.
const matchedIndices = computed(() => new Set(log.matches))
const currentMatchIndex = computed(() => log.matches[log.current])

// The rows marked selected. An active comparison marks its two rows
// compare-from/compare-to instead (below), not selected.
const selectedShas = computed(() => {
  if (compare.active) return new Set<string | undefined>()
  const rows = log.selection.length ? log.selection : [log.selectedRow]
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

// Ctrl/Cmd-click toggles row in the selection. Shift-click replaces the
// selection with the range from the anchor to row. Any other plain click
// drops a working-tree comparison and collapses the selection to this row.
function onRowClick(row: LogCommit | undefined, index: number, event: MouseEvent): void {
  if (!row) return
  if (event.ctrlKey || event.metaKey) {
    void log.togglePick(row, index)
    return
  }
  compare.dropWorkingCompare()
  if (event.shiftKey) void log.shiftSelectTo(index)
  else log.selectOnly(row, index)
  void log.select(row)
}

// Right-clicking a row opens the menu that fits what is selected: two or
// more selected commits, however selected, or (default) this row alone,
// which drops any selection behind it. Every menu on several commits ends
// in squash and fixup.
const menuRef = ref<InstanceType<typeof ContextMenu>>()
const menuItems = ref<CommitMenuItem[]>([])

async function onContextMenu(
  event: MouseEvent,
  row: LogCommit | undefined,
  index: number,
): Promise<void> {
  if (!row) return

  if (log.selection.length >= 2 && multiSelected.value.has(row.sha)) {
    void log.select(row)
    const squash = await squashItems(log.selection)
    showMenu(event, buildSelectionMenuItems(log.selection, selectionActions(), squash, t))
    return
  }

  // compareWithPrevious pairs with whatever was plainly selected before
  // this right-click, captured ahead of the selection changes below.
  const previous = plainSelectionOtherThan(row)
  compare.dropWorkingCompare()
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
      shortSha: previous.shortSha,
      run: () => log.selectExactly([previous, row]),
    }
  }
  showMenu(event, buildOneCommitMenuItems(row, oneActions, t))
}

function showMenu(event: MouseEvent, items: CommitMenuItem[]): void {
  menuItems.value = items
  if (items.length) menuRef.value?.show(event)
}

// squashItems is the squash and fixup section for rows, as SquashCheck
// allows them; a failed check shows in the ErrorBanner and adds nothing.
async function squashItems(rows: LogCommit[]): Promise<CommitMenuItem[]> {
  const shas = shasOf(rows)
  const check = await run(() => Facade.SquashCheck(props.wt, shas))
  if (!check) return []
  const open = (fixup: boolean) => () => (squashDialog.value = { shas, check, fixup })
  const blocker = check.blocker ? errorHeadline(check.blocker) : null
  return buildSquashMenuItems(shas.length, blocker, { squash: open(false), fixup: open(true) }, t)
}

// The squash dialog, for squash or fixup of shas as check allows.
const squashDialog = ref<{ shas: string[]; check: SquashCheck; fixup: boolean } | null>(null)

function onFolded(): void {
  squashDialog.value = null
  compare.exit()
}

// plainSelectionOtherThan is the plainly selected row, the other half of a
// comparison with row; null if none or row itself.
function plainSelectionOtherThan(row: LogCommit): LogCommit | null {
  const selected = log.selectedRow
  return selected && log.anchorIndex !== null && selected.sha !== row.sha ? selected : null
}

// pickAndRevertActions is the cherry-pick/revert action set shared by the
// one-commit and selection menus: both act on log.selection, which
// selectOnly/togglePick/shiftSelectTo keep in sync with the visible
// selection.
function pickAndRevertActions() {
  return {
    cherryPick,
    cherryPickOptions: () => (pickDialog.value = { verb: 'cherry-pick', ...pickTarget() }),
    revert: () =>
      void runAndClose(
        () => Facade.Revert(props.wt, pickTarget().shas, gitDefaults),
        log.dropSelection,
      ),
    revertOptions: () => (pickDialog.value = { verb: 'revert', ...pickTarget() }),
  }
}

function selectionActions(): Parameters<typeof buildSelectionMenuItems>[1] {
  return {
    ...pickAndRevertActions(),
    copyHashes: () => void navigator.clipboard.writeText(shasOf(log.selection).join('\n')),
    swap: compare.swap,
    stopComparing: compare.exit,
  }
}

function shasOf(rows: LogCommit[]): string[] {
  return rows.map((r) => r.sha)
}

// pickTarget is the selection's shas, and whether they are one merge
// commit. The selection always holds at least the right-clicked row.
// CherryPick and Revert do their own ordering, so the shas go as selected.
function pickTarget(): { shas: string[]; singleMerge: boolean } {
  return {
    shas: shasOf(log.selection),
    singleMerge: log.selection.length === 1 && !!log.selection[0]?.merge,
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
    void runAndClose(() => Facade.CherryPick(props.wt, target.shas, gitDefaults), log.dropSelection)
}
</script>

<template>
  <ErrorBanner v-if="error" :error="error" />
  <div
    ref="parentRef"
    class="log-list"
    :style="{
      '--row-height': `${ROW_HEIGHT}px`,
      '--hash-width': hashWidth,
      userSelect: 'none',
    }"
  >
    <div class="spacer" :style="{ height: `${virtualizer.getTotalSize()}px` }">
      <div
        v-for="{ item, row } in visibleRows"
        :key="item.index"
        :data-index="item.index"
        class="row"
        :class="{
          selected: selectedShas.has(row?.sha ?? ''),
          'multi-selected': multiSelected.size > 1 && multiSelected.has(row?.sha ?? ''),
          'compare-from': compare.active && row?.sha === compare.from?.sha,
          'compare-to': compare.active && row?.sha === compare.to?.sha,
          match: matchedIndices.has(item.index),
          'match-current': item.index === currentMatchIndex,
        }"
        :style="{ transform: `translateY(${item.start}px)` }"
        tabindex="0"
        @click="onRowClick(row, item.index, $event)"
        @dblclick="emit('open')"
        @keydown.enter.self="emit('open')"
        @contextmenu.prevent="void onContextMenu($event, row, item.index)"
      >
        <template v-if="row">
          <GraphCell v-if="!log.flat" :graph="row.graph" :lanes="graphLanes" :height="ROW_HEIGHT" />
          <span class="message">
            <span class="subject" :title="row.subject">{{ row.subject }}</span>
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
          </span>
          <span class="author" :title="row.author">{{ row.author }}</span>
          <span class="date" :title="timeAgo(row.authorAt)">{{ timeAgo(row.authorAt) }}</span>
          <span class="hash">{{ row.shortSha }}</span>
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
  <SquashDialog
    v-if="squashDialog"
    :wt="wt"
    v-bind="squashDialog"
    @close="squashDialog = null"
    @done="onFolded"
  />
</template>

<style scoped>
.log-list {
  flex: 1;
  min-height: 0;
  overflow: auto;
  position: relative;
  container-type: inline-size;
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
/* A compared row's own colour replaces the shared .selected look: a bar
   at its left edge, drawn as an inset shadow so it takes no space, plus a
   light tint of the row background in that colour. */
.row.compare-from {
  box-shadow: inset 3px 0 var(--nb-from);
  background: color-mix(in srgb, var(--nb-from) 15%, var(--p-content-background));
}
.row.compare-to {
  box-shadow: inset 3px 0 var(--nb-to);
  background: color-mix(in srgb, var(--nb-to) 15%, var(--p-content-background));
}
/* A search match, and the current one stronger: yellow, kept apart from
   selection's blue and the comparison's own colours. */
.row.match {
  background: color-mix(in srgb, var(--p-yellow-500) 20%, var(--p-content-background));
}
.row.match-current {
  background: color-mix(in srgb, var(--p-yellow-500) 45%, var(--p-content-background));
}
/* Every row splits its width the same way, whatever its text: the message
   column takes what the others leave, down to its minimum; below that the
   author gives way first, then the date, each shrinking down to where it
   could still show a readable value. The row clips what still does not
   fit, so the list never scrolls sideways. */
.message {
  flex: 1 1 0;
  min-width: 13em;
  display: flex;
  align-items: center;
  gap: 0.4em;
  overflow: hidden;
}
.author {
  flex: 0 1000 8em;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}
.date {
  flex: 0 1 7em;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--p-text-muted-color);
}
.hash {
  flex: none;
  width: var(--hash-width);
  font-family: var(--nb-font-mono);
  color: var(--p-text-muted-color);
}
/* Narrower than that, shrinking stops being readable at all: the column
   hides instead of shrinking to a sliver, author first. */
@container (max-width: 480px) {
  .author {
    display: none;
  }
}
@container (max-width: 360px) {
  .date {
    display: none;
  }
}
/* Inside the message column the subject takes its whole text first, then
   the ref labels. At 6.5em the subject stops shrinking and the labels
   shrink instead, each down to 2em, ending in an ellipsis. */
.subject {
  flex: 1000 1 0;
  min-width: 6.5em;
  max-width: max-content;
  overflow: hidden;
  text-overflow: ellipsis;
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
