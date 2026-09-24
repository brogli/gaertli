<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { EditorState, type Extension, Text } from '@codemirror/state'
import { type Command, EditorView } from '@codemirror/view'
import {
  type Chunk,
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
import { useI18n } from '../i18n'
import { useDiffStore } from '../stores/diff'
import { useSettingsStore } from '../stores/settings'
import { useChangesStore } from '../stores/changes'
import { useCoreCall } from '../composables/useCoreCall'
import { useConfirmAction } from '../composables/useConfirmAction'
import { hunkChanges, hunksIn } from '../lib/gitHunks'
import { basename } from '../lib/path'
import { iconClass, type IconName } from '../lib/icons'
import { useWindowEvent } from '../lib/windowEvents'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import ErrorBanner from './ErrorBanner.vue'
import type {
  DiffHunk,
  LineSelection,
} from '../../bindings/github.com/brogli/nebegit/internal/core'

// DiffView shows one file's diff, read-only: a file of a commit, a file
// between two commits (range), a commit against the working tree
// (workingCompare), or an uncommitted change (working). git computes the
// diff; CodeMirror draws its hunks. Previous/next file ask the parent,
// which holds the file list, with step-file.
type CommitTarget = { sha: string; path: string; oldPath: string }
type DiffRange = { from: string; to: string; path: string; oldPath: string }
type WorkingTarget = { path: string; oldPath: string; staged: boolean; status: string }

const props = defineProps<{
  wt: string
  commit?: CommitTarget | null
  range?: DiffRange | null
  workingCompare?: CommitTarget | null
  working?: WorkingTarget | null
}>()
const emit = defineEmits<{ 'step-file': [step: 1 | -1] }>()

const diff = useDiffStore()
const settings = useSettingsStore()
const changes = useChangesStore()
const { error, run, runAndClose } = useCoreCall()
const { confirmAction } = useConfirmAction()
const { t } = useI18n()

const container = ref<HTMLElement | null>(null)
let view: EditorView | MergeView | null = null

// Reactive copies of CodeMirror state: the mounted view's chunks, and
// whether the selection lies inside one.
const chunks = ref<readonly Chunk[]>([])
const selectedLinesEnabled = ref(false)

type Target =
  | { kind: 'commit'; sha: string; path: string; oldPath: string }
  | { kind: 'range'; from: string; to: string; path: string; oldPath: string }
  | { kind: 'againstWorking'; sha: string; path: string; oldPath: string }
  | ({ kind: 'working' } & WorkingTarget)

const target = computed<Target | null>(() => {
  if (props.commit) return { kind: 'commit', ...props.commit }
  if (props.range) return { kind: 'range', ...props.range }
  if (props.workingCompare) return { kind: 'againstWorking', ...props.workingCompare }
  if (props.working) return { kind: 'working', ...props.working }
  return null
})

const mode = computed(() => settings.settings?.diffMode ?? 'unified')
const ignoreWhitespace = computed(() => settings.settings?.ignoreWhitespace ?? false)
if (!settings.settings) void settings.load()

function load(force = false): void {
  const tgt = target.value
  if (!tgt) return
  const opts = { force, ignoreWhitespace: ignoreWhitespace.value }
  if (tgt.kind === 'range') {
    void run(() => diff.loadRange(props.wt, tgt.from, tgt.to, tgt.path, tgt.oldPath, opts))
  } else if (tgt.kind === 'againstWorking') {
    void run(() => diff.loadAgainstWorking(props.wt, tgt.sha, tgt.path, tgt.oldPath, opts))
  } else if (tgt.kind === 'working') {
    void run(() => diff.loadWorking(props.wt, tgt.path, tgt.oldPath, tgt.staged, opts))
  } else {
    void run(() => diff.load(props.wt, tgt.sha, tgt.path, tgt.oldPath, opts))
  }
}

watch([target, ignoreWhitespace], () => load(), { immediate: true })

// repo:changed{scope:"index"} fires on any change to this worktree's files
// or index.
useWindowEvent('repo:changed', (ev) => {
  const kind = target.value?.kind
  if (ev.data.scope === 'index' && (kind === 'working' || kind === 'againstWorking'))
    void run(() => diff.refetch())
})

// openEditor opens the file shown at the cursor's line on the new side;
// line 0 when no editor is mounted (a binary or too-large file).
function openEditor(): void {
  const tgt = target.value
  if (!tgt) return
  const cm = changeTarget()
  const line = cm ? cm.state.doc.lineAt(cm.state.selection.main.head).number : 0
  void run(() => Facade.OpenFile(props.wt, tgt.path, line))
}

// A binary or too-large file shows only its placeholder.
const showsContent = computed(() => !!diff.file && !diff.file.binary && !diff.file.tooLarge)

function toggleMode(): void {
  const next = mode.value === 'unified' ? 'side-by-side' : 'unified'
  if (settings.settings) void settings.save({ ...settings.settings, diffMode: next })
}

// toggleIgnoreWhitespace saves the setting; the load watch refetches with it.
function toggleIgnoreWhitespace(): void {
  if (settings.settings)
    void settings.save({ ...settings.settings, ignoreWhitespace: !ignoreWhitespace.value })
}

function destroyView(): void {
  view?.destroy()
  view = null
  chunks.value = []
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

  // classHighlighter emits plain "tok-*" classes, themed below.
  const lang = language ? [language, syntaxHighlighting(classHighlighter)] : []
  // Lines split at "\n" only, as git splits them: a lone "\r" stays in its
  // line, so git's hunks sit on git's lines.
  const docExtensions = [EditorState.readOnly.of(true), EditorState.lineSeparator.of('\n'), ...lang]
  // git's diff is line-based, so changed lines are marked whole.
  const gitDiff = {
    diffConfig: { override: hunkChanges(file.hunks ?? []) },
    highlightChanges: false,
  }
  view =
    mode.value === 'unified'
      ? new EditorView({
          parent: el,
          state: EditorState.create({
            doc: file.newContent ?? '',
            extensions: [
              ...docExtensions,
              unifiedMergeView({
                original: Text.of((file.oldContent ?? '').split('\n')),
                mergeControls: false,
                ...gitDiff,
              }),
              EditorView.updateListener.of((update) => {
                if (update.selectionSet) updateSelectedLinesEnabled()
              }),
            ],
          }),
        })
      : new MergeView({
          parent: el,
          a: { doc: file.oldContent ?? '', extensions: docExtensions },
          b: { doc: file.newContent ?? '', extensions: docExtensions },
          ...gitDiff,
        })
  chunks.value = view instanceof MergeView ? view.chunks : (getChunks(view.state)?.chunks ?? [])
  updateSelectedLinesEnabled()
}

// flush: 'post', so the .cm-container div exists when mountView reads it.
// The "Loading…" placeholder replaces that div during a fetch, so a new div
// mounts the view again even when the fetch returned the same diff.
watch([() => diff.file, mode, container], mountView, { flush: 'post' })

onBeforeUnmount(destroyView)

// changeTarget is the new-side editor: the unified view, or side-by-side's b.
function changeTarget(): EditorView | null {
  return view instanceof MergeView ? view.b : view
}

// stepChange moves the new-side cursor to a neighbouring change.
function stepChange(go: Command): void {
  const cm = changeTarget()
  if (cm) go(cm)
}

// stageLines stages sel, or unstages it on the staged side.
function stageLines(sel: LineSelection | null): void {
  if (!sel || !props.working) return
  const { path, staged } = props.working
  void runAndClose(() => Facade.StageLines(props.wt, path, sel, staged), reloadNow)
}

// discardLines is offered on the unstaged side only; it confirms first,
// naming what is lost.
function discardLines(sel: LineSelection | null): void {
  if (!sel || !props.working) return
  const { path } = props.working
  confirmAction(
    t('diffWindow.discardChangesTitle'),
    t('diffWindow.discardLineConfirm', { path }),
    () => {
      void runAndClose(() => Facade.DiscardLines(props.wt, path, sel), reloadNow)
    },
  )
}

// reloadNow reloads the working tree's changes and this diff after a line
// action, not after the watcher's debounce.
function reloadNow(): void {
  void changes.load(props.wt)
  load(true)
}

// lines is count consecutive 1-based line numbers from start.
function lines(start: number, count: number): number[] {
  return Array.from({ length: count }, (_, i) => start + i)
}

// oldLinesOf and newLinesOf are the line numbers h changes on each side.
const oldLinesOf = (h: DiffHunk) => lines(h.oldStart, h.oldLines)
const newLinesOf = (h: DiffHunk) => lines(h.newStart, h.newLines)

// chunkDocs is the mounted view's old and new documents.
function chunkDocs(): { old: Text; new: Text } | null {
  if (!view) return null
  if (view instanceof MergeView) return { old: view.a.state.doc, new: view.b.state.doc }
  return { old: getOriginalDoc(view.state), new: view.state.doc }
}

// offersLineActions: the file's hunks can be staged or discarded here, for
// a plain modification only. Untracked files have no diff to re-diff; git
// refuses a partial patch for an added or deleted file; a rename's file
// header would need rewriting. Those are staged whole from the changes
// view. The ignore-whitespace diff is for reading: its hunks are not git's
// patch, so the template holds the actions back until whitespace is shown.
const offersLineActions = computed(() => showsContent.value && props.working?.status === 'M')

// selection names lines of the shown diff. Its fingerprint lets Go refuse
// the selection if the file changed since it was shown.
function selection(old: number[], added: number[]): LineSelection {
  return { old, new: added, fingerprint: diff.file?.fingerprint ?? '' }
}

// hunkSelection names the lines of git's hunks that chunk shows, not the
// chunk's whole range: a joined chunk also covers unchanged lines.
function hunkSelection(chunk: Chunk): LineSelection | null {
  const docs = chunkDocs()
  const file = diff.file
  if (!docs || !file) return null
  const hunks = hunksIn(chunk, file.hunks ?? [], docs.old, docs.new)
  return selection(hunks.flatMap(oldLinesOf), hunks.flatMap(newLinesOf))
}

// selectedLines names the selected lines that git's hunks add, or null
// unless the selection lies inside one chunk and holds one. Unified mode
// only, so the selection holds new-side lines only: deleted lines are
// decorations there. Per git hunk it touches: an insertion gives just the
// selected lines; a one-for-one replacement each selected line with the
// old line at its offset; any other replacement the whole hunk, so a
// discard never removes an old line without restoring it.
function selectedLines(): LineSelection | null {
  if (!(view instanceof EditorView)) return null
  const sel = view.state.selection.main
  // toB, not endB: a selection through a chunk's whole last line extends
  // past endB, up to the next line's start.
  if (!chunks.value.some((c) => sel.from >= c.fromB && sel.to <= c.toB)) return null

  const doc = view.state.doc
  const selStartLine = doc.lineAt(sel.from).number
  const selEndLine = doc.lineAt(Math.max(sel.to - 1, sel.from)).number
  const old = new Set<number>()
  const added = new Set<number>()
  for (let n = selStartLine; n <= selEndLine; n++) {
    const h = diff.file?.hunks?.find(
      (hunk) => n >= hunk.newStart && n < hunk.newStart + hunk.newLines,
    )
    if (!h) continue
    if (h.oldLines && h.oldLines !== h.newLines) {
      oldLinesOf(h).forEach((o) => old.add(o))
      newLinesOf(h).forEach((a) => added.add(a))
      continue
    }
    added.add(n)
    if (h.oldLines) old.add(h.oldStart + n - h.newStart)
  }
  return added.size ? selection([...old], [...added]) : null
}

interface ToolbarButton {
  test: string
  label: string
  icon?: IconName
  pressed?: boolean
  action: () => void
}

const toolbar = computed<ToolbarButton[]>(() => {
  const buttons: ToolbarButton[] = []
  if (showsContent.value) {
    buttons.push(
      {
        test: 'mode-toggle',
        label:
          mode.value === 'unified'
            ? t('diffWindow.switchToSideBySide')
            : t('diffWindow.switchToUnified'),
        icon: mode.value === 'unified' ? 'unifiedDiff' : 'sideBySideDiff',
        pressed: mode.value === 'side-by-side',
        action: toggleMode,
      },
      {
        test: 'ignore-whitespace-toggle',
        label: ignoreWhitespace.value
          ? t('diffWindow.showWhitespaceChanges')
          : t('diffWindow.ignoreWhitespace'),
        pressed: ignoreWhitespace.value,
        action: toggleIgnoreWhitespace,
      },
      {
        test: 'prev-change',
        label: t('diffWindow.previousChange'),
        icon: 'prevChange',
        action: () => stepChange(goToPreviousChunk),
      },
      {
        test: 'next-change',
        label: t('diffWindow.nextChange'),
        icon: 'nextChange',
        action: () => stepChange(goToNextChunk),
      },
    )
  }
  buttons.push(
    {
      test: 'prev-file',
      label: t('diffWindow.previousFile'),
      icon: 'prevFile',
      action: () => emit('step-file', -1),
    },
    {
      test: 'next-file',
      label: t('diffWindow.nextFile'),
      icon: 'nextFile',
      action: () => emit('step-file', 1),
    },
    {
      test: 'open-editor',
      label: t('common.openInEditor'),
      icon: 'openEditor',
      action: openEditor,
    },
  )
  return buttons
})
</script>

<template>
  <section class="diff-view">
    <ErrorBanner v-if="error" :error="error" />
    <p v-else-if="diff.loading">{{ t('diffWindow.loading') }}</p>
    <template v-else-if="diff.file">
      <div class="diff-toolbar">
        <button
          v-for="b in toolbar"
          :key="b.test"
          type="button"
          :class="{ 'glyph-btn': b.icon }"
          :data-test="b.test"
          :title="b.label"
          :aria-label="b.label"
          :aria-pressed="b.pressed"
          @click="b.action"
        >
          <i v-if="b.icon" :class="iconClass(b.icon)" aria-hidden="true"></i>
          <template v-else>{{ b.label }}</template>
        </button>
      </div>
      <p v-if="diff.file.binary">{{ t('diffWindow.binaryFile') }}</p>
      <template v-else-if="diff.file.tooLarge">
        <p>{{ t('diffWindow.tooLarge') }}</p>
        <button type="button" @click="load(true)">{{ t('diffWindow.loadAnyway') }}</button>
      </template>
      <template v-if="offersLineActions && working">
        <p v-if="ignoreWhitespace" data-test="whitespace-hint">
          {{
            working.staged
              ? t('diffWindow.showWhitespaceToUnstageHint')
              : t('diffWindow.showWhitespaceToStageHint')
          }}
        </p>
        <div v-else class="hunk-actions">
          <button
            v-for="(chunk, i) in chunks"
            :key="i"
            type="button"
            data-test="stage-hunk"
            @click="stageLines(hunkSelection(chunk))"
          >
            {{ working.staged ? t('diffWindow.unstageHunk') : t('diffWindow.stageHunk') }}
          </button>
          <button
            type="button"
            data-test="stage-selected-lines"
            :disabled="!selectedLinesEnabled"
            @click="stageLines(selectedLines())"
          >
            {{
              working.staged
                ? t('diffWindow.unstageSelectedLines')
                : t('diffWindow.stageSelectedLines')
            }}
          </button>
          <template v-if="!working.staged">
            <button
              v-for="(chunk, i) in chunks"
              :key="'discard-' + i"
              type="button"
              data-test="discard-hunk"
              @click="discardLines(hunkSelection(chunk))"
            >
              {{ t('diffWindow.discardHunk') }}
            </button>
            <button
              type="button"
              data-test="discard-selected-lines"
              :disabled="!selectedLinesEnabled"
              @click="discardLines(selectedLines())"
            >
              {{ t('diffWindow.discardSelectedLines') }}
            </button>
          </template>
        </div>
      </template>
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
.tok-operator {
  color: var(--p-primary-color);
}
.tok-comment {
  color: var(--p-text-muted-color);
  font-style: italic;
}
</style>
