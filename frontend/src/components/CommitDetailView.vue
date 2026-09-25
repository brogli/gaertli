<script setup lang="ts">
import { computed } from 'vue'
import Splitter from 'openvue/splitter'
import SplitterPanel from 'openvue/splitterpanel'
import { useI18n } from '../i18n'
import { useLogStore } from '../stores/log'
import { useCompareStore } from '../stores/compare'
import { useCoreCall } from '../composables/useCoreCall'
import { useOpenInEditor } from '../composables/useOpenInEditor'
import { AppService } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { DiffKind, fileTarget } from '../lib/diffTarget'
import { shortSha, timeAgo } from '../lib/format'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
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

function absoluteDate(unixSeconds: number): string {
  return new Date(unixSeconds * 1000).toLocaleString()
}

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
  <CompareView v-if="compare.active" class="pane" :wt="wt" />
  <CompareWorkingView v-else-if="compare.workingCommit" class="pane" :wt="wt" />
  <p v-else-if="pickedCount >= 3" class="picked-count pane">
    {{ t('commitDetailView.selectedCount', { n: pickedCount }, pickedCount) }}
  </p>
  <Splitter
    v-else-if="log.details"
    layout="vertical"
    class="commit-details"
    state-key="nebegit.layout.commitDetail"
    state-storage="local"
  >
    <SplitterPanel :size="50" :min-size="10" class="split-part commit-message">
      <ErrorBanner v-if="error" :error="error" />
      <p class="subject">{{ subject }}</p>
      <pre v-if="restBody" class="body">{{ restBody }}</pre>
      <dl class="fields">
        <dt>{{ t('commitDetailView.commit') }}</dt>
        <dd :title="log.details.sha">{{ shortSha(log.details.sha) }}</dd>
        <dt>{{ t('commitDetailView.author') }}</dt>
        <dd :title="absoluteDate(log.details.authorAt)">
          {{ log.details.author }} &lt;{{ log.details.authorEmail }}&gt; ·
          {{ timeAgo(log.details.authorAt) }}
        </dd>
        <template v-if="committerDiffers">
          <dt>{{ t('commitDetailView.committer') }}</dt>
          <dd :title="absoluteDate(log.details.committerAt)">
            {{ log.details.committer }} &lt;{{ log.details.committerEmail }}&gt; ·
            {{ timeAgo(log.details.committerAt) }}
          </dd>
        </template>
        <template v-if="log.details.parents?.length">
          <dt>{{ t('commitDetailView.parents') }}</dt>
          <dd class="parents">
            <span v-for="parent in log.details.parents" :key="parent" :title="parent">{{
              shortSha(parent)
            }}</span>
          </dd>
        </template>
      </dl>
    </SplitterPanel>
    <SplitterPanel :size="50" :min-size="10" class="split-part">
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

<style scoped>
.picked-count {
  margin: 0;
}
/* Each content fills the view: the rest of a flex column (medium, narrow)
   or all of a splitter panel (wide). A commit's two parts, like the other
   contents, are inset from their edges; the message part scrolls, the
   files part leaves that to its file region. */
.pane,
.commit-details {
  flex: 1 1 auto;
  height: 100%;
  min-height: 0;
}
.pane,
.split-part {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  padding: 0.4rem 0.6rem;
  box-sizing: border-box;
}
.pane,
.commit-message {
  overflow-y: auto;
}
.subject {
  margin: 0;
  font-weight: 600;
  overflow-wrap: break-word;
}
.body {
  margin: 0;
  font-family: inherit;
  white-space: pre-wrap;
}
.fields {
  display: grid;
  grid-template-columns: auto 1fr;
  gap: 0.15rem 0.6rem;
  margin: 0;
}
.fields dt {
  color: var(--p-text-muted-color);
}
.fields dd {
  margin: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.parents {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  white-space: normal;
}
</style>
