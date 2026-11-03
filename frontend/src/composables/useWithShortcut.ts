import { useI18n } from '@/i18n'

// useWithShortcut returns a function that builds a tooltip from an action's
// and a shortcut's locale keys.
export function useWithShortcut(): (actionKey: string, shortcutKey: string) => string {
  const { t } = useI18n()
  return (actionKey, shortcutKey) =>
    t('shortcuts.withShortcut', { action: t(actionKey), shortcut: t(shortcutKey) })
}
