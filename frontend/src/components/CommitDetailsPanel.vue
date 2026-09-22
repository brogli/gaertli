<script setup lang="ts">
import { useLogStore } from '../stores/log'
import ChangedFileList from './ChangedFileList.vue'

// CommitDetailsPanel shows the log store's selected commit (FR-LOG-03):
// message, committer info, parents, and changed files. Empty until a row
// is selected.
const log = useLogStore()
</script>

<template>
  <section v-if="log.details" class="commit-details">
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
    />
  </section>
</template>
