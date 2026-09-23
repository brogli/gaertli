<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import Tree from 'openvue/tree'
import type { TreeNode } from 'openvue/treenode'
import { AppService, Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { RecentRepo } from '../../bindings/github.com/brogli/nebegit/internal/settings'
import type { BranchSummary, Worktree } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { useCoreCall } from '../composables/useCoreCall'
import { useConfirmAction } from '../composables/useConfirmAction'
import { useRepoStore } from '../stores/repo'
import { basename, parentPath, repoPath } from '../lib/path'
import ErrorBanner from './ErrorBanner.vue'
import OpenWSLFolderDialog from './OpenWSLFolderDialog.vue'
import CreateBranchDialog from './CreateBranchDialog.vue'
import CreateWorktreeDialog from './CreateWorktreeDialog.vue'

// SelectorPanel is the left column of every window: recent repos, each
// expandable to its worktrees and branches, loaded on first expand so
// browsing a repo starts no poller. Present with or without a selection.
const props = defineProps<{ selectedPath: string }>()
const emit = defineEmits<{ 'open-settings': [] }>()

// Bindable, so a parent Splitter panel can shrink to match (MainView.vue);
// uncontrolled (a plain local ref) when no v-model is bound, e.g. in tests.
const collapsed = defineModel<boolean>('collapsed', { default: false })
const wslDialogOpen = ref(false)
const createBranchOpen = ref(false)
const createWorktreeFor = ref<RepoNode | null>(null)
const { error, run } = useCoreCall()
const { confirmAction } = useConfirmAction()
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
    if (rn) void reload(rn)
  })
})
onUnmounted(() => unlistenChanged?.())

function pickAndOpen(): void {
  void run(() => AppService.PickAndOpen())
}

function openRecent(path: string, distro?: string): void {
  void run(() => AppService.OpenRecent(repoPath(path, distro)))
}

// reload re-fetches rn's worktrees and branches. Its own call, not a flag
// for a later event: the watcher's event can arrive before the action that
// caused it returns.
function reload(rn: RepoNode): Promise<void> {
  rn.loaded = false
  return expand(rn)
}

