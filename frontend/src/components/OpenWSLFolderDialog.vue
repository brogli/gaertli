<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Dialog from 'openvue/dialog'
import InputText from 'openvue/inputtext'
import Button from 'openvue/button'
import { useI18n } from '../i18n'
import { useCoreCall } from '../composables/useCoreCall'
import { useSettingsStore } from '../stores/settings'
import { AppService } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { Distro } from '../../bindings/github.com/brogli/nebegit/internal/remote'
import { repoPath } from '../lib/path'
import ErrorBanner from './ErrorBanner.vue'

// OpenWSLFolderDialog is "Open WSL folder": pick an installed distro, then
// either browse (the native picker, seeded at its UNC root) or type a
// folder inside it. WSL 1 distros are listed but cannot be picked.
const emit = defineEmits<{ close: [] }>()

const distros = ref<Distro[]>([])
const selected = ref<Distro | null>(null)
const typedPath = ref('')
const { error, run, runAndClose } = useCoreCall()
const { t } = useI18n()
const settings = useSettingsStore()

onMounted(async () => {
  distros.value = (await run(() => settings.loadDistros())) ?? []
})

function browse(): void {
  const distro = selected.value
  if (!distro) return
  void runAndClose(
    () => AppService.PickAndOpenWSL(distro.name),
    () => emit('close'),
  )
}

function openTyped(): void {
  const distro = selected.value
  if (!distro || !typedPath.value) return
  void runAndClose(
    () => AppService.OpenPath(repoPath('/' + typedPath.value.replace(/^\/+/, ''), distro.name)),
    () => emit('close'),
  )
}
</script>

<template>
  <Dialog visible :header="t('dialogs.openWSLFolder.header')" modal @update:visible="emit('close')">
    <ErrorBanner v-if="error" :error="error" />
    <ul>
      <li v-for="distro in distros" :key="distro.name">
        <button
          type="button"
          data-test="wsl-distro"
          :disabled="distro.version === 1"
          :aria-pressed="selected?.name === distro.name"
          @click="selected = distro"
        >
          {{ distro.name }}
        </button>
        <span v-if="distro.version === 1">{{ t('dialogs.openWSLFolder.wsl1Unsupported') }}</span>
      </li>
    </ul>
    <InputText v-model="typedPath" :placeholder="t('dialogs.openWSLFolder.folderPlaceholder')" />
    <template #footer>
      <Button
        :label="t('dialogs.openWSLFolder.browse')"
        data-test="wsl-browse"
        :disabled="!selected"
        @click="browse"
      />
      <Button
        :label="t('dialogs.openWSLFolder.open')"
        data-test="wsl-open"
        :disabled="!selected || !typedPath"
        @click="openTyped"
      />
    </template>
  </Dialog>
</template>
