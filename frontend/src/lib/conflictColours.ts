import { EditorView } from '@codemirror/view'

const OUTLINE = '2px'
const ACCENT = 'var(--p-primary-color)'

// The line classes of the merge editor's panes (conflictRegions.ts), each
// with the theme tone it mixes into the background and the share of it, in
// percent, for the line and for the selected region's stronger tint.
const TINTS: Record<string, [tone: string, mix: number, selectedMix: number]> = {
  'cm-change-changed': ['var(--nb-tone-blue)', 25, 40],
  'cm-change-added': ['var(--nb-tone-green)', 25, 40],
  'cm-conflict-unresolved': ['var(--nb-tone-red)', 25, 40],
  'cm-conflict-resolved': ['var(--p-text-color)', 22, 36],
}

// conflictColours tints the merge editor's lines from the theme's tone
// tokens: blue for lines a side changed, green for lines it added, red for an
// unresolved conflict, a neutral tint of the text colour for a resolved
// one. The selected region's lines take a stronger tint and an outline in the
// theme's accent colour around the region, and a marker in the pane's gutter.
export const conflictColours = EditorView.theme({
  ...Object.fromEntries(
    Object.entries(TINTS).map(([name, [tone, mix, selectedMix]]) => [
      `.${name}`,
      {
        '--conflict-tone': tone,
        '--conflict-mix': `${mix}%`,
        '--conflict-mix-selected': `${selectedMix}%`,
      },
    ]),
  ),
  '.cm-chunk-selected': {
    '--conflict-mix': 'var(--conflict-mix-selected)',
    '--outline-top': '0px',
    '--outline-bottom': '0px',
    boxShadow: [
      `inset ${OUTLINE} 0 0 0 ${ACCENT}`,
      `inset -${OUTLINE} 0 0 0 ${ACCENT}`,
      `inset 0 var(--outline-top) 0 0 ${ACCENT}`,
      `inset 0 calc(-1 * var(--outline-bottom)) 0 0 ${ACCENT}`,
    ].join(', '),
  },
  '.cm-chunk-first': { '--outline-top': OUTLINE },
  '.cm-chunk-last': { '--outline-bottom': OUTLINE },
  '.cm-conflict-placeholder': { fontStyle: 'italic' },
  '.cm-chunk-gutter': { minWidth: '6px' },
  '.cm-chunk-gutter-mark': { backgroundColor: ACCENT },
  '[class*="cm-conflict-"], [class*="cm-change-"]': {
    backgroundColor:
      'color-mix(in srgb, var(--conflict-tone) var(--conflict-mix), var(--p-content-background))',
  },
})