async function expand(rn: RepoNode): Promise<void> {
  if (rn.loaded) return
  const path = repoPath(rn.repo.path, rn.repo.distro)
  const result = await run(() => Promise.all([Facade.ListWorktrees(path), Facade.ListBranches(path)]))
  if (!result) return
  const [worktrees, branches] = result
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

// PanelData rides along on each Tree node; kind says which of
// worktree/branch is set. Flat rather than a discriminated union so the
// template can read it with optional chaining, no casts.
interface PanelData {
  kind: 'repo' | 'header' | 'worktree' | 'branch' | 'new-branch' | 'new-worktree' | 'prune'
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

// leafNode is a non-selectable row: a group header ("Worktrees",
// "Branches") or a button row ("+ New worktree", "New branch…", "Clean up
// stale entries").
function leafNode(
  rn: RepoNode,
  suffix: string,
  label: string,
  kind: 'header' | 'new-branch' | 'new-worktree' | 'prune',
): PanelNode {
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
// selection: CreateBranch acts on the selected worktree. Any repo can get
// a new worktree.
function repoChildren(rn: RepoNode): PanelNode[] {
  if (!rn.loaded) return []
  const isSelectedRepo = rn.repo.path === selectedRepoPath.value
  return [
    leafNode(rn, 'wt-header', 'Worktrees', 'header'),
    ...rn.worktrees.map((wt) => worktreeNode(rn, wt)),
    leafNode(rn, 'new-worktree', '+ New worktree', 'new-worktree'),
    ...(rn.worktrees.some((w) => w.missing)
      ? [leafNode(rn, 'prune', 'Clean up stale entries', 'prune')]
      : []),
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
  const distro = data.repoNode.repo.distro
  if (data.kind === 'repo') selectWorktree(data.repoNode.repo.path, distro)
  if (data.kind === 'worktree' && data.worktree) selectWorktree(data.worktree.path, distro)
  if (data.kind === 'branch' && data.branch?.checkedOutIn) selectWorktree(data.branch.checkedOutIn, distro)
}

// selectWorktree switches this window's selection; on failure the window
// keeps showing what it showed before.
function selectWorktree(path: string, distro?: string): void {
  void run(() => AppService.SelectWorktree(repoPath(path, distro)))
}

// checkout and "New branch…" act on this window's selected worktree, so
// they render only under the selected repo's node.
function checkout(name: string): void {
  void run(() => Facade.Checkout(props.selectedPath, name))
}

// openInEditor needs no worktree session; hidden for a missing (prunable)
// row, which has no folder to open.
function openInEditor(path: string, distro?: string): void {
  void run(() => Facade.OpenFolderAt(repoPath(path, distro)))
}

// Every removal is confirmed. A row known dirty names the loss and removes
// forced. A row believed clean removes unforced; if git finds changes
// after all (DIRTY_WORKTREE), a second confirmation names the loss, then
// retries forced.
function removeWorktree(rn: RepoNode, wt: Worktree): void {
  const lossText = ' It has uncommitted changes that will be lost.'
  if (wt.dirty) {
    confirmAction('Remove worktree', `Remove the worktree at ${wt.path}?${lossText}`, () => void remove(rn, wt, true))
    return
  }
  confirmAction('Remove worktree', `Remove the worktree at ${wt.path}?`, () => void removeThenRetryIfDirty(rn, wt))
}

// removeThenRetryIfDirty is the clean-row path's accept callback: an
// unforced remove that, on a DIRTY_WORKTREE refusal, asks once more naming
// the loss before retrying forced.
async function removeThenRetryIfDirty(rn: RepoNode, wt: Worktree): Promise<void> {
  if ((await remove(rn, wt, false)) !== 'DIRTY_WORKTREE') return
  confirmAction(
    'Remove worktree',
    `${wt.path} has uncommitted changes that will be lost. Remove anyway?`,
    () => void remove(rn, wt, true),
  )
}

// remove returns the refusal's code, not shown in ErrorBanner when it is
// DIRTY_WORKTREE (the caller re-confirms instead); reloads the node on
// success.
async function remove(rn: RepoNode, wt: Worktree, force: boolean): Promise<string | undefined> {
  const distro = rn.repo.distro
  await run(() => Facade.RemoveWorktree(repoPath(rn.repo.path, distro), repoPath(wt.path, distro), force))
  const code = error.value?.code
  if (!code) {
    void reload(rn)
    return undefined
  }
  if (code === 'DIRTY_WORKTREE') error.value = null
  return code
}

// repoFolderName is a repo row's own line: its path's last segment, plus
// the WSL distro when set. The muted line under it is its parentPath.
function repoFolderName(repo: RecentRepo): string {
  const name = basename(repo.path)
  return repo.distro ? `${name} (${repo.distro})` : name
}

function pruneWorktrees(rn: RepoNode): void {
  void run(() => Facade.PruneWorktrees(repoPath(rn.repo.path, rn.repo.distro))).then(() => {
    if (!error.value) void reload(rn)
  })
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
      <div class="panel-actions">
        <button type="button" @click="pickAndOpen">Open folder…</button>
        <button type="button" data-test="open-wsl-folder" @click="wslDialogOpen = true">
          Open WSL folder…
        </button>
      </div>
      <Tree
        v-if="treeNodes.length"
        :value="treeNodes"
        selection-mode="single"
        @node-expand="onNodeExpand"
        @node-select="onNodeSelect"
      >
        <template #default="{ node }">
          <span v-if="nodeData(node).kind === 'header'" class="panel-header">{{ node.label }}</span>
          <span v-else-if="nodeData(node).kind === 'worktree'" class="panel-row" data-test="worktree-row">
            <span>{{ marker(nodeData(node)) }}</span>
            <span class="ellipsis" data-test="worktree-branch" :title="node.label">{{ node.label }}</span>
            <span class="muted ellipsis leftover" data-test="worktree-folder" :title="nodeData(node).worktree!.path">
              {{ basename(nodeData(node).worktree!.path) }}
            </span>
            <span v-if="nodeData(node).worktree?.dirty" title="Uncommitted changes">*</span>
            <span v-if="nodeData(node).worktree?.locked" title="Locked">🔒</span>
            <span v-if="nodeData(node).worktree?.missing">missing</span>
            <button
              type="button"
              class="glyph-btn"
              data-test="open-in-new-window"
              title="Open in new window"
              aria-label="Open in new window"
              @click.stop="openRecent(nodeData(node).worktree!.path, nodeData(node).repoNode.repo.distro)"
            >
              &#10697;
            </button>
            <button
              v-if="!nodeData(node).worktree?.missing"
              type="button"
              class="glyph-btn"
              data-test="open-in-editor"
              title="Open in editor"
              aria-label="Open in editor"
              @click.stop="openInEditor(nodeData(node).worktree!.path, nodeData(node).repoNode.repo.distro)"
            >
              &#9998;
            </button>
            <button
              v-if="!nodeData(node).worktree?.main && nodeData(node).worktree?.path !== selectedPath"
              type="button"
              class="glyph-btn"
              data-test="remove-worktree"
              title="Remove worktree"
              aria-label="Remove"
              @click.stop="removeWorktree(nodeData(node).repoNode, nodeData(node).worktree!)"
            >
              &#10005;
            </button>
          </span>
          <span v-else-if="nodeData(node).kind === 'branch'" class="panel-row" data-test="branch-row">
            <span class="ellipsis" :title="node.label">{{ node.label }}</span>
            <button
              v-if="nodeData(node).repoNode.repo.path === selectedRepoPath"
              type="button"
              class="glyph-btn"
              data-test="checkout-branch"
              title="Checkout"
              aria-label="Checkout"
              @click.stop="checkout(nodeData(node).branch!.name)"
            >
              &#8618;
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
          <span v-else-if="nodeData(node).kind === 'prune'">
            <button type="button" data-test="prune-worktrees" @click.stop="pruneWorktrees(nodeData(node).repoNode)">
              {{ node.label }}
            </button>
          </span>
          <span v-else class="repo-node" :title="nodeData(node).repoNode.repo.path">
            <span class="repo-name ellipsis">{{ repoFolderName(nodeData(node).repoNode.repo) }}</span>
            <span v-if="parentPath(nodeData(node).repoNode.repo.path)" class="repo-parent muted ellipsis">
              {{ parentPath(nodeData(node).repoNode.repo.path) }}
            </span>
          </span>
        </template>
      </Tree>
      <button type="button" data-test="open-settings" @click="emit('open-settings')">Settings</button>
    </div>
    <OpenWSLFolderDialog
      v-if="wslDialogOpen"
      @close="wslDialogOpen = false"
    />
    <CreateBranchDialog
      v-if="createBranchOpen"
      :wt="selectedPath"
      :sha="repo.summary?.sha ?? ''"
      @close="createBranchOpen = false"
    />
    <CreateWorktreeDialog
      v-if="createWorktreeFor"
      :repo-path="createWorktreeFor.repo.path"
      :distro="createWorktreeFor.repo.distro"
      :branches="createWorktreeFor.branches"
      @close="createWorktreeFor = null"
      @created="reload(createWorktreeFor!)"
    />
  </aside>
</template>

<style scoped>
/* Sizing is the parent Splitter panel's job (MainView.vue); this just fills
   it and scrolls its own content. */
.selector-panel {
  height: 100%;
  overflow: auto;
}
.panel-body {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.5rem 0.6rem;
}
.panel-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.4rem;
}
.repo-node {
  display: flex;
  flex-direction: column;
  min-width: 0;
  max-width: 100%;
}
.panel-row {
  display: flex;
  align-items: center;
  gap: 0.3em;
  min-width: 0;
}
.muted {
  font-size: 0.85em;
  color: var(--p-text-muted-color);
}
/* Takes only the space the rest of the row leaves, so it gives way first. */
.leftover {
  flex: 1 1 0;
}
</style>
