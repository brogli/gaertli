import type { EditorView } from '@codemirror/view'

// lineNumbersOf are the numbers shown in the editor's gutter, one per line.
export function lineNumbersOf(view: EditorView): string[] {
  return [...view.dom.querySelectorAll('.cm-lineNumbers .cm-gutterElement')]
    .slice(1) // the first element is the hidden width spacer
    .map((el) => el.textContent ?? '')
}
