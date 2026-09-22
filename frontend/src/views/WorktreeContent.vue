<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRepoStore } from '../stores/repo'
import { useRefsStore } from '../stores/refs'
import { useLogStore } from '../stores/log'
import { useCompareStore } from '../stores/compare'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { BranchSummary } from '../../bindings/github.com/brogli/nebegit/internal/core'
import ErrorBanner from '../components/ErrorBanner.vue'
import CommandLogPanel from '../components/CommandLogPanel.vue'
import LogList from '../components/LogList.vue'
import CommitDetailsPanel from '../components/CommitDetailsPanel.vue'
import CompareView from '../components/CompareView.vue'
import DiffView from '../components/DiffView.vue'
import RefsSidebar from '../components/RefsSidebar.vue'
import FilterBar from '../components/FilterBar.vue'
import CreateBranchDialog from '../components/CreateBranchDialog.vue'
import DetachWarningDialog from '../components/DetachWarningDialog.vue'
import ResetDialog from '../components/ResetDialog.vue'
import CreateWorktreeDialog from '../components/CreateWorktreeDialog.vue'
import type { WorktreeStart } from '../components/worktreeStart'
import { basename } from '../lib/path'

const props = defineProps<{ path: string }>()
const emit = defineEmits<{ 'open-settings': [] }>()
const name = basename(props.path)
const repo = useRepoStore()
const refs = useRefsStore()
const log = useLogStore()
const compare = useCompareStore()

// The compared file's diff range (FR-CMP-02); null outside compare mode or
// before a file is picked, so the template needs no non-null assertions.
const compareRange = computed(() => {
  const { from, to, selectedFile } = compare
  return from && to && selectedFile
    ? { from: from.sha, to: to.sha, path: selectedFile.path, oldPath: selectedFile.oldPath ?? '' }
    : null
})

// One decision for the diff pane: a compared file, or a commit's file
// outside compare mode — never a stale commit diff under CompareView.
const showDiff = computed(
  () => compareRange.value !== null || (!compare.active && !!log.selectedFile),
)
const { error, run } = useCoreCall()

// The dialogs LogList's context menu opens (FR-BR-01, FR-BR-02, FR-BR-08):
// one instance each, owned here rather than by LogList itself.
const createBranchSha = ref<string | null>(null)
const checkoutCommitSha = ref<string | null>(null)
const resetTarget = ref<{ sha: string; subject: string } | null>(null)

// FR-WT-06: "Create worktree here…" from the log's commit menu or the refs
// sidebar's branch menu, pre-filling CreateWorktreeDialog.
const createWorktreeStart = ref<WorktreeStart | null>(null)
const createWorktreeBranches = ref<BranchSummary[]>([])

async function openCreateWorktree(start: WorktreeStart): Promise<void> {
  createWorktreeBranches.value = (await Facade.ListBranches(props.path)) ?? []
  createWorktreeStart.value = start
}

// FR-BR-02: the branch ResetDialog names — "" while detached, shown there
// as "HEAD" (same as the summary panel just above it).
const currentBranch = computed(() => (repo.summary?.detached ? '' : (repo.summary?.branch ?? '')))

onMounted(() => {
  void run(() => repo.loadSummary(props.path))
  void refs.load(props.path)
})

// FR-REPO-04: manual refresh. No loading/disabled state — repo:changed, if
// anything actually differs, arrives the normal way.
function refresh(): void {
  void run(() => Facade.Refresh(props.path))
}

// FR-ED-01, FR-ED-05: labelled with the effective profile's name once the
// summary has loaded; a generic fallback only before then.
const editorLabel = computed(() => `Open in ${repo.summary?.editorName || 'editor'}`)

function openInEditor(): void {
  void run(() => Facade.OpenFolder(props.path))
}

// FR-REPO-03: the refs store already carries the current branch's upstream
// state; nothing to count without an upstream, and a gone upstream has no
// meaningful ahead/behind either.
const tracked = computed(() => {
  const current = refs.refs?.branches?.find((b) => b.current)
  return current?.upstream && !current.upstreamGone ? current : undefined
})

// FR-BR-08's "offer 'create branch here'" while detached: only reachable
// from the button that already requires repo.summary to be loaded, and its
// sha is always populated once the summary panel shows detached at all.
function openCreateBranchHere(): void {
  createBranchSha.value = repo.summary!.sha ?? null
}
</script>

<template>
  <main>
    <h1>{{ name }}</h1>
    <button type="button" @click="refresh">Refresh</button>
    <button type="button" @click="openInEditor">{{ editorLabel }}</button>
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
    </p>
    <div class="worktree-body">
      <RefsSidebar
        :wt="path"
        @create-worktree="
          (name, isRemote) =>
            openCreateWorktree(isRemote ? { newBranch: true, start: name } : { newBranch: false, branch: name })
        "
      />
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
        <DiffView v-if="showDiff" :wt="path" :range="compareRange" />
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
/* FR-UX-08: refs left, log in the middle, details and diff in a column to
   its right. LogList's virtual scroll (QA-PERF-03) needs a bounded viewport
   to clip against: fill the app (style.css's height chain), give the body
   the leftover space, everything else its natural size. */
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
