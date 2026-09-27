<script setup lang="ts">
import { computed, ref, useTemplateRef } from 'vue'
import { useI18n } from '@/i18n'
import Popover from 'openvue/popover'
import Button from 'openvue/button'
import Checkbox from 'openvue/checkbox'
import type { MenuItem } from 'openvue/menuitem'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { Code, type Branch } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { useRefsStore } from '@/stores/refs'
import { useRepoStore } from '@/stores/repo'
import { useLogStore } from '@/stores/log'
import { useCoreCall, type CoreError } from '@/composables/useCoreCall'
import { useConfirmAction } from '@/composables/useConfirmAction'
import { shortSha } from '@/lib/format'
import { iconClass } from '@/lib/icons'
import { useLayout, useToolbarButton } from '@/lib/layout'
import { errorNotice, useNotices } from '@/lib/notice'
import ErrorBanner from './ErrorBanner.vue'
import PickerList from './PickerList.vue'
import RenameBranchDialog from './RenameBranchDialog.vue'
import { filterRows, type PickerRow } from './pickerRow'
import { branchStart, commitStart, type StartPoint, type WorktreeStart } from './startPoint'

// BranchPicker is the toolbar button naming the current branch; it opens a
// searchable list of the local branches, remote branches and tags. A click
// checks an entry out; a right-click opens a context menu of its actions.
// The dialogs another component already hosts are asked for by event: a new
// branch at a start point, a new worktree, a detached checkout.
// distro is set only for a WSL window; a path from the core is POSIX there.
const props = defineProps<{ wt: string; distro?: string | undefined }>()
const emit = defineEmits<{
  'create-branch': [start: StartPoint]
  'create-worktree': [start: WorktreeStart]
  'checkout-commit': [sha: string]
  'checkout-elsewhere': [branch: string, path: string]
  'show-log': []
}>()
const refs = useRefsStore()
const repo = useRepoStore()
const log = useLogStore()
const { error, run, runAndClose } = useCoreCall()
const { confirmAction } = useConfirmAction()
const notices = useNotices()
const { t } = useI18n()
const layout = useLayout()
const toolbarButton = useToolbarButton()

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
// filter matches, e.g. "origin/x" for a remote branch, ref its full ref name.
type RefRow = PickerRow & { name: string; ref: string; sha: string } & (
    | { kind: 'local'; branch: Branch }
    | { kind: 'remote'; shortName: string }
    | { kind: 'tag' }
  )
type ActionRow = PickerRow & { kind: 'action'; label: string; run: () => void }
type SectionRow = PickerRow & { kind: 'section'; label: string }
type Row = RefRow | ActionRow | SectionRow

function isRef(row: Row): row is RefRow {
  return row.kind !== 'action' && row.kind !== 'section'
}

function rowAttrs(row: Row): Record<string, string> {
  if (row.kind === 'section') return {}
  return { 'data-test': row.kind === 'action' ? 'picker-action' : 'ref-row' }
}

function actionRow(key: string, label: string, run: () => void): ActionRow {
  return { key, kind: 'action', label, run }
}

const localRows = computed(() =>
  (refs.refs?.branches ?? []).map((b): RefRow => ({
    key: `local:${b.name}`,
    kind: 'local',
    name: b.name,
    ref: `refs/heads/${b.name}`,
    sha: b.sha,
    branch: b,
  })),
)

const sections = computed<{ label: string; rows: RefRow[] }[]>(() => [
  { label: t('refs.local'), rows: localRows.value },
  {
    label: t('refs.remote'),
    rows: (refs.refs?.remotes ?? []).flatMap((r) =>
      (r.branches ?? []).map((b): RefRow => ({
        key: `remote:${r.name}/${b.name}`,
        kind: 'remote',
        name: `${r.name}/${b.name}`,
        ref: `refs/remotes/${r.name}/${b.name}`,
        sha: b.sha,
        shortName: b.name,
      })),
    ),
  },
  {
    label: t('refs.tags'),
    rows: (refs.refs?.tags ?? []).map((tag) => ({
      key: `tag:${tag.name}`,
      kind: 'tag',
      name: tag.name,
      ref: `refs/tags/${tag.name}`,
      sha: tag.sha,
    })),
  },
])

