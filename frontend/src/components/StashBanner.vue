<script setup lang="ts">
import { computed } from 'vue'
import Button from 'openvue/button'
import Message from 'openvue/message'
import { useI18n } from '@/i18n'
import { useRepoStore } from '@/stores/repo'
import { useChangesStore } from '@/stores/changes'
import { useConfirmAction } from '@/composables/useConfirmAction'
import { useCoreCall } from '@/composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import ErrorBanner from './ErrorBanner.vue'

// StashBanner offers to restore or drop the stash a pull, merge or rebase with stash made
// (Summary.WaitingStash) once no conflicted file remains.
const props = defineProps<{ wt: string }>()
const repo = useRepoStore()
const changes = useChangesStore()
const { error, run } = useCoreCall()
const { confirmAction } = useConfirmAction()
const { t } = useI18n()

const stashRef = computed(() => repo.summary?.waitingStash ?? '')
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
  <template v-if="visible">
    <Message severity="info" data-test="stash-banner">
      <div class="flex flex-wrap items-center gap-2">
        <span>{{ t('notices.stashedHint') }}</span>
        <Button :label="t('notices.restore')" size="small" @click="onRestore" />
        <Button :label="t('notices.drop')" size="small" @click="onDrop" />
      </div>
    </Message>
    <ErrorBanner v-if="error" :error="error" />
  </template>
</template>
