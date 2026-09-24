<script setup lang="ts">
import { computed, ref } from 'vue'
import Dialog from 'openvue/dialog'
import Checkbox from 'openvue/checkbox'
import Button from 'openvue/button'
import { useI18n } from '../i18n'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import ErrorBanner from './ErrorBanner.vue'

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
  <Dialog visible :header="header" modal @update:visible="emit('close')">
    <ErrorBanner v-if="error" :error="error" />
    <label>
      <Checkbox v-model="noCommit" binary />
      {{ t('dialogs.pick.applyWithoutCommitting') }}
    </label>
    <label v-if="verb === 'cherry-pick'">
      <Checkbox v-model="recordOrigin" binary />
      {{ t('dialogs.pick.recordOrigin') }}
    </label>
    <label v-if="showMainline">
      {{ t('dialogs.pick.pickAgainstParent') }}
      <input v-model.number="mainline" type="number" min="1" data-test="pick-mainline" />
    </label>
    <template #footer>
      <Button :label="t('dialogs.pick.confirm')" data-test="pick-submit" @click="onSubmit" />
    </template>
  </Dialog>
</template>
