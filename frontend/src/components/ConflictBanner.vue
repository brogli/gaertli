<script setup lang="ts">
import { computed } from 'vue'
import Button from 'openvue/button'
import Message from 'openvue/message'
import { useI18n } from '@/i18n'
import { useRepoStore } from '@/stores/repo'
import { useChangesStore } from '@/stores/changes'
import { useConfirmAction } from '@/composables/useConfirmAction'
import { useCoreCall } from '@/composables/useCoreCall'
import { openConflicts } from '@/lib/openConflicts'
import { Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'
import { Operation } from '../../bindings/github.com/brogli/gaertli/internal/core'
import ErrorBanner from './ErrorBanner.vue'

// ConflictBanner shows while a merge, rebase, cherry-pick, or revert is
// stopped on conflicts: Resolve conflicts (opens the diff window while a
// conflicted file remains), Continue (once no conflicted file remains), Skip
// (cherry-pick and revert only: merge has no --skip, rebase's is not
// supported), and Abort. The repo watcher keeps the stores it reads
// current, so it refreshes nothing itself.
const props = defineProps<{ wt: string }>()
const repo = useRepoStore()
const changes = useChangesStore()
const { error, run } = useCoreCall()
const { confirmAction } = useConfirmAction()
const { t } = useI18n()

const unresolvedCount = computed(() => changes.files?.conflicted?.length ?? 0)
const skippable = computed(() => {
  const operation = repo.summary?.operation
  return operation === Operation.CherryPickOperation || operation === Operation.RevertOperation
})

function onContinue(): void {
  void run(() => Facade.ContinueOperation(props.wt))
}

function onResolve(): void {
  void run(() => openConflicts(props.wt))
}

function onSkip(): void {
  void run(() => Facade.SkipOperation(props.wt))
}

function onAbort(): void {
  confirmAction(
    t('notices.abort'),
    t('notices.abortConfirm', { operation: repo.summary?.operation ?? '' }),
    () => void run(() => Facade.AbortOperation(props.wt)),
  )
}
</script>

<template>
  <template v-if="repo.summary?.operation">
    <Message severity="warn" data-test="conflict-banner">
      <div class="flex flex-wrap items-center gap-2">
        <span>{{ t('notices.conflictStopped', { operation: repo.summary.operation }) }}</span>
        <Button
          v-if="unresolvedCount > 0"
          :label="t('notices.resolveConflicts')"
          size="small"
          @click="onResolve"
        />
        <Button
          :label="t('notices.continue')"
          size="small"
          :disabled="unresolvedCount > 0"
          @click="onContinue"
        />
        <Button
          v-if="skippable"
          :label="t('notices.skip')"
          size="small"
          data-test="skip-operation"
          @click="onSkip"
        />
        <Button :label="t('notices.abortEllipsis')" size="small" @click="onAbort" />
      </div>
    </Message>
    <ErrorBanner v-if="error" :error="error" />
  </template>
</template>
