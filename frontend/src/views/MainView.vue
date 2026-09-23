<script setup lang="ts">
import { provide, ref } from 'vue'
import Splitter from 'openvue/splitter'
import SplitterPanel from 'openvue/splitterpanel'
import Dialog from 'openvue/dialog'
import SelectorPanel from '../components/SelectorPanel.vue'
import SettingsPanel from '../components/SettingsPanel.vue'
import WorktreeContent from './WorktreeContent.vue'
import { openSettingsKey } from '../lib/openSettings'

// MainView is every window's root content: the selector panel always
// renders; WorktreeContent only once a worktree is selected. distro is set
// only for a WSL window (path is then its POSIX form).
defineProps<{ path: string; distro?: string | undefined }>()

// showSettings lives here: settings are reachable with no selection too,
// from the panel and from any ErrorBanner's link.
const showSettings = ref(false)
function openSettings(): void {
  showSettings.value = true
}
provide(openSettingsKey, openSettings)

// SplitterPanel has no built-in collapse; the panel's own toggle
// (SelectorPanel.vue) shrinks its size to a sliver instead, reactively.
const selectorCollapsed = ref(false)
</script>

<template>
  <Splitter layout="horizontal" class="main-window">
    <SplitterPanel :size="selectorCollapsed ? 4 : 20" :min-size="selectorCollapsed ? 4 : 12">
      <SelectorPanel
        v-model:collapsed="selectorCollapsed"
        :selected-path="path"
        @open-settings="openSettings"
      />
    </SplitterPanel>
    <SplitterPanel :size="selectorCollapsed ? 96 : 80" class="main-content">
      <WorktreeContent v-if="path" :path="path" :distro="distro" />
      <p v-else class="empty-state">Select a worktree from the panel, or open a folder to get started.</p>
    </SplitterPanel>
  </Splitter>
  <Dialog
    v-model:visible="showSettings"
    header="Settings"
    modal
    :style="{ width: '720px', maxWidth: '90vw' }"
  >
    <SettingsPanel @close="showSettings = false" />
  </Dialog>
</template>

<style scoped>
/* The panel is the left column; the rest fills the window (style.css's
   height chain). */
.main-window {
  height: 100%;
  min-height: 0;
}
.main-content {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
}
</style>
