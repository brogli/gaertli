<script setup lang="ts">
import ConfirmDialog from 'openvue/confirmdialog'
import WorktreeView from './views/WorktreeView.vue'
import HomeView from './views/HomeView.vue'
import CompareDiffView from './views/CompareDiffView.vue'

// A window's query string is fixed for its lifetime: parsed once here, it
// decides which root view this window is (spec 04's ?path=, spec 23's
// ?view=compare-diff — an explicit discriminator, checked first, so the two
// window kinds never compete for the same branch).
const query = new URLSearchParams(window.location.search)
const path = query.get('path')

const compareDiffProps = (() => {
  if (query.get('view') !== 'compare-diff') return null
  const wt = query.get('wt'),
    from = query.get('from'),
    to = query.get('to')
  if (!wt || !from || !to) return null
  return { wt, from, to, path: query.get('file') ?? '', oldPath: query.get('oldFile') ?? '' }
})()
</script>

<template>
  <ConfirmDialog />
  <CompareDiffView v-if="compareDiffProps" v-bind="compareDiffProps" />
  <WorktreeView v-else-if="path" :path="path" />
  <HomeView v-else />
</template>
