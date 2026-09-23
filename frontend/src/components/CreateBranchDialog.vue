<script setup lang="ts">
import { ref } from 'vue'
import Dialog from 'openvue/dialog'
import InputText from 'openvue/inputtext'
import Checkbox from 'openvue/checkbox'
import Button from 'openvue/button'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import ErrorBanner from './ErrorBanner.vue'

// CreateBranchDialog is the "new branch from here" dialog. sha is the
// start point: a commit's sha, or HEAD's sha while detached.
const props = defineProps<{ wt: string; sha: string }>()
const emit = defineEmits<{ close: [] }>()

const name = ref('')
const checkout = ref(true) // most git GUIs default to switching to a freshly created branch
const { error, runAndClose } = useCoreCall()

async function submit(): Promise<void> {
  await runAndClose(
    () => Facade.CreateBranch(props.wt, name.value, props.sha, checkout.value),
    () => {
      name.value = ''
      emit('close')
    },
  )
}
</script>

<template>
  <Dialog visible header="New branch" modal @update:visible="emit('close')">
    <ErrorBanner v-if="error" :error="error" />
    <InputText v-model="name" placeholder="Branch name" />
    <label>
      <Checkbox v-model="checkout" binary />
      Check out after creation
    </label>
    <template #footer>
      <Button label="Create" data-test="create-branch-submit" @click="submit" />
    </template>
  </Dialog>
</template>
