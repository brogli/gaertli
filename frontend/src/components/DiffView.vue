<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { EditorState, type Extension, type Text } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import {
  Change,
  type Chunk,
  diff as diffText, // aliased: the diff store below is already named diff
  type DiffConfig,
  getChunks,
  getOriginalDoc,
  goToNextChunk,
  goToPreviousChunk,
  MergeView,
  unifiedMergeView,
} from '@codemirror/merge'
import { LanguageDescription, syntaxHighlighting } from '@codemirror/language'
import { languages } from '@codemirror/language-data'
import { classHighlighter } from '@lezer/highlight'
import { useDiffStore } from '../stores/diff'
import { useSettingsStore } from '../stores/settings'
import { useLogStore } from '../stores/log'
import { useChangesStore } from '../stores/changes'
import { useCoreCall } from '../composables/useCoreCall'
import { useConfirmAction } from '../composables/useConfirmAction'
import { basename } from '../lib/path'
import { Events } from '@wailsio/runtime'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import ErrorBanner from './ErrorBanner.vue'
import type {
  ChangedFile,
  EditorFallbackKind,
  LineSelection,
} from '../../bindings/github.com/brogli/nebegit/internal/core'

// DiffView shows one file's diff, read-only: the log's selected file at its
// commit, a file between two commits (range, with files the compare list it
// steps through), a commit against the working tree (workingCompare), or an
// uncommitted change (working). CodeMirror diffs the fetched old/new content.
type DiffRange = { from: string; to: string; path: string; oldPath: string }
type WorkingTarget = { path: string; oldPath: string; staged: boolean; status: string }
type CompareWorkingTarget = { sha: string; path: string; oldPath: string }

const props = defineProps<{
  wt: string
  range?: DiffRange | null
  workingCompare?: CompareWorkingTarget | null
  working?: WorkingTarget | null
  files?: ChangedFile[]
}>()
const emit = defineEmits<{ 'update:range': [range: DiffRange] }>()

const diff = useDiffStore()
const settings = useSettingsStore()
const log = useLogStore()
const changes = useChangesStore()
const { error, run, runAndClose } = useCoreCall()
const { confirmAction } = useConfirmAction()

const container = ref<HTMLElement | null>(null)
let view: EditorView | MergeView | null = null

// Reactive copies of CodeMirror state: the mounted view's chunks, and
// whether the selection lies inside one.
const hunks = ref<readonly Chunk[]>([])
const selectedLinesEnabled = ref(false)

type Target =
  | { kind: 'commit'; sha: string; path: string; oldPath: string }
  | { kind: 'range'; from: string; to: string; path: string; oldPath: string }
  | { kind: 'againstWorking'; sha: string; path: string; oldPath: string }
  | ({ kind: 'working' } & WorkingTarget)

const target = computed<Target | null>(() => {
  if (props.range) return { kind: 'range', ...props.range }
  if (props.workingCompare) return { kind: 'againstWorking', ...props.workingCompare }
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
  } else if (t.kind === 'againstWorking') {
    void run(() => diff.loadAgainstWorking(props.wt, t.sha, t.path, t.oldPath, force))
  } else if (t.kind === 'working') {
    void run(() => diff.loadWorking(props.wt, t.path, t.oldPath, t.staged, force))
  } else {
    void run(() => diff.load(props.wt, t.sha, t.path, t.oldPath, force))
  }
}

watch(target, () => load(), { immediate: true })

// repo:changed{scope:"index"} fires on any change to this worktree's files
// or index.
const unlistenChanged = Events.On('repo:changed', (ev) => {
  const { scope } = ev.data as { scope: string }
  const kind = target.value?.kind
  if (scope === 'index' && (kind === 'working' || kind === 'againstWorking')) void run(() => diff.refetch())
})
onBeforeUnmount(unlistenChanged)

// currentEditorTarget is the file shown and the cursor's line on the new
// side; line 0 when no editor is mounted (a binary or too-large file).
function currentEditorTarget(): { path: string; line: number } | null {
  const t = target.value
  if (!t) return null
  const cm = changeTarget()
  const line = cm ? cm.state.doc.lineAt(cm.state.selection.main.head).number : 0
  return { path: t.path, line }
}

function openEditor(): void {
  const editorTarget = currentEditorTarget()
  if (!editorTarget) return
  void run(() => Facade.OpenFile(props.wt, editorTarget.path, editorTarget.line))
}

// openEditorFallback retries after OpenFile returned EDITOR_NOT_FOUND.
function openEditorFallback(kind: EditorFallbackKind): void {
  const editorTarget = currentEditorTarget()
  if (!editorTarget) return
  void run(() => Facade.OpenFileFallback(props.wt, editorTarget.path, editorTarget.line, kind))
}

// A binary or too-large file shows only its placeholder.
const showsContent = computed(() => !!diff.file && !diff.file.binary && !diff.file.tooLarge)

