<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useSettingsStore } from '../stores/settings'
import { useCoreCall } from '../composables/useCoreCall'
import { AppService, Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type {
  EditorProfile,
  Version,
} from '../../bindings/github.com/brogli/nebegit/internal/core'
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

// EDITOR_PRESETS fill a profile's command fields in one click, each preset
// fully replacing the form's current values. IntelliJ has no
// fileAtLineCmd: its line-jump flag isn't verified.
const EDITOR_PRESETS: Record<string, EditorProfile> = {
  Zed: { name: 'Zed', folderCmd: 'zed {path}', fileCmd: 'zed {path}', fileAtLineCmd: 'zed {path}:{line}' },
  'VS Code': {
    name: 'VS Code',
    folderCmd: 'code {path}',
    fileCmd: 'code {path}',
    fileAtLineCmd: 'code -g {path}:{line}',
  },
  'IntelliJ IDEA': { name: 'IntelliJ IDEA', folderCmd: 'idea {path}', fileCmd: 'idea {path}' },
  'Sublime Text': {
    name: 'Sublime Text',
    folderCmd: 'subl {path}',
    fileCmd: 'subl {path}',
    fileAtLineCmd: 'subl {path}:{line}',
  },
  'Neovim (terminal)': {
    name: 'Neovim',
    folderCmd: 'x-terminal-emulator -e nvim {path}',
    fileCmd: 'x-terminal-emulator -e nvim {path}',
    fileAtLineCmd: 'x-terminal-emulator -e nvim +{line} {path}',
  },
}

// applyPreset fully replaces profile's command fields with preset's own,
// never merges: a field the preset omits (IntelliJ's fileAtLineCmd) clears
// the form's current value for it too.
function applyPreset(profile: EditorProfile, preset: EditorProfile): void {
  profile.name = preset.name
  profile.folderCmd = preset.folderCmd
  profile.fileCmd = preset.fileCmd
  profile.fileAtLineCmd = preset.fileAtLineCmd ?? ''
}

const emit = defineEmits<{ close: [] }>()
const store = useSettingsStore()
const { error, run, runAndClose } = useCoreCall()

const form = reactive({
  protectedBranches: '',
  defaultPullMode: 'merge',
  diffMode: 'unified',
  autoFetchIntervalMinutes: 0,
  theme: '',
  ignoreWhitespace: false,
})

// editorProfiles is edited directly, one form per environment, keyed like
// Settings.EditorProfiles ("" = local, else a WSL distro name).
const editorProfiles = reactive<Record<string, EditorProfile>>({})

const distros = ref<Distro[]>([])
const version = ref<Version | null>(null)

// loaded keeps the form hidden until it holds the stored settings, which
// would otherwise overwrite a choice made earlier.
const loaded = ref(false)

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

function testProfile(env: string, profile: EditorProfile): void {
  void run(() => AppService.TestEditorProfile(env, profile))
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
    form.theme = s.theme
    form.ignoreWhitespace = s.ignoreWhitespace
    Object.assign(editorProfiles, s.editorProfiles ?? {})
    if (!('' in editorProfiles)) editorProfiles[''] = { ...DEFAULT_PROFILE } // Local always has a form, even unconfigured
  }
  loaded.value = true
  // Rendered only when distros are listable at all (Windows host): on any
  // other platform this rejects (WslNotFound) and both the "add a distro
  // profile" control and the "WSL command" section stay empty.
  distros.value = (await run(() => store.loadDistros())) ?? []
  version.value = await Facade.Version()
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
    theme: form.theme,
    ignoreWhitespace: form.ignoreWhitespace,
  }
  await runAndClose(
    () => store.save(updated),
    () => emit('close'),
  )
}
</script>

<template>
  <form v-if="loaded" class="settings-panel" @submit.prevent="submit">
    <ErrorBanner v-if="error" :error="error" :editor-actions="false" />

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
      <div class="presets">
        Preset:
        <button
          v-for="(preset, presetName) in EDITOR_PRESETS"
          :key="presetName"
          type="button"
          data-test="editor-preset"
          @click="applyPreset(profile, preset)"
        >
          {{ presetName }}
        </button>
      </div>
      <button type="button" data-test="test-editor-profile" @click="testProfile(name, profile)">Test</button>
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
    <label>
      <input v-model="form.ignoreWhitespace" data-test="ignore-whitespace" type="checkbox" />
      Ignore whitespace in diffs
    </label>
    <label>
      Theme
      <select v-model="form.theme" data-test="theme-select">
        <option value="">System</option>
        <option value="light">Light</option>
        <option value="dark">Dark</option>
      </select>
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

    <div class="footer">
      <p v-if="version" data-test="app-version" class="version">nebegit {{ version.app }}</p>
      <button type="submit">Save</button>
    </div>
  </form>
</template>

<style scoped>
.settings-panel {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
  padding: 0.5rem 0.6rem;
}
.settings-panel fieldset {
  display: flex;
  flex-wrap: wrap;
  gap: 0.6rem;
  align-items: end;
  border-color: var(--p-content-border-color);
  border-radius: 4px;
}
.footer {
  display: flex;
  justify-content: flex-end;
  align-items: center;
  gap: 0.6rem;
}
.version {
  margin: 0;
  font-size: 0.85em;
  color: var(--p-text-muted-color);
}
</style>
