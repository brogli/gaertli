<script setup lang="ts">
import { computed } from 'vue'
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

const unresolvedCount = computed(() => (changes.files?.conflicted ?? []).length)
const skippable = computed(() => repo.summary?.operation === 'cherry-pick' || repo.summary?.operation === 'revert')

async function onContinue(): Promise<void> {
  await run(() => Facade.ContinueOperation(props.wt))
}

async function onSkip(): Promise<void> {
  await run(() => Facade.SkipOperation(props.wt))
}

function onAbort(): void {
  confirmAction(
    'Abort',
    `Abort the ${repo.summary?.operation}? This discards its changes.`,
    () => void run(() => Facade.AbortOperation(props.wt)),
  )
}
</script>

<template>
  <div v-if="repo.summary?.operation" class="conflict-banner" data-test="conflict-banner">
    <span>{{ repo.summary.operation }} stopped on conflicts</span>
    <ErrorBanner v-if="error" :error="error" />
    <button type="button" :disabled="unresolvedCount > 0" @click="onContinue">Continue</button>
    <button v-if="skippable" type="button" data-test="skip-operation" @click="onSkip">Skip</button>
    <button type="button" @click="onAbort">Abort…</button>
  </div>
</template>
