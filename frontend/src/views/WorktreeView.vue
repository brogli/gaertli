<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRepoStore } from '../stores/repo'
import { useCoreCall } from '../composables/useCoreCall'
import ErrorBanner from '../components/ErrorBanner.vue'
import CommandLogPanel from '../components/CommandLogPanel.vue'
import SettingsPanel from '../components/SettingsPanel.vue'

const props = defineProps<{ path: string }>()
const name = props.path.split(/[\\/]/).pop() || props.path
const repo = useRepoStore()
const { error, run } = useCoreCall()
const showSettings = ref(false)

onMounted(() => {
  void run(() => repo.loadSummary(props.path))
})
</script>

<template>
  <main>
    <h1>{{ name }}</h1>
    <button type="button" @click="showSettings = !showSettings">Settings</button>
    <ErrorBanner v-if="error" :error="error" />
    <p v-else-if="repo.summary">
      {{ repo.summary.detached ? 'detached HEAD' : repo.summary.branch }}
      <template v-if="repo.summary.operation"> ({{ repo.summary.operation }})</template>
    </p>
    <SettingsPanel v-if="showSettings" />
    <CommandLogPanel :wt="path" />
  </main>
</template>
