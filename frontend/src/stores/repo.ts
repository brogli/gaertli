import { defineStore } from 'pinia'
import { ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { Summary } from '../../bindings/github.com/brogli/nebegit/internal/core'

// repo holds the open worktree's FR-REPO-03 summary (branch, detached,
// operation).
export const useRepoStore = defineStore('repo', () => {
  const summary = ref<Summary | null>(null)

  async function loadSummary(wt: string): Promise<void> {
    summary.value = await Facade.Summary(wt)
  }

  return { summary, loadSummary }
})
