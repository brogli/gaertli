<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { useRepoStore } from '../stores/repo'
import { useRefsStore } from '../stores/refs'
import { useLogStore } from '../stores/log'
import { useCompareStore } from '../stores/compare'
import { useChangesStore } from '../stores/changes'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { BranchSummary } from '../../bindings/github.com/brogli/nebegit/internal/core'
import ErrorBanner from '../components/ErrorBanner.vue'
import CommandLogPanel from '../components/CommandLogPanel.vue'
import JobsStrip from '../components/JobsStrip.vue'
import LogList from '../components/LogList.vue'
import CommitDetailsPanel from '../components/CommitDetailsPanel.vue'
import CompareView from '../components/CompareView.vue'
import ChangesPanel from '../components/ChangesPanel.vue'
import ConflictBanner from '../components/ConflictBanner.vue'
import DiffView from '../components/DiffView.vue'
import RefsSidebar from '../components/RefsSidebar.vue'
import FilterBar from '../components/FilterBar.vue'
import CreateBranchDialog from '../components/CreateBranchDialog.vue'
import DetachWarningDialog from '../components/DetachWarningDialog.vue'
import ResetDialog from '../components/ResetDialog.vue'
import CreateWorktreeDialog from '../components/CreateWorktreeDialog.vue'
import SyncControls from '../components/SyncControls.vue'
import type { WorktreeStart } from '../components/worktreeStart'
import { basename } from '../lib/path'

const props = defineProps<{ path: string }>()
const emit = defineEmits<{ 'open-settings': [] }>()
const name = basename(props.path)
const repo = useRepoStore()
const refs = useRefsStore()
const log = useLogStore()
const compare = useCompareStore()
const changes = useChangesStore()

// The compared file's diff range; null outside compare mode or before a
// file is picked.
const compareRange = computed(() => {
  const { from, to, selectedFile } = compare
  return from && to && selectedFile
    ? { from: from.sha, to: to.sha, path: selectedFile.path, oldPath: selectedFile.oldPath ?? '' }
    : null
})

// Three mutually exclusive diff sources: a compared file wins while compare
// is active, then a changes-panel selection, then a commit's file. LogList
// and CommitDetailsPanel clear the changes selection themselves.
const workingTarget = computed(() => changes.selectedFile)
const showChangesDiff = computed(() => !compare.active && !!workingTarget.value)
const showCommitDiff = computed(() => !compare.active && !workingTarget.value && !!log.selectedFile)
const showDiff = computed(
  () => (compare.active && compareRange.value !== null) || showChangesDiff.value || showCommitDiff.value,
)
const { error, run } = useCoreCall()

// The dialogs LogList's context menu opens, one instance each.
const createBranchSha = ref<string | null>(null)
const checkoutCommitSha = ref<string | null>(null)
const resetTarget = ref<{ sha: string; subject: string } | null>(null)

// "Create worktree here…" from the log's commit menu or the refs sidebar's
// branch menu, pre-filling CreateWorktreeDialog.
const createWorktreeStart = ref<WorktreeStart | null>(null)
const createWorktreeBranches = ref<BranchSummary[]>([])

async function openCreateWorktree(start: WorktreeStart): Promise<void> {
  const branches = await run(() => Facade.ListBranches(props.path))
  if (branches === undefined) return
  createWorktreeBranches.value = branches ?? []
  createWorktreeStart.value = start
}

// The branch ResetDialog names; "" while detached, shown there as "HEAD".
const currentBranch = computed(() => (repo.summary?.detached ? '' : (repo.summary?.branch ?? '')))

onMounted(() => {
  void run(() => repo.loadSummary(props.path))
  void refs.load(props.path)
  void changes.load(props.path)
})

// repo:changed{scope:"index"} covers this worktree's staging writes and
// outside changes to the working tree.
let unlistenChanged: (() => void) | undefined
onMounted(() => {
  unlistenChanged = Events.On('repo:changed', (ev) => {
    const { scope } = ev.data as { scope: string }
    if (scope === 'index') void changes.load(props.path)
  })
})
onUnmounted(() => unlistenChanged?.())

