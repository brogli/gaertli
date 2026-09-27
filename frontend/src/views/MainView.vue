<script setup lang="ts">
import { ref } from 'vue'
import Dialog from 'openvue/dialog'
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import { AppService } from '../../bindings/github.com/brogli/nebegit/internal/app'
import SettingsPanel from '@/components/SettingsPanel.vue'
import RepoPicker from '@/components/RepoPicker.vue'
import DebugArea from '@/components/DebugArea.vue'
import WorktreeContent from './WorktreeContent.vue'
import { useSettingsStore } from '@/stores/settings'
import { useShortcut } from '@/composables/useShortcut'
import { iconClass } from '@/lib/icons'
import { ICON_BUTTON } from '@/lib/layout'

// MainView is every main window's root content: WorktreeContent once a
// worktree is selected, else the repo list as the page. distro is set only
// for a WSL window (path is then its POSIX form).
defineProps<{ path: string; distro?: string | undefined }>()
const { t } = useI18n()
const settings = useSettingsStore()

// showSettings lives here: settings are reachable with no selection too.
const showSettings = ref(false)

// Ctrl+Shift+N opens a new window, anywhere in a main window; the diff
// window has no such shortcut.
useShortcut(
  (event) => event.ctrlKey && event.shiftKey && event.key.toLowerCase() === 'n',
  () => void AppService.NewWindow(),
)
</script>

<template>
  <!-- The window's content on top, the debug strip (if shown) fixed below
       it: the same shape WorktreeContent.vue uses for its own toolbar and
       body. -->
  <div class="flex h-full min-h-0 flex-col">
    <WorktreeContent
      v-if="path"
      :path="path"
      :distro="distro"
      @open-settings="showSettings = true"
    />
    <main v-else class="start-page flex min-h-0 flex-1 flex-col overflow-auto">
      <header class="flex items-center justify-between border-b border-surface py-1.5 px-2.5">
        <h1>nebegit</h1>
        <Button
          v-bind="ICON_BUTTON"
          :icon="iconClass('settings')"
          data-test="open-settings"
          :title="t('toolbar.settings')"
          :aria-label="t('toolbar.settings')"
          @click="showSettings = true"
        />
      </header>
      <RepoPicker path="" as-page />
    </main>
    <DebugArea v-if="settings.settings?.debugMode" />
  </div>
  <Dialog
    v-model:visible="showSettings"
    :header="t('toolbar.settings')"
    modal
    :style="{ width: '720px', maxWidth: '90vw' }"
  >
    <SettingsPanel @close="showSettings = false" />
  </Dialog>
</template>
