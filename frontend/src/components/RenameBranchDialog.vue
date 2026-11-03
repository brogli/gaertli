<script setup lang="ts">
import { ref } from 'vue'
import Dialog from 'openvue/dialog'
import InputText from 'openvue/inputtext'
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import { useCoreCall } from '@/composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { DIALOG } from '@/lib/layout'
import ErrorBanner from './ErrorBanner.vue'
import FormField from './FormField.vue'

// RenameBranchDialog edits a local branch's name in place, pre-filled with
// its current one.
const props = defineProps<{ wt: string; name: string }>()
const emit = defineEmits<{ close: [] }>()

const newName = ref(props.name)
const { error, runAndClose } = useCoreCall()
const { t } = useI18n()

async function submit(): Promise<void> {
  await runAndClose(
    () => Facade.RenameBranch(props.wt, props.name, newName.value),
    () => emit('close'),
  )
}
</script>

<template>
  <Dialog
    visible
    v-bind="DIALOG"
    :header="t('dialogs.renameBranch.header')"
    @update:visible="emit('close')"
  >
    <ErrorBanner v-if="error" :error="error" />
    <FormField v-slot="{ id }" :label="t('dialogs.branchName')">
      <InputText :id="id" v-model="newName" autofocus fluid />
    </FormField>
    <template #footer>
      <Button
        :label="t('common.cancel')"
        severity="secondary"
        data-test="dialog-cancel"
        @click="emit('close')"
      />
      <Button
        :label="t('dialogs.renameBranch.rename')"
        data-test="rename-branch-submit"
        @click="submit"
      />
    </template>
  </Dialog>
</template>
