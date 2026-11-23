import { EditorState, type Extension } from '@codemirror/state'
import { EditorView, lineNumbers } from '@codemirror/view'
import { LanguageDescription, syntaxHighlighting } from '@codemirror/language'
import { languages } from '@codemirror/language-data'
import { classHighlighter } from '@lezer/highlight'
import { basename } from './path'

// PANE_LAYOUT makes an editor fill its panel and scroll on its own.
export const PANE_LAYOUT = EditorView.theme({
  '&': { height: '100%' },
  '.cm-scroller': { overflow: 'auto' },
})

// classHighlighter emits plain "tok-*" classes; they take the app's own
// tokens rather than CodeMirror's default colours.
const CODE_COLOURS = EditorView.theme({
  '.tok-keyword, .tok-operator': { color: 'var(--p-primary-color)' },
  '.tok-comment': { color: 'var(--p-text-muted-color)', fontStyle: 'italic' },
})

// FIXED_WIDTH_FONT sets the text in the app's fixed-width font, in place of
// CodeMirror's default system monospace.
const FIXED_WIDTH_FONT = EditorView.theme({ '.cm-scroller': { fontFamily: 'var(--nb-font-mono)' } })

// EDITOR_LOOK sets what CodeMirror's light base theme would show in every app
// theme (gutters) to the app's content tokens.
const EDITOR_LOOK = EditorView.theme({
  '.cm-gutters': {
    backgroundColor: 'var(--p-content-background)',
    color: 'var(--p-text-muted-color)',
    borderRight: '1px solid var(--p-content-border-color)',
  },
})

// lineNumbersWithMenu is the line-number gutter; right-clicking a number
// calls onMenu with that line's 1-based number and the event.
export function lineNumbersWithMenu(onMenu: (line: number, event: MouseEvent) => void): Extension {
  return lineNumbers({
    domEventHandlers: {
      contextmenu(view, block, event) {
        if (!(event instanceof MouseEvent)) return false
        event.preventDefault()
        onMenu(view.state.doc.lineAt(block.from).number, event)
        return true
      },
    },
  })
}

// languageExtensionFor is the syntax highlighting for path's filename, or
// null (plain text) when @codemirror/language-data knows none.
async function languageExtensionFor(path: string): Promise<Extension | null> {
  const lang = LanguageDescription.matchFilename(languages, basename(path))
  return lang ? await lang.load() : null
}

// codeEditorExtensions is what every code editor of the app shares for the
// file at path: the fixed-width font, syntax highlighting, and lines split
// at "\n" only, as git splits them (a lone "\r" stays in its line, so git's
// line numbers match the editor's).
export async function codeEditorExtensions(path: string): Promise<Extension[]> {
  const language = await languageExtensionFor(path)
  return [
    EditorState.lineSeparator.of('\n'),
    FIXED_WIDTH_FONT,
    EDITOR_LOOK,
    ...(language ? [language, syntaxHighlighting(classHighlighter), CODE_COLOURS] : []),
  ]
}
