import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { JobInfo } from '../../bindings/github.com/brogli/nebegit/internal/core'

// jobs holds the worktree's repo-level jobs (FR-SYNC-07, components.md):
// load fetches the current list, upsert applies one "job:update" event —
// same load/append shape as commandLog, whose own "command" subscription
// likewise lives in its component (CommandLogPanel.vue), not the store.
export const useJobsStore = defineStore('jobs', () => {
  const items = ref<JobInfo[]>([])
  let wt = ''

  // active is what the strip shows: a finished job's own terminal
  // "job:update" (e.g. cancelled) simply drops it out (Design).
  const active = computed(() => items.value.filter((j) => j.state === 'queued' || j.state === 'running'))

  async function load(id: string): Promise<void> {
    wt = id
    items.value = (await Facade.Jobs(wt)) ?? []
  }

  function upsert(job: JobInfo): void {
    const i = items.value.findIndex((j) => j.id === job.id)
    if (i >= 0) items.value[i] = job
    else items.value.push(job)
  }

  function cancel(id: string): void {
    void Facade.CancelJob(wt, id)
  }

  return { items, active, load, upsert, cancel }
})
