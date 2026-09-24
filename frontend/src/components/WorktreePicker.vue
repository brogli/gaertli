<script setup lang="ts">
import { computed, onMounted, ref, useTemplateRef } from 'vue'
import { useI18n } from '../i18n'
import Popover from 'openvue/popover'
import { AppService, Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import {
  Code,
  type BranchSummary,
  type Worktree,
} from '../../bindings/github.com/brogli/nebegit/internal/core'
import { useCoreCall } from '../composables/useCoreCall'
import { useOpenInEditor } from '../composables/useOpenInEditor'
import { useConfirmAction } from '../composables/useConfirmAction'
import { basename, repoPath } from '../lib/path'
import { shortSha } from '../lib/format'
import { useWindowEvent } from '../lib/windowEvents'
import { iconClass } from '../lib/icons'
import { useLayout } from '../lib/layout'
import ErrorBanner from './ErrorBanner.vue'
import CreateWorktreeDialog from './CreateWorktreeDialog.vue'
import PickerList from './PickerList.vue'
import type { PickerRow } from './pickerRow'

// WorktreePicker is the toolbar button naming this window's worktree; it
// opens a searchable list of the repo's worktrees. wt is the id every core
// call takes (the UNC form for WSL); path is this window's own worktree,
// always POSIX.
const props = defineProps<{ wt: string; path: string; distro?: string | undefined }>()
const { error, run } = useCoreCall()
const { openFolderAt } = useOpenInEditor(() => props.wt, run)
const { confirmAction } = useConfirmAction()
const { t } = useI18n()
const layout = useLayout()

const worktrees = ref<Worktree[]>([])

async function load(): Promise<void> {
  const list = await run(() => Facade.ListWorktrees(props.wt))
  if (list) worktrees.value = list
}
onMounted(load)

// repo:changed{scope:"worktrees"} covers a create, remove or prune from any
// window on this repo.
useWindowEvent('repo:changed', (ev) => {
  if (ev.data.scope === 'worktrees') void load()
})

// id is the "wt id" a shell-level call takes for one of this repo's
// worktrees: path- or worktree-routed for a local repo, its UNC form for a
// WSL one. The list itself carries POSIX paths, native to the core.
function id(path: string): string {
  return repoPath(path, props.distro)
}

function worktreeLabel(w: Worktree): string {
  return w.branch || t('worktreePicker.detachedAt', { sha: shortSha(w.sha) })
}
function isCurrent(w: Worktree): boolean {
  return w.path === props.path
}

type Row = PickerRow & { text: string } & (
    { kind: 'action'; label: string; run: () => void } | { kind: 'worktree'; worktree: Worktree }
  )
function actionRow(key: string, label: string, action: () => void): Row {
  return { key, kind: 'action', label, run: action, text: label }
}

const query = ref('')
const rows = computed<Row[]>(() => {
  const all: Row[] = [
    actionRow('new-worktree', t('worktreePicker.newWorktree'), () => void openNewWorktree()),
    ...worktrees.value.map((w): Row => ({
      key: w.path,
      kind: 'worktree',
      worktree: w,
      text: `${worktreeLabel(w)} ${basename(w.path)}`,
    })),
    ...(worktrees.value.some((w) => w.missing)
      ? [actionRow('prune', t('worktreePicker.cleanUpStale'), () => void prune())]
      : []),
  ]
  const q = query.value.trim().toLowerCase()
  return q ? all.filter((r) => r.text.toLowerCase().includes(q)) : all
})

// Each opening starts with an empty filter and this repo's worktrees listed
// afresh.
function onShow(): void {
  query.value = ''
  void load()
}

function activate(row: Row): void {
  if (row.kind === 'action') row.run()
  else if (!isCurrent(row.worktree))
    void run(() => AppService.SelectWorktree(id(row.worktree.path)))
}

const popover = useTemplateRef<InstanceType<typeof Popover>>('popover')

// closeAnd closes the popover, then runs action: for an action that leaves
// the list, e.g. for a dialog.
function closeAnd(action: () => void): void {
  popover.value?.hide()
  action()
}

function openInNewWindow(w: Worktree): void {
  closeAnd(() => void run(() => AppService.OpenPath(id(w.path))))
}

function openInEditor(w: Worktree): void {
  closeAnd(() => openFolderAt(id(w.path)))
}

const newWorktreeBranches = ref<BranchSummary[] | null>(null)
async function openNewWorktree(): Promise<void> {
  const branches = await run(() => Facade.ListBranches(props.wt))
  if (branches === undefined) return
  closeAnd(() => (newWorktreeBranches.value = branches ?? []))
}

async function prune(): Promise<void> {
  await run(() => Facade.PruneWorktrees(props.wt))
  if (!error.value) await load()
}

// Every removal is confirmed. A worktree known dirty names the loss and
// removes forced. One believed clean removes unforced; if git finds changes
// after all (DIRTY_WORKTREE), a second confirmation names the loss, then
// retries forced.
function removeWorktree(w: Worktree): void {
  if (w.dirty) {
    confirmAction(
      t('worktreePicker.removeWorktree'),
      t('worktreePicker.removeConfirmDirty', { path: w.path }),
      () => void remove(w, true),
    )
    return
  }
  confirmAction(
    t('worktreePicker.removeWorktree'),
    t('worktreePicker.removeConfirmClean', { path: w.path }),
    () => void removeThenRetryIfDirty(w),
  )
}

async function removeThenRetryIfDirty(w: Worktree): Promise<void> {
  if ((await remove(w, false)) !== Code.DirtyWorktree) return
  confirmAction(
    t('worktreePicker.removeWorktree'),
    t('worktreePicker.removeConfirmRetryDirty', { path: w.path }),
    () => void remove(w, true),
  )
}

// remove returns the refusal's code; a DIRTY_WORKTREE one is not shown, the
// caller asks again instead.
async function remove(w: Worktree, force: boolean): Promise<string | undefined> {
  await run(() => Facade.RemoveWorktree(props.wt, id(w.path), force))
  const code = error.value?.code
  if (!code) void load()
  if (code === Code.DirtyWorktree) error.value = null
  return code
}
</script>

<template>
  <button
    type="button"
    class="picker-button"
    data-test="worktree-picker"
    :title="path"
    :aria-label="path"
    @click="popover?.toggle($event)"
  >
    <i :class="iconClass('worktree')" aria-hidden="true"></i>
    <template v-if="layout !== 'narrow'">
      <span class="ellipsis">{{ basename(path) }}</span>
      <i :class="iconClass('dropdown')" aria-hidden="true"></i>
    </template>
  </button>
  <Popover ref="popover" @show="onShow">
    <div class="worktree-list">
      <ErrorBanner v-if="error" :error="error" />
      <PickerList
        v-model:query="query"
        :rows="rows"
        :placeholder="t('worktreePicker.searchPlaceholder')"
        @activate="activate"
      >
        <template #row="{ row, activate: onActivate }">
          <span
            v-if="row.kind === 'action'"
            class="row"
            data-test="picker-action"
            @click="onActivate"
            >{{ row.label }}</span
          >
          <template v-else>
            <span
              class="row"
              data-test="worktree-row"
              :title="row.worktree.path"
              @click="onActivate"
            >
              <span class="current-marker" aria-hidden="true">
                <i v-if="isCurrent(row.worktree)" :class="iconClass('current')"></i>
              </span>
              <span
                class="ellipsis"
                data-test="worktree-branch"
                :title="worktreeLabel(row.worktree)"
              >
                {{ worktreeLabel(row.worktree) }}
              </span>
              <span
                class="muted ellipsis leftover"
                data-test="worktree-folder"
                :title="row.worktree.path"
              >
                {{ basename(row.worktree.path) }}
              </span>
              <span v-if="row.worktree.main" class="muted">{{ t('worktreePicker.main') }}</span>
              <span
                v-if="row.worktree.dirty"
                class="dirty-marker"
                role="img"
                data-test="dirty-marker"
                :title="t('worktreePicker.uncommittedChanges')"
                :aria-label="t('worktreePicker.uncommittedChanges')"
              >
                <i :class="iconClass('dirty')"></i>
              </span>
              <span
                v-if="row.worktree.locked"
                role="img"
                data-test="locked-marker"
                :title="t('worktreePicker.locked')"
                :aria-label="t('worktreePicker.locked')"
              >
                <i :class="iconClass('locked')"></i>
              </span>
              <span v-if="row.worktree.missing" class="muted">{{
                t('worktreePicker.missing')
              }}</span>
            </span>
            <button
              type="button"
              class="glyph-btn"
              data-test="open-in-new-window"
              :title="t('worktreePicker.openInNewWindow')"
              :aria-label="t('worktreePicker.openInNewWindow')"
              @click="openInNewWindow(row.worktree)"
            >
              <i :class="iconClass('externalLink')" aria-hidden="true"></i>
            </button>
            <button
              v-if="!row.worktree.missing"
              type="button"
              class="glyph-btn"
              data-test="open-in-editor"
              :title="t('worktreePicker.openInEditor')"
              :aria-label="t('worktreePicker.openInEditor')"
              @click="openInEditor(row.worktree)"
            >
              <i :class="iconClass('openEditor')" aria-hidden="true"></i>
            </button>
            <button
              v-if="!row.worktree.main && !isCurrent(row.worktree)"
              type="button"
              class="glyph-btn"
              data-test="remove-worktree"
              :title="t('worktreePicker.removeWorktree')"
              :aria-label="t('worktreePicker.removeWorktree')"
              @click="removeWorktree(row.worktree)"
            >
              <i :class="iconClass('removeWorktree')" aria-hidden="true"></i>
            </button>
          </template>
        </template>
      </PickerList>
    </div>
  </Popover>
  <CreateWorktreeDialog
    v-if="newWorktreeBranches"
    :repo-path="path"
    :distro="distro"
    :branches="newWorktreeBranches"
    @close="newWorktreeBranches = null"
    @created="load"
  />
</template>

<style scoped>
.worktree-list {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  width: 30rem;
  max-width: 90vw;
  max-height: 70vh;
}
.dirty-marker i {
  color: var(--nb-tone-orange);
}
</style>
