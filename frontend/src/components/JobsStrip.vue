<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import { Events } from '@wailsio/runtime'
import ProgressBar from 'openvue/progressbar'
import Button from 'openvue/button'
import { useJobsStore } from '../stores/jobs'
import type { JobInfo } from '../../bindings/github.com/brogli/nebegit/internal/core'

const props = defineProps<{ wt: string }>()
const jobs = useJobsStore()

let unlisten: (() => void) | undefined

onMounted(() => {
  void jobs.load(props.wt)
  unlisten = Events.On('job:update', (ev) => jobs.upsert(ev.data as JobInfo))
})

onUnmounted(() => {
  unlisten?.()
})
</script>

<template>
  <section v-if="jobs.active.length" class="jobs-strip">
    <div v-for="job in jobs.active" :key="job.id" class="job-row">
      <span>{{ job.kind }}</span>
      <span v-if="job.message">{{ job.message }}</span>
      <ProgressBar :value="job.progress" :mode="job.progress < 0 ? 'indeterminate' : 'determinate'" />
      <Button label="Cancel" data-test="job-cancel" @click="jobs.cancel(job.id)" />
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
