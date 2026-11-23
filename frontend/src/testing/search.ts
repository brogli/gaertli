import type { EditorView } from '@codemirror/view'

// searchFieldOf is the text field of view's open search panel, or null.
export function searchFieldOf(view: EditorView): HTMLInputElement | null {
  return view.dom.querySelector('.cm-search input[name="search"]')
}

function openFieldOf(view: EditorView): HTMLInputElement {
  const field = searchFieldOf(view)
  if (!field) throw new Error('no search panel open')
  return field
}

// searchFor types text into view's open search field.
export function searchFor(view: EditorView, text: string): void {
  const field = openFieldOf(view)
  field.value = text
  field.dispatchEvent(new Event('keyup'))
}

// pressEnterIn presses Enter, or Shift+Enter, in view's open search field.
export function pressEnterIn(view: EditorView, shiftKey = false): void {
  // The panel reads the key's keyCode, which happy-dom's init leaves at 0.
  const event = new KeyboardEvent('keydown', { key: 'Enter', shiftKey, bubbles: true })
  Object.defineProperty(event, 'keyCode', { value: 13 })
  openFieldOf(view).dispatchEvent(event)
}

// matchesOf are the texts of view's highlighted search matches.
export function matchesOf(view: EditorView): string[] {
  return [...view.dom.querySelectorAll('.cm-searchMatch')].map((el) => el.textContent ?? '')
}
