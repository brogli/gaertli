import { defineStore } from 'pinia'
import { ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { AppService, Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { Summary } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { basename } from '../lib/path'

// scopes that mean the summary panel might now be stale (FR-REPO-04,
// git-mapping.md's "Noticing outside changes").
const SUMMARY_SCOPES = new Set(['state', 'refs'])

// repo holds the open worktree's FR-REPO-03 summary (branch, detached,
// operation), refreshed on the worktree's outside-change events.
export const useRepoStore = defineStore('repo', () => {
  const summary = ref<Summary | null>(null)
  let wt = ''
  let unlistenChanged: (() => void) | undefined
  let unlistenReconnected: (() => void) | undefined

  async function refresh(): Promise<void> {
    summary.value = await Facade.Summary(wt)
    // FR-WT-09: the window title tracks the branch from mount onward, not
    // only at open/switch time, so a later checkout updates it too, riding
    // this same Summary refresh.
    const name = basename(wt)
    void AppService.UpdateWindowTitle(name, summary.value.detached ? '' : summary.value.branch)
  }

  async function loadSummary(id: string): Promise<void> {
    wt = id

    unlistenChanged?.()
    unlistenChanged = Events.On('repo:changed', (ev) => {
      const { scope } = ev.data as { scope: string }
      if (SUMMARY_SCOPES.has(scope)) void refresh()
    })

    // core:reconnected (ADR-4): the window's distro core respawned, so this
    // window's data starts over, same as its own mount load.
    unlistenReconnected?.()
    unlistenReconnected = Events.On('core:reconnected', () => void refresh())

    await refresh()
  }

  return { summary, loadSummary }
})
