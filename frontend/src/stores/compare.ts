import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { ChangedFile, LogCommit } from '../../bindings/github.com/brogli/nebegit/internal/core'

interface Pick {
  row: LogCommit
  index: number
}

// compare tracks a Ctrl/Cmd-click two-commit selection and the changed
// files between them. Each pick keeps the row's log index at selection
// time: git log lists newest first, so the higher index is the older
// commit. from/to default to older/newer and swap with the reversed flag,
// whatever the click order was.
export const useCompareStore = defineStore('compare', () => {
  const picks = ref<Pick[]>([])
  const reversed = ref(false)
  const files = ref<ChangedFile[]>([])
  const selectedFile = ref<ChangedFile | null>(null)
  // workingCommit is set instead of a pick pair for "Compare with working
  // tree": one commit, paired with the working tree rather than a second
  // commit.
  const workingCommit = ref<LogCommit | null>(null)

  const selection = computed(() => picks.value.map((p) => p.row))
  const active = computed(() => picks.value.length === 2)

  const older = computed(() => orderedPicks(picks.value)?.[0].row ?? null)
  const newer = computed(() => orderedPicks(picks.value)?.[1].row ?? null)
  const from = computed(() => (reversed.value ? newer.value : older.value))
  const to = computed(() => (reversed.value ? older.value : newer.value))

  // toggleSelect's index is a snapshot: an outside commit can reload the log
  // and shift every row between the first pick and the second. Once the
  // pair is complete, resolveIndexes re-reads both from the current log so
  // older/newer never rests on two indexes from different reloads.
  function toggleSelect(wt: string, row: LogCommit, index: number): void {
    workingCommit.value = null // a two-commit selection replaces a working-tree compare
    const i = picks.value.findIndex((p) => p.row.sha === row.sha)
    if (i >= 0) {
      picks.value.splice(i, 1) // clicking an already-picked row deselects it
    } else {
      picks.value.push({ row, index })
      if (picks.value.length > 2) picks.value.shift() // FIFO: the two most recently picked
    }
    reversed.value = false
    selectedFile.value = null
    files.value = []
    if (picks.value.length === 2) void resolveIndexes(wt)
  }

  // resolveIndexes looks up each pick's current row index and applies it if
  // that pick is still selected: a later toggleSelect may have replaced it
  // while this was in flight.
  async function resolveIndexes(wt: string): Promise<void> {
    const shas = picks.value.map((p) => p.row.sha)
    const results = await Promise.all(shas.map((sha) => Facade.LogIndexOf(wt, sha)))
    results.forEach((result, i) => {
      const pick = picks.value.find((p) => p.row.sha === shas[i])
      if (pick && result.found) pick.index = result.index
    })
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
    picks.value = []
    selectedFile.value = null
    files.value = []
    workingCommit.value = row
  }

  function swap(): void {
    reversed.value = !reversed.value // CompareView's watcher re-fetches
  }

  function selectFile(f: ChangedFile): void {
    selectedFile.value = f
  }

  function exit(): void {
    picks.value = []
    reversed.value = false
    files.value = []
    selectedFile.value = null
    workingCommit.value = null
  }

  return {
    selection,
    active,
    older,
    newer,
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

// orderedPicks is [older, newer]: the higher log index is the older commit.
function orderedPicks(picks: Pick[]): [Pick, Pick] | null {
  if (picks.length !== 2) return null
  const [a, b] = picks as [Pick, Pick]
  return a.index > b.index ? [a, b] : [b, a]
}
