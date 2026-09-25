<script setup lang="ts">
import Button from 'openvue/button'
import { useI18n } from '../i18n'
import { SECONDARY_BUTTON } from '../lib/layout'
import { noticeIconClass } from '../lib/icons'
import { useNotices } from '../lib/notice'

// DebugArea is the debug mode strip along the main window's bottom: one
// button per notice severity, each showing a sample notice through the
// app's normal notice path (lib/notice.ts), so it looks and behaves exactly
// like a real one. Each keeps its own notice kind, so the three samples
// stack instead of replacing each other.
const { t } = useI18n()
const { show } = useNotices()

function sample(kind: string, severity: 'error' | 'warn' | 'success', summary: string): void {
  show(kind, { severity, summary })
}
</script>

<template>
  <div class="debug-area">
    <Button
      v-bind="SECONDARY_BUTTON"
      :icon="noticeIconClass('error')"
      :label="t('debugArea.error')"
      @click="sample('debug-error', 'error', t('debugArea.errorSample'))"
    />
    <Button
      v-bind="SECONDARY_BUTTON"
      :icon="noticeIconClass('warn')"
      :label="t('debugArea.warning')"
      @click="sample('debug-warning', 'warn', t('debugArea.warningSample'))"
    />
    <Button
      v-bind="SECONDARY_BUTTON"
      :icon="noticeIconClass('success')"
      :label="t('debugArea.success')"
      @click="sample('debug-success', 'success', t('debugArea.successSample'))"
    />
  </div>
</template>

<style scoped>
/* A low strip along the window's bottom, the same shape as the toolbar
   above the body (WorktreeContent.vue): fixed height, never grown or
   shrunk by its siblings. */
.debug-area {
  display: flex;
  flex: none;
  align-items: center;
  gap: 0.5rem;
  padding: 0.4rem 0.6rem;
  border-top: 1px solid var(--p-content-border-color);
}
</style>
