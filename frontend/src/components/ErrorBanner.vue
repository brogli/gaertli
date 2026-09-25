<script setup lang="ts">
import Button from 'openvue/button'
import Message from 'openvue/message'
import { useI18n } from '../i18n'
import type { CoreError } from '../composables/useCoreCall'
import { errorHeadline } from '../lib/errorText'
import ErrorDetails from './ErrorDetails.vue'

// ErrorBanner is an error shown in place: OpenVue's error Message with the
// error's headline, a retry button when the error carries one, and its
// details toggle.
defineProps<{ error: CoreError }>()
const { t } = useI18n()
</script>

<template>
  <Message severity="error" class="error-banner">
    <div class="action-row">
      <p>{{ errorHeadline(error) }}</p>
      <Button
        v-if="error.retry"
        :label="t('common.tryAgain')"
        size="small"
        @click="error.retry()"
      />
      <ErrorDetails :error="error" />
    </div>
  </Message>
</template>

<style scoped>
p {
  margin: 0;
  white-space: pre-line;
}
</style>
