<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import { EditorState, type Extension, Prec } from '@codemirror/state'
import { EditorView, keymap, type ViewUpdate } from '@codemirror/view'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import Button from 'openvue/button'
import Message from 'openvue/message'
import Splitter from 'openvue/splitter'
import SplitterPanel from 'openvue/splitterpanel'
import { useI18n } from '@/i18n'
import { codeEditorExtensions, PANE_LAYOUT } from '@/lib/codeEditor'
import { useConflictsStore } from '@/stores/conflicts'
import { useCoreCall, type CoreError } from '@/composables/useCoreCall'
import { conflictColours } from '@/lib/conflictColours'
import { DEFAULT_MARKER_SIZE } from '@/lib/conflictMarkers'
import { resultFromBase, takenLines, type Take } from '@/lib/conflictText'
import { SIDE_PANES, type Pane } from '@/lib/conflictPanes'
import {
  hasStrayMarkers,
  isUnresolved,
  markDone,
  nextUnresolved,
  pickSideRegion,
  regionCount,
  regionsExtension,
  regionTint,
  regionStart,
  regionStates,
  resultRegions,
  selectedRegion,
  selectRegion,
  setSideRegionStates,
  sideRegionAt,
  sideRegionStart,
  sideRegionsExtension,
  takeRegion,
  takeRegions,
  unresolvedCount,
} from '@/lib/conflictRegions'
import { errorHeadline } from '@/lib/errorText'
import { conflictSideTitles } from '@/lib/conflictSides'
import { regionAnchors, tieScrollY, type Anchor } from '@/lib/conflictScroll'
import { iconClass } from '@/lib/icons'
import { ICON_BUTTON, SECONDARY_BUTTON } from '@/lib/layout'
import { Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'
import {
  MergeChangeKind,
  type ConflictFile,
} from '../../bindings/github.com/brogli/gaertli/internal/core'
import ErrorBanner from './ErrorBanner.vue'
import FileStepButtons from './FileStepButtons.vue'

// MergeEditor shows one text conflict in three editors side by side: the
// current side, the editable Result and the incoming side. The Result's
// regions, the conflicts and the changes only one side made, start as the base
// text. git placed the regions and the changes of each side; the editors only
// mark them. The selected region is the one last clicked in any pane or
// holding the Result's cursor.
const props = defineProps<{ wt: string; file: ConflictFile }>()

const emit = defineEmits<{
  'step-file': [step: 1 | -1]
  reload: []
  saved: [fingerprint: string]
}>()

const { t } = useI18n()
const { error, run } = useCoreCall()
const conflicts = useConflictsStore()

// conflictId is the conflict the editor shows; a later one in the path
// mounts another editor.
const conflictId = props.file.conflictId ?? ''
// The result may start as unsaved edits left by an earlier view of the file.
const draft = conflicts.get(props.wt, props.file.path, conflictId)
const initial = resultFromBase(props.file)
// fingerprint is the working file's state the result is based on: the one
// the file was opened with, then each save's.
const fingerprint = ref(draft?.fingerprint ?? props.file.fingerprint ?? '')
// diskFingerprint is the working file's latest known state.
const diskFingerprint = computed(() => props.file.fingerprint ?? '')
const dirty = computed(() => conflicts.has(props.wt, props.file.path, conflictId))
const fileChanged = ref<CoreError | null>(null)
// shownError is the last core call's failure, else the draft's failed write.
const shownError = computed(() => error.value ?? conflicts.writeError(props.wt, props.file.path))

const changes = computed(() => props.file.changes ?? [])
const markerSize = computed(() => props.file.markerSize ?? DEFAULT_MARKER_SIZE)

const hosts: Partial<Record<Pane, HTMLElement>> = {}
const views: Partial<Record<Pane, EditorView>> = {}

// The result editor's state when its document or selected region last changed.
const resultState = shallowRef<EditorState>()

const status = computed(() => {
  const state = resultState.value
  if (!state)
    return { count: 0, unresolved: 0, selected: -1, selectedUnresolved: false, hasMarkers: false }
  const selected = selectedRegion(state)
  return {
    count: regionCount(state),
    unresolved: unresolvedCount(state),
    selected,
    selectedUnresolved: selected >= 0 && isUnresolved(state, selected),
    hasMarkers: hasStrayMarkers(state),
  }
})

const titles = computed(() =>
  conflictSideTitles(props.file.operation, props.file.current, props.file.incoming, t),
)

const panels = computed(
  () =>
    [
      { pane: 'current', title: titles.value.current, size: 33 },
      { pane: 'result', title: t('mergeEditor.result'), size: 34 },
      { pane: 'incoming', title: titles.value.incoming, size: 33 },
    ] as const,
)

const position = computed(() =>
  status.value.unresolved === 0
    ? t('mergeEditor.noChangesLeft')
    : t('mergeEditor.changePosition', {
        n: Math.max(status.value.selected, 0) + 1,
        total: status.value.count,
        left: status.value.unresolved,
      }),
)

// resolveBlocker says why the file cannot be marked resolved yet.
const resolveBlocker = computed(() => {
  if (status.value.unresolved > 0) return t('mergeEditor.resolveFirst')
  return status.value.hasMarkers ? t('mergeEditor.strayMarker') : undefined
})

const previousLabel = computed(() =>
  t('shortcuts.withShortcut', {
    action: t('mergeEditor.previousChange'),
    shortcut: t('shortcuts.previousConflict'),
  }),
)
const nextLabel = computed(() =>
  t('shortcuts.withShortcut', {
    action: t('mergeEditor.nextChange'),
    shortcut: t('shortcuts.nextConflict'),
  }),
)

// changedMessage says the file changed under the result: refused by a save,
// or seen on a reload while the result holds unsaved edits.
const changedMessage = computed(() => {
  if (fileChanged.value) return errorHeadline(fileChanged.value)
  const changed = dirty.value && fingerprint.value !== diskFingerprint.value
  return changed ? t('mergeEditor.changedOnDisk', { path: props.file.path }) : ''
})

const saveTitle = computed(
  () =>
    resolveBlocker.value ??
    t('shortcuts.withShortcut', {
      action: t('mergeEditor.saveAndResolve'),
      shortcut: t('shortcuts.save'),
    }),
)

// openOneSided are the unresolved changes only one side made, or both alike.
const openOneSided = computed(() => {
  const state = resultState.value
  if (!state) return []
  return changes.value.flatMap((change, i) =>
    change.kind !== MergeChangeKind.ConflictChange &&
    i < regionCount(state) &&
    isUnresolved(state, i)
      ? [i]
      : [],
  )
})

const takes = computed<{ take: Take; label: string }[]>(() => [
  { take: 'current', label: t('mergeEditor.takeCurrent') },
  { take: 'incoming', label: t('mergeEditor.takeIncoming') },
  { take: 'currentFirst', label: t('mergeEditor.takeCurrentFirst') },
  { take: 'incomingFirst', label: t('mergeEditor.takeIncomingFirst') },
])

// hostRef collects the element a pane's editor is created in.
function hostRef(pane: Pane) {
  return (el: unknown) => {
    if (el instanceof HTMLElement) hosts[pane] = el
  }
}

function hostOf(pane: Pane): HTMLElement {
  const host = hosts[pane]
  if (!host) throw new Error(`no host for the ${pane} pane`)
  return host
}

// Alt+Up and Alt+Down step between conflicts in every pane, ahead of the
// result's move-line keys; Ctrl+S saves from any pane.
const CONFLICT_KEYS = Prec.highest(
  keymap.of([
    { key: 'Alt-ArrowUp', run: () => (goToRegion(-1), true) },
    { key: 'Alt-ArrowDown', run: () => (goToRegion(1), true) },
    { key: 'Mod-s', run: () => (void save(), true) },
  ]),
)

// storeDraft keeps the result as the file's draft, read from result as it
// is when the draft is written.
function storeDraft(result: EditorView): void {
  conflicts.set(props.wt, props.file.path, conflictId, () => ({
    text: result.state.doc.toString(),
    fingerprint: fingerprint.value,
    changes: resultRegions(result.state),
  }))
}

// sentSideStates is what the side panes were last told of the result's regions.
let sentSideStates = ''

// syncSidePanes shows the result's resolved and selected regions in the side
// panes, when they changed.
function syncSidePanes(state: EditorState): void {
  const states = regionStates(state)
  const key = JSON.stringify(states)
  if (key === sentSideStates) return
  sentSideStates = key
  for (const pane of SIDE_PANES) {
    views[pane]?.dispatch({ effects: setSideRegionStates.of(states) })
  }
}

// anchors align the panes by change; the result's regions follow its
// edits.
let anchors: Anchor[] = []

function updateAnchors(state: EditorState): void {
  const resultRanges = resultRegions(state).map(({ range }) => range)
  const aligned = changes.value.map((change, i) => ({
    ...change,
    ...(resultRanges[i] && { result: resultRanges[i] }),
  }))
  const lineCounts = Object.fromEntries(
    SIDE_PANES.map((pane) => [pane, views[pane]?.state.doc.lines]),
  )
  anchors = regionAnchors(aligned, { ...lineCounts, result: state.doc.lines })
}

// onResultUpdate keeps the status and the side panes current and notes that
// the result has unsaved edits.
function onResultUpdate(update: ViewUpdate): void {
  const selectionMoved = selectedRegion(update.state) !== selectedRegion(update.startState)
  const marked = unresolvedCount(update.state) !== unresolvedCount(update.startState)
  if (!update.docChanged && !selectionMoved && !marked) return
  resultState.value = update.state
  syncSidePanes(update.state)
  if (update.docChanged) updateAnchors(update.state)
  if (update.docChanged || marked) storeDraft(update.view)
}

// selectClickedRegion selects the region a side pane's user selection
// landed in.
function selectClickedRegion(update: ViewUpdate): void {
  const picked = update.transactions
    .flatMap((tr) => tr.effects)
    .find((effect) => effect.is(pickSideRegion))
  const byUser = update.transactions.some((tr) => tr.isUserEvent('select'))
  const clicked =
    update.selectionSet && byUser
      ? sideRegionAt(update.state, update.state.selection.main.head)
      : -1
  const region = picked?.value ?? clicked
  const result = views.result
  if (result && region >= 0 && region < regionCount(result.state))
    result.dispatch({ effects: selectRegion.of(region) })
}

function createViews(shared: Extension[]): void {
  const { file } = props
  const paneExtensions = [...shared, PANE_LAYOUT, conflictColours, CONFLICT_KEYS]
  for (const pane of SIDE_PANES) {
    views[pane] = new EditorView({
      parent: hostOf(pane),
      state: EditorState.create({
        doc: file[pane].content ?? '',
        extensions: [
          ...paneExtensions,
          EditorState.readOnly.of(true),
          sideRegionsExtension(
            changes.value.map((change) => change[pane]),
            (pane === 'current' ? file.currentChanges : file.incomingChanges) ?? [],
            t('mergeEditor.emptyChange'),
            changes.value.map((change) => regionTint(change, pane)),
          ),
          EditorView.updateListener.of(selectClickedRegion),
        ],
      }),
    })
  }

  const resultView = new EditorView({
    parent: hostOf('result'),
    state: EditorState.create({
      doc: draft?.text ?? initial.text,
      extensions: [
        ...paneExtensions,
        history(),
        keymap.of([...defaultKeymap, ...historyKeymap]),
        regionsExtension(
          draft?.changes ?? initial.changes.map((range) => ({ range, resolved: false })),
          markerSize.value,
          t('mergeEditor.emptyChange'),
          changes.value.map((change) => regionTint(change, 'result')),
        ),
        EditorView.updateListener.of(onResultUpdate),
      ],
    }),
  })
  views.result = resultView
  resultView.dispatch({ effects: selectRegion.of(nextUnresolved(resultView.state, -1, 1)) })
  resultState.value = resultView.state
  syncSidePanes(resultView.state)
  updateAnchors(resultView.state)
}

// goToRegion selects the neighbouring unresolved region, puts the result's
// cursor on its first line and scrolls it to the middle of every pane.
function goToRegion(dir: 1 | -1): void {
  const result = views.result
  if (!result) return
  const target = nextUnresolved(result.state, selectedRegion(result.state), dir)
  if (target < 0) return
  const centre = (pos: number) => EditorView.scrollIntoView(pos, { y: 'center' })
  const start = regionStart(result.state, target)
  result.dispatch({
    selection: { anchor: start },
    effects: [selectRegion.of(target), centre(start)],
  })
  for (const pane of SIDE_PANES) {
    const view = views[pane]
    const from = view && sideRegionStart(view.state, target)
    if (view && from !== undefined) view.dispatch({ effects: centre(from) })
  }
}

function takeFileChanged(err: CoreError): boolean {
  if (err.key !== 'fileChangedSinceOpened') return false
  fileChanged.value = err
  return true
}

async function save(): Promise<void> {
  if (resolveBlocker.value) return
  const text = views.result?.state.doc.toString() ?? ''
  fileChanged.value = null
  const saved = await run(
    () => Facade.SaveConflict(props.wt, props.file.path, text, fingerprint.value, true),
    takeFileChanged,
  )
  if (saved === undefined) return
  fingerprint.value = saved
  keepEditsSince(text)
  emit('saved', saved)
}

// keepEditsSince drops the draft when the result still reads as saved; text
// typed during the save stays a draft on the saved file.
function keepEditsSince(saved: string): void {
  const result = views.result
  if (!result || result.state.doc.toString() === saved) {
    void conflicts.discard(props.wt, props.file.path)
    return
  }
  storeDraft(result)
}

function take(kind: Take): void {
  const result = views.result
  if (!result) return
  const region = selectedRegion(result.state)
  result.dispatch(takeRegion(result.state, region, takenLines(props.file, region, kind)))
}

// applyNonConflicting takes the changed side of every open change only one
// side made, as one undo step.
function applyNonConflicting(): void {
  const result = views.result
  if (!result) return
  const takes = openOneSided.value.map((region) => {
    const incoming = changes.value[region]?.kind === MergeChangeKind.IncomingOnlyChange
    return { region, lines: takenLines(props.file, region, incoming ? 'incoming' : 'current') }
  })
  result.dispatch(takeRegions(result.state, takes))
}

function markSelectedDone(): void {
  views.result?.dispatch(markDone(status.value.selected))
}

let untieScroll: (() => void) | undefined

// tieScroll keeps the shown panes aligned by change.
function tieScroll(): void {
  untieScroll?.()
  untieScroll = tieScrollY(views, () => anchors)
}

let unmounted = false

onMounted(async () => {
  const shared = await codeEditorExtensions(props.file.path)
  if (unmounted) return
  createViews(shared)
  tieScroll()
})

onBeforeUnmount(() => {
  unmounted = true
  untieScroll?.()
  conflicts.flush(props.wt, props.file.path)
  Object.values(views).forEach((view) => view.destroy())
})
</script>

<template>
  <div class="merge-editor flex h-full min-h-0 flex-col gap-2">
    <div class="merge-toolbar flex flex-wrap items-center gap-1">
      <FileStepButtons @step="emit('step-file', $event)" />
      <span class="mx-1 h-4 border-l border-(--p-content-border-color)" />
      <Button
        v-bind="ICON_BUTTON"
        :icon="iconClass('prevChange')"
        :title="previousLabel"
        :aria-label="previousLabel"
        data-test="prev-conflict"
        @click="goToRegion(-1)"
      />
      <Button
        v-bind="ICON_BUTTON"
        :icon="iconClass('nextChange')"
        :title="nextLabel"
        :aria-label="nextLabel"
        data-test="next-conflict"
        @click="goToRegion(1)"
      />
      <span class="px-1" data-test="conflict-position">{{ position }}</span>
      <Button
        v-for="{ take: kind, label } in takes"
        :key="kind"
        v-bind="SECONDARY_BUTTON"
        :label="label"
        :disabled="status.selected < 0"
        :data-test="`take-${kind}`"
        @click="take(kind)"
      />
      <Button
        v-bind="SECONDARY_BUTTON"
        :label="t('mergeEditor.markDone')"
        :disabled="!status.selectedUnresolved"
        data-test="mark-done"
        @click="markSelectedDone"
      />
      <Button
        v-bind="SECONDARY_BUTTON"
        :label="t('mergeEditor.applyNonConflicting')"
        :disabled="openOneSided.length === 0"
        data-test="apply-non-conflicting"
        @click="applyNonConflicting"
      />
      <Button
        v-bind="SECONDARY_BUTTON"
        class="ml-auto"
        :label="t('mergeEditor.saveAndResolve')"
        :title="saveTitle"
        :disabled="!!resolveBlocker"
        data-test="save-resolve"
        @click="save"
      />
    </div>
    <Message v-if="changedMessage" severity="warn" class="save-conflict">
      <div class="flex flex-wrap items-center gap-2">
        <p>{{ changedMessage }}</p>
        <Button
          :label="t('mergeEditor.reload')"
          size="small"
          data-test="reload"
          @click="emit('reload')"
        />
      </div>
    </Message>
    <ErrorBanner v-else-if="shownError" :error="shownError" />
    <Splitter
      layout="horizontal"
      class="min-h-0 flex-1"
      state-key="gaertli.layout.mergePanes"
      state-storage="local"
    >
      <SplitterPanel
        v-for="{ pane, title, size } in panels"
        :key="pane"
        :size="size"
        :min-size="10"
        class="flex min-h-0 flex-col"
      >
        <div class="truncate px-2 py-0.5 font-semibold" :data-test="`pane-header-${pane}`">
          {{ title }}
        </div>
        <div :ref="hostRef(pane)" class="min-h-0 flex-1" />
      </SplitterPanel>
    </Splitter>
  </div>
</template>
