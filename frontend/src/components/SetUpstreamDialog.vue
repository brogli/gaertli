<script setup lang="ts">
import { computed, ref } from 'vue'
import Dialog from 'openvue/dialog'
import Select from 'openvue/select'
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import { useCoreCall } from '@/composables/useCoreCall'
import { useRefsStore } from '@/stores/refs'
import { Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'
import { DIALOG } from '@/lib/layout'
import ErrorBanner from './ErrorBanner.vue'
import FormField from './FormField.vue'

// SetUpstreamDialog confirms a first push: the branch has no upstream yet,
// and push.autoSetupRemote does not allow setting one without asking
// (usePush.ts). A question, not a hard action, so it does not use
// useConfirmAction, the same as DetachWarningDialog.
const props = defineProps<{ wt: string; branch: string; remote: string }>()
const emit = defineEmits<{ close: [] }>()

const refs = useRefsStore()
const remoteNames = computed(() => (refs.refs?.remotes ?? []).map((r) => r.name))
const hasChoice = computed(() => remoteNames.value.length > 1)
const chosen = ref(props.remote)
const { error, runAndClose } = useCoreCall()
const { t } = useI18n()

async function confirm(): Promise<void> {
  await runAndClose(
    () => Facade.Push(props.wt, chosen.value, props.branch),
    () => emit('close'),
  )
}
</script>

<template>
  <Dialog
    visible
    v-bind="DIALOG"
    :header="t('dialogs.setUpstream.header')"
    @update:visible="emit('close')"
  >
    <ErrorBanner v-if="error" :error="error" />
    <FormField v-if="hasChoice" v-slot="{ id }" :label="t('refs.remote')">
      <Select
        v-model="chosen"
        :options="remoteNames"
        :input-id="id"
        fluid
        data-test="set-upstream-remote"
      />
    </FormField>
    <p>
      {{
        hasChoice
          ? t('dialogs.setUpstream.messageChoose', { branch })
          : t('dialogs.setUpstream.message', { branch, remote })
      }}
    </p>
    <template #footer>
      <Button
        :label="t('common.cancel')"
        severity="secondary"
        data-test="dialog-cancel"
        @click="emit('close')"
      />
      <Button
        :label="t('dialogs.setUpstream.header')"
        :disabled="!chosen"
        data-test="set-upstream-confirm"
        @click="confirm"
      />
    </template>
  </Dialog>
</template>
