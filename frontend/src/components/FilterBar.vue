<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from '../i18n'
import { useDebounceFn } from '@vueuse/core'
import AutoComplete from 'openvue/autocomplete'
import MultiSelect from 'openvue/multiselect'
import ClearableInput from './ClearableInput.vue'
import { useLogStore } from '../stores/log'
import { useRefsStore } from '../stores/refs'
import type { LogFilter } from '../../bindings/github.com/brogli/nebegit/internal/core'

// FilterBar is the log filter: free-text query/author/path (debounced
// 300ms) and branches. Each field carries its own clear icon and applies at
// once when cleared; there is no active-filter list or clear-all.
const log = useLogStore()
const refs = useRefsStore()
const { t } = useI18n()

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

function currentFilter(): LogFilter {
  const f: LogFilter = {}
  if (query.value) f.query = query.value
  if (author.value) f.author = author.value
  if (branches.value.length > 0) f.branches = branches.value
  if (path.value) f.path = path.value
  return f
}

const applyDebounced = useDebounceFn(() => void log.applyFilter(currentFilter()), 300)

// A discrete choice (branches) or an explicit clear applies at once, never
// waiting on the debounce a keystroke stream needs.
function applyNow(): void {
  void log.applyFilter(currentFilter())
}

function search(event: { query: string }): void {
  const q = event.query.toLowerCase()
  authorSuggestions.value = log.knownAuthors.filter((a) => a.toLowerCase().includes(q))
}

function clearAuthor(): void {
  author.value = ''
  applyNow()
}
</script>

<template>
  <div class="filter-bar">
    <ClearableInput
      v-model="query"
      :placeholder="t('filterBar.searchMessage')"
      test-id="clear-query"
      @update:model-value="applyDebounced"
      @clear="applyNow"
    />
    <AutoComplete
      v-model="author"
      :suggestions="authorSuggestions"
      :placeholder="t('filterBar.author')"
      show-clear
      @complete="search"
      @update:model-value="applyDebounced"
      @clear="clearAuthor"
    />
    <MultiSelect
      v-model="branches"
      :options="branchOptions"
      filter
      show-clear
      :show-toggle-all="false"
      :filter-placeholder="t('filterBar.searchBranches')"
      :placeholder="t('filterBar.branches')"
      @update:model-value="applyNow"
    />
    <ClearableInput
      v-model="path"
      :placeholder="t('filterBar.path')"
      test-id="clear-path"
      @update:model-value="applyDebounced"
      @clear="applyNow"
    />
  </div>
</template>

<style scoped>
/* The fields share one row and shrink with the log view. */
.filter-bar {
  display: flex;
  gap: 0.5em;
}
.filter-bar > * {
  flex: 1 1 0;
  min-width: 0;
}
.filter-bar :deep(.p-autocomplete-input) {
  width: 100%;
  min-width: 0;
}
/* IconField (ClearableInput's root) and its InputText: WebKitGTK, unlike
   Chromium, keeps an input's intrinsic width in a flex row unless both the
   field and the input itself are told they may shrink to 0. */
.filter-bar :deep(.p-iconfield) {
  min-width: 0;
}
.filter-bar :deep(.p-inputtext) {
  width: 100%;
  min-width: 0;
}
</style>
