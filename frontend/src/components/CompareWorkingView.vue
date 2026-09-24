<script setup lang="ts">
import { watch } from 'vue'
import { onKeyStroke } from '@vueuse/core'
import { useI18n } from '../i18n'
import { useCompareStore } from '../stores/compare'
import { useCoreCall } from '../composables/useCoreCall'
import { useOpenInEditor } from '../composables/useOpenInEditor'
import { AppService } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { DiffKind } from '../lib/diffTarget'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { iconClass } from '../lib/icons'
import ChangedFileList from './ChangedFileList.vue'
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
  void AppService.OpenDiff(props.wt, {
    kind: DiffKind.AgainstWorkingDiff,
    sha: compare.workingCommit.sha,
    path: f.path,
    oldPath: f.oldPath ?? '',
  })
}

watch(
  () => compare.workingCommit,
  (commit) => {
    if (commit) void run(() => compare.loadAgainstWorking(props.wt, commit.sha))
  },
  { immediate: true },
)

onKeyStroke('Escape', () => compare.exit())
</script>

<template>
  <section class="compare-working-view">
    <header>
      <span class="from">{{ compare.workingCommit?.shortSha }}</span>
      {{ compare.workingCommit?.subject }}
      &rarr; {{ t('compareView.toWorkingTree') }}
      <button
        type="button"
        class="glyph-btn"
        data-test="exit"
        :title="t('compareView.close')"
        :aria-label="t('compareView.close')"
        @click="compare.exit"
      >
        <i :class="iconClass('close')" aria-hidden="true"></i>
      </button>
    </header>
    <p class="hint">{{ t('compareView.untrackedHint') }}</p>
    <ErrorBanner v-if="error" :error="error" />
    <ChangedFileList
      :files="compare.files"
      :selected="compare.selectedFile"
      @select="compare.selectFile"
      @open="openDiff"
      @open-editor="openEditor"
    />
  </section>
</template>

<style scoped>
.hint {
  margin: 0.25rem 0;
  color: var(--p-text-muted-color);
  font-size: 0.85em;
}
</style>
