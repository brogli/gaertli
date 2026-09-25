<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from '../i18n'
import { onKeyStroke } from '@vueuse/core'
import Splitter from 'openvue/splitter'
import SplitterPanel from 'openvue/splitterpanel'
import SelectButton from 'openvue/selectbutton'
import Button from 'openvue/button'
import { useRepoStore } from '../stores/repo'
import { useRefsStore } from '../stores/refs'
import { useLogStore } from '../stores/log'
import { useCompareStore } from '../stores/compare'
import { useChangesStore } from '../stores/changes'
import { useCoreCall } from '../composables/useCoreCall'
import { useOpenInEditor } from '../composables/useOpenInEditor'
import { useAutoFetch } from '../composables/useAutoFetch'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { ChangeScope } from '../../bindings/github.com/brogli/nebegit/internal/core'
import ErrorBanner from '../components/ErrorBanner.vue'
import JobsMenu from '../components/JobsMenu.vue'
import LogView from '../components/LogView.vue'
import CommitDetailView from '../components/CommitDetailView.vue'
import ChangesView from '../components/ChangesView.vue'
import ConflictBanner from '../components/ConflictBanner.vue'
import StashBanner from '../components/StashBanner.vue'
import BranchPicker from '../components/BranchPicker.vue'
import RepoPicker from '../components/RepoPicker.vue'
import WorktreePicker from '../components/WorktreePicker.vue'
import CreateBranchDialog from '../components/CreateBranchDialog.vue'
import DetachWarningDialog from '../components/DetachWarningDialog.vue'
import BranchInWorktreeDialog from '../components/BranchInWorktreeDialog.vue'
import ResetDialog from '../components/ResetDialog.vue'
import CreateWorktreeDialog from '../components/CreateWorktreeDialog.vue'
import SyncControls from '../components/SyncControls.vue'
import { commitStart, type StartPoint, type WorktreeStart } from '../components/startPoint'
import { repoPath } from '../lib/path'
import { useRepoChanged, useWindowEvent } from '../lib/windowEvents'
import { iconClass } from '../lib/icons'
import { useLayout, useToolbarButton } from '../lib/layout'

const props = defineProps<{ path: string; distro?: string | undefined }>()
const emit = defineEmits<{ 'open-settings': [] }>()
const { t } = useI18n()
// The id every core call and child takes: path for a local worktree, its
// UNC form for a WSL one. path stays what the window shows.
const wt = repoPath(props.path, props.distro)
const repo = useRepoStore()
const refs = useRefsStore()
const log = useLogStore()
const compare = useCompareStore()
const changes = useChangesStore()
const layout = useLayout()
const toolbarButton = useToolbarButton()

const { error, run } = useCoreCall()
const { openFolder: openInEditor } = useOpenInEditor(wt, run)
useAutoFetch(() => wt)

// The dialogs the log view's context menu and the branch picker open, one
// instance each, and what they start at.
const createBranchStart = ref<StartPoint | null>(null)
const createWorktreeStart = ref<WorktreeStart | null>(null)
const checkoutCommitSha = ref<string | null>(null)
const checkoutElsewhere = ref<{ branch: string; path: string } | null>(null)
const resetTarget = ref<{ sha: string; subject: string } | null>(null)

onMounted(() => {
  void run(() => repo.loadSummary(wt))
  void refs.load(wt)
  void changes.load(wt)
})

// repo:changed{scope:"index"} covers this worktree's staging writes and
// outside changes to the working tree.
useRepoChanged([ChangeScope.IndexChanged], () => void changes.load(wt))

// A restarted WSL core knows nothing of this window: it gets the commit
// message being typed, then the window reloads from scratch.
useWindowEvent('core:reconnected', () => {
  void Facade.SetMessageDraft(wt, changes.message).finally(() => window.location.reload())
})

// Manual refresh: if anything differs, repo:changed arrives the normal way.
function refresh(): void {
  void run(() => Facade.Refresh(wt))
}

// F5 needs its own preventDefault: unlike a browser tab, the WebView would
// otherwise reload the whole page.
onKeyStroke('F5', (event) => {
  event.preventDefault()
  refresh()
})

// labelled with the effective editor profile's name once the summary loads
const editorLabel = computed(() =>
  t('toolbar.openInEditor', { editor: repo.summary?.editorName || t('toolbar.editorFallback') }),
)

// "Create branch here" while detached: its button renders only with a
// loaded summary.
function openCreateBranchHere(): void {
  createBranchStart.value = commitStart(repo.summary!.sha ?? '')
}

