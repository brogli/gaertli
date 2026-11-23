import type { EditorView } from '@codemirror/view'
import { openSearchPanel } from '@codemirror/search'
import { useShortcut } from './useShortcut'

// useEditorSearch opens defaultEditor's search on a Ctrl+F from outside the
// editors; inside one, the editor opens its own.
export function useEditorSearch(defaultEditor: () => EditorView | null | undefined): void {
  useShortcut(
    (event) => event.ctrlKey && !event.shiftKey && !event.altKey && event.key.toLowerCase() === 'f',
    (event) => {
      if (event.target instanceof Element && event.target.closest('.cm-editor')) return
      const view = defaultEditor()
      if (view) openSearchPanel(view)
    },
  )
}
