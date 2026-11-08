import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'
import type { ChangedFile, LogCommit } from '../../bindings/github.com/brogli/gaertli/internal/core'
import { useLogStore } from './log'

// compare reads the log store's one selection: exactly two commits is a
// comparison, from the first selected to the second. swap() reverses the
// log selection itself, so from and to (and so the log view's marks, which
// are keyed on them) move to the other commit along with the comparison
// header's hashes.
export const useCompareStore = defineStore('compare', () => {
  const log = useLogStore()
  const files = ref<ChangedFile[]>([])
  const selectedFile = ref<ChangedFile | null>(null)
  // workingCommit is set instead of a two-commit comparison for "Compare
  // with working tree": one commit, paired with the working tree rather
  // than a second commit.
  const workingCommit = ref<LogCommit | null>(null)

  const active = computed(() => log.selection.length === 2)
  const from = computed(() => log.selection[0] ?? null)
  const to = computed(() => log.selection[1] ?? null)

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
  // replacing any selection.
  function startAgainstWorking(row: LogCommit): void {
    exit()
    workingCommit.value = row
  }

  function swap(): void {
    log.swapSelection() // CompareView's watcher re-fetches
  }

  function selectFile(f: ChangedFile): void {
    selectedFile.value = f
  }

  // dropWorkingCompare exits a working-tree comparison without touching the
  // log selection: a plain or Shift-click on a row already replaces that
  // itself.
  function dropWorkingCompare(): void {
    workingCommit.value = null
    files.value = []
    selectedFile.value = null
  }

  // exit stops comparing outright: no commit stays selected.
  function exit(): void {
    log.clearSelection()
    dropWorkingCompare()
  }

  return {
    active,
    from,
    to,
    files,
    selectedFile,
    workingCommit,
    load,
    loadAgainstWorking,
    startAgainstWorking,
    swap,
    selectFile,
    dropWorkingCompare,
    exit,
  }
})
