<script setup lang="ts">
import { useLogStore } from '@/stores/log'
import ErrorBanner from './ErrorBanner.vue'
import FilterBar from './FilterBar.vue'
import LogList from './LogList.vue'

// LogView is the log view: the filter bar above the commit list, and
// between them the error a failed load ended with. It forwards LogList's
// own events unchanged.
defineProps<{ wt: string }>()
const log = useLogStore()
const emit = defineEmits<{
  'create-branch': [sha: string]
  'checkout-commit': [sha: string]
  reset: [sha: string, subject: string]
  'create-worktree': [sha: string]
  open: []
}>()
</script>

<template>
  <!-- Fills whichever layout container places it; LogList's virtual scroll
       needs this bounded so it, not the page, scrolls. -->
  <div class="log-column flex h-full min-w-0 min-h-0 flex-col">
    <FilterBar />
    <ErrorBanner v-if="log.error" :error="log.error" />
    <LogList
      :wt="wt"
      @create-branch="emit('create-branch', $event)"
      @checkout-commit="emit('checkout-commit', $event)"
      @reset="(sha, subject) => emit('reset', sha, subject)"
      @create-worktree="emit('create-worktree', $event)"
      @open="emit('open')"
    />
  </div>
</template>
