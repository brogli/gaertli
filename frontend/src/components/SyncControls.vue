<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRefsStore } from '../stores/refs'
import { useSettingsStore } from '../stores/settings'
import { useJobsStore } from '../stores/jobs'
import { useConfirmAction } from '../composables/useConfirmAction'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { Code, PullMode } from '../../bindings/github.com/brogli/nebegit/internal/core'
import type { JobInfo } from '../../bindings/github.com/brogli/nebegit/internal/core'
import type { CoreError } from '../composables/useCoreCall'
import ErrorBanner from './ErrorBanner.vue'

// SyncControls is the Fetch/Pull/Push top-bar controls, plus force push
// with its protected-branch warning and the push-rejection recovery
// banner. Mounted beside WorktreeContent's branch summary. This component
// only starts jobs; the jobs strip (JobsStrip.vue) owns progress and
// cancel, feeding every "job:update" into the shared jobs store that this
// component's own rejection banner reads.
const props = defineProps<{ wt: string }>()
const refs = useRefsStore()
const settings = useSettingsStore()
const jobs = useJobsStore()
const { confirmAction } = useConfirmAction()

const currentBranch = computed(() => refs.refs?.branches?.find((b) => b.current))

// Seeded once from the default pull-mode setting, then just local
// component state for "last choice preselected".
const pullMode = ref<PullMode>(
  settings.settings?.defaultPullMode === PullMode.PullRebase
    ? PullMode.PullRebase
    : PullMode.PullMerge,
)
const upstreamRemote = ref('')

function onFetch(): void {
  void Facade.Fetch(props.wt, '')
}
function onPull(): void {
  void Facade.Pull(props.wt, pullMode.value)
}
function onPush(): void {
  void Facade.Push(props.wt, currentBranch.value?.upstream ? '' : upstreamRemote.value)
}
function onForcePush(): void {
  const branchName = currentBranch.value?.name ?? ''
  const protectedBranch = (settings.settings?.protectedBranches ?? []).includes(branchName)
  const warning = protectedBranch ? ` "${branchName}" is a protected branch.` : ''
  confirmAction(
    'Force push',
    `Force push "${branchName}"?${warning} This can overwrite work on the remote.`,
    () => void Facade.ForcePush(props.wt),
  )
}

// The rejection banner, offering Pull or Force push, driven by the most
// recently started push/force-push job in the shared jobs store — a fresh
// attempt (state "running") naturally clears a stale banner, since it
// becomes the latest job and isn't "failed".
const latestPushJob = computed(() =>
  jobs.items
    .filter((j) => j.kind === 'push' || j.kind === 'force-push')
    .reduce<JobInfo | undefined>((latest, j) => (!latest || j.startedAt > latest.startedAt ? j : latest), undefined),
)
const rejection = computed<CoreError | null>(() => {
  const job = latestPushJob.value
  if (!job || job.state !== 'failed' || !job.error) return null
  if (job.error.code !== Code.PushRejected && job.error.code !== Code.NotFastForward) return null
  return job.error
})
</script>

<template>
  <div class="sync-controls">
    <button type="button" @click="onFetch">Fetch</button>
    <template v-if="currentBranch?.upstream">
      <select v-model="pullMode">
        <option value="merge">Merge</option>
        <option value="rebase">Rebase</option>
      </select>
      <button type="button" @click="onPull">Pull</button>
      <button type="button" @click="onPush">Push</button>
    </template>
    <template v-else-if="currentBranch">
      <select v-model="upstreamRemote">
        <option v-for="r in refs.refs?.remotes ?? []" :key="r.name" :value="r.name">
          {{ r.name }}
        </option>
      </select>
      <button type="button" :disabled="!upstreamRemote" @click="onPush">
        Push and set upstream
      </button>
    </template>
    <div v-if="rejection" class="rejection" data-test="push-rejection">
      <ErrorBanner :error="rejection" />
      <button type="button" @click="onPull">Pull</button>
      <button type="button" @click="onForcePush">Force push</button>
    </div>
  </div>
</template>

<style scoped>
.sync-controls {
  display: flex;
  align-items: center;
  gap: 0.5em;
}
</style>
