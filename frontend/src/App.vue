<script setup lang="ts">
import { onMounted } from 'vue'
import ConfirmDialog from 'openvue/confirmdialog'
import NoticeArea from './components/NoticeArea.vue'
import MainView from './views/MainView.vue'
import DiffWindowView from './views/DiffWindowView.vue'
import { diffWindowQuery } from './lib/diffTarget'
import { useSettingsStore } from './stores/settings'
import { DIALOG } from './lib/layout'

// A window's query string, fixed for its lifetime, picks its root view:
// ?view=diff, else the main view.
const query = new URLSearchParams(window.location.search)
const path = query.get('path')
const distro = query.get('distro')
const diffWindow = diffWindowQuery(query)

// Settings load here for every view: the settings store applies
// settings.theme window-wide.
const settings = useSettingsStore()
onMounted(() => void settings.load())
</script>

<template>
  <ConfirmDialog :style="DIALOG.style" />
  <NoticeArea />
  <DiffWindowView v-if="diffWindow" v-bind="diffWindow" />
  <MainView v-else :path="path ?? ''" :distro="distro ?? undefined" />
</template>
