<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '../i18n'
import { useRepoStore } from '../stores/repo'
import { useChangesStore } from '../stores/changes'
import { useConfirmAction } from '../composables/useConfirmAction'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import ErrorBanner from './ErrorBanner.vue'

// StashBanner offers to restore or drop the stash PullWithStash made
// (Summary.PullStash) once no conflicted file remains.
const props = defineProps<{ wt: string }>()
const repo = useRepoStore()
const changes = useChangesStore()
const { error, run } = useCoreCall()
const { confirmAction } = useConfirmAction()
const { t } = useI18n()

const stashRef = computed(() => repo.summary?.pullStash ?? '')
const visible = computed(() => !!stashRef.value && !changes.files?.conflicted?.length)

function onRestore(): void {
  void run(() => Facade.PopStash(props.wt, stashRef.value))
}

function onDrop(): void {
  confirmAction(t('notices.dropStashTitle'), t('notices.dropStashConfirm'), () => {
    void run(() => Facade.DropStash(props.wt, stashRef.value))
  })
}
</script>

<template>
  <div v-if="visible" data-test="stash-banner">
    <span>{{ t('notices.stashedHint') }}</span>
    <ErrorBanner v-if="error" :error="error" />
    <button type="button" @click="onRestore">{{ t('notices.restore') }}</button>
    <button type="button" @click="onDrop">{{ t('notices.drop') }}</button>
  </div>
</template>