const mode = computed(() => settings.settings?.diffMode ?? 'unified')
if (!settings.settings) void settings.load()

function toggleMode(): void {
  const next = mode.value === 'unified' ? 'side-by-side' : 'unified'
  if (settings.settings) void settings.save({ ...settings.settings, diffMode: next })
}

const ignoreWhitespace = computed(() => settings.settings?.ignoreWhitespace ?? false)

function toggleIgnoreWhitespace(): void {
  if (settings.settings) void settings.save({ ...settings.settings, ignoreWhitespace: !ignoreWhitespace.value })
}

// stripHorizontalWhitespace drops spaces and tabs from s. map[i] is the
// original index of text's i-th character; its extra last entry, s.length,
// maps text's end.
function stripHorizontalWhitespace(s: string): { text: string; map: number[] } {
  let text = ''
  const map: number[] = []
  for (let i = 0; i < s.length; i++) {
    const c = s[i]!
    if (c === ' ' || c === '\t') continue
    text += c
    map.push(i)
  }
  map.push(s.length)
  return { text, map }
}

// buildIgnoreWhitespaceOverride diffs whitespace-stripped copies of both
// texts and maps each change's offsets back to the originals. The results
// must be Change instances: CodeMirror calls Change methods its type
// declaration does not list, so a plain object type-checks but throws.
function buildIgnoreWhitespaceOverride(): (a: string, b: string) => readonly Change[] {
  return (a, b) => {
    const { text: sa, map: ma } = stripHorizontalWhitespace(a)
    const { text: sb, map: mb } = stripHorizontalWhitespace(b)
    return diffText(sa, sb).map((c) => new Change(ma[c.fromA]!, ma[c.toA]!, mb[c.fromB]!, mb[c.toB]!))
  }
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

// languageExtensionFor is the syntax highlighting for path's filename, or
// null (plain text) when @codemirror/language-data knows none.
async function languageExtensionFor(path: string): Promise<Extension | null> {
  const lang = LanguageDescription.matchFilename(languages, basename(path))
  return lang ? await lang.load() : null
}

// loadSeq drops a language load that resolves after a newer file started.
let loadSeq = 0

async function mountView(): Promise<void> {
  const seq = ++loadSeq
  destroyView()
  const file = diff.file
  const el = container.value
  if (!el || !file || file.binary || file.tooLarge) return

  const language = await languageExtensionFor(target.value?.path ?? '')
  if (seq !== loadSeq) return

  const readOnly = EditorState.readOnly.of(true)
  // classHighlighter emits plain "tok-*" classes, themed below.
  const lang = language ? [language, syntaxHighlighting(classHighlighter)] : []
  // Spread in: exactOptionalPropertyTypes rejects an explicit undefined.
  const diffConfig: { diffConfig?: DiffConfig } = ignoreWhitespace.value
    ? { diffConfig: { override: buildIgnoreWhitespaceOverride() } }
    : {}
  view =
    mode.value === 'unified'
      ? new EditorView({
          parent: el,
          state: EditorState.create({
            doc: file.newContent ?? '',
            extensions: [
              readOnly,
              ...lang,
              unifiedMergeView({ original: file.oldContent ?? '', mergeControls: false, ...diffConfig }),
              EditorView.updateListener.of((update) => {
                if (update.selectionSet) updateSelectedLinesEnabled()
              }),
            ],
          }),
        })
      : new MergeView({
          parent: el,
          a: { doc: file.oldContent ?? '', extensions: [readOnly, ...lang] },
          b: { doc: file.newContent ?? '', extensions: [readOnly, ...lang] },
          ...diffConfig,
        })
  hunks.value = view instanceof MergeView ? view.chunks : (getChunks(view.state)?.chunks ?? [])
  updateSelectedLinesEnabled()
}

// flush: 'post', so the .cm-container div exists when mountView reads it.
watch([() => diff.file, mode, ignoreWhitespace], mountView, { flush: 'post' })

onBeforeUnmount(destroyView)

// changeTarget is the new-side editor: the unified view, or side-by-side's b.
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

// stageLines stages sel, or unstages it on the staged side. On success the
// changes panel and this diff reload at once, not after the watcher's
// debounce.
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

// discardLines is offered on the unstaged side only; it confirms first,
// naming what is lost.
function discardLines(sel: LineSelection | null): void {
  if (!sel || !props.working) return
  const { path } = props.working
  confirmAction('Discard changes', `Discard this change to "${path}"? This cannot be undone.`, () => {
    void runAndClose(
      () => Facade.DiscardLines(props.wt, path, sel),
      () => {
        void changes.load(props.wt)
        load(true)
      },
    )
  })
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

// chunkDocs is the mounted view's old and new documents.
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

// hunkSelection names chunk's changed lines on each side. endA/endB, not
// toA/toB: those may point past the chunk's last line.
function hunkSelection(chunk: Chunk): LineSelection | null {
  const docs = chunkDocs()
  if (!docs) return null
  return {
    old: linesInRange(docs.old, chunk.fromA, chunk.endA),
    new: linesInRange(docs.new, chunk.fromB, chunk.endB),
  }
}

// selectedLines names the selection's lines, or null unless it lies inside
// one chunk. Unified mode only, and new-side lines only: deleted lines are
// decorations there.
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

// Previous/next file step through the compare list with range, else through
// the log's selected commit's files; a no-op past either end.
const stepsFiles = computed(() => (props.range ? !!props.files : !props.workingCompare && !props.working))

function stepFile(step: 1 | -1): void {
  if (!props.range) {
    if (step > 0) log.nextFile()
    else log.prevFile()
    return
  }
  const { range, files = [] } = props
  const i = files.findIndex((f) => f.path === range.path && (f.oldPath ?? '') === range.oldPath)
  const next = files[i + step]
  if (i >= 0 && next) emit('update:range', { ...range, path: next.path, oldPath: next.oldPath ?? '' })
}

interface ToolbarButton {
  test: string
  label: string
  glyph: string
  pressed?: boolean
  action: () => void
}

const toolbar = computed<ToolbarButton[]>(() => [
  ...(showsContent.value
    ? [
        {
          test: 'mode-toggle',
          label: mode.value === 'unified' ? 'Switch to side-by-side' : 'Switch to unified',
          glyph: '⇆',
          pressed: mode.value === 'side-by-side',
          action: toggleMode,
        },
        {
          test: 'ignore-whitespace-toggle',
          label: ignoreWhitespace.value ? 'Show whitespace changes' : 'Ignore whitespace',
          glyph: '␣',
          pressed: ignoreWhitespace.value,
          action: toggleIgnoreWhitespace,
        },
        { test: 'prev-change', label: 'Previous change', glyph: '↑', action: prevChange },
        { test: 'next-change', label: 'Next change', glyph: '↓', action: nextChange },
      ]
    : []),
  ...(stepsFiles.value
    ? [
        { test: 'prev-file', label: 'Previous file', glyph: '◀', action: () => stepFile(-1) },
        { test: 'next-file', label: 'Next file', glyph: '▶', action: () => stepFile(1) },
      ]
    : []),
  { test: 'open-editor', label: 'Open in editor', glyph: '✎', action: openEditor },
])
</script>

<template>
  <section class="diff-view">
    <ErrorBanner v-if="error" :error="error" @fallback="openEditorFallback" />
    <p v-else-if="diff.loading">Loading…</p>
    <template v-else-if="diff.file">
      <div class="diff-toolbar">
        <button
          v-for="b in toolbar"
          :key="b.test"
          type="button"
          class="glyph-btn"
          :data-test="b.test"
          :title="b.label"
          :aria-label="b.label"
          :aria-pressed="b.pressed"
          @click="b.action"
        >
          {{ b.glyph }}
        </button>
      </div>
      <p v-if="diff.file.binary">Binary file, no preview available.</p>
      <template v-else-if="diff.file.tooLarge">
        <p>File is too large to show automatically.</p>
        <button type="button" @click="load(true)">Load anyway</button>
      </template>
      <div v-if="showsContent && working && canStagePartially(working.status)" class="hunk-actions">
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
        <template v-if="!working.staged">
          <button
            v-for="(chunk, i) in hunks"
            :key="'discard-' + i"
            type="button"
            data-test="discard-hunk"
            @click="discardLines(hunkSelection(chunk))"
          >
            Discard hunk
          </button>
          <button
            type="button"
            data-test="discard-selected-lines"
            :disabled="!selectedLinesEnabled"
            @click="discardLines(selectedLines())"
          >
            Discard selected lines
          </button>
        </template>
      </div>
      <div v-if="showsContent" ref="container" class="cm-container"></div>
    </template>
  </section>
</template>

<style scoped>
/* Sticky: the side column scrolls as a whole, and next/previous change
   must stay clickable wherever a jump lands. */
.diff-toolbar {
  position: sticky;
  top: 0;
  z-index: 1;
  display: flex;
  flex-wrap: nowrap;
  align-items: center;
  gap: 0.3em;
  background: var(--p-content-background);
}
</style>

<style>
/* CodeMirror's own DOM is built outside Vue, so this is unscoped:
   classHighlighter's "tok-*" classes, themed with this app's own tokens
   rather than CodeMirror's default inline colors. */
.tok-keyword,
.tok-operator,
.tok-definitionKeyword,
.tok-moduleKeyword,
.tok-controlKeyword {
  color: var(--p-primary-color);
}
.tok-comment,
.tok-lineComment,
.tok-blockComment {
  color: var(--p-text-muted-color);
  font-style: italic;
}
</style>
