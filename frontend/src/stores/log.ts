import { defineStore } from 'pinia'
import { reactive, ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type {
  CommitDetails,
  LogCommit,
} from '../../bindings/github.com/brogli/nebegit/internal/core'

// log holds one repo session's commit log (FR-LOG-01/09): loaded once per
// LoadLog, row windows cached by index, one commit's details cached by sha.
export const useLogStore = defineStore('log', () => {
  const loaded = ref(0)
  const done = ref(false)
  const selectedRow = ref<LogCommit | null>(null)
  const details = ref<CommitDetails | null>(null)
  const pendingScrollTo = ref<number | null>(null)

  const rowCache = reactive(new Map<number, LogCommit>())
  const detailsCache = new Map<string, CommitDetails>()
  let wt = ''
  let generation = 0
  let unlistenProgress: (() => void) | undefined
  let unlistenChanged: (() => void) | undefined

  async function load(id: string): Promise<void> {
    wt = id
    loaded.value = 0
    done.value = false
    selectedRow.value = null
    details.value = null
    rowCache.clear()
    detailsCache.clear()
    generation = 0

    unlistenProgress?.()
    unlistenProgress = Events.On('log:progress', (ev) => {
      const payload = ev.data as { loaded: number; done: boolean }
      loaded.value = payload.loaded
      done.value = payload.done
    })

    unlistenChanged?.()
    unlistenChanged = Events.On('repo:changed', (ev) => {
      const { scope } = ev.data as { scope: string }
      if (scope === 'refs') void Facade.ReloadLog(wt)
    })

    await Facade.LoadLog(wt)
  }

  async function rows(from: number, to: number): Promise<LogCommit[]> {
    const cached: LogCommit[] = []
    for (let i = from; i < to; i++) {
      const row = rowCache.get(i)
      if (!row) {
        cached.length = 0
        break
      }
      cached.push(row)
    }
    if (cached.length === to - from) return cached

    const result = await Facade.LogRows(wt, from, to)
    if (result.generation !== generation) {
      generation = result.generation
      rowCache.clear() // a reload restarts row indices from 0 (spec 12 Design)
    }
    ;(result.rows ?? []).forEach((row, i) => rowCache.set(from + i, row))
    return result.rows ?? []
  }

  async function select(row: LogCommit): Promise<void> {
    selectedRow.value = row
    const cached = detailsCache.get(row.sha)
    if (cached) {
      details.value = cached
      return
    }
    const result = await Facade.CommitDetails(wt, row.sha)
    detailsCache.set(row.sha, result)
    if (selectedRow.value?.sha === row.sha) details.value = result
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
    selectedRow,
    details,
    pendingScrollTo,
    rowCache,
    load,
    rows,
    select,
    jumpTo,
  }
})
