<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import { onKeyStroke } from '@vueuse/core'
import Button from 'openvue/button'
import SplitButton from 'openvue/splitbutton'
import type { MenuItem } from 'openvue/menuitem'
import { useRefsStore } from '@/stores/refs'
import { useSettingsStore } from '@/stores/settings'
import { useConfirmAction } from '@/composables/useConfirmAction'
import { useCoreCall } from '@/composables/useCoreCall'
import { useJobNotices } from '@/composables/useJobNotices'
import { usePush } from '@/composables/usePush'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import {
  JobKind,
  PullMode,
  type JobInfo,
} from '../../bindings/github.com/brogli/nebegit/internal/core'
import { iconClass } from '@/lib/icons'
import { useLayout, useToolbarButton } from '@/lib/layout'
import ErrorBanner from './ErrorBanner.vue'
import SetUpstreamDialog from './SetUpstreamDialog.vue'

// SyncControls is the Fetch/Pull/Push controls: it only starts jobs and
// offers what can act. JobsMenu shows their progress; useJobNotices shows
// their results.
const props = defineProps<{ wt: string; compact?: boolean }>()
const refs = useRefsStore()
const settings = useSettingsStore()
const { confirmAction } = useConfirmAction()
const { error, run } = useCoreCall()
const { push, upstreamQuestion } = usePush(() => props.wt, run)
const { t } = useI18n()
const layout = useLayout()
const toolbarButton = useToolbarButton()

// Below the medium width, the controls' own gap tightens too.
const narrow = computed(() => layout.value === 'narrow')

const NO_REMOTE_TOOLTIP = t('toolbar.noRemoteTooltip')
const NO_BRANCH_TOOLTIP = t('toolbar.noBranchTooltip')

const remoteNames = computed(() => (refs.refs?.remotes ?? []).map((r) => r.name))
const hasRemote = computed(() => remoteNames.value.length > 0)

// Pull and Push need both a remote and a checked-out branch; this names
// why they're disabled, or '' when they can act.
const disabledReason = computed(() => {
  if (!hasRemote.value) return NO_REMOTE_TOOLTIP
  if (!refs.current) return NO_BRANCH_TOOLTIP
  return ''
})

// The button's own mode: Settings' default pull mode. Live in every
// window: settings.settings follows a save in any window
// (stores/settings.ts).
const pullMode = computed<PullMode>(() =>
  settings.settings?.defaultPullMode === PullMode.PullRebase
    ? PullMode.PullRebase
    : PullMode.PullMerge,
)
const pullLabel = computed(() =>
  pullMode.value === PullMode.PullRebase ? t('toolbar.pullRebase') : t('toolbar.pullMerge'),
)
const pullMenuItems = computed<MenuItem[]>(() => [
  { label: t('toolbar.pullWithMerge'), command: () => runPull(PullMode.PullMerge) },
  { label: t('toolbar.pullWithRebase'), command: () => runPull(PullMode.PullRebase) },
])

function onFetch(): void {
  void run(() => Facade.Fetch(props.wt, ''))
}
function runPull(mode: PullMode): void {
  void run(() => Facade.Pull(props.wt, mode))
}
function onPull(): void {
  runPull(pullMode.value)
}
function onForcePush(): void {
  const branchName = refs.current?.name ?? ''
  const protectedBranch = (settings.settings?.protectedBranches ?? []).includes(branchName)
  const warning = protectedBranch
    ? t('toolbar.forcePushProtectedWarning', { branch: branchName })
    : ''
  confirmAction(
    t('toolbar.forcePush'),
    t('toolbar.forcePushConfirm', { branch: branchName, warning }),
    () => void run(() => Facade.ForcePush(props.wt)),
  )
}
function onStashPull(): void {
  void run(() => Facade.PullWithStash(props.wt, pullMode.value))
}

// onRetryJob starts a fresh job the same way job's own kind started.
const JOB_RETRIES: Partial<Record<JobKind, (job: JobInfo) => void>> = {
  [JobKind.FetchJob]: onFetch,
  [JobKind.PullJob]: onPull,
  [JobKind.PushJob]: push,
  [JobKind.ForcePushJob]: onForcePush,
  [JobKind.UpdateBranchJob]: (job) =>
    void run(() => Facade.UpdateBranch(props.wt, job.branch ?? '')),
}
function onRetryJob(job: JobInfo): void {
  JOB_RETRIES[job.kind]?.(job)
}

// Gated the same as the Pull/Push buttons: a shortcut is never a bypass of
// their own visibility (no upstream means no plain Pull/Push here).
onKeyStroke(
  (event) => event.ctrlKey && !event.shiftKey && event.key.toLowerCase() === 't',
  (event) => {
    event.preventDefault()
    if (refs.current?.upstream) onPull()
  },
)
onKeyStroke(
  (event) => event.ctrlKey && event.shiftKey && event.key.toLowerCase() === 'k',
  (event) => {
    event.preventDefault()
    if (!disabledReason.value) push()
  },
)

useJobNotices({ onPull, onForcePush, onStashPull, onRetryJob })
</script>

<template>
  <div class="sync-controls flex items-center" :class="narrow ? 'gap-1' : 'gap-2'">
    <ErrorBanner v-if="error" :error="error" />
    <Button
      v-bind="toolbarButton"
      :icon="iconClass('fetch')"
      :label="compact ? '' : t('toolbar.fetch')"
      :disabled="!hasRemote"
      :title="!hasRemote ? NO_REMOTE_TOOLTIP : t('toolbar.fetch')"
      :aria-label="t('toolbar.fetch')"
      @click="onFetch"
    />
    <template v-if="disabledReason">
      <Button
        v-bind="toolbarButton"
        :icon="iconClass('pull')"
        :label="compact ? '' : t('toolbar.pull')"
        disabled
        :title="disabledReason"
        :aria-label="t('toolbar.pull')"
      />
      <Button
        v-bind="toolbarButton"
        :icon="iconClass('push')"
        :label="compact ? '' : t('toolbar.push')"
        disabled
        :title="disabledReason"
        :aria-label="t('toolbar.push')"
      />
    </template>
    <template v-else>
      <SplitButton
        v-if="refs.current!.upstream"
        v-bind="toolbarButton"
        :label="compact ? '' : pullLabel"
        :button-props="{ title: pullLabel, 'aria-label': pullLabel }"
        :model="pullMenuItems"
        @click="onPull"
      >
        <template #icon><i :class="iconClass('pull')" aria-hidden="true"></i></template>
      </SplitButton>
      <Button
        v-bind="toolbarButton"
        :icon="iconClass('push')"
        :label="compact ? '' : t('toolbar.push')"
        :title="t('toolbar.push')"
        :aria-label="t('toolbar.push')"
        @click="push"
      />
    </template>
    <SetUpstreamDialog
      v-if="upstreamQuestion"
      :wt="wt"
      :branch="upstreamQuestion.branch"
      :remote="upstreamQuestion.remote"
      @close="upstreamQuestion = null"
    />
  </div>
</template>
