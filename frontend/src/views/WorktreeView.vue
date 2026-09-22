<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRepoStore } from '../stores/repo'
import { useRefsStore } from '../stores/refs'
import { useLogStore } from '../stores/log'
import { useCompareStore } from '../stores/compare'
import { useCoreCall } from '../composables/useCoreCall'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import ErrorBanner from '../components/ErrorBanner.vue'
import CommandLogPanel from '../components/CommandLogPanel.vue'
import SettingsPanel from '../components/SettingsPanel.vue'
import LogList from '../components/LogList.vue'
import CommitDetailsPanel from '../components/CommitDetailsPanel.vue'
import CompareView from '../components/CompareView.vue'
import DiffView from '../components/DiffView.vue'
import RefsSidebar from '../components/RefsSidebar.vue'
import FilterBar from '../components/FilterBar.vue'

const props = defineProps<{ path: string }>()
const name = props.path.split(/[\\/]/).pop() || props.path
const repo = useRepoStore()
const refs = useRefsStore()
const log = useLogStore()
const compare = useCompareStore()

// The compared file's diff range (FR-CMP-02); null outside compare mode or
// before a file is picked, so the template needs no non-null assertions.
const compareRange = computed(() => {
  const { from, to, selectedFile } = compare
  return from && to && selectedFile
    ? { from: from.sha, to: to.sha, path: selectedFile.path, oldPath: selectedFile.oldPath ?? '' }
    : null
})

// One decision for the diff pane: a compared file, or a commit's file
// outside compare mode — never a stale commit diff under CompareView.
const showDiff = computed(
  () => compareRange.value !== null || (!compare.active && !!log.selectedFile),
)
const { error, run } = useCoreCall()
const showSettings = ref(false)

onMounted(() => {
  void run(() => repo.loadSummary(props.path))
  void refs.load(props.path)
})

// FR-REPO-04: manual refresh. No loading/disabled state — repo:changed, if
// anything actually differs, arrives the normal way.
function refresh(): void {
  void run(() => Facade.Refresh(props.path))
}

// FR-REPO-03: the refs store already carries the current branch's
// upstream state; nothing to count without an upstream, and a gone
// upstream has no meaningful ahead/behind either (Design).
const tracked = computed(() => {
  const current = refs.refs?.branches?.find((b) => b.current)
  return current?.upstream && !current.upstreamGone ? current : undefined
})
</script>

<template>
  <main>
    <h1>{{ name }}</h1>
    <button type="button" @click="showSettings = !showSettings">Settings</button>
    <button type="button" @click="refresh">Refresh</button>
    <ErrorBanner v-if="error" :error="error" />
    <p v-else-if="repo.summary">
      {{ repo.summary.detached ? 'detached HEAD' : repo.summary.branch }}
      <template v-if="repo.summary.operation"> ({{ repo.summary.operation }})</template>
      <span v-if="tracked" class="ahead-behind" data-test="ahead-behind">
        ↑{{ tracked.ahead }} ↓{{ tracked.behind }}
      </span>
    </p>
    <SettingsPanel v-if="showSettings" />
    <div class="worktree-body">
      <RefsSidebar />
      <div class="log-area">
        <FilterBar />
        <LogList :wt="path" />
        <CompareView v-if="compare.active" :wt="path" />
        <CommitDetailsPanel v-else />
        <DiffView v-if="showDiff" :wt="path" :range="compareRange" />
      </div>
    </div>
    <CommandLogPanel :wt="path" />
  </main>
</template>

<style scoped>
/* LogList's virtual scroll (QA-PERF-03) needs a bounded viewport to clip
   against: fill the app (style.css's height chain), give the log area the
   leftover space, everything else its natural size. */
main {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}
.worktree-body {
  display: flex;
  gap: 1em;
  flex: 1;
  min-height: 0;
}
.log-area {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
  min-height: 0;
}
</style>
