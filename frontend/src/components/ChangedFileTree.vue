<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Tree, { type TreeExpandedKeys, type TreePassThroughMethodOptions } from 'openvue/tree'
import type { TreeNode } from 'openvue/treenode'
import type { ChangedFile } from '../../bindings/github.com/brogli/gaertli/internal/core'
import { fileTree, folderKeys } from '@/lib/fileTree'
import { iconClass } from '@/lib/icons'
import type { ChangedFileAction } from './changedFileAction'
import ChangedFileButton from './ChangedFileButton.vue'
import FileStatusIcon from './FileStatusIcon.vue'

// ChangedFileTree shows changed files by folder in OpenVue's Tree, every
// folder unfolded, or folded while folded is set, whenever the files or
// folded change. A click selects a file, a
// double-click or Enter opens it; a click or Enter on a folder folds or
// unfolds it. action, when set, adds a per-file button before the status
// icon (the changes view's stage/unstage); secondAction adds one after Open
// in editor (discard, reopen conflict) for each file it names an action for.
// A folder node gets no buttons.
const props = defineProps<{
  files: ChangedFile[]
  selected?: ChangedFile | null
  folded?: boolean
  action?: ChangedFileAction
  // | undefined, not just optional: ChangesView binds a per-group value that
  // may be undefined, which exactOptionalPropertyTypes tells from absent.
  secondAction?: ((file: ChangedFile) => ChangedFileAction | undefined) | undefined
}>()
const emit = defineEmits<{
  select: [file: ChangedFile]
  open: [file: ChangedFile]
  'open-editor': [file: ChangedFile]
  action: [file: ChangedFile]
  'second-action': [file: ChangedFile]
}>()

const nodes = computed(() => fileTree(props.files))
// Resolved once per file, keyed by path, rather than on every render of a row.
const secondActions = computed(
  () => new Map(props.files.map((file) => [file.path, props.secondAction?.(file)])),
)
const expandedKeys = ref<TreeExpandedKeys>({})
watch(
  [nodes, () => props.folded],
  ([n, folded]) => (expandedKeys.value = folded ? {} : folderKeys(n)),
  { immediate: true },
)

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
  // A row's name takes the room its button leaves and ends in an ellipsis;
  // nothing scrolls sideways.
  nodeLabel: 'flex-1 min-w-0 flex items-center gap-1.5',
}
</script>

<template>
  <Tree
    v-model:expanded-keys="expandedKeys"
    :value="nodes"
    selection-mode="single"
    :selection-keys="selectionKeys"
    :pt="pt"
    @node-select="onNodeSelect"
  >
    <template #nodeicon="{ node }">
      <i v-if="!node.data" :class="iconClass('folder')"></i>
    </template>
    <template #default="{ node }">
      <ChangedFileButton
        v-if="node.data && action"
        :kind="action"
        data-test="action"
        @click.stop="emit('action', node.data)"
      />
      <FileStatusIcon v-if="node.data" :status="node.data.status" />
      <span class="name flex-1 min-w-0 truncate" :title="node.data?.path ?? node.label">
        <template v-if="node.data?.oldPath">{{ node.data.oldPath }} &rarr; </template
        >{{ node.label }}
      </span>
      <template v-if="node.data">
        <ChangedFileButton
          kind="openEditor"
          data-test="open-editor"
          @click.stop="emit('open-editor', node.data)"
        />
        <ChangedFileButton
          v-if="secondActions.get(node.data.path)"
          :kind="secondActions.get(node.data.path)!"
          data-test="second-action"
          @click.stop="emit('second-action', node.data)"
        />
      </template>
    </template>
  </Tree>
</template>
