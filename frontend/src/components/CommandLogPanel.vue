<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { Events } from '@wailsio/runtime'
import { useCommandLogStore } from '../stores/commandLog'
import type { CommandLogEntry } from '../../bindings/github.com/brogli/nebegit/internal/core'

const props = defineProps<{ wt: string }>()
const log = useCommandLogStore()

let unlisten: (() => void) | undefined

onMounted(() => {
  void log.load(props.wt)
  unlisten = Events.On('command', (ev) => log.append(ev.data as CommandLogEntry))
})

onUnmounted(() => {
  unlisten?.()
})

// The list scrolls to the newest entry whenever one is added or the
// background-commands filter reveals more of them.
const entriesEl = ref<HTMLElement | null>(null)
watch(
  () => log.visible.length,
  async () => {
    await nextTick()
    if (entriesEl.value) entriesEl.value.scrollTop = entriesEl.value.scrollHeight
  },
)
</script>

<template>
  <section class="command-log">
    <div class="header">
      <h2>Command log</h2>
      <label>
        <input v-model="log.showBackground" type="checkbox" />
        show background commands
      </label>
    </div>
    <div ref="entriesEl" class="entries">
      <div
        v-for="(entry, i) in log.visible"
        :key="i"
        class="entry"
        data-test="command-log-entry"
        :title="entry.command"
      >
        {{ entry.command }}
      </div>
    </div>
  </section>
</template>

<style scoped>
/* A bounded strip at the bottom of the view: the entries scroll inside it
   instead of crowding out the log. */
.command-log {
  flex: none;
  max-height: 25%;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.6rem;
  padding: 0.15rem 0.3rem;
}
.entries {
  overflow-y: auto;
  min-height: 0;
}
.entry {
  padding: 0.05rem 0.3rem;
  font-family: monospace;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
</style>
