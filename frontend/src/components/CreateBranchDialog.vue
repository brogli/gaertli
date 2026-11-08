<script setup lang="ts">
import { ref } from 'vue'
import Dialog from 'openvue/dialog'
import InputText from 'openvue/inputtext'
import Checkbox from 'openvue/checkbox'
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import { useCoreCall } from '@/composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'
import { DIALOG } from '@/lib/layout'
import ErrorBanner from './ErrorBanner.vue'
import FormField from './FormField.vue'
import StartPointField from './StartPointField.vue'
import { startRev, type StartPoint } from './startPoint'

// CreateBranchDialog is the "new branch" dialog, opened on its start point.
const props = defineProps<{ wt: string; start: StartPoint }>()
const emit = defineEmits<{ close: [] }>()

const name = ref('')
const start = ref(props.start)
const checkout = ref(true)
const { error, runAndClose } = useCoreCall()
const { t } = useI18n()

async function submit(): Promise<void> {
  await runAndClose(
    () => Facade.CreateBranch(props.wt, name.value, startRev(start.value), checkout.value),
    () => emit('close'),
  )
}
</script>

<template>
  <Dialog
    visible
    v-bind="DIALOG"
    :header="t('dialogs.createBranch.header')"
    @update:visible="emit('close')"
  >
    <ErrorBanner v-if="error" :error="error" />
    <FormField v-slot="{ id }" :label="t('dialogs.branchName')">
      <InputText :id="id" v-model="name" autofocus fluid />
    </FormField>
    <StartPointField v-model="start" />
    <label class="flex flex-wrap items-center gap-2">
      <Checkbox v-model="checkout" binary />
      {{ t('dialogs.createBranch.checkoutAfter') }}
    </label>
    <template #footer>
      <Button
        :label="t('common.cancel')"
        severity="secondary"
        data-test="dialog-cancel"
        @click="emit('close')"
      />
      <Button
        :label="t('dialogs.createBranch.create')"
        data-test="create-branch-submit"
        @click="submit"
      />
    </template>
  </Dialog>
</template>