// Manual refresh: if anything differs, repo:changed arrives the normal way.
function refresh(): void {
  void run(() => Facade.Refresh(props.path))
}

// labelled with the effective editor profile's name once the summary loads
const editorLabel = computed(() => `Open in ${repo.summary?.editorName || 'editor'}`)

function openInEditor(): void {
  void run(() => Facade.OpenFolder(props.path))
}

// The current branch, if it tracks an upstream that still exists.
const tracked = computed(() => {
  const current = refs.refs?.branches?.find((b) => b.current)
  return current?.upstream && !current.upstreamGone ? current : undefined
})

// "Create branch here" while detached: its button renders only with a
// loaded summary.
function openCreateBranchHere(): void {
  createBranchSha.value = repo.summary!.sha ?? null
}
</script>

<template>
  <main>
    <h1>{{ name }}</h1>
    <button type="button" @click="refresh">Refresh</button>
    <button type="button" @click="openInEditor">{{ editorLabel }}</button>
    <JobsStrip :wt="path" />
    <ErrorBanner v-if="error" :error="error" @open-settings="emit('open-settings')" />
    <p v-else-if="repo.summary">
      {{ repo.summary.detached ? 'detached HEAD' : repo.summary.branch }}
      <template v-if="repo.summary.operation"> ({{ repo.summary.operation }})</template>
      <span v-if="tracked" class="ahead-behind" data-test="ahead-behind">
        ↑{{ tracked.ahead }} ↓{{ tracked.behind }}
      </span>
      <button v-if="repo.summary.detached" type="button" @click="openCreateBranchHere">
        Create branch here
      </button>
      <SyncControls :wt="path" />
    </p>
    <ConflictBanner :wt="path" />
    <div class="worktree-body">
      <RefsSidebar
        :wt="path"
        @create-worktree="
          (name, isRemote) =>
            openCreateWorktree(isRemote ? { newBranch: true, start: name } : { newBranch: false, branch: name })
        "
      />
      <ChangesPanel :wt="path" />
      <div class="log-column">
        <FilterBar />
        <LogList
          :wt="path"
          @create-branch="createBranchSha = $event"
          @checkout-commit="checkoutCommitSha = $event"
          @reset="(sha, subject) => (resetTarget = { sha, subject })"
          @create-worktree="(sha) => openCreateWorktree({ newBranch: true, start: sha })"
        />
      </div>
      <div class="side-column">
        <CompareView v-if="compare.active" :wt="path" />
        <CommitDetailsPanel v-else :wt="path" />
        <DiffView v-if="showDiff" :wt="path" :range="compareRange" :working="workingTarget" />
      </div>
    </div>
    <CommandLogPanel :wt="path" />
    <CreateBranchDialog
      v-if="createBranchSha !== null"
      :wt="path"
      :sha="createBranchSha"
      :visible="true"
      @update:visible="createBranchSha = null"
    />
    <DetachWarningDialog
      v-if="checkoutCommitSha !== null"
      :wt="path"
      :sha="checkoutCommitSha"
      :visible="true"
      @update:visible="checkoutCommitSha = null"
    />
    <ResetDialog
      v-if="resetTarget !== null"
      :wt="path"
      :branch="currentBranch"
      :sha="resetTarget.sha"
      :subject="resetTarget.subject"
      :visible="true"
      @update:visible="resetTarget = null"
    />
    <CreateWorktreeDialog
      v-if="createWorktreeStart"
      :repo-path="path"
      :branches="createWorktreeBranches"
      :start="createWorktreeStart"
      :visible="true"
      @update:visible="createWorktreeStart = null"
    />
  </main>
</template>

<style scoped>
/* Refs left, log in the middle, details and diff in a column to its right.
   LogList's virtual scroll needs a bounded viewport: fill the app
   (style.css's height chain), give the body the leftover space. */
main {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}
.worktree-body {
  display: flex;
  gap: 1em;
  flex: 1;
  min-height: 0;
}
.log-column,
.side-column {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
  min-height: 0;
}
.side-column {
  overflow: auto;
}
</style>
