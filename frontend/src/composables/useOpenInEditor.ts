import { toValue, type MaybeRefOrGetter } from 'vue'
import { useToast } from 'openvue/usetoast'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import { Code, type ChangedFile } from '../../bindings/github.com/brogli/nebegit/internal/core'
import { NOTICE_GROUP, type AppNotice } from '../lib/notice'
import type { CoreError, useCoreCall } from './useCoreCall'

// useOpenInEditor opens a folder or a file in the configured editor through
// run. An EDITOR_NOT_FOUND failure shows as its own notice naming the
// command, instead of run's error banner.
export function useOpenInEditor(
  wt: MaybeRefOrGetter<string>,
  run: ReturnType<typeof useCoreCall>['run'],
) {
  const toast = useToast()

  function showEditorNotice(error: CoreError): boolean {
    if (error.code !== Code.EditorNotFound) return false
    const notice: AppNotice = {
      severity: 'error',
      group: NOTICE_GROUP,
      summary: error.message,
      closable: true,
      data: { error },
    }
    toast.add(notice)
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
