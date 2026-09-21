<script setup lang="ts">
import { onMounted, reactive } from 'vue'
import { useSettingsStore } from '../stores/settings'

const store = useSettingsStore()
const form = reactive({
  editor: '',
  protectedBranches: '',
  defaultPullMode: 'merge',
  diffMode: 'unified',
  autoFetchIntervalMinutes: 0,
})

onMounted(async () => {
  await store.load()
  const s = store.settings
  if (!s) return
  form.editor = s.editor
  form.protectedBranches = (s.protectedBranches ?? []).join(', ')
  form.defaultPullMode = s.defaultPullMode
  form.diffMode = s.diffMode
  form.autoFetchIntervalMinutes = s.autoFetchIntervalMinutes
})

async function submit(): Promise<void> {
  if (!store.settings) return
  await store.save({
    ...store.settings,
    editor: form.editor,
    protectedBranches: form.protectedBranches
      .split(',')
      .map((b) => b.trim())
      .filter(Boolean),
    defaultPullMode: form.defaultPullMode,
    diffMode: form.diffMode,
    autoFetchIntervalMinutes: form.autoFetchIntervalMinutes,
  })
}
</script>

<template>
  <form class="settings-panel" @submit.prevent="submit">
    <label>
      Editor
      <input v-model="form.editor" type="text" />
    </label>
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
    <button type="submit">Save</button>
  </form>
</template>
