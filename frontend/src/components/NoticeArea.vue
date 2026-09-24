<script setup lang="ts">
import Toast from 'openvue/toast'
import { useI18n } from '../i18n'
import { NOTICE_GROUP, type NoticeAction } from '../lib/notice'
import type { CoreError } from '../composables/useCoreCall'
import ErrorBanner from './ErrorBanner.vue'

// NoticeArea is the app's one notice area, bottom-right so it never covers
// the toolbar. It renders an AppNotice's extras.
const { t } = useI18n()

function copyToClipboard(text: string): void {
  void navigator.clipboard.writeText(text)
}
</script>

<template>
  <Toast :group="NOTICE_GROUP" position="bottom-right">
    <template #container="{ message, closeCallback }">
      <div class="notice">
        <ErrorBanner v-if="message.data?.error" :error="message.data.error as CoreError" />
        <span v-else>{{ message.summary }}</span>
        <button
          v-for="action in (message.data?.actions as NoticeAction[] | undefined) ?? []"
          :key="action.label"
          type="button"
          @click="action.run()"
        >
          {{ action.label }}
        </button>
        <button
          v-if="message.data?.copyText"
          type="button"
          data-test="notice-copy"
          @click="copyToClipboard(message.data.copyText)"
        >
          {{ t('notices.copyHash') }}
        </button>
        <button type="button" data-test="notice-close" @click="closeCallback">
          {{ t('notices.close') }}
        </button>
      </div>
    </template>
  </Toast>
</template>

<style scoped>
.notice {
  display: flex;
  align-items: center;
  gap: 0.6em;
  padding: 0.5em 0.8em;
  border-radius: 4px;
  background: var(--p-content-background);
  border: 1px solid var(--p-content-border-color);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.2);
}
</style>
