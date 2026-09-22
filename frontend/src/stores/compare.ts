import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { ChangedFile, LogCommit } from '../../bindings/github.com/brogli/nebegit/internal/core'

// compare tracks a Ctrl/Cmd-click two-commit selection (FR-CMP-01) and the
// changed files between them (FR-CMP-02). from/to are click order, not list
// order (FR-CMP-03: "from → to" is what the user picked, not chronology).
export const useCompareStore = defineStore('compare', () => {
  const selection = ref<LogCommit[]>([])
  const files = ref<ChangedFile[]>([])
  const selectedFile = ref<ChangedFile | null>(null)

  const active = computed(() => selection.value.length === 2)
  const from = computed(() => selection.value[0] ?? null)
  const to = computed(() => selection.value[1] ?? null)

  function toggleSelect(row: LogCommit): void {
    const i = selection.value.findIndex((r) => r.sha === row.sha)
    if (i >= 0) {
      selection.value.splice(i, 1) // clicking an already-picked row deselects it
    } else {
      selection.value.push(row)
      if (selection.value.length > 2) selection.value.shift() // FIFO: the two most recently picked
    }
    selectedFile.value = null
    files.value = []
  }

  // load throws on purpose: CompareView.vue owns the useCoreCall/
  // ErrorBanner wrapping, not the store.
  async function load(wt: string): Promise<void> {
    if (!from.value || !to.value) return
    files.value = (await Facade.CompareCommits(wt, from.value.sha, to.value.sha)) ?? []
  }

  function swap(): void {
    selection.value.reverse() // CompareView's own watcher re-fetches
  }

  function selectFile(f: ChangedFile): void {
    selectedFile.value = f
  }

  function exit(): void {
    selection.value = []
    files.value = []
    selectedFile.value = null
  }

  return { selection, active, from, to, files, selectedFile, toggleSelect, load, swap, selectFile, exit }
})
