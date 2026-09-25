<script setup lang="ts">
import { computed, ref } from 'vue'
import Button from 'openvue/button'
import { useI18n } from '../i18n'
import { SECONDARY_BUTTON } from '../lib/layout'
import { useChangesStore } from '../stores/changes'
import { useCoreCall } from '../composables/useCoreCall'
import { useOpenInEditor } from '../composables/useOpenInEditor'
import { useConfirmAction } from '../composables/useConfirmAction'
import { AppService, Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { DiffKind, fileTarget } from '../lib/diffTarget'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import ChangedFileList from './ChangedFileList.vue'
import CommitBox from './CommitBox.vue'
import ErrorBanner from './ErrorBanner.vue'
import type { ChangedFileAction } from './changedFileAction'

// ChangesView is the changes view: the change list, which scrolls on its
// own, above the commit box, which never does. A click selects a row, a
// double-click opens its side of the change in the diff window. A
// conflicted file has no working-tree diff: it opens in the editor, and
// "Mark resolved" stages it (git add).
const props = defineProps<{ wt: string }>()
const changes = useChangesStore()
const { error, run } = useCoreCall()
const { openEditor } = useOpenInEditor(() => props.wt, run)
const { confirmAction } = useConfirmAction()
const { t } = useI18n()

// The selected row, with its group's side: one path can be both staged
// and unstaged.
const selected = ref<{ file: ChangedFile; staged: boolean } | null>(null)

function openDiff(file: ChangedFile, staged: boolean): void {
  void AppService.OpenDiff(props.wt, fileTarget(file, { kind: DiffKind.WorkingDiff, staged }))
}

// The index-scope repo:changed event after each write reloads the list.
function stageAll(): void {
  void run(() => Facade.StageAll(props.wt))
}

function unstageAll(): void {
  void run(() => Facade.UnstageAll(props.wt))
}

function stageFile(file: ChangedFile): void {
  void run(() => Facade.StageFile(props.wt, file.path))
}

function unstageFile(file: ChangedFile): void {
  void run(() => Facade.UnstageFile(props.wt, file.path))
}

// discardFile confirms first, naming what is lost: an untracked file is
// deleted outright, a tracked one loses its uncommitted changes.
function discardFile(file: ChangedFile, untracked: boolean): void {
  const message = untracked
    ? t('changesView.discardUntrackedConfirm', { path: file.path })
    : t('changesView.discardTrackedConfirm', { path: file.path })
  confirmAction(
    t('changesView.discardTitle'),
    message,
    () => void run(() => Facade.DiscardFile(props.wt, file.path, untracked)),
  )
}

interface Group {
  title: string
  empty: string
  files: ChangedFile[]
  staged: boolean
  action: ChangedFileAction
  onAction: (file: ChangedFile) => void
  secondAction?: ChangedFileAction
  onSecondAction?: (file: ChangedFile) => void
}

// The three selectable groups differ only in their action and which side
// the diff shows. Conflicted (below, in the template) is not one of them.
// Only Unstaged and Untracked offer Discard — staged content is unstaged
// first, same boundary DiffView's hunk discard draws.
const groups = computed<Group[]>(() => {
  const f = changes.files
  return [
    {
      title: t('changesView.stagedTitle'),
      empty: t('changesView.stagedEmpty'),
      files: f?.staged ?? [],
      staged: true,
      action: 'unstage',
      onAction: unstageFile,
    },
    {
      title: t('changesView.unstagedTitle'),
      empty: t('changesView.unstagedEmpty'),
      files: f?.unstaged ?? [],
      staged: false,
      action: 'stage',
      onAction: stageFile,
      secondAction: 'discard',
      onSecondAction: (file) => discardFile(file, false),
    },
    {
      title: t('changesView.untrackedTitle'),
      empty: t('changesView.untrackedEmpty'),
      files: f?.untracked ?? [],
      staged: false,
      action: 'stage',
      onAction: stageFile,
      secondAction: 'discard',
      onSecondAction: (file) => discardFile(file, true),
    },
  ]
})
const conflicted = computed(() => changes.files?.conflicted ?? [])
</script>

<template>
  <aside class="changes-column">
    <section class="changes-body">
      <header>
        <h2>{{ t('changesView.heading') }}</h2>
        <div class="header-actions">
          <Button v-bind="SECONDARY_BUTTON" :label="t('changesView.stageAll')" @click="stageAll" />
          <Button
            v-bind="SECONDARY_BUTTON"
            :label="t('changesView.unstageAll')"
            @click="unstageAll"
          />
        </div>
      </header>
      <ErrorBanner v-if="error" :error="error" />

      <template v-for="g in groups" :key="g.title">
        <h3>{{ g.title }}</h3>
        <p v-if="!g.files.length">{{ g.empty }}</p>
        <ChangedFileList
          :files="g.files"
          :selected="selected?.staged === g.staged ? selected.file : null"
          :action="g.action"
          :second-action="g.secondAction"
          @select="(file) => (selected = { file, staged: g.staged })"
          @open="(file) => openDiff(file, g.staged)"
          @open-editor="openEditor"
          @action="g.onAction"
          @second-action="g.onSecondAction"
        />
      </template>

      <h3>{{ t('changesView.conflictedTitle') }}</h3>
      <p v-if="!conflicted.length">{{ t('changesView.conflictedEmpty') }}</p>
      <ChangedFileList
        :files="conflicted"
        action="markResolved"
        @open="openEditor"
        @open-editor="openEditor"
        @action="stageFile"
      />
    </section>
    <CommitBox :key="wt" :wt="wt" />
  </aside>
</template>

<style scoped>
/* Sizing is the parent layout's job (WorktreeContent.vue). The change list
   scrolls on its own; the commit box below it does not. */
.changes-column {
  height: 100%;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.changes-body {
  flex: 1;
  overflow: auto;
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
.changes-column > :deep(.commit-box) {
  flex: none;
  padding: 0.5rem 0.6rem;
  border-top: 1px solid var(--p-content-border-color);
}
</style>
