<script setup lang="ts">
import { computed, ref } from 'vue'
import Dialog from 'openvue/dialog'
import Checkbox from 'openvue/checkbox'
import Button from 'openvue/button'
import InputNumber from 'openvue/inputnumber'
import { useI18n } from '../i18n'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { DIALOG } from '../lib/layout'
import ErrorBanner from './ErrorBanner.vue'
import FormField from './FormField.vue'

// PickDialog is cherry-pick's and revert's options dialog: apply without
// committing, add the "cherry picked from" line (cherry-pick only), and,
// for a single merge commit, which parent to pick against. The caller
// knows singleMerge from the loaded log row.
const props = defineProps<{
  wt: string
  verb: 'cherry-pick' | 'revert'
  shas: string[]
  singleMerge: boolean
}>()
const emit = defineEmits<{ close: [] }>()

const noCommit = ref(false)
const recordOrigin = ref(false)
const mainline = ref(0)
const { error, runAndClose } = useCoreCall()
const { t } = useI18n()

const showMainline = computed(() => props.verb === 'cherry-pick' && props.singleMerge)
const header = computed(() =>
  props.verb === 'cherry-pick' ? t('dialogs.pick.cherryPick') : t('dialogs.pick.revert'),
)

async function onSubmit(): Promise<void> {
  const opts = {
    noCommit: noCommit.value,
    recordOrigin: recordOrigin.value,
    mainline: showMainline.value ? mainline.value : 0,
  }
  const pick = props.verb === 'cherry-pick' ? Facade.CherryPick : Facade.Revert
  await runAndClose(
    () => pick(props.wt, props.shas, opts),
    () => emit('close'),
  )
}
</script>

<template>
  <Dialog visible v-bind="DIALOG" :header="header" @update:visible="emit('close')">
    <ErrorBanner v-if="error" :error="error" />
    <label class="action-row">
      <Checkbox v-model="noCommit" binary />
      {{ t('dialogs.pick.applyWithoutCommitting') }}
    </label>
    <label v-if="verb === 'cherry-pick'" class="action-row">
      <Checkbox v-model="recordOrigin" binary />
      {{ t('dialogs.pick.recordOrigin') }}
    </label>
    <FormField v-if="showMainline" v-slot="{ id }" :label="t('dialogs.pick.pickAgainstParent')">
      <InputNumber v-model="mainline" :input-id="id" :min="1" fluid data-test="pick-mainline" />
    </FormField>
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
