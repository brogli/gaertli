<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Button from 'openvue/button'
import Tree, { type TreeExpandedKeys, type TreePassThroughMethodOptions } from 'openvue/tree'
import type { TreeNode } from 'openvue/treenode'
import { useI18n } from '../i18n'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { fileTree, folderKeys } from '../lib/fileTree'
import { iconClass } from '../lib/icons'
import { ICON_BUTTON } from '../lib/layout'
import FileStatusIcon from './FileStatusIcon.vue'

// ChangedFileTree shows changed files by folder in OpenVue's Tree, every
// folder unfolded whenever the files change. A click selects a file, a
// double-click or Enter opens it; a click or Enter on a folder folds or
// unfolds it.
const props = defineProps<{ files: ChangedFile[]; selected?: ChangedFile | null }>()
const emit = defineEmits<{
  select: [file: ChangedFile]
  open: [file: ChangedFile]
  'open-editor': [file: ChangedFile]
}>()
const { t } = useI18n()

const nodes = computed(() => fileTree(props.files))
const expandedKeys = ref<TreeExpandedKeys>({})
watch(nodes, (n) => (expandedKeys.value = folderKeys(n)), { immediate: true })

// The tree shows only the selected file as selected, so every click or
// Enter on a folder reaches onNodeSelect.
const selectionKeys = computed(() => (props.selected ? { [props.selected.path]: true } : {}))

function onNodeSelect(node: TreeNode): void {
  if (node.data) emit('select', node.data)
  else toggleFolder(node.key)
}

function toggleFolder(key: string): void {
  const { [key]: unfolded, ...others } = expandedKeys.value
  expandedKeys.value = unfolded ? others : { ...others, [key]: true }
}

function openFile(node: TreeNode): void {
  if (node.data) emit('open', node.data)
}

// The Tree has no open event: a node's row takes the double-click, the
// node itself (the focused element) the Enter.
const pt = {
  nodeContent: ({ context }: TreePassThroughMethodOptions) => ({
    onDblclick: () => openFile(context.node),
  }),
  node: ({ context }: TreePassThroughMethodOptions) => ({
    onKeydown: (event: KeyboardEvent) => {
      if (event.key === 'Enter' && event.target === event.currentTarget) openFile(context.node)
    },
  }),
}
</script>

<template>
  <Tree
    v-model:expanded-keys="expandedKeys"
    class="file-tree"
    :value="nodes"
    selection-mode="single"
    :selection-keys="selectionKeys"
    :pt="pt"
    @node-select="onNodeSelect"
  >
    <template #nodeicon="{ node }">
      <FileStatusIcon v-if="node.data" :status="node.data.status" />
      <i v-else :class="iconClass('folder')"></i>
    </template>
    <template #default="{ node }">
      <span class="name" :title="node.data?.path ?? node.label">
        <template v-if="node.data?.oldPath">{{ node.data.oldPath }} &rarr; </template
        >{{ node.label }}
      </span>
      <Button
        v-if="node.data"
        v-bind="ICON_BUTTON"
        :icon="iconClass('openEditor')"
        data-test="open-editor"
        :title="t('common.openInEditor')"
        :aria-label="t('common.openInEditor')"
        @click.stop="emit('open-editor', node.data)"
      />
    </template>
  </Tree>
</template>

<style scoped>
/* A row's name takes the room its button leaves and ends in an ellipsis;
   nothing scrolls sideways. */
.file-tree :deep(.p-tree-node-label) {
  flex: 1 1 auto;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 0.4em;
}
.name {
  flex: 1 1 auto;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
