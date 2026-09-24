<script setup lang="ts">
import FilterBar from './FilterBar.vue'
import LogList from './LogList.vue'

// LogView is the log view: the filter bar above the commit list. It
// forwards LogList's own events unchanged.
defineProps<{ wt: string }>()
const emit = defineEmits<{
  'create-branch': [sha: string]
  'checkout-commit': [sha: string]
  reset: [sha: string, subject: string]
  'create-worktree': [sha: string]
  open: []
}>()
</script>

<template>
  <div class="log-column">
    <FilterBar />
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

<style scoped>
/* Fills whichever layout container places it; LogList's virtual scroll
   needs this bounded so it, not the page, scrolls. */
.log-column {
  display: flex;
  flex-direction: column;
  min-width: 0;
  min-height: 0;
  height: 100%;
}
</style>
