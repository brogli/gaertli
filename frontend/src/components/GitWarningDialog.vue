<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import Dialog from 'openvue/dialog'
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import { AppService, type GitWarning } from '../../bindings/github.com/brogli/gaertli/internal/app'
import { useRepoStore } from '@/stores/repo'
import { DIALOG } from '@/lib/layout'

// GitWarningDialog warns about outdated gits. The shell hands each git's
// warning to the first window that asks: on load, and once the window's
// repo has loaded, since a WSL distro's git is checked on its first use.
const { t } = useI18n()
const repo = useRepoStore()
const warnings = ref<GitWarning[]>([])
const downloads = 'https://git-scm.com/downloads'

async function take(): Promise<void> {
  warnings.value.push(...((await AppService.TakeGitWarnings()) ?? []))
}

onMounted(take)
const stop = watch(
  () => repo.summary,
  () => {
    stop()
    void take()
  },
)

function gitName(warning: GitWarning): string {
  return warning.distro
    ? t('dialogs.gitWarning.distroGit', { distro: warning.distro })
    : t('dialogs.gitWarning.installedGit')
}
</script>

<template>
  <Dialog
    v-if="warnings.length"
    visible
    v-bind="DIALOG"
    :header="t('dialogs.gitWarning.header')"
    @update:visible="warnings = []"
  >
    <p v-for="warning in warnings" :key="warning.distro">
      {{
        t('dialogs.gitWarning.message', {
          latest: warning.latest,
          git: gitName(warning),
          version: warning.version || t('dialogs.gitWarning.unknownVersion'),
        })
      }}
    </p>
    <p>
      {{ t('dialogs.gitWarning.update') }}
      <a :href="downloads" target="_blank" rel="noreferrer" class="underline">{{ downloads }}</a>
    </p>
    <template #footer>
      <Button :label="t('common.ok')" data-test="git-warning-ok" @click="warnings = []" />
    </template>
  </Dialog>
</template>
