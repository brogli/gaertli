<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from '../i18n'
import { onKeyStroke, useLocalStorage } from '@vueuse/core'
import Button from 'openvue/button'
import SplitButton from 'openvue/splitbutton'
import Select from 'openvue/select'
import type { MenuItem } from 'openvue/menuitem'
import { useRefsStore } from '../stores/refs'
import { useSettingsStore } from '../stores/settings'
import { useConfirmAction } from '../composables/useConfirmAction'
import { useCoreCall } from '../composables/useCoreCall'
import { useJobNotices } from '../composables/useJobNotices'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { PullMode } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { iconClass } from '../lib/icons'
import { useLayout, useToolbarButton } from '../lib/layout'
import ErrorBanner from './ErrorBanner.vue'

// SyncControls is the Fetch/Pull/Push controls: it only starts jobs and
// offers what can act. JobsMenu shows their progress; useJobNotices shows
// their results.
const props = defineProps<{ wt: string; compact?: boolean }>()
const refs = useRefsStore()
const settings = useSettingsStore()
const { confirmAction } = useConfirmAction()
const { error, run } = useCoreCall()
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

// The mode git config names for the current branch, "" if none. An answer
// that arrives after the branch changed again is dropped.
const configuredPullMode = ref<PullMode>(PullMode.$zero)
watch(
  () => refs.current?.name,
  async (branch, _, onCleanup) => {
    if (!branch) return
    let stale = false
    onCleanup(() => (stale = true))
    const mode = await run(() => Facade.ConfiguredPullMode(props.wt))
    if (!stale) configuredPullMode.value = mode ?? PullMode.$zero
  },
  { immediate: true },
)

// The last pull mode chosen, kept across reloads and restarts; until one
// is chosen, the configured mode, else the default pull-mode setting.
const lastPullMode = useLocalStorage<PullMode | ''>('nebegit.pullMode', '')
const pullMode = computed<PullMode>({
  get: () =>
    lastPullMode.value ||
    configuredPullMode.value ||
    (settings.settings?.defaultPullMode === PullMode.PullRebase
      ? PullMode.PullRebase
      : PullMode.PullMerge),
  set: (mode) => {
    lastPullMode.value = mode
  },
})
const pullLabel = computed(() =>
  pullMode.value === PullMode.PullRebase ? t('toolbar.pullRebase') : t('toolbar.pullMerge'),
)
const pullMenuItems = computed<MenuItem[]>(() => [
  { label: t('toolbar.pullWithMerge'), command: () => runPull(PullMode.PullMerge) },
  { label: t('toolbar.pullWithRebase'), command: () => runPull(PullMode.PullRebase) },
])

const upstreamRemote = ref('')

// Preselects "origin" once its remotes are known, else the first remote;
// also fires when refs load after mount. Leaves a value the user already
// picked alone.
watch(
  remoteNames,
  (names) => {
    upstreamRemote.value ||= names.includes('origin') ? 'origin' : (names[0] ?? '')
  },
  { immediate: true },
)

function onFetch(): void {
  void run(() => Facade.Fetch(props.wt, ''))
}
function runPull(mode: PullMode): void {
  pullMode.value = mode
  void run(() => Facade.Pull(props.wt, mode))
}
function onPull(): void {
  runPull(pullMode.value)
}
function onPush(): void {
  void run(() => Facade.Push(props.wt, refs.current?.upstream ? '' : upstreamRemote.value))
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
    if (refs.current?.upstream) onPush()
  },
)

useJobNotices({ onPull, onForcePush, onStashPull })
</script>

<template>
  <div class="sync-controls" :class="{ narrow }">
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
    <template v-else-if="refs.current!.upstream">
      <SplitButton
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
        @click="onPush"
      />
    </template>
    <template v-else>
      <Select
        v-if="remoteNames.length > 1"
        v-model="upstreamRemote"
        :options="remoteNames"
        data-test="upstream-remote"
      />
      <Button
        v-bind="toolbarButton"
        :icon="iconClass('push')"
        :label="compact ? '' : t('toolbar.pushAndSetUpstream')"
        :disabled="!upstreamRemote"
        :title="remoteNames.length === 1 ? remoteNames[0] : t('toolbar.pushAndSetUpstream')"
        :aria-label="t('toolbar.pushAndSetUpstream')"
        @click="onPush"
      />
    </template>
  </div>
</template>

<style scoped>
.sync-controls {
  display: flex;
  align-items: center;
  gap: 0.5em;
}
.sync-controls.narrow {
  gap: 0.25rem;
}
</style>
