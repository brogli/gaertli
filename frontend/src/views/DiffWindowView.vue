<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Window } from '@wailsio/runtime'
import { useEventListener } from '@vueuse/core'
import { useI18n } from '@/i18n'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import {
  ChangeScope,
  type ChangedFile,
} from '../../bindings/github.com/brogli/nebegit/internal/core'
import { DiffKind, fileTarget, readsWorkingTree, type DiffTarget } from '@/lib/diffTarget'
import { useRepoChanged, useWindowEvent } from '@/lib/windowEvents'
import { useChangesStore } from '@/stores/changes'
import { useDiffStore } from '@/stores/diff'
import { useCoreCall } from '@/composables/useCoreCall'
import { useOpenInEditor } from '@/composables/useOpenInEditor'
import ErrorBanner from '@/components/ErrorBanner.vue'
import ChangedFileList from '@/components/ChangedFileList.vue'
import DiffView from '@/components/DiffView.vue'

// DiffWindowView is the diff window's root view: the files of its target
// on the left, one file's diff on the right. The window is its own JS
// realm, so it loads its own file list: a commit's files, a comparison's,
// or the working tree's changes in the changes view's groups.
const props = defineProps<{ wt: string; target: DiffTarget }>()

const wt = ref(props.wt)
const target = ref(props.target)
const files = ref<ChangedFile[]>([])
const changes = useChangesStore()
const diff = useDiffStore()
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
  target.value = fileTarget(file, { ...target.value, staged })
}

// stepFile shows the next or previous file of the list; a no-op past
// either end.
function stepFile(step: 1 | -1): void {
  const next = entries.value[currentIndex.value + step]
  if (currentIndex.value >= 0 && next) show(next.file, next.staged)
}

async function loadFiles(): Promise<void> {
  const { kind, sha = '', from = '', to = '' } = target.value
  switch (kind) {
    case DiffKind.CommitDiff:
      files.value = (await Facade.CommitDetails(wt.value, sha)).files ?? []
      return
    case DiffKind.RangeDiff:
      files.value = (await Facade.CompareCommits(wt.value, from, to)) ?? []
      return
    case DiffKind.AgainstWorkingDiff:
      files.value = (await Facade.CompareWithWorking(wt.value, sha)) ?? []
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
useRepoChanged([ChangeScope.IndexChanged], () => {
  if (readsWorkingTree(target.value)) void run(loadFiles)
})

// A restarted WSL core may see a changed repo: the list and the diff reload.
// Not a page reload: the URL names the window's first target, not its
// current one.
useWindowEvent('core:reconnected', () => {
  void run(loadFiles)
  void run(() => diff.refetch())
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
  <div class="flex h-full min-h-0 gap-4">
    <aside class="file-list max-w-[35%] flex-none overflow-auto pl-2.5">
      <ErrorBanner v-if="error" :error="error" />
      <template v-for="g in groups" :key="g.title">
        <template v-if="g.files.length">
          <h3 v-if="g.title" class="mt-2 mb-1 text-sm">{{ g.title }}</h3>
          <ChangedFileList
            :files="g.files"
            :selected="current?.staged === g.staged ? current.file : null"
            @select="(f) => show(f, g.staged)"
            @open-editor="openEditor"
          />
        </template>
      </template>
      <template v-if="conflicted.length">
        <h3 class="mt-2 mb-1 text-sm">{{ t('changesView.conflictedTitle') }}</h3>
        <ChangedFileList :files="conflicted" @open="openEditor" @open-editor="openEditor" />
      </template>
    </aside>
    <!-- The diff takes the rest of the window, not just its content's width. -->
    <DiffView
      class="min-w-0 flex-auto overflow-auto"
      :wt="wt"
      :target="target"
      :status="current?.file.status ?? ''"
      @step-file="stepFile"
    />
  </div>
</template>
