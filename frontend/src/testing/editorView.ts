import type { VueWrapper } from '@vue/test-utils'
import { EditorView } from '@codemirror/view'

// editorViews are the EditorViews mounted in the wrapper, in document order.
export function editorViews(wrapper: VueWrapper): EditorView[] {
  return wrapper.findAll<HTMLElement>('.cm-editor').map((el) => {
    const view = EditorView.findFromDOM(el.element)
    if (!view) throw new Error('no EditorView mounted')
    return view
  })
}

// lineNumbersOf are the numbers shown in the editor's gutter, one per line.
export function lineNumbersOf(view: EditorView): string[] {
  return [...view.dom.querySelectorAll('.cm-lineNumbers .cm-gutterElement')]
    .slice(1) // the first element is the hidden width spacer
    .map((el) => el.textContent ?? '')
}

// selectedLineOf is the 1-based line of view's selection.
export function selectedLineOf(view: EditorView): number {
  return view.state.doc.lineAt(view.state.selection.main.from).number
}

// pressIn dispatches a keydown in view's content, as typing there does.
export function pressIn(view: EditorView, init: KeyboardEventInit): void {
  view.contentDOM.dispatchEvent(
    new KeyboardEvent('keydown', { ...init, bubbles: true, cancelable: true }),
  )
}
