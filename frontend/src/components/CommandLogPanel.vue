<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
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
</script>

<template>
  <section class="command-log">
    <label>
      <input v-model="log.showBackground" type="checkbox" />
      show background commands
    </label>
    <ul>
      <li v-for="(entry, i) in log.visible" :key="i">{{ entry.command }}</li>
    </ul>
  </section>
</template>
