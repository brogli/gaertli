<script setup lang="ts">
import { computed, onMounted, ref, useTemplateRef } from 'vue'
import { useI18n } from '../i18n'
import Popover from 'openvue/popover'
import Button from 'openvue/button'
import Tag from 'openvue/tag'
import { AppService, Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { RecentRepo } from '../../bindings/github.com/brogli/nebegit/internal/settings'
import type { Worktree } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { useCoreCall } from '../composables/useCoreCall'
import { basename, repoPath } from '../lib/path'
import { iconClass } from '../lib/icons'
import { useLayout, useToolbarButton } from '../lib/layout'
import ErrorBanner from './ErrorBanner.vue'
import PickerList from './PickerList.vue'
import type { PickerRow } from './pickerRow'

// RepoPicker is the toolbar button naming this window's repo; it opens a
// searchable list of the recent repos. asPage shows that list in place
// instead, as the content of a window with no selection.
const props = defineProps<{ path: string; distro?: string | undefined; asPage?: boolean }>()

const { error, run } = useCoreCall()
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
    | { kind: 'action'; label: string; run: () => void }
    | { kind: 'repo'; repo: RecentRepo; current: boolean }
  )

function actionRow(key: string, label: string, action: () => void): Row {
  return { key, kind: 'action', label, run: action, text: label }
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
    actionRow('new-window', t('repoPicker.newWindow'), newWindow),
    ...(ownRepo.value ? [repoRow(ownRepo.value, true)] : []),
    ...recents.value.filter((r) => !isOwn(r)).map((r) => repoRow(r, false)),
  ]
  const q = query.value.trim().toLowerCase()
  return q ? all.filter((r) => r.text.toLowerCase().includes(q)) : all
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

const popover = useTemplateRef<InstanceType<typeof Popover>>('popover')

// openFolder opens the folder dialog; the folder picked tells a local repo
// from one inside a WSL distro.
function openFolder(): void {
  void run(() => AppService.PickAndOpen())
}

// newWindow opens a window with no selection.
function newWindow(): void {
  void AppService.NewWindow()
}
</script>

<template>
  <Button
    v-if="!asPage"
    v-bind="toolbarButton"
    class="picker-button"
    data-test="repo-picker"
    :title="buttonTitle"
    :aria-label="buttonTitle"
    @click="popover?.toggle($event)"
  >
    <i :class="iconClass('repo')" aria-hidden="true"></i>
    <template v-if="layout !== 'narrow'">
      <span class="ellipsis">{{ repoName }}</span>
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
    <div class="repo-list" :class="{ page: asPage }">
      <ErrorBanner v-if="error" :error="error" />
      <PickerList
        v-model:query="query"
        :rows="rows"
        :placeholder="t('repoPicker.searchPlaceholder')"
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
          <span v-else class="row" data-test="repo-row" :title="row.repo.path" @click="onActivate">
            <span class="current-marker" aria-hidden="true">
              <i v-if="row.current" :class="iconClass('current')"></i>
            </span>
            <span class="repo-name ellipsis">{{ basename(row.repo.path) }}</span>
            <span class="muted ellipsis leftover">{{ row.repo.path }}</span>
            <Tag
              v-if="row.repo.distro"
              severity="secondary"
              :value="t('repoPicker.wslTitle', { distro: row.repo.distro })"
            />
          </span>
        </template>
      </PickerList>
    </div>
  </component>
</template>

<style scoped>
.repo-list {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  width: 28rem;
  max-width: 90vw;
  max-height: 70vh;
}
.repo-list.page {
  width: auto;
  max-width: 44rem;
  max-height: none;
  min-height: 0;
  padding: 0.6rem;
}
.repo-name {
  font-weight: 600;
}
</style>
