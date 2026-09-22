import { defineStore } from 'pinia'
import { computed, reactive, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
type RowView = {
  cache: Map<number, LogCommit>
  generation: number
  pending: Map<string, Promise<LogCommit[]>>
}

function newRowView(): RowView {
  return { cache: reactive(new Map<number, LogCommit>()), generation: 0, pending: new Map() }
}

function resetView(view: RowView): void {
  view.cache.clear()
  view.pending.clear()
  view.generation = 0
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

import type {
  ChangedFile,
  CommitDetails,
  LogCommit,
  LogFilter,
} from '../../bindings/github.com/brogli/nebegit/internal/core'

// log holds one repo session's commit log (FR-LOG-01/09): loaded once per
// LoadLog, row windows cached by index, one commit's details cached by sha.
// It also holds this window's own filtered view (FR-LOG-05..08, spec 22):
// a filter never replaces the base log/loaded/done pair (Design, "Per-
// window, not per-repo") — filterLoaded/filterDone track the filtered
// stream separately, and activeLoaded/activeDone pick whichever pair is
// current.
export const useLogStore = defineStore('log', () => {
  const loaded = ref(0)
  const done = ref(false)
  const filter = ref<LogFilter>({})
  const filterLoaded = ref(0)
  const filterDone = ref(false)
  const selectedRow = ref<LogCommit | null>(null)
  const details = ref<CommitDetails | null>(null)
  const selectedFile = ref<ChangedFile | null>(null)
  const pendingScrollTo = ref<number | null>(null)

  const detailsCache = new Map<string, CommitDetails>()
  let wt = ''
  let unlistenProgress: (() => void) | undefined
  let unlistenChanged: (() => void) | undefined

  // filterActive mirrors core.LogFilter.Empty() (Go): whether filter narrows
  // anything at all. activeLoaded/activeDone read the filtered pair while
  // active, the base pair otherwise — what LogList's virtualizer counts on.
  const filterActive = computed(() => {
    const f = filter.value
    return !!(f.query || f.author || (f.branches && f.branches.length > 0) || f.path)
  })
  const activeLoaded = computed(() => (filterActive.value ? filterLoaded.value : loaded.value))
  const activeDone = computed(() => (filterActive.value ? filterDone.value : done.value))
  // flat is true while a text/author search narrows the view (FR-LOG-02):
  // the graph column is hidden. Mirrors the Go side's own rule.
  const flat = computed(() => !!(filter.value.query || filter.value.author))

  // One row-window cache per view (base log, filtered view): clearing a
  // filter shows the base rows already fetched, and a slow response from the
  // other view lands in that view's cache, never the one on screen.
  // generation is LogRowsResult.Generation as last seen for that view; a
  // change means the view restarted from row 0 (reload, new filter).
  // pending dedupes a window already in flight: the same [from, to) can be
  // requested twice (e.g. two watchEffect ticks) before the first LogRows
  // call fills the cache.
  const baseView = newRowView()
  const filterView = newRowView()
  const activeView = computed(() => (filterActive.value ? filterView : baseView))
  const rowCache = computed(() => activeView.value.cache)

  // knownAuthors is a best-effort suggestion list for the author filter's
  // AutoComplete (Design: not FR-backed, a plain nicety) — every author seen
  // among rows fetched so far, not a complete list of the repo's authors.
  const authors = reactive(new Set<string>())
  const knownAuthors = computed(() => [...authors].sort())

  async function load(id: string): Promise<void> {
    wt = id
    loaded.value = 0
    done.value = false
    filter.value = {}
    filterLoaded.value = 0
    filterDone.value = false
    selectedRow.value = null
    details.value = null
    selectedFile.value = null
    resetView(baseView)
    resetView(filterView)
    authors.clear()
    detailsCache.clear()

    unlistenProgress?.()
    unlistenProgress = Events.On('log:progress', (ev) => {
      const payload = ev.data as { loaded: number; done: boolean; filtered?: boolean }
      if (payload.filtered) {
        filterLoaded.value = payload.loaded
        filterDone.value = payload.done
      } else {
        loaded.value = payload.loaded
        done.value = payload.done
      }
    })

    unlistenChanged?.()
    unlistenChanged = Events.On('repo:changed', (ev) => {
      const { scope } = ev.data as { scope: string }
      if (scope === 'refs') void Facade.ReloadLog(wt)
    })

    await Facade.LoadLog(wt)
  }

  // applyFilter starts (f non-empty) or clears (f = {}) this window's
  // filtered view (FR-LOG-05..08).
  async function applyFilter(f: LogFilter): Promise<void> {
    filter.value = f
    filterLoaded.value = 0
    filterDone.value = false
    selectedRow.value = null
    details.value = null
    selectedFile.value = null
    resetView(filterView)
    await Facade.SetLogFilter(wt, f)
  }

  // clearFilter is FR-LOG-08's "one action clears them all".
  function clearFilter(): Promise<void> {
    return applyFilter({})
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
    if (result.generation !== view.generation) {
      view.generation = result.generation
      view.cache.clear() // the view restarted from row 0 (spec 12 Design)
    }
    const fetched = result.rows ?? []
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

  // nextFile/prevFile step through the selected commit's already-loaded
  // file list (FR-DIFF-02); a no-op at either end, or with no file selected.
  function stepFile(step: 1 | -1): void {
    const files = details.value?.files ?? []
    const i = files.findIndex((f) => f.path === selectedFile.value?.path)
    const next = files[i + step]
    if (i >= 0 && next) selectedFile.value = next
  }

  function nextFile(): void {
    stepFile(1)
  }

  function prevFile(): void {
    stepFile(-1)
  }

  // jumpTo selects sha's row and asks LogList to scroll to it (FR-LOG-01,
  // a sidebar ref click). Not found (not yet streamed, or absent) is a
  // deliberate no-op (Design), not a retry loop or an error.
  async function jumpTo(sha: string): Promise<void> {
    const { index, found } = await Facade.LogIndexOf(wt, sha)
    if (!found) return
    const [row] = await rows(index, index + 1)
    if (row) void select(row)
    pendingScrollTo.value = index
  }

  return {
    loaded,
    done,
    flat,
    filter,
    filterLoaded,
    filterDone,
    filterActive,
    activeLoaded,
    activeDone,
    knownAuthors,
    selectedRow,
    details,
    selectedFile,
    pendingScrollTo,
    rowCache,
    load,
    rows,
    select,
    selectFile,
    nextFile,
    prevFile,
    jumpTo,
    applyFilter,
    clearFilter,
  }
})
