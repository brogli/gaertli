<script setup lang="ts">
import { computed } from 'vue'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { basename, parentPath } from '@/lib/path'
import type { ChangedFileAction } from './changedFileAction'
import ChangedFileButton from './ChangedFileButton.vue'
import FileStatusIcon from './FileStatusIcon.vue'

// ChangedFileList renders one row per changed file (status, and oldPath ->
// path for a rename). A click selects a row; a double-click, or Enter on
// the focused row, opens it. action, when set, adds a per-row action button
// (the changes view's stage/unstage); secondAction adds a second one
// (discard). Each names its icon and its own translated label.
const props = defineProps<{
  files: ChangedFile[]
  selected?: ChangedFile | null
  action?: ChangedFileAction
  // | undefined, not just optional: ChangesView binds a per-group value that
  // may be undefined, which exactOptionalPropertyTypes tells from absent.
  secondAction?: ChangedFileAction | undefined
}>()
const emit = defineEmits<{
  select: [file: ChangedFile]
  open: [file: ChangedFile]
  'open-editor': [file: ChangedFile]
  action: [file: ChangedFile]
  'second-action': [file: ChangedFile]
}>()

// rows splits each path into its directory (rendered muted) and basename,
// once per file rather than on every access.
const rows = computed(() =>
  props.files.map((file) => ({
    file,
    dir: parentPath(file.path),
    base: basename(file.path),
  })),
)
</script>

<template>
  <ul class="files">
    <li
      v-for="{ file, dir, base } in rows"
      :key="file.path"
      class="flex items-center gap-1.5 rounded py-0.5 px-1 cursor-pointer"
      :class="file.path === selected?.path ? 'selected bg-highlight' : 'hover:bg-emphasis'"
      tabindex="0"
      @click="emit('select', file)"
      @dblclick="emit('open', file)"
      @keydown.enter.self="emit('open', file)"
    >
      <ChangedFileButton
        v-if="action"
        :kind="action"
        data-test="action"
        @click.stop="emit('action', file)"
      />
      <FileStatusIcon :status="file.status" />
      <span class="path flex-1 min-w-0 truncate" :title="file.path">
        <template v-if="file.oldPath">{{ file.oldPath }} &rarr; </template>
        <span v-if="dir" class="text-muted-color">{{ dir }}/</span>{{ base }}
      </span>
      <span class="actions flex flex-none gap-1">
        <ChangedFileButton
          kind="openEditor"
          data-test="open-editor"
          @click.stop="emit('open-editor', file)"
        />
        <ChangedFileButton
          v-if="secondAction"
          :kind="secondAction"
          data-test="second-action"
          @click.stop="emit('second-action', file)"
        />
      </span>
    </li>
  </ul>
</template>
