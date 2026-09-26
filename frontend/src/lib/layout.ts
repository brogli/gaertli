import { computed, type ComputedRef } from 'vue'
import { useBreakpoints } from '@vueuse/core'

// The window widths (px) where the three-view layout switches between
// wide, medium and narrow.
const LAYOUT_BREAKPOINTS = { medium: 900, wide: 1400 } as const

export type Layout = 'wide' | 'medium' | 'narrow'

// useLayout reports the current layout for the window's width, reactively.
export function useLayout(): ComputedRef<Layout> {
  const active = useBreakpoints(LAYOUT_BREAKPOINTS).active()
  return computed<Layout>(() => active.value || 'narrow')
}

// ICON_BUTTON is an icon-only Button's look in a view or list row, bound
// with v-bind. The icon carries no readable text, so callers pair it with
// title and aria-label naming the action.
export const ICON_BUTTON = { text: true, size: 'small', severity: 'secondary' } as const

// SECONDARY_BUTTON is a labelled Button's look in a view or dialog body,
// beside its content (hunk actions, presets, Stage all), bound with v-bind.
export const SECONDARY_BUTTON = { size: 'small', severity: 'secondary', outlined: true } as const

// DIALOG is a form dialog's props, bound with v-bind: modal, one width that
// gives its fields room without passing the window's edge, and its body's
// rows one below the other.
export const DIALOG = {
  modal: true,
  style: { width: '30rem', maxWidth: '90vw' },
  contentClass: 'flex flex-col gap-4',
} as const

interface ToolbarButton {
  severity: 'secondary'
  outlined: true
  size: 'small' | undefined
}

// useToolbarButton is every toolbar Button's look, bound with v-bind:
// secondary and outlined, and small below the medium width.
export function useToolbarButton(): ComputedRef<ToolbarButton> {
  const layout = useLayout()
  return computed(() => ({
    severity: 'secondary',
    outlined: true,
    size: layout.value === 'narrow' ? 'small' : undefined,
  }))
}
