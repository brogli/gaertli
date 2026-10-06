import { EditorView } from '@codemirror/view'

const MUTED = 'var(--nb-tone-muted)'

// The tone and the share of it, in percent, mixed into the background: for
// the line and, in a selected region, for the stronger tint.
const TINTS: Record<string, [tone: string, mix: number, selectedMix: number]> = {
  current: ['var(--nb-from)', 16, 34],
  incoming: ['var(--nb-to)', 16, 34],
  base: [MUTED, 14, 30],
  marker: [MUTED, 22, 40],
  resolved: [MUTED, 8, 18],
}

// conflictColours tints the lines of the merge editor's conflict regions
// (the cm-conflict-* line classes of conflictRegions.ts) from the theme's
// tone tokens. The selected region's lines take a stronger tint and a bar
// at the left edge.
export const conflictColours = EditorView.theme({
  ...Object.fromEntries(
    Object.entries(TINTS).map(([name, [tone, mix, selectedMix]]) => [
      `.cm-conflict-${name}`,
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
  '[class*="cm-conflict-"]': {
    backgroundColor:
      'color-mix(in srgb, var(--conflict-tone) var(--conflict-mix), var(--p-content-background))',
  },
  '.cm-conflict-marker': { color: 'var(--p-text-muted-color)' },
})
