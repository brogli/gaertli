<script setup lang="ts">
import { ref } from 'vue'
import SelectorPanel from '../components/SelectorPanel.vue'
import SettingsPanel from '../components/SettingsPanel.vue'
import WorktreeContent from './WorktreeContent.vue'

// MainView is every window's root content: the selector panel always
// renders; WorktreeContent only once a worktree is selected.
defineProps<{ path: string }>()

// showSettings lives here: settings are reachable with no selection too,
// from the panel and from a worktree's ErrorBanner link.
const showSettings = ref(false)
</script>

<template>
  <div class="main-window">
    <SelectorPanel :selected-path="path" @open-settings="showSettings = true" />
    <div class="main-content">
      <SettingsPanel v-if="showSettings" @close="showSettings = false" />
      <WorktreeContent v-if="path" :path="path" @open-settings="showSettings = true" />
      <p v-else class="empty-state">Select a worktree from the panel, or open a folder to get started.</p>
    </div>
  </div>
</template>

<style scoped>
/* The panel is the left column; the rest fills the window (style.css's
   height chain). */
.main-window {
  display: flex;
  height: 100%;
  min-height: 0;
}
.main-content {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
  min-height: 0;
}
</style>
