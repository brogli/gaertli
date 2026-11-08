<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Window } from '@wailsio/runtime'
import { useEventListener } from '@vueuse/core'
import Splitter from 'openvue/splitter'
import SplitterPanel from 'openvue/splitterpanel'
import { useI18n } from '@/i18n'
import { Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'
import {
  ChangeScope,
  type ChangedFile,
} from '../../bindings/github.com/brogli/gaertli/internal/core'
import { DiffKind, fileTarget, readsWorkingTree, type DiffTarget } from '@/lib/diffTarget'
import { useRepoChanged, useWindowEvent } from '@/lib/windowEvents'
import { useChangesStore } from '@/stores/changes'
import { useDiffStore } from '@/stores/diff'
import { useMergeDrafts } from '@/composables/useMergeDrafts'
import { useCoreCall } from '@/composables/useCoreCall'
import { useOpenInEditor } from '@/composables/useOpenInEditor'
import { useFileTreeChoice } from '@/composables/useFileTreeChoice'
import ErrorBanner from '@/components/ErrorBanner.vue'
import ChangedFileList from '@/components/ChangedFileList.vue'
import ChangedFileTree from '@/components/ChangedFileTree.vue'
import FileTreeControls from '@/components/FileTreeControls.vue'
import DiffView from '@/components/DiffView.vue'
import ConflictView from '@/components/ConflictView.vue'

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
const asTree = useFileTreeChoice()
const folded = ref(false)
const { t } = useI18n()
const { keepOnly } = useMergeDrafts()

const isWorking = computed(() => target.value.kind === DiffKind.WorkingDiff)

interface Group {
  title: string
  files: ChangedFile[]
  staged: boolean
  conflicted: boolean
}

// An entry is a file of the list in the group it is shown in.
type Entry = Pick<Group, 'staged' | 'conflicted'> & { file: ChangedFile }

// groups are the files the window can show, in list order: the working
// tree's conflicted files last, each shown as a conflict, not a diff.
const groups = computed<Group[]>(() => {
  if (!isWorking.value) return [{ title: '', files: files.value, staged: false, conflicted: false }]
  const f = changes.files
  return [
    {
      title: t('changesView.stagedTitle'),
      files: f?.staged ?? [],
      staged: true,
      conflicted: false,
    },
    {
      title: t('changesView.unstagedTitle'),
      files: f?.unstaged ?? [],
      staged: false,
      conflicted: false,
    },
    {
      title: t('changesView.untrackedTitle'),
      files: f?.untracked ?? [],
      staged: false,
      conflicted: false,
    },
    {
      title: t('changesView.conflictedTitle'),
      files: f?.conflicted ?? [],
      staged: false,
      conflicted: true,
    },
  ]
})

const entries = computed<Entry[]>(() =>
  groups.value.flatMap(({ files, staged, conflicted }) =>
    files.map((file) => ({ file, staged, conflicted })),
  ),
)
const currentIndex = computed(() =>
  entries.value.findIndex(
    (e) =>
      e.file.path === target.value.path &&
      e.staged === !!target.value.staged &&
      e.conflicted === !!target.value.conflicted,
  ),
)
const current = computed(() => entries.value[currentIndex.value])

function show({ file, staged, conflicted }: Entry): void {
  target.value = fileTarget(file, { ...target.value, staged, conflicted })
}

// stepFile shows the next or previous file of the list; a no-op past
// either end.
function stepFile(step: 1 | -1): void {
  const next = entries.value[currentIndex.value + step]
  if (currentIndex.value >= 0 && next) show(next)
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

// A conflicted file that leaves the list (resolved) loses its unsaved merge
// result and gives way to the conflicted file now at its place, if any.
watch(
  () => changes.files?.conflicted ?? [],
  (now, before) => {
    keepOnly(
      wt.value,
      now.map((f) => f.path),
    )
    if (!target.value.conflicted || now.some((f) => f.path === target.value.path)) return
    const next = at(
      now,
      before.findIndex((f) => f.path === target.value.path),
    )
    if (next) show({ file: next, staged: false, conflicted: true })
  },
)

// at is the file of files at index, clamped into the list; undefined for an
// empty list.
function at(files: ChangedFile[], index: number): ChangedFile | undefined {
  return files[Math.min(Math.max(index, 0), files.length - 1)]
}

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
  <Splitter
    layout="horizontal"
    class="flex h-full min-h-0"
    state-key="gaertli.layout.diffWindow"
    state-storage="local"
  >
    <SplitterPanel :size="25" :min-size="10" class="flex min-h-0 flex-col">
      <ErrorBanner v-if="error" :error="error" class="mx-2.5 mt-2.5" />
      <aside class="file-list min-h-0 flex-1 overflow-auto pl-2.5">
        <FileTreeControls v-model:as-tree="asTree" v-model:folded="folded" />
        <template v-for="g in groups" :key="g.title">
          <template v-if="g.files.length">
            <h3 v-if="g.title" class="mt-2 mb-1 text-sm">{{ g.title }}</h3>
            <component
              :is="asTree ? ChangedFileTree : ChangedFileList"
              :folded="asTree ? folded : undefined"
              :files="g.files"
              :selected="
                current?.staged === g.staged && current.conflicted === g.conflicted
                  ? current.file
                  : null
              "
              @select="(file) => show({ file, staged: g.staged, conflicted: g.conflicted })"
              @open-editor="openEditor"
            />
          </template>
        </template>
      </aside>
    </SplitterPanel>
    <SplitterPanel :size="75" :min-size="30">
      <template v-if="target.conflicted">
        <ConflictView v-if="current" :wt="wt" :path="target.path" @step-file="stepFile" />
        <p v-else-if="changes.files" class="p-2.5">{{ t('diffWindow.noConflictsLeft') }}</p>
      </template>
      <DiffView
        v-else
        class="h-full overflow-auto"
        :wt="wt"
        :target="target"
        :status="current?.file.status ?? ''"
        @step-file="stepFile"
      />
    </SplitterPanel>
  </Splitter>
</template>
