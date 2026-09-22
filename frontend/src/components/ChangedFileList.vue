<script setup lang="ts">
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'

// ChangedFileList renders one row per changed file (status, and oldPath ->
// path for a rename), shared by CommitDetailsPanel (FR-LOG-03), CompareView
// (FR-CMP-01) and CompareDiffView. Only CompareView listens for the
// double-click that opens the compare diff window (FR-CMP-02); open-editor
// (FR-ED-02) is every caller's "open this file in the editor" control.
defineProps<{ files: ChangedFile[]; selected?: ChangedFile | null }>()
const emit = defineEmits<{ select: [file: ChangedFile]; open: [file: ChangedFile]; 'open-editor': [file: ChangedFile] }>()
</script>

<template>
  <ul class="files">
    <li
      v-for="file in files"
      :key="file.path"
      :class="{ selected: file.path === selected?.path }"
      @click="emit('select', file)"
      @dblclick="emit('open', file)"
    >
      {{ file.status }}
      <template v-if="file.oldPath">{{ file.oldPath }} &rarr; </template>
      {{ file.path }}
      <button type="button" data-test="open-editor" @click.stop="emit('open-editor', file)">
        Open in editor
      </button>
    </li>
  </ul>
</template>
