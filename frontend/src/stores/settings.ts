import { defineStore } from 'pinia'
import { ref, watch } from 'vue'
import { usePreferredDark } from '@vueuse/core'
import { Events } from '@wailsio/runtime'
import { AppService, SettingsService } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { Theme, type Preferences } from '../../bindings/github.com/brogli/nebegit/internal/settings'

export const useSettingsStore = defineStore('settings', () => {
  const settings = ref<Preferences | null>(null)
  // The installed distros do not change while the app runs; listing them
  // spawns wsl.exe, so the first successful answer is kept. A host without
  // WSL has none.
  let distros: string[] | undefined

  // Settings.theme applies live, whichever window saved it: dark or light
  // force it, the default (System) follows the OS.
  const prefersDark = usePreferredDark()
  watch(
    [() => settings.value?.theme, prefersDark],
    ([theme, osPrefersDark]) => {
      const dark = theme === Theme.DarkTheme || (theme !== Theme.LightTheme && osPrefersDark)
      document.documentElement.classList.toggle('nebegit-dark', dark)
    },
    { immediate: true },
  )

  async function load(): Promise<void> {
    settings.value = await SettingsService.Get()
  }

  // A save in any window, this one included, reaches every window.
  Events.On('settings:changed', () => void load())

  async function loadDistros(): Promise<string[]> {
    distros ??= (await AppService.ListWSLDistros()) ?? []
    return distros
  }

  async function save(next: Preferences): Promise<void> {
    await SettingsService.Set(next)
    settings.value = next
  }

  return { settings, load, loadDistros, save }
})
