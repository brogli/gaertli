<script setup lang="ts">
import { ref } from 'vue'
import Dialog from 'openvue/dialog'
import InputText from 'openvue/inputtext'
import Checkbox from 'openvue/checkbox'
import Button from 'openvue/button'
import { useCoreCall } from '../composables/useCoreCall'
import { AppService, Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { BranchSummary, Worktree } from '../../bindings/github.com/brogli/nebegit/internal/core'
import ErrorBanner from './ErrorBanner.vue'
import type { WorktreeStart } from './worktreeStart'

// CreateWorktreeDialog is "+ New worktree": an existing branch, or a new
// one from a branch/commit start point. git itself refuses a non-empty
// target.
const props = defineProps<{ repoPath: string; branches: BranchSummary[]; visible: boolean; start?: WorktreeStart }>()
const emit = defineEmits<{ 'update:visible': [boolean]; created: [] }>()

const newBranch = ref(props.start?.newBranch ?? false)
const existingBranch = ref(props.start?.newBranch === false ? props.start.branch : (props.branches[0]?.name ?? ''))
const name = ref('')
const start = ref(props.start?.newBranch === true ? props.start.start : '')
const target = ref('')
const { error, run, runAndClose } = useCoreCall()

// created is set on a successful submit instead of closing: the dialog
// then offers to open the result.
const created = ref<Worktree | null>(null)

// PickFolder, not PickAndOpen: the target is not an existing repo.
async function browse(): Promise<void> {
  const [path, ok] = await AppService.PickFolder('')
  if (ok) target.value = path
}

async function submit(): Promise<void> {
  const branch = newBranch.value ? name.value : existingBranch.value
  const startPoint = newBranch.value ? start.value : ''
  const wt = await run(() => Facade.AddWorktree(props.repoPath, target.value, branch, startPoint, newBranch.value))
  if (wt) created.value = wt
}

function openWindow(): Promise<void> {
  return runAndClose(() => AppService.OpenRecent(created.value!.path), finish)
}
function openEditor(): Promise<void> {
  return runAndClose(() => Facade.OpenFolderAt(created.value!.path), finish)
}
async function openBoth(): Promise<void> {
  await run(() => AppService.OpenRecent(created.value!.path))
  if (!error.value) await openEditor()
}

// finish closes the dialog once the follow-up (or "Not now") is resolved,
// and only then emits created, so the panel reloads after the user is
// done.
function finish(): void {
  emit('created')
  created.value = null
  name.value = ''
  emit('update:visible', false)
}
</script>

<template>
  <Dialog
    :visible="visible"
    header="New worktree"
    modal
    @update:visible="(v: boolean) => emit('update:visible', v)"
  >
    <ErrorBanner v-if="error" :error="error" />
    <template v-if="created">
      <p>Worktree created at {{ created.path }}.</p>
    </template>
    <template v-else>
      <label>
        <Checkbox v-model="newBranch" binary />
        New branch
      </label>
      <select v-if="!newBranch" v-model="existingBranch" data-test="existing-branch">
        <option v-for="b in branches" :key="b.name" :value="b.name">{{ b.name }}</option>
      </select>
      <template v-else>
        <InputText v-model="name" placeholder="Branch name" />
        <InputText v-model="start" placeholder="Start point (branch or commit)" />
      </template>
      <InputText v-model="target" placeholder="Target folder" />
      <Button label="Browse…" data-test="browse-target" @click="browse" />
    </template>
    <template #footer>
      <template v-if="created">
        <Button label="Open in app window" data-test="open-window" @click="openWindow" />
        <Button label="Open in editor" data-test="open-editor" @click="openEditor" />
        <Button label="Both" data-test="open-both" @click="openBoth" />
        <Button label="Not now" data-test="not-now" @click="finish" />
      </template>
      <Button v-else label="Create" data-test="create-worktree-submit" @click="submit" />
    </template>
  </Dialog>
</template>
