import type { InjectionKey } from 'vue'

// openSettingsKey is MainView's action that opens the Settings dialog, for
// an ErrorBanner anywhere below it to link to the editor setting.
export const openSettingsKey: InjectionKey<() => void> = Symbol('openSettings')
