<script setup lang="ts">
import { computed } from 'vue'
import { useChangesStore } from '../stores/changes'
import { useCompareStore } from '../stores/compare'
import { useCoreCall } from '../composables/useCoreCall'
import { useOpenInEditor } from '../composables/useOpenInEditor'
import { useConfirmAction } from '../composables/useConfirmAction'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import ChangedFileList from './ChangedFileList.vue'
import CommitBox from './CommitBox.vue'
import ErrorBanner from './ErrorBanner.vue'

// ChangesPanel lists a worktree's current changes in four groups and
// stages/unstages them; selecting a row drives the diff pane via the
// changes store, replacing any active compare selection. Conflicted files
// get no selection: an unmerged path has no working-tree diff view to
// show. Its own "Mark resolved" button reuses stageFile — resolving a
// conflict is git add, the same call an ordinary stage is.
const props = defineProps<{ wt: string }>()
// Bindable, so a parent Splitter panel can shrink to match
// (WorktreeContent.vue); uncontrolled (a plain local ref) when no v-model
// is bound, e.g. in tests.
const collapsed = defineModel<boolean>('collapsed', { default: false })
const changes = useChangesStore()
const compare = useCompareStore()
const { error, run } = useCoreCall()
const { openEditor, openEditorFallback } = useOpenInEditor(() => props.wt, run)
const { confirmAction } = useConfirmAction()

function select(file: ChangedFile, staged: boolean): void {
  compare.exit()
  changes.selectFile(file, staged)
}

// write runs a stage/unstage call; the index-scope repo:changed event that
// follows reloads the list, the same way the refs and log stores refresh.
function write(call: () => Promise<void>): void {
  void run(call)
}

function stageAll(): void {
  write(() => Facade.StageAll(props.wt))
}

function unstageAll(): void {
  write(() => Facade.UnstageAll(props.wt))
}

function stageFile(file: ChangedFile): void {
  write(() => Facade.StageFile(props.wt, file.path))
}

function unstageFile(file: ChangedFile): void {
  write(() => Facade.UnstageFile(props.wt, file.path))
}

// discardFile confirms first, naming what is lost: an untracked file is
// deleted outright, a tracked one loses its uncommitted changes.
function discardFile(file: ChangedFile, untracked: boolean): void {
  const message = untracked
    ? `Discard "${file.path}"? The file will be deleted.`
    : `Discard the uncommitted changes to "${file.path}"? This cannot be undone.`
  confirmAction('Discard changes', message, () => write(() => Facade.DiscardFile(props.wt, file.path, untracked)))
}

interface Group {
  title: string
  empty: string
  files: ChangedFile[]
  staged: boolean
  actionLabel: string
  action: (file: ChangedFile) => void
  secondActionLabel?: string
  secondAction?: (file: ChangedFile) => void
}

// The three selectable groups differ only in label, action and which side
// the diff shows. Conflicted (below, in the template) is not one of them:
// it takes no @select. Only Unstaged and Untracked offer Discard — staged
// content is unstaged first, same boundary DiffView's hunk discard draws.
const groups = computed<Group[]>(() => {
  const f = changes.files
  return [
    { title: 'Staged', empty: 'No staged changes', files: f?.staged ?? [], staged: true, actionLabel: 'Unstage', action: unstageFile },
    { title: 'Unstaged', empty: 'No unstaged changes', files: f?.unstaged ?? [], staged: false, actionLabel: 'Stage', action: stageFile, secondActionLabel: 'Discard', secondAction: (file) => discardFile(file, false) },
    { title: 'Untracked', empty: 'No untracked files', files: f?.untracked ?? [], staged: false, actionLabel: 'Stage', action: stageFile, secondActionLabel: 'Discard', secondAction: (file) => discardFile(file, true) },
  ]
})
const conflicted = computed(() => changes.files?.conflicted ?? [])
</script>

<template>
  <aside class="changes-column" :class="{ collapsed }">
    <button type="button" data-test="changes-collapse" @click="collapsed = !collapsed">
      {{ collapsed ? '»' : '«' }}
    </button>
    <section v-show="!collapsed" class="changes-body">
    <header>
      <h2>Changes</h2>
      <div class="header-actions">
        <button type="button" @click="stageAll">Stage all</button>
        <button type="button" @click="unstageAll">Unstage all</button>
      </div>
    </header>
    <CommitBox :key="wt" :wt="wt" />
    <ErrorBanner v-if="error" :error="error" @fallback="openEditorFallback" />

    <template v-for="g in groups" :key="g.title">
      <h3>{{ g.title }}</h3>
      <p v-if="!g.files.length">{{ g.empty }}</p>
      <ChangedFileList
        :files="g.files"
        :action-label="g.actionLabel"
        :second-action-label="g.secondActionLabel"
        @select="(f) => select(f, g.staged)"
        @open-editor="openEditor"
        @action="g.action"
        @second-action="g.secondAction"
      />
    </template>

    <h3>Conflicted</h3>
    <p v-if="!conflicted.length">No conflicts</p>
    <ChangedFileList
      :files="conflicted"
      action-label="Mark resolved"
      @open="openEditor"
      @open-editor="openEditor"
      @action="stageFile"
    />
    </section>
  </aside>
</template>

<style scoped>
/* Sizing is the parent Splitter panel's job (WorktreeContent.vue); this
   just fills it and scrolls its own content. */
.changes-column {
  height: 100%;
  overflow: auto;
}
.changes-body {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  padding: 0.5rem 0.6rem;
}
/* The buttons move below the heading, together, when both do not fit. */
.changes-body header {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 0.35rem 0.5rem;
}
.header-actions {
  display: flex;
  gap: 0.35rem;
  white-space: nowrap;
}
</style>
