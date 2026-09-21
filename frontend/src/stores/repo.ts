import { defineStore } from 'pinia'
import { ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { Summary } from '../../bindings/github.com/brogli/nebegit/internal/core'

// scopes that mean the summary panel might now be stale (FR-REPO-04,
// git-mapping.md's "Noticing outside changes").
const SUMMARY_SCOPES = new Set(['state', 'refs'])

// repo holds the open worktree's FR-REPO-03 summary (branch, detached,
// operation), refreshed on the worktree's outside-change events.
export const useRepoStore = defineStore('repo', () => {
  const summary = ref<Summary | null>(null)
  let wt = ''
  let unlistenChanged: (() => void) | undefined

  async function refresh(): Promise<void> {
    summary.value = await Facade.Summary(wt)
  }

  async function loadSummary(id: string): Promise<void> {
    wt = id

    unlistenChanged?.()
    unlistenChanged = Events.On('repo:changed', (ev) => {
      const { scope } = ev.data as { scope: string }
      if (SUMMARY_SCOPES.has(scope)) void refresh()
    })

    await refresh()
  }

  return { summary, loadSummary }
})
