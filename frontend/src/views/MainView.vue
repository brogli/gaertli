<script setup lang="ts">
import { ref } from 'vue'
import SelectorPanel from '../components/SelectorPanel.vue'
import SettingsPanel from '../components/SettingsPanel.vue'
import WorktreeContent from './WorktreeContent.vue'

// MainView is every window's root content (ADR-9): the selector panel
// always renders, whether or not this window has a worktree selected
// (FR-UX-04's "Settings reachable with no selection too"); WorktreeContent
// renders only once one is. There is no distinct "no worktree" view, just
// the panel's own empty state.
defineProps<{ path: string }>()

// showSettings lives here, not in WorktreeContent: it must be reachable
// with no selection too, and it is shared by the panel's own entry point
// and a selected worktree's ErrorBanner link (FR-ED-06).
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
/* FR-UX-07/08: the panel is the left column; the rest fills the window
   (style.css's height chain). */
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
