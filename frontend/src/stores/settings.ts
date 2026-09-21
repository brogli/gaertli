import { defineStore } from 'pinia'
import { ref } from 'vue'
import { SettingsService } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { Settings } from '../../bindings/github.com/brogli/nebegit/internal/settings'

export const useSettingsStore = defineStore('settings', () => {
  const settings = ref<Settings | null>(null)

  async function load(): Promise<void> {
    settings.value = await SettingsService.Get()
  }

  async function save(next: Settings): Promise<void> {
    await SettingsService.Set(next)
    settings.value = next
  }

  return { settings, load, save }
})
