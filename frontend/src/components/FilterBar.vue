<script setup lang="ts">
import { computed, ref } from 'vue'
import { useDebounceFn } from '@vueuse/core'
import InputText from 'openvue/inputtext'
import AutoComplete from 'openvue/autocomplete'
import MultiSelect from 'openvue/multiselect'
import Chip from 'openvue/chip'
import { useLogStore } from '../stores/log'
import { useRefsStore } from '../stores/refs'
import type { LogFilter } from '../../bindings/github.com/brogli/nebegit/internal/core'

// FilterBar is FR-LOG-05..08's filter UI: free-text query/author/path
// (debounced 300ms, the repo watcher's own debounce number), branches (a
// discrete MultiSelect choice, applied immediately), active-filter chips,
// and one clear action (button or Escape).
const log = useLogStore()
const refs = useRefsStore()

const query = ref(log.filter.query ?? '')
const author = ref(log.filter.author ?? '')
const path = ref(log.filter.path ?? '')
const branches = ref<string[]>(log.filter.branches ?? [])
const authorSuggestions = ref<string[]>([])

// Branch options: local branch names, plus "<remote>/<name>" for each
// remote branch.
const branchOptions = computed(() => {
  const local = (refs.refs?.branches ?? []).map((b) => b.name)
  const remote = (refs.refs?.remotes ?? []).flatMap((r) =>
    (r.branches ?? []).map((b) => `${r.name}/${b.name}`),
  )
  return [...local, ...remote]
})

// Active-filter chips, one per non-empty field (FR-LOG-08).
const chips = computed(() => {
  const f = log.filter
  return [
    { key: 'query', text: f.query },
    { key: 'author', text: f.author },
    { key: 'branches', text: f.branches?.join(', ') },
    { key: 'path', text: f.path },
  ].filter((chip): chip is { key: string; text: string } => !!chip.text)
})

function currentFilter(): LogFilter {
  const f: LogFilter = {}
  if (query.value) f.query = query.value
  if (author.value) f.author = author.value
  if (branches.value.length > 0) f.branches = branches.value
  if (path.value) f.path = path.value
  return f
}

const applyDebounced = useDebounceFn(() => void log.applyFilter(currentFilter()), 300)

// Branches is a discrete choice, not a keystroke stream: applied
// immediately, no debounce.
function onBranchesChange(): void {
  void log.applyFilter(currentFilter())
}

function search(event: { query: string }): void {
  const q = event.query.toLowerCase()
  authorSuggestions.value = log.knownAuthors.filter((a) => a.toLowerCase().includes(q))
}

function clear(): void {
  query.value = ''
  author.value = ''
  path.value = ''
  branches.value = []
  void log.clearFilter()
}
</script>

<template>
  <div class="filter-bar" @keydown.escape="clear">
    <InputText v-model="query" placeholder="Search message" @update:model-value="applyDebounced" />
    <AutoComplete
      v-model="author"
      :suggestions="authorSuggestions"
      placeholder="Author"
      @complete="search"
      @update:model-value="applyDebounced"
    />
    <MultiSelect
      v-model="branches"
      :options="branchOptions"
      placeholder="Branches"
      @update:model-value="onBranchesChange"
    />
    <InputText v-model="path" placeholder="Path" @update:model-value="applyDebounced" />

    <template v-if="log.filterActive">
      <Chip v-for="chip in chips" :key="chip.key" :label="chip.text" :data-test="`chip-${chip.key}`" />
      <button type="button" data-test="clear-filters" @click="clear">Clear filters</button>
    </template>
  </div>
</template>

<style scoped>
.filter-bar {
  display: flex;
  align-items: center;
  gap: 0.5em;
  flex-wrap: wrap;
}
</style>
