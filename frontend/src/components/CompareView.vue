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
import { iconClass } from '@/lib/icons'
import { ICON_BUTTON, SECONDARY_BUTTON } from '@/lib/layout'
import ChangedFiles from './ChangedFiles.vue'
import ErrorBanner from './ErrorBanner.vue'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'

// CompareView shows the compare store's two selected commits, their
// direction and a swap action, and their changed files. Escape leaves the
// comparison; a click selects a file, a double-click opens it in the diff
// window.
const props = defineProps<{ wt: string }>()
const compare = useCompareStore()
const { error, run } = useCoreCall()
const { openEditor } = useOpenInEditor(() => props.wt, run)
const { t } = useI18n()

function openDiff(f: ChangedFile): void {
  if (!compare.from || !compare.to) return
  void AppService.OpenDiff(
    props.wt,
    fileTarget(f, { kind: DiffKind.RangeDiff, from: compare.from.sha, to: compare.to.sha }),
  )
}

watch(
  [() => compare.from, () => compare.to],
  () => {
    if (compare.active) void run(() => compare.load(props.wt))
  },
  { immediate: true },
)

useShortcut('Escape', () => compare.exit(), { inTextField: false })
</script>

<template>
  <section class="compare-view flex flex-col gap-1.5 overflow-y-auto px-2.5 py-1.5">
    <header class="flex items-center gap-1.5">
      <!-- Each side's hash in its box, in the colour of its mark in the log view. -->
      <span
        class="from px-1.5 border border-current rounded-sm font-mono text-from"
        data-test="from-tag"
        :title="compare.from?.sha"
        >{{ compare.from?.shortSha }}</span
      >
      &rarr;
      <span
        class="to px-1.5 border border-current rounded-sm font-mono text-to"
        data-test="to-tag"
        :title="compare.to?.sha"
        >{{ compare.to?.shortSha }}</span
      >
      <Button
        v-bind="SECONDARY_BUTTON"
        :icon="iconClass('swap')"
        :label="t('compareView.swap')"
        data-test="swap"
        @click="compare.swap"
      />
      <Button
        v-bind="ICON_BUTTON"
        :icon="iconClass('close')"
        data-test="exit"
        :title="
          t('shortcuts.withShortcut', {
            action: t('compareView.stopComparing'),
            shortcut: t('shortcuts.stopComparing'),
          })
        "
        :aria-label="t('compareView.stopComparing')"
        @click="compare.exit"
      />
    </header>
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
