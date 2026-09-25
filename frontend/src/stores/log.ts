import { defineStore } from 'pinia'
import { computed, reactive, ref } from 'vue'
import { watchDebounced } from '@vueuse/core'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import {
  ChangeScope,
  type ChangedFile,
  type CommitDetails,
  type LogCommit,
  type LogFilter,
} from '../../bindings/github.com/brogli/nebegit/internal/core'
import { onRepoChanged, onWindowEvent } from '../lib/windowEvents'
import type { CoreError } from '../composables/useCoreCall'

type RowView = {
  cache: Map<number, LogCommit>
  generation: number
  pending: Map<string, Promise<LogCommit[]>>
}

function newRowView(): RowView {
  return { cache: reactive(new Map<number, LogCommit>()), generation: 0, pending: new Map() }
}

function resetView(view: RowView, generation = 0): void {
  view.cache.clear()
  view.pending.clear()
  view.generation = generation
}

// cachedWindow returns rows [from, to) if every one is cached, else null.
function cachedWindow(view: RowView, from: number, to: number): LogCommit[] | null {
  const rows: LogCommit[] = []
  for (let i = from; i < to; i++) {
    const row = view.cache.get(i)
    if (!row) return null
    rows.push(row)
  }
  return rows
}

// log holds one repo session's commit log: loaded once per LoadLog, row
// windows cached by index, each commit's details cached by sha. It also
// holds the worktree's filtered view: filterLoaded counts the filtered
// stream's rows beside the base loaded count. Each view keeps the error its
// last load failed with, until a newer progress of that view.
export const useLogStore = defineStore('log', () => {
  const loaded = ref(0)
  const filter = ref<LogFilter>({})
  const filterLoaded = ref(0)
  const baseError = ref<CoreError | null>(null)
  const filterError = ref<CoreError | null>(null)
  const selectedRow = ref<LogCommit | null>(null)
  const details = ref<CommitDetails | null>(null)
  const selectedFile = ref<ChangedFile | null>(null)
  const pendingScrollTo = ref<number | null>(null)

  // selection is the one ordered selection Ctrl-click and Shift-click both
  // build: Ctrl-click toggles one commit in it, Shift-click replaces it with
  // the range from the anchor to the clicked row. anchorIndex is the log
  // index Shift-click extends from: the last row clicked plainly or with
  // Ctrl.
  const selection = ref<LogCommit[]>([])
  const anchorIndex = ref<number | null>(null)

  // The search: query is the text or hash typed into the filter bar's
  // search field, matches the row indices it resolved to (in the active
  // view, log order), current the position in matches the log is showing.
  const query = ref('')
  const matches = ref<number[]>([])
  const current = ref(0)

  const detailsCache = new Map<string, CommitDetails>()
  let wt = ''
  let unlisten: (() => void)[] = []

  // filterActive is !core.LogFilter.Empty(). activeLoaded is the row count
  // LogList's virtualizer shows.
  const filterActive = computed(() => {
    const f = filter.value
    return !!(f.author || (f.branches && f.branches.length > 0) || f.path)
  })
  const activeLoaded = computed(() => (filterActive.value ? filterLoaded.value : loaded.value))
  const error = computed(() => (filterActive.value ? filterError.value : baseError.value))
  // flat hides the graph column while an author search narrows the view,
  // the same rule as the Go side.
  const flat = computed(() => !!filter.value.author)

  // One row-window cache per view (base log, filtered view): clearing a
  // filter shows the base rows already fetched, and a slow response lands
  // in its own view's cache. generation is LogRowsResult.Generation as last
  // seen for that view, and for the base view also log:progress's: a higher
  // one means the view restarted from row 0, a lower one is a response from
  // before that. pending dedupes a window already in flight.
  const baseView = newRowView()
  const filterView = newRowView()
  const activeView = computed(() => (filterActive.value ? filterView : baseView))
  const rowCache = computed(() => activeView.value.cache)

  // knownAuthors is a best-effort suggestion list for the author filter's
  // AutoComplete: every author seen among the rows fetched so far, not a
  // complete list of the repo's authors.
  const authors = reactive(new Set<string>())
  const knownAuthors = computed(() => [...authors].sort())

  async function load(id: string): Promise<void> {
    wt = id
    loaded.value = 0
    filter.value = {}
    filterLoaded.value = 0
    baseError.value = null
    filterError.value = null
    clearDetails()
    resetView(baseView)
    resetView(filterView)
    authors.clear()
    detailsCache.clear()
    clearQuery()

    unlisten.forEach((off) => off())
    unlisten = [
      onWindowEvent('log:progress', ({ data: progress }) => {
        // A reload restarts its stream even when the row count stays the
        // same, base or filtered: either can drop a commit a reset left
        // unreachable without changing how many rows are loaded. The
        // active view's own restart, and its own done, each re-run the
        // search: a restart's rows are wholly new, and rows that streamed
        // in after an earlier search can hold matches that one missed.
        const view = progress.filtered ? filterView : baseView
        const freshGeneration = progress.generation !== view.generation
        if (freshGeneration) resetView(view, progress.generation)
        if (progress.filtered) {
          filterLoaded.value = progress.loaded
          filterError.value = progress.error ?? null
        } else {
          loaded.value = progress.loaded
          baseError.value = progress.error ?? null
        }
        if (view === activeView.value && (freshGeneration || progress.done)) void runSearch()
      }),
      onRepoChanged([ChangeScope.RefsChanged], () => void Facade.ReloadLog(wt)),
    ]

    await Facade.LoadLog(wt)
  }

  // applyFilter starts (f non-empty) or clears (f = {}) the worktree's
  // filtered view, then re-runs the search over it.
  async function applyFilter(f: LogFilter): Promise<void> {
    filter.value = f
    filterLoaded.value = 0
    filterError.value = null
    clearDetails()
    resetView(filterView)
    await Facade.SetLogFilter(wt, f)
    await runSearch()
  }

  // runSearch asks the core for query's matches in the active view and
  // steps to the first. The base and filtered views number their
  // generations separately, so an answer is current only when it names
  // both the active view's own kind and its current generation; anything
  // else is a superseded answer and is dropped.
  async function runSearch(): Promise<void> {
    if (!query.value) return clearQuery()
    const result = await Facade.SearchLog(wt, query.value)
    if (result.filtered !== filterActive.value) return
    if (result.generation !== activeView.value.generation) return
    matches.value = result.indices ?? []
    current.value = 0
    await gotoMatch(0)
  }

  watchDebounced(query, runSearch, { debounce: 250 })

  // clearQuery empties the search field and its matches at once, bypassing
  // the debounce.
  function clearQuery(): void {
    query.value = ''
    matches.value = []
    current.value = 0
  }

  // gotoMatch selects matches[i] like a plain click and scrolls it into
  // view.
  async function gotoMatch(i: number): Promise<void> {
    const index = matches.value[i]
    if (index === undefined) return
    current.value = i
    const [row] = await rows(index, index + 1)
    if (row) {
      selectOnly(row, index)
      await select(row)
    }
    pendingScrollTo.value = index
  }

  // nextMatch/prevMatch step through matches, wrapping.
  function nextMatch(): void {
    if (matches.value.length === 0) return
    void gotoMatch((current.value + 1) % matches.value.length)
  }

  function prevMatch(): void {
    if (matches.value.length === 0) return
    void gotoMatch((current.value - 1 + matches.value.length) % matches.value.length)
  }

  async function rows(from: number, to: number): Promise<LogCommit[]> {
    const view = activeView.value
    const cached = cachedWindow(view, from, to)
    if (cached) return cached

    const key = `${from}-${to}`
    const inFlight = view.pending.get(key)
    if (inFlight) return inFlight

    const request = fetchRows(view, from, to).finally(() => {
      if (view.pending.get(key) === request) view.pending.delete(key)
    })
    view.pending.set(key, request)
    return request
  }

  async function fetchRows(view: RowView, from: number, to: number): Promise<LogCommit[]> {
    const result = await Facade.LogRows(wt, from, to)
    const fetched = result.rows ?? []
    if (result.generation < view.generation) return fetched // from before the view restarted
    if (result.generation > view.generation) resetView(view, result.generation)
    fetched.forEach((row, i) => {
      view.cache.set(from + i, row)
      authors.add(row.author)
    })
    return fetched
  }

  async function select(row: LogCommit): Promise<void> {
    selectedRow.value = row
    selectedFile.value = null
    const cached = detailsCache.get(row.sha)
    if (cached) {
      details.value = cached
      return
    }
    const result = await Facade.CommitDetails(wt, row.sha)
    detailsCache.set(row.sha, result)
    if (selectedRow.value?.sha === row.sha) details.value = result
  }

  function selectFile(file: ChangedFile): void {
    selectedFile.value = file
  }

  // jumpTo selects sha's row and asks LogList to scroll to it. A sha not
  // found (not yet streamed, or absent) is a no-op.
  async function jumpTo(sha: string): Promise<void> {
    const { index, found } = await Facade.LogIndexOf(wt, sha)
    if (!found) return
    const [row] = await rows(index, index + 1)
    if (row) {
      selectOnly(row, index)
      void select(row)
    }
    pendingScrollTo.value = index
  }

  // selectOnly collapses the selection to just row: a plain click, or a
  // right-click on a row outside the current selection.
  function selectOnly(row: LogCommit, index: number): void {
    selection.value = [row]
    anchorIndex.value = index
  }

  // selectExactly replaces the selection with rows, in order, without
  // touching the Shift-click anchor: "Compare with <short sha>" pairs the
  // row clicked before with the one just right-clicked.
  function selectExactly(rows: LogCommit[]): void {
    selection.value = rows
  }

  // togglePick adds row to the selection or takes it out, keeping the rest
  // in order; either way row becomes the new Shift-click anchor. Once the
  // selection is down to one commit, that commit's details load, as a plain
  // click's would.
  async function togglePick(row: LogCommit, index: number): Promise<void> {
    const i = selection.value.findIndex((r) => r.sha === row.sha)
    if (i >= 0) selection.value.splice(i, 1)
    else selection.value.push(row)
    anchorIndex.value = index

    const [only, ...more] = selection.value
    if (!only) clearDetails()
    else if (!more.length) await select(only)
  }

  // shiftSelectTo replaces the selection with the range from the anchor to
  // toIndex, anchor first. It goes through rows(), not rowCache: the range
  // may span rows never rendered.
  async function shiftSelectTo(toIndex: number): Promise<void> {
    if (anchorIndex.value === null) anchorIndex.value = toIndex
    const anchor = anchorIndex.value
    const [lo, hi] = anchor <= toIndex ? [anchor, toIndex] : [toIndex, anchor]
    const range = await rows(lo, hi + 1)
    selection.value = anchor <= toIndex ? range : [...range].reverse()
  }

  // swapSelection reverses the selection order: swap moves the compare
  // marks and from/to to the other commit.
  function swapSelection(): void {
    selection.value.reverse()
  }

  // dropSelection empties the selection without touching the shown
  // details: a range action (cherry-pick, revert) leaves whatever single
  // commit was last shown.
  function dropSelection(): void {
    selection.value = []
    anchorIndex.value = null
  }

  // clearSelection leaves no commit selected.
  function clearSelection(): void {
    dropSelection()
    clearDetails()
  }

  // clearDetails drops the selected row, its details and its selected file.
  function clearDetails(): void {
    selectedRow.value = null
    details.value = null
    selectedFile.value = null
  }

  return {
    loaded,
    flat,
    filter,
    filterActive,
    activeLoaded,
    error,
    knownAuthors,
    query,
    matches,
    current,
    selectedRow,
    details,
    selectedFile,
    pendingScrollTo,
    selection,
    anchorIndex,
    rowCache,
    load,
    rows,
    select,
    selectFile,
    nextMatch,
    prevMatch,
    clearQuery,
    jumpTo,
    selectOnly,
    selectExactly,
    togglePick,
    shiftSelectTo,
    swapSelection,
    dropSelection,
    clearSelection,
    applyFilter,
  }
})
