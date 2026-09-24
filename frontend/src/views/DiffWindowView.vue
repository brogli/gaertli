<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Window } from '@wailsio/runtime'
import { useEventListener } from '@vueuse/core'
import { useI18n } from '../i18n'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { DiffKind, type DiffTarget } from '../lib/diffTarget'
import { useWindowEvent } from '../lib/windowEvents'
import { useChangesStore } from '../stores/changes'
import { useCoreCall } from '../composables/useCoreCall'
import { useOpenInEditor } from '../composables/useOpenInEditor'
import ErrorBanner from '../components/ErrorBanner.vue'
import ChangedFileList from '../components/ChangedFileList.vue'
import DiffView from '../components/DiffView.vue'

// DiffWindowView is the diff window's root view: the files of its target
// on the left, one file's diff on the right. The window is its own JS
// realm, so it loads its own file list: a commit's files, a comparison's,
// or the working tree's changes in the changes view's groups.
const props = defineProps<{ wt: string; target: DiffTarget }>()

const wt = ref(props.wt)
const target = ref(props.target)
const files = ref<ChangedFile[]>([])
const changes = useChangesStore()
const { error, run } = useCoreCall()
const { openEditor } = useOpenInEditor(wt, run)
const { t } = useI18n()

const isWorking = computed(() => target.value.kind === DiffKind.WorkingDiff)

interface Group {
  title: string
  files: ChangedFile[]
  staged: boolean
}

// groups are the files the diff can show, in list order. A conflicted file
// has no diff here: it opens in the editor.
const groups = computed<Group[]>(() => {
  if (!isWorking.value) return [{ title: '', files: files.value, staged: false }]
  const f = changes.files
  return [
    { title: t('changesView.stagedTitle'), files: f?.staged ?? [], staged: true },
    { title: t('changesView.unstagedTitle'), files: f?.unstaged ?? [], staged: false },
    { title: t('changesView.untrackedTitle'), files: f?.untracked ?? [], staged: false },
  ]
})
const conflicted = computed(() => (isWorking.value ? (changes.files?.conflicted ?? []) : []))

const entries = computed(() =>
  groups.value.flatMap((g) => g.files.map((file) => ({ file, staged: g.staged }))),
)
const currentIndex = computed(() =>
  entries.value.findIndex(
    (e) => e.file.path === target.value.path && e.staged === !!target.value.staged,
  ),
)
const current = computed(() => entries.value[currentIndex.value])

function show(file: ChangedFile, staged: boolean): void {
  target.value = { ...target.value, path: file.path, oldPath: file.oldPath ?? '', staged }
}

// stepFile shows the next or previous file of the list; a no-op past
// either end.
function stepFile(step: 1 | -1): void {
  const next = entries.value[currentIndex.value + step]
  if (currentIndex.value >= 0 && next) show(next.file, next.staged)
}

// diffProps is DiffView's prop for the target's kind.
const diffProps = computed(() => {
  const t = target.value
  const file = { path: t.path, oldPath: t.oldPath ?? '' }
  switch (t.kind) {
    case DiffKind.CommitDiff:
      return { commit: { sha: t.sha ?? '', ...file } }
    case DiffKind.RangeDiff:
      return { range: { from: t.from ?? '', to: t.to ?? '', ...file } }
    case DiffKind.AgainstWorkingDiff:
      return { workingCompare: { sha: t.sha ?? '', ...file } }
    default:
      return { working: { ...file, staged: !!t.staged, status: current.value?.file.status ?? '' } }
  }
})

async function loadFiles(): Promise<void> {
  const t = target.value
  switch (t.kind) {
    case DiffKind.CommitDiff:
      files.value = (await Facade.CommitDetails(wt.value, t.sha ?? '')).files ?? []
      return
    case DiffKind.RangeDiff:
      files.value = (await Facade.CompareCommits(wt.value, t.from ?? '', t.to ?? '')) ?? []
      return
    case DiffKind.AgainstWorkingDiff:
      files.value = (await Facade.CompareWithWorking(wt.value, t.sha ?? '')) ?? []
      return
    default:
      await changes.load(wt.value)
  }
}

// The list reloads when the window turns to another commit, comparison or
// worktree, not when it turns to another file.
const listSource = computed(() => {
  const { kind, sha, from, to } = target.value
  return [wt.value, kind, sha, from, to].join('\n')
})
watch(listSource, () => void run(loadFiles), { immediate: true })

// repo:changed{scope:"index"} covers staging and outside edits, which
// change the working tree's lists.
useWindowEvent('repo:changed', (ev) => {
  const kind = target.value.kind
  if (
    ev.data.scope === 'index' &&
    (kind === DiffKind.WorkingDiff || kind === DiffKind.AgainstWorkingDiff)
  ) {
    void run(loadFiles)
  }
})

// diff:show turns this window to what its main window opened.
useWindowEvent('diff:show', (ev) => {
  wt.value = ev.data.wt
  target.value = ev.data.target
})

// Escape closes the window, unless a dialog (a discard's confirmation) is
// open: that Escape is the dialog's. Capturing runs this before the dialog
// closes itself.
useEventListener(
  window,
  'keydown',
  (e: KeyboardEvent) => {
    if (e.key === 'Escape' && !document.querySelector('[role=dialog], [role=alertdialog]'))
      void Window.Close()
  },
  { capture: true },
)
</script>

<template>
  <div class="diff-window">
    <aside class="file-list">
      <ErrorBanner v-if="error" :error="error" />
      <template v-for="g in groups" :key="g.title">
        <template v-if="g.files.length">
          <h3 v-if="g.title">{{ g.title }}</h3>
          <ChangedFileList
            :files="g.files"
            :selected="current?.staged === g.staged ? current.file : null"
            @select="(f) => show(f, g.staged)"
            @open-editor="openEditor"
          />
        </template>
      </template>
      <template v-if="conflicted.length">
        <h3>{{ t('changesView.conflictedTitle') }}</h3>
        <ChangedFileList :files="conflicted" @open="openEditor" @open-editor="openEditor" />
      </template>
    </aside>
    <DiffView :wt="wt" v-bind="diffProps" @step-file="stepFile" />
  </div>
</template>

<style scoped>
.diff-window {
  display: flex;
  gap: 1em;
  height: 100%;
  min-height: 0;
}
.file-list {
  overflow: auto;
  flex: 0 0 auto;
  max-width: 35%;
}
.file-list h3 {
  margin: 0.5rem 0 0.2rem;
  font-size: 0.9em;
}
/* The diff takes the rest of the window, not just its content's width. */
.diff-view {
  flex: 1 1 auto;
  min-width: 0;
  overflow: auto;
}
</style>
