<script setup lang="ts">
import { computed } from 'vue'
import { useRepoStore } from '../stores/repo'
import { useChangesStore } from '../stores/changes'
import { useConfirmAction } from '../composables/useConfirmAction'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import ErrorBanner from './ErrorBanner.vue'

// StashNotice offers to restore or drop the stash PullWithStash made
// (Summary.PullStash) once no conflicted file remains.
const props = defineProps<{ wt: string }>()
const repo = useRepoStore()
const changes = useChangesStore()
const { error, run } = useCoreCall()
const { confirmAction } = useConfirmAction()

const stashRef = computed(() => repo.summary?.pullStash ?? '')
const visible = computed(() => !!stashRef.value && (changes.files?.conflicted ?? []).length === 0)

function onRestore(): void {
  void run(() => Facade.PopStash(props.wt, stashRef.value))
}

function onDrop(): void {
  confirmAction('Drop stashed changes', 'Drop the stashed changes? They will be lost.', () => {
    void run(() => Facade.DropStash(props.wt, stashRef.value))
  })
}
</script>

<template>
  <div v-if="visible" class="stash-notice" data-test="stash-notice">
    <span>Local changes from before a pull are stashed.</span>
    <ErrorBanner v-if="error" :error="error" />
    <button type="button" @click="onRestore">Restore</button>
    <button type="button" @click="onDrop">Drop…</button>
  </div>
</template>
