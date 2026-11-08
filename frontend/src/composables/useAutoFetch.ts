import { useIntervalFn } from '@vueuse/core'
import { watch } from 'vue'
import { useSettingsStore } from '@/stores/settings'
import { Facade } from '../../bindings/github.com/brogli/gaertli/internal/app'

// useAutoFetch fetches every settings.autoFetchIntervalMinutes while
// mounted; 0 (the default) is off. A background fetch's failure is not
// shown: only a manual fetch reports errors. useIntervalFn follows an
// interval change only while active, so the watch turns it on and off at 0.
export function useAutoFetch(wt: () => string): void {
  const settings = useSettingsStore()
  const { pause, resume } = useIntervalFn(
    () => void Facade.Fetch(wt(), '').catch(() => undefined),
    () => Math.max(1, settings.settings?.autoFetchIntervalMinutes ?? 0) * 60_000,
    { immediate: false },
  )
  watch(
    () => settings.settings?.autoFetchIntervalMinutes,
    (minutes) => ((minutes ?? 0) > 0 ? resume() : pause()),
    { immediate: true },
  )
}
