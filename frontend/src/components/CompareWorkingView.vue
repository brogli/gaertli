<script setup lang="ts">
import { watch } from 'vue'
import Button from 'openvue/button'
import { useI18n } from '@/i18n'
import { useCompareStore } from '@/stores/compare'
import { useCoreCall } from '@/composables/useCoreCall'
import { useOpenInEditor } from '@/composables/useOpenInEditor'
import { useShortcut } from '@/composables/useShortcut'
import { AppService } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { DiffKind, fileTarget } from '@/lib/diffTarget'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { iconClass } from '@/lib/icons'
import { ICON_BUTTON } from '@/lib/layout'
import ChangedFiles from './ChangedFiles.vue'
import ErrorBanner from './ErrorBanner.vue'

// CompareWorkingView shows the compare store's workingCommit and the files
// that differ from the working tree; no swap. A click selects a file, a
// double-click opens it in the diff window. git diff <sha> never lists
// untracked files, so the template says they are missing.
const props = defineProps<{ wt: string }>()
const compare = useCompareStore()
const { error, run } = useCoreCall()
const { openEditor } = useOpenInEditor(() => props.wt, run)
const { t } = useI18n()

function openDiff(f: ChangedFile): void {
  if (!compare.workingCommit) return
  void AppService.OpenDiff(
    props.wt,
    fileTarget(f, { kind: DiffKind.AgainstWorkingDiff, sha: compare.workingCommit.sha }),
  )
}

watch(
  () => compare.workingCommit,
  (commit) => {
    if (commit) void run(() => compare.loadAgainstWorking(props.wt, commit.sha))
  },
  { immediate: true },
)

useShortcut('Escape', () => compare.exit(), { inTextField: false })
</script>

<template>
  <section class="compare-working-view flex flex-col gap-1.5 overflow-y-auto px-2.5 py-1.5">
    <header>
      <!-- The from box, in the colour of its mark in the log view. -->
      <span
        class="from px-1.5 border border-current rounded-sm font-mono text-from"
        data-test="from-tag"
        :title="compare.workingCommit?.sha"
        >{{ compare.workingCommit?.shortSha }}</span
      >
      {{ compare.workingCommit?.subject }}
      &rarr; {{ t('compareView.toWorkingTree') }}
      <Button
        v-bind="ICON_BUTTON"
        :icon="iconClass('close')"
        data-test="exit"
        :title="
          t('shortcuts.withShortcut', {
            action: t('compareView.close'),
            shortcut: t('shortcuts.stopComparing'),
          })
        "
        :aria-label="t('compareView.close')"
        @click="compare.exit"
      />
    </header>
    <p class="my-1 text-muted-color text-sm">{{ t('compareView.untrackedHint') }}</p>
    <ErrorBanner v-if="error" :error="error" />
    <ChangedFiles
      :files="compare.files"
      :selected="compare.selectedFile"
      @select="compare.selectFile"
      @open="openDiff"
      @open-editor="openEditor"
    />
  </section>
</template>
