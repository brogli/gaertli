<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Dialog from 'openvue/dialog'
import InputText from 'openvue/inputtext'
import Checkbox from 'openvue/checkbox'
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import { useCoreCall } from '@/composables/useCoreCall'
import { AppService, Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { Worktree } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { DIALOG } from '@/lib/layout'
import { repoPath as toRepoPath } from '@/lib/path'
import { useRefsStore } from '@/stores/refs'
import BranchSelect from './BranchSelect.vue'
import ErrorBanner from './ErrorBanner.vue'
import FormField from './FormField.vue'
import StartPointField from './StartPointField.vue'
import WorktreeFolderFields from './WorktreeFolderFields.vue'
import { branchStart, startRev, type StartPoint, type WorktreeStart } from './startPoint'

// CreateWorktreeDialog is "New worktree…": an existing branch, or a new
// one at a start point, both picked from the repo's refs, and the folder it
// goes to. git itself refuses a non-empty target.
const props = defineProps<{
  repoPath: string
  distro?: string | undefined
  start?: WorktreeStart
}>()
const emit = defineEmits<{ created: []; close: [] }>()
const refs = useRefsStore()
// repo is repoPath as core calls take it.
const repo = toRepoPath(props.repoPath, props.distro)

const newBranch = ref(props.start?.newBranch ?? false)
const existingBranch = ref(
  props.start?.newBranch === false ? props.start.branch : (refs.refs?.branches?.[0]?.name ?? ''),
)
const branchName = ref('')
const branch = computed(() => (newBranch.value ? branchName.value : existingBranch.value))
const start = ref<StartPoint>(
  props.start?.newBranch ? props.start.start : branchStart(refs.current?.name),
)
const target = ref('')
const { error, run, runAndClose } = useCoreCall()
const { t } = useI18n()

// The repo's main worktree names the new folder and holds the default base
// folder; this worktree stands in until it is found.
const mainWorktree = ref(props.repoPath)
onMounted(async () => {
  const worktrees = await run(() => Facade.ListWorktrees(repo))
  const main = worktrees?.find((w) => w.main)
  if (main) mainWorktree.value = main.path
})

// created is set on a successful submit instead of closing: the dialog
// then offers to open the result.
const created = ref<Worktree | null>(null)

async function submit(): Promise<void> {
  const startPoint = newBranch.value ? startRev(start.value) : ''
  const wt = await run(() =>
    Facade.AddWorktree(
      repo,
      toRepoPath(target.value, props.distro),
      branch.value,
      startPoint,
      newBranch.value,
    ),
  )
  if (wt) created.value = wt
}

// createdPath is the created worktree's path as core calls take it, called
// only from the footer buttons shown once created is set.
function createdPath(): string {
  // eslint-disable-next-line @typescript-eslint/no-non-null-assertion -- guarded by the v-if="created" the calling buttons render under
  return toRepoPath(created.value!.path, props.distro)
}

function openWindow(): Promise<void> {
  return runAndClose(() => AppService.OpenPath(createdPath()), finish)
}
function openEditor(): Promise<void> {
  return runAndClose(() => Facade.OpenFolderAt(createdPath()), finish)
}
async function openBoth(): Promise<void> {
  await run(() => AppService.OpenPath(createdPath()))
  if (!error.value) await openEditor()
}

// finish emits created and closes the dialog once the follow-up (or "Not
// now") is resolved, not at submit, so the worktree picker reloads after
// the user is done.
function finish(): void {
  emit('created')
  emit('close')
}
</script>

<template>
  <Dialog
    visible
    v-bind="DIALOG"
    :header="t('dialogs.createWorktree.header')"
    @update:visible="emit('close')"
  >
    <ErrorBanner v-if="error" :error="error" />
    <p v-if="created">
      {{ t('dialogs.createWorktree.created', { path: created.path }) }}
    </p>
    <template v-else>
      <label class="flex flex-wrap items-center gap-2">
        <Checkbox v-model="newBranch" binary />
        {{ t('dialogs.createWorktree.newBranch') }}
      </label>
      <template v-if="newBranch">
        <FormField v-slot="{ id }" :label="t('dialogs.branchName')">
          <InputText :id="id" v-model="branchName" autofocus fluid />
        </FormField>
        <StartPointField v-model="start" />
      </template>
      <FormField v-else v-slot="{ id }" :label="t('dialogs.createWorktree.branch')">
        <BranchSelect v-model="existingBranch" :input-id="id" data-test="existing-branch" />
      </FormField>
      <WorktreeFolderFields
        v-model:location="target"
        :main-worktree="mainWorktree"
        :branch="branch"
        :distro="distro"
      />
    </template>
    <template #footer>
      <template v-if="created">
        <Button
          :label="t('dialogs.createWorktree.openInWindow')"
          data-test="open-window"
          @click="openWindow"
        />
        <Button
          :label="t('dialogs.createWorktree.openInEditor')"
          data-test="open-editor"
          @click="openEditor"
        />
        <Button
          :label="t('dialogs.createWorktree.openBoth')"
          data-test="open-both"
          @click="openBoth"
        />
        <Button :label="t('dialogs.createWorktree.notNow')" data-test="not-now" @click="finish" />
      </template>
      <template v-else>
        <Button
          :label="t('common.cancel')"
          severity="secondary"
          data-test="dialog-cancel"
          @click="emit('close')"
        />
        <Button
          :label="t('dialogs.createWorktree.create')"
          data-test="create-worktree-submit"
          @click="submit"
        />
      </template>
    </template>
  </Dialog>
</template>
