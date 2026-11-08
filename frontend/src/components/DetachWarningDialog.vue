<script setup lang="ts">
import Dialog from 'openvue/dialog'
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import { useCoreCall } from '@/composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'
import { DIALOG } from '@/lib/layout'
import ErrorBanner from './ErrorBanner.vue'

// DetachWarningDialog confirms "Check out commit". A warning, not a hard
// action, so it does not use useConfirmAction.
const props = defineProps<{ wt: string; sha: string }>()
const emit = defineEmits<{ close: [] }>()

const { error, runAndClose } = useCoreCall()
const { t } = useI18n()

async function confirm(): Promise<void> {
  await runAndClose(
    () => Facade.CheckoutCommit(props.wt, props.sha),
    () => emit('close'),
  )
}
</script>

<template>
  <Dialog
    visible
    v-bind="DIALOG"
    :header="t('dialogs.detachWarning.header')"
    @update:visible="emit('close')"
  >
    <ErrorBanner v-if="error" :error="error" />
    <p>{{ t('dialogs.detachWarning.message') }}</p>
    <template #footer>
      <Button
        :label="t('common.cancel')"
        severity="secondary"
        data-test="detach-cancel"
        @click="emit('close')"
      />
      <Button
        :label="t('dialogs.detachWarning.checkOut')"
        data-test="detach-confirm"
        @click="confirm"
      />
    </template>
  </Dialog>
</template>
