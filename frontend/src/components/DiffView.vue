<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { EditorState, type Text } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import {
  type Chunk,
  getChunks,
  getOriginalDoc,
  goToNextChunk,
  goToPreviousChunk,
  MergeView,
  unifiedMergeView,
} from '@codemirror/merge'
import { useDiffStore } from '../stores/diff'
import { useSettingsStore } from '../stores/settings'
import { useLogStore } from '../stores/log'
import { useChangesStore } from '../stores/changes'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import ErrorBanner from './ErrorBanner.vue'
import type { ChangedFile, LineSelection } from '../../bindings/github.com/brogli/nebegit/internal/core'

// DiffView renders a file's two-sided content: the log store's selected
// file at its selected commit, a compare's file between two commits when
// range is given, or a working-tree change when working is given.
// CodeMirror computes the diff from the fetched old/new content. Read-only;
// unified/side-by-side is Settings.DiffMode.
//
// files is the compare window's ordered file list, used only with range;
// stepping through it emits update:range.
type DiffRange = { from: string; to: string; path: string; oldPath: string }
type WorkingTarget = { path: string; oldPath: string; staged: boolean; status: string }

const props = defineProps<{
  wt: string
  range?: DiffRange | null
  working?: WorkingTarget | null
  files?: ChangedFile[]
}>()
const emit = defineEmits<{ 'update:range': [range: DiffRange] }>()

const diff = useDiffStore()
const settings = useSettingsStore()
const log = useLogStore()
const changes = useChangesStore()
const { error, run, runAndClose } = useCoreCall()

const container = ref<HTMLElement | null>(null)
let view: EditorView | MergeView | null = null

// hunks and selectedLinesEnabled mirror CodeMirror state, which lives
// outside Vue's reactivity: hunks is the mounted view's chunks (one
// Stage/Unstage hunk button each), refreshed by mountView;
// selectedLinesEnabled is selectedLines() !== null, refreshed on every
// selection change.
const hunks = ref<readonly Chunk[]>([])
const selectedLinesEnabled = ref(false)

type Target =
  | { kind: 'commit'; sha: string; path: string; oldPath: string }
  | { kind: 'range'; from: string; to: string; path: string; oldPath: string }
  | ({ kind: 'working' } & WorkingTarget)

const target = computed<Target | null>(() => {
  if (props.range) return { kind: 'range', ...props.range }
  if (props.working) return { kind: 'working', ...props.working }
  const file = log.selectedFile
  const sha = log.details?.sha
  return file && sha ? { kind: 'commit', sha, path: file.path, oldPath: file.oldPath ?? '' } : null
})

function load(force = false): void {
  const t = target.value
  if (!t) return
  if (t.kind === 'range') {
    void run(() => diff.loadRange(props.wt, t.from, t.to, t.path, t.oldPath, force))
  } else if (t.kind === 'working') {
    void run(() => diff.loadWorking(props.wt, t.path, t.oldPath, t.staged, force))
  } else {
    void run(() => diff.load(props.wt, t.sha, t.path, t.oldPath, force))
  }
}

watch(target, () => load(), { immediate: true })

// openEditor opens the file shown, at no particular line.
function openEditor(): void {
  const t = target.value
  if (t) void run(() => Facade.OpenFile(props.wt, t.path, 0))
}

const mode = computed(() => settings.settings?.diffMode ?? 'unified')
if (!settings.settings) void settings.load()

function toggleMode(): void {
  const next = mode.value === 'unified' ? 'side-by-side' : 'unified'
  if (settings.settings) void settings.save({ ...settings.settings, diffMode: next })
}

function destroyView(): void {
  view?.destroy()
  view = null
  hunks.value = []
  selectedLinesEnabled.value = false
}

function updateSelectedLinesEnabled(): void {
  selectedLinesEnabled.value = selectedLines() !== null
}

function mountView(): void {
  destroyView()
  const file = diff.file
  const el = container.value
  if (!el || !file || file.binary || file.tooLarge) return

  const readOnly = EditorState.readOnly.of(true)
  view =
    mode.value === 'unified'
      ? new EditorView({
          parent: el,
          state: EditorState.create({
            doc: file.newContent ?? '',
            extensions: [
              readOnly,
              unifiedMergeView({ original: file.oldContent ?? '', mergeControls: false }),
              EditorView.updateListener.of((update) => {
                if (update.selectionSet) updateSelectedLinesEnabled()
              }),
            ],
          }),
        })
      : new MergeView({
          parent: el,
          a: { doc: file.oldContent ?? '', extensions: [readOnly] },
          b: { doc: file.newContent ?? '', extensions: [readOnly] },
        })
  hunks.value = view instanceof MergeView ? view.chunks : (getChunks(view.state)?.chunks ?? [])
  updateSelectedLinesEnabled()
}

// flush: 'post' so the target branch's DOM (the .cm-container div) has
// already rendered by the time mountView reads the template ref.
watch([() => diff.file, mode], mountView, { flush: 'post' })

onBeforeUnmount(destroyView)

// Next/previous change run CodeMirror's chunk commands against the unified
// view's editor, or the side-by-side view's b (new/right) editor.
function changeTarget(): EditorView | null {
  if (view instanceof MergeView) return view.b
  return view
}

function nextChange(): void {
  const target = changeTarget()
  if (target) goToNextChunk(target)
}

function prevChange(): void {
  const target = changeTarget()
  if (target) goToPreviousChunk(target)
}

