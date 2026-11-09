import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'
import { JobState, type JobInfo } from '../../bindings/github.com/brogli/gaertli/internal/core'

// sync holds the worktree's repo-level jobs: load fetches the current list,
// upsert applies one "job:update" event.
export const useSyncStore = defineStore('sync', () => {
  const jobs = ref<JobInfo[]>([])
  let wt = ''

  // running is what the jobs menu shows: a job's terminal "job:update" drops
  // it.
  const running = computed(() => jobs.value.filter((j) => j.state === JobState.JobRunning))

  async function load(id: string): Promise<void> {
    wt = id
    jobs.value = (await Facade.Jobs(wt)) ?? []
  }

  function upsert(job: JobInfo): void {
    const i = jobs.value.findIndex((j) => j.id === job.id)
    if (i >= 0) jobs.value[i] = job
    else jobs.value.push(job)
  }

  // cancel throws: JobsMenu.vue reports the error.
  async function cancel(id: string): Promise<void> {
    await Facade.CancelJob(wt, id)
  }

  return { jobs, running, load, upsert, cancel }
})
