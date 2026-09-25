<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { System } from '@wailsio/runtime'
import AutoComplete from 'openvue/autocomplete'
import Button from 'openvue/button'
import Checkbox from 'openvue/checkbox'
import Fieldset from 'openvue/fieldset'
import InputNumber from 'openvue/inputnumber'
import InputText from 'openvue/inputtext'
import SelectButton from 'openvue/selectbutton'
import { useI18n } from '../i18n'
import { SECONDARY_BUTTON } from '../lib/layout'
import { useRefsStore } from '../stores/refs'
import { useSettingsStore } from '../stores/settings'
import { useCoreCall } from '../composables/useCoreCall'
import { AppService, Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import {
  PullMode,
  type EditorProfile,
  type Version,
} from '../../bindings/github.com/brogli/nebegit/internal/core'
import { DiffMode, Theme } from '../../bindings/github.com/brogli/nebegit/internal/settings'
import ErrorBanner from './ErrorBanner.vue'
import FormField from './FormField.vue'

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
const refs = useRefsStore()
const { error, run, runAndClose } = useCoreCall()
const { t } = useI18n()

const form = reactive({
  protectedBranches: [] as string[],
  defaultPullMode: PullMode.PullMerge,
  diffMode: DiffMode.UnifiedDiff,
  autoFetchIntervalMinutes: 0,
  ignoreWhitespace: false,
  debugMode: false,
})

interface Choice {
  label: string
  value: string
}

// choices binds a SelectButton to options; one of them is always chosen.
function choices(options: Choice[]) {
  return { options, optionLabel: 'label', optionValue: 'value', allowEmpty: false }
}

const pullModes = computed<Choice[]>(() => [
  { label: t('settings.pullModeMerge'), value: PullMode.PullMerge },
  { label: t('settings.pullModeRebase'), value: PullMode.PullRebase },
])
const diffModes = computed<Choice[]>(() => [
  { label: t('settings.diffModeUnified'), value: DiffMode.UnifiedDiff },
  { label: t('settings.diffModeSideBySide'), value: DiffMode.SideBySideDiff },
])
// The theme switch's value for the stored "" (follow the OS preference):
// OpenVue's form controls drop an empty initial value, so "" itself would
// show no option selected.
const SYSTEM_THEME = 'system'
const theme = ref<Theme | typeof SYSTEM_THEME>(Theme.SystemTheme)
const themes = computed<Choice[]>(() => [
  { label: t('settings.themeSystem'), value: SYSTEM_THEME },
  { label: t('settings.themeLight'), value: Theme.LightTheme },
  { label: t('settings.themeDark'), value: Theme.DarkTheme },
])

// editorProfiles is edited directly, one form per environment, keyed like
// Settings.EditorProfiles ("" = local, else a WSL distro name).
const editorProfiles = reactive<Record<string, EditorProfile>>({})

// worktreeFolders is edited directly, keyed like Settings.WorktreeFolders.
// Every environment on this host has a field; an entry of a distro not
// found now is kept as stored.
const worktreeFolders = reactive<Record<string, string>>({})

const distros = ref<string[]>([])
const version = ref<Version | null>(null)
const logPath = ref('')
const logError = ref('')

// loaded keeps the form hidden until it holds the stored settings, which
// would otherwise overwrite a choice made earlier.
const loaded = ref(false)

// environments is the local system ('') and each installed WSL distro.
const environments = computed(() => ['', ...distros.value])

// editorEnvironments lists every environment with an editor form: each of
// environments, and any distro no longer installed whose profile is still
// stored (case: a stored "Debian" profile with only "Ubuntu" installed).
const editorEnvironments = computed(() => [
  ...new Set([...environments.value, ...Object.keys(editorProfiles)]),
])

// A newly listed environment (a distro just discovered, or the local one
// before its stored settings load) starts from the built-in default.
watch(
  editorEnvironments,
  (envs) => {
    for (const name of envs) editorProfiles[name] ??= { ...DEFAULT_PROFILE }
  },
  { immediate: true },
)

// profile reads name's editor form state; the watch above guarantees an
// entry exists for every name in editorEnvironments before it renders.
function profile(name: string): EditorProfile {
  return editorProfiles[name]!
}

// Each environment's form sets the command behind its "Open in …".
function environmentLabel(name: string): string {
  return name === ''
    ? t('settings.editorLocal', { os: System.IsWindows() ? 'Windows' : 'Linux' })
    : t('settings.openInEditorWsl', { name })
}

function worktreeFolderLabel(name: string): string {
  return name === '' ? t('settings.worktreeFolder') : t('settings.worktreeFolderWsl', { name })
}

// nonEmptyFolders is worktreeFolders trimmed, without empty entries: an
// empty one means the default.
function nonEmptyFolders(): Record<string, string> {
  return Object.fromEntries(
    Object.entries(worktreeFolders)
      .map(([name, folder]) => [name, folder.trim()])
      .filter(([, folder]) => folder),
  )
}

const protectedBranchSuggestions = ref<string[]>([])

// completeProtectedBranches suggests the repo's branch names (local and
// remote, short names, deduplicated) containing the typed query, then the
// typed query itself, so Enter (which picks the first, focused suggestion)
// always turns the typed text into a chip. A name already chosen is never
// suggested.
function completeProtectedBranches(event: { query: string }): void {
  const local = (refs.refs?.branches ?? []).map((b) => b.name)
  const remote = (refs.refs?.remotes ?? []).flatMap((r) => (r.branches ?? []).map((b) => b.name))
  const query = event.query.toLowerCase()
  const matches = [...local, ...remote].filter((name) => name.toLowerCase().includes(query))
  protectedBranchSuggestions.value = [...new Set([...matches, event.query])].filter(
    (name) => !form.protectedBranches.includes(name),
  )
}

async function loadForm(): Promise<void> {
  await store.load()
  const s = store.settings
  if (s) {
    form.protectedBranches = s.protectedBranches ?? []
    form.defaultPullMode = s.defaultPullMode
    form.diffMode = s.diffMode
    form.autoFetchIntervalMinutes = s.autoFetchIntervalMinutes
    theme.value = s.theme || SYSTEM_THEME
    form.ignoreWhitespace = s.ignoreWhitespace
    form.debugMode = s.debugMode
    Object.assign(editorProfiles, s.editorProfiles ?? {})
    Object.assign(worktreeFolders, s.worktreeFolders ?? {})
  }
  loaded.value = true
}

onMounted(() =>
  Promise.all([
    loadForm(),
    // A host without WSL lists none: no "add a distro profile" control.
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
    theme: theme.value === SYSTEM_THEME ? Theme.SystemTheme : theme.value,
    editorProfiles: { ...editorProfiles },
    worktreeFolders: nonEmptyFolders(),
    protectedBranches: form.protectedBranches.map((b) => b.trim()).filter(Boolean),
  }
  await runAndClose(
    () => store.save(updated),
    () => emit('close'),
  )
}
</script>

<template>
  <form v-if="loaded" class="settings-panel form-rows" @submit.prevent="submit">
    <ErrorBanner v-if="error" :error="error" />

    <Fieldset v-for="name in editorEnvironments" :key="name" :legend="environmentLabel(name)">
      <div class="form-rows">
        <FormField
          v-slot="{ id }"
          :label="
            t('settings.nameLabel', { name: profile(name).name || t('toolbar.editorFallback') })
          "
        >
          <InputText :id="id" v-model="profile(name).name" fluid />
        </FormField>
        <FormField v-slot="{ id }" :label="t('settings.folderCommand')">
          <InputText :id="id" v-model="profile(name).folderCmd" fluid data-test="folder-cmd" />
        </FormField>
        <FormField v-slot="{ id }" :label="t('settings.fileCommand')">
          <InputText :id="id" v-model="profile(name).fileCmd" fluid />
        </FormField>
        <FormField v-slot="{ id }" :label="t('settings.fileAtLineCommand')">
          <InputText :id="id" v-model="profile(name).fileAtLineCmd" fluid />
        </FormField>
        <div class="action-row">
          {{ t('settings.preset') }}
          <Button
            v-for="(preset, presetName) in EDITOR_PRESETS"
            :key="presetName"
            v-bind="SECONDARY_BUTTON"
            :label="presetName"
            data-test="editor-preset"
            @click="applyPreset(profile(name), preset)"
          />
        </div>
      </div>
    </Fieldset>

    <FormField v-slot="{ id }" :label="t('settings.protectedBranches')">
      <AutoComplete
        v-model="form.protectedBranches"
        :suggestions="protectedBranchSuggestions"
        multiple
        fluid
        auto-option-focus
        :input-id="id"
        @complete="completeProtectedBranches"
      />
    </FormField>
    <FormField v-slot="{ labelId }" :label="t('settings.defaultPullMode')">
      <SelectButton
        v-model="form.defaultPullMode"
        v-bind="choices(pullModes)"
        :aria-labelledby="labelId"
      />
    </FormField>
    <FormField v-slot="{ labelId }" :label="t('settings.diffMode')">
      <SelectButton
        v-model="form.diffMode"
        v-bind="choices(diffModes)"
        :aria-labelledby="labelId"
      />
    </FormField>
    <FormField v-slot="{ id }" :label="t('settings.autoFetchInterval')">
      <InputNumber v-model="form.autoFetchIntervalMinutes" :input-id="id" :min="0" />
    </FormField>
    <FormField
      v-for="name in environments"
      :key="name"
      v-slot="{ id }"
      :label="worktreeFolderLabel(name)"
    >
      <InputText
        :id="id"
        v-model="worktreeFolders[name]"
        :placeholder="t('settings.worktreeFolderDefault')"
        fluid
      />
    </FormField>
    <label class="action-row">
      <Checkbox v-model="form.ignoreWhitespace" binary data-test="ignore-whitespace" />
      {{ t('settings.ignoreWhitespace') }}
    </label>
    <label class="action-row">
      <Checkbox v-model="form.debugMode" binary data-test="debug-mode" />
      {{ t('settings.debugMode') }}
    </label>
    <FormField v-slot="{ labelId }" :label="t('settings.theme')">
      <SelectButton
        v-model="theme"
        v-bind="choices(themes)"
        :aria-labelledby="labelId"
        data-test="theme-select"
      />
    </FormField>

    <p v-if="logError" data-test="app-log">{{ t('settings.appLogError', { error: logError }) }}</p>
    <p v-else-if="logPath" data-test="app-log" class="log-path">
      {{ t('settings.appLog', { path: logPath }) }}
    </p>

    <div class="footer">
      <p v-if="version" data-test="app-version" class="version">nebegit {{ version.app }}</p>
      <Button type="submit" :label="t('settings.save')" />
    </div>
  </form>
</template>

<style scoped>
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
  font-family: var(--nb-font-mono);
  font-size: 0.85em;
  color: var(--p-text-muted-color);
}
</style>
