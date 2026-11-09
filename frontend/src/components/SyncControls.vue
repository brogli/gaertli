<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '@/i18n'
import Button from 'openvue/button'
import SplitButton from 'openvue/splitbutton'
import type { MenuItem } from 'openvue/menuitem'
import { useRefsStore } from '@/stores/refs'
import { useSettingsStore } from '@/stores/settings'
import { useConfirmAction } from '@/composables/useConfirmAction'
import { useCoreCall } from '@/composables/useCoreCall'
import { openConflicts } from '@/lib/openConflicts'
import { useJobNotices } from '@/composables/useJobNotices'
import { usePush } from '@/composables/usePush'
import { useShortcut } from '@/composables/useShortcut'
import { useWithShortcut } from '@/composables/useWithShortcut'
import { Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'
import {
  Code,
  JobKind,
  PullMode,
  type JobInfo,
} from '../../bindings/github.com/brogli/gaertli/internal/core'
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
const withShortcut = useWithShortcut()
const layout = useLayout()
const toolbarButton = useToolbarButton()

// Below the medium width, the controls' own gap tightens too.
const narrow = computed(() => layout.value === 'narrow')

const NO_REMOTE_TOOLTIP = t('toolbar.noRemoteTooltip')
const NO_BRANCH_TOOLTIP = t('toolbar.noBranchTooltip')

const remoteNames = computed(() => (refs.refs?.remotes ?? []).map((r) => r.name))
const hasRemote = computed(() => remoteNames.value.length > 0)

// Pull and Push need both a remote and a current branch; this names
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
const pullLabelKey = computed(() =>
  pullMode.value === PullMode.PullRebase ? 'toolbar.pullRebase' : 'toolbar.pullMerge',
)
const pullLabel = computed(() => t(pullLabelKey.value))
const pullTooltip = computed(() => withShortcut(pullLabelKey.value, 'shortcuts.pull'))
const pushTooltip = computed(() => withShortcut('toolbar.push', 'shortcuts.push'))
const pullMenuTooltip = t('toolbar.pullModeMenu')
const pullMenuItems = computed<MenuItem[]>(() => [
  {
    label: t('toolbar.pullMerge'),
    icon: pullMode.value === PullMode.PullMerge ? iconClass('current') : undefined,
    command: () => choosePullMode(PullMode.PullMerge),
  },
  {
    label: t('toolbar.pullRebase'),
    icon: pullMode.value === PullMode.PullRebase ? iconClass('current') : undefined,
    command: () => choosePullMode(PullMode.PullRebase),
  },
])

function onFetch(): void {
  void run(() => Facade.Fetch(props.wt, ''))
}
// choosePullMode only saves the chosen mode as Settings' default pull mode;
// it never pulls. settings:changed then updates every window's button.
function choosePullMode(mode: PullMode): void {
  if (!settings.settings) return
  const next = { ...settings.settings, defaultPullMode: mode }
  void run(() => settings.save(next))
}
// startedPulls are the ids of the running pull jobs this window started:
// only their conflicts open the diff window here.
const startedPulls = new Set<string>()
// A pull job can end before its start call returns. While a start call is
// out, endedPulls keeps each pull end: its id and whether it stopped on
// conflicts.
let startCallsOut = 0
const endedPulls = new Map<string, boolean>()

async function startPull(call: () => Promise<string>): Promise<void> {
  startCallsOut++
  const id = await run(call)
  startCallsOut--
  if (id !== undefined) {
    if (!endedPulls.has(id)) startedPulls.add(id)
    else if (endedPulls.get(id)) void run(() => openConflicts(props.wt))
  }
  if (startCallsOut === 0) endedPulls.clear()
}
function onPull(): void {
  void startPull(() => Facade.Pull(props.wt, pullMode.value))
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
  void startPull(() => Facade.PullWithStash(props.wt, pullMode.value))
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

// Ctrl+T pulls only where the Pull button is enabled and shows (an
// upstream); Ctrl+Shift+K pushes only where the Push button is enabled.
useShortcut(
  (event) => event.ctrlKey && !event.shiftKey && event.key.toLowerCase() === 't',
  () => {
    if (!disabledReason.value && refs.current?.upstream) onPull()
  },
)
useShortcut(
  (event) => event.ctrlKey && event.shiftKey && event.key.toLowerCase() === 'k',
  () => {
    if (!disabledReason.value) push()
  },
)

function onPullEnded(job: JobInfo): void {
  const conflicted = job.error?.code === Code.Conflicts
  if (startedPulls.delete(job.id)) {
    if (conflicted) void run(() => openConflicts(props.wt))
  } else if (startCallsOut > 0) {
    endedPulls.set(job.id, conflicted)
  }
}

useJobNotices({ onPull, onForcePush, onStashPull, onRetryJob, onPullEnded })
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
        :button-props="{ title: pullTooltip, 'aria-label': pullLabel }"
        :menu-button-props="{ title: pullMenuTooltip, 'aria-label': pullMenuTooltip }"
        :model="pullMenuItems"
        @click="onPull"
      >
        <template #icon><i :class="iconClass('pull')" aria-hidden="true"></i></template>
      </SplitButton>
      <Button
        v-bind="toolbarButton"
        :icon="iconClass('push')"
        :label="compact ? '' : t('toolbar.push')"
        :title="pushTooltip"
        :aria-label="t('toolbar.push')"
        @click="push"
      />
    </template>
    <SetUpstreamDialog
      v-if="upstreamQuestion"
      :wt="wt"
      v-bind="upstreamQuestion"
      @close="upstreamQuestion = null"
    />
  </div>
</template>
