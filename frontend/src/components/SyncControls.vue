<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { onKeyStroke, useLocalStorage } from '@vueuse/core'
import { useRefsStore } from '../stores/refs'
import { useSettingsStore } from '../stores/settings'
import { useJobsStore } from '../stores/jobs'
import { useConfirmAction } from '../composables/useConfirmAction'
import { useCoreCall, type CoreError } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { Code, PullMode } from '../../bindings/github.com/brogli/nebegit/internal/core'
import ErrorBanner from './ErrorBanner.vue'

// SyncControls is the Fetch/Pull/Push controls, plus force push with its
// protected-branch warning and the push-rejection recovery banner. It only
// starts jobs; JobsStrip.vue shows their progress.
const props = defineProps<{ wt: string }>()
const refs = useRefsStore()
const settings = useSettingsStore()
const jobs = useJobsStore()
const { confirmAction } = useConfirmAction()
const { error, run } = useCoreCall()

const currentBranch = computed(() => refs.refs?.branches?.find((b) => b.current))

// The last pull mode chosen, kept across reloads and restarts; the
// default pull-mode setting until one is chosen.
const lastPullMode = useLocalStorage<PullMode | ''>('nebegit.pullMode', '')
const pullMode = computed<PullMode>({
  get: () =>
    lastPullMode.value ||
    (settings.settings?.defaultPullMode === PullMode.PullRebase ? PullMode.PullRebase : PullMode.PullMerge),
  set: (mode) => {
    lastPullMode.value = mode
  },
})
const upstreamRemote = ref('')

// Preselects "origin" once its remotes are known, else the first remote;
// also fires when refs load after mount. Leaves a value the user already
// picked alone.
watch(
  () => refs.refs?.remotes,
  (remotes) => {
    if (upstreamRemote.value) return
    const names = (remotes ?? []).map((r) => r.name)
    if (names.length === 0) return
    upstreamRemote.value = names.includes('origin') ? 'origin' : names[0]!
  },
  { immediate: true },
)

function onFetch(): void {
  void run(() => Facade.Fetch(props.wt, ''))
}
function onPull(): void {
  void run(() => Facade.Pull(props.wt, pullMode.value))
}
function onPush(): void {
  void run(() => Facade.Push(props.wt, currentBranch.value?.upstream ? '' : upstreamRemote.value))
}
function onForcePush(): void {
  const branchName = currentBranch.value?.name ?? ''
  const protectedBranch = (settings.settings?.protectedBranches ?? []).includes(branchName)
  const warning = protectedBranch ? ` "${branchName}" is a protected branch.` : ''
  confirmAction(
    'Force push',
    `Force push "${branchName}"?${warning} This can overwrite work on the remote.`,
    () => void run(() => Facade.ForcePush(props.wt)),
  )
}

// Gated the same as the Pull/Push buttons: a shortcut is never a bypass of
// their own visibility (no upstream means no plain Pull/Push here).
onKeyStroke(
  (event) => event.ctrlKey && !event.shiftKey && event.key.toLowerCase() === 't',
  (event) => {
    event.preventDefault()
    if (currentBranch.value?.upstream) onPull()
  },
)
onKeyStroke(
  (event) => event.ctrlKey && event.shiftKey && event.key.toLowerCase() === 'k',
  (event) => {
    event.preventDefault()
    if (currentBranch.value?.upstream) onPush()
  },
)

// The rejection banner, offering Pull or Force push, reads the latest push
// or pull job: a fresh push attempt or a pull since clears a stale banner.
const rejection = computed<CoreError | null>(() => {
  const job = jobs.latest((kind) => kind === 'push' || kind === 'force-push' || kind === 'pull')
  if (!job || job.kind === 'pull' || job.state !== 'failed' || !job.error) return null
  if (job.error.code !== Code.PushRejected && job.error.code !== Code.NotFastForward) return null
  return job.error
})

// The blocked-pull banner: the latest pull job, reclassified DirtyWorktree
// by classifyDirtyIntegrate (services/sync.go), offers to stash local
// changes out of the way and retry.
const pullBlocked = computed<CoreError | null>(() => {
  const job = jobs.latest((kind) => kind === 'pull')
  if (!job || job.state !== 'failed' || job.error?.code !== Code.DirtyWorktree) return null
  return job.error
})
function onStashPull(): void {
  void run(() => Facade.PullWithStash(props.wt, pullMode.value))
}

// explainedElsewhere are the job failures another banner already shows:
// the rejection and blocked-pull banners above, ConflictBanner, and a
// cancel the user asked for.
const explainedElsewhere: Code[] = [Code.PushRejected, Code.NotFastForward, Code.DirtyWorktree, Code.Conflicts, Code.Cancelled]

// The latest fetch, pull or push job's failure that no other banner
// explains, e.g. a failed sign-in. A later job of these kinds replaces it.
const syncFailure = computed<CoreError | null>(() => {
  const job = jobs.latest((kind) => ['fetch', 'pull', 'push', 'force-push'].includes(kind))
  if (!job || job.state !== 'failed' || !job.error || explainedElsewhere.includes(job.error.code)) return null
  return job.error
})
</script>

<template>
  <div class="sync-controls">
    <ErrorBanner v-if="error" :error="error" />
    <ErrorBanner v-else-if="syncFailure" :error="syncFailure" data-test="sync-failure" />
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
    <div v-if="pullBlocked" class="rejection" data-test="pull-blocked">
      <ErrorBanner :error="pullBlocked" />
      <button type="button" @click="onStashPull">Stash and pull</button>
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
