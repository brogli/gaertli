<script setup lang="ts">
import { onMounted } from 'vue'
import ConfirmDialog from 'openvue/confirmdialog'
import MainView from './views/MainView.vue'
import CompareDiffView from './views/CompareDiffView.vue'
import { useSettingsStore } from './stores/settings'

// A window's query string, fixed for its lifetime, picks its root view:
// ?view=compare-diff, else the main view.
const query = new URLSearchParams(window.location.search)
const path = query.get('path')
const distro = query.get('distro')

const compareDiffProps = (() => {
  if (query.get('view') !== 'compare-diff') return null
  const wt = query.get('wt'),
    from = query.get('from'),
    to = query.get('to')
  if (!wt || !from || !to) return null
  return { wt, from, to, path: query.get('file') ?? '', oldPath: query.get('oldFile') ?? '' }
})()

// Settings load here for every view: the settings store applies
// settings.theme window-wide.
const settings = useSettingsStore()
onMounted(() => void settings.load())
</script>

<template>
  <ConfirmDialog />
  <CompareDiffView v-if="compareDiffProps" v-bind="compareDiffProps" />
  <MainView v-else :path="path ?? ''" :distro="distro ?? undefined" />
</template>
