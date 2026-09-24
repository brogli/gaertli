<script setup lang="ts">
import { watch } from 'vue'
import { useI18n } from '../i18n'
import { useCompareStore } from '../stores/compare'
import { onKeyStroke } from '@vueuse/core'
import { useCoreCall } from '../composables/useCoreCall'
import { useOpenInEditor } from '../composables/useOpenInEditor'
import { AppService } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { DiffKind } from '../lib/diffTarget'
import { iconClass } from '../lib/icons'
import ChangedFileList from './ChangedFileList.vue'
import ErrorBanner from './ErrorBanner.vue'
import type { ChangedFile, LogCommit } from '../../bindings/github.com/brogli/nebegit/internal/core'

// CompareView shows the compare store's two selected commits, their
// direction and a swap action, and their changed files. Escape leaves the
// comparison; a click selects a file, a double-click opens it in the diff
// window.
const props = defineProps<{ wt: string }>()
const compare = useCompareStore()
const { error, run } = useCoreCall()
const { openEditor } = useOpenInEditor(() => props.wt, run)
const { t } = useI18n()

function openDiffWindow(f: ChangedFile): void {
  if (!compare.from || !compare.to) return
  void AppService.OpenDiff(props.wt, {
    kind: DiffKind.RangeDiff,
    from: compare.from.sha,
    to: compare.to.sha,
    path: f.path,
    oldPath: f.oldPath ?? '',
  })
}

watch(
  [() => compare.from, () => compare.to],
  () => {
    if (compare.active) void run(() => compare.load(props.wt))
  },
  { immediate: true },
)

onKeyStroke('Escape', () => compare.exit())

// sideLabel names a compared commit's fixed identity (older/newer by log
// index), independent of which side (from/to) Swap currently puts it on.
function sideLabel(commit: LogCommit | null): 'older' | 'newer' {
  return commit && commit.sha === compare.older?.sha ? 'older' : 'newer'
}

function sideLabelText(commit: LogCommit | null): string {
  return sideLabel(commit) === 'older' ? t('compareView.older') : t('compareView.newer')
}
</script>

<template>
  <section class="compare-view">
    <header>
      <span class="side-tag" :class="`tag-${sideLabel(compare.from)}`" data-test="from-tag">{{
        sideLabelText(compare.from)
      }}</span>
      <span class="from">{{ ' ' + (compare.from?.shortSha ?? '') }}</span>
      &rarr;
      <span class="side-tag" :class="`tag-${sideLabel(compare.to)}`" data-test="to-tag">{{
        sideLabelText(compare.to)
      }}</span>
      <span class="to">{{ ' ' + (compare.to?.shortSha ?? '') }}</span>
      <button type="button" data-test="swap" @click="compare.swap">
        <i :class="iconClass('swap')" aria-hidden="true"></i> {{ t('compareView.swap') }}
      </button>
      <button
        type="button"
        class="glyph-btn"
        data-test="exit"
        :title="t('compareView.stopComparing')"
        :aria-label="t('compareView.stopComparing')"
        @click="compare.exit"
      >
        <i :class="iconClass('close')" aria-hidden="true"></i>
      </button>
    </header>
    <ErrorBanner v-if="error" :error="error" />
    <ChangedFileList
      :files="compare.files"
      :selected="compare.selectedFile"
      @select="compare.selectFile"
      @open="openDiffWindow"
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
.side-tag {
  padding: 0 0.4em;
  border: 1px solid currentColor;
  border-radius: 3px;
  font-size: 0.85em;
}
.tag-older {
  color: var(--nb-older);
}
.tag-newer {
  color: var(--nb-newer);
}
</style>
