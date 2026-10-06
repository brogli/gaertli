<script setup lang="ts">
import { computed, ref } from 'vue'
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import { SECONDARY_BUTTON } from '@/lib/layout'
import { useChangesStore } from '@/stores/changes'
import { useCoreCall } from '@/composables/useCoreCall'
import { useOpenInEditor } from '@/composables/useOpenInEditor'
import { useConfirmAction } from '@/composables/useConfirmAction'
import { useFileTreeChoice } from '@/composables/useFileTreeChoice'
import { AppService, Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { DiffKind, fileTarget } from '@/lib/diffTarget'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import ChangedFileList from './ChangedFileList.vue'
import ChangedFileTree from './ChangedFileTree.vue'
import CommitBox from './CommitBox.vue'
import ErrorBanner from './ErrorBanner.vue'
import FileTreeToggle from './FileTreeToggle.vue'
import type { ChangedFileAction } from './changedFileAction'

// ChangesView is the changes view: the change list, which scrolls on its
// own, above the commit box, which never does. A click selects a row, a
// double-click opens its side of the change in the diff window. A
// conflicted file opens in the diff window on its conflict, and "Mark
// resolved" stages it (git add); a staged file resolved since the operation
// started offers "Reopen conflict", and so does a resolved file without changes,
// listed under Conflicted. Each group shows as a flat list or a
// folder tree, the same remembered choice (useFileTreeChoice) as the commit
// detail view and the diff window.
const props = defineProps<{ wt: string }>()
const changes = useChangesStore()
const { error, run } = useCoreCall()
const { openEditor } = useOpenInEditor(() => props.wt, run)
const { confirmAction } = useConfirmAction()
const asTree = useFileTreeChoice()
const { t } = useI18n()

// The selected row, with its group's side: one path can be both staged
// and unstaged.
const selected = ref<{ file: ChangedFile; staged: boolean } | null>(null)

function openDiff(file: ChangedFile, staged: boolean): void {
  void AppService.OpenDiff(props.wt, fileTarget(file, { kind: DiffKind.WorkingDiff, staged }))
}

function openConflict(file: ChangedFile): void {
  void AppService.OpenDiff(
    props.wt,
    fileTarget(file, { kind: DiffKind.WorkingDiff, conflicted: true }),
  )
}

// reopenConflict confirms first: the resolution is replaced.
function reopenConflict(file: ChangedFile): void {
  confirmAction(
    t('changesView.reopenConflict'),
    t('changesView.reopenConflictConfirm', { path: file.path }),
    () => void run(() => Facade.ReopenConflict(props.wt, file.path)),
  )
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
  secondAction?: (file: ChangedFile) => ChangedFileAction | undefined
  onSecondAction?: (file: ChangedFile) => void
}

// The three selectable groups differ only in their action and which side
// the diff shows. Conflicted (below, in the template) is not one of them.
// Only Unstaged and Untracked offer Discard — staged content is unstaged
// first, same boundary DiffView's hunk discard draws.
const resolved = computed(() => new Set(changes.files?.resolved ?? []))
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
      secondAction: (file) => (resolved.value.has(file.path) ? 'reopenConflict' : undefined),
      onSecondAction: reopenConflict,
    },
    {
      title: t('changesView.unstagedTitle'),
      empty: t('changesView.unstagedEmpty'),
      files: f?.unstaged ?? [],
      staged: false,
      action: 'stage',
      onAction: stageFile,
      secondAction: () => 'discard',
      onSecondAction: (file) => discardFile(file, false),
    },
    {
      title: t('changesView.untrackedTitle'),
      empty: t('changesView.untrackedEmpty'),
      files: f?.untracked ?? [],
      staged: false,
      action: 'stage',
      onAction: stageFile,
      secondAction: () => 'discard',
      onSecondAction: (file) => discardFile(file, true),
    },
  ]
})
const conflicted = computed(() => changes.files?.conflicted ?? [])
// A file resolved to the current side's version has no change and is in no
// other group; it stays listed so it can be reopened.
const resolvedUnchanged = computed<ChangedFile[]>(() => {
  const listed = new Set(
    groups.value
      .flatMap((g) => g.files)
      .concat(conflicted.value)
      .map((file) => file.path),
  )
  return [...resolved.value]
    .filter((path) => !listed.has(path))
    .map((path) => ({ path, status: 'M' }))
})
</script>

<template>
  <!-- Sizing is the parent layout's job (WorktreeContent.vue). -->
  <aside class="changes-column flex h-full min-h-0 flex-col">
    <section class="changes-body flex flex-1 flex-col gap-3 overflow-auto py-2 px-2.5">
      <!-- The buttons move below the heading, together, when both do not fit. -->
      <header class="flex flex-wrap items-center justify-between gap-x-2 gap-y-1.5">
        <h2>{{ t('changesView.heading') }}</h2>
        <div class="flex gap-1.5 whitespace-nowrap">
          <Button v-bind="SECONDARY_BUTTON" :label="t('changesView.stageAll')" @click="stageAll" />
          <Button
            v-bind="SECONDARY_BUTTON"
            :label="t('changesView.unstageAll')"
            @click="unstageAll"
          />
          <FileTreeToggle v-model:as-tree="asTree" />
        </div>
      </header>
      <ErrorBanner v-if="error" :error="error" />

      <section v-for="g in groups" :key="g.title" class="flex flex-col gap-1">
        <h3>{{ g.title }}</h3>
        <p v-if="!g.files.length">{{ g.empty }}</p>
        <component
          :is="asTree ? ChangedFileTree : ChangedFileList"
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
      </section>

      <section class="flex flex-col gap-1">
        <h3>{{ t('changesView.conflictedTitle') }}</h3>
        <p v-if="!conflicted.length && !resolvedUnchanged.length">
          {{ t('changesView.conflictedEmpty') }}
        </p>
        <component
          :is="asTree ? ChangedFileTree : ChangedFileList"
          :files="conflicted"
          action="markResolved"
          @open="openConflict"
          @open-editor="openEditor"
          @action="stageFile"
        />
        <component
          :is="asTree ? ChangedFileTree : ChangedFileList"
          v-if="resolvedUnchanged.length"
          :files="resolvedUnchanged"
          :second-action="() => 'reopenConflict'"
          @open-editor="openEditor"
          @second-action="reopenConflict"
        />
      </section>
    </section>
    <CommitBox :key="wt" :wt="wt" class="flex-none border-t border-surface py-2 px-2.5" />
  </aside>
</template>
