<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Dialog from 'openvue/dialog'
import RadioButton from 'openvue/radiobutton'
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import { useCoreCall } from '@/composables/useCoreCall'
import { useConfirmAction } from '@/composables/useConfirmAction'
import { Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'
import { ResetMode } from '../../bindings/github.com/brogli/gaertli/internal/core'
import { shortSha } from '@/lib/format'
import { useNotices } from '@/lib/notice'
import { DIALOG } from '@/lib/layout'
import ErrorBanner from './ErrorBanner.vue'

// ResetDialog is the reset mode picker (soft/mixed/hard/keep). Every mode
// but hard runs immediately; hard asks for confirmation, naming how many
// uncommitted changes would be lost. A successful reset closes the dialog
// and leaves a notice naming the previous commit, so the user can still
// copy its hash or reset back to it.
const props = defineProps<{ wt: string; branch: string; sha: string; subject: string }>()
const emit = defineEmits<{ close: [] }>()
const { t } = useI18n()

const modes = computed(() => [
  {
    value: ResetMode.ResetSoft,
    label: t('dialogs.reset.soft'),
    text: t('dialogs.reset.softText'),
  },
  {
    value: ResetMode.ResetMixed,
    label: t('dialogs.reset.mixed'),
    text: t('dialogs.reset.mixedText'),
  },
  {
    value: ResetMode.ResetHard,
    label: t('dialogs.reset.hard'),
    text: t('dialogs.reset.hardText'),
  },
  {
    value: ResetMode.ResetKeep,
    label: t('dialogs.reset.keep'),
    text: t('dialogs.reset.keepText'),
  },
])

const mode = ref<ResetMode>(ResetMode.ResetMixed) // git's own default
const uncommittedCount = ref<number | null>(null) // null while loading
const { error, run } = useCoreCall()
const { confirmAction } = useConfirmAction()
const notices = useNotices()

const branchLabel = computed(() => props.branch || 'HEAD')

onMounted(async () => {
  uncommittedCount.value = (await run(() => Facade.UncommittedCount(props.wt))) ?? null
})

async function doReset(target: ResetMode): Promise<void> {
  const result = await run(() => Facade.Reset(props.wt, props.sha, target))
  if (!result) return
  notices.show('reset', {
    severity: 'success',
    summary: t('dialogs.reset.resetNotice', {
      branch: branchLabel.value,
      sha: shortSha(props.sha),
      previousSha: shortSha(result.previousHead),
    }),
    data: { copyText: result.previousHead },
  })
  emit('close')
}

function onResetClick(): void {
  if (mode.value !== ResetMode.ResetHard) {
    void doReset(mode.value)
    return
  }
  const count = uncommittedCount.value ?? 0
  confirmAction(
    t('dialogs.reset.hardConfirmTitle'),
    t(
      'dialogs.reset.hardConfirm',
      { n: count, branch: branchLabel.value, subject: props.subject },
      count,
    ),
    () => void doReset(ResetMode.ResetHard),
  )
}
</script>

<template>
  <Dialog
    visible
    v-bind="DIALOG"
    :header="t('dialogs.reset.header')"
    @update:visible="emit('close')"
  >
    <ErrorBanner v-if="error" :error="error" />
    <p>
      {{ t('dialogs.reset.resetTo', { branch: branchLabel }) }}
      <span class="font-mono">{{ shortSha(sha) }}</span
      >{{ ' ' }}<span :title="subject">{{ subject }}</span>
    </p>
    <div class="flex flex-col gap-3">
      <label v-for="m in modes" :key="m.value" class="reset-mode flex items-baseline gap-2">
        <RadioButton
          v-model="mode"
          :value="m.value"
          name="reset-mode"
          :input-id="`reset-mode-${m.value}`"
        />
        <span
          ><strong>{{ m.label }}</strong> {{ m.text }}</span
        >
      </label>
    </div>
    <template #footer>
      <Button
        :label="t('common.cancel')"
        severity="secondary"
        data-test="dialog-cancel"
        @click="emit('close')"
      />
      <Button
        :label="t('dialogs.reset.reset')"
        data-test="reset-submit"
        :disabled="mode === ResetMode.ResetHard && uncommittedCount === null"
        @click="onResetClick"
      />
    </template>
  </Dialog>
</template>
