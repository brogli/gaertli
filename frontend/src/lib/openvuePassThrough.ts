import type { PrimeVuePTOptions } from 'openvue/config'

// The Splitter's own mouse-down handler never cancels the browser's
// default action, and its CSS rule meant to stop selection while resizing
// keys on a class it never sets, so a drag over other text selects it.
// Canceling the gutter's mouse-down stops that selection without changing
// resizing: the gutter merges this with its own listener (mergeProps), so
// both run.
export const openvuePassThrough: PrimeVuePTOptions = {
  splitter: {
    gutter: {
      onMousedown: (event: MouseEvent) => event.preventDefault(),
    },
  },
}
