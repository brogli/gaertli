<script setup lang="ts">
import { inject, ref } from 'vue'
import type { CoreError } from '../composables/useCoreCall'
import { EditorFallbackKind } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { openSettingsKey } from '../lib/openSettings'

// editorActions: false hides the editor buttons for an error with no
// worktree editor behind it (SettingsPanel's Test button).
const { editorActions = true } = defineProps<{ error: CoreError; editorActions?: boolean }>()
const emit = defineEmits<{ fallback: [kind: EditorFallbackKind] }>()
// openSettings is absent outside a main window (the compare-diff window has
// no Settings dialog), and so is its link.
const openSettings = inject(openSettingsKey, null)
const expanded = ref(false)
</script>

<template>
  <div class="error-banner">
    <p>{{ error.message }}</p>
    <button type="button" @click="expanded = !expanded">show details</button>
    <template v-if="editorActions && error.code === 'EDITOR_NOT_FOUND'">
      <button v-if="openSettings" type="button" @click="openSettings">Open editor setting</button>
      <button type="button" @click="emit('fallback', EditorFallbackKind.FallbackGitEditor)">
        Open with git's editor
      </button>
      <button type="button" @click="emit('fallback', EditorFallbackKind.FallbackSystem)">
        Open with system default
      </button>
    </template>
    <pre v-if="expanded">{{ [error.command, error.gitStderr].filter(Boolean).join('\n') }}</pre>
  </div>
</template>

<style scoped>
.error-banner {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
  padding: 0.3rem 0.6rem;
  border: 1px solid var(--p-content-border-color);
  border-radius: 4px;
  background: var(--p-content-hover-background);
}
.error-banner p {
  margin: 0;
  white-space: pre-line;
}
.error-banner pre {
  flex-basis: 100%;
  margin: 0;
  white-space: pre-wrap;
}
</style>
