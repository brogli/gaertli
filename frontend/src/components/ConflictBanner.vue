<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '../i18n'
import { useRepoStore } from '../stores/repo'
import { useChangesStore } from '../stores/changes'
import { useConfirmAction } from '../composables/useConfirmAction'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import ErrorBanner from './ErrorBanner.vue'

// ConflictBanner shows while a merge, rebase, cherry-pick, or revert is
// stopped on conflicts: Continue (once no conflicted file remains), Skip
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
const skippable = computed(
  () => repo.summary?.operation === 'cherry-pick' || repo.summary?.operation === 'revert',
)

function onContinue(): void {
  void run(() => Facade.ContinueOperation(props.wt))
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
  <div v-if="repo.summary?.operation" class="conflict-banner" data-test="conflict-banner">
    <span>{{ t('notices.conflictStopped', { operation: repo.summary.operation }) }}</span>
    <ErrorBanner v-if="error" :error="error" />
    <button type="button" :disabled="unresolvedCount > 0" @click="onContinue">
      {{ t('notices.continue') }}
    </button>
    <button v-if="skippable" type="button" data-test="skip-operation" @click="onSkip">
      {{ t('notices.skip') }}
    </button>
    <button type="button" @click="onAbort">{{ t('notices.abortEllipsis') }}</button>
  </div>
</template>
