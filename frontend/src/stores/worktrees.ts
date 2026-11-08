import { defineStore } from 'pinia'
import { ref } from 'vue'
import { Events } from '@wailsio/runtime'
import { AppService, Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'
import { ChangeScope, type Summary } from '../../bindings/github.com/brogli/gaertli/internal/core'
import { onRepoChanged } from '@/lib/windowEvents'

// scopes that mean the summary might now be stale
const SUMMARY_SCOPES = [ChangeScope.StateChanged, ChangeScope.RefsChanged]

// worktrees holds the open worktree's summary (branch, detached, operation),
// refreshed on the worktree's outside-change events.
export const useWorktreesStore = defineStore('worktrees', () => {
  const summary = ref<Summary | null>(null)
  let wt = ''
  let unlisten: (() => void)[] = []

  async function refresh(): Promise<void> {
    summary.value = await Facade.Summary(wt)
    // the window title tracks the branch, so a later checkout updates it too
    void AppService.UpdateWindowTitle(wt, summary.value.branch)
  }

  async function load(id: string): Promise<void> {
    wt = id

    unlisten.forEach((off) => off())
    unlisten = [
      onRepoChanged(SUMMARY_SCOPES, () => void refresh()),
      // a saved editor profile renames "Open in …"
      Events.On('settings:changed', () => void refresh()),
    ]

    await refresh()
  }

  return { summary, load }
})
