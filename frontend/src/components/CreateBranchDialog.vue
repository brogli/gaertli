<script setup lang="ts">
import { ref } from 'vue'
import Dialog from 'openvue/dialog'
import InputText from 'openvue/inputtext'
import Checkbox from 'openvue/checkbox'
import Button from 'openvue/button'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import ErrorBanner from './ErrorBanner.vue'

// CreateBranchDialog is FR-BR-01's "new branch from here" dialog, opened
// from the commit-row context menu (spec 22, FR-LOG-11) or the
// detached-HEAD "Create branch here" action (FR-BR-08). sha is the start
// point the caller already resolved (a commit's own sha, or the worktree's
// own HEAD sha while detached) — this component has no branch-listing
// logic of its own.
const props = defineProps<{ wt: string; sha: string; visible: boolean }>()
const emit = defineEmits<{ 'update:visible': [boolean] }>()

const name = ref('')
const checkout = ref(true) // most git GUIs default to switching to a freshly created branch
const { error, runAndClose } = useCoreCall()

async function submit(): Promise<void> {
  await runAndClose(
    () => Facade.CreateBranch(props.wt, name.value, props.sha, checkout.value),
    () => {
      name.value = ''
      emit('update:visible', false)
    },
  )
}
</script>

<template>
  <Dialog
    :visible="visible"
    header="New branch"
    modal
    @update:visible="(v: boolean) => emit('update:visible', v)"
  >
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
