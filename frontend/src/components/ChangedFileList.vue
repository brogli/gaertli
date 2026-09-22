<script setup lang="ts">
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'

// ChangedFileList renders one row per changed file (status, and oldPath ->
// path for a rename). actionLabel, when set, adds a per-row action button
// (the changes panel's stage/unstage).
defineProps<{ files: ChangedFile[]; selected?: ChangedFile | null; actionLabel?: string }>()
const emit = defineEmits<{
  select: [file: ChangedFile]
  open: [file: ChangedFile]
  'open-editor': [file: ChangedFile]
  action: [file: ChangedFile]
}>()
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
      <button v-if="actionLabel" type="button" data-test="action" @click.stop="emit('action', file)">
        {{ actionLabel }}
      </button>
    </li>
  </ul>
</template>
