<script setup lang="ts">
import { computed, ref } from 'vue'
import Tree from 'openvue/tree'
import ContextMenu from 'openvue/contextmenu'
import type { TreeNode } from 'openvue/treenode'
import { useRefsStore } from '../stores/refs'
import { useLogStore } from '../stores/log'
import { useJobsStore } from '../stores/jobs'
import { useCoreCall, type CoreError } from '../composables/useCoreCall'
import { useConfirmAction } from '../composables/useConfirmAction'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { buildBranchMenuItems } from '../menus/branchMenu'
import ErrorBanner from './ErrorBanner.vue'
import RenameBranchDialog from './RenameBranchDialog.vue'
import { buildRefTree, type RefTreeNode } from './refTree'

// UpdateBranchResult is core.UpdateBranchResult, UpdateBranch's
// JobInfo.result; the bindings type JobInfo.result as `any`.
interface UpdateBranchResult {
  outcome: 'updated' | 'upToDate'
  from: string
  to: string
  commits: number
}

// RefsSidebar shows the open worktree's branches, remotes, and tags as
// OpenVue trees.
const props = defineProps<{ wt: string }>()
const emit = defineEmits<{ 'create-worktree': [name: string, isRemote: boolean] }>()
const refs = useRefsStore()
const log = useLogStore()
const { error, run } = useCoreCall()
const { confirmAction } = useConfirmAction()
const renameTarget = ref<string | null>(null)

const branchNodes = computed(() => buildRefTree(refs.refs?.branches ?? []))
const tagNodes = computed(() => buildRefTree(refs.refs?.tags ?? []))
const remoteGroups = computed(() =>
  (refs.refs?.remotes ?? []).map((remote) => ({
    name: remote.name,
    nodes: buildRefTree(remote.branches ?? []),
  })),
)

// The current-branch marker is independent of Tree's selectionKeys, which
// hold the transient "just clicked" state.
function isCurrent(node: TreeNode): boolean {
  return (node as RefTreeNode).data?.current === true
}

// A leaf carries data.sha; a folder node carries none and is a no-op
// click here (Tree still handles its own expand/collapse).
function onNodeSelect(node: TreeNode): void {
  const sha = (node as RefTreeNode).data?.sha
  if (sha) void log.jumpTo(sha)
}

// Right-click a branch leaf (local or remote) for its menu; a folder node
// has none. node.key is the branch name, without the remote prefix for a
// remote branch.
const menuRef = ref<InstanceType<typeof ContextMenu>>()
const menuItems = ref<ReturnType<typeof buildBranchMenuItems>>([])

function onNodeContextMenu(event: MouseEvent, node: TreeNode, isRemote: boolean): void {
  const data = (node as RefTreeNode).data
  if (!data) return
  menuItems.value = buildBranchMenuItems(
    { name: node.key, current: data.current, isRemote, hasUpstream: !!data.upstream },
    {
      checkout,
      createWorktree: (name, remote) => emit('create-worktree', name, remote),
      updateBranch,
      openRename,
      deleteBranch: (name) => void deleteBranch(name),
    },
  )
  menuRef.value?.show(event)
}

// The refs and log stores reload themselves from the resulting
// repo:changed event.
function checkout(name: string): void {
  void run(() => Facade.Checkout(props.wt, name))
}

// updateBranch starts the job; its outcome shows in updateResult below.
// A synchronous refusal (no upstream, etc.) shows in error/ErrorBanner
// instead of ever starting a job.
function updateBranch(name: string): void {
  void run(() => Facade.UpdateBranch(props.wt, name))
}

function openRename(name: string): void {
  renameTarget.value = name
}

// A merge-safe delete needs no confirmation. An Unmerged refusal re-asks,
// naming git's own reason, then retries forced. Any other refusal (the
// branch in use elsewhere) surfaces via ErrorBanner as-is, no retry
// offered.
async function deleteBranch(name: string): Promise<void> {
  await run(() => Facade.DeleteBranch(props.wt, name, false))
  if (!error.value || error.value.code !== 'UNMERGED_BRANCH') return
  const reason = error.value.message
  error.value = null // the confirmation carries the reason; no banner as well
  confirmAction(
    'Delete unmerged branch',
    `${reason} Delete anyway? Its commits will become unreachable.`,
    () => {
      void run(() => Facade.DeleteBranch(props.wt, name, true))
    },
  )
}

// The result banner for the latest update-branch job; a running retry
// hides the stale result.
const jobs = useJobsStore()
const updateResult = computed<{
  branch: string
  result?: UpdateBranchResult
  error?: CoreError
} | null>(() => {
  const job = jobs.latest((kind) => kind === 'update-branch')
  if (!job) return null
  const branch = job.branch ?? ''
  if (job.state === 'done') return { branch, result: job.result as UpdateBranchResult }
  if (job.state === 'failed') return job.error ? { branch, error: job.error } : { branch }
  return null
})

