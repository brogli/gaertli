<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { Events, Window } from '@wailsio/runtime'
import { onKeyStroke } from '@vueuse/core'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { useCoreCall } from '../composables/useCoreCall'
import ErrorBanner from '../components/ErrorBanner.vue'
import ChangedFileList from '../components/ChangedFileList.vue'
import DiffView from '../components/DiffView.vue'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'

// CompareDiffView is the compare diff window's own root view (FR-CMP-02):
// a fresh JS realm that cannot reach the main window's Pinia
// compare store, so it fetches its own copy of the comparison directly.
const props = defineProps<{ wt: string; from: string; to: string; path: string; oldPath: string }>()

const files = ref<ChangedFile[]>([])
const current = ref({ path: props.path, oldPath: props.oldPath })
const { error, run } = useCoreCall()

onMounted(() => {
  void run(async () => {
    files.value = (await Facade.CompareCommits(props.wt, props.from, props.to)) ?? []
  })
})

const currentFile = computed(() => files.value.find((f) => f.path === current.value.path) ?? null)
const range = computed(() => ({ from: props.from, to: props.to, ...current.value }))

function selectFile(f: ChangedFile): void {
  current.value = { path: f.path, oldPath: f.oldPath ?? '' }
}
function onUpdateRange(r: { path: string; oldPath: string }): void {
  current.value = { path: r.path, oldPath: r.oldPath }
}

// FR-ED-02: open a listed file in the editor — comes for free from reusing
// ChangedFileList, not one of the three call sites the requirement names
// wired here so the control isn't a dead button in this window.
function openEditor(f: ChangedFile): void {
  void run(() => Facade.OpenFile(props.wt, f.path, 0))
}

// "compare:show-file" retargets this window to a newly double-clicked file
// of the same comparison (Registry.OpenOrFocusCompareDiff, Design, Events)
// without opening a second window.
let unlistenShowFile: (() => void) | undefined
onMounted(() => {
  unlistenShowFile = Events.On('compare:show-file', (ev) => {
    const { path, oldPath } = ev.data as { path: string; oldPath?: string }
    current.value = { path, oldPath: oldPath ?? '' }
  })
})
onBeforeUnmount(() => unlistenShowFile?.())

// Escape closes the window; the native close button needs no code.
onKeyStroke('Escape', () => void Window.Close())
</script>

<template>
  <div class="compare-diff-window">
    <ErrorBanner v-if="error" :error="error" />
    <aside class="file-list">
      <ChangedFileList :files="files" :selected="currentFile" @select="selectFile" @open-editor="openEditor" />
    </aside>
    <DiffView :wt="wt" :range="range" :files="files" @update:range="onUpdateRange" />
  </div>
</template>

<style scoped>
.compare-diff-window {
  display: flex;
  gap: 1em;
  height: 100%;
  min-height: 0;
}
.file-list {
  overflow: auto;
  flex: 0 0 auto;
}
</style>
