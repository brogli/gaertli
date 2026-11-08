import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'
import { JobState, type JobInfo } from '../../bindings/github.com/brogli/gaertli/internal/core'

// jobs holds the worktree's repo-level jobs: load fetches the current list,
// upsert applies one "job:update" event.
export const useJobsStore = defineStore('jobs', () => {
  const items = ref<JobInfo[]>([])
  let wt = ''

  // active is what the jobs menu shows: a job's terminal "job:update" drops
  // it.
  const active = computed(() => items.value.filter((j) => j.state === JobState.JobRunning))

  async function load(id: string): Promise<void> {
    wt = id
    items.value = (await Facade.Jobs(wt)) ?? []
  }

  function upsert(job: JobInfo): void {
    const i = items.value.findIndex((j) => j.id === job.id)
    if (i >= 0) items.value[i] = job
    else items.value.push(job)
  }

  // cancel throws: JobsMenu.vue reports the error.
  async function cancel(id: string): Promise<void> {
    await Facade.CancelJob(wt, id)
  }

  return { items, active, load, upsert, cancel }
})
