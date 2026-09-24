<script setup lang="ts">
import { ref } from 'vue'
import Dialog from 'openvue/dialog'
import { useI18n } from '../i18n'
import SettingsPanel from '../components/SettingsPanel.vue'
import RepoPicker from '../components/RepoPicker.vue'
import WorktreeContent from './WorktreeContent.vue'
import { iconClass } from '../lib/icons'

// MainView is every window's root content: WorktreeContent once a worktree
// is selected, else the repo list as the page. distro is set only for
// a WSL window (path is then its POSIX form).
defineProps<{ path: string; distro?: string | undefined }>()
const { t } = useI18n()

// showSettings lives here: settings are reachable with no selection too.
const showSettings = ref(false)
</script>

<template>
  <WorktreeContent v-if="path" :path="path" :distro="distro" @open-settings="showSettings = true" />
  <main v-else class="start-page">
    <header class="start-header">
      <h1>nebegit</h1>
      <button
        type="button"
        class="glyph-btn"
        data-test="open-settings"
        :title="t('toolbar.settings')"
        :aria-label="t('toolbar.settings')"
        @click="showSettings = true"
      >
        <i :class="iconClass('settings')" aria-hidden="true"></i>
      </button>
    </header>
    <RepoPicker path="" as-page />
  </main>
  <Dialog
    v-model:visible="showSettings"
    :header="t('toolbar.settings')"
    modal
    :style="{ width: '720px', maxWidth: '90vw' }"
  >
    <SettingsPanel @close="showSettings = false" />
  </Dialog>
</template>

<style scoped>
.start-page {
  display: flex;
  flex-direction: column;
  height: 100%;
  overflow: auto;
}
.start-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.4rem 0.6rem;
  border-bottom: 1px solid var(--p-content-border-color);
}
</style>
