<script setup lang="ts">
import Dialog from 'openvue/dialog'
import Button from 'openvue/button'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import ErrorBanner from './ErrorBanner.vue'

// DetachWarningDialog is FR-BR-08's own one-off confirmation, opened from
// the commit-row context menu's "Check out commit" (spec 22, FR-LOG-11).
// Not FR-UX-03's shared confirmation-rule component: that is spec 41's own
// build, and FR-BR-08's wording is not in FR-UX-03's fixed list anyway.
const props = defineProps<{ wt: string; sha: string; visible: boolean }>()
const emit = defineEmits<{ 'update:visible': [boolean] }>()

const { error, runAndClose } = useCoreCall()

async function confirm(): Promise<void> {
  await runAndClose(
    () => Facade.CheckoutCommit(props.wt, props.sha),
    () => emit('update:visible', false),
  )
}

function cancel(): void {
  emit('update:visible', false)
}
</script>

<template>
  <Dialog
    :visible="visible"
    header="Check out commit"
    modal
    @update:visible="(v: boolean) => emit('update:visible', v)"
  >
    <ErrorBanner v-if="error" :error="error" />
    <p>Checking out this commit detaches HEAD. Commits made in this state belong to no branch.</p>
    <template #footer>
      <Button label="Cancel" severity="secondary" data-test="detach-cancel" @click="cancel" />
      <Button label="Check out" data-test="detach-confirm" @click="confirm" />
    </template>
  </Dialog>
</template>
