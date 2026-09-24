<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from '../i18n'
import { useSettingsStore } from '../stores/settings'
import { useCoreCall } from '../composables/useCoreCall'
import { AppService, Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { EditorProfile, Version } from '../../bindings/github.com/brogli/nebegit/internal/core'
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

// EDITOR_PRESETS fill a profile's fields in one click. IntelliJ has no
// fileAtLineCmd: its line-jump flag isn't verified.
const EDITOR_PRESETS: Record<string, EditorProfile> = {
  Zed: DEFAULT_PROFILE,
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

// applyPreset replaces profile's fields with preset's own, never merges: a
// field the preset omits (IntelliJ's fileAtLineCmd) clears the form's value.
function applyPreset(profile: EditorProfile, preset: EditorProfile): void {
  Object.assign(profile, { fileAtLineCmd: '' }, preset)
}

const emit = defineEmits<{ close: [] }>()
const store = useSettingsStore()
const { error, run, runAndClose } = useCoreCall()
const { t } = useI18n()

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
const logPath = ref('')
const logError = ref('')

// loaded keeps the form hidden until it holds the stored settings, which
// would otherwise overwrite a choice made earlier.
const loaded = ref(false)

// distrosWithoutProfile feeds "add a distro profile": only distros not
// already configured are offered.
const distrosWithoutProfile = computed(() =>
  distros.value.filter((d) => !(d.name in editorProfiles)),
)

// Each environment's form sets the command behind its "Open in …".
function environmentLabel(name: string): string {
  return name === '' ? t('settings.openInEditor') : t('settings.openInEditorWsl', { name })
}

function addDistroProfile(name: string): void {
  editorProfiles[name] = { ...DEFAULT_PROFILE }
}

function installLauncher(distro: string): void {
  void run(() => AppService.InstallWSLLauncher(distro))
}

async function loadForm(): Promise<void> {
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
    // The local one always has a form, even unconfigured.
    editorProfiles[''] ??= { ...DEFAULT_PROFILE }
  }
  loaded.value = true
}

onMounted(() =>
  Promise.all([
    loadForm(),
    // A host without WSL lists none: no "add a distro profile" control and
    // no "WSL command" section.
    run(() => store.loadDistros()).then((found) => (distros.value = found ?? [])),
    Facade.Version().then((v) => (version.value = v)),
    AppService.LogPath().then((log) => {
      logPath.value = log.path
      logError.value = log.error
    }),
  ]),
)

async function submit(): Promise<void> {
  if (!store.settings) return
  const updated = {
    ...store.settings,
    ...form,
    editorProfiles: { ...editorProfiles },
    protectedBranches: form.protectedBranches
      .split(',')
      .map((b) => b.trim())
      .filter(Boolean),
  }
  await runAndClose(
    () => store.save(updated),
    () => emit('close'),
  )
}
</script>

<template>
  <form v-if="loaded" class="settings-panel" @submit.prevent="submit">
    <ErrorBanner v-if="error" :error="error" />

    <fieldset v-for="(profile, name) in editorProfiles" :key="name">
      <legend>{{ environmentLabel(name) }}</legend>
      <label>
        {{ t('settings.nameLabel', { name: profile.name || t('toolbar.editorFallback') }) }}
        <input v-model="profile.name" type="text" />
      </label>
      <label>
        {{ t('settings.folderCommand') }}
        <input v-model="profile.folderCmd" data-test="folder-cmd" type="text" />
      </label>
      <label>
        {{ t('settings.fileCommand') }}
        <input v-model="profile.fileCmd" type="text" />
      </label>
      <label>
        {{ t('settings.fileAtLineCommand') }}
        <input v-model="profile.fileAtLineCmd" type="text" />
      </label>
      <div class="presets">
        {{ t('settings.preset') }}
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
    </fieldset>

    <div v-if="distrosWithoutProfile.length">
      {{ t('settings.addWslProfile') }}
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
      {{ t('settings.protectedBranches') }}
      <input v-model="form.protectedBranches" type="text" />
    </label>
    <label>
      {{ t('settings.defaultPullMode') }}
      <select v-model="form.defaultPullMode">
        <option value="merge">{{ t('settings.pullModeMerge') }}</option>
        <option value="rebase">{{ t('settings.pullModeRebase') }}</option>
      </select>
    </label>
    <label>
      {{ t('settings.diffMode') }}
      <select v-model="form.diffMode">
        <option value="unified">{{ t('settings.diffModeUnified') }}</option>
        <option value="side-by-side">{{ t('settings.diffModeSideBySide') }}</option>
      </select>
    </label>
    <label>
      {{ t('settings.autoFetchInterval') }}
      <input v-model.number="form.autoFetchIntervalMinutes" min="0" type="number" />
    </label>
    <label>
      <input v-model="form.ignoreWhitespace" data-test="ignore-whitespace" type="checkbox" />
      {{ t('settings.ignoreWhitespace') }}
    </label>
    <label>
      {{ t('settings.theme') }}
      <select v-model="form.theme" data-test="theme-select">
        <option value="">{{ t('settings.themeSystem') }}</option>
        <option value="light">{{ t('settings.themeLight') }}</option>
        <option value="dark">{{ t('settings.themeDark') }}</option>
      </select>
    </label>

    <div v-if="distros.length">
      <h2>{{ t('settings.wslCommand') }}</h2>
      <ul>
        <li v-for="d in distros" :key="d.name">
          {{ d.name }}
          <button type="button" @click="installLauncher(d.name)">
            {{ t('settings.installLauncher') }}
          </button>
        </li>
      </ul>
    </div>

    <p v-if="logError" data-test="app-log">{{ t('settings.appLogError', { error: logError }) }}</p>
    <p v-else-if="logPath" data-test="app-log" class="log-path">
      {{ t('settings.appLog', { path: logPath }) }}
    </p>

    <div class="footer">
      <p v-if="version" data-test="app-version" class="version">nebegit {{ version.app }}</p>
      <button type="submit">{{ t('settings.save') }}</button>
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
.log-path {
  font-family: monospace;
  font-size: 0.85em;
  color: var(--p-text-muted-color);
}
</style>
