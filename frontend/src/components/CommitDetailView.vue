<script setup lang="ts">
import { computed } from 'vue'
import Splitter from 'openvue/splitter'
import SplitterPanel from 'openvue/splitterpanel'
import { useI18n } from '@/i18n'
import { useLogStore } from '@/stores/log'
import { useCompareStore } from '@/stores/compare'
import { useCoreCall } from '@/composables/useCoreCall'
import { useOpenInEditor } from '@/composables/useOpenInEditor'
import { AppService } from '../../bindings/github.com/brogli/gaertli/internal/app'
import { DiffKind, fileTarget } from '@/lib/diffTarget'
import { exactDate, shortSha, timeAgo } from '@/lib/format'
import type { ChangedFile } from '../../bindings/github.com/brogli/gaertli/internal/core'
import ChangedFiles from './ChangedFiles.vue'
import ErrorBanner from './ErrorBanner.vue'
import CompareView from './CompareView.vue'
import CompareWorkingView from './CompareWorkingView.vue'

// CommitDetailView is the commit detail view. It shows the selected commit
// in two parts split by a movable divider: message, committer info and
// parents above, changed files below. In its place, when one is active, it
// shows the comparison or the working-tree comparison instead, and with
// three or more picked commits only their count. Empty until a row is
// selected. A file opens in the diff window.
const props = defineProps<{ wt: string }>()
const log = useLogStore()
const compare = useCompareStore()
const { error, run } = useCoreCall()
const { openEditor } = useOpenInEditor(() => props.wt, run)
const { t } = useI18n()

function openDiff(f: ChangedFile): void {
  if (!log.details) return
  void AppService.OpenDiff(
    props.wt,
    fileTarget(f, { kind: DiffKind.CommitDiff, sha: log.details.sha }),
  )
}

const pickedCount = computed(() => log.selection.length)

// subject is the message's first line; restBody is the rest, its leading
// blank lines dropped, shown separately below it.
const subject = computed(() => log.details?.body.split('\n')[0] ?? '')
const restBody = computed(
  () => log.details?.body.slice(subject.value.length).replace(/^\n+/, '') ?? '',
)

// committerDiffers: the committer gets its own field only when it differs
// from the author.
const committerDiffers = computed(() => {
  const d = log.details
  if (!d) return false
  return (
    d.committer !== d.author || d.committerEmail !== d.authorEmail || d.committerAt !== d.authorAt
  )
})
</script>

<template>
  <!-- Each branch fills the view (a flex column's rest, or a splitter
       panel). -->
  <CompareView v-if="compare.active" class="flex-1 min-h-0 h-full" :wt="wt" />
  <CompareWorkingView v-else-if="compare.workingCommit" class="flex-1 min-h-0 h-full" :wt="wt" />
  <p v-else-if="pickedCount >= 3" class="flex-1 min-h-0 h-full px-2.5 py-1.5">
    {{ t('commitDetailView.selectedCount', { n: pickedCount }, pickedCount) }}
  </p>
  <Splitter
    v-else-if="log.details"
    layout="vertical"
    class="commit-details flex-1 min-h-0 h-full"
    state-key="gaertli.layout.commitDetail"
    state-storage="local"
  >
    <SplitterPanel
      :size="50"
      :min-size="10"
      class="commit-message flex flex-col gap-1.5 overflow-y-auto px-2.5 py-1.5"
    >
      <ErrorBanner v-if="error" :error="error" />
      <p class="subject font-semibold break-words">{{ subject }}</p>
      <pre v-if="restBody" class="whitespace-pre-wrap font-sans">{{ restBody }}</pre>
      <dl class="fields grid grid-cols-[auto_1fr] gap-x-2.5 gap-y-0.5">
        <dt class="text-muted-color">{{ t('commitDetailView.commit') }}</dt>
        <dd class="truncate" :title="log.details.sha">{{ shortSha(log.details.sha) }}</dd>
        <dt class="text-muted-color">{{ t('commitDetailView.author') }}</dt>
        <dd class="truncate" :title="exactDate(log.details.authorAt)">
          {{ log.details.author }} &lt;{{ log.details.authorEmail }}&gt; ·
          {{ timeAgo(log.details.authorAt) }}
        </dd>
        <template v-if="committerDiffers">
          <dt class="text-muted-color">{{ t('commitDetailView.committer') }}</dt>
          <dd class="truncate" :title="exactDate(log.details.committerAt)">
            {{ log.details.committer }} &lt;{{ log.details.committerEmail }}&gt; ·
            {{ timeAgo(log.details.committerAt) }}
          </dd>
        </template>
        <template v-if="log.details.parents?.length">
          <dt class="text-muted-color">{{ t('commitDetailView.parents') }}</dt>
          <dd class="parents flex flex-wrap gap-2 whitespace-normal">
            <span v-for="parent in log.details.parents" :key="parent" :title="parent">{{
              shortSha(parent)
            }}</span>
          </dd>
        </template>
      </dl>
    </SplitterPanel>
    <SplitterPanel :size="50" :min-size="10" class="flex flex-col gap-1.5 px-2.5 py-1.5">
      <ChangedFiles
        :files="log.details.files ?? []"
        :selected="log.selectedFile"
        @select="log.selectFile"
        @open="openDiff"
        @open-editor="openEditor"
      />
    </SplitterPanel>
  </Splitter>
</template>
