<script setup lang="ts">
import { computed, nextTick, ref, useTemplateRef, watch, type ComponentPublicInstance } from 'vue'
import Button from 'openvue/button'
import InputGroup from 'openvue/inputgroup'
import InputText from 'openvue/inputtext'
import { useI18n } from '@/i18n'
import { AppService } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { useSettingsStore } from '@/stores/settings'
import { basename, joinPath, lastSegment, parentPath, repoPath, withLastSegment } from '@/lib/path'
import FormField from './FormField.vue'

// WorktreeFolderFields are a new worktree's folder name and location, kept
// in step: the location is the base folder for worktrees plus the name.
// Both follow branch until the user edits either one or browses.
// mainWorktree is the repo's main worktree, POSIX for a WSL repo.
const props = defineProps<{ mainWorktree: string; branch: string; distro?: string | undefined }>()
const location = defineModel<string>('location', { required: true })
const settings = useSettingsStore()
const { t } = useI18n()

// Settings' base folder for this repo's environment, else the folder
// holding the repo's main worktree.
const baseFolder = computed(
  () => settings.settings?.worktreeFolders?.[props.distro ?? ''] || parentPath(props.mainWorktree),
)

// The repo's folder name and the branch's, a "/" in it as "-".
const suggestedName = computed(() =>
  [basename(props.mainWorktree), props.branch.replaceAll('/', '-')].filter(Boolean).join('-'),
)

const name = ref('')
const followsBranch = ref(true)
const locationInput = useTemplateRef<ComponentPublicInstance>('locationInput')

// A long path shows its start by default; standard input behaviour keeps
// the caret in view, so moving it to the end after a programmatic change
// scrolls the field to show the end instead, where the folder name (the
// part that changes) sits.
function scrollLocationToEnd(): void {
  void nextTick(() => {
    const el = locationInput.value?.$el
    if (el instanceof HTMLInputElement) el.setSelectionRange(el.value.length, el.value.length)
  })
}

watch(
  [baseFolder, suggestedName],
  ([base, suggested]) => {
    if (!followsBranch.value) return
    name.value = suggested
    location.value = joinPath(base, suggested)
    scrollLocationToEnd()
  },
  { immediate: true },
)

function editName(value = ''): void {
  followsBranch.value = false
  name.value = value
  location.value = withLastSegment(location.value, value)
  scrollLocationToEnd()
}

function editLocation(value = ''): void {
  followsBranch.value = false
  location.value = value
  name.value = lastSegment(value)
}

// PickFolder, not PickAndOpen: the target is not an existing repo. Seeded at
// the distro's UNC root for a WSL repo, so the native picker browses there.
async function browse(): Promise<void> {
  const [path, ok] = await AppService.PickFolder(repoPath('', props.distro))
  if (ok) {
    editLocation(path)
    scrollLocationToEnd()
  }
}
</script>

<template>
  <FormField v-slot="{ id }" :label="t('dialogs.createWorktree.folderName')">
    <InputText :id="id" :model-value="name" fluid @update:model-value="editName" />
  </FormField>
  <FormField v-slot="{ id }" :label="t('dialogs.createWorktree.location')">
    <InputGroup>
      <InputText
        :id="id"
        ref="locationInput"
        :model-value="location"
        :title="location"
        @update:model-value="editLocation"
      />
      <Button
        :label="t('dialogs.createWorktree.browse')"
        severity="secondary"
        data-test="browse-target"
        @click="browse"
      />
    </InputGroup>
  </FormField>
</template>
