import { defineStore } from 'pinia'
import { computed, reactive, ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type {
  ChangedFile,
  CommitDetails,
  LogCommit,
  LogFilter,
} from '../../bindings/github.com/brogli/nebegit/internal/core'
import { onWindowEvent } from '../lib/windowEvents'

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
// windows cached by index, each commit's details cached by sha. It also holds this
// window's filtered view: filterLoaded counts the filtered stream's rows
// beside the base loaded count.
export const useLogStore = defineStore('log', () => {
  const loaded = ref(0)
  const done = ref(false)
  const filter = ref<LogFilter>({})
  const filterLoaded = ref(0)
  const selectedRow = ref<LogCommit | null>(null)
  const details = ref<CommitDetails | null>(null)
  const selectedFile = ref<ChangedFile | null>(null)
  const pendingScrollTo = ref<number | null>(null)

  // multiSelect is a Shift-click contiguous range, independent of
  // selectedRow (single-row details) and of the compare store's Ctrl-click
  // pair. anchorIndex is that single selected row's log index, the anchor a
  // Shift-click range extends from and the index the compare store needs
  // when pairing it into a comparison.
  const multiSelect = ref<LogCommit[]>([])
  const anchorIndex = ref<number | null>(null)

  const detailsCache = new Map<string, CommitDetails>()
  let wt = ''
  let unlisten: (() => void)[] = []

  // filterActive is !core.LogFilter.Empty(). activeLoaded is the row count
  // LogList's virtualizer shows.
  const filterActive = computed(() => {
    const f = filter.value
    return !!(f.query || f.author || (f.branches && f.branches.length > 0) || f.path)
  })
  const activeLoaded = computed(() => (filterActive.value ? filterLoaded.value : loaded.value))
  // flat hides the graph column while a text/author search narrows the
  // view, the same rule as the Go side.
  const flat = computed(() => !!(filter.value.query || filter.value.author))

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
    done.value = false
    filter.value = {}
    filterLoaded.value = 0
    selectedRow.value = null
    details.value = null
    selectedFile.value = null
    resetView(baseView)
    resetView(filterView)
    authors.clear()
    detailsCache.clear()

    unlisten.forEach((off) => off())
    unlisten = [
      onWindowEvent('log:progress', ({ data: progress }) => {
        // A reload restarts its stream even when the row count stays the
        // same, base or filtered: either can drop a commit a reset left
        // unreachable without changing how many rows are loaded.
        if (progress.filtered) {
          if (progress.generation !== filterView.generation)
            resetView(filterView, progress.generation)
          filterLoaded.value = progress.loaded
        } else {
          if (progress.generation !== baseView.generation) resetView(baseView, progress.generation)
          loaded.value = progress.loaded
          done.value = progress.done
        }
      }),
      onWindowEvent('repo:changed', (ev) => {
        const { scope } = ev.data
        if (scope === 'refs') void Facade.ReloadLog(wt)
      }),
      // a respawned core's generation numbers are unrelated to the old
      // ones, so the log starts over rather than reloading
      onWindowEvent('core:reconnected', () => void Facade.LoadLog(wt)),
    ]

    await Facade.LoadLog(wt)
  }

  // applyFilter starts (f non-empty) or clears (f = {}) this window's
  // filtered view.
  async function applyFilter(f: LogFilter): Promise<void> {
    filter.value = f
    filterLoaded.value = 0
    selectedRow.value = null
    details.value = null
    selectedFile.value = null
    resetView(filterView)
    await Facade.SetLogFilter(wt, f)
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
    if (row) void select(row)
    pendingScrollTo.value = index
  }

  // selectOnly collapses the multi-select to just row: a plain click, or a
  // right-click on a row outside the current range.
  function selectOnly(row: LogCommit, index: number): void {
    multiSelect.value = [row]
    anchorIndex.value = index
  }

  // shiftSelectTo extends/creates a range from the last plain-clicked row
  // to toIndex. It goes through rows(), not rowCache: the range may span
  // rows never rendered.
  async function shiftSelectTo(toIndex: number): Promise<void> {
    if (anchorIndex.value === null) anchorIndex.value = toIndex
    const [lo, hi] =
      anchorIndex.value <= toIndex ? [anchorIndex.value, toIndex] : [toIndex, anchorIndex.value]
    multiSelect.value = await rows(lo, hi + 1)
  }

  function clearMultiSelect(): void {
    multiSelect.value = []
    anchorIndex.value = null
  }

  return {
    loaded,
    done,
    flat,
    filter,
    filterActive,
    activeLoaded,
    knownAuthors,
    selectedRow,
    details,
    selectedFile,
    pendingScrollTo,
    multiSelect,
    anchorIndex,
    rowCache,
    load,
    rows,
    select,
    selectFile,
    jumpTo,
    selectOnly,
    shiftSelectTo,
    clearMultiSelect,
    applyFilter,
  }
})
