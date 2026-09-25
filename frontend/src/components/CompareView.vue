<script setup lang="ts">
import { watch } from 'vue'
import Button from 'openvue/button'
import { useI18n } from '../i18n'
import { useCompareStore } from '../stores/compare'
import { onKeyStroke } from '@vueuse/core'
import { useCoreCall } from '../composables/useCoreCall'
import { useOpenInEditor } from '../composables/useOpenInEditor'
import { AppService } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { DiffKind, fileTarget } from '../lib/diffTarget'
import { iconClass } from '../lib/icons'
import { ICON_BUTTON, SECONDARY_BUTTON } from '../lib/layout'
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

onKeyStroke('Escape', () => compare.exit())
</script>

<template>
  <section class="compare-view">
    <header>
      <span class="side-tag from" data-test="from-tag" :title="compare.from?.sha">{{
        compare.from?.shortSha
      }}</span>
      &rarr;
      <span class="side-tag to" data-test="to-tag" :title="compare.to?.sha">{{
        compare.to?.shortSha
      }}</span>
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
        :title="t('compareView.stopComparing')"
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

<style scoped>
header {
  display: flex;
  align-items: center;
  gap: 0.4em;
}
/* Each side's hash in its box, in the colour of its mark in the log view. */
.side-tag {
  padding: 0 0.4em;
  border: 1px solid currentColor;
  border-radius: 3px;
  font-family: var(--nb-font-mono);
}
.from {
  color: var(--nb-from);
}
.to {
  color: var(--nb-to);
}
</style>
