<script setup lang="ts">
import { computed, ref } from 'vue'
import Dialog from 'openvue/dialog'
import Checkbox from 'openvue/checkbox'
import Button from 'openvue/button'
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

const showMainline = computed(() => props.verb === 'cherry-pick' && props.singleMerge)
const header = computed(() => (props.verb === 'cherry-pick' ? 'Cherry-pick' : 'Revert'))

async function onSubmit(): Promise<void> {
  const opts = {
    noCommit: noCommit.value,
    recordOrigin: recordOrigin.value,
    mainline: showMainline.value ? mainline.value : 0,
  }
  const call =
    props.verb === 'cherry-pick'
      ? () => Facade.CherryPickWith(props.wt, props.shas, opts)
      : () => Facade.RevertWith(props.wt, props.shas, opts)
  await runAndClose(call, () => emit('close'))
}
</script>

<template>
  <Dialog visible :header="header" modal @update:visible="emit('close')">
    <ErrorBanner v-if="error" :error="error" />
    <label>
      <Checkbox v-model="noCommit" binary />
      Apply without committing
    </label>
    <label v-if="verb === 'cherry-pick'">
      <Checkbox v-model="recordOrigin" binary />
      Add "cherry picked from" line
    </label>
    <label v-if="showMainline">
      Pick against parent
      <input v-model.number="mainline" type="number" min="1" data-test="pick-mainline" />
    </label>
    <template #footer>
      <Button label="Confirm" data-test="pick-submit" @click="onSubmit" />
    </template>
  </Dialog>
</template>
