<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useSettingsStore } from '../stores/settings'
import { useCoreCall } from '../composables/useCoreCall'
import { AppService } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { EditorProfile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import type { Distro } from '../../bindings/github.com/brogli/nebegit/internal/remote'
import ErrorBanner from './ErrorBanner.vue'

// DEFAULT_PROFILE mirrors editor.Default(), so a newly added profile form
// starts pre-filled.
const DEFAULT_PROFILE: EditorProfile = {
  name: 'Zed',
  folderCmd: 'zed {path}',
  fileCmd: 'zed {path}',
  fileAtLineCmd: 'zed {path}:{line}',
}

const emit = defineEmits<{ close: [] }>()
const store = useSettingsStore()
const { error, run } = useCoreCall()

const form = reactive({
  protectedBranches: '',
  defaultPullMode: 'merge',
  diffMode: 'unified',
  autoFetchIntervalMinutes: 0,
})

// editorProfiles is edited directly, one form per environment, keyed like
// Settings.EditorProfiles ("" = local, else a WSL distro name).
const editorProfiles = reactive<Record<string, EditorProfile>>({})

const distros = ref<Distro[]>([])

// distrosWithoutProfile feeds "add a distro profile": only distros not
// already configured are offered.
const distrosWithoutProfile = computed(() =>
  distros.value.filter((d) => !(d.name in editorProfiles)),
)

function environmentLabel(name: string): string {
  return name === '' ? 'Local' : name
}

function addDistroProfile(name: string): void {
  editorProfiles[name] = { ...DEFAULT_PROFILE }
}

function installLauncher(distro: string): void {
  void run(() => AppService.InstallWSLLauncher(distro))
}

onMounted(async () => {
  await store.load()
  const s = store.settings
  if (s) {
    form.protectedBranches = (s.protectedBranches ?? []).join(', ')
    form.defaultPullMode = s.defaultPullMode
    form.diffMode = s.diffMode
    form.autoFetchIntervalMinutes = s.autoFetchIntervalMinutes
    Object.assign(editorProfiles, s.editorProfiles ?? {})
    if (!('' in editorProfiles)) editorProfiles[''] = { ...DEFAULT_PROFILE } // Local always has a form, even unconfigured
  }
  // Rendered only when distros are listable at all (Windows host): on any
  // other platform this rejects (WslNotFound) and both the "add a distro
  // profile" control and the "WSL command" section stay empty.
  distros.value = (await run(() => store.loadDistros())) ?? []
})

async function submit(): Promise<void> {
  if (!store.settings) return
  const updated = {
    ...store.settings,
    editorProfiles: { ...editorProfiles },
    protectedBranches: form.protectedBranches
      .split(',')
      .map((b) => b.trim())
      .filter(Boolean),
    defaultPullMode: form.defaultPullMode,
    diffMode: form.diffMode,
    autoFetchIntervalMinutes: form.autoFetchIntervalMinutes,
  }
  await run(() => store.save(updated))
}
</script>

<template>
  <form class="settings-panel" @submit.prevent="submit">
    <button type="button" data-test="close-settings" @click="emit('close')">Close</button>
    <ErrorBanner v-if="error" :error="error" />

    <fieldset v-for="(profile, name) in editorProfiles" :key="name">
      <legend>{{ environmentLabel(name) }}</legend>
      <label>
        Name
        <input v-model="profile.name" type="text" />
      </label>
      <label>
        Folder command
        <input v-model="profile.folderCmd" data-test="folder-cmd" type="text" />
      </label>
      <label>
        File command
        <input v-model="profile.fileCmd" type="text" />
      </label>
      <label>
        File-at-line command
        <input v-model="profile.fileAtLineCmd" type="text" />
      </label>
    </fieldset>

    <div v-if="distrosWithoutProfile.length">
      Add a distro profile:
      <button
        v-for="d in distrosWithoutProfile"
        :key="d.name"
        type="button"
        data-test="add-distro-profile"
        @click="addDistroProfile(d.name)"
      >
        {{ d.name }}
      </button>
    </div>

    <label>
      Protected branches
      <input v-model="form.protectedBranches" type="text" />
    </label>
    <label>
      Default pull mode
      <select v-model="form.defaultPullMode">
        <option value="merge">merge</option>
        <option value="rebase">rebase</option>
      </select>
    </label>
    <label>
      Diff mode
      <select v-model="form.diffMode">
        <option value="unified">unified</option>
        <option value="side-by-side">side-by-side</option>
      </select>
    </label>
    <label>
      Auto-fetch interval (minutes)
      <input v-model.number="form.autoFetchIntervalMinutes" min="0" type="number" />
    </label>

    <div v-if="distros.length">
      <h2>WSL command</h2>
      <ul>
        <li v-for="d in distros" :key="d.name">
          {{ d.name }}
          <button type="button" @click="installLauncher(d.name)">Install nebegit command</button>
        </li>
      </ul>
    </div>

    <button type="submit">Save</button>
  </form>
</template>
