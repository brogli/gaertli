import type { Extension } from '@codemirror/state'
import { EditorView, keymap } from '@codemirror/view'
import {
  closeSearchPanel,
  getSearchQuery,
  openSearchPanel,
  search,
  searchKeymap,
  searchPanelOpen,
  setSearchQuery,
  type SearchQuery,
} from '@codemirror/search'

// SEARCH_KEYS are the library's search bindings the app offers: Ctrl+F
// opens, F3 and Shift+F3 step, Escape closes.
const SEARCH_KEYS = new Set(['Mod-f', 'F3', 'Escape'])

// editorSearch is the search panel at the editor's top; a match it steps to
// is scrolled to the middle.
export const editorSearch: Extension = [
  search({
    top: true,
    scrollToMatch: (range) => EditorView.scrollIntoView(range, { y: 'center' }),
  }),
  keymap.of(searchKeymap.filter((binding) => binding.key && SEARCH_KEYS.has(binding.key))),
]

// closeOpenSearches closes the search of every editor in the window and
// says whether one was open.
export function closeOpenSearches(): boolean {
  let closed = false
  for (const el of document.querySelectorAll<HTMLElement>('.cm-editor')) {
    const view = EditorView.findFromDOM(el)
    if (view && closeSearchPanel(view)) closed = true
  }
  return closed
}

// A KeptSearch is what of an editor's open search a remount brings back.
export interface KeptSearch {
  query: SearchQuery
  focused: boolean
}

// keptSearch is cm's open search, or undefined when none is.
export function keptSearch(cm: EditorView): KeptSearch | undefined {
  if (!searchPanelOpen(cm.state)) return undefined
  const focused = !!cm.dom.querySelector('.cm-search')?.contains(document.activeElement)
  return { query: getSearchQuery(cm.state), focused }
}

// restoreSearch reopens kept in cm, the focus in its field only if it was
// there. Opening focuses the field, so otherwise the focus goes back.
export function restoreSearch(cm: EditorView, kept: KeptSearch | undefined): void {
  if (!kept) return
  const before = document.activeElement
  openSearchPanel(cm)
  cm.dispatch({ effects: setSearchQuery.of(kept.query) })
  const field = cm.dom.querySelector<HTMLElement>('.cm-search [main-field]')
  if (kept.focused) return field?.focus()
  field?.blur()
  if (before instanceof HTMLElement) before.focus()
}
