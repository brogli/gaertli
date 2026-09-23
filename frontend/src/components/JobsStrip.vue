<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { Events } from '@wailsio/runtime'
import ProgressBar from 'openvue/progressbar'
import Button from 'openvue/button'
import { useJobsStore } from '../stores/jobs'
import { useCoreCall } from '../composables/useCoreCall'
import type { JobInfo } from '../../bindings/github.com/brogli/nebegit/internal/core'
import ErrorBanner from './ErrorBanner.vue'

const props = defineProps<{ wt: string }>()
const jobs = useJobsStore()
const { error, run } = useCoreCall()

let unlisten: (() => void) | undefined

onMounted(() => {
  void jobs.load(props.wt)
  unlisten = Events.On('job:update', (ev) => jobs.upsert(ev.data as JobInfo))
})

onUnmounted(() => {
  unlisten?.()
})

function cancelJob(id: string): void {
  void run(() => jobs.cancel(id))
}

// jobLabel names an update-branch job by its branch instead of its kind.
function jobLabel(job: JobInfo): string {
  return job.kind === 'update-branch' ? `Update ${job.branch}` : job.kind
}
</script>

<template>
  <section v-if="jobs.active.length" class="jobs-strip">
    <ErrorBanner v-if="error" :error="error" />
    <div v-for="job in jobs.active" :key="job.id" class="job-row">
      <span>{{ jobLabel(job) }}</span>
      <span v-if="job.message">{{ job.message }}</span>
      <ProgressBar
        :value="job.progress"
        :mode="job.progress < 0 ? 'indeterminate' : 'determinate'"
      />
      <Button label="Cancel" data-test="job-cancel" @click="cancelJob(job.id)" />
    </div>
  </section>
</template>

<style scoped>
.jobs-strip {
  display: flex;
  flex-direction: column;
  gap: 0.25em;
}
.job-row {
  display: flex;
  align-items: center;
  gap: 0.5em;
}
</style>