// Per-branch selection for updating several branches at once, independent
// of Tree's own selection-mode="single" (click-to-jump): a checkbox is
// offered only where buildBranchMenuItems' own "Update from upstream" item
// is (a local, non-current branch with an upstream).
const selected = ref<Set<string>>(new Set())

function eligibleForUpdate(node: TreeNode): boolean {
  const data = (node as RefTreeNode).data
  return !!data && !data.current && !!data.upstream
}

// Vue's reactivity instruments Set/Map methods directly, so mutating the
// ref's own Set in place (no rebuild-and-reassign) is enough to notify.
function toggleSelected(name: string, checked: boolean): void {
  if (checked) selected.value.add(name)
  else selected.value.delete(name)
}

async function updateSelected(): Promise<void> {
  await Promise.all(
    [...selected.value].map((name) => run(() => Facade.UpdateBranch(props.wt, name))),
  )
  selected.value.clear()
}
</script>

<template>
  <aside class="refs-sidebar">
    <ErrorBanner v-if="error" :error="error" />
    <section>
      <h2>Branches</h2>
      <button v-if="selected.size >= 2" type="button" @click="updateSelected">
        Update {{ selected.size }} branches
      </button>
      <!-- v-if: an empty Tree's empty-message slot reads global config this
           app never installs -->
      <Tree
        v-if="branchNodes.length"
        :value="branchNodes"
        selection-mode="single"
        @node-select="onNodeSelect"
      >
        <template #default="{ node }">
          <span class="ref-node" @contextmenu.prevent="onNodeContextMenu($event, node, false)">
            <input
              v-if="eligibleForUpdate(node)"
              type="checkbox"
              data-test="update-select"
              :checked="selected.has(node.key)"
              :title="`Select ${node.label} to update`"
              :aria-label="`Select ${node.label} to update`"
              @click.stop
              @change="toggleSelected(node.key, ($event.target as HTMLInputElement).checked)"
            />
            <span class="ref-name ellipsis" :title="node.label">{{ node.label }}</span>
            <span
              v-if="isCurrent(node)"
              class="current-marker"
              data-test="current-marker"
              title="Current branch"
              aria-label="Current branch"
              >●</span
            >
          </span>
        </template>
      </Tree>
    </section>
    <section v-if="remoteGroups.length">
      <h2>Remotes</h2>
      <div v-for="group in remoteGroups" :key="group.name" class="remote-group">
        <h3>{{ group.name }}</h3>
        <Tree
          v-if="group.nodes.length"
          :value="group.nodes"
          selection-mode="single"
          @node-select="onNodeSelect"
        >
          <template #default="{ node }">
            <span class="ref-node" @contextmenu.prevent="onNodeContextMenu($event, node, true)">
              <span class="ref-name ellipsis" :title="node.label">{{ node.label }}</span>
              <span
                v-if="isCurrent(node)"
                class="current-marker"
                data-test="current-marker"
                title="Current branch"
                aria-label="Current branch"
                >●</span
              >
            </span>
          </template>
        </Tree>
      </div>
    </section>
    <section>
      <h2>Tags</h2>
      <Tree
        v-if="tagNodes.length"
        :value="tagNodes"
        selection-mode="single"
        @node-select="onNodeSelect"
      />
    </section>
    <ContextMenu ref="menuRef" :model="menuItems" />
    <RenameBranchDialog
      v-if="renameTarget !== null"
      :wt="wt"
      :name="renameTarget"
      @close="renameTarget = null"
    />
    <div v-if="updateResult" data-test="update-result">
      <ErrorBanner v-if="updateResult.error" :error="updateResult.error" />
      <p v-else-if="updateResult.result?.outcome === 'updated'">
        Updated "{{ updateResult.branch }}": {{ updateResult.result.commits }} commit(s), now at
        {{ updateResult.result.to.slice(0, 7) }}.
      </p>
      <p v-else>"{{ updateResult.branch }}" is already up to date.</p>
    </div>
  </aside>
</template>

<style scoped>
.refs-sidebar {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.5rem 0.6rem;
  height: 100%;
  overflow: auto;
}
.remote-group h3 {
  margin-top: 0.35rem;
}
.ref-node {
  display: flex;
  align-items: center;
  gap: 0.3em;
  min-width: 0;
}
.current-marker {
  flex-shrink: 0;
  font-size: 0.75em;
  color: var(--p-primary-color);
}
</style>
