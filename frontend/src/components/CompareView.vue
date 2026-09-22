<script setup lang="ts">
import { onBeforeUnmount, onMounted, watch } from 'vue'
import { useCompareStore } from '../stores/compare'
import { useCoreCall } from '../composables/useCoreCall'
import ChangedFileList from './ChangedFileList.vue'
import ErrorBanner from './ErrorBanner.vue'

// CompareView shows the compare store's two selected commits (FR-CMP-01),
// their direction and a swap action (FR-CMP-03), and their changed files
// (FR-CMP-02). Escape leaves compare mode.
const props = defineProps<{ wt: string }>()
const compare = useCompareStore()
const { error, run } = useCoreCall()

watch(
  [() => compare.from, () => compare.to],
  () => {
    if (compare.active) void run(() => compare.load(props.wt))
  },
  { immediate: true },
)

function onKeydown(e: KeyboardEvent): void {
  if (e.key === 'Escape') compare.exit()
}
onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
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
    />
  </section>
</template>
