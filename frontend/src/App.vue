<script setup lang="ts">
import { computed } from 'vue'
import WorktreeView from './views/WorktreeView.vue'
import HomeView from './views/HomeView.vue'

const path = computed(() => new URLSearchParams(window.location.search).get('path'))
</script>

<template>
  <WorktreeView v-if="path" :path="path" />
  <HomeView v-else />
</template>
