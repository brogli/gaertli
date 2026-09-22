<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { EditorState } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { goToNextChunk, goToPreviousChunk, MergeView, unifiedMergeView } from '@codemirror/merge'
import { useDiffStore } from '../stores/diff'
import { useSettingsStore } from '../stores/settings'
import { useLogStore } from '../stores/log'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import ErrorBanner from './ErrorBanner.vue'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'

// DiffView renders a file's two-sided content: either the log store's
// selected file at its selected commit (FR-LOG-04, ADR-7) or, when range is
// given, a compare's file between two arbitrary commits (FR-CMP-02).
// CodeMirror computes the diff client-side from the store's fetched
// old/new content. Read-only; unified/side-by-side is Settings.DiffMode
// (FR-DIFF-01).
//
// files is the compare diff window's own ordered file list, meaningful only
// together with range. DiffView doesn't own range, so stepping through
// files emits update:range instead of mutating the prop.
type DiffRange = { from: string; to: string; path: string; oldPath: string }

const props = defineProps<{
  wt: string
  range?: DiffRange | null
  files?: ChangedFile[]
}>()
const emit = defineEmits<{ 'update:range': [range: DiffRange] }>()

const diff = useDiffStore()
const settings = useSettingsStore()
const log = useLogStore()
const { error, run } = useCoreCall()

const container = ref<HTMLElement | null>(null)
let view: EditorView | MergeView | null = null

type Target =
  | { kind: 'commit'; sha: string; path: string; oldPath: string }
  | { kind: 'range'; from: string; to: string; path: string; oldPath: string }

const target = computed<Target | null>(() => {
  if (props.range) return { kind: 'range', ...props.range }
  const file = log.selectedFile
  const sha = log.details?.sha
  return file && sha ? { kind: 'commit', sha, path: file.path, oldPath: file.oldPath ?? '' } : null
})

function load(force = false): void {
  const t = target.value
  if (!t) return
  if (t.kind === 'range') {
    void run(() => diff.loadRange(props.wt, t.from, t.to, t.path, t.oldPath, force))
  } else {
    void run(() => diff.load(props.wt, t.sha, t.path, t.oldPath, force))
  }
}

watch(target, () => load(), { immediate: true })

// FR-ED-02: open the file currently shown, commit or range mode alike —
// both Target kinds carry the same path. Line is always 0 (FR-ED-09's "at
// the line shown" is a SHOULD, still open).
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
            ],
          }),
        })
      : new MergeView({
          parent: el,
          a: { doc: file.oldContent ?? '', extensions: [readOnly] },
          b: { doc: file.newContent ?? '', extensions: [readOnly] },
        })
}

// flush: 'post' so the target branch's DOM (the .cm-container div) has
// already rendered by the time mountView reads the template ref.
watch([() => diff.file, mode], mountView, { flush: 'post' })

onBeforeUnmount(destroyView)

// Next/previous change (FR-DIFF-02) run CodeMirror's own chunk commands
// against the side a reviewer reads top-to-bottom: the unified view's
// single editor, or the side-by-side view's b (new/right) editor.
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

// stepRangeFile is files' own next/previous-file (FR-CMP-02), a no-op past
// either end like the log store's nextFile/prevFile. File and change
// navigation stay two independent controls.
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
      <template v-else-if="!range">
        <button type="button" data-test="prev-file" @click="log.prevFile()">Previous file</button>
        <button type="button" data-test="next-file" @click="log.nextFile()">Next file</button>
      </template>
      <button type="button" data-test="open-editor" @click="openEditor">Open in editor</button>
      <div ref="container" class="cm-container"></div>
    </template>
  </section>
</template>
