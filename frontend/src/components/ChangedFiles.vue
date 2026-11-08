<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from '@/i18n'
import type { ChangedFile } from '../../bindings/github.com/brogli/gaertli/internal/core'
import { useFileTreeChoice } from '@/composables/useFileTreeChoice'
import ChangedFileList from './ChangedFileList.vue'
import ChangedFileTree from './ChangedFileTree.vue'
import FileTreeControls from './FileTreeControls.vue'

// ChangedFiles is the commit detail view's changed files: their count and
// the list/tree toggle on top, below them the files as a flat list or a
// folder tree, scrolling on their own. The last choice holds for every
// window.
defineProps<{ files: ChangedFile[]; selected: ChangedFile | null }>()
const emit = defineEmits<{
  select: [file: ChangedFile]
  open: [file: ChangedFile]
  'open-editor': [file: ChangedFile]
}>()
const { t } = useI18n()

const asTree = useFileTreeChoice()
const folded = ref(false)
const listeners = {
  select: (file: ChangedFile) => emit('select', file),
  open: (file: ChangedFile) => emit('open', file),
  'open-editor': (file: ChangedFile) => emit('open-editor', file),
}
</script>

<template>
  <div class="changed-files flex flex-1 flex-col min-h-0 gap-1">
    <div class="flex flex-none items-center justify-between gap-1.5">
      <h3>{{ t('commitDetailView.changedFiles', { n: files.length }, files.length) }}</h3>
      <FileTreeControls v-model:as-tree="asTree" v-model:folded="folded" />
    </div>
    <div class="file-scroll flex-1 min-h-0 overflow-y-auto overflow-x-hidden">
      <ChangedFileTree
        v-if="asTree"
        :files="files"
        :selected="selected"
        :folded="folded"
        v-on="listeners"
      />
      <ChangedFileList v-else :files="files" :selected="selected" v-on="listeners" />
    </div>
  </div>
</template>