// The view the medium layout's side panel, or the narrow layout's full
// width, shows; 'log' is only ever chosen in narrow. Below wide, the
// switch picks it; selecting a commit picks Details too, except in narrow,
// where only opening it (LogView's open) does. It starts on Details in
// medium, Log in narrow.
type ActiveView = 'log' | 'details' | 'changes'
const view = ref<ActiveView>(layout.value === 'narrow' ? 'log' : 'details')

watch([() => log.selectedRow, () => compare.active, () => compare.workingCommit], () => {
  if (layout.value !== 'narrow') view.value = 'details'
})

// LogView's own events, shared by every layout.
const logViewListeners = {
  'create-branch': (sha: string) => (createBranchStart.value = commitStart(sha)),
  'checkout-commit': (sha: string) => (checkoutCommitSha.value = sha),
  reset: (sha: string, subject: string) => (resetTarget.value = { sha, subject }),
  'create-worktree': (sha: string) =>
    (createWorktreeStart.value = { newBranch: true, start: commitStart(sha) }),
  open: () => (view.value = 'details'),
}

// The branch picker's Show in log and Show history: only narrow can hide
// the log view.
function showLog(): void {
  if (layout.value === 'narrow') view.value = 'log'
}

// (n) beside "Changes" in the switch: every file the changes view lists,
// absent at 0.
const changesCount = computed(() => {
  const f = changes.files
  return (
    (f?.staged?.length ?? 0) +
    (f?.unstaged?.length ?? 0) +
    (f?.untracked?.length ?? 0) +
    (f?.conflicted?.length ?? 0)
  )
})
const changesLabel = computed(() =>
  changesCount.value
    ? t('layout.switchChangesCount', { n: changesCount.value })
    : t('layout.switchChanges'),
)

const sideOptions = computed(() => [
  { label: t('layout.switchDetails'), value: 'details' as const },
  { label: changesLabel.value, value: 'changes' as const },
])
const narrowOptions = computed(() => [
  { label: t('layout.switchLog'), value: 'log' as const },
  ...sideOptions.value,
])

// The medium layout's side panel never shows Log; a stale 'log' value
// (carried over from narrow, before a resize) falls back to Details.
const sideView = computed<'details' | 'changes'>(() =>
  view.value === 'changes' ? 'changes' : 'details',
)

// Below wide, toolbar buttons keep their name in title/aria-label and drop
// it as visible text, so the toolbar fits down to the window's own
// minimum width.
const compact = computed(() => layout.value !== 'wide')

const narrow = computed(() => layout.value === 'narrow')
</script>

