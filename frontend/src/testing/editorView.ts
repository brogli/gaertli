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
