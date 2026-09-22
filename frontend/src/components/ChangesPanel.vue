<script setup lang="ts">
import { computed, ref } from 'vue'
import { useChangesStore } from '../stores/changes'
import { useCompareStore } from '../stores/compare'
import { useCoreCall } from '../composables/useCoreCall'
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
const collapsed = ref(false)
const changes = useChangesStore()
const compare = useCompareStore()
const { error, run } = useCoreCall()

function select(file: ChangedFile, staged: boolean): void {
  compare.exit()
  changes.selectFile(file, staged)
}

function openEditor(file: ChangedFile): void {
  void run(() => Facade.OpenFile(props.wt, file.path, 0))
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

// The three selectable groups differ only in label, action and which side
// the diff shows. Conflicted (below, in the template) is not one of them:
// it takes no @select.
const groups = computed(() => {
  const f = changes.files
  return [
    { title: 'Staged', empty: 'No staged changes', files: f?.staged ?? [], staged: true, actionLabel: 'Unstage', action: unstageFile },
    { title: 'Unstaged', empty: 'No unstaged changes', files: f?.unstaged ?? [], staged: false, actionLabel: 'Stage', action: stageFile },
    { title: 'Untracked', empty: 'No untracked files', files: f?.untracked ?? [], staged: false, actionLabel: 'Stage', action: stageFile },
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
      <button type="button" @click="stageAll">Stage all</button>
      <button type="button" @click="unstageAll">Unstage all</button>
    </header>
    <CommitBox :key="wt" :wt="wt" />
    <ErrorBanner v-if="error" :error="error" />

    <template v-for="g in groups" :key="g.title">
      <h3>{{ g.title }}</h3>
      <p v-if="!g.files.length">{{ g.empty }}</p>
      <ChangedFileList
        :files="g.files"
        :action-label="g.actionLabel"
        @select="(f) => select(f, g.staged)"
        @open-editor="openEditor"
        @action="g.action"
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
/* A resizable column like the selector panel; collapsed, only the toggle stays. */
.changes-column {
  width: 22%;
  min-width: 12em;
  resize: horizontal;
  overflow: auto;
}
.changes-column.collapsed {
  width: auto;
  min-width: 0;
  resize: none;
}
</style>
