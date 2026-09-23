<script setup lang="ts">
import Dialog from 'openvue/dialog'
import Button from 'openvue/button'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import ErrorBanner from './ErrorBanner.vue'

// DetachWarningDialog confirms "Check out commit". A warning, not a hard
// action, so it does not use useConfirmAction.
const props = defineProps<{ wt: string; sha: string }>()
const emit = defineEmits<{ close: [] }>()

const { error, runAndClose } = useCoreCall()

async function confirm(): Promise<void> {
  await runAndClose(
    () => Facade.CheckoutCommit(props.wt, props.sha),
    () => emit('close'),
  )
}
</script>

<template>
  <Dialog visible header="Check out commit" modal @update:visible="emit('close')">
    <ErrorBanner v-if="error" :error="error" />
    <p>Checking out this commit detaches HEAD. Commits made in this state belong to no branch.</p>
    <template #footer>
      <Button label="Cancel" severity="secondary" data-test="detach-cancel" @click="emit('close')" />
      <Button label="Check out" data-test="detach-confirm" @click="confirm" />
    </template>
  </Dialog>
</template>
