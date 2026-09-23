<script setup lang="ts">
import { watch } from 'vue'
import { onKeyStroke } from '@vueuse/core'
import { useCompareStore } from '../stores/compare'
import { useCoreCall } from '../composables/useCoreCall'
import { useOpenInEditor } from '../composables/useOpenInEditor'
import ChangedFileList from './ChangedFileList.vue'
import ErrorBanner from './ErrorBanner.vue'

// CompareWorkingView shows the compare store's workingCommit and the files
// that differ from the working tree; no swap. A file click selects it for
// the DiffView mounted alongside. git diff <sha> never lists untracked
// files, so the template says they are missing.
const props = defineProps<{ wt: string }>()
const compare = useCompareStore()
const { error, run } = useCoreCall()
const { openEditor, openEditorFallback } = useOpenInEditor(() => props.wt, run)

watch(
  () => compare.workingCommit,
  (commit) => {
    if (commit) void run(() => compare.loadAgainstWorking(props.wt, commit.sha))
  },
  { immediate: true },
)

onKeyStroke('Escape', () => compare.exit())
</script>

<template>
  <section class="compare-working-view">
    <header>
      <span class="from">{{ compare.workingCommit?.shortSha }}</span>
      {{ compare.workingCommit?.subject }}
      &rarr; working tree
      <button type="button" data-test="exit" @click="compare.exit">&times;</button>
    </header>
    <p class="hint">New, untracked files are not part of this comparison.</p>
    <ErrorBanner v-if="error" :error="error" @fallback="openEditorFallback" />
    <ChangedFileList
      :files="compare.files"
      :selected="compare.selectedFile"
      @select="compare.selectFile"
      @open-editor="openEditor"
    />
  </section>
</template>

<style scoped>
.hint {
  margin: 0.25rem 0;
  color: var(--p-text-muted-color);
  font-size: 0.85em;
}
</style>
