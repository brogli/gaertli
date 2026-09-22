import { defineStore } from 'pinia'
import { ref } from 'vue'
import { AppService, SettingsService } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type { Settings } from '../../bindings/github.com/brogli/nebegit/internal/settings'
import type { Distro } from '../../bindings/github.com/brogli/nebegit/internal/remote'

export const useSettingsStore = defineStore('settings', () => {
  const settings = ref<Settings | null>(null)
  // The installed distros do not change while the app runs; listing them
  // spawns wsl.exe, so the first successful answer is kept for every later
  // mount of the settings panel.
  const distros = ref<Distro[] | null>(null)

  async function load(): Promise<void> {
    settings.value = await SettingsService.Get()
  }

  async function loadDistros(): Promise<Distro[]> {
    distros.value ??= (await AppService.ListWSLDistros()) ?? []
    return distros.value
  }

  async function save(next: Settings): Promise<void> {
    await SettingsService.Set(next)
    settings.value = next
  }

  return { settings, distros, load, loadDistros, save }
})
