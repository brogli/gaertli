<script setup lang="ts">
import { computed, onMounted, ref, watch, watchEffect } from 'vue'
import { useI18n } from '@/i18n'
import { useVirtualizer } from '@tanstack/vue-virtual'
import ContextMenu from 'openvue/contextmenu'
import { useLogStore } from '@/stores/log'
import { useCompareStore } from '@/stores/compare'
import { useCoreCall } from '@/composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import GraphCell from './GraphCell.vue'
import ErrorBanner from './ErrorBanner.vue'
import PickDialog from './PickDialog.vue'
import SquashDialog from './SquashDialog.vue'
import ShortcutHint from './ShortcutHint.vue'
import {
  buildOneCommitMenuItems,
  buildSelectionMenuItems,
  buildSquashMenuItems,
  type CommitMenuItem,
} from '@/menus/commitMenu'
import { exactDate, timeAgo } from '@/lib/format'
import { laneSpan } from '@/lib/graphLanes'
import { openOnConflicts } from '@/lib/openConflicts'
import { errorHeadline } from '@/lib/errorText'
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

// The cherry-pick/revert options dialog: opened by the "…" menu items only.
const pickDialog = ref<{ verb: 'cherry-pick' | 'revert'; shas: string[] } | null>(null)
function closePickDialog(): void {
  pickDialog.value = null
  log.dropSelection()
}

onMounted(() => {
  void log.load(props.wt)
})

const parentRef = ref<HTMLElement | null>(null)

// Rows are one fixed-height line (subject ellipsised), so the virtualizer
// never measures them: measuring shifts scrollTop while rows stream in,
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

// A row's marks, strongest first: a search match beats a comparison mark,
// which beats multi-select, which beats a plain selection. The row takes
// the strongest mark's background; hover only shows on an unmarked row.
function rowMarks(sha: string | undefined, index: number): RowMark[] {
  const has = (shas: Set<string | undefined>) => sha !== undefined && shas.has(sha)
  const marks: [RowMark, boolean][] = [
    ['match-current', index === currentMatchIndex.value],
    ['match', matchedIndices.value.has(index)],
    ['compare-to', compare.active && sha !== undefined && sha === compare.to?.sha],
    ['compare-from', compare.active && sha !== undefined && sha === compare.from?.sha],
    ['multi-selected', multiSelected.value.size > 1 && has(multiSelected.value)],
    ['selected', has(selectedShas.value)],
  ]
  return marks.filter(([, on]) => on).map(([mark]) => mark)
}

type RowMark = keyof typeof MARK_BACKGROUND

// A search match is yellow, kept apart from selection's blue and the
// comparison's own colours.
const MARK_BACKGROUND = {
  'match-current': 'bg-(--p-yellow-500)/45',
  match: 'bg-(--p-yellow-500)/20',
  'compare-to': 'bg-to/15',
  'compare-from': 'bg-from/15',
  'multi-selected': 'bg-emphasis',
  selected: 'bg-highlight',
}

// A compared row also gets a bar at its left edge, drawn as an inset shadow
// so it takes no space, whatever its background.
function rowClasses(sha: string | undefined, index: number): string[] {
  const on = rowMarks(sha, index)
  const background = on[0] ? MARK_BACKGROUND[on[0]] : 'hover:bg-emphasis'
  const bar = on.includes('compare-to')
    ? 'shadow-[inset_3px_0_var(--nb-to)]'
    : on.includes('compare-from')
      ? 'shadow-[inset_3px_0_var(--nb-from)]'
      : ''
  return [...on, background, bar]
}

// One LogRows call per window: from the first visible row not yet cached to
// the last.
watchEffect(() => {
  const missing = visibleRows.value.filter(({ row }) => !row).map(({ item }) => item.index)
  const [first] = missing
  const last = missing.at(-1)
  if (first !== undefined && last !== undefined) void log.rows(first, last + 1)
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
    cherryPick: () => pickWithDefaults(Facade.CherryPick),
    cherryPickOptions: () => openPickDialog('cherry-pick'),
    revert: () => pickWithDefaults(Facade.Revert),
    revertOptions: () => openPickDialog('revert'),
  }
}

