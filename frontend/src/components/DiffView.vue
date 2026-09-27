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
import Button from 'openvue/button'
import ToggleButton from 'openvue/togglebutton'
import { useI18n } from '@/i18n'
import { useDiffStore } from '@/stores/diff'
import { useSettingsStore } from '@/stores/settings'
import { useChangesStore } from '@/stores/changes'
import { useCoreCall } from '@/composables/useCoreCall'
import { useConfirmAction } from '@/composables/useConfirmAction'
import { useSyncedScrollX } from '@/composables/useSyncedScrollX'
import { hunkChanges, hunksIn } from '@/lib/gitHunks'
import { basename } from '@/lib/path'
import { iconClass, type IconName } from '@/lib/icons'
import { ICON_BUTTON, SECONDARY_BUTTON } from '@/lib/layout'
import { useRepoChanged } from '@/lib/windowEvents'
import { DiffKind, readsWorkingTree, type DiffTarget } from '@/lib/diffTarget'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { DiffMode } from '../../bindings/github.com/brogli/nebegit/internal/settings'
import ErrorBanner from './ErrorBanner.vue'
import {
  ChangeScope,
  type DiffHunk,
  type LineSelection,
} from '../../bindings/github.com/brogli/nebegit/internal/core'

// DiffView shows target's file diff, read-only: a file of a commit, a file
// between two commits, a commit against the working tree, or an
// uncommitted change. status is a working-tree file's status: line actions
// need it. git computes the diff; CodeMirror draws its hunks.
// Previous/next file ask the parent, which holds the file list, with
// step-file.
const props = defineProps<{ wt: string; target: DiffTarget; status?: string }>()
const emit = defineEmits<{ 'step-file': [step: 1 | -1] }>()

const diff = useDiffStore()
const settings = useSettingsStore()
const changes = useChangesStore()
const { error, run, runAndClose } = useCoreCall()
// An outside change's refetch reports through its own call, so the user's
// last action keeps a retryable error and its Try again.
const refetchCall = useCoreCall()
const shownError = computed(() => error.value ?? refetchCall.error.value)
const { confirmAction } = useConfirmAction()
const { t } = useI18n()

const container = ref<HTMLElement | null>(null)
let view: EditorView | MergeView | null = null
let untieScrollX: (() => void) | null = null

// Reactive copies of CodeMirror state: the mounted view's chunks, and
// whether the selection lies inside one.
const chunks = ref<readonly Chunk[]>([])
const selectedLinesEnabled = ref(false)

const working = computed(() => props.target.kind === DiffKind.WorkingDiff)
const staged = computed(() => !!props.target.staged)

const mode = computed(() => settings.settings?.diffMode ?? DiffMode.UnifiedDiff)
const ignoreWhitespace = computed(() => settings.settings?.ignoreWhitespace ?? false)

function load(force = false): void {
  refetchCall.error.value = null
  const opts = { force, ignoreWhitespace: ignoreWhitespace.value }
  void run(() => diff.load(props.wt, props.target, opts))
}

watch([() => props.target, ignoreWhitespace], () => load(), { immediate: true })

// repo:changed{scope:"index"} fires on any change to this worktree's files
// or index. An action's error without a retry is stale once the diff
// reloads.
useRepoChanged([ChangeScope.IndexChanged], () => {
  if (!readsWorkingTree(props.target)) return
  if (!error.value?.retry) error.value = null
  void refetchCall.run(() => diff.refetch())
})

// openEditor opens the file shown at the cursor's line on the new side;
// line 0 when no editor is mounted (a binary or too-large file).
function openEditor(): void {
  const cm = changeTarget()
  const line = cm ? cm.state.doc.lineAt(cm.state.selection.main.head).number : 0
  void run(() => Facade.OpenFile(props.wt, props.target.path, line))
}

// A binary or too-large file shows only its placeholder.
const showsContent = computed(() => !!diff.file && !diff.file.binary && !diff.file.tooLarge)

