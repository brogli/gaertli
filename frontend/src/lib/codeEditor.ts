import { EditorState, type Extension } from '@codemirror/state'
import { EditorView } from '@codemirror/view'
import { LanguageDescription, syntaxHighlighting } from '@codemirror/language'
import { languages } from '@codemirror/language-data'
import { classHighlighter } from '@lezer/highlight'
import { basename } from './path'

// classHighlighter emits plain "tok-*" classes; they take the app's own
// tokens rather than CodeMirror's default colours.
const CODE_COLOURS = EditorView.theme({
  '.tok-keyword, .tok-operator': { color: 'var(--p-primary-color)' },
  '.tok-comment': { color: 'var(--p-text-muted-color)', fontStyle: 'italic' },
})

// FIXED_WIDTH_FONT sets the text in the app's fixed-width font, in place of
// CodeMirror's default system monospace.
const FIXED_WIDTH_FONT = EditorView.theme({ '.cm-scroller': { fontFamily: 'var(--nb-font-mono)' } })

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
    ...(language ? [language, syntaxHighlighting(classHighlighter), CODE_COLOURS] : []),
  ]
}
