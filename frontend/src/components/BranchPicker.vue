<script setup lang="ts">
import { computed, nextTick, ref, useTemplateRef } from 'vue'
import { useI18n } from '../i18n'
import Popover from 'openvue/popover'
import Menu from 'openvue/menu'
import type { MenuItem } from 'openvue/menuitem'
import { AppService, Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { Code, type Branch } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { useRefsStore } from '../stores/refs'
import { useRepoStore } from '../stores/repo'
import { useLogStore } from '../stores/log'
import { useCoreCall } from '../composables/useCoreCall'
import { useConfirmAction } from '../composables/useConfirmAction'
import { repoPath } from '../lib/path'
import { shortSha } from '../lib/format'
import { iconClass } from '../lib/icons'
import { useLayout } from '../lib/layout'
import ErrorBanner from './ErrorBanner.vue'
import PickerList from './PickerList.vue'
import RenameBranchDialog from './RenameBranchDialog.vue'
import type { PickerRow } from './pickerRow'
import type { WorktreeStart } from './worktreeStart'

// BranchPicker is the toolbar button naming the current branch; it opens a
// searchable list of the local branches, remote branches and tags. A click
// checks an entry out; its actions button opens a menu of the rest. The
// dialogs another component already hosts are asked for by event: a new
// branch at a start point, a new worktree, a detached checkout.
// distro is set only for a WSL window; a path from the core is POSIX there.
const props = defineProps<{ wt: string; distro?: string | undefined }>()
const emit = defineEmits<{
  'create-branch': [start: string]
  'create-worktree': [start: WorktreeStart]
  'checkout-commit': [sha: string]
}>()
const refs = useRefsStore()
const repo = useRepoStore()
const log = useLogStore()
const { error, run, runAndClose } = useCoreCall()
const { confirmAction } = useConfirmAction()
const { t } = useI18n()
const layout = useLayout()

// The button's text: the branch, or the short hash while detached, and any
// operation in progress.
const headLabel = computed(() => {
  const summary = repo.summary
  if (!summary) return ''
  const head = summary.detached
    ? t('branchPicker.detachedLabel', { sha: shortSha(summary.sha ?? '') })
    : summary.branch
  return summary.operation
    ? t('branchPicker.operationLabel', { head, operation: summary.operation })
    : head
})

// The current branch, if it tracks an upstream that still exists.
const tracked = computed(() => {
  const current = refs.current
  return current?.upstream && !current.upstreamGone ? current : undefined
})

// RefRow is one checkout-able entry; name is what the list shows and the
// filter matches, e.g. "origin/x" for a remote branch.
type RefRow = PickerRow & { name: string; sha: string } & (
    { kind: 'local'; branch: Branch } | { kind: 'remote'; shortName: string } | { kind: 'tag' }
  )
type ActionRow = PickerRow & { kind: 'action'; label: string; run: () => void }
type SectionRow = PickerRow & { kind: 'section'; label: string }
type Row = RefRow | ActionRow | SectionRow

function actionRow(key: string, label: string, run: () => void): ActionRow {
  return { key, kind: 'action', label, run }
}

const sections = computed<{ label: string; rows: RefRow[] }[]>(() => [
  {
    label: t('branchPicker.local'),
    rows: (refs.refs?.branches ?? []).map((b) => ({
      key: `local:${b.name}`,
      kind: 'local',
      name: b.name,
      sha: b.sha,
      branch: b,
    })),
  },
  {
    label: t('branchPicker.remote'),
    rows: (refs.refs?.remotes ?? []).flatMap((r) =>
      (r.branches ?? []).map((b): RefRow => ({
        key: `remote:${r.name}/${b.name}`,
        kind: 'remote',
        name: `${r.name}/${b.name}`,
        sha: b.sha,
        shortName: b.name,
      })),
    ),
  },
  {
    label: t('branchPicker.tags'),
    rows: (refs.refs?.tags ?? []).map((tag) => ({
      key: `tag:${tag.name}`,
      kind: 'tag',
      name: tag.name,
      sha: tag.sha,
    })),
  },
])

// Local branches with an upstream, other than the current one, can be
// checked to update several at once.
function updatable(row: RefRow): boolean {
  return row.kind === 'local' && !row.branch.current && !!row.branch.upstream
}
const checked = ref(new Set<string>())

function setChecked(name: string, on: boolean): void {
  if (on) checked.value.add(name)
  else checked.value.delete(name)
}

const query = ref('')
const rows = computed<Row[]>(() => {
  const q = query.value.trim().toLowerCase()
  const matches = (text: string) => text.toLowerCase().includes(q)
  const actions = [
    actionRow('new-branch', t('branchPicker.newBranch'), () =>
      closeAnd(() => emit('create-branch', repo.summary?.sha ?? '')),
    ),
    ...(checked.value.size >= 2
      ? [
          actionRow(
            'update-checked',
            t('branchPicker.updateChecked', { n: checked.value.size }),
            () => void updateChecked(),
          ),
        ]
      : []),
  ]
  return [
    ...actions.filter((a) => matches(a.label)),
    ...sections.value.flatMap(({ label, rows }): Row[] => {
      const kept = rows.filter((r) => matches(r.name))
      return kept.length ? [{ key: label, kind: 'section', label, header: true }, ...kept] : []
    }),
  ]
})

const popover = useTemplateRef<InstanceType<typeof Popover>>('popover')

function hide(): void {
  popover.value?.hide()
}

function closeAnd(action: () => void): void {
  hide()
  action()
}

function activate(row: Row): void {
  if (row.kind === 'action') row.run()
  else if (row.kind !== 'section') checkout(row)
}

// A local branch checked out in another worktree switches this window to
// that worktree instead. A remote branch checks out by its short name: git's
// DWIM makes it a new tracking branch. A tag detaches HEAD, after the warning.
// A refusal stays in the list with git's reason.
function checkout(row: RefRow): void {
  if (row.kind === 'local' && row.branch.current) hide()
  else if (row.kind === 'local' && row.branch.checkedOutIn) {
    const path = repoPath(row.branch.checkedOutIn, props.distro)
    void runAndClose(() => AppService.SelectWorktree(path), hide)
  } else if (row.kind === 'local') void runAndClose(() => Facade.Checkout(props.wt, row.name), hide)
  else if (row.kind === 'remote')
    void runAndClose(() => Facade.Checkout(props.wt, row.shortName), hide)
  else closeAnd(() => emit('checkout-commit', row.sha))
}

// A local branch checked out somewhere cannot get a second worktree; a new
// branch starts at it instead, as for a remote branch or a tag.
function worktreeStart(row: RefRow): WorktreeStart {
  return row.kind === 'local' && !row.branch.current && !row.branch.checkedOutIn
    ? { newBranch: false, branch: row.name }
    : { newBranch: true, start: row.name }
}

function menuItems(row: RefRow): MenuItem[] {
  const local = row.kind === 'local' ? row.branch : undefined
  const other = !local?.current
  return [
    ...(other ? [{ label: t('branchPicker.checkOut'), command: () => checkout(row) }] : []),
    {
      label: t('branchPicker.newBranchFromHere'),
      command: () => closeAnd(() => emit('create-branch', row.name)),
    },
    {
      label: t('branchPicker.newWorktreeFromHere'),
      command: () => closeAnd(() => emit('create-worktree', worktreeStart(row))),
    },
    ...(local && other && local.upstream
      ? [
          {
            label: t('branchPicker.updateFromUpstream'),
            command: () => void runAndClose(() => Facade.UpdateBranch(props.wt, row.name), hide),
          },
        ]
      : []),
    ...(local && other
      ? [
          {
            label: t('branchPicker.rename'),
            command: () => closeAnd(() => (renameTarget.value = row.name)),
          },
          { label: t('branchPicker.delete'), command: () => void deleteBranch(row.name) },
        ]
      : []),
    { label: t('branchPicker.showInLog'), command: () => closeAnd(() => void log.jumpTo(row.sha)) },
  ]
}

const menu = useTemplateRef<InstanceType<typeof Menu>>('menu')
const menuModel = ref<MenuItem[]>([])
// menuButton is the actions button whose menu is open.
let menuButton: EventTarget | null = null

// A second click on the same button closes its menu. Moving to another
// row's button shows the menu anew on the next tick: Menu positions itself
// only when it enters, not when shown again while still open.
function openMenu(event: MouseEvent, row: RefRow): void {
  const button = event.currentTarget
  const openHere = menuButton === button
  menu.value?.hide()
  if (openHere) return
  menuModel.value = menuItems(row)
  void nextTick(() => {
    menu.value?.show(event, button)
    menuButton = button
  })
}

const renameTarget = ref<string | null>(null)

// A merged branch is deleted without asking. An UNMERGED_BRANCH refusal asks,
// naming git's reason, then deletes forced. Any other refusal (the branch in
// use elsewhere) stays in the list, no retry offered.
async function deleteBranch(name: string): Promise<void> {
  await runAndClose(() => Facade.DeleteBranch(props.wt, name, false), hide)
  if (error.value?.code !== Code.Unmerged) return
  const reason = error.value.message
  error.value = null
  confirmAction(
    t('branchPicker.deleteUnmergedTitle'),
    t('branchPicker.deleteUnmergedConfirm', { reason }),
    () => {
      void runAndClose(() => Facade.DeleteBranch(props.wt, name, true), hide)
    },
  )
}

async function updateChecked(): Promise<void> {
  await Promise.all(
    [...checked.value].map((name) => run(() => Facade.UpdateBranch(props.wt, name))),
  )
  checked.value.clear()
  if (!error.value) hide()
}
</script>

<template>
  <button
    type="button"
    class="picker-button"
    data-test="branch-picker"
    :title="headLabel"
    :aria-label="headLabel"
    @click="popover?.toggle($event)"
  >
    <i :class="iconClass('branch')" aria-hidden="true"></i>
    <template v-if="layout !== 'narrow'">
      <span class="ellipsis">{{ headLabel }}</span>
      <span v-if="tracked" data-test="ahead-behind">
        <i :class="iconClass('ahead')" aria-hidden="true"></i>{{ tracked.ahead }}
        <i :class="iconClass('behind')" aria-hidden="true"></i>{{ tracked.behind }}
      </span>
      <i :class="iconClass('dropdown')" aria-hidden="true"></i>
    </template>
  </button>
  <Popover ref="popover" @show="query = ''" @hide="menu?.hide()">
    <div class="branch-list">
      <ErrorBanner v-if="error" :error="error" />
      <PickerList
        v-model:query="query"
        :rows="rows"
        :placeholder="t('branchPicker.searchPlaceholder')"
        @activate="activate"
      >
        <template #row="{ row, activate: onActivate }">
          <h3 v-if="row.kind === 'section'">{{ row.label }}</h3>
          <span
            v-else-if="row.kind === 'action'"
            class="row"
            data-test="picker-action"
            @click="onActivate"
            >{{ row.label }}</span
          >
          <template v-else>
            <input
              v-if="updatable(row)"
              type="checkbox"
              data-test="update-select"
              :checked="checked.has(row.name)"
              :title="t('branchPicker.selectToUpdate', { name: row.name })"
              :aria-label="t('branchPicker.selectToUpdate', { name: row.name })"
              @change="setChecked(row.name, ($event.target as HTMLInputElement).checked)"
            />
            <span class="row" data-test="ref-row" @click="onActivate">
              <span class="current-marker">
                <i
                  v-if="row.kind === 'local' && row.branch.current"
                  :class="iconClass('current')"
                  data-test="current-marker"
                  :title="t('branchPicker.currentBranch')"
                  :aria-label="t('branchPicker.currentBranch')"
                ></i>
              </span>
              <span class="ref-name ellipsis" :title="row.name">{{ row.name }}</span>
              <span v-if="row.kind === 'local' && row.branch.upstream" class="muted leftover">
                <span v-if="row.branch.ahead"
                  ><i :class="iconClass('ahead')" aria-hidden="true"></i
                  >{{ row.branch.ahead }}</span
                >
                <span v-if="row.branch.behind"
                  ><i :class="iconClass('behind')" aria-hidden="true"></i
                  >{{ row.branch.behind }}</span
                >
              </span>
              <span v-else class="leftover"></span>
            </span>
            <button
              type="button"
              class="glyph-btn"
              data-test="ref-actions"
              :title="t('branchPicker.actionsFor', { name: row.name })"
              :aria-label="t('branchPicker.actionsFor', { name: row.name })"
              @click="openMenu($event, row)"
            >
              <i :class="iconClass('rowMenu')" aria-hidden="true"></i>
            </button>
          </template>
        </template>
      </PickerList>
      <Menu ref="menu" :model="menuModel" popup append-to="self" @hide="menuButton = null" />
    </div>
  </Popover>
  <RenameBranchDialog
    v-if="renameTarget !== null"
    :wt="wt"
    :name="renameTarget"
    @close="renameTarget = null"
  />
</template>

<style scoped>
/* Wider than the other pickers, for the ahead/behind counts. */
.picker-button {
  max-width: 20rem;
  white-space: nowrap;
}
.branch-list {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  width: 28rem;
  max-width: 90vw;
  max-height: 70vh;
}
.current-marker {
  font-size: 0.75em;
  color: var(--p-primary-color);
}
.muted {
  white-space: nowrap;
}
</style>
