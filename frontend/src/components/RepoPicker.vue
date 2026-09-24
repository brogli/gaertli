<script setup lang="ts">
import { computed, onMounted, ref, useTemplateRef } from 'vue'
import { useI18n } from '../i18n'
import Popover from 'openvue/popover'
import { AppService, Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { RecentRepo } from '../../bindings/github.com/brogli/nebegit/internal/settings'
import type { Worktree } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { useCoreCall } from '../composables/useCoreCall'
import { useSettingsStore } from '../stores/settings'
import { basename, repoPath } from '../lib/path'
import { iconClass } from '../lib/icons'
import { useLayout } from '../lib/layout'
import ErrorBanner from './ErrorBanner.vue'
import OpenWSLFolderDialog from './OpenWSLFolderDialog.vue'
import PickerList from './PickerList.vue'
import type { PickerRow } from './pickerRow'

// RepoPicker is the toolbar button naming this window's repo; it opens a
// searchable list of the recent repos. asPage shows that list in place
// instead, as the content of a window with no selection.
const props = defineProps<{ path: string; distro?: string | undefined; asPage?: boolean }>()

const { error, run } = useCoreCall()
const { t } = useI18n()
const layout = useLayout()

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

// "Open WSL folder…" is offered only where WSL has a distro to open; a
// listing that fails for another reason keeps it, so the dialog shows why.
const settings = useSettingsStore()
const wslOffered = ref(false)
const wslDialogOpen = ref(false)
onMounted(() => {
  settings.loadDistros().then(
    (distros) => (wslOffered.value = distros.length > 0),
    () => (wslOffered.value = true),
  )
})

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
    ...(wslOffered.value
      ? [
          actionRow('open-wsl', t('repoPicker.openWslFolder'), () =>
            closeAnd(() => (wslDialogOpen.value = true)),
          ),
        ]
      : []),
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
// dir well enough to read worktrees from it.
function activate(row: Row): void {
  if (row.kind === 'action') row.run()
  else if (!row.current) void run(() => AppService.SelectRecentRepo(row.repo))
}

const popover = useTemplateRef<InstanceType<typeof Popover>>('popover')

// closeAnd closes the popover, then runs action: for an action that leaves
// the list, e.g. for a dialog.
function closeAnd(action: () => void): void {
  if (!props.asPage) popover.value?.hide()
  action()
}

function openFolder(): void {
  closeAnd(() => void run(() => AppService.PickAndOpen()))
}
</script>

<template>
  <button
    v-if="!asPage"
    type="button"
    class="picker-button"
    data-test="repo-picker"
    :title="buttonTitle"
    :aria-label="buttonTitle"
    @click="popover?.toggle($event)"
  >
    <i :class="iconClass('repo')" aria-hidden="true"></i>
    <template v-if="layout !== 'narrow'">
      <span class="ellipsis">{{ repoName }}</span>
      <span v-if="distro" class="wsl-tag" :title="t('repoPicker.wslTitle', { distro })">{{
        t('repoPicker.wsl')
      }}</span>
      <i :class="iconClass('dropdown')" aria-hidden="true"></i>
    </template>
  </button>
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
            <span v-if="row.repo.distro" class="wsl-tag">{{
              t('repoPicker.wslTitle', { distro: row.repo.distro })
            }}</span>
          </span>
        </template>
      </PickerList>
    </div>
  </component>
  <OpenWSLFolderDialog v-if="wslDialogOpen" @close="wslDialogOpen = false" />
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
.wsl-tag {
  flex: none;
  font-size: 0.8em;
  padding: 0 0.3em;
  border: 1px solid var(--p-content-border-color);
  border-radius: 3px;
}
</style>
