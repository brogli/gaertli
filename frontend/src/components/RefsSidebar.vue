<script setup lang="ts">
import { computed } from 'vue'
import Tree from 'openvue/tree'
import type { TreeNode } from 'openvue/treenode'
import { useRefsStore } from '../stores/refs'
import { useLogStore } from '../stores/log'
import { buildRefTree, type RefTreeNode } from './refTree'

// RefsSidebar shows the open worktree's branches, remotes, and tags as
// OpenVue trees (FR-LOG-01). No Worktrees section here (spec 50).
const refs = useRefsStore()
const log = useLogStore()

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
</script>

<template>
  <aside class="refs-sidebar">
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
          <span>{{ node.label }}</span>
          <span v-if="isCurrent(node)" class="current-marker" data-test="current-marker"
            >current</span
          >
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
        />
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
  </aside>
</template>

<style scoped>
.current-marker {
  margin-left: 0.4em;
  font-size: 0.75em;
  color: var(--current-marker-color, #2f7d32);
}
</style>