function toggleMode(): void {
  const next = mode.value === DiffMode.UnifiedDiff ? DiffMode.SideBySideDiff : DiffMode.UnifiedDiff
  if (settings.settings) void settings.save({ ...settings.settings, diffMode: next })
}

// toggleIgnoreWhitespace saves the setting; the load watch refetches with it.
function toggleIgnoreWhitespace(): void {
  if (settings.settings)
    void settings.save({ ...settings.settings, ignoreWhitespace: !ignoreWhitespace.value })
}

// whitespaceLabel names the diff's current whitespace state: the toggle's
// label, title and accessible name.
const whitespaceLabel = computed(() =>
  ignoreWhitespace.value ? t('diffWindow.ignoringWhitespace') : t('diffWindow.showingWhitespace'),
)

function destroyView(): void {
  untieScrollX?.()
  untieScrollX = null
  view?.destroy()
  view = null
  chunks.value = []
  selectedLinesEnabled.value = false
}

function updateSelectedLinesEnabled(): void {
  selectedLinesEnabled.value = selectedLines() !== null
}

// classHighlighter emits plain "tok-*" classes; they take the app's own
// tokens rather than CodeMirror's default colours.
const CODE_COLOURS = EditorView.theme({
  '.tok-keyword, .tok-operator': { color: 'var(--p-primary-color)' },
  '.tok-comment': { color: 'var(--p-text-muted-color)', fontStyle: 'italic' },
})

// languageExtensionFor is the syntax highlighting for path's filename, or
// null (plain text) when @codemirror/language-data knows none.
async function languageExtensionFor(path: string): Promise<Extension | null> {
  const lang = LanguageDescription.matchFilename(languages, basename(path))
  return lang ? await lang.load() : null
}

// FIXED_WIDTH_FONT sets the diff text in the app's fixed-width font, in
// place of CodeMirror's default system monospace.
const FIXED_WIDTH_FONT = EditorView.theme({ '.cm-scroller': { fontFamily: 'var(--nb-font-mono)' } })

// loadSeq drops a language load that resolves after a newer file started.
let loadSeq = 0

