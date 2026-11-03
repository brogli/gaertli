// The merge editor's panes, in display order.
export const PANES = ['current', 'result', 'incoming'] as const

export type Pane = (typeof PANES)[number]

// The read-only panes beside the result.
export type SidePane = Exclude<Pane, 'result'>

export const SIDE_PANES = PANES.filter((pane): pane is SidePane => pane !== 'result')
