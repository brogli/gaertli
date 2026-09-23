import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { ChangedFile, LogCommit } from '../../bindings/github.com/brogli/nebegit/internal/core'

// compare tracks a Ctrl/Cmd-click two-commit selection and the changed
// files between them. from/to are click order, not list order.
export const useCompareStore = defineStore('compare', () => {
  const selection = ref<LogCommit[]>([])
  const files = ref<ChangedFile[]>([])
  const selectedFile = ref<ChangedFile | null>(null)
  // workingCommit is set instead of selection for "Compare with working
  // tree": one commit, paired with the working tree rather than a second
  // commit.
  const workingCommit = ref<LogCommit | null>(null)

  const active = computed(() => selection.value.length === 2)
  const from = computed(() => selection.value[0] ?? null)
  const to = computed(() => selection.value[1] ?? null)

  function toggleSelect(row: LogCommit): void {
    workingCommit.value = null // a two-commit selection replaces a working-tree compare
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

  // load throws: CompareView.vue reports the error
  async function load(wt: string): Promise<void> {
    if (!from.value || !to.value) return
    files.value = (await Facade.CompareCommits(wt, from.value.sha, to.value.sha)) ?? []
  }

  // loadAgainstWorking throws: CompareWorkingView.vue reports the error
  async function loadAgainstWorking(wt: string, sha: string): Promise<void> {
    files.value = (await Facade.CompareWithWorking(wt, sha)) ?? []
  }

  // startAgainstWorking begins a "Compare with working tree" for row,
  // replacing any two-commit selection.
  function startAgainstWorking(row: LogCommit): void {
    selection.value = []
    selectedFile.value = null
    files.value = []
    workingCommit.value = row
  }

  function swap(): void {
    selection.value.reverse() // CompareView's watcher re-fetches
  }

  function selectFile(f: ChangedFile): void {
    selectedFile.value = f
  }

  function exit(): void {
    selection.value = []
    files.value = []
    selectedFile.value = null
    workingCommit.value = null
  }

  return {
    selection,
    active,
    from,
    to,
    files,
    selectedFile,
    workingCommit,
    toggleSelect,
    load,
    loadAgainstWorking,
    startAgainstWorking,
    swap,
    selectFile,
    exit,
  }
})
