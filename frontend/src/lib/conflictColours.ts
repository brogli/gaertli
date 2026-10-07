import { EditorView } from '@codemirror/view'

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
// unresolved conflict region, a neutral tint of the text colour for a resolved one. The selected
// region's lines take a stronger tint and a bar at the left edge.
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
  '.cm-conflict-selected': {
    '--conflict-mix': 'var(--conflict-mix-selected)',
    boxShadow: 'inset 3px 0 0 var(--p-primary-color)',
  },
  '[class*="cm-conflict-"], [class*="cm-change-"]': {
    backgroundColor:
      'color-mix(in srgb, var(--conflict-tone) var(--conflict-mix), var(--p-content-background))',
  },
})