// stageLines stages or unstages sel, whichever matches the side shown
// (working.staged). On success the changes panel and this diff reload at
// once rather than after the watcher's debounce.
function stageLines(sel: LineSelection | null): void {
  if (!sel || !props.working) return
  const { path, staged } = props.working
  void runAndClose(
    () => Facade.StageLines(props.wt, path, sel, staged),
    () => {
      void changes.load(props.wt)
      load(true)
    },
  )
}

// lineRange returns the inclusive 1-based line numbers from first to last.
function lineRange(first: number, last: number): number[] {
  const lines: number[] = []
  for (let n = first; n <= last; n++) lines.push(n)
  return lines
}

// linesInRange returns the 1-based line numbers doc's [from, to] range
// covers, or [] if the range is empty (a pure insert has no old-side
// lines, a pure delete no new-side lines).
function linesInRange(doc: Text, from: number, to: number): number[] {
  return from === to ? [] : lineRange(doc.lineAt(from).number, doc.lineAt(to).number)
}

// chunkDocs returns the mounted view's old/new documents: a MergeView's
// a/b editors, or a unified view's document and its getOriginalDoc.
function chunkDocs(): { old: Text; new: Text } | null {
  if (!view) return null
  if (view instanceof MergeView) return { old: view.a.state.doc, new: view.b.state.doc }
  return { old: getOriginalDoc(view.state), new: view.state.doc }
}

// canStagePartially gates the hunk/line controls to a plain modification.
// Untracked files have no diff to re-diff; git refuses a partial patch for
// an added or deleted file; a rename's file header would need rewriting.
// Those are staged whole from the changes panel.
function canStagePartially(status: string): boolean {
  return status === 'M'
}

// hunkSelection names every changed line of chunk, by its line number on
// each diff side. endA/endB, not toA/toB: toA/toB may point past the
// chunk's last line, which lineAt would misread.
function hunkSelection(chunk: Chunk): LineSelection | null {
  const docs = chunkDocs()
  if (!docs) return null
  return {
    old: linesInRange(docs.old, chunk.fromA, chunk.endA),
    new: linesInRange(docs.new, chunk.fromB, chunk.endB),
  }
}

// selectedLines names the text selection's line numbers, or null if the
// selection isn't entirely inside one chunk. Unified mode only. The
// unified document is the new content (deleted lines are decorations), so
// this selects new-side lines only.
function selectedLines(): LineSelection | null {
  if (!(view instanceof EditorView)) return null
  const sel = view.state.selection.main
  // toB, not endB: a selection through a chunk's whole last line extends
  // past endB, up to the next line's start.
  if (!hunks.value.some((c) => sel.from >= c.fromB && sel.to <= c.toB)) return null

  const doc = view.state.doc
  const selStartLine = doc.lineAt(sel.from).number
  const selEndLine = doc.lineAt(Math.max(sel.to - 1, sel.from)).number
  return { old: [], new: lineRange(selStartLine, selEndLine) }
}

// stepRangeFile steps through files, a no-op past either end like the log
// store's nextFile/prevFile.
function stepRangeFile(step: 1 | -1): void {
  if (!props.range || !props.files) return
  const i = props.files.findIndex(
    (f) => f.path === props.range!.path && (f.oldPath ?? '') === props.range!.oldPath,
  )
  const next = props.files[i + step]
  if (i >= 0 && next) {
    emit('update:range', { from: props.range.from, to: props.range.to, path: next.path, oldPath: next.oldPath ?? '' })
  }
}
</script>

<template>
  <section class="diff-view">
    <ErrorBanner v-if="error" :error="error" />
    <p v-else-if="diff.loading">Loading…</p>
    <template v-else-if="diff.file?.binary">
      <p>Binary file, no preview available.</p>
    </template>
    <template v-else-if="diff.file?.tooLarge">
      <p>File is too large to show automatically.</p>
      <button type="button" @click="load(true)">Load anyway</button>
    </template>
    <template v-else-if="diff.file">
      <button type="button" data-test="mode-toggle" @click="toggleMode">
        {{ mode === 'unified' ? 'Side-by-side' : 'Unified' }}
      </button>
      <button type="button" data-test="prev-change" @click="prevChange">Previous change</button>
      <button type="button" data-test="next-change" @click="nextChange">Next change</button>
      <template v-if="range && files">
        <button type="button" data-test="prev-file" @click="stepRangeFile(-1)">Previous file</button>
        <button type="button" data-test="next-file" @click="stepRangeFile(1)">Next file</button>
      </template>
      <template v-else-if="!range && !working">
        <button type="button" data-test="prev-file" @click="log.prevFile()">Previous file</button>
        <button type="button" data-test="next-file" @click="log.nextFile()">Next file</button>
      </template>
      <button type="button" data-test="open-editor" @click="openEditor">Open in editor</button>
      <div v-if="working && canStagePartially(working.status)" class="hunk-actions">
        <button
          v-for="(chunk, i) in hunks"
          :key="i"
          type="button"
          data-test="stage-hunk"
          @click="stageLines(hunkSelection(chunk))"
        >
          {{ working.staged ? 'Unstage hunk' : 'Stage hunk' }}
        </button>
        <button
          type="button"
          data-test="stage-selected-lines"
          :disabled="!selectedLinesEnabled"
          @click="stageLines(selectedLines())"
        >
          {{ working.staged ? 'Unstage selected lines' : 'Stage selected lines' }}
        </button>
      </div>
      <div ref="container" class="cm-container"></div>
    </template>
  </section>
</template>
