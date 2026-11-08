<script setup lang="ts">
import { computed, ref } from 'vue'
import Dialog from 'openvue/dialog'
import Textarea from 'openvue/textarea'
import Message from 'openvue/message'
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import { useCoreCall, type CoreError } from '@/composables/useCoreCall'
import { useWorktreesStore } from '@/stores/worktrees'
import { useSettingsStore } from '@/stores/settings'
import { Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'
import { Code, type SquashCheck } from '../../bindings/github.com/brogli/gaertli/internal/core'
import { shortSha } from '@/lib/format'
import { DIALOG } from '@/lib/layout'
import { openOnConflicts } from '@/lib/openConflicts'
import { errorNotice, useNotices } from '@/lib/notice'
import ErrorBanner from './ErrorBanner.vue'
import FormField from './FormField.vue'

// SquashDialog asks before shas are folded into one commit: squash with the
// combined message, edited here, or fixup keeping the oldest's. It warns
// when a folded commit is already pushed. A stop closes it: the conflict
// banner takes over.
const props = defineProps<{ wt: string; shas: string[]; check: SquashCheck; fixup: boolean }>()
const emit = defineEmits<{ close: []; done: [] }>()
const { t } = useI18n()
const repo = useWorktreesStore()
const settings = useSettingsStore()
const notices = useNotices()
const { error, run } = useCoreCall()

const message = ref(props.check.message)
const n = props.shas.length
const branch = computed(() => repo.summary?.branch ?? '')

const pushedWarning = computed(() => {
  if (!props.check.pushed) return ''
  const onProtected = (settings.settings?.protectedBranches ?? []).includes(branch.value)
  const warning = t('dialogs.squash.pushedWarning')
  return onProtected
    ? `${warning} ${t('dialogs.squash.protectedWarning', { branch: branch.value })}`
    : warning
})

// leftForBanner handles a stop: the rebase waits in the operation banner and
// a stop on conflicts opens the diff window; a stop without conflicts names
// git's reason in a notice.
function leftForBanner(e: CoreError): boolean {
  if (e.code === Code.Stopped) notices.show('squash', errorNotice(e))
  return e.code === Code.Stopped || openOnConflicts(props.wt)(e)
}

async function submit(): Promise<void> {
  const result = await run(
    () =>
      props.fixup
        ? Facade.Fixup(props.wt, props.shas)
        : Facade.Squash(props.wt, props.shas, message.value),
    leftForBanner,
  )
  if (error.value) return
  if (result) {
    const key = props.fixup ? 'dialogs.squash.fixedUpNotice' : 'dialogs.squash.squashedNotice'
    notices.show('squash', {
      severity: 'success',
      summary: t(key, { n, branch: branch.value, previousSha: shortSha(result.previousHead) }),
      data: { copyText: result.previousHead },
    })
  }
  emit('done')
}
</script>

<template>
  <Dialog
    visible
    v-bind="DIALOG"
    :header="
      fixup ? t('dialogs.squash.fixupHeader', { n }, n) : t('dialogs.squash.squashHeader', { n }, n)
    "
    @update:visible="emit('close')"
  >
    <ErrorBanner v-if="error" :error="error" />
    <p v-if="fixup">{{ t('dialogs.squash.fixupText') }}</p>
    <FormField v-else v-slot="{ id }" :label="t('dialogs.squash.message')">
      <Textarea :id="id" v-model="message" rows="8" fluid data-test="squash-message" />
    </FormField>
    <Message v-if="pushedWarning" severity="warn">{{ pushedWarning }}</Message>
    <template #footer>
      <Button
        :label="t('common.cancel')"
        severity="secondary"
        data-test="dialog-cancel"
        @click="emit('close')"
      />
      <Button
        :label="fixup ? t('dialogs.squash.fixup') : t('dialogs.squash.squash')"
        :disabled="!fixup && !message.trim()"
        data-test="squash-submit"
        @click="submit"
      />
    </template>
  </Dialog>
</template>
