// The selected change of a code editor: the diff view's selected chunk, the
// merge editor's selected region. Its lines are outlined by a thin, muted
// line in the text colour and marked in the accent colour in a gutter of their own.

import type { Extension, StateField } from '@codemirror/state'
import { type BlockInfo, EditorView, gutter, GutterMarker } from '@codemirror/view'

const OUTLINE = '1px'
const ACCENT = 'var(--p-primary-color)'
const MUTED = 'color-mix(in srgb, var(--p-text-color) 35%, transparent)'

const SELECTED_LOOK = EditorView.theme({
  '.cm-chunk-selected': {
    '--outline-top': '0px',
    '--outline-bottom': '0px',
    boxShadow: [
      `inset ${OUTLINE} 0 0 0 ${MUTED}`,
      `inset -${OUTLINE} 0 0 0 ${MUTED}`,
      `inset 0 var(--outline-top) 0 0 ${MUTED}`,
      `inset 0 calc(-1 * var(--outline-bottom)) 0 0 ${MUTED}`,
    ].join(', '),
  },
  '.cm-chunk-first': { '--outline-top': OUTLINE },
  '.cm-chunk-last': { '--outline-bottom': OUTLINE },
  '.cm-chunk-gutter': { minWidth: '6px' },
  '.cm-chunk-gutter-mark': { backgroundColor: ACCENT },
})

// selectedClasses are the line classes of a line of the selected change:
// the outline, and its top on the change's first line, its bottom on the last.
export const selectedClasses = (first: boolean, last: boolean): string =>
  `cm-chunk-selected${first ? ' cm-chunk-first' : ''}${last ? ' cm-chunk-last' : ''}`

const selectionMarker = new (class extends GutterMarker {
  elementClass = 'cm-chunk-gutter-mark'
})()

// selectionGutter marks the selected change in a gutter beside the pane's
// lines: the lines and block widgets that, by field's value, belong to it.
// It brings the selected change's look.
export function selectionGutter<V>(
  field: StateField<V>,
  lineMarked: (value: V, line: BlockInfo) => boolean,
  widgetMarked: (value: V, block: BlockInfo) => boolean = () => false,
): Extension {
  return [
    SELECTED_LOOK,
    gutter({
      class: 'cm-chunk-gutter',
      lineMarker: ({ state }, line) =>
        lineMarked(state.field(field), line) ? selectionMarker : null,
      widgetMarker: ({ state }, _widget, block) =>
        widgetMarked(state.field(field), block) ? selectionMarker : null,
      lineMarkerChange: (update) => update.startState.field(field) !== update.state.field(field),
    }),
  ]
}
