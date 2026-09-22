import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { JobInfo } from '../../bindings/github.com/brogli/nebegit/internal/core'

// jobs holds the worktree's repo-level jobs: load fetches the current list,
// upsert applies one "job:update" event.
export const useJobsStore = defineStore('jobs', () => {
  const items = ref<JobInfo[]>([])
  let wt = ''

  // active is what the strip shows: a job's terminal "job:update" drops it.
  const active = computed(() => items.value.filter((j) => j.state === 'queued' || j.state === 'running'))

  // latest is the most recently started job whose kind matches.
  function latest(matches: (kind: string) => boolean): JobInfo | undefined {
    return items.value
      .filter((j) => matches(j.kind))
      .reduce<JobInfo | undefined>(
        (newest, j) => (!newest || j.startedAt > newest.startedAt ? j : newest),
        undefined,
      )
  }

  async function load(id: string): Promise<void> {
    wt = id
    items.value = (await Facade.Jobs(wt)) ?? []
  }

  function upsert(job: JobInfo): void {
    const i = items.value.findIndex((j) => j.id === job.id)
    if (i >= 0) items.value[i] = job
    else items.value.push(job)
  }

  // cancel throws: JobsStrip.vue reports the error.
  async function cancel(id: string): Promise<void> {
    await Facade.CancelJob(wt, id)
  }

  return { items, active, latest, load, upsert, cancel }
})
