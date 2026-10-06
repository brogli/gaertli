<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { EditorState, type Extension, Prec } from '@codemirror/state'
import { EditorView, keymap, type ViewUpdate } from '@codemirror/view'
import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
import Button from 'openvue/button'
import Message from 'openvue/message'
import Splitter from 'openvue/splitter'
import SplitterPanel from 'openvue/splitterpanel'
import ToggleButton from 'openvue/togglebutton'
import { useI18n } from '@/i18n'
import { codeEditorExtensions } from '@/lib/codeEditor'
import { useMergeDrafts } from '@/composables/useMergeDrafts'
import { useCoreCall, type CoreError } from '@/composables/useCoreCall'
import { conflictColours } from '@/lib/conflictColours'
import { DEFAULT_MARKER_SIZE, type Take } from '@/lib/conflictMarkers'
import { SIDE_PANES, type Pane } from '@/lib/conflictPanes'
import {
  canTakeRegion,
  hasStrayMarkers,
  nextUnresolved,
  regionCount,
  regionLineRanges,
  regionsExtension,
  regionStart,
  regionStates,
  selectedRegion,
  selectRegion,
  setSideRegionStates,
  sideRegionAt,
  sideRegionStart,
  sideRegionsExtension,
  takeRegion,
  unresolvedCount,
} from '@/lib/conflictRegions'
import { errorHeadline } from '@/lib/errorText'
import { conflictSideTitles } from '@/lib/conflictSides'
import { regionAnchors, tieScrollY, type Anchor } from '@/lib/conflictScroll'
import { iconClass } from '@/lib/icons'
import { ICON_BUTTON, SECONDARY_BUTTON } from '@/lib/layout'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { ConflictFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import ErrorBanner from './ErrorBanner.vue'
import FileStepButtons from './FileStepButtons.vue'

// MergeEditor shows one text conflict in three editors side by side: the
// current side, the editable Result (git's marker blocks), and the incoming
// side, with the base on demand above them. git placed the conflict regions;
// the editors only mark them. The selected region is the one last clicked in
// any pane or holding the Result's cursor.
const props = defineProps<{ wt: string; file: ConflictFile }>()

const emit = defineEmits<{
  'step-file': [step: 1 | -1]
  reload: []
  saved: [fingerprint: string]
}>()

const { t } = useI18n()
const { error, run } = useCoreCall()
const drafts = useMergeDrafts()

// The result may start as unsaved edits left by an earlier view of the file.
const draft = drafts.get(props.wt, props.file.path)
// fingerprint is the working file's state the result is based on: the one
// the file was opened with, then each save's.
const fingerprint = ref(draft?.fingerprint ?? props.file.fingerprint ?? '')
// diskFingerprint is the working file's latest known state.
const diskFingerprint = computed(() => props.file.fingerprint ?? '')
// The draft's text is written when the editor unmounts or a save leaves
// edits; until then the draft only records that edits exist.
const dirty = computed(() => !!drafts.get(props.wt, props.file.path))
const fileChanged = ref<CoreError | null>(null)

const regions = computed(() => props.file.regions ?? [])
const markerSize = computed(() => props.file.markerSize ?? DEFAULT_MARKER_SIZE)

const hosts: Partial<Record<Pane, HTMLElement>> = {}
const views: Partial<Record<Pane, EditorView>> = {}
const showBase = ref(false)

// The result editor's state when its document or selected region last changed.
const resultState = shallowRef<EditorState>()

const status = computed(() => {
  const state = resultState.value
  if (!state)
    return { count: 0, unresolved: 0, selected: -1, selectedTakeable: false, hasMarkers: false }
  const selected = selectedRegion(state)
  return {
    count: regionCount(state),
    unresolved: unresolvedCount(state),
    selected,
    selectedTakeable: selected >= 0 && canTakeRegion(state, selected),
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
    ? t('mergeEditor.noConflictsLeft')
    : t('mergeEditor.conflictPosition', {
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
    action: t('mergeEditor.previousConflict'),
    shortcut: t('shortcuts.previousConflict'),
  }),
)
const nextLabel = computed(() =>
  t('shortcuts.withShortcut', {
    action: t('mergeEditor.nextConflict'),
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

const saveLabel = computed(() =>
  t('shortcuts.withShortcut', {
    action: t('mergeEditor.save'),
    shortcut: t('shortcuts.save'),
  }),
)

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

// Each pane fills its panel and scrolls on its own.
const PANE_LAYOUT = EditorView.theme({
  '&': { height: '100%' },
  '.cm-scroller': { overflow: 'auto' },
})

// Alt+Up and Alt+Down step between conflicts in every pane, ahead of the
// result's move-line keys; Ctrl+S saves from any pane.
const CONFLICT_KEYS = Prec.highest(
  keymap.of([
    { key: 'Alt-ArrowUp', run: () => (goToRegion(-1), true) },
    { key: 'Alt-ArrowDown', run: () => (goToRegion(1), true) },
    { key: 'Mod-s', run: () => (void save(false), true) },
  ]),
)

// storeDraft keeps the result's text as the file's draft.
function storeDraft(state: EditorState): void {
  drafts.set(props.wt, props.file.path, {
    text: state.doc.toString(),
    fingerprint: fingerprint.value,
    regions: regionLineRanges(state),
  })
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

// anchors align the panes by conflict region; the result's regions follow its
// edits.
let anchors: Anchor[] = []

function updateAnchors(state: EditorState): void {
  const resultRanges = regionLineRanges(state)
  const aligned = regions.value.map((region, i) => ({
    ...region,
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
  if (!update.docChanged && !selectionMoved) return
  resultState.value = update.state
  syncSidePanes(update.state)
  if (!update.docChanged) return
  updateAnchors(update.state)
  if (!dirty.value) storeDraft(update.state)
}

// selectClickedRegion selects the region a side pane's user selection
// landed in.
function selectClickedRegion(update: ViewUpdate): void {
  const byUser = update.transactions.some((tr) => tr.isUserEvent('select'))
  if (!update.selectionSet || !byUser) return
  const region = sideRegionAt(update.state, update.state.selection.main.head)
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
            regions.value.map((region) => region[pane]),
            pane,
          ),
          EditorView.updateListener.of(selectClickedRegion),
        ],
      }),
    })
  }

  const resultView = new EditorView({
    parent: hostOf('result'),
    state: EditorState.create({
      doc: draft?.text ?? file.result ?? '',
      extensions: [
        ...paneExtensions,
        history(),
        keymap.of([...defaultKeymap, ...historyKeymap]),
        regionsExtension(
          draft?.regions ?? regions.value.map((region) => region.result),
          markerSize.value,
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

async function save(markResolved: boolean): Promise<void> {
  const text = views.result?.state.doc.toString() ?? ''
  fileChanged.value = null
  const saved = await run(
    () => Facade.SaveConflict(props.wt, props.file.path, text, fingerprint.value, markResolved),
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
    drafts.discard(props.wt, props.file.path)
    return
  }
  storeDraft(result.state)
}

function take(kind: Take): void {
  const result = views.result
  if (result) result.dispatch(takeRegion(result.state, selectedRegion(result.state), kind))
}

let untieScroll: (() => void) | undefined

// tieScroll keeps the shown panes aligned by conflict region.
function tieScroll(): void {
  untieScroll?.()
  const shown = Object.entries(views).filter(([pane]) => showBase.value || pane !== 'base')
  untieScroll = tieScrollY(Object.fromEntries(shown), () => anchors)
}

let unmounted = false

onMounted(async () => {
  const shared = await codeEditorExtensions(props.file.path)
  if (unmounted) return
  createViews(shared)
  tieScroll()
})

watch(showBase, tieScroll)

onBeforeUnmount(() => {
  unmounted = true
  untieScroll?.()
  if (dirty.value && views.result) storeDraft(views.result.state)
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
        :disabled="!status.selectedTakeable"
        :data-test="`take-${kind}`"
        @click="take(kind)"
      />
      <Button
        v-bind="SECONDARY_BUTTON"
        class="ml-auto"
        :label="t('mergeEditor.save')"
        :title="saveLabel"
        data-test="save"
        @click="save(false)"
      />
      <Button
        v-bind="SECONDARY_BUTTON"
        :label="t('mergeEditor.saveAndResolve')"
        :title="resolveBlocker"
        :disabled="status.hasMarkers"
        data-test="save-resolve"
        @click="save(true)"
      />
      <ToggleButton
        v-if="file.base.present"
        v-model="showBase"
        :on-label="t('mergeEditor.hideBase')"
        :off-label="t('mergeEditor.showBase')"
        size="small"
        data-test="show-base"
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
    <ErrorBanner v-else-if="error" :error="error" />
    <Splitter
      layout="vertical"
      class="min-h-0 flex-1"
      :pt="{ gutter: { class: showBase ? undefined : 'hidden' } }"
    >
      <SplitterPanel
        :size="showBase ? 30 : 0"
        :min-size="showBase ? 10 : 0"
        :inert="!showBase"
        class="flex min-h-0 flex-col overflow-hidden"
      >
        <div class="truncate px-2 py-0.5 font-semibold" data-test="pane-header-base">
          {{ t('mergeEditor.base') }}
        </div>
        <div :ref="hostRef('base')" class="min-h-0 flex-1" />
      </SplitterPanel>
      <SplitterPanel :size="showBase ? 70 : 100" :min-size="20" class="min-h-0">
        <Splitter
          layout="horizontal"
          class="h-full"
          state-key="nebegit.layout.mergePanes"
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
      </SplitterPanel>
    </Splitter>
  </div>
</template>
