<script setup lang="ts">
import { computed } from 'vue'
import Button from 'openvue/button'
import { useLocalStorage } from '@vueuse/core'
import { useI18n } from '../i18n'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { iconClass } from '../lib/icons'
import { ICON_BUTTON } from '../lib/layout'
import ChangedFileList from './ChangedFileList.vue'
import ChangedFileTree from './ChangedFileTree.vue'

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

const asTree = useLocalStorage('nebegit.changedFiles.tree', false)
const toggleLabel = computed(() =>
  asTree.value ? t('commitDetailView.showAsList') : t('commitDetailView.showAsTree'),
)
const listeners = {
  select: (file: ChangedFile) => emit('select', file),
  open: (file: ChangedFile) => emit('open', file),
  'open-editor': (file: ChangedFile) => emit('open-editor', file),
}
</script>

<template>
  <div class="changed-files">
    <div class="files-header">
      <h3>{{ t('commitDetailView.changedFiles', { n: files.length }, files.length) }}</h3>
      <Button
        v-bind="ICON_BUTTON"
        :text="!asTree"
        :icon="iconClass(asTree ? 'fileTree' : 'fileList')"
        data-test="file-tree-toggle"
        :title="toggleLabel"
        :aria-label="toggleLabel"
        :aria-pressed="asTree"
        @click="asTree = !asTree"
      />
    </div>
    <div class="file-scroll">
      <ChangedFileTree v-if="asTree" :files="files" :selected="selected" v-on="listeners" />
      <ChangedFileList v-else :files="files" :selected="selected" v-on="listeners" />
    </div>
  </div>
</template>

<style scoped>
/* The header stays; the files below it scroll. */
.changed-files {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}
.files-header {
  flex: none;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.4rem;
}
.file-scroll {
  flex: 1 1 auto;
  min-height: 0;
  overflow-x: hidden;
  overflow-y: auto;
}
</style>
