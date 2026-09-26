<script setup lang="ts">
import Dialog from 'openvue/dialog'
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import { useCoreCall } from '@/composables/useCoreCall'
import { AppService } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { DIALOG } from '@/lib/layout'
import { basename, repoPath } from '@/lib/path'
import ErrorBanner from './ErrorBanner.vue'

// BranchInWorktreeDialog offers to switch this window to the worktree that
// has branch checked out: git checks a branch out in one worktree only.
// path is as the branch's own worktree gives it (POSIX even for a WSL
// repo); distro turns it into this window's id.
const props = defineProps<{ branch: string; path: string; distro?: string | undefined }>()
const emit = defineEmits<{ close: [] }>()

const { error, runAndClose } = useCoreCall()
const { t } = useI18n()

async function confirm(): Promise<void> {
  await runAndClose(
    () => AppService.SelectWorktree(repoPath(props.path, props.distro)),
    () => emit('close'),
  )
}
</script>

<template>
  <Dialog
    visible
    v-bind="DIALOG"
    :header="t('dialogs.branchInWorktree.header')"
    @update:visible="emit('close')"
  >
    <ErrorBanner v-if="error" :error="error" />
    <p>{{ t('dialogs.branchInWorktree.message', { branch, folder: basename(path) }) }}</p>
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