async function mountView(): Promise<void> {
  const seq = ++loadSeq
  destroyView()
  const file = diff.file
  const el = container.value
  if (!el || !file || file.binary || file.tooLarge) return

  const language = await languageExtensionFor(props.target.path)
  if (seq !== loadSeq) return

  const lang = language ? [language, syntaxHighlighting(classHighlighter), CODE_COLOURS] : []
  // Lines split at "\n" only, as git splits them: a lone "\r" stays in its
  // line, so git's hunks sit on git's lines.
  const docExtensions = [
    EditorState.readOnly.of(true),
    EditorState.lineSeparator.of('\n'),
    FIXED_WIDTH_FONT,
    ...lang,
  ]
  // git's diff is line-based, so changed lines are marked whole.
  const gitDiff = {
    diffConfig: { override: hunkChanges(file.hunks ?? []) },
    highlightChanges: false,
  }
  view =
    mode.value === DiffMode.UnifiedDiff
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
  if (view instanceof MergeView) untieScrollX = useSyncedScrollX(view.a.scrollDOM, view.b.scrollDOM)
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

// stageLines stages sel, or unstages it on the staged side. The file and
// side are read at the click: a retry after a lock stages what was clicked.
function stageLines(sel: LineSelection | null): void {
  if (!sel || !working.value) return
  const { path } = props.target
  const unstage = staged.value
  void runAndClose(() => Facade.StageLines(props.wt, path, sel, unstage), reloadNow)
}

// discardLines is offered on the unstaged side only; it confirms first,
// naming what is lost.
function discardLines(sel: LineSelection | null): void {
  if (!sel || !working.value) return
  const { path } = props.target
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
const offersLineActions = computed(
  () => showsContent.value && working.value && props.status === 'M',
)

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

// A toolbar toggle shows pressed by dropping ICON_BUTTON's text look.
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
          mode.value === DiffMode.UnifiedDiff
            ? t('diffWindow.switchToSideBySide')
            : t('diffWindow.switchToUnified'),
        icon: mode.value === DiffMode.UnifiedDiff ? 'unifiedDiff' : 'sideBySideDiff',
        pressed: mode.value === DiffMode.SideBySideDiff,
        action: toggleMode,
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
  <section class="diff-view flex flex-col gap-2">
    <ErrorBanner v-if="shownError" :error="shownError" />
    <p v-else-if="diff.loading">{{ t('diffWindow.loading') }}</p>
    <template v-else-if="diff.file">
      <!-- Sticky: the diff view scrolls as a whole (DiffWindowView.vue), and
           next/previous change must stay clickable wherever a jump lands. -->
      <div
        class="diff-toolbar sticky top-0 z-10 flex items-center gap-1 bg-(--p-content-background)"
      >
        <ToggleButton
          v-if="showsContent"
          :model-value="ignoreWhitespace"
          :on-label="t('diffWindow.ignoringWhitespace')"
          :off-label="t('diffWindow.showingWhitespace')"
          size="small"
          data-test="ignore-whitespace-toggle"
          :title="whitespaceLabel"
          :aria-label="whitespaceLabel"
          @update:model-value="toggleIgnoreWhitespace"
        />
        <Button
          v-for="b in toolbar"
          :key="b.test"
          v-bind="ICON_BUTTON"
          :text="!b.pressed"
          :icon="b.icon && iconClass(b.icon)"
          :label="b.icon ? undefined : b.label"
          :data-test="b.test"
          :title="b.label"
          :aria-label="b.label"
          :aria-pressed="b.pressed"
          @click="b.action"
        />
      </div>
      <p v-if="diff.file.binary">{{ t('diffWindow.binaryFile') }}</p>
      <template v-else-if="diff.file.tooLarge">
        <p>{{ t('diffWindow.tooLarge') }}</p>
        <Button :label="t('diffWindow.loadAnyway')" size="small" @click="load(true)" />
      </template>
      <template v-if="offersLineActions">
        <p v-if="ignoreWhitespace" data-test="whitespace-hint">
          {{
            staged
              ? t('diffWindow.showWhitespaceToUnstageHint')
              : t('diffWindow.showWhitespaceToStageHint')
          }}
        </p>
        <div v-else class="flex flex-wrap items-center gap-2">
          <Button
            v-for="(chunk, i) in chunks"
            :key="i"
            v-bind="SECONDARY_BUTTON"
            :label="staged ? t('diffWindow.unstageHunk') : t('diffWindow.stageHunk')"
            data-test="stage-hunk"
            @click="stageLines(hunkSelection(chunk))"
          />
          <Button
            v-bind="SECONDARY_BUTTON"
            :label="
              staged ? t('diffWindow.unstageSelectedLines') : t('diffWindow.stageSelectedLines')
            "
            data-test="stage-selected-lines"
            :disabled="!selectedLinesEnabled"
            @click="stageLines(selectedLines())"
          />
          <template v-if="!staged">
            <Button
              v-for="(chunk, i) in chunks"
              :key="'discard-' + i"
              v-bind="SECONDARY_BUTTON"
              :label="t('diffWindow.discardHunk')"
              data-test="discard-hunk"
              @click="discardLines(hunkSelection(chunk))"
            />
            <Button
              v-bind="SECONDARY_BUTTON"
              :label="t('diffWindow.discardSelectedLines')"
              data-test="discard-selected-lines"
              :disabled="!selectedLinesEnabled"
              @click="discardLines(selectedLines())"
            />
          </template>
        </div>
      </template>
      <div v-if="showsContent" ref="container" class="cm-container"></div>
    </template>
  </section>
</template>
