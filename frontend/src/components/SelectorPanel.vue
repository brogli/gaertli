<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import Tree from 'openvue/tree'
import type { TreeNode } from 'openvue/treenode'
import { AppService, Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { RecentRepo } from '../../bindings/github.com/brogli/nebegit/internal/settings'
import type { BranchSummary, Worktree } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { useCoreCall } from '../composables/useCoreCall'
import { useRepoStore } from '../stores/repo'
import ErrorBanner from './ErrorBanner.vue'
import OpenWSLFolderDialog from './OpenWSLFolderDialog.vue'
import CreateBranchDialog from './CreateBranchDialog.vue'
import CreateWorktreeDialog from './CreateWorktreeDialog.vue'

// SelectorPanel is the left column of every window (FR-WT-13, ADR-9): recent
// repos, each expandable to its worktrees and branches, loaded on first
// expand so browsing a repo starts no poller. Present with or without a
// selection (FR-UX-04).
const props = defineProps<{ selectedPath: string }>()
const emit = defineEmits<{ 'open-settings': [] }>()

const collapsed = ref(false)
const wslDialogOpen = ref(false)
const createBranchOpen = ref(false)
const createWorktreeFor = ref<RepoNode | null>(null)
const { error, run } = useCoreCall()
const repo = useRepoStore()

// RepoNode is one recent repo's panel state; worktrees and branches load
// on first expand.
interface RepoNode {
  repo: RecentRepo
  worktrees: Worktree[]
  branches: BranchSummary[]
  loaded: boolean
}
const nodes = ref<RepoNode[]>([])

onMounted(async () => {
  const repos = (await AppService.RecentRepos()) ?? []
  nodes.value = repos.map((repo) => ({ repo, worktrees: [], branches: [], loaded: false }))
})

// repo:changed{scope:"worktrees"} refreshes only the repo node of this
// window's own selection; a repo merely expanded here is not push-live,
// re-expanding it picks up changes.
let unlistenChanged: (() => void) | undefined
onMounted(() => {
  unlistenChanged = Events.On('repo:changed', (ev) => {
    const { scope } = ev.data as { scope: string }
    if (scope !== 'worktrees') return
    const rn = nodes.value.find((n) => n.repo.path === selectedRepoPath.value)
    if (!rn) return
    rn.loaded = false
    void expand(rn)
  })
})
onUnmounted(() => unlistenChanged?.())

function pickAndOpen(): void {
  void AppService.PickAndOpen()
}

function openRecent(path: string): void {
  void AppService.OpenRecent(path)
}

async function expand(rn: RepoNode): Promise<void> {
  if (rn.loaded) return
  const [worktrees, branches] = await Promise.all([
    Facade.ListWorktrees(rn.repo.path),
    Facade.ListBranches(rn.repo.path),
  ])
  rn.worktrees = worktrees ?? []
  rn.branches = branches ?? []
  rn.loaded = true
}

// selectedRepoPath is the repo node whose worktrees contain this window's
// selection: the marker's second condition, and what scopes "New branch…"
// and "Checkout" to the right node.
const selectedRepoPath = computed(
  () => nodes.value.find((n) => n.worktrees.some((w) => w.path === props.selectedPath))?.repo.path,
)

// PanelData rides along on each Tree node, as RefsSidebar's RefTreeNode
// does; kind narrows which of
// worktree/branch is populated. Flat rather than a discriminated union so
// the template can read it with plain optional chaining, no casts.
interface PanelData {
  kind: 'repo' | 'header' | 'worktree' | 'branch' | 'new-branch' | 'new-worktree'
  repoNode: RepoNode
  worktree?: Worktree
  branch?: BranchSummary
}
type PanelNode = TreeNode & { data: PanelData }

function nodeData(node: TreeNode): PanelData {
  return (node as PanelNode).data
}

// marker is the ●/○ shown on a worktree row.
function marker(data: PanelData): string {
  const isThisWindow = data.repoNode.repo.path === selectedRepoPath.value
  return isThisWindow && data.worktree?.path === props.selectedPath ? '●' : '○'
}

// leafNode is a plain, non-selectable row: a group header ("Worktrees",
// "Branches") or a button row ("+ New worktree", "New branch…") — never
// selectable, so a click on one never also triggers node-select.
function leafNode(rn: RepoNode, suffix: string, label: string, kind: 'header' | 'new-branch' | 'new-worktree'): PanelNode {
  return { key: `${rn.repo.path}#${suffix}`, label, leaf: true, selectable: false, data: { kind, repoNode: rn } }
}

function worktreeNode(rn: RepoNode, wt: Worktree): PanelNode {
  return {
    key: `${rn.repo.path}#wt#${wt.path}`,
    label: wt.branch || `detached @ ${wt.sha.slice(0, 7)}`,
    leaf: true,
    selectable: true,
    data: { kind: 'worktree', repoNode: rn, worktree: wt },
  }
}

function branchNode(rn: RepoNode, branch: BranchSummary): PanelNode {
  return {
    key: `${rn.repo.path}#br#${branch.name}`,
    label: branch.name,
    leaf: true,
    // Clickable like a worktree row only when it names an existing
    // worktree: switching to the branch is switching to that worktree.
    selectable: !!branch.checkedOutIn,
    data: { kind: 'branch', repoNode: rn, branch },
  }
}

// "New branch…" renders only under the repo node of this window's
// selection: CreateBranch acts on the selected worktree, which only exists
// for that repo. "+ New worktree" has no such restriction — any browsed repo
// can grow a worktree, whether or not it is this window's own selection.
function repoChildren(rn: RepoNode): PanelNode[] {
  if (!rn.loaded) return []
  const isSelectedRepo = rn.repo.path === selectedRepoPath.value
  return [
    leafNode(rn, 'wt-header', 'Worktrees', 'header'),
    ...rn.worktrees.map((wt) => worktreeNode(rn, wt)),
    leafNode(rn, 'new-worktree', '+ New worktree', 'new-worktree'),
    leafNode(rn, 'br-header', 'Branches', 'header'),
    ...rn.branches.map((b) => branchNode(rn, b)),
    ...(isSelectedRepo ? [leafNode(rn, 'new-branch', 'New branch…', 'new-branch')] : []),
  ]
}

const treeNodes = computed<PanelNode[]>(() =>
  nodes.value.map((rn) => ({
    key: rn.repo.path,
    label: rn.repo.distro ? `${rn.repo.path} (${rn.repo.distro})` : rn.repo.path,
    leaf: false,
    data: { kind: 'repo', repoNode: rn },
    children: repoChildren(rn),
  })),
)

function onNodeExpand(node: TreeNode): void {
  const data = nodeData(node)
  if (data.kind === 'repo') void expand(data.repoNode)
}

function onNodeSelect(node: TreeNode): void {
  const data = nodeData(node)
  if (data.kind === 'repo') selectWorktree(data.repoNode.repo.path)
  if (data.kind === 'worktree' && data.worktree) selectWorktree(data.worktree.path)
  if (data.kind === 'branch' && data.branch?.checkedOutIn) selectWorktree(data.branch.checkedOutIn)
}

// selectWorktree switches this window's own selection (FR-WT-05); a
// rejection surfaces through ErrorBanner (FR-UX-01) and leaves the window
// showing whatever it showed before.
function selectWorktree(path: string): void {
  void run(() => AppService.SelectWorktree(path))
}

// checkout and "New branch…" (below) always act on this window's own
// selected worktree (FR-BR-05's own wording, Design) — the same reason
// they render only under the selected repo's node.
function checkout(name: string): void {
  void run(() => Facade.Checkout(props.selectedPath, name))
}

// openInEditor is FR-WT-04's row control, no worktree session required;
// hidden for a missing (prunable) row, which has no folder to open.
function openInEditor(path: string): void {
  void run(() => Facade.OpenFolderAt(path))
}
</script>

<template>
  <aside class="selector-panel" :class="{ collapsed }">
    <button type="button" data-test="panel-collapse" @click="collapsed = !collapsed">
      {{ collapsed ? '»' : '«' }}
    </button>
    <div v-show="!collapsed" class="panel-body">
      <h1>nebegit</h1>
      <ErrorBanner v-if="error" :error="error" />
      <button type="button" @click="pickAndOpen">Open folder…</button>
      <button type="button" data-test="open-wsl-folder" @click="wslDialogOpen = true">
        Open WSL folder…
      </button>
      <Tree
        v-if="treeNodes.length"
        :value="treeNodes"
        selection-mode="single"
        @node-expand="onNodeExpand"
        @node-select="onNodeSelect"
      >
        <template #default="{ node }">
          <span v-if="nodeData(node).kind === 'header'" class="panel-header">{{ node.label }}</span>
          <span v-else-if="nodeData(node).kind === 'worktree'" class="worktree-row" data-test="worktree-row">
            <span>{{ marker(nodeData(node)) }}</span>
            <span>{{ node.label }}</span>
            <span v-if="nodeData(node).worktree?.dirty">*</span>
            <span v-if="nodeData(node).worktree?.locked">🔒</span>
            <span v-if="nodeData(node).worktree?.missing">missing</span>
            <button
              type="button"
              data-test="open-in-new-window"
              title="Open in new window"
              @click.stop="openRecent(nodeData(node).worktree!.path)"
            >
              ⧉
            </button>
            <button
              v-if="!nodeData(node).worktree?.missing"
              type="button"
              data-test="open-in-editor"
              title="Open in editor"
              @click.stop="openInEditor(nodeData(node).worktree!.path)"
            >
              Open in editor
            </button>
          </span>
          <span v-else-if="nodeData(node).kind === 'branch'" class="branch-row" data-test="branch-row">
            <span>{{ node.label }}</span>
            <button
              v-if="nodeData(node).repoNode.repo.path === selectedRepoPath"
              type="button"
              data-test="checkout-branch"
              @click.stop="checkout(nodeData(node).branch!.name)"
            >
              Checkout
            </button>
          </span>
          <span v-else-if="nodeData(node).kind === 'new-branch'">
            <button type="button" data-test="new-branch" @click.stop="createBranchOpen = true">
              {{ node.label }}
            </button>
          </span>
          <span v-else-if="nodeData(node).kind === 'new-worktree'">
            <button type="button" data-test="new-worktree" @click.stop="createWorktreeFor = nodeData(node).repoNode">
              {{ node.label }}
            </button>
          </span>
          <span v-else class="repo-node">{{ node.label }}</span>
        </template>
      </Tree>
      <button type="button" data-test="open-settings" @click="emit('open-settings')">Settings</button>
    </div>
    <OpenWSLFolderDialog
      v-if="wslDialogOpen"
      :visible="true"
      @update:visible="wslDialogOpen = false"
    />
    <CreateBranchDialog
      v-if="createBranchOpen"
      :wt="selectedPath"
      :sha="repo.summary?.sha ?? ''"
      :visible="true"
      @update:visible="createBranchOpen = false"
    />
    <CreateWorktreeDialog
      v-if="createWorktreeFor"
      :repo-path="createWorktreeFor.repo.path"
      :branches="createWorktreeFor.branches"
      :visible="true"
      @update:visible="createWorktreeFor = null"
      @created="createWorktreeFor!.loaded = false"
    />
  </aside>
</template>

<style scoped>
/* A resizable left column (FR-UX-07): the browser's own resize handle,
   no splitter component. Collapsed, only the toggle stays. */
.selector-panel {
  width: 22%;
  min-width: 12em;
  resize: horizontal;
  overflow: auto;
}
.selector-panel.collapsed {
  width: auto;
  min-width: 0;
  resize: none;
}
</style>
