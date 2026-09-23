<script setup lang="ts">
import { onMounted, ref } from 'vue'
import Dialog from 'openvue/dialog'
import InputText from 'openvue/inputtext'
import Button from 'openvue/button'
import { useCoreCall } from '../composables/useCoreCall'
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

onMounted(async () => {
  distros.value = (await run(() => AppService.ListWSLDistros())) ?? []
})

function select(distro: Distro): void {
  if (distro.version !== 1) selected.value = distro
}

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
  <Dialog visible header="Open WSL folder" modal @update:visible="emit('close')">
    <ErrorBanner v-if="error" :error="error" />
    <ul>
      <li v-for="distro in distros" :key="distro.name">
        <button
          type="button"
          data-test="wsl-distro"
          :disabled="distro.version === 1"
          :aria-pressed="selected?.name === distro.name"
          @click="select(distro)"
        >
          {{ distro.name }}
        </button>
        <span v-if="distro.version === 1">WSL 1 is not supported</span>
      </li>
    </ul>
    <InputText v-model="typedPath" placeholder="Folder inside the distro" />
    <template #footer>
      <Button label="Browse…" data-test="wsl-browse" :disabled="!selected" @click="browse" />
      <Button label="Open" data-test="wsl-open" :disabled="!selected || !typedPath" @click="openTyped" />
    </template>
  </Dialog>
</template>
