import { toValue, type MaybeRefOrGetter } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { Code, type ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { errorNotice, useNotices } from '@/lib/notice'
import type { CoreError, useCoreCall } from './useCoreCall'

// useOpenInEditor opens a folder or a file in the configured editor through
// run. An EDITOR_NOT_FOUND failure shows as its own notice naming the
// command, instead of run's error banner; a newer one replaces it.
export function useOpenInEditor(
  wt: MaybeRefOrGetter<string>,
  run: ReturnType<typeof useCoreCall>['run'],
): {
  openEditor: (f: ChangedFile) => void
  openFolder: () => void
  openFolderAt: (path: string) => void
} {
  const notices = useNotices()

  function showEditorNotice(error: CoreError): boolean {
    if (error.code !== Code.EditorNotFound) return false
    notices.show('editor', errorNotice(error))
    return true
  }

  function open(call: () => Promise<unknown>): void {
    void run(call, showEditorNotice)
  }

  return {
    openEditor: (f: ChangedFile) => open(() => Facade.OpenFile(toValue(wt), f.path, 0)),
    openFolder: () => open(() => Facade.OpenFolder(toValue(wt))),
    openFolderAt: (path: string) => open(() => Facade.OpenFolderAt(path)),
  }
}
