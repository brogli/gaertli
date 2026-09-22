<script setup lang="ts">
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'

// ChangedFileList renders one row per changed file (status, and oldPath ->
// path for a rename), shared by CommitDetailsPanel (FR-LOG-03) and
// CompareView (spec 21, FR-CMP-01).
defineProps<{ files: ChangedFile[]; selected?: ChangedFile | null }>()
const emit = defineEmits<{ select: [file: ChangedFile] }>()
</script>

<template>
  <ul class="files">
    <li
      v-for="file in files"
      :key="file.path"
      :class="{ selected: file.path === selected?.path }"
      @click="emit('select', file)"
    >
      {{ file.status }}
      <template v-if="file.oldPath">{{ file.oldPath }} &rarr; </template>
      {{ file.path }}
    </li>
  </ul>
</template>
