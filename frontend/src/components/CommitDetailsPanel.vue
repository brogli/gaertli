<script setup lang="ts">
import { useLogStore } from '../stores/log'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import ChangedFileList from './ChangedFileList.vue'
import ErrorBanner from './ErrorBanner.vue'

// CommitDetailsPanel shows the log store's selected commit (FR-LOG-03):
// message, committer info, parents, and changed files. Empty until a row
// is selected. wt (FR-ED-02) is this window's own worktree, needed
// to resolve a file's editor-open call against it.
const props = defineProps<{ wt: string }>()
const log = useLogStore()
const { error, run } = useCoreCall()

function openEditor(f: ChangedFile): void {
  void run(() => Facade.OpenFile(props.wt, f.path, 0))
}
</script>

<template>
  <section v-if="log.details" class="commit-details">
    <ErrorBanner v-if="error" :error="error" />
    <pre class="body">{{ log.details.body }}</pre>
    <p class="author">{{ log.details.author }} &lt;{{ log.details.authorEmail }}&gt;</p>
    <p class="committer">{{ log.details.committer }} &lt;{{ log.details.committerEmail }}&gt;</p>
    <ul class="parents">
      <li v-for="parent in log.details.parents ?? []" :key="parent">{{ parent }}</li>
    </ul>
    <ChangedFileList
      :files="log.details.files ?? []"
      :selected="log.selectedFile"
      @select="log.selectFile"
      @open-editor="openEditor"
    />
  </section>
</template>
