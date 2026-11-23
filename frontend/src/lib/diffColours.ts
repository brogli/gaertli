import { EditorView } from '@codemirror/view'

// diffColours overrides @codemirror/merge's own low-contrast base theme
// (an 8% alpha barely different from the editor background) with the
// app's added/removed backgrounds and gutter markers, tuned per theme in
// main.css to FR-DIFF-11's colour-difference and contrast bars.
// EditorView.theme has higher precedence than merge's EditorView.baseTheme,
// so these rules win regardless of load order. The selected change's lines
// take the stronger selected backgrounds.
export const diffColours = EditorView.theme({
  '&.cm-merge-a .cm-changedLine, .cm-deletedChunk': {
    backgroundColor: 'var(--nb-diff-removed)',
  },
  '&.cm-merge-b .cm-changedLine': {
    backgroundColor: 'var(--nb-diff-added)',
  },
  '&.cm-merge-a .cm-changedLine.cm-chunk-selected, .cm-deletedChunk.cm-chunk-selected': {
    backgroundColor: 'var(--nb-diff-removed-selected)',
  },
  '&.cm-merge-b .cm-changedLine.cm-chunk-selected': {
    backgroundColor: 'var(--nb-diff-added-selected)',
  },
  '&.cm-merge-a .cm-changedLineGutter, .cm-deletedLineGutter': {
    background: 'var(--nb-diff-removed-marker)',
  },
  '&.cm-merge-b .cm-changedLineGutter': {
    background: 'var(--nb-diff-added-marker)',
  },
})
