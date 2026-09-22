<script setup lang="ts">
import ConfirmDialog from 'openvue/confirmdialog'
import MainView from './views/MainView.vue'
import CompareDiffView from './views/CompareDiffView.vue'

// A window's query string is fixed for its lifetime: parsed once here, it
// decides which root view this window is. ?view=compare-diff is an explicit
// discriminator, checked first, so the two window kinds never compete for
// the same branch.
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
  <MainView v-else :path="path ?? ''" />
</template>
