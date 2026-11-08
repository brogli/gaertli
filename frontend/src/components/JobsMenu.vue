<script setup lang="ts">
import { computed, onMounted, useTemplateRef } from 'vue'
import { useI18n } from '@/i18n'
import Popover from 'openvue/popover'
import Button from 'openvue/button'
import ProgressBar from 'openvue/progressbar'
import { useSyncStore } from '@/stores/sync'
import { useCoreCall } from '@/composables/useCoreCall'
import { useWindowEvent } from '@/lib/windowEvents'
import { iconClass, spinClass } from '@/lib/icons'
import { ICON_BUTTON, useToolbarButton } from '@/lib/layout'
import { JobKind, type JobInfo } from '../../bindings/github.com/brogli/gaertli/internal/core'
import ErrorBanner from './ErrorBanner.vue'

// JobsMenu is the toolbar's jobs menu: a fixed-size button naming how many
// jobs are running, opening a Popover that lists them with an abort button
// each. Starting or ending a job never resizes the button, so the rest of
// the toolbar and the log below it never move.
const props = defineProps<{ wt: string }>()
const jobs = useSyncStore()
const { error, run } = useCoreCall()
const { t } = useI18n()
const toolbarButton = useToolbarButton()

onMounted(() => void jobs.load(props.wt))
useWindowEvent('job:update', (ev) => jobs.upsert(ev.data))

// jobLabel names an update-branch job by its branch instead of its kind.
function jobLabel(job: JobInfo): string {
  if (job.kind === JobKind.UpdateBranchJob)
    return t('jobsMenu.updateBranch', { branch: job.branch })
  const kindLabels: Partial<Record<JobKind, string>> = {
    [JobKind.FetchJob]: t('toolbar.fetch'),
    [JobKind.PullJob]: t('toolbar.pull'),
    [JobKind.PushJob]: t('toolbar.push'),
    [JobKind.ForcePushJob]: t('toolbar.forcePush'),
  }
  return kindLabels[job.kind] ?? job.kind
}

const count = computed(() => jobs.running.length)
const tooltip = computed(() =>
  count.value === 0
    ? t('jobsMenu.noJobs')
    : t('jobsMenu.runningJobsCount', { n: count.value }, count.value),
)

function cancelJob(id: string): void {
  void run(() => jobs.cancel(id))
}

const popover = useTemplateRef<InstanceType<typeof Popover>>('popover')
</script>

<template>
  <Button
    v-bind="toolbarButton"
    data-test="jobs-menu"
    :icon="count ? spinClass('jobsRunning') : iconClass('jobsIdle')"
    :badge="count ? String(count) : undefined"
    badge-severity="info"
    :pt="{ pcBadge: { root: { 'data-test': 'jobs-badge' } } }"
    :title="tooltip"
    :aria-label="tooltip"
    @click="popover?.toggle($event)"
  />
  <Popover ref="popover">
    <div class="jobs-list flex flex-col gap-1.5 w-88 max-w-[90vw]">
      <ErrorBanner v-if="error" :error="error" />
      <p v-if="!count" class="text-muted-color">{{ t('jobsMenu.noJobs') }}</p>
      <div
        v-for="job in jobs.running"
        :key="job.id"
        class="flex items-center gap-2"
        data-test="job-row"
      >
        <span class="flex-none">{{ jobLabel(job) }}</span>
        <span v-if="job.message" class="flex-1 min-w-0 truncate text-muted-color">{{
          job.message
        }}</span>
        <ProgressBar
          :value="job.progress"
          :mode="job.progress < 0 ? 'indeterminate' : 'determinate'"
        />
        <Button
          v-bind="ICON_BUTTON"
          :icon="iconClass('abort')"
          :title="t('jobsMenu.abort')"
          :aria-label="t('jobsMenu.abort')"
          data-test="job-cancel"
          @click="cancelJob(job.id)"
        />
      </div>
    </div>
  </Popover>
</template>
