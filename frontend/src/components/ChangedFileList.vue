<script setup lang="ts">
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'

// ChangedFileList renders one row per changed file (status, and oldPath ->
// path for a rename), shared by CommitDetailsPanel (FR-LOG-03) and
// CompareView (spec 21, FR-CMP-01). A double-click opens the compare diff
// window (spec 23, FR-CMP-02); CommitDetailsPanel and the diff window's own
// list don't listen for it — an unused optional emit, no dead code either
// place.
defineProps<{ files: ChangedFile[]; selected?: ChangedFile | null }>()
const emit = defineEmits<{ select: [file: ChangedFile]; open: [file: ChangedFile] }>()
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
    </li>
  </ul>
</template>