<template>
  <main>
    <header class="toolbar" :class="{ narrow }">
      <RepoPicker :path="path" :distro="distro" />
      <WorktreePicker :wt="wt" :path="path" :distro="distro" />
      <BranchPicker
        :wt="wt"
        :distro="distro"
        @create-branch="createBranchStart = $event"
        @create-worktree="createWorktreeStart = $event"
        @checkout-commit="checkoutCommitSha = $event"
        @checkout-elsewhere="(branch, path) => (checkoutElsewhere = { branch, path })"
        @show-log="showLog"
      />
      <Button
        v-if="repo.summary?.detached"
        v-bind="toolbarButton"
        :icon="iconClass('branch')"
        :label="compact ? '' : t('toolbar.createBranchHere')"
        :title="t('toolbar.createBranchHere')"
        :aria-label="t('toolbar.createBranchHere')"
        @click="openCreateBranchHere"
      />
      <SyncControls :wt="wt" :compact="compact" />
      <ErrorBanner v-if="error" :error="error" />
      <div class="toolbar-spacer"></div>
      <JobsMenu :wt="wt" />
      <Button
        v-bind="toolbarButton"
        :icon="iconClass('refresh')"
        :label="compact ? '' : t('toolbar.refresh')"
        :title="t('toolbar.refresh')"
        :aria-label="t('toolbar.refresh')"
        @click="refresh"
      />
      <Button
        v-bind="toolbarButton"
        :icon="iconClass('externalLink')"
        :label="compact ? '' : editorLabel"
        :title="editorLabel"
        :aria-label="editorLabel"
        @click="openInEditor"
      />
      <Button
        v-bind="toolbarButton"
        data-test="open-settings"
        :icon="iconClass('settings')"
        :title="t('toolbar.settings')"
        :aria-label="t('toolbar.settings')"
        @click="emit('open-settings')"
      />
    </header>
    <ConflictBanner :wt="wt" />
    <StashBanner :wt="wt" />
    <Splitter
      v-if="layout === 'wide'"
      layout="horizontal"
      class="worktree-body"
      state-key="nebegit.layout.wide"
      state-storage="local"
    >
      <SplitterPanel :size="50" :min-size="20">
        <LogView :wt="wt" v-on="logViewListeners" />
      </SplitterPanel>
      <SplitterPanel :size="25" :min-size="15">
        <CommitDetailView :wt="wt" />
      </SplitterPanel>
      <SplitterPanel :size="25" :min-size="15">
        <ChangesView :wt="wt" />
      </SplitterPanel>
    </Splitter>
    <Splitter
      v-else-if="layout === 'medium'"
      layout="horizontal"
      class="worktree-body"
      state-key="nebegit.layout.medium"
      state-storage="local"
    >
      <SplitterPanel :size="60" :min-size="25">
        <LogView :wt="wt" v-on="logViewListeners" />
      </SplitterPanel>
      <SplitterPanel :size="40" :min-size="20" class="side-column">
        <SelectButton
          v-model="view"
          :options="sideOptions"
          option-label="label"
          option-value="value"
          :allow-empty="false"
        />
        <div v-show="sideView === 'details'" class="side-view">
          <CommitDetailView :wt="wt" />
        </div>
        <div v-show="sideView === 'changes'" class="side-view">
          <ChangesView :wt="wt" />
        </div>
      </SplitterPanel>
    </Splitter>
    <div v-else class="worktree-body narrow-body">
      <SelectButton
        v-model="view"
        :options="narrowOptions"
        option-label="label"
        option-value="value"
        :allow-empty="false"
      />
      <div v-show="view === 'log'" class="narrow-view">
        <LogView :wt="wt" v-on="logViewListeners" />
      </div>
      <div v-show="view === 'details'" class="narrow-view">
        <CommitDetailView :wt="wt" />
      </div>
      <div v-show="view === 'changes'" class="narrow-view">
        <ChangesView :wt="wt" />
      </div>
    </div>
    <CreateBranchDialog
      v-if="createBranchStart"
      :wt="wt"
      :start="createBranchStart"
      @close="createBranchStart = null"
    />
    <DetachWarningDialog
      v-if="checkoutCommitSha !== null"
      :wt="wt"
      :sha="checkoutCommitSha"
      @close="checkoutCommitSha = null"
    />
    <BranchInWorktreeDialog
      v-if="checkoutElsewhere"
      :branch="checkoutElsewhere.branch"
      :path="checkoutElsewhere.path"
      :distro="distro"
      @close="checkoutElsewhere = null"
    />
    <ResetDialog
      v-if="resetTarget !== null"
      :wt="wt"
      :branch="repo.summary?.branch ?? ''"
      :sha="resetTarget.sha"
      :subject="resetTarget.subject"
      @close="resetTarget = null"
    />
    <CreateWorktreeDialog
      v-if="createWorktreeStart"
      :repo-path="path"
      :distro="distro"
      :start="createWorktreeStart"
      @close="createWorktreeStart = null"
    />
  </main>
</template>

<style scoped>
/* The toolbar on top; below it the three views, laid out by width. Their
   virtual scroll and internal scrolling need a bounded viewport: fill the
   app (style.css's height chain), give the body the leftover space. */
main {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}
/* One compact row: the pickers and sync controls (or an error beside
   them), then the actions, pushed right by the spacer. */
.toolbar {
  display: flex;
  flex: none;
  align-items: center;
  gap: 0.5rem;
  padding: 0.4rem 0.6rem;
  border-bottom: 1px solid var(--p-content-border-color);
  white-space: nowrap;
}
.toolbar-spacer {
  flex: 1;
}
.toolbar :deep(.error-banner) {
  flex: 1;
  min-width: 0;
}
/* Below the medium width, every control shrinks to icon-only: a tighter
   gap buys back the room the three pickers need. */
.toolbar.narrow {
  gap: 0.25rem;
}
.worktree-body {
  display: flex;
  flex: 1;
  min-height: 0;
}
/* The medium layout's side panel: the switch fixed on top, the chosen
   view below filling the rest. The narrow layout's single view is the
   same shape, full width instead of a side panel. */
.side-column,
.narrow-body {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}
.side-view,
.narrow-view {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
</style>
