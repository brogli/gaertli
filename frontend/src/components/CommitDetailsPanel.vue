<script setup lang="ts">
import { computed } from 'vue'
import { formatTimeAgo } from '@vueuse/core'
import { useLogStore } from '../stores/log'
import { useChangesStore } from '../stores/changes'
import { useCoreCall } from '../composables/useCoreCall'
import { useOpenInEditor } from '../composables/useOpenInEditor'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import ChangedFileList from './ChangedFileList.vue'
import ErrorBanner from './ErrorBanner.vue'

// CommitDetailsPanel shows the log store's selected commit: message,
// committer info, parents, and changed files. Empty until a row is
// selected.
const props = defineProps<{ wt: string }>()
const log = useLogStore()
const changes = useChangesStore()
const { error, run } = useCoreCall()
const { openEditor, openEditorFallback } = useOpenInEditor(() => props.wt, run)

// selecting one of this commit's files wins over a stale changes-panel diff
function selectFile(f: ChangedFile): void {
  changes.clearSelection()
  log.selectFile(f)
}

function shortHash(sha: string): string {
  return sha.slice(0, 7)
}

function relativeDate(unixSeconds: number): string {
  return formatTimeAgo(new Date(unixSeconds * 1000))
}

function absoluteDate(unixSeconds: number): string {
  return new Date(unixSeconds * 1000).toLocaleString()
}

// subject is the message's first line; restBody is whatever follows a
// blank line, shown separately below it.
const subject = computed(() => log.details?.body.split('\n')[0] ?? '')
const restBody = computed(() => log.details?.body.slice(subject.value.length).replace(/^\n+/, '') ?? '')

// committerDiffers: the committer earns its own field only when it
// actually differs from the author.
const committerDiffers = computed(() => {
  const d = log.details
  if (!d) return false
  return d.committer !== d.author || d.committerEmail !== d.authorEmail || d.committerAt !== d.authorAt
})
</script>

<template>
  <section v-if="log.details" class="commit-details">
    <ErrorBanner v-if="error" :error="error" @fallback="openEditorFallback" />
    <p class="subject">{{ subject }}</p>
    <pre v-if="restBody" class="body">{{ restBody }}</pre>
    <dl class="fields">
      <dt>Commit</dt>
      <dd :title="log.details.sha">{{ shortHash(log.details.sha) }}</dd>
      <dt>Author</dt>
      <dd :title="absoluteDate(log.details.authorAt)">
        {{ log.details.author }} &lt;{{ log.details.authorEmail }}&gt; · {{ relativeDate(log.details.authorAt) }}
      </dd>
      <template v-if="committerDiffers">
        <dt>Committer</dt>
        <dd :title="absoluteDate(log.details.committerAt)">
          {{ log.details.committer }} &lt;{{ log.details.committerEmail }}&gt; ·
          {{ relativeDate(log.details.committerAt) }}
        </dd>
      </template>
      <template v-if="(log.details.parents ?? []).length">
        <dt>Parents</dt>
        <dd class="parents">
          <span v-for="parent in log.details.parents" :key="parent" :title="parent">{{ shortHash(parent) }}</span>
        </dd>
      </template>
    </dl>
    <ChangedFileList
      :files="log.details.files ?? []"
      :selected="log.selectedFile"
      @select="selectFile"
      @open-editor="openEditor"
    />
  </section>
</template>

<style scoped>
/* Natural height, up to 45% of the side column, so an open diff below
   never squeezes the details out of sight; beyond that it scrolls. */
.commit-details {
  flex: none;
  max-height: 45%;
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
