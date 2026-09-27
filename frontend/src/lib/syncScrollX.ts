// syncScrollX ties two elements' sideways scrolling together: scrolling
// either one sets the other's scrollLeft, which the browser clamps to its
// own scrollable width, so a longer side keeps scrolling past a shorter
// side's end without snapping back. Setting scrollLeft fires its own scroll
// event; that echo carries the value just written and is ignored, so the
// two sides never bounce off each other. Returns untie.
export function syncScrollX(a: HTMLElement, b: HTMLElement): () => void {
  // The value our own write last set into each side; a scroll event
  // reporting that same value is the echo of that write, not a user scroll.
  let echoA: number | null = null
  let echoB: number | null = null

  function onScrollA(): void {
    if (a.scrollLeft === echoA) {
      echoA = null
      return
    }
    b.scrollLeft = a.scrollLeft
    echoB = b.scrollLeft
  }

  function onScrollB(): void {
    if (b.scrollLeft === echoB) {
      echoB = null
      return
    }
    a.scrollLeft = b.scrollLeft
    echoA = a.scrollLeft
  }

  a.addEventListener('scroll', onScrollA)
  b.addEventListener('scroll', onScrollB)
  return () => {
    a.removeEventListener('scroll', onScrollA)
    b.removeEventListener('scroll', onScrollB)
  }
}
