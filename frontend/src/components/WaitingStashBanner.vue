<script setup lang="ts">
import { computed } from 'vue'
import Button from 'openvue/button'
import Message from 'openvue/message'
import { useI18n } from '@/i18n'
import { useWorktreesStore } from '@/stores/worktrees'
import { useChangesStore } from '@/stores/changes'
import { useConfirmAction } from '@/composables/useConfirmAction'
import { useCoreCall } from '@/composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'
import { OperationKind } from '../../bindings/github.com/brogli/gaertli/internal/core'
import ErrorBanner from './ErrorBanner.vue'

// WaitingStashBanner offers to restore or drop the stash a pull, merge or rebase with stash made
// (Summary.WaitingStash) once no conflicted file remains and no operation is in progress: a stash
// restored into an open merge would land in the merge commit.
const props = defineProps<{ wt: string }>()
const repo = useWorktreesStore()
const changes = useChangesStore()
const { error, run } = useCoreCall()
const { confirmAction } = useConfirmAction()
const { t } = useI18n()

const stashRef = computed(() => repo.summary?.waitingStash ?? '')
const visible = computed(
  () =>
    !!stashRef.value &&
    !changes.files?.conflicted?.length &&
    repo.summary?.operation === OperationKind.$zero,
)

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
    <Message severity="info" data-test="waiting-stash-banner">
      <div class="flex flex-wrap items-center gap-2">
        <span>{{ t('notices.stashedHint') }}</span>
        <Button :label="t('notices.restore')" size="small" @click="onRestore" />
        <Button :label="t('notices.drop')" size="small" @click="onDrop" />
      </div>
    </Message>
    <ErrorBanner v-if="error" :error="error" />
  </template>
</template>
