import { defineStore } from 'pinia'
import { ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { AppService, Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { Summary } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { onWindowEvent } from '../lib/windowEvents'

// scopes that mean the summary might now be stale
const SUMMARY_SCOPES = new Set(['state', 'refs'])

// repo holds the open worktree's summary (branch, detached, operation),
// refreshed on the worktree's outside-change events.
export const useRepoStore = defineStore('repo', () => {
  const summary = ref<Summary | null>(null)
  let wt = ''
  let unlisten: (() => void)[] = []

  async function refresh(): Promise<void> {
    summary.value = await Facade.Summary(wt)
    // the window title tracks the branch, so a later checkout updates it too
    void AppService.UpdateWindowTitle(wt, summary.value.branch)
  }

  async function loadSummary(id: string): Promise<void> {
    wt = id

    unlisten.forEach((off) => off())
    unlisten = [
      onWindowEvent('repo:changed', (ev) => {
        const { scope } = ev.data
        if (SUMMARY_SCOPES.has(scope)) void refresh()
      }),
      // the window's distro core respawned: reload as on mount
      onWindowEvent('core:reconnected', () => void refresh()),
      // a saved editor profile renames "Open in …"
      Events.On('settings:changed', () => void refresh()),
    ]

    await refresh()
  }

  return { summary, loadSummary }
})
