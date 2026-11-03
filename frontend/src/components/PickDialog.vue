<script setup lang="ts">
import { computed, ref } from 'vue'
import Dialog from 'openvue/dialog'
import Checkbox from 'openvue/checkbox'
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import { useCoreCall } from '@/composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { openOnConflicts } from '@/lib/openConflicts'
import { DIALOG } from '@/lib/layout'
import ErrorBanner from './ErrorBanner.vue'

// PickDialog is cherry-pick's and revert's options dialog: apply without
// committing, and add the "cherry picked from" line (cherry-pick only). A
// stop on conflicts opens the diff window and closes the dialog.
const props = defineProps<{
  wt: string
  verb: 'cherry-pick' | 'revert'
  shas: string[]
}>()
const emit = defineEmits<{ close: [] }>()

const noCommit = ref(false)
const recordOrigin = ref(false)
const { error, runAndClose } = useCoreCall()
const { t } = useI18n()

const header = computed(() =>
  props.verb === 'cherry-pick' ? t('dialogs.pick.cherryPick') : t('dialogs.pick.revert'),
)

async function onSubmit(): Promise<void> {
  const opts = { noCommit: noCommit.value, recordOrigin: recordOrigin.value }
  const pick = props.verb === 'cherry-pick' ? Facade.CherryPick : Facade.Revert
  await runAndClose(
    () => pick(props.wt, props.shas, opts),
    () => emit('close'),
    openOnConflicts(props.wt),
  )
}
</script>

<template>
  <Dialog visible v-bind="DIALOG" :header="header" @update:visible="emit('close')">
    <ErrorBanner v-if="error" :error="error" />
    <label class="flex flex-wrap items-center gap-2">
      <Checkbox v-model="noCommit" binary />
      {{ t('dialogs.pick.applyWithoutCommitting') }}
    </label>
    <label v-if="verb === 'cherry-pick'" class="flex flex-wrap items-center gap-2">
      <Checkbox v-model="recordOrigin" binary />
      {{ t('dialogs.pick.recordOrigin') }}
    </label>
    <template #footer>
      <Button
        :label="t('common.cancel')"
        severity="secondary"
        data-test="dialog-cancel"
        @click="emit('close')"
      />
      <Button :label="t('dialogs.pick.confirm')" data-test="pick-submit" @click="onSubmit" />
    </template>
  </Dialog>
</template>
