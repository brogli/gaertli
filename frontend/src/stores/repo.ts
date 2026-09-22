import { defineStore } from 'pinia'
import { ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { AppService, Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { Summary } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { basename } from '../lib/path'

// scopes that mean the summary might now be stale
const SUMMARY_SCOPES = new Set(['state', 'refs'])

// repo holds the open worktree's summary (branch, detached, operation),
// refreshed on the worktree's outside-change events.
export const useRepoStore = defineStore('repo', () => {
  const summary = ref<Summary | null>(null)
  let wt = ''
  let unlistenChanged: (() => void) | undefined
  let unlistenReconnected: (() => void) | undefined

  async function refresh(): Promise<void> {
    summary.value = await Facade.Summary(wt)
    // the window title tracks the branch, so a later checkout updates it too
    void AppService.UpdateWindowTitle(basename(wt), summary.value.detached ? '' : summary.value.branch)
  }

  async function loadSummary(id: string): Promise<void> {
    wt = id

    unlistenChanged?.()
    unlistenChanged = Events.On('repo:changed', (ev) => {
      const { scope } = ev.data as { scope: string }
      if (SUMMARY_SCOPES.has(scope)) void refresh()
    })

    // the window's distro core respawned: reload as on mount
    unlistenReconnected?.()
    unlistenReconnected = Events.On('core:reconnected', () => void refresh())

    await refresh()
  }

  return { summary, loadSummary }
})
