<script setup lang="ts">
import { computed, ref } from 'vue'
import Tree from 'openvue/tree'
import ContextMenu from 'openvue/contextmenu'
import type { TreeNode } from 'openvue/treenode'
import { useRefsStore } from '../stores/refs'
import { useLogStore } from '../stores/log'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { buildBranchMenuItems } from '../menus/branchMenu'
import ErrorBanner from './ErrorBanner.vue'
import { buildRefTree, type RefTreeNode } from './refTree'

// RefsSidebar shows the open worktree's branches, remotes, and tags as
// OpenVue trees (FR-LOG-01). No Worktrees section here (spec 50).
const props = defineProps<{ wt: string }>()
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

// The current-branch marker (data.current) is kept independent of Tree's
// own selectionKeys, which is reserved for the transient "user just
// clicked this" state, not the persistent "this is HEAD" fact (Design).
function isCurrent(node: TreeNode): boolean {
  return (node as RefTreeNode).data?.current === true
}

// A leaf carries data.sha; a folder node carries none and is a no-op
// click here (Tree still handles its own expand/collapse).
function onNodeSelect(node: TreeNode): void {
  const sha = (node as RefTreeNode).data?.sha
  if (sha) void log.jumpTo(sha)
}

// Right-click a branch leaf (local or remote): FR-BR-05's "Checkout" item.
// A folder node carries no data and offers no menu. node.key is the
// branch's own name (buildRefTree's namespaced path — for a remote branch,
// already the short name with no remote prefix, since each remote's own
// leaf list already excludes it).
const menuRef = ref<InstanceType<typeof ContextMenu>>()
const menuItems = ref<ReturnType<typeof buildBranchMenuItems>>([])

function onNodeContextMenu(event: MouseEvent, node: TreeNode): void {
  const data = (node as RefTreeNode).data
  if (!data) return
  menuItems.value = buildBranchMenuItems(node.key, data.current, checkout)
  menuRef.value?.show(event)
}

// Checkout is a stateless write action reported through ErrorBanner alone
// (WorktreeView.vue's own refresh() pattern) — the refs/log stores reload
// themselves from the resulting repo:changed event, no store round-trip
// needed here.
function checkout(name: string): void {
  void run(() => Facade.Checkout(props.wt, name))
}
</script>

<template>
  <aside class="refs-sidebar">
    <ErrorBanner v-if="error" :error="error" />
    <section>
      <h2>Branches</h2>
      <!-- OpenVue's Tree needs a v-if guard, not just an empty `value`:
           its empty-message slot reads global config that this app never
           installs (Own code vs library — no ThemeProvider is wired,
           Design doesn't call for one), and an empty repo (case 9,
           Verification) must render with no crash. -->
      <Tree
        v-if="branchNodes.length"
        :value="branchNodes"
        selection-mode="single"
        @node-select="onNodeSelect"
      >
        <template #default="{ node }">
          <span class="ref-node" @contextmenu.prevent="onNodeContextMenu($event, node)">
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
            <span class="ref-node" @contextmenu.prevent="onNodeContextMenu($event, node)">
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
  </aside>
</template>

<style scoped>
.current-marker {
  margin-left: 0.4em;
  font-size: 0.75em;
  color: var(--current-marker-color, #2f7d32);
}
</style>
