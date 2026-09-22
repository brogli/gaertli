<script setup lang="ts">
import { computed } from 'vue'
import { useRepoStore } from '../stores/repo'
import { useChangesStore } from '../stores/changes'
import { useConfirmAction } from '../composables/useConfirmAction'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import ErrorBanner from './ErrorBanner.vue'

// ConflictBanner shows above the changes column while a merge, rebase,
// cherry-pick, or revert is stopped on conflicts, naming the operation and
// offering Continue (disabled while any conflicted file remains) or Abort.
// Both read from stores already kept current by the repo watcher's
// index/HEAD events, the same way the changes panel's stage/unstage does
// — no explicit refresh here.
const props = defineProps<{ wt: string }>()
const repo = useRepoStore()
const changes = useChangesStore()
const { error, run } = useCoreCall()
const { confirmAction } = useConfirmAction()

const unresolvedCount = computed(() => (changes.files?.conflicted ?? []).length)

async function onContinue(): Promise<void> {
  await run(() => Facade.ContinueOperation(props.wt))
}

async function abort(): Promise<void> {
  await run(() => Facade.AbortOperation(props.wt))
}

function onAbort(): void {
  confirmAction(
    'Abort',
    `Abort the ${repo.summary?.operation}? This discards its own changes.`,
    () => void abort(),
  )
}
</script>

<template>
  <div v-if="repo.summary?.operation" class="conflict-banner" data-test="conflict-banner">
    <span>{{ repo.summary.operation }} stopped on conflicts</span>
    <ErrorBanner v-if="error" :error="error" />
    <button type="button" :disabled="unresolvedCount > 0" @click="onContinue">Continue</button>
    <button type="button" @click="onAbort">Abort…</button>
  </div>
</template>
