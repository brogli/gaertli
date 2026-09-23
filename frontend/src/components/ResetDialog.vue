<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Dialog from 'openvue/dialog'
import RadioButton from 'openvue/radiobutton'
import Button from 'openvue/button'
import { useCoreCall } from '../composables/useCoreCall'
import { useConfirmAction } from '../composables/useConfirmAction'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { ResetMode } from '../../bindings/github.com/brogli/nebegit/internal/core'
import ErrorBanner from './ErrorBanner.vue'

// ResetDialog is the reset mode picker (soft/mixed/hard/keep). Every mode
// but hard runs immediately; hard asks for confirmation, naming how many
// uncommitted changes would be lost. After a successful reset the dialog
// stays open and shows the previous HEAD, which the user might want to
// copy or reset back to.
const props = defineProps<{ wt: string; branch: string; sha: string; subject: string }>()
const emit = defineEmits<{ close: [] }>()

const mode = ref<ResetMode>(ResetMode.ResetMixed) // git's own default
const uncommittedCount = ref<number | null>(null) // null while loading
const previousHead = ref<string | null>(null) // set once a reset has succeeded
const { error, run } = useCoreCall()
const { confirmAction } = useConfirmAction()

const branchLabel = computed(() => props.branch || 'HEAD')

onMounted(async () => {
  uncommittedCount.value = (await run(() => Facade.UncommittedCount(props.wt))) ?? null
})

async function doReset(target: ResetMode): Promise<void> {
  const result = await run(() => Facade.Reset(props.wt, props.sha, target))
  if (result) previousHead.value = result.previousHead
}

function onResetClick(): void {
  if (mode.value !== ResetMode.ResetHard) {
    void doReset(mode.value)
    return
  }
  const count = uncommittedCount.value ?? 0
  confirmAction(
    'Hard reset',
    `Discard ${count} uncommitted change${count === 1 ? '' : 's'} and reset ${branchLabel.value} to ${props.subject}? This cannot be undone.`,
    () => void doReset(ResetMode.ResetHard),
  )
}

function close(): void {
  previousHead.value = null
  emit('close')
}
</script>

<template>
  <Dialog visible header="Reset" modal @update:visible="emit('close')">
    <ErrorBanner v-if="error" :error="error" />
    <template v-if="previousHead !== null">
      <p>Reset done. Previous HEAD: {{ previousHead }}</p>
    </template>
    <template v-else>
      <p>Reset {{ branchLabel }} to {{ subject }}</p>
      <div class="reset-modes">
        <label>
          <RadioButton v-model="mode" :value="ResetMode.ResetSoft" name="reset-mode" input-id="reset-mode-soft" />
          Soft — moves the branch pointer. Index and working files are unchanged.
        </label>
        <label>
          <RadioButton v-model="mode" :value="ResetMode.ResetMixed" name="reset-mode" input-id="reset-mode-mixed" />
          Mixed — moves the branch pointer and resets the index. Working files are unchanged.
        </label>
        <label>
          <RadioButton v-model="mode" :value="ResetMode.ResetHard" name="reset-mode" input-id="reset-mode-hard" />
          Hard — moves the branch pointer, resets the index, and overwrites working files. Uncommitted changes are lost.
        </label>
        <label>
          <RadioButton v-model="mode" :value="ResetMode.ResetKeep" name="reset-mode" input-id="reset-mode-keep" />
          Keep — moves the branch pointer, resets the index and working files, but refuses if any local change would be lost.
        </label>
      </div>
    </template>
    <template #footer>
      <Button v-if="previousHead !== null" label="Close" data-test="reset-close" @click="close" />
      <Button
        v-else
        label="Reset"
        data-test="reset-submit"
        :disabled="mode === ResetMode.ResetHard && uncommittedCount === null"
        @click="onResetClick"
      />
    </template>
  </Dialog>
</template>
