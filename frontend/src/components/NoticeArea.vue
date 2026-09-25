<script setup lang="ts">
import Toast from 'openvue/toast'
import Button from 'openvue/button'
import { useI18n } from '../i18n'
import { NOTICE_GROUP, type AppNotice } from '../lib/notice'
import { noticeIconClass } from '../lib/icons'
import ErrorDetails from './ErrorDetails.vue'

// NoticeArea is the app's one notice area: OpenVue's Toast in its default
// position and look. The message slot renders what the Toast's own content
// does (severity icon, summary) with its classes, plus an AppNotice's
// extras.
const { t } = useI18n()

function copyToClipboard(text: string): void {
  void navigator.clipboard.writeText(text)
}
</script>

<template>
  <Toast :group="NOTICE_GROUP">
    <template #message="{ message }: { message: AppNotice }">
      <i :class="['p-toast-message-icon', noticeIconClass(message.severity)]" aria-hidden="true" />
      <div class="p-toast-message-text">
        <span class="p-toast-summary">{{ message.summary }}</span>
        <div v-if="message.data" class="p-toast-detail action-row">
          <Button
            v-for="action in message.data.actions ?? []"
            :key="action.label"
            :label="action.label"
            size="small"
            @click="action.run()"
          />
          <Button
            v-if="message.data.copyText"
            :label="t('notices.copyHash')"
            size="small"
            data-test="notice-copy"
            @click="copyToClipboard(message.data.copyText)"
          />
          <ErrorDetails v-if="message.data.error" :error="message.data.error" />
        </div>
      </div>
    </template>
  </Toast>
</template>
