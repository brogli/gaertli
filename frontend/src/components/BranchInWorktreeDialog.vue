<script setup lang="ts">
import { computed } from 'vue'
import Dialog from 'openvue/dialog'
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import { useCoreCall } from '@/composables/useCoreCall'
import { AppService } from '../../bindings/github.com/brogli/gaertli/internal/app'
import { BranchInUse } from '../../bindings/github.com/brogli/gaertli/internal/core'
import { DIALOG } from '@/lib/layout'
import { basename, repoPath } from '@/lib/path'
import ErrorBanner from './ErrorBanner.vue'

// BranchInWorktreeDialog offers to switch this window to the worktree that
// has branch checked out: git checks a branch out in one worktree only.
// path is as the branch's own worktree gives it (POSIX even for a WSL
// repo); distro turns it into this window's id. inUse names the rebase or
// bisect the other worktree has that branch's HEAD detached for, "" for a
// plain checkout.
const props = defineProps<{
  branch: string
  path: string
  inUse?: BranchInUse | undefined
  distro?: string | undefined
}>()
const emit = defineEmits<{ close: [] }>()

const { error, runAndClose } = useCoreCall()
const { t } = useI18n()

const header = computed(() =>
  t(props.inUse ? 'dialogs.branchInWorktree.headerInUse' : 'dialogs.branchInWorktree.header'),
)

const message = computed(() => {
  const args = { branch: props.branch, folder: basename(props.path) }
  if (props.inUse === BranchInUse.BranchRebasing) {
    return t('dialogs.branchInWorktree.messageRebasing', args)
  }
  if (props.inUse === BranchInUse.BranchBisecting) {
    return t('dialogs.branchInWorktree.messageBisecting', args)
  }
  return t('dialogs.branchInWorktree.message', args)
})

async function confirm(): Promise<void> {
  await runAndClose(
    () => AppService.SelectWorktree(repoPath(props.path, props.distro)),
    () => emit('close'),
  )
}
</script>

<template>
  <Dialog visible v-bind="DIALOG" :header="header" @update:visible="emit('close')">
    <ErrorBanner v-if="error" :error="error" />
    <p>{{ message }}</p>
    <p class="text-muted-color">{{ path }}</p>
    <template #footer>
      <Button
        :label="t('common.cancel')"
        severity="secondary"
        data-test="branch-in-worktree-cancel"
        @click="emit('close')"
      />
      <Button
        :label="t('dialogs.branchInWorktree.switch')"
        data-test="branch-in-worktree-confirm"
        @click="confirm"
      />
    </template>
  </Dialog>
</template>
