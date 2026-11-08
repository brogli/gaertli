import { useLocalStorage } from '@vueuse/core'
import type { RemovableRef } from '@vueuse/core'

// useFileTreeChoice is the changed files' list/tree choice: shared and
// remembered across every place that shows changed files (the commit
// detail view, the diff window, the changes view), synced live across
// windows.
export function useFileTreeChoice(): RemovableRef<boolean> {
  return useLocalStorage('gaertli.changedFiles.tree', false)
}
