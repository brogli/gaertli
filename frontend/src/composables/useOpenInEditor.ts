import { toValue, type MaybeRefOrGetter } from 'vue'
import { Facade } from '../../bindings/github.com/brogli/nebegit/internal/app'
import type {
  ChangedFile,
  EditorFallbackKind,
} from '../../bindings/github.com/brogli/nebegit/internal/core'
import type { useCoreCall } from './useCoreCall'

// useOpenInEditor opens a changed file in wt's configured editor through
// run, and remembers it so openEditorFallback can retry it in a fallback
// editor after EDITOR_NOT_FOUND.
export function useOpenInEditor(
  wt: MaybeRefOrGetter<string>,
  run: ReturnType<typeof useCoreCall>['run'],
) {
  let lastPath = ''

  function openEditor(f: ChangedFile): void {
    lastPath = f.path
    void run(() => Facade.OpenFile(toValue(wt), f.path, 0))
  }

  function openEditorFallback(kind: EditorFallbackKind): void {
    void run(() => Facade.OpenFileFallback(toValue(wt), lastPath, 0, kind))
  }

  return { openEditor, openEditorFallback }
}
