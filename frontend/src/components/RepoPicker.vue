<script setup lang="ts">
import { computed, onMounted, ref, useTemplateRef } from 'vue'
import { useI18n } from '@/i18n'
import Popover from 'openvue/popover'
import Button from 'openvue/button'
import Tag from 'openvue/tag'
import type { MenuItem } from 'openvue/menuitem'
import { AppService, Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'
import type { RecentRepo } from '../../bindings/github.com/brogli/gaertli/internal/settings'
import type { Worktree } from '../../bindings/github.com/brogli/gaertli/internal/core'
import { useCoreCall } from '@/composables/useCoreCall'
import { basename, repoPath } from '@/lib/path'
import { iconClass } from '@/lib/icons'
import { useLayout, useToolbarButton } from '@/lib/layout'
import { errorNotice, useNotices } from '@/lib/notice'
import ErrorBanner from './ErrorBanner.vue'
import PickerList from './PickerList.vue'
import ShortcutHint from './ShortcutHint.vue'
import { filterRows, type PickerRow } from './pickerRow'

// RepoPicker is the toolbar button naming this window's repo; it opens a
// searchable list of the recent repos. asPage shows that list in place
// instead, as the content of a window with no selection.
const props = defineProps<{ path: string; distro?: string | undefined; asPage?: boolean }>()

const { error, run } = useCoreCall()
const notices = useNotices()

// outOfList runs a menu action's call: its refusal shows as a notice, since
// the action has already closed the list.
function outOfList<T>(call: () => Promise<T>): Promise<T | undefined> {
  return run(call, (e) => {
    notices.show('repo-picker', errorNotice(e))
    return true
  })
}

const { t } = useI18n()
const layout = useLayout()
const toolbarButton = useToolbarButton()

// This window's own worktrees, fetched once to find the repo's main one
// (the button's label) and to keep a recent entry of any of them from also
// listing as a separate repo.
const ownWorktrees = ref<Worktree[] | null>(null)
onMounted(async () => {
  if (!props.path) return
  ownWorktrees.value =
    (await run(() => Facade.ListWorktrees(repoPath(props.path, props.distro)))) ?? null
})

const ownRepo = computed<RecentRepo | undefined>(() => {
  if (!props.path) return undefined
  const main = ownWorktrees.value?.find((w) => w.main)?.path ?? props.path
  return { path: main, lastOpened: '', ...(props.distro ? { distro: props.distro } : {}) }
})
const repoName = computed(() => basename(ownRepo.value?.path ?? props.path))
const buttonTitle = computed(() =>
  props.distro
    ? t('repoPicker.pathWithWsl', { path: props.path, distro: props.distro })
    : props.path,
)

const ownPaths = computed(
  () => new Set([props.path, ...(ownWorktrees.value?.map((w) => w.path) ?? [])]),
)
function isOwn(r: RecentRepo): boolean {
  return (r.distro ?? '') === (props.distro ?? '') && ownPaths.value.has(r.path)
}

const recents = ref<RecentRepo[]>([])
async function loadRecents(): Promise<void> {
  recents.value = (await AppService.RecentRepos()) ?? []
}
onMounted(loadRecents)

type Row = PickerRow & { text: string } & (
    | { kind: 'action'; label: string; run: () => void; shortcut?: string }
    | { kind: 'repo'; repo: RecentRepo; current: boolean }
  )

function actionRow(key: string, label: string, action: () => void, shortcut?: string): Row {
  return { key, kind: 'action', label, run: action, text: label, ...(shortcut && { shortcut }) }
}
function repoRow(repo: RecentRepo, current: boolean): Row {
  return {
    key: `${repo.distro ?? ''}#${repo.path}`,
    kind: 'repo',
    repo,
    current,
    text: `${basename(repo.path)} ${repo.path} ${repo.distro ?? ''}`,
  }
}

const query = ref('')
const rows = computed<Row[]>(() => {
  const all: Row[] = [
    actionRow('open-folder', t('repoPicker.openFolder'), openFolder),
    actionRow('new-window', t('repoPicker.newWindow'), newWindow, t('shortcuts.newWindow')),
    ...(ownRepo.value ? [repoRow(ownRepo.value, true)] : []),
    ...recents.value.filter((r) => !isOwn(r)).map((r) => repoRow(r, false)),
  ]
  return filterRows(all, query.value, (r) => r.text)
})

function onShow(): void {
  query.value = ''
  void loadRecents()
}

// Choosing another repo, and the gone-entry retry at its main worktree, are
// the core's own job (SelectRecentRepo): it alone knows a repo's common git
// dir well enough to read worktrees from it. An action closes the popover
// first.
function activate(row: Row): void {
  if (row.kind === 'action') {
    if (!props.asPage) popover.value?.hide()
    row.run()
  } else if (!row.current) void run(() => AppService.SelectRecentRepo(row.repo))
}

function rowAttrs(row: Row): Record<string, string> {
  return row.kind === 'action'
    ? { 'data-test': 'picker-action' }
    : { 'data-test': 'repo-row', title: row.repo.path }
}

// menuFor is a row's menu, held by PickerList: an action row and the
// window's own repo get none.
function menuFor(row: Row): MenuItem[] {
  return row.kind === 'repo' && !row.current
    ? [
        { label: t('repoPicker.openInNewWindow'), command: () => openInNewWindow(row.repo) },
        { label: t('repoPicker.removeFromRecents'), command: () => removeFromRecents(row.repo) },
      ]
    : []
}

const popover = useTemplateRef<InstanceType<typeof Popover>>('popover')

// closeAnd closes the popover, then runs action; the page form has no
// popover and stays.
function closeAnd(action: () => void): void {
  if (!props.asPage) popover.value?.hide()
  action()
}

// openFolder opens the folder dialog; the folder picked tells a local repo
// from one inside a WSL distro.
function openFolder(): void {
  void run(() => AppService.PickAndOpen())
}

// newWindow opens a window with no selection.
function newWindow(): void {
  void AppService.NewWindow()
}

function openInNewWindow(repo: RecentRepo): void {
  closeAnd(() => void outOfList(() => AppService.OpenRecentRepo(repo)))
}

function removeFromRecents(repo: RecentRepo): void {
  closeAnd(() => void removeRecent(repo))
}

async function removeRecent(repo: RecentRepo): Promise<void> {
  if (await outOfList(() => AppService.RemoveRecentRepo(repo).then(() => true))) void loadRecents()
}
</script>

<template>
  <Button
    v-if="!asPage"
    v-bind="toolbarButton"
    class="min-w-0 max-w-64"
    data-test="repo-picker"
    :title="buttonTitle"
    :aria-label="buttonTitle"
    @click="popover?.toggle($event)"
  >
    <i :class="iconClass('repo')" aria-hidden="true"></i>
    <template v-if="layout !== 'narrow'">
      <span class="min-w-10 flex-auto truncate">{{ repoName }}</span>
      <Tag
        v-if="distro"
        severity="secondary"
        :value="t('repoPicker.wsl')"
        :title="t('repoPicker.wslTitle', { distro })"
      />
      <i :class="iconClass('dropdown')" aria-hidden="true"></i>
    </template>
  </Button>
  <component :is="asPage ? 'section' : Popover" ref="popover" @show="onShow">
    <div
      class="repo-list flex flex-col gap-1.5"
      :class="asPage ? 'max-w-176 min-h-0 p-2.5' : 'w-112 max-w-[90vw] max-h-[70vh]'"
    >
      <ErrorBanner v-if="error" :error="error" />
      <PickerList
        v-model:query="query"
        :rows="rows"
        :placeholder="t('repoPicker.searchPlaceholder')"
        :row-attrs="rowAttrs"
        :menu="menuFor"
        menu-button
        @activate="activate"
      >
        <template #row="{ row }">
          <template v-if="row.kind === 'action'">
            <span data-test="picker-action-label" class="truncate">{{ row.label }}</span>
            <ShortcutHint v-if="row.shortcut" :shortcut="row.shortcut" />
          </template>
          <template v-else>
            <span class="current-marker w-[1em] flex-none" aria-hidden="true">
              <i v-if="row.current" :class="iconClass('current')"></i>
            </span>
            <span class="min-w-0 truncate font-semibold">{{ basename(row.repo.path) }}</span>
            <span class="min-w-0 flex-1 truncate text-sm text-muted-color">{{
              row.repo.path
            }}</span>
            <Tag
              v-if="row.repo.distro"
              severity="secondary"
              :value="t('repoPicker.wslTitle', { distro: row.repo.distro })"
            />
          </template>
        </template>
      </PickerList>
    </div>
  </component>
</template>