// Local branches with an upstream, other than the current one, can be
// checked to update several at once.
function updatable(row: Row): boolean {
  return row.kind === 'local' && !row.branch.current && !!row.branch.upstream
}
const checked = ref(new Set<string>())

function setChecked(name: string, on: boolean): void {
  if (on) checked.value.add(name)
  else checked.value.delete(name)
}

const query = ref('')
const rows = computed<Row[]>(() => {
  const actions = [
    actionRow('new-branch', t('branchPicker.newBranch'), () =>
      closeAnd(() => emit('create-branch', currentStart())),
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
    ...filterRows(actions, query.value, (a) => a.label),
    ...sections.value.flatMap(({ label, rows }): Row[] => {
      const kept = filterRows(rows, query.value, (r) => r.name)
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

// A Runner runs a core call for a row click or a menu action.
type Runner = (call: () => Promise<unknown>) => Promise<unknown>

// inList runs a row click's call and closes the list once it succeeded; a
// refusal stays in the list with git's reason.
const inList: Runner = (call) => runAndClose(call, hide)

// fromMenu runs a menu action's call. The click on the menu, outside the
// list, has closed the list, so a refusal shows as a notice.
const fromMenu: Runner = (call) => run(call, showRefusal)

function showRefusal(refusal: CoreError): boolean {
  notices.show('branch-picker', errorNotice(refusal))
  return true
}

// A local branch checked out in another worktree asks before switching this
// window there instead of checking it out. A remote branch checks out as
// the local branch of its short name if there is one, else by its full ref
// as a new tracking branch. A tag detaches HEAD, after the warning.
function checkout(row: RefRow, runCall: Runner = inList): void {
  if (row.kind === 'remote') {
    const local = localRows.value.find((r) => r.name === row.shortName)
    if (local) checkout(local, runCall)
    else void runCall(() => Facade.Checkout(props.wt, row.ref))
  } else if (row.kind === 'tag') closeAnd(() => emit('checkout-commit', row.sha))
  else if (row.branch.current) hide()
  else if (row.branch.checkedOutIn) {
    const path = row.branch.checkedOutIn
    closeAnd(() => emit('checkout-elsewhere', row.name, path))
  } else void runCall(() => Facade.Checkout(props.wt, row.name))
}

// showHistory filters the log to ref, keeping the filter's other fields.
function showHistory(ref: string): void {
  void log.applyFilter({ ...log.filter, branches: [ref] })
  emit('show-log')
}

// A new branch starts at the current branch, or at HEAD while detached.
function currentStart(): StartPoint {
  return refs.current ? branchStart(refs.current.name) : commitStart(repo.summary?.sha ?? '')
}

// A local branch checked out somewhere cannot get a second worktree; a new
// branch starts at it instead, as for a remote branch or a tag.
function worktreeStart(row: RefRow): WorktreeStart {
  return row.kind === 'local' && !row.branch.current && !row.branch.checkedOutIn
    ? { newBranch: false, branch: row.name }
    : { newBranch: true, start: { ref: row.ref } }
}

function menuItems(row: RefRow): MenuItem[] {
  const local = row.kind === 'local' ? row.branch : undefined
  const other = !local?.current
  return [
    ...(other
      ? [{ label: t('branchPicker.checkOut'), command: () => checkout(row, fromMenu) }]
      : []),
    {
      label: t('branchPicker.newBranchFromHere'),
      command: () => emit('create-branch', { ref: row.ref }),
    },
    {
      label: t('branchPicker.newWorktreeFromHere'),
      command: () => emit('create-worktree', worktreeStart(row)),
    },
    ...(local && other && local.upstream
      ? [
          {
            label: t('branchPicker.updateFromUpstream'),
            command: () => void fromMenu(() => Facade.UpdateBranch(props.wt, row.name)),
          },
        ]
      : []),
    ...(local && other
      ? [
          { label: t('branchPicker.rename'), command: () => (renameTarget.value = row.name) },
          { label: t('branchPicker.delete'), command: () => deleteBranch(row.name) },
        ]
      : []),
    {
      label: t('branchPicker.showInLog'),
      command: () => {
        void log.jumpTo(row.sha)
        emit('show-log')
      },
    },
    ...(row.kind === 'tag'
      ? [{ label: t('branchPicker.showHistory'), command: () => showHistory(row.ref) }]
      : []),
  ]
}

// menuFor is a row's menu, held by PickerList: a ref row's actions, empty
// for an action or section row.
function menuFor(row: Row): MenuItem[] {
  return isRef(row) ? menuItems(row) : []
}

const renameTarget = ref<string | null>(null)

// A merged branch is deleted without asking. An UNMERGED_BRANCH refusal asks,
// naming git's reason, then deletes forced. Any other refusal (the branch in
// use elsewhere) is a notice, no retry offered.
function deleteBranch(name: string): void {
  void run(
    () => Facade.DeleteBranch(props.wt, name, false),
    (refusal) => {
      if (refusal.code !== Code.Unmerged) return showRefusal(refusal)
      confirmAction(
        t('branchPicker.deleteUnmergedTitle'),
        t('branchPicker.deleteUnmergedConfirm', { reason: refusal.message }),
        () => void fromMenu(() => Facade.DeleteBranch(props.wt, name, true)),
      )
      return true
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
  <Button
    v-bind="toolbarButton"
    class="min-w-0 max-w-80"
    data-test="branch-picker"
    :title="headLabel"
    :aria-label="headLabel"
    @click="popover?.toggle($event)"
  >
    <i :class="iconClass('branch')" aria-hidden="true"></i>
    <template v-if="layout !== 'narrow'">
      <span class="min-w-10 flex-auto truncate">{{ headLabel }}</span>
      <span v-if="tracked" data-test="ahead-behind">
        <i :class="iconClass('ahead')" aria-hidden="true"></i>{{ tracked.ahead }}
        <i :class="iconClass('behind')" aria-hidden="true"></i>{{ tracked.behind }}
      </span>
      <i :class="iconClass('dropdown')" aria-hidden="true"></i>
    </template>
  </Button>
  <Popover ref="popover" @show="query = ''">
    <div class="branch-list flex flex-col gap-1.5 w-112 max-w-[90vw] max-h-[70vh]">
      <ErrorBanner v-if="error" :error="error" />
      <PickerList
        v-model:query="query"
        :rows="rows"
        :placeholder="t('branchPicker.searchPlaceholder')"
        :row-attrs="rowAttrs"
        :menu="menuFor"
        menu-button
        @activate="activate"
      >
        <template #lead="{ row }">
          <span v-if="isRef(row)" class="flex w-(--p-checkbox-width) flex-none">
            <Checkbox
              v-if="updatable(row)"
              binary
              data-test="update-select"
              :model-value="checked.has(row.name)"
              :title="t('branchPicker.selectToUpdate', { name: row.name })"
              :aria-label="t('branchPicker.selectToUpdate', { name: row.name })"
              @update:model-value="setChecked(row.name, $event)"
            />
          </span>
        </template>
        <template #row="{ row }">
          <h3 v-if="row.kind === 'section'">{{ row.label }}</h3>
          <template v-else-if="row.kind === 'action'">{{ row.label }}</template>
          <template v-else>
            <span class="current-marker w-[1em] flex-none text-xs text-primary">
              <i
                v-if="row.kind === 'local' && row.branch.current"
                :class="iconClass('current')"
                data-test="current-marker"
                :title="t('branchPicker.currentBranch')"
                :aria-label="t('branchPicker.currentBranch')"
              ></i>
            </span>
            <span class="ref-name min-w-0 truncate" :title="row.name">{{ row.name }}</span>
            <span
              v-if="row.kind === 'local' && row.branch.upstream"
              class="flex-1 whitespace-nowrap text-sm text-muted-color"
            >
              <span v-if="row.branch.ahead"
                ><i :class="iconClass('ahead')" aria-hidden="true"></i>{{ row.branch.ahead }}</span
              >
              <span v-if="row.branch.behind"
                ><i :class="iconClass('behind')" aria-hidden="true"></i
                >{{ row.branch.behind }}</span
              >
            </span>
            <span v-else class="flex-1"></span>
          </template>
        </template>
      </PickerList>
    </div>
  </Popover>
  <RenameBranchDialog
    v-if="renameTarget !== null"
    :wt="wt"
    :name="renameTarget"
    @close="renameTarget = null"
  />
</template>
