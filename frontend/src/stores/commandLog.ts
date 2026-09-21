import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { CommandLogEntry } from '../../bindings/github.com/brogli/nebegit/internal/core'

// commandLog is view-only (FR-UX-02): not persisted, reset on every mount.
export const useCommandLogStore = defineStore('commandLog', () => {
  const entries = ref<CommandLogEntry[]>([])
  const showBackground = ref(false)

  const visible = computed(() =>
    showBackground.value ? entries.value : entries.value.filter((e) => !e.background),
  )

  async function load(wt: string): Promise<void> {
    entries.value = (await Facade.CommandLog(wt)) ?? []
  }

  function append(entry: CommandLogEntry): void {
    entries.value.push(entry)
  }

  return { entries, visible, showBackground, load, append }
})
