<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from '@/i18n'
import { useDebounceFn } from '@vueuse/core'
import AutoComplete from 'openvue/autocomplete'
import MultiSelect from 'openvue/multiselect'
import ClearableInput from './ClearableInput.vue'
import { useLogStore } from '@/stores/log'
import { useRefsStore } from '@/stores/refs'
import { refGroups } from '@/lib/refOptions'
import type { LogFilter } from '../../bindings/github.com/brogli/gaertli/internal/core'

// FilterBar is the log filter: free-text author/path (debounced 300ms) and
// branches or tags narrow the log; the search field highlights matches
// instead (debounced in the store) and steps through them with
// Enter/Shift+Enter. Each field carries its own clear icon and applies at
// once when cleared. The branch field shows the log store's own filter, which the branch picker's
// Show history also sets.
const log = useLogStore()
const refs = useRefsStore()
const { t } = useI18n()

const author = ref(log.filter.author ?? '')
const path = ref(log.filter.path ?? '')
const authorSuggestions = ref<string[]>([])

const branchOptions = computed(() => refGroups(refs.refs, t))

// matchLabel is "n of m" once the search resolved, "No match" once it
// resolved to nothing, empty while the query field is itself empty.
const matchLabel = computed(() => {
  if (!log.query) return ''
  if (log.matches.length === 0) return t('filterBar.noMatch')
  return t('filterBar.matchPosition', { current: log.current + 1, total: log.matches.length })
})

function currentFilter(branches = log.filter.branches ?? []): LogFilter {
  const f: LogFilter = {}
  if (author.value) f.author = author.value
  if (branches.length > 0) f.branches = branches
  if (path.value) f.path = path.value
  return f
}

const applyDebounced = useDebounceFn(() => void log.applyFilter(currentFilter()), 300)

// A discrete choice (branches) or an explicit clear applies at once, never
// waiting on the debounce a keystroke stream needs.
function applyNow(): void {
  void log.applyFilter(currentFilter())
}

function filterBranches(branches: string[]): void {
  void log.applyFilter(currentFilter(branches))
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
  <!-- The fields share one row and shrink with the log view. -->
  <div class="filter-bar flex gap-2">
    <div class="flex flex-1 min-w-0 items-center gap-1.5">
      <ClearableInput
        v-model="log.query"
        class="flex-1 min-w-0"
        :placeholder="t('filterBar.searchMessage')"
        test-id="clear-query"
        @clear="log.clearQuery"
        @keydown.enter.exact.prevent="log.nextMatch"
        @keydown.shift.enter.prevent="log.prevMatch"
        @keydown.esc="log.clearQuery"
      />
      <span
        v-if="matchLabel"
        class="flex-none whitespace-nowrap text-muted-color text-sm"
        data-test="match-label"
        >{{ matchLabel }}</span
      >
    </div>
    <AutoComplete
      v-model="author"
      class="flex-1 min-w-0"
      input-class="w-full min-w-0"
      :suggestions="authorSuggestions"
      :placeholder="t('filterBar.author')"
      show-clear
      @complete="search"
      @update:model-value="applyDebounced"
      @clear="clearAuthor"
    />
    <MultiSelect
      class="flex-1 min-w-0"
      :model-value="log.filter.branches ?? []"
      :options="branchOptions"
      option-label="label"
      option-value="value"
      option-group-label="label"
      option-group-children="items"
      filter
      show-clear
      :show-toggle-all="false"
      :filter-placeholder="t('filterBar.searchBranches')"
      :placeholder="t('filterBar.branches')"
      @update:model-value="filterBranches"
    />
    <ClearableInput
      v-model="path"
      class="flex-1 min-w-0"
      :placeholder="t('filterBar.path')"
      test-id="clear-path"
      @update:model-value="applyDebounced"
      @clear="applyNow"
    />
  </div>
</template>
