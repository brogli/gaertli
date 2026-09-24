<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from '../i18n'
import { useLogStore } from '../stores/log'
import { useCompareStore } from '../stores/compare'
import { useCoreCall } from '../composables/useCoreCall'
import { useOpenInEditor } from '../composables/useOpenInEditor'
import { AppService } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { DiffKind } from '../lib/diffTarget'
import { shortSha, timeAgo } from '../lib/format'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import ChangedFileList from './ChangedFileList.vue'
import ErrorBanner from './ErrorBanner.vue'
import CompareView from './CompareView.vue'
import CompareWorkingView from './CompareWorkingView.vue'

// CommitDetailView is the commit detail view. It shows the selected commit:
// message, committer info, parents, and changed files. In its place, when
// one is active, it shows the comparison or the working-tree comparison
// instead. Empty until a row is selected. A file opens in the diff window.
const props = defineProps<{ wt: string }>()
const log = useLogStore()
const compare = useCompareStore()
const { error, run } = useCoreCall()
const { openEditor } = useOpenInEditor(() => props.wt, run)
const { t } = useI18n()

function openDiff(f: ChangedFile): void {
  if (!log.details) return
  void AppService.OpenDiff(props.wt, {
    kind: DiffKind.CommitDiff,
    sha: log.details.sha,
    path: f.path,
    oldPath: f.oldPath ?? '',
  })
}

function absoluteDate(unixSeconds: number): string {
  return new Date(unixSeconds * 1000).toLocaleString()
}

// subject is the message's first line; restBody is whatever follows a
// blank line, shown separately below it.
const subject = computed(() => log.details?.body.split('\n')[0] ?? '')
const restBody = computed(
  () => log.details?.body.slice(subject.value.length).replace(/^\n+/, '') ?? '',
)

// committerDiffers: the committer earns its own field only when it
// actually differs from the author.
const committerDiffers = computed(() => {
  const d = log.details
  if (!d) return false
  return (
    d.committer !== d.author || d.committerEmail !== d.authorEmail || d.committerAt !== d.authorAt
  )
})
</script>

<template>
  <CompareView v-if="compare.active" :wt="wt" />
  <CompareWorkingView v-else-if="compare.workingCommit" :wt="wt" />
  <section v-else-if="log.details" class="commit-details">
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
    <ChangedFileList
      :files="log.details.files ?? []"
      :selected="log.selectedFile"
      @select="log.selectFile"
      @open="openDiff"
      @open-editor="openEditor"
    />
  </section>
</template>

<style scoped>
/* Fills the side column and scrolls within it. */
.commit-details {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  padding: 0.4rem 0.6rem;
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
