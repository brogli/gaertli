import { computed } from 'vue'
import { useBreakpoints } from '@vueuse/core'

// The window widths (px) where the three-view layout switches between
// wide, medium and narrow.
const LAYOUT_BREAKPOINTS = { medium: 900, wide: 1400 } as const

export type Layout = 'wide' | 'medium' | 'narrow'

// useLayout reports the current layout for the window's width, reactively.
export function useLayout() {
  const active = useBreakpoints(LAYOUT_BREAKPOINTS).active()
  return computed<Layout>(() => active.value || 'narrow')
}

// useToolbarButton is every toolbar Button's look, bound with v-bind:
// secondary and outlined, and small below the medium width.
export function useToolbarButton() {
  const layout = useLayout()
  return computed(() => ({
    severity: 'secondary' as const,
    outlined: true,
    size: layout.value === 'narrow' ? ('small' as const) : undefined,
  }))
}
