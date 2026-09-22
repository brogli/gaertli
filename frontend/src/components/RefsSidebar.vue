<script setup lang="ts">
import { computed, ref } from 'vue'
import Tree from 'openvue/tree'
import ContextMenu from 'openvue/contextmenu'
import type { TreeNode } from 'openvue/treenode'
import { useRefsStore } from '../stores/refs'
import { useLogStore } from '../stores/log'
import { useJobsStore } from '../stores/jobs'
import { useCoreCall, type CoreError } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { buildBranchMenuItems } from '../menus/branchMenu'
import ErrorBanner from './ErrorBanner.vue'
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
const updateKindPrefix = 'update-branch:'
const props = defineProps<{ wt: string }>()
const emit = defineEmits<{ 'create-worktree': [name: string, isRemote: boolean] }>()
const refs = useRefsStore()
const log = useLogStore()
const { error, run } = useCoreCall()

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
    node.key,
    data.current,
    isRemote,
    !!data.upstream,
    checkout,
    (name, remote) => emit('create-worktree', name, remote),
    updateBranch,
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

// The result banner for the latest update-branch job; a running retry
// hides the stale result.
const jobs = useJobsStore()
const updateResult = computed<{ branch: string; result?: UpdateBranchResult; error?: CoreError } | null>(() => {
  const job = jobs.latest((kind) => kind.startsWith(updateKindPrefix))
  if (!job) return null
  const branch = job.kind.slice(updateKindPrefix.length)
  if (job.state === 'done') return { branch, result: job.result as UpdateBranchResult }
  if (job.state === 'failed') return job.error ? { branch, error: job.error } : { branch }
  return null
})
</script>

<template>
  <aside class="refs-sidebar">
    <ErrorBanner v-if="error" :error="error" />
    <section>
      <h2>Branches</h2>
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
            <span>{{ node.label }}</span>
            <span v-if="isCurrent(node)" class="current-marker" data-test="current-marker"
              >current</span
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
              <span>{{ node.label }}</span>
              <span v-if="isCurrent(node)" class="current-marker" data-test="current-marker"
                >current</span
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
.current-marker {
  margin-left: 0.4em;
  font-size: 0.75em;
  color: var(--current-marker-color, #2f7d32);
}
</style>
