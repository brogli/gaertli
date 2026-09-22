<script setup lang="ts">
import { watch } from 'vue'
import { useCompareStore } from '../stores/compare'
import { onKeyStroke } from '@vueuse/core'
import { useCoreCall } from '../composables/useCoreCall'
import { AppService, Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import ChangedFileList from './ChangedFileList.vue'
import ErrorBanner from './ErrorBanner.vue'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'

// CompareView shows the compare store's two selected commits (FR-CMP-01),
// their direction and a swap action (FR-CMP-03), and their changed files
// (FR-CMP-02). Escape leaves compare mode; a double-click on a file opens
// its own diff window.
const props = defineProps<{ wt: string }>()
const compare = useCompareStore()
const { error, run } = useCoreCall()

function openDiffWindow(f: ChangedFile): void {
  if (!compare.from || !compare.to) return
  void AppService.OpenCompareDiff(props.wt, compare.from.sha, compare.to.sha, f.path, f.oldPath ?? '')
}

// FR-ED-02: open a compared file in the editor.
function openEditor(f: ChangedFile): void {
  void run(() => Facade.OpenFile(props.wt, f.path, 0))
}

watch(
  [() => compare.from, () => compare.to],
  () => {
    if (compare.active) void run(() => compare.load(props.wt))
  },
  { immediate: true },
)

onKeyStroke('Escape', () => compare.exit())
</script>

<template>
  <section class="compare-view">
    <header>
      <span class="from">{{ compare.from?.shortSha }}</span>
      &rarr;
      <span class="to">{{ compare.to?.shortSha }}</span>
      <button type="button" data-test="swap" @click="compare.swap">Swap</button>
      <button type="button" data-test="exit" @click="compare.exit">&times;</button>
    </header>
    <ErrorBanner v-if="error" :error="error" />
    <ChangedFileList
      :files="compare.files"
      :selected="compare.selectedFile"
      @select="compare.selectFile"
      @open="openDiffWindow"
      @open-editor="openEditor"
    />
  </section>
</template>