// pickWithDefaults runs pick on the selection with git's defaults; a
// refusal shows in the error banner, a stop on conflicts opens the diff
// window.
function pickWithDefaults(pick: typeof Facade.CherryPick): void {
  void runAndClose(
    () => pick(props.wt, shasOf(log.selection), gitDefaults),
    log.dropSelection,
    openOnConflicts(props.wt),
  )
}

function openPickDialog(verb: 'cherry-pick' | 'revert'): void {
  pickDialog.value = { verb, shas: shasOf(log.selection) }
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

// gitDefaults are the plain "Cherry-pick" and "Revert" items' options.
const gitDefaults: PickOptions = { noCommit: false, recordOrigin: false }
</script>

<template>
  <ErrorBanner v-if="error" :error="error" />
  <div
    ref="parentRef"
    class="log-list @container relative flex-1 min-h-0 overflow-auto"
    :style="{
      '--row-height': `${ROW_HEIGHT}px`,
      '--hash-width': hashWidth,
      userSelect: 'none',
    }"
  >
    <div class="relative w-full" :style="{ height: `${virtualizer.getTotalSize()}px` }">
      <div
        v-for="{ item, row } in visibleRows"
        :key="item.index"
        :data-index="item.index"
        class="row absolute top-0 left-0 flex h-(--row-height) w-full cursor-pointer items-center gap-1.5 overflow-hidden px-2 whitespace-nowrap"
        :class="rowClasses(row?.sha, item.index)"
        :style="{ transform: `translateY(${item.start}px)` }"
        tabindex="0"
        @click="onRowClick(row, item.index, $event)"
        @dblclick="emit('open')"
        @keydown.enter.self="emit('open')"
        @contextmenu.prevent="void onContextMenu($event, row, item.index)"
      >
        <template v-if="row">
          <GraphCell v-if="!log.flat" :graph="row.graph" :lanes="graphLanes" :height="ROW_HEIGHT" />
          <!-- The message column takes what the others leave, down to its
               minimum; below that the author gives way first, then the
               date, each shrinking to where it could still show a
               readable value. The row clips what still does not fit, so
               the list never scrolls sideways. -->
          <span class="message flex min-w-[13em] flex-1 items-center gap-1.5 overflow-hidden">
            <!-- The subject takes its whole text first, then the ref
                 labels. At 6.5em it stops shrinking and the labels shrink
                 instead, each down to 2em, ending in an ellipsis. -->
            <span
              class="subject max-w-max min-w-[6.5em] flex-[1000_1_0] truncate"
              :title="row.subject"
              >{{ row.subject }}</span
            >
            <span
              v-for="label in row.refs?.slice(0, MAX_LABELS)"
              :key="label.name"
              class="ref-label min-w-[2em] max-w-[6em] flex-[0_1_auto] truncate rounded-sm bg-emphasis px-1.5 text-sm"
              :class="label.kind"
              :title="label.name"
            >
              {{ label.name }}
            </span>
            <span
              v-if="(row.refs?.length ?? 0) > MAX_LABELS"
              class="ref-more max-w-[6em] flex-none truncate rounded-sm bg-emphasis px-1.5 text-sm"
              :title="row.refs!.map((r) => r.name).join(', ')"
              >+{{ row.refs!.length - MAX_LABELS }}</span
            >
          </span>
          <!-- Narrower than that, shrinking stops being readable at all:
               the column hides instead of shrinking to a sliver, author
               first. -->
          <span
            class="author min-w-0 flex-[0_1000_8em] truncate @max-[480px]:hidden"
            :title="row.author"
            >{{ row.author }}</span
          >
          <span
            class="date text-muted-color min-w-0 flex-[0_1_7em] truncate @max-[360px]:hidden"
            :title="exactDate(row.authorAt)"
            >{{ timeAgo(row.authorAt) }}</span
          >
          <span class="hash text-muted-color w-(--hash-width) flex-none font-mono">{{
            row.shortSha
          }}</span>
        </template>
      </div>
    </div>
  </div>
  <ContextMenu ref="menuRef" :model="menuItems">
    <template #item="{ item, label, props: itemProps }">
      <a v-bind="itemProps.action">
        <span v-bind="itemProps.label">{{ label }}</span>
        <ShortcutHint v-if="item.shortcut" :shortcut="item.shortcut" />
      </a>
    </template>
  </ContextMenu>
  <PickDialog
    v-if="pickDialog"
    :wt="wt"
    :verb="pickDialog.verb"
    :shas="pickDialog.shas"
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
